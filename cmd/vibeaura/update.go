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
	"golang.org/x/mod/semver"
	"syscall"

	"github.com/spf13/cobra"
)

const repo = "nathfavour/vibeauracle"

type releaseInfo struct {
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	Prerelease      bool   `json:"prerelease"`
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
		Timeout:   30 * time.Second,
	}
}

// fetchWithFallback attempts to fetch a URL using Go's http client,
// and falls back to 'curl' if a network error occurs.
func fetchWithFallback(url string) ([]byte, error) {
	client := getResilientClient()
	resp, err := client.Get(url)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return io.ReadAll(resp.Body)
		}
		// If it's a 404 or other error, we don't want to fallback to curl
		// if the Go client successfully contacted the server.
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
		}
	}

	// Network error detected (e.g. DNS failure), try curl as a fallback (highly reliable on Termux/Mobile)
	if _, curlErr := exec.LookPath("curl"); curlErr == nil {
		// Use -f to fail on server errors, -s for silent, -L to follow redirects
		cmd := exec.Command("curl", "-fsL", url)
		data, cmdErr := cmd.Output()
		if cmdErr == nil {
			return data, nil
		}
	}

	return nil, err // Return original Go error if curl also fails or is missing
}

func getLatestRelease(channel string) (*releaseInfo, error) {
	var data []byte
	var err error

	// If no specific channel is requested, use the standard GitHub 'latest' endpoint
	if channel == "" {
		data, err = fetchWithFallback(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
		if err == nil {
			var latest releaseInfo
			if err := json.Unmarshal(data, &latest); err == nil && latest.TagName != "" {
				// Success, return it
				populateActualSHA(&latest)
				return &latest, nil
			}
		}
	}

	// Fallback or explicit channel request: fetch the list of all releases
	data, err = fetchWithFallback(fmt.Sprintf("https://api.github.com/repos/%s/releases", repo))
	if err != nil {
		return nil, err
	}

	var releases []releaseInfo
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	var latest *releaseInfo

	// If a specific channel is requested, find the exact match (or fallback to SemVer)
	if channel != "" {
		for i := range releases {
			if strings.EqualFold(releases[i].TagName, channel) {
				latest = &releases[i]
				break
			}
		}
	}

	// If no channel match found, or channel is empty, find the best release via SemVer
	if latest == nil {
		for i := range releases {
			tag := releases[i].TagName
			
			// Priority: if channel is empty, prefer "latest" or valid semver stable releases
			if channel == "" && tag == "latest" {
				latest = &releases[i]
				break
			}

			vTag := tag
			if !strings.HasPrefix(vTag, "v") {
				vTag = "v" + vTag
			}

			// Only consider non-prereleases for automatic 'latest' if channel is empty
			if semver.IsValid(vTag) && (channel != "" || semver.Prerelease(vTag) == "") {
				if latest == nil {
					latest = &releases[i]
					continue
				}

				latestVTag := latest.TagName
				if latestVTag == "latest" {
					continue // Already found "latest", respect it
				}
				if !strings.HasPrefix(latestVTag, "v") {
					latestVTag = "v" + latestVTag
				}

				if semver.IsValid(latestVTag) && semver.Compare(vTag, latestVTag) > 0 {
					latest = &releases[i]
				}
			}
		}
	}

	// Final fallback: just the first release in the list if still nil (and it has a tag)
	if latest == nil && len(releases) > 0 {
		latest = &releases[0]
	}

	if latest == nil || latest.TagName == "" {
		return nil, fmt.Errorf("could not resolve a valid release")
	}

	populateActualSHA(latest)
	return latest, nil
}

func populateActualSHA(latest *releaseInfo) {
	// Try to fetch metadata.json from assets for precise versioning
	for _, asset := range latest.Assets {
		if asset.Name == "metadata.json" {
			metaData, err := fetchWithFallback(asset.BrowserDownloadURL)
			if err == nil {
				var m metadata
				if err := json.Unmarshal(metaData, &m); err == nil && m.Commit != "" {
					latest.ActualSHA = m.Commit
					return
				}
			}
		}
	}

	// Fallback to tag-based SHA resolution if metadata.json is missing
	tagData, err := fetchWithFallback(fmt.Sprintf("https://api.github.com/repos/%s/git/ref/tags/%s", repo, latest.TagName))
	if err == nil {
		var tagInfo struct {
			Object struct {
				SHA string `json:"sha"`
			} `json:"object"`
		}
		if err := json.Unmarshal(tagData, &tagInfo); err == nil && tagInfo.Object.SHA != "" {
			latest.ActualSHA = tagInfo.Object.SHA
		} else {
			// Try branch-based if tag resolution fails (for 'latest' or 'beta' which might be branches/rolling tags)
			sha, _ := getBranchCommitSHA(latest.TagName)
			if sha != "" {
				latest.ActualSHA = sha
			}
		}
	}
}

func isUpdateAvailable(latest *releaseInfo, silent bool) bool {
	if latest == nil || latest.TagName == "" {
		return false
	}

	// 1. Try Semantic Versioning comparison
	vLocal := Version
	if !strings.HasPrefix(vLocal, "v") && semver.IsValid("v"+vLocal) {
		vLocal = "v" + vLocal
	}
	vRemote := latest.TagName
	if !strings.HasPrefix(vRemote, "v") && semver.IsValid("v"+vRemote) {
		vRemote = "v" + vRemote
	}

	if semver.IsValid(vLocal) && semver.IsValid(vRemote) {
		return semver.Compare(vRemote, vLocal) > 0
	}

	// 2. Rolling tags logic (latest/beta)
	// If the tags match, we MUST compare SHAs to know if there's a new build.
	if latest.TagName == Version {
		return latest.ActualSHA != "" && latest.ActualSHA != Commit
	}

	// 3. Fallback: if we are in a dev build, we usually don't want auto-update prompts
	// when in silent mode (startup check).
	if silent && (Version == "dev" || strings.HasPrefix(Version, "dev-")) {
		return false
	}

	// 4. Default fallback: if names differ and aren't semver, it's likely an update
	return true
}

func getBranchCommitSHA(branch string) (string, error) {
	data, err := fetchWithFallback(fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, branch))
	if err != nil {
		return "", err
	}

	var commit struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(data, &commit); err != nil {
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
	var latest *releaseInfo

	if useBeta && !buildFromSource {
		latest, err = getLatestRelease("beta")
		if err == nil && isUpdateAvailable(latest, true) {
			latestSHA = latest.ActualSHA
			latestTag = latest.TagName
			channel = "Beta"
		}
	} else if buildFromSource {
		branch := "release"
		if useBeta {
			branch = "master"
		}
		latestSHA, _ = getBranchCommitSHA(branch)
		latestTag = branch
		channel = "Source (" + branch + ")"
	} else {
		latest, err = getLatestRelease("")
		if err == nil && isUpdateAvailable(latest, true) {
			latestSHA = latest.ActualSHA
			latestTag = latest.TagName
			channel = "Stable"
		}
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
				updated, err := updateFromSource(branch, cm)
				if err == nil && updated {
					restartSelf()
				} else if err != nil {
					// Mark as failed so we don't try again
					cfg.Update.FailedCommits = append(cfg.Update.FailedCommits, latestSHA)
					cm.Save(cfg)
				}
			} else if latest != nil {
				// Stable/Beta binary update
				err := performBinaryUpdate(latest)
				if err == nil {
					restartSelf()
				} else {
					// Binary updates usually don't "fail" in the same way builds do,
					// but we'll mark it anyway if it does.
					cfg.Update.FailedCommits = append(cfg.Update.FailedCommits, latestSHA)
					cm.Save(cfg)
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
	}

	data, err := fetchWithFallback(downloadURL)
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("", "vibeaura-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
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

// restartSelf replaces the current process with the newly installed binary
func restartSelf() {
	if runtime.GOOS == "windows" {
		fmt.Println("\n✅ Update complete. Please restart vibeaura.")
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable path for restart: %v\n", err)
		os.Exit(1)
	}

	// Hand off to the new binary while preserving environment and arguments
	err = syscall.Exec(exe, os.Args, os.Environ())
	if err != nil {
		fmt.Printf("Error handing off to new binary: %v\n", err)
		os.Exit(1)
	}
}

func updateFromSource(branch string, cm *sys.ConfigManager) (bool, error) {
	cfg, _ := cm.Load()
	verbose := cfg.Update.Verbose

	// Check if Go is installed
	if _, err := exec.LookPath("go"); err != nil {
		return false, fmt.Errorf("Go is not installed. Source build requires Go.")
	}
	// Check if git is installed
	if _, err := exec.LookPath("git"); err != nil {
		return false, fmt.Errorf("Git is not installed. Source build requires Git.")
	}

	sourceRoot := cm.GetDataPath(filepath.Join("source", branch))
	if err := os.MkdirAll(filepath.Dir(sourceRoot), 0755); err != nil {
		return false, fmt.Errorf("creating source directory: %w", err)
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
			return false, fmt.Errorf("cloning repo: %w", err)
		}
	} else {
		if verbose {
			fmt.Printf("Fetching updates for %s...\n", branch)
		}
		fetchCmd := exec.Command("git", "-C", sourceRoot, "fetch", "origin", branch)
		if err := fetchCmd.Run(); err != nil {
			return false, fmt.Errorf("fetching updates: %w", err)
		}

		// Get remote SHA
		remoteCmd := exec.Command("git", "-C", sourceRoot, "rev-parse", "origin/"+branch)
		remoteSHABytes, err := remoteCmd.Output()
		if err != nil {
			return false, fmt.Errorf("getting remote SHA: %w", err)
		}
		remoteSHA := strings.TrimSpace(string(remoteSHABytes))

		if remoteSHA == Commit && !strings.HasPrefix(Version, "dev") {
			return false, nil
		}

		// Check if this commit previously failed
		for _, failed := range cfg.Update.FailedCommits {
			if failed == remoteSHA {
				return false, nil
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
			return false, fmt.Errorf("pulling updates: %w", err)
		}
	}

	return buildAndInstallFromSource(sourceRoot, branch, cm)
}

func buildAndInstallFromSource(sourceRoot, branch string, cm *sys.ConfigManager) (bool, error) {
	cfg, err := cm.Load()
	if err != nil {
		return false, err
	}
	verbose := cfg.Update.Verbose

	if verbose {
		fmt.Println("Building from source...")
	}
	
	// Get current commit SHA for the local build
	commitCmd := exec.Command("git", "-C", sourceRoot, "rev-parse", "HEAD")
	commitSHABytes, _ := commitCmd.Output()
	localCommit := strings.TrimSpace(string(commitSHABytes))

	// Final check: if the localCommit we just pulled/checked out matches current Commit, no update needed.
	// This covers cases where 'remoteSHA' was fetched but we are already running that code.
	if localCommit == Commit && !strings.HasPrefix(Version, "dev") {
		return false, nil
	}
	
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
		goos, _ := getPlatform()
		if goos == "android" {
			fmt.Println("\n🛠️  Build failed. Attempting to upgrade Go toolchain automatically...")
			upgradeCmd := exec.Command("pkg", "upgrade", "golang", "-y")
			upgradeCmd.Stdout = os.Stdout
			upgradeCmd.Stderr = os.Stderr
			if err := upgradeCmd.Run(); err == nil {
				fmt.Println("✅ Go upgraded. Retrying build...")
				if err := buildCmd.Run(); err == nil {
					if !verbose {
						fmt.Println("DONE")
					}
					if err := installBinary(buildOut); err != nil {
						return false, err
					}
					return true, nil
				}
			}
		}

		if verbose {
			fmt.Println("\n❌ Build failed! This usually happens if your installed Go version is older than the one required by the project.")
			if goos == "android" {
				fmt.Println("👉 Try running: pkg upgrade golang (on Termux)")
			} else {
				fmt.Println("👉 Try updating Go on your desktop.")
			}
		}
		commitCmd := exec.Command("git", "-C", sourceRoot, "rev-parse", "HEAD")
		if out, err := commitCmd.Output(); err == nil {
			failedSHA := strings.TrimSpace(string(out))
			cfg, err := cm.Load()
			if err == nil {
				cfg.Update.FailedCommits = append(cfg.Update.FailedCommits, failedSHA)
				cm.Save(cfg)
			}
		}
		return false, fmt.Errorf("building from source: %w", err)
	}

	if !verbose {
		fmt.Println("DONE")
	}

	if err := installBinary(buildOut); err != nil {
		return false, err
	}

	return true, nil
}

var (
	betaFlag       bool
	listAssetsFlag bool
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

		if listAssetsFlag {
			if buildFromSource {
				return fmt.Errorf("--list-assets is only supported for the pre-built update pipeline (source updates do not use assets)")
			}

			fmt.Println("Fetching latest release assets...")
			reqChannel := ""
			if useBeta {
				reqChannel = "beta"
			}
			latest, err := getLatestRelease(reqChannel)
			if err != nil {
				return fmt.Errorf("checking for updates: %w", err)
			}

			fmt.Printf("\n📦 Assets for release %s:\n", latest.TagName)
			for _, asset := range latest.Assets {
				fmt.Printf("  - %s\n", asset.Name)
			}
			fmt.Println()
			return nil
		}

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
			
			updated, err := updateFromSource(branch, cm)
			if err != nil {
				if !verbose {
					fmt.Println("FAILED")
				}
				return err
			}

			if !updated {
				if !verbose {
					fmt.Println("ALREADY UP TO DATE")
				} else {
					fmt.Println("vibeaura is already up to date on this branch.")
				}
				return nil
			}

			if !verbose {
				fmt.Println("DONE")
			} else {
				fmt.Printf("Successfully updated to bleeding-edge %s from source!\n", branch)
			}
			restartSelf()
			return nil
		}

		fmt.Println("Checking for updates...")
		reqChannel := ""
		if useBeta {
			reqChannel = "beta"
		}
		latest, err := getLatestRelease(reqChannel)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		isDev := strings.HasPrefix(Version, "dev")
		if !isUpdateAvailable(latest, false) && !isDev {
			fmt.Println("vibeaura is already up to date!")
			return nil
		}

		if isDev {
			fmt.Printf("Dev build detected. Force-updating to latest stable binary (%s)...\n", latest.TagName)
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

		restartSelf()
		return nil
	},
}

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

	fmt.Println("DONE")
	restartSelf()
	return nil
}

func rollbackFromSource(target string, cm *sys.ConfigManager) error {
	cm, _ = sys.NewConfigManager()
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

	fmt.Println("DONE")
	restartSelf()
	return nil
}

func init() {
	updateCmd.Flags().BoolVar(&betaFlag, "beta", false, "Install bleeding-edge version from source (master branch)")
	updateCmd.Flags().BoolVar(&listAssetsFlag, "list-assets", false, "List all assets available in the latest release")
	
	rollbackCmd.Flags().StringVar(&rollbackVersion, "version", "", "Specific version/commit to roll back to")
	updateCmd.AddCommand(rollbackCmd)

	rootCmd.AddCommand(updateCmd)
}
