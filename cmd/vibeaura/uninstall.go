package main

import (
	"fmt"
	"os"

	"github.com/nathfavour/vibeauracle/sys"
	"github.com/spf13/cobra"
)

var cleanUninstall bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove vibeaura from your system",
	Long: `Uninstall the vibeaura binary. 
By default, the application data directory (~/.vibeauracle) is preserved. 
Use the --clean flag to wipe everything.`,
	Run: func(cmd *cobra.Command, args []string) {
		printTitle("🗑️", "UNINSTALL VIBE AURACLE")

		// 1. Get binary path
		exePath, err := os.Executable()
		if err != nil {
			printError("Could not determine binary path: " + err.Error())
			return
		}

		// 2. Get data directory
		cm, err := sys.NewConfigManager()
		var dataDir string
		if err == nil {
			cfg, err := cm.Load()
			if err == nil {
				dataDir = cfg.DataDir
			}
		} else {
			// Fallback if config manager fails
			if home, err := os.UserHomeDir(); err == nil {
				dataDir = fmt.Sprintf("%s/.vibeauracle", home)
			}
		}

		// 3. Remove binary
		printInfo("Removing binary: " + exePath)
		if err := os.Remove(exePath); err != nil {
			printError("Failed to remove binary: " + err.Error())
			// We continue to data wiping even if binary removal fails (e.g. permission issues)
		} else {
			printBullet("Binary removed successfully")
		}

		// 4. Clean data if requested
		if cleanUninstall && dataDir != "" {
			if _, err := os.Stat(dataDir); err == nil {
				printInfo("Wiping data directory: " + dataDir)
				if err := os.RemoveAll(dataDir); err != nil {
					printError("Failed to wipe data: " + err.Error())
				} else {
					printBullet("Application data wiped successfully")
				}
			} else {
				printInfo("Data directory not found, skipping wipe.")
			}
		} else if dataDir != "" {
			printInfo("Keeping application data at: " + dataDir)
		}

		printDone()
		printNewline()
		fmt.Println(cliMuted.Render("Note: If you established any shells integrations manually, you may need to remove them from your shell profile."))
	},
}

func init() {
	uninstallCmd.Flags().BoolVar(&cleanUninstall, "clean", false, "Wipe both binary and the entire data directory")
	rootCmd.AddCommand(uninstallCmd)
}
