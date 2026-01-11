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

	// Initialize vault
	v, _ := vault.New("vibeauracle")

	// Initialize model provider based on config
	configMap := map[string]string{
		"endpoint": cfg.Model.Endpoint,
		"model":    cfg.Model.Name,
		"api_key":  cfg.Model.Endpoint, // Simplified for now
	}

	// Try to get secrets from vault
	if v != nil {
		if token, err := v.Get("github_models_pat"); err == nil {
			configMap["token"] = token
		}
		if key, err := v.Get("openai_api_key"); err == nil {
			configMap["api_key"] = key
		}
	}

	p, _ := model.GetProvider(cfg.Model.Provider, configMap)

	fs := sys.NewLocalFS("")
	registry := tooling.NewRegistry()
	registry.Register(tooling.NewTraversalTool(fs))

	return &Brain{
		model:    model.New(p),
		monitor:  sys.NewMonitor(),
		fs:       fs,
		config:   cfg,
		auth:     auth.NewHandler(),
		vault:    v,
		memory:   vcontext.NewMemory(),
		tools:    registry,
		security: tooling.NewSecurityGuard(),
		sessions: make(map[string]*tooling.Session),
	}
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

