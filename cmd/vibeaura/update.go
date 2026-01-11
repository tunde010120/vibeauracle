package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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

type metadata struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func getResilientClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Try IPv4 first if dual-stack DNS is flaky
			conn, err := dialer.DialContext(ctx, "tcp4", addr)
			if err != nil {
				// Fallback to default behavior (IPv6/IPv4 as system prefers)
				return dialer.DialContext(ctx, "tcp", addr)
			}
			return conn, nil
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}

func getLatestRelease() (*releaseInfo, error) {
	client := getResilientClient()
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

	// Try to fetch metadata.json from assets for precise versioning
	for _, asset := range latest.Assets {
		if asset.Name == "metadata.json" {
			metaResp, err := client.Get(asset.BrowserDownloadURL)
			if err == nil {
				defer metaResp.Body.Close()
				var m metadata
				if err := json.NewDecoder(metaResp.Body).Decode(&m); err == nil && m.Commit != "" {
					latest.ActualSHA = m.Commit
					return &latest, nil
				}
			}
		}
	}

	// Fallback to tag-based SHA resolution if metadata.json is missing
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
	// If we are in a dev build, we don't automatically suggest updates unless it's a forced check
	// This avoids the "dumb" behavior of dev always suggesting updates.
	if Version == "dev" {
		return false 
	}

	remoteSHA := latest.ActualSHA
	if remoteSHA == "" {
		// If we still don't have a SHA, we can't reliably say there's an update
		// unless the tag name is different.
		return latest.TagName != Version
	}

	// If tags match (e.g. both are 'latest'), compare SHAs
	if latest.TagName == Version {
		return remoteSHA != Commit
	}

	// Otherwise, tags differ, so update is available
	return true
}

func getBranchCommitSHA(branch string) (string, error) {
	client := getResilientClient()
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, branch))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var commit struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return "", err
	}
	return commit.SHA, nil
}

// checkUpdateSilent checks for updates and prints a message if one is available.
// If auto-update is enabled, it attempts to update quietly.
func checkUpdateSilent() {
	cm, err := sys.NewConfigManager()
	if err != nil {
		return
	}
	cfg, err := cm.Load()
	if err != nil {
		return
	}

	useBeta := cfg.Update.Beta
	buildFromSource := cfg.Update.BuildFromSource || useBeta
	autoUpdate := cfg.Update.AutoUpdate

	var latestSHA string
	var latestTag string
	var channel string

	if useBeta {
		latestSHA, _ = getBranchCommitSHA("master")
		latestTag = "beta"
		channel = "Beta (master)"
	} else if buildFromSource {
		latestSHA, _ = getBranchCommitSHA("release")
		latestTag = "source"
		channel = "Source (release)"
	} else {
		latest, err := getLatestRelease()
		if err != nil {
			return
		}
		if !isUpdateAvailable(latest) {
			return
		}
		latestSHA = latest.ActualSHA
		latestTag = latest.TagName
		channel = "Stable"
	}

	if latestSHA != "" && latestSHA != Commit {
		// Check if this commit has previously failed to build
		for _, failed := range cfg.Update.FailedCommits {
			if failed == latestSHA {
				return // Don't nag or auto-update for a known failed commit
			}
		}

		if autoUpdate {
			// Perform quiet auto-update
			if buildFromSource {
				branch := "release"
				if useBeta {
					branch = "master"
				}
				// We run this in a way that doesn't block the main tool too much, 
				// but since it's "integrated", we'll just run it.
				// Note: installBinary might request sudo, which isn't exactly "quiet".
				// But for many users (like in /usr/local/bin), they will see the sudo prompt.
				err := updateFromSource(branch, cm)
				if err != nil {
					// Mark as failed so we don't try again
					cfg.Update.FailedCommits = append(cfg.Update.FailedCommits, latestSHA)
					cm.Save(cfg)
				}
			} else {
				// Stable binary update
				latest, _ := getLatestRelease() // Re-fetch to get assets
				if latest != nil {
					err := performBinaryUpdate(latest)
					if err != nil {
						// Binary updates usually don't "fail" in the same way builds do,
						// but we'll mark it anyway if it does.
						cfg.Update.FailedCommits = append(cfg.Update.FailedCommits, latestSHA)
						cm.Save(cfg)
					}
				}
			}
			return // After auto-update, no need to print notification
		}

		styleNew := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)      // Bright Green
		styleChannel := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Italic(true) // Bright Blue
		styleCmd := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)      // Bright Yellow
		styleDim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))                  // Gray

		displayLatestSHA := latestSHA
		if len(displayLatestSHA) >= 7 {
			displayLatestSHA = displayLatestSHA[:7]
		}

		displayCurCommit := Commit
		if len(displayCurCommit) >= 7 {
			displayCurCommit = displayCurCommit[:7]
		}

		fmt.Println()
		fmt.Printf("✨ %s %s %s\n",
			styleNew.Render("A new update is available on the"),
			styleChannel.Render(channel),
			styleNew.Render("channel!"),
		)
		fmt.Printf("   %s %s (%s) %s %s\n",
			styleDim.Render("Latest:"), displayLatestSHA, latestTag,
			styleDim.Render("Current:"), displayCurCommit,
		)
		fmt.Printf("   👉 Run %s %s\n",
			styleCmd.Render("vibeaura update"),
			styleDim.Render("to stay on the bleeding edge."),
		)
		fmt.Println()
	}
}

func getPlatform() (string, string) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Termux/Android detection
	if goos == "linux" {
		if _, err := os.Stat("/data/data/com.termux/files/usr/bin/bash"); err == nil || os.Getenv("TERMUX_VERSION") != "" {
			goos = "android"
		}
	}

	return goos, goarch
}

func performBinaryUpdate(latest *releaseInfo) error {
	cm, _ := sys.NewConfigManager()
	cfg, _ := cm.Load()
	verbose := cfg.Update.Verbose

	// Determine target asset name
	goos, goarch := getPlatform()
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
		return fmt.Errorf("no binary for %s/%s", goos, goarch)
	}

	if verbose {
		fmt.Printf("Downloading %s...\n", targetAsset)
	} else {
		// Silent
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp("", "vibeaura-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}
	tmpFile.Close()

	return installBinary(tmpFile.Name())
}

func installBinary(srcPath string) error {
	cm, _ := sys.NewConfigManager()
	cfg, _ := cm.Load()
	verbose := cfg.Update.Verbose

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	if verbose {
		fmt.Println("Installing binary...")
	}
	
	// Ensure the new binary is executable
	if err := os.Chmod(srcPath, 0755); err != nil {
		return fmt.Errorf("setting permissions on new binary: %w", err)
	}

	// Move temp file to current executable path
	if err := os.Rename(srcPath, exePath); err != nil {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("could not replace running binary on Windows. Please download and install manually.")
		}

		goos, _ := getPlatform()
		if goos == "android" {
			// On Termux, sudo is missing. Try a direct move as it should be in user home.
			// If rename fails, it might be cross-device.
			cpCmd := exec.Command("cp", srcPath, exePath)
			if err := cpCmd.Run(); err != nil {
				return fmt.Errorf("replacing binary on Android: %w", err)
			}
			return nil
		}

		// If rename fails (e.g. permission denied or cross-device), try sudo mv
		if verbose {
			fmt.Println("Permission denied or cross-device move. Trying with sudo...")
		} else {
			fmt.Print("🔒  Elevating for installation... ")
		}
		
		sudoCmd := exec.Command("sudo", "mv", srcPath, exePath)
		sudoCmd.Stdout = os.Stdout
		sudoCmd.Stderr = os.Stderr
		sudoCmd.Stdin = os.Stdin // For password prompt
		
		if err := sudoCmd.Run(); err != nil {
			if !verbose {
				fmt.Println("FAILED")
			}
			return fmt.Errorf("replacing binary with sudo: %w", err)
		}
		if !verbose {
			fmt.Println("DONE")
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
	cfg, _ := cm.Load()
	verbose := cfg.Update.Verbose

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
		if verbose {
			fmt.Printf("Cloning %s branch to %s...\n", branch, sourceRoot)
		}
		cloneCmd := exec.Command("git", "clone", "-b", branch, "https://github.com/"+repo+".git", sourceRoot)
		if verbose {
			cloneCmd.Stdout = os.Stdout
			cloneCmd.Stderr = os.Stderr
		}
		if err := cloneCmd.Run(); err != nil {
			os.RemoveAll(sourceRoot)
			return fmt.Errorf("cloning repo: %w", err)
		}
	} else {
		if verbose {
			fmt.Printf("Fetching updates for %s...\n", branch)
		}
		fetchCmd := exec.Command("git", "-C", sourceRoot, "fetch", "origin", branch)
		if err := fetchCmd.Run(); err != nil {
			return fmt.Errorf("fetching updates: %w", err)
		}

		// Get remote SHA
		remoteCmd := exec.Command("git", "-C", sourceRoot, "rev-parse", "origin/"+branch)
		remoteSHABytes, err := remoteCmd.Output()
		if err != nil {
			return fmt.Errorf("getting remote SHA: %w", err)
		}
		remoteSHA := strings.TrimSpace(string(remoteSHABytes))

		if remoteSHA == Commit && Version != "dev" {
			return nil
		}

		// Check if this commit previously failed
		for _, failed := range cfg.Update.FailedCommits {
			if failed == remoteSHA {
				return nil
			}
		}

		if verbose {
			fmt.Printf("Updating local source in %s...\n", sourceRoot)
		}
		pullCmd := exec.Command("git", "-C", sourceRoot, "pull", "origin", branch)
		if verbose {
			pullCmd.Stdout = os.Stdout
			pullCmd.Stderr = os.Stderr
		}
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("pulling updates: %w", err)
		}
	}

	if verbose {
		fmt.Println("Building from source...")
	}
	
	// Get current commit SHA for the local build
	commitCmd := exec.Command("git", "-C", sourceRoot, "rev-parse", "HEAD")
	commitSHABytes, _ := commitCmd.Output()
	localCommit := strings.TrimSpace(string(commitSHABytes))
	
	buildDate := time.Now().UTC().Format(time.RFC3339)
	ldflags := fmt.Sprintf("-s -w -X main.Version=%s -X main.Commit=%s -X main.BuildDate=%s", branch, localCommit, buildDate)

	buildOut := filepath.Join(sourceRoot, "vibeaura_new")
	buildCmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", buildOut, "./cmd/vibeaura")
	buildCmd.Dir = sourceRoot
	
	// Force Go to use the locally installed toolchain and avoid automatic downloads
	// which often fail on mobile/Termux.
	buildCmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")

	if verbose {
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
	}
	
	if err := buildCmd.Run(); err != nil {
		if verbose {
			fmt.Println("\n❌ Build failed! This usually happens if your installed Go version is older than the one required by the project.")
			fmt.Println("👉 Try running: pkg upgrade golang (on Termux) or update Go on your desktop.")
		}
		// Quietly mark this commit as failed if possible
		commitCmd := exec.Command("git", "-C", sourceRoot, "rev-parse", "HEAD")
		if out, err := commitCmd.Output(); err == nil {
			failedSHA := strings.TrimSpace(string(out))
			cfg, err := cm.Load()
			if err == nil {
				cfg.Update.FailedCommits = append(cfg.Update.FailedCommits, failedSHA)
				cm.Save(cfg)
			}
		}
		return fmt.Errorf("building from source: %w", err)
	}

	if !verbose {
		fmt.Println("DONE")
	}

	if err := installBinary(buildOut); err != nil {
		return err
	}

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
		verbose := cfg.Update.Verbose

		curCommit := Commit
		if len(curCommit) > 7 {
			curCommit = curCommit[:7]
		}
		
		if verbose {
			fmt.Printf("Current version: %s (commit: %s)\n", Version, curCommit)
		}

		if buildFromSource {
			branch := "release"
			if useBeta {
				branch = "master"
			}
			
			if !verbose {
				fmt.Printf("🔄  Updating to %s... ", branch)
			} else {
				if useBeta {
					fmt.Println("🚀 Entering Beta Mode: Building bleeding-edge from master...")
				} else {
					fmt.Println("🛠️ Building from source (release branch)...")
				}
			}
			
			err := updateFromSource(branch, cm)
			if err != nil {
				if !verbose {
					fmt.Println("FAILED")
				}
				return err
			}
			if !verbose {
				fmt.Println("DONE")
			} else {
				fmt.Printf("Successfully updated to bleeding-edge %s from source!\n", branch)
			}
			return nil
		}

		fmt.Println("Checking for updates...")
		latest, err := getLatestRelease()
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		if !isUpdateAvailable(latest) && Version != "dev" {
			fmt.Println("vibeaura is already up to date!")
			return nil
		}

		remoteVer := latest.ActualSHA
		if remoteVer == "" {
			remoteVer = latest.TargetCommitish
		}

		// Check if this commit has previously failed
		for _, failed := range cfg.Update.FailedCommits {
			if failed == remoteVer {
				fmt.Printf("\n⚠️ The latest version (%s) has previously failed to install/build and is likely unstable.\n", remoteVer[:7])
				fmt.Println("👉 Use '--beta' or '--source' flags to force a retry if you've fixed the issue.")
				return nil
			}
		}
		
		displaySHA := remoteVer
		if len(displaySHA) > 7 {
			displaySHA = displaySHA[:7]
		}

		fmt.Printf("New version available: %s (commit: %s)\n", latest.TagName, displaySHA)

		// Determine target asset name
		goos, goarch := getPlatform()
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

		if verbose {
			fmt.Printf("Downloading %s...\n", targetAsset)
		}
		
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

		if verbose {
			fmt.Printf("Successfully updated to %s!\n", latest.TagName)
		} else {
			fmt.Println("DONE")
		}
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&betaFlag, "beta", false, "Install bleeding-edge version from source (master branch)")
	rootCmd.AddCommand(updateCmd)
}
