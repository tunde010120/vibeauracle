package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/nathfavour/vibeauracle/sys"

	"github.com/spf13/cobra"
)

const repo = "nathfavour/vibeauracle"

type releaseInfo struct {
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	ActualSHA       string `json:"-"`
	Assets          []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func getLatestRelease() (*releaseInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases", repo))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releases []releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	latest := releases[0]

	// Fetch actual SHA for the tag to avoid 'master' or 'release' branch name issues
	tagResp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/git/ref/tags/%s", repo, latest.TagName))
	if err == nil {
		defer tagResp.Body.Close()
		var tagInfo struct {
			Object struct {
				SHA string `json:"sha"`
			} `json:"object"`
		}
		if err := json.NewDecoder(tagResp.Body).Decode(&tagInfo); err == nil && tagInfo.Object.SHA != "" {
			latest.ActualSHA = tagInfo.Object.SHA
		}
	}

	return &latest, nil
}

func isUpdateAvailable(latest *releaseInfo) bool {
	if Version == "dev" {
		return true // Always allow update from dev
	}

	remoteVer := latest.ActualSHA
	if remoteVer == "" {
		remoteVer = latest.TargetCommitish
	}

	// If tags differ, update is available
	if latest.TagName != Version {
		return true
	}

	// If tags match (e.g. both are 'latest'), compare SHAs
	return remoteVer != Commit
}

// checkUpdateSilent checks for updates and prints a message if one is available
func checkUpdateSilent() {
	latest, err := getLatestRelease()
	if err != nil {
		return // Fail silently for background checks
	}

	if isUpdateAvailable(latest) {
		remoteSHA := latest.ActualSHA
		if remoteSHA == "" {
			remoteSHA = latest.TargetCommitish
		}

		fmt.Printf("\n✨ A new version of vibeaura is available: %s", latest.TagName)
		if len(remoteSHA) >= 7 {
			fmt.Printf(" (%s)", remoteSHA[:7])
		} else if remoteSHA != "" {
			fmt.Printf(" (%s)", remoteSHA)
		}
		fmt.Printf(" (current: %s", Version)
		if Commit != "none" {
			if len(Commit) >= 7 {
				fmt.Printf("-%s", Commit[:7])
			} else {
				fmt.Printf("-%s", Commit)
			}
		}
		fmt.Println(")")
		fmt.Println("👉 Run 'vibeaura update' to install it instantly.\n")
	}
}

func installBinary(srcPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	fmt.Println("Installing binary...")
	
	// Ensure the new binary is executable
	if err := os.Chmod(srcPath, 0755); err != nil {
		return fmt.Errorf("setting permissions on new binary: %w", err)
	}

	// Move temp file to current executable path
	if err := os.Rename(srcPath, exePath); err != nil {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("could not replace running binary on Windows. Please download and install manually.")
		}

		// If rename fails (e.g. permission denied or cross-device), try sudo mv
		fmt.Println("Permission denied or cross-device move. Trying with sudo...")
		
		sudoCmd := exec.Command("sudo", "mv", srcPath, exePath)
		sudoCmd.Stdout = os.Stdout
		sudoCmd.Stderr = os.Stderr
		sudoCmd.Stdin = os.Stdin // For password prompt
		
		if err := sudoCmd.Run(); err != nil {
			return fmt.Errorf("replacing binary with sudo: %w", err)
		}
	}

	// Ensure the final binary is executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(exePath, 0755); err != nil {
			exec.Command("sudo", "chmod", "+x", exePath).Run()
		}
	}

	return nil
}

func updateFromSource(branch string, cm *sys.ConfigManager) error {
	// Check if Go is installed
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go is not installed. Source build requires Go.")
	}
	// Check if git is installed
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("Git is not installed. Source build requires Git.")
	}

	sourceRoot := cm.GetDataPath(filepath.Join("source", branch))
	if err := os.MkdirAll(filepath.Dir(sourceRoot), 0755); err != nil {
		return fmt.Errorf("creating source directory: %w", err)
	}

	if _, err := os.Stat(filepath.Join(sourceRoot, ".git")); os.IsNotExist(err) {
		fmt.Printf("Cloning %s branch to %s...\n", branch, sourceRoot)
		cloneCmd := exec.Command("git", "clone", "-b", branch, "https://github.com/"+repo+".git", sourceRoot)
		cloneCmd.Stdout = os.Stdout
		cloneCmd.Stderr = os.Stderr
		if err := cloneCmd.Run(); err != nil {
			os.RemoveAll(sourceRoot)
			return fmt.Errorf("cloning repo: %w", err)
		}
	} else {
		fmt.Printf("Updating local source in %s...\n", sourceRoot)
		pullCmd := exec.Command("git", "-C", sourceRoot, "pull", "origin", branch)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("pulling updates: %w", err)
		}
	}

	fmt.Println("Building from source...")
	buildOut := filepath.Join(sourceRoot, "vibeaura_new")
	buildCmd := exec.Command("go", "build", "-o", buildOut, "./cmd/vibeaura")
	buildCmd.Dir = sourceRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	
	if err := buildCmd.Run(); err != nil {
		fmt.Println("\n❌ Build failed! The beta version might be unstable.")
		return fmt.Errorf("building from source: %w", err)
	}

	if err := installBinary(buildOut); err != nil {
		return err
	}

	fmt.Printf("Successfully updated to bleeding-edge %s from source!\n", branch)
	return nil
}

var (
	betaFlag bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update vibeaura to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		cm, err := sys.NewConfigManager()
		if err != nil {
			return fmt.Errorf("initializing config: %w", err)
		}
		cfg, err := cm.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		useBeta := betaFlag || cfg.Update.Beta
		buildFromSource := cfg.Update.BuildFromSource || useBeta

		curCommit := Commit
		if len(curCommit) > 7 {
			curCommit = curCommit[:7]
		}
		fmt.Printf("Current version: %s (commit: %s)\n", Version, curCommit)

		if buildFromSource {
			branch := "release"
			if useBeta {
				branch = "master"
				fmt.Println("🚀 Entering Beta Mode: Building bleeding-edge from master...")
			} else {
				fmt.Println("🛠️ Building from source (release branch)...")
			}
			return updateFromSource(branch, cm)
		}

		fmt.Println("Checking for updates...")
		latest, err := getLatestRelease()
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		if !isUpdateAvailable(latest) {
			fmt.Println("vibeaura is already up to date!")
			return nil
		}

		remoteVer := latest.ActualSHA
		if remoteVer == "" {
			remoteVer = latest.TargetCommitish
		}
		if len(remoteVer) > 7 {
			remoteVer = remoteVer[:7]
		}

		fmt.Printf("New version available: %s (commit: %s)\n", latest.TagName, remoteVer)

		// ... (rest of the download/install logic)

		// Determine target asset name
		goos := runtime.GOOS
		goarch := runtime.GOARCH
		targetAsset := fmt.Sprintf("vibeaura-%s-%s", goos, goarch)
		if goos == "windows" {
			targetAsset += ".exe"
		}

		var downloadURL string
		for _, asset := range latest.Assets {
			if asset.Name == targetAsset {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}

		if downloadURL == "" {
			return fmt.Errorf("could not find binary for %s/%s in release %s", goos, goarch, latest.TagName)
		}

		fmt.Printf("Downloading %s...\n", targetAsset)
		
		// Download to temp file
		tmpFile, err := os.CreateTemp("", "vibeaura-update-*")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		resp, err := http.Get(downloadURL)
		if err != nil {
			return fmt.Errorf("downloading update: %w", err)
		}
		defer resp.Body.Close()

		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			return fmt.Errorf("saving update: %w", err)
		}
		tmpFile.Close()

		if err := installBinary(tmpFile.Name()); err != nil {
			return err
		}

		fmt.Printf("Successfully updated to %s!\n", latest.TagName)
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&betaFlag, "beta", false, "Install bleeding-edge version from source (master branch)")
	rootCmd.AddCommand(updateCmd)
}
