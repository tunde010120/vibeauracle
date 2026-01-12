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
  update.auto_update      Enable/disable automatic updates (default: true)
  update.verbose          Show detailed output during updates (default: false)
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
			printTitle("⚙️", "CONFIGURATION")
			printKeyValue("update.beta            ", fmt.Sprintf("%v", cfg.Update.Beta))
			printKeyValue("update.build_from_source", fmt.Sprintf("%v", cfg.Update.BuildFromSource))
			printKeyValueHighlight("update.auto_update     ", fmt.Sprintf("%v", cfg.Update.AutoUpdate))
			printKeyValue("update.verbose         ", fmt.Sprintf("%v", cfg.Update.Verbose))
			printKeyValue("model.provider         ", cfg.Model.Provider)
			printKeyValueHighlight("model.name             ", cfg.Model.Name)
			printKeyValue("model.endpoint         ", cfg.Model.Endpoint)
			printKeyValue("ui.theme               ", cfg.UI.Theme)
			printNewline()
			return nil
		}

		key := args[0]
		if len(args) == 1 {
			switch key {
			case "update.beta":
				fmt.Println(cfg.Update.Beta)
			case "update.build_from_source":
				fmt.Println(cfg.Update.BuildFromSource)
			case "update.auto_update":
				fmt.Println(cfg.Update.AutoUpdate)
			case "update.verbose":
				fmt.Println(cfg.Update.Verbose)
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
		case "update.auto_update":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean value for %s: %s", key, value)
			}
			cfg.Update.AutoUpdate = b
		case "update.verbose":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean value for %s: %s", key, value)
			}
			cfg.Update.Verbose = b
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

		printStatus("SET", key+" → "+value)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
