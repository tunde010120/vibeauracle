package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print detailed version information",
	Run: func(cmd *cobra.Command, args []string) {
		printTitle("✨", "VIBE AURACLE")
		printKeyValueHighlight("Version  ", Version)
		printKeyValue("Commit   ", Commit)
		printKeyValue("Built    ", BuildDate)
		printKeyValue("Platform ", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
		printKeyValue("Compiler ", runtime.Version())
		printNewline()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
