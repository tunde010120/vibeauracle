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
		fmt.Println()
		fmt.Println(cliTitle.Render("✨ VIBEAURACLE"))
		fmt.Println(cliMuted.Render("─────────────────────────────────────────────"))
		fmt.Printf("%s %s\n", cliLabel.Render("Version:  "), cliHighlight.Render(Version))
		fmt.Printf("%s %s\n", cliLabel.Render("Commit:   "), cliValue.Render(Commit))
		fmt.Printf("%s %s\n", cliLabel.Render("Built:    "), cliValue.Render(BuildDate))
		fmt.Printf("%s %s\n", cliLabel.Render("Platform: "), cliSubtitle.Render(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)))
		fmt.Printf("%s %s\n", cliLabel.Render("Compiler: "), cliMuted.Render(runtime.Version()))
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
