package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/99designs/keyring"
)

// Vault handles secure credential storage
type Vault struct {
	ring         keyring.Keyring
	fallbackPath string
	mu           sync.RWMutex
}

func New(serviceName string, dataDir string) (*Vault, error) {
	v := &Vault{
		fallbackPath: filepath.Join(dataDir, "secrets.json"),
	}

	ring, err := keyring.Open(keyring.Config{
		ServiceName: serviceName,
	})
	if err == nil {
		v.ring = ring
	}
	// If keyring fails, we just don't set v.ring and use fallbackPath
	return v, nil
}

// Set stores a secret in the OS keyring or fallback file
func (v *Vault) Set(key, value string) error {
	if v.ring != nil {
		err := v.ring.Set(keyring.Item{
			Key:  key,
			Data: []byte(value),
		})
		if err == nil {
			return nil
		}
		// If keyring set fails, fall through to file fallback
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	secrets := make(map[string]string)
	if data, err := os.ReadFile(v.fallbackPath); err == nil {
		json.Unmarshal(data, &secrets)
	}

	secrets[key] = value
	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling secrets: %w", err)
	}

	return os.WriteFile(v.fallbackPath, data, 0600)
}

// Get retrieves a secret from the OS keyring or fallback file
func (v *Vault) Get(key string) (string, error) {
	if v.ring != nil {
		item, err := v.ring.Get(key)
		if err == nil {
			return string(item.Data), nil
		}
		// If keyring get fails (e.g. not found), check fallback
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	if data, err := os.ReadFile(v.fallbackPath); err == nil {
		secrets := make(map[string]string)
		if err := json.Unmarshal(data, &secrets); err == nil {
			if val, ok := secrets[key]; ok {
				return val, nil
			}
		}
	}

	return "", fmt.Errorf("secret not found in vault or fallback")
}

