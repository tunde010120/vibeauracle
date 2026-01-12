package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/nathfavour/vibeauracle/internal/doctor"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathfavour/vibeauracle/brain"
	"github.com/nathfavour/vibeauracle/tooling"
	"github.com/spf13/cobra"
)

var (
	Version         = "dev"
	Commit          = "none"
	BuildDate       = "unknown"
	resumeStateFile string // For hot-swap restoration
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
	Short:   "vibe auracle - Distributed, System-Intimate AI Engineering Ecosystem",
	Long: `vibe auracle is a keyboard-centric interface that unifies the terminal, 
the IDE, and the AI assistant into a single system-aware experience.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Ensure the tool is installed in a standard system directory
		ensureInstalled()

		// Only check for updates on the root command or major interactive commands,
		// and skip for the 'update' command itself to avoid double checks.
		if cmd.CommandPath() != "vibeaura update" && cmd.CommandPath() != "vibeaura completion" && cmd.CommandPath() != "vibeaura rollback" {
			checkUpdateSilent()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		b := brain.New()

		// Inject Status Reporting into Tooling
		tooling.StatusReporter = func(icon, step, msg string) {
			doctor.Send("tooling", doctor.SignalInit, fmt.Sprintf("%s %s", step, msg), nil)
			select {
			case StatusStream <- StatusEvent{Icon: icon, Step: step, Message: msg}:
			default:
				// Drop if buffer full
			}
		}

		// Ensure we are in an interactive terminal
		p := tea.NewProgram(initialModel(b), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			doctor.Send("tui", doctor.SignalError, err.Error(), nil)
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
			printError(err.Error())
			os.Exit(1)
		}
		printSuccess("GitHub Models PAT stored in secure vault.")
	},
}

var authOllamaCmd = &cobra.Command{
	Use:   "ollama <endpoint>",
	Short: "Configure Ollama endpoint",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		endpoint := args[0]
		b := brain.New()
		cfg := b.Config()
		cfg.Model.Endpoint = endpoint
		if err := b.UpdateConfig(cfg); err != nil {
			printError(err.Error())
			os.Exit(1)
		}
		printSuccess("Ollama endpoint set to: " + endpoint)
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
			printError(err.Error())
			os.Exit(1)
		}
		printSuccess("OpenAI API key stored in secure vault.")
	},
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Discover and manage AI models",
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all models from active providers",
	Run: func(cmd *cobra.Command, args []string) {
		b := brain.New()
		printInfo("Discovering models...")
		discoveries, err := b.DiscoverModels(cmd.Context())
		if err != nil {
			printError(err.Error())
			os.Exit(1)
		}

		if len(discoveries) == 0 {
			printWarning("No models found. Use 'vibeaura auth' to configure providers.")
			return
		}

		printTitle("✨", "AVAILABLE MODELS")
		for _, d := range discoveries {
			displayName := brain.ShortenModelName(d.Name)
			printBulletWithMeta(fmt.Sprintf("%-30s", displayName), fmt.Sprintf("%s: %s", d.Provider, d.Name))
		}
		printNewline()
		printCommand("💡 Use", "vibeaura models use <provider> <model>", "to switch.")
	},
}

var modelsUseCmd = &cobra.Command{
	Use:   "use <provider> <model>",
	Short: "Switch the active model",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		provider := args[0]
		modelName := args[1]
		b := brain.New()
		err := b.SetModel(provider, modelName)
		if err != nil {
			printError(err.Error())
			os.Exit(1)
		}
		printStatus("SWITCHED", modelName+" via "+provider)
	},
}

var sysCmd = &cobra.Command{
	Use:   "sys",
	Short: "System and hardware intimacy controls",
}

var sysStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show system resource usage",
	Run: func(cmd *cobra.Command, args []string) {
		b := brain.New()
		snapshot, _ := b.GetSnapshot()
		printTitle("⚡", "POWER SNAPSHOT")
		printKeyValueHighlight("CPU Usage", fmt.Sprintf("%.1f%%", snapshot.CPUUsage))
		printKeyValueHighlight("Mem Usage", fmt.Sprintf("%.1f%%", snapshot.MemoryUsage))
		printKeyValue("CWD      ", snapshot.WorkingDir)
		printNewline()
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the vibeaura application",
	Run: func(cmd *cobra.Command, args []string) {
		printInfo("Restarting vibeaura...")
		restartSelf()
	},
}

func main() {
	// Install colorized output for Cobra (affects --help, usage, errors)
	rootCmd.SetOut(NewColorWriter(os.Stdout))
	rootCmd.SetErr(NewColorWriter(os.Stderr))

	rootCmd.PersistentFlags().StringVar(&resumeStateFile, "resume-state", "", "Internal use: resume state from file")
	rootCmd.PersistentFlags().MarkHidden("resume-state")

	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authGithubCmd)
	authCmd.AddCommand(authOllamaCmd)
	authCmd.AddCommand(authOpenAICmd)

	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsUseCmd)

	rootCmd.AddCommand(sysCmd)
	sysCmd.AddCommand(sysStatsCmd)

	rootCmd.AddCommand(restartCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
