package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vibe",
	Short: "vibeauracle - Distributed, System-Intimate AI Engineering Ecosystem",
	Long: `vibeauracle is a keyboard-centric interface that unifies the terminal, 
the IDE, and the AI assistant into a single system-aware experience.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Welcome to vibeauracle! Use 'vibe chat' to start the TUI.")
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

