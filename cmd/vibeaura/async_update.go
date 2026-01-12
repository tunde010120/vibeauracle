package main

import (
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathfavour/vibeauracle/sys"
)

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
				var latest *releaseInfo
				var err error

				if cfg.Update.BuildFromSource {
					// For source builds, we check the branch commit directly
					branch := "release"
					if cfg.Update.Beta {
						branch = "master"
					}
					sha, err := getBranchCommitSHA(branch)
					if err == nil {
						latest = &releaseInfo{
							TagName:   branch,
							ActualSHA: sha,
						}
					}
				} else {
					// Binary updates
					latest, err = getLatestRelease("")
					if cfg.Update.Beta {
						latest, err = getLatestRelease("beta")
					}
				}

				if err == nil && latest != nil && isUpdateAvailable(latest, !manual) {
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
