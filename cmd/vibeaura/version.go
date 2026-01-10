package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of vibeaura",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("vibeaura version %s\n", Version)
		if Commit != "none" {
			fmt.Printf("commit: %s\n", Commit)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

