package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathfavour/vibeauracle/brain"
	"github.com/nathfavour/vibeauracle/tooling"
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
			select {
			case StatusStream <- StatusEvent{Icon: icon, Step: step, Message: msg}:
			default:
				// Drop if buffer full
			}
		}

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
			fmt.Printf("\033[31mError storing secret: %v\033[0m\n", err)
			os.Exit(1)
		}
		fmt.Println("\033[32mGitHub Models PAT stored successfully in secure vault.\033[0m")
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
			fmt.Printf("\033[31mError updating endpoint: %v\033[0m\n", err)
			os.Exit(1)
		}
		fmt.Printf("\033[32mOllama endpoint set to: %s\033[0m\n", endpoint)
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
			fmt.Printf("\033[31mError storing secret: %v\033[0m\n", err)
			os.Exit(1)
		}
		fmt.Println("\033[32mOpenAI API key stored successfully in secure vault.\033[0m")
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
		fmt.Println("\033[35mDISCOVERING MODELS...\033[0m")
		discoveries, err := b.DiscoverModels(cmd.Context())
		if err != nil {
			fmt.Printf("\033[31mError discovering models: %v\033[0m\n", err)
			os.Exit(1)
		}

		if len(discoveries) == 0 {
			fmt.Println("\033[33mNo models found. Use 'auth' to configure providers.\033[0m")
			return
		}

		fmt.Println("\033[1;36mAVAILABLE MODELS:\033[0m")
		for _, d := range discoveries {
			displayName := brain.ShortenModelName(d.Name)
			fmt.Printf("\033[32m•\033[0m \033[1m%-30s\033[0m \033[90m(%s: %s)\033[0m\n", displayName, d.Provider, d.Name)
		}
		fmt.Println("\n\033[34mUse 'models use <provider> <model>' to switch.\033[0m")
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
			fmt.Printf("\033[31mError switching model: %v\033[0m\n", err)
			os.Exit(1)
		}
		fmt.Printf("\033[32mSuccessfully switched to \033[1m%s\033[0m \033[32mvia %s\033[0m\n", modelName, provider)
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
		fmt.Printf("\033[1;36mPOWER SNAPSHOT\033[0m\n")
		fmt.Printf("\033[32mCPU Usage:\033[0m %.1f%%\n", snapshot.CPUUsage)
		fmt.Printf("\033[32mMem Usage:\033[0m %.1f%%\n", snapshot.MemoryUsage)
		fmt.Printf("\033[32mCWD:\033[0m       %s\n", snapshot.WorkingDir)
	},
}

func main() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authGithubCmd)
	authCmd.AddCommand(authOllamaCmd)
	authCmd.AddCommand(authOpenAICmd)

	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsUseCmd)

	rootCmd.AddCommand(sysCmd)
	sysCmd.AddCommand(sysStatsCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
