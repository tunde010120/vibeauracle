package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

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
		}
		fmt.Printf(" (current: %s", Version)
		if Commit != "none" && len(Commit) >= 7 {
			fmt.Printf("-%s", Commit[:7])
		}
		fmt.Println(")")
		fmt.Println("👉 Run 'vibeaura update' to install it instantly.\n")
	}
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update vibeaura to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		curCommit := Commit
		if len(curCommit) > 7 {
			curCommit = curCommit[:7]
		}
		fmt.Printf("Current version: %s (commit: %s)\n", Version, curCommit)
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

		// Get current executable path
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("getting executable path: %w", err)
		}

		// Try to replace current binary
		fmt.Println("Installing update...")
		
		// Ensure the new binary is executable
		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			return fmt.Errorf("setting permissions on new binary: %w", err)
		}

		// Move temp file to current executable path
		if err := os.Rename(tmpFile.Name(), exePath); err != nil {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("could not replace running binary on Windows. Please download and install manually.")
			}

			// If rename fails (e.g. permission denied or cross-device), try sudo mv
			fmt.Println("Permission denied or cross-device move. Trying with sudo...")
			
			// We use 'sudo mv' because it's the most frictionless way to handle /usr/local/bin
			sudoCmd := exec.Command("sudo", "mv", tmpFile.Name(), exePath)
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

		fmt.Printf("Successfully updated to %s!\n", latest.TagName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
