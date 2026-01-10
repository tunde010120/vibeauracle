package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathfavour/vibeauracle/brain"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
)

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
		p := tea.NewProgram(initialModel(b))
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

