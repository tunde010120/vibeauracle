package sys

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for vibeauracle
type Config struct {
	Model struct {
		Provider string `mapstructure:"provider"`
		Endpoint string `mapstructure:"endpoint"`
		Name     string `mapstructure:"name"`
	} `mapstructure:"model"`

	Prompt struct {
		Enabled                    bool    `mapstructure:"enabled"`
		Mode                       string  `mapstructure:"mode"` // auto|ask|plan|crud
		ProjectInstructions         string  `mapstructure:"project_instructions"`
		LearningEnabled             bool    `mapstructure:"learning_enabled"`
		RecommendationsEnabled      bool    `mapstructure:"recommendations_enabled"`
		RecommendationsSampleRate   float64 `mapstructure:"recommendations_sample_rate"`
		RecommendationsMaxPerRun    int     `mapstructure:"recommendations_max_per_run"`
	} `mapstructure:"prompt"`

	Update struct {
		BuildFromSource bool     `mapstructure:"build_from_source"`
		Beta            bool     `mapstructure:"beta"`
		AutoUpdate      bool     `mapstructure:"auto_update"`
		Verbose         bool     `mapstructure:"verbose"`
		FailedCommits   []string `mapstructure:"failed_commits"`
	} `mapstructure:"update"`

	UI struct {
		Theme         string `mapstructure:"theme"`
		ScreenshotDir string `mapstructure:"screenshot_dir"`
	} `mapstructure:"ui"`

	DataDir string `mapstructure:"-"`
}

// ConfigManager handles loading and saving configuration
type ConfigManager struct {
	v *viper.Viper
}

// NewConfigManager initializes the configuration system
func NewConfigManager() (*ConfigManager, error) {
	v := viper.New()
	
	// Determine the home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting user home dir: %w", err)
	}
	
	dataDir := filepath.Join(home, ".vibeauracle")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}
	
	// Default configuration
	v.SetDefault("model.provider", "ollama")
	v.SetDefault("model.endpoint", "http://localhost:11434")
	v.SetDefault("model.name", "llama3")
	v.SetDefault("ui.theme", "dark")

	// Prompt system defaults
	v.SetDefault("prompt.enabled", true)
	v.SetDefault("prompt.mode", "auto")
	v.SetDefault("prompt.project_instructions", "")
	v.SetDefault("prompt.learning_enabled", true)
	// Keep recommendations extremely conservative by default to avoid cost.
	v.SetDefault("prompt.recommendations_enabled", false)
	v.SetDefault("prompt.recommendations_sample_rate", 0.02)
	v.SetDefault("prompt.recommendations_max_per_run", 1)
	
	// Platform-specific screenshot directory
	var defaultShotDir string
	if _, err := os.Stat("/data/data/com.termux/files/usr/bin/bash"); err == nil {
		defaultShotDir = filepath.Join(home, "downloads", "vibeaura")
	} else {
		defaultShotDir = filepath.Join(home, "Downloads", "vibeaura")
	}
	v.SetDefault("ui.screenshot_dir", defaultShotDir)

	v.SetDefault("update.build_from_source", false)
	v.SetDefault("update.beta", false)
	v.SetDefault("update.auto_update", true)
	v.SetDefault("update.verbose", false)
	v.SetDefault("update.failed_commits", []string{})
	
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(dataDir)
	
	// Create config file if it doesn't exist
	configPath := filepath.Join(dataDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := v.SafeWriteConfig(); err != nil {
			return nil, fmt.Errorf("writing initial config: %w", err)
		}
	}
	
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	
	return &ConfigManager{v: v}, nil
}

// Get returns the current configuration
func (cm *ConfigManager) Load() (*Config, error) {
	var cfg Config
	if err := cm.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	
	home, _ := os.UserHomeDir()
	cfg.DataDir = filepath.Join(home, ".vibeauracle")
	
	return &cfg, nil
}

// Save persists the current configuration
func (cm *ConfigManager) Save(cfg *Config) error {
	cm.v.Set("model.provider", cfg.Model.Provider)
	cm.v.Set("model.endpoint", cfg.Model.Endpoint)
	cm.v.Set("model.name", cfg.Model.Name)
	cm.v.Set("prompt.enabled", cfg.Prompt.Enabled)
	cm.v.Set("prompt.mode", cfg.Prompt.Mode)
	cm.v.Set("prompt.project_instructions", cfg.Prompt.ProjectInstructions)
	cm.v.Set("prompt.learning_enabled", cfg.Prompt.LearningEnabled)
	cm.v.Set("prompt.recommendations_enabled", cfg.Prompt.RecommendationsEnabled)
	cm.v.Set("prompt.recommendations_sample_rate", cfg.Prompt.RecommendationsSampleRate)
	cm.v.Set("prompt.recommendations_max_per_run", cfg.Prompt.RecommendationsMaxPerRun)
	cm.v.Set("update.build_from_source", cfg.Update.BuildFromSource)
	cm.v.Set("update.beta", cfg.Update.Beta)
	cm.v.Set("update.auto_update", cfg.Update.AutoUpdate)
	cm.v.Set("update.verbose", cfg.Update.Verbose)
	cm.v.Set("update.failed_commits", cfg.Update.FailedCommits)
	cm.v.Set("ui.theme", cfg.UI.Theme)
	cm.v.Set("ui.screenshot_dir", cfg.UI.ScreenshotDir)
	
	return cm.v.WriteConfig()
}

// GetDataPath returns a path inside the .vibeauracle directory
func (cm *ConfigManager) GetDataPath(subpath string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vibeauracle", subpath)
}

