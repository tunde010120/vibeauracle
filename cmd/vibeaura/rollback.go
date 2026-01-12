package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nathfavour/vibeauracle/sys"
	"github.com/spf13/cobra"
)

var rollbackVersion string

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back to a previous version",
	RunE: func(cmd *cobra.Command, args []string) error {
		cm, err := sys.NewConfigManager()
		if err != nil {
			return fmt.Errorf("initializing config: %w", err)
		}
		cfg, err := cm.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		isSource := Version == "release" || Version == "master" || cfg.Update.BuildFromSource
		if isSource {
			return rollbackFromSource(rollbackVersion, cm)
		}
		return rollbackBinary(rollbackVersion)
	},
}

func rollbackBinary(target string) error {
	fmt.Println("🔍 Searching for previous versions...")
	data, err := fetchWithFallback(fmt.Sprintf("https://api.github.com/repos/%s/releases", repo))
	if err != nil {
		return fmt.Errorf("fetching releases: %w", err)
	}

	var releases []releaseInfo
	if err := json.Unmarshal(data, &releases); err != nil {
		return fmt.Errorf("parsing releases: %w", err)
	}

	var targetRelease *releaseInfo
	if target != "" {
		for i := range releases {
			if releases[i].TagName == target || strings.Contains(releases[i].TagName, target) {
				targetRelease = &releases[i]
				break
			}
		}
		if targetRelease == nil {
			return fmt.Errorf("could not find version: %s", target)
		}
	} else {
		// Find current version and pick the one after it
		currentIndex := -1
		for i := range releases {
			if releases[i].TagName == Version {
				currentIndex = i
				break
			}
		}

		if currentIndex == -1 {
			// If we can't find exact match, just take the second one (first is current latest)
			if len(releases) > 1 {
				targetRelease = &releases[1]
			}
		} else if currentIndex+1 < len(releases) {
			targetRelease = &releases[currentIndex+1]
		}

		if targetRelease == nil {
			return fmt.Errorf("no previous version found to roll back to")
		}
	}

	populateActualSHA(targetRelease)
	fmt.Printf("⏪ Rolling back to %s...\n", targetRelease.TagName)
	if err := performBinaryUpdate(targetRelease); err != nil {
		return err
	}

	// Disable auto-update after rollback
	cm, _ := sys.NewConfigManager()
	if cfg, err := cm.Load(); err == nil {
		cfg.Update.AutoUpdate = false
		cm.Save(cfg)
		fmt.Println("ℹ️  Automatic updates disabled. Run 'vibeaura update' manually to re-enable.")
	}

	printSuccess("Rollback complete")

	// For rollbacks, we don't hand off the 'rollback' command (to avoid recursion).
	// Instead, we implicitly hand off a 'version' command to the newly installed binary.
	restartWithArgs([]string{"vibeaura", "version"})
	return nil
}

func rollbackFromSource(target string, cm *sys.ConfigManager) error {
	cfg, _ := cm.Load()
	branch := Version
	if branch != "master" && branch != "release" {
		branch = "release"
		if cfg.Update.Beta {
			branch = "master"
		}
	}

	sourceRoot := cm.GetDataPath(filepath.Join("source", branch))
	if _, err := os.Stat(sourceRoot); os.IsNotExist(err) {
		return fmt.Errorf("source directory for %s not found. Please run 'update --source' first", branch)
	}

	if target == "" {
		target = "HEAD^"
	}

	fmt.Printf("⏪ Rolling back source to %s...\n", target)

	// Checkout target
	checkoutCmd := exec.Command("git", "-C", sourceRoot, "checkout", target)
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout failed: %s", string(out))
	}

	// Build and install
	updated, err := buildAndInstallFromSource(sourceRoot, branch, cm)
	if err != nil {
		return err
	}

	if !updated {
		fmt.Println("Already at requested version.")
		return nil
	}

	// Disable auto-update after rollback
	cfg.Update.AutoUpdate = false
	cm.Save(cfg)
	fmt.Println("ℹ️  Automatic updates disabled. Run 'vibeaura update' manually to re-enable.")

	fmt.Println("DONE")

	// For rollbacks, we don't hand off the 'rollback' command (to avoid recursion).
	// Instead, we implicitly hand off a 'version' command to the newly installed binary.
	restartWithArgs([]string{"vibeaura", "version"})
	return nil
}

func init() {
	rollbackCmd.Flags().StringVar(&rollbackVersion, "version", "", "Specific version/commit to roll back to")
	rootCmd.AddCommand(rollbackCmd)
}
