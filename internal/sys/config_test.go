package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager(t *testing.T) {
	// Temporarily redirect home for testing
	tmpHome, err := os.MkdirTemp("", "vibeaura-config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)
	
	// We'll mock the home directory by setting the HOME environment variable
	// Note: In some OSs/Go versions, UserHomeDir might not respect $HOME,
	// but for this test we'll manually check the directory structure.
	
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	cm, err := NewConfigManager()
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	cfg, err := cm.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify defaults
	if cfg.Model.Provider != "ollama" {
		t.Errorf("got provider %q, want 'ollama'", cfg.Model.Provider)
	}

	// Verify file existence
	dataDir := filepath.Join(tmpHome, ".vibeauracle")
	configPath := filepath.Join(dataDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Test Save/Update
	cfg.Model.Name = "custom-model"
	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify
	cm2, _ := NewConfigManager()
	cfg2, _ := cm2.Load()
	if cfg2.Model.Name != "custom-model" {
		t.Errorf("got model name %q, want 'custom-model'", cfg2.Model.Name)
	}
}

