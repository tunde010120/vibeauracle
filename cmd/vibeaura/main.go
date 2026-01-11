package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathfavour/vibeauracle/brain"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func init() {
	// Try to populate Version and Commit from build info if they are defaults
	if info, ok := debug.ReadBuildInfo(); ok {
		// If Version is still the default "dev", try to get it from the build info (e.g. go install)
		if Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if Commit == "none" {
					Commit = setting.Value
				}
			case "vcs.time":
				if BuildDate == "unknown" {
					BuildDate = setting.Value
				}
			}
		}
	}

	// If we're still in "dev" mode, try to find the current git branch
	if Version == "dev" {
		// Only try this if we are in a git repo
		if _, err := os.Stat(".git"); err == nil {
			branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			if branchBytes, err := branchCmd.Output(); err == nil {
				Version = "dev-" + strings.TrimSpace(string(branchBytes))
			}
		}
	}
}

var rootCmd = &cobra.Command{
	Use:     "vibeaura",
	Version: Version,
	Short:   "vibeauracle - Distributed, System-Intimate AI Engineering Ecosystem",
	Long: `vibeauracle is a keyboard-centric interface that unifies the terminal, 
the IDE, and the AI assistant into a single system-aware experience.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Only check for updates on the root command or major interactive commands,
		// and skip for the 'update' command itself to avoid double checks.
		if cmd.CommandPath() != "vibeaura update" && cmd.CommandPath() != "vibeaura completion" {
			checkUpdateSilent()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		b := brain.New()
		
		// Ensure we are in an interactive terminal
		p := tea.NewProgram(initialModel(b), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	},
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage AI provider credentials",
	Long:  "Securely store and manage API keys for providers like GitHub Models, OpenAI, and Ollama.",
}

var authGithubCmd = &cobra.Command{
	Use:   "github-models <token>",
	Short: "Configure GitHub Models PAT",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		token := args[0]
		b := brain.New()
		err := b.StoreSecret("github_models_pat", token)
		if err != nil {
			fmt.Printf("Error storing secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("GitHub Models PAT stored successfully in secure vault.")
	},
}

var authOpenAICmd = &cobra.Command{
	Use:   "openai <api-key>",
	Short: "Configure OpenAI API key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		b := brain.New()
		err := b.StoreSecret("openai_api_key", key)
		if err != nil {
			fmt.Printf("Error storing secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OpenAI API key stored successfully in secure vault.")
	},
}

func main() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authGithubCmd)
	authCmd.AddCommand(authOpenAICmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

