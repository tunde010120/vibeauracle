package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathfavour/vibeauracle/sys"
)

// execGitCommand runs a git command and returns stdout.
func execGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// --- Async Hot-Swap Logic ---

type UpdateAvailableMsg struct {
	Latest *releaseInfo
}

type UpdateReadyMsg struct {
	Target string // SHA
}

type AsyncUpdateManager struct {
	cm *sys.ConfigManager
}

func NewAsyncUpdateManager() *AsyncUpdateManager {
	cm, _ := sys.NewConfigManager()
	return &AsyncUpdateManager{cm: cm}
}

// CheckUpdateCmd returns a command that checks for updates in the background.
func (chk *AsyncUpdateManager) CheckUpdateCmd(manual bool) tea.Cmd {
	return func() tea.Msg {
		// Initial startup delay for background checks
		if !manual {
			time.Sleep(5 * time.Second)
		}

		for {
			chk.cm, _ = sys.NewConfigManager() // Reload config
			cfg, _ := chk.cm.Load()

			// Manual updates always proceed; AutoUpdate setting is only for background.
			if manual || cfg.Update.AutoUpdate {
				updateAvailable, latest := checkForUpdateSimple(cfg)
				if updateAvailable && latest != nil {
					// Don't auto-update failed commits
					failed := false
					for _, f := range cfg.Update.FailedCommits {
						if f == latest.ActualSHA {
							failed = true
							break
						}
					}
					if !failed {
						return UpdateAvailableMsg{Latest: latest}
					}
				}
			}

			// If it's a manual check and we got here, no update was found or something failed.
			if manual {
				return UpdateNoUpdateMsg{}
			}

			// Wait 30 minutes before checking again
			time.Sleep(30 * time.Minute)
		}
	}
}

// checkForUpdateSimple is a straightforward update check.
// It fetches the local git HEAD and compares it to the remote.
// Returns (updateAvailable, releaseInfo).
func checkForUpdateSimple(cfg *sys.Config) (bool, *releaseInfo) {
	// 1. Get local commit (try git first, fall back to embedded Commit var)
	localSHA := getLocalCommit()
	if localSHA == "" {
		// Can't determine local state, assume no update
		return false, nil
	}

	// 2. Get remote commit based on update channel
	var remoteSHA string
	var branch string

	if cfg.Update.BuildFromSource || cfg.Update.Beta {
		// Source builds track branches
		branch = "release"
		if cfg.Update.Beta {
			branch = "master"
		}
		sha, err := getBranchCommitSHA(branch)
		if err != nil {
			return false, nil
		}
		remoteSHA = sha
	} else {
		// Stable binary: check latest release
		latest, err := getLatestRelease("")
		if err != nil || latest == nil {
			return false, nil
		}
		// For releases, we use the actual SHA
		remoteSHA = latest.ActualSHA
		if remoteSHA == "" {
			return false, nil
		}
		// Direct comparison
		if remoteSHA != localSHA {
			return true, latest
		}
		return false, nil
	}

	// 3. Simple comparison
	if remoteSHA != localSHA {
		return true, &releaseInfo{
			TagName:   branch,
			ActualSHA: remoteSHA,
		}
	}

	return false, nil
}

// getLocalCommit tries to get the current commit hash.
// First tries `git rev-parse HEAD`, falls back to embedded Commit variable.
func getLocalCommit() string {
	// Try git first (most accurate for dev/source builds)
	if out, err := execGitCommand("rev-parse", "HEAD"); err == nil {
		return strings.TrimSpace(out)
	}

	// Fall back to embedded commit (for installed binaries)
	if Commit != "" && Commit != "none" {
		return Commit
	}

	return ""
}

type UpdateNoUpdateMsg struct{}

// DownloadUpdateCmd downloads the update in background
func (chk *AsyncUpdateManager) DownloadUpdateCmd(latest *releaseInfo) tea.Cmd {
	return func() tea.Msg {
		// For hot-swap, on Linux/Mac, we can overwrite the binary while running.
		// performBinaryUpdate is defined in update.go (package main)
		err := performBinaryUpdate(latest)
		if err != nil {
			return nil
		}
		return UpdateReadyMsg{Target: latest.ActualSHA}
	}
}

// PerformHotSwap saves state and execs the new binary
func PerformHotSwap(headers []string, input string) {
	state := map[string]interface{}{
		"messages": headers,
		"input":    input,
	}

	bytes, _ := json.Marshal(state)
	tmpState, _ := os.CreateTemp("", "vibeaura-state-*.json")
	tmpState.Write(bytes)
	tmpState.Close()

	// 2. Restart
	exe, _ := os.Executable()

	// We need to construct args. We can't just use os.Args because we need to strip previous restart flags if any
	// and add the new one.
	var newArgs []string
	// Filter old flag
	// Note: os.Args[0] is the program name
	if len(os.Args) > 0 {
		newArgs = append(newArgs, os.Args[0])
	}

	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--resume-state" {
			i++ // skip value
			continue
		}
		if strings.HasPrefix(os.Args[i], "--resume-state=") {
			continue
		}
		newArgs = append(newArgs, os.Args[i])
	}
	newArgs = append(newArgs, "--resume-state", tmpState.Name())

	// Exec replaces the process
	syscall.Exec(exe, newArgs, os.Environ())
}
