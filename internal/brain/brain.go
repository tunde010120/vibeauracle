package brain

import (
	"context"
	"fmt"
	"strings"

	"github.com/nathfavour/vibeauracle/auth"
	vcontext "github.com/nathfavour/vibeauracle/context"
	"github.com/nathfavour/vibeauracle/model"
	"github.com/nathfavour/vibeauracle/sys"
	"github.com/nathfavour/vibeauracle/tooling"
	"github.com/nathfavour/vibeauracle/vault"
)

// Request represents a user request or system trigger
type Request struct {
	ID      string
	Content string
}

// Response represents the brain's output
type Response struct {
	Content string
	Error   error
}

// Brain is the cognitive orchestrator
type Brain struct {
	model    *model.Model
	monitor  *sys.Monitor
	fs       sys.FS
	config   *sys.Config
	cm       *sys.ConfigManager
	auth     *auth.Handler
	vault    *vault.Vault
	memory   *vcontext.Memory
	tools    *tooling.Registry
	security *tooling.SecurityGuard
	sessions map[string]*tooling.Session
}

func New() *Brain {
	// Initialize config
	cm, _ := sys.NewConfigManager()
	cfg, _ := cm.Load()

	// Initialize vault with data directory fallback
	v, _ := vault.New("vibeauracle", cfg.DataDir)

	b := &Brain{
		monitor:  sys.NewMonitor(),
		config:   cfg,
		cm:       cm,
		auth:     auth.NewHandler(),
		vault:    v,
		memory:   vcontext.NewMemory(),
		security: tooling.NewSecurityGuard(),
		sessions: make(map[string]*tooling.Session),
	}

	// Initialize model provider based on config
	b.initProvider()

	fs := sys.NewLocalFS("")
	registry := tooling.NewRegistry()
	registry.Register(tooling.NewTraversalTool(fs))
	b.fs = fs
	b.tools = registry

	return b
}

func (b *Brain) initProvider() {
	configMap := map[string]string{
		"endpoint": b.config.Model.Endpoint,
		"model":    b.config.Model.Name,
	}

	// Fetch credentials from vault
	if b.vault != nil {
		if token, err := b.vault.Get("github_models_pat"); err == nil {
			configMap["token"] = token
		}
		if key, err := b.vault.Get("openai_api_key"); err == nil {
			configMap["api_key"] = key
		}
	}

	p, err := model.GetProvider(b.config.Model.Provider, configMap)
	if err != nil {
		// Fallback or log error
		fmt.Printf("Error initializing provider %s: %v\n", b.config.Model.Provider, err)
	}
	b.model = model.New(p)
}

// ModelDiscovery represents a discovered model with its provider
type ModelDiscovery struct {
	Name     string
	Provider string
}

// DiscoverModels fetches available models from all configured providers
func (b *Brain) DiscoverModels(ctx context.Context) ([]ModelDiscovery, error) {
	var discoveries []ModelDiscovery

	// List of potential providers to check
	providersToCheck := []string{"ollama", "openai", "github-models"}

	for _, pName := range providersToCheck {
		configMap := map[string]string{
			"endpoint": b.config.Model.Endpoint,
		}

		// Hydrate with credentials
		if b.vault != nil {
			switch pName {
			case "github-models":
				if token, err := b.vault.Get("github_models_pat"); err == nil {
					configMap["token"] = token
				} else {
					continue // No token, skip
				}
			case "openai":
				if key, err := b.vault.Get("openai_api_key"); err == nil {
					configMap["api_key"] = key
				} else {
					continue // No key, skip
				}
			case "ollama":
				// Usually no auth needed for local ollama
			}
		}

		p, err := model.GetProvider(pName, configMap)
		if err != nil {
			continue
		}

		models, err := p.ListModels(ctx)
		if err != nil {
			continue
		}

		for _, m := range models {
			discoveries = append(discoveries, ModelDiscovery{
				Name:     m,
				Provider: pName,
			})
		}
	}

	return discoveries, nil
}

// SetModel updates the active model and provider
func (b *Brain) SetModel(provider, name string) error {
	b.config.Model.Provider = provider
	b.config.Model.Name = name
	
	// If provider is ollama, we might need to handle endpoint too, 
	// but for now we keep the existing one or reset to default if changed.
	if provider == "ollama" && b.config.Model.Endpoint == "" {
		b.config.Model.Endpoint = "http://localhost:11434"
	}

	if err := b.cm.Save(b.config); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	b.initProvider()
	return nil
}

// Process handles the "Plan-Execute-Reflect" loop
func (b *Brain) Process(ctx context.Context, req Request) (Response, error) {
	// 1. Session & Thread Management
	sessionID := "default" // In a real app, this would come from the request
	session, ok := b.sessions[sessionID]
	if !ok {
		session = tooling.NewSession(sessionID)
		b.sessions[sessionID] = session
	}

	// 2. Perceive: Receive request + SystemSnapshot
	snapshot, _ := b.monitor.GetSnapshot()
	
	// 3. Recall (RAG/Context)
	snippets, _ := b.memory.Recall(req.Content)
	contextStr := strings.Join(snippets, "\n")

	// 4. Tool Awareness
	toolDefs := b.tools.GetPromptDefinitions()

	// 5. Plan & Execute via Model
	augmentedPrompt := fmt.Sprintf(`System Context:
%s

System CWD: %s
Available Tools:
%s

User Request (Thread ID: %s):
%s`, contextStr, snapshot.WorkingDir, toolDefs, req.ID, req.Content)
	
	// Pre-execution Security Check (Simplified for example)
	if strings.Contains(req.Content, ".env") {
		if err := b.security.CheckPath(".env"); err != nil {
			return Response{Content: fmt.Sprintf("Security Alert: %v. You must explicitly enable sensitive file access.", err)}, nil
		}
	}

	resp, err := b.model.Generate(ctx, augmentedPrompt)
	if err != nil {
		return Response{}, fmt.Errorf("generating response: %w", err)
	}
	
	// 6. Record interaction in Session
	session.AddThread(&tooling.Thread{
		ID:       req.ID,
		Prompt:   req.Content,
		Response: resp,
	})

	// Store result in memory
	_ = b.memory.Store(req.ID, resp)
	
	return Response{
		Content: resp,
	}, nil
}

// StoreState persists application state
func (b *Brain) StoreState(id string, state interface{}) error {
	return b.memory.SaveState(id, state)
}

// RecallState retrieves application state
func (b *Brain) RecallState(id string, target interface{}) error {
	return b.memory.LoadState(id, target)
}

// ClearState removes application state
func (b *Brain) ClearState(id string) error {
	return b.memory.ClearState(id)
}

// GetConfig returns the brain's configuration
func (b *Brain) GetConfig() *sys.Config {
	return b.config
}

// Config is an alias for GetConfig
func (b *Brain) Config() *sys.Config {
	return b.config
}

// UpdateConfig updates the brain's configuration and persists it
func (b *Brain) UpdateConfig(cfg *sys.Config) error {
	b.config = cfg
	if err := b.cm.Save(b.config); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	b.initProvider()
	return nil
}

// GetSnapshot returns a current snapshot of system resources via the monitor
func (b *Brain) GetSnapshot() (sys.Snapshot, error) {
	return b.monitor.GetSnapshot()
}

// StoreSecret saves a secret in the vault
func (b *Brain) StoreSecret(key, value string) error {
	if b.vault == nil {
		return fmt.Errorf("vault not initialized")
	}
	return b.vault.Set(key, value)
}

// GetSecret retrieves a secret from the vault
func (b *Brain) GetSecret(key string) (string, error) {
	if b.vault == nil {
		return "", fmt.Errorf("vault not initialized")
	}
	return b.vault.Get(key)
}

