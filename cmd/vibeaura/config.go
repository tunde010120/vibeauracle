package main

import (
	"fmt"
	"strconv"

	"github.com/nathfavour/vibeauracle/sys"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "View or update configuration settings",
	Long: `View or update configuration settings for vibeauracle.
If no arguments are provided, it lists all current settings.
If only a key is provided, it shows the current value for that key.
If both key and value are provided, it updates the setting.

Keys:
  update.beta             Enable/disable beta updates (build from master)
  update.build_from_source  Enable/disable building from source for all updates
  model.provider          AI provider (ollama, openai)
  model.name              AI model name
  model.endpoint          AI provider endpoint`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cm, err := sys.NewConfigManager()
		if err != nil {
			return fmt.Errorf("initializing config: %w", err)
		}
		cfg, err := cm.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if len(args) == 0 {
			fmt.Printf("Current Configuration:\n")
			fmt.Printf("  update.beta:             %v\n", cfg.Update.Beta)
			fmt.Printf("  update.build_from_source: %v\n", cfg.Update.BuildFromSource)
			fmt.Printf("  model.provider:          %s\n", cfg.Model.Provider)
			fmt.Printf("  model.name:              %s\n", cfg.Model.Name)
			fmt.Printf("  model.endpoint:          %s\n", cfg.Model.Endpoint)
			fmt.Printf("  ui.theme:                %s\n", cfg.UI.Theme)
			return nil
		}

		key := args[0]
		if len(args) == 1 {
			switch key {
			case "update.beta":
				fmt.Println(cfg.Update.Beta)
			case "update.build_from_source":
				fmt.Println(cfg.Update.BuildFromSource)
			case "model.provider":
				fmt.Println(cfg.Model.Provider)
			case "model.name":
				fmt.Println(cfg.Model.Name)
			case "model.endpoint":
				fmt.Println(cfg.Model.Endpoint)
			case "ui.theme":
				fmt.Println(cfg.UI.Theme)
			default:
				return fmt.Errorf("unknown config key: %s", key)
			}
			return nil
		}

		value := args[1]
		switch key {
		case "update.beta":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean value for %s: %s", key, value)
			}
			cfg.Update.Beta = b
		case "update.build_from_source":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean value for %s: %s", key, value)
			}
			cfg.Update.BuildFromSource = b
		case "model.provider":
			cfg.Model.Provider = value
		case "model.name":
			cfg.Model.Name = value
		case "model.endpoint":
			cfg.Model.Endpoint = value
		case "ui.theme":
			cfg.UI.Theme = value
		default:
			return fmt.Errorf("unknown config key: %s", key)
		}

		if err := cm.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("✅ Config set %s = %s\n", key, value)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

