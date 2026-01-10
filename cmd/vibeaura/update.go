package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const repo = "nathfavour/vibeauracle"

type releaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update vibeaura to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Current version: %s\n", Version)
		fmt.Println("Checking for updates...")

		// Get latest release
		resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases", repo))
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}
		defer resp.Body.Close()

		var releases []releaseInfo
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			return fmt.Errorf("decoding release info: %w", err)
		}

		if len(releases) == 0 {
			return fmt.Errorf("no releases found")
		}

		latest := releases[0]
		if latest.TagName == Version && Version != "dev" {
			fmt.Println("vibeaura is already up to date!")
			return nil
		}

		fmt.Printf("New version available: %s\n", latest.TagName)

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

		resp, err = http.Get(downloadURL)
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
		
		// On Unix, we can rename/overwrite the running executable
		// On Windows, this is harder, but for now we focus on frictionless Linux/macOS
		
		// Ensure the new binary is executable
		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			return fmt.Errorf("setting permissions on new binary: %w", err)
		}

		// Move temp file to current executable path
		// We use a rename which is atomic on most Unix systems
		if err := os.Rename(tmpFile.Name(), exePath); err != nil {
			if strings.Contains(err.Error(), "permission denied") {
				fmt.Printf("\nPermission denied. Please run the update with sudo:\n")
				fmt.Printf("sudo vibeaura update\n\n")
				return nil
			}
			return fmt.Errorf("replacing binary: %w", err)
		}

		fmt.Printf("Successfully updated to %s!\n", latest.TagName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

