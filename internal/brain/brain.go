package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathfavour/vibeauracle/auth"
	vcontext "github.com/nathfavour/vibeauracle/context"
	"github.com/nathfavour/vibeauracle/model"
	"github.com/nathfavour/vibeauracle/prompt"
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
	prompts  *prompt.System
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

	// Initialize Security
	guard := tooling.NewSecurityGuard()

	// Create Enclave (Hardware-Intimate Agentic Security)
	enclaveDir := cfg.DataDir
	if enclaveDir == "" {
		home, _ := os.UserHomeDir()
		enclaveDir = filepath.Join(home, ".vibeauracle")
	}

	enclave, err := tooling.NewEnclave(enclaveDir)
	if err == nil {
		guard.SetInterceptor(enclave.Interceptor)
	}

	b := &Brain{
		monitor:  sys.NewMonitor(),
		config:   cfg,
		cm:       cm,
		auth:     auth.NewHandler(),
		vault:    v,
		memory:   vcontext.NewMemory(),
		security: guard,
		sessions: make(map[string]*tooling.Session),
	}

	// Prompt system is modular and configurable.
	b.prompts = prompt.New(cfg, b.memory, &prompt.NoopRecommender{})

	b.initProvider()

	// Proactive Autofix: If the configured model is missing or it's the first run,
	// try to autodetect what's available on the system.
	go b.autodetectBestModel()

	b.fs = sys.NewLocalFS("")
	b.tools = tooling.Setup(b.fs, b.monitor, b.security)

	return b
}

func (b *Brain) initProvider() {
	configMap := map[string]string{
		"endpoint": b.config.Model.Endpoint,
		"model":    b.config.Model.Name,
		"base_url": b.config.Model.Endpoint, // Map endpoint to base_url for OpenAI/Others
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

	// Update the prompt system's recommender to use the newly initialized model.
	if b.prompts != nil {
		b.prompts.SetRecommender(prompt.NewModelRecommender(b.model))
	}
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
			"base_url": b.config.Model.Endpoint,
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
	tooling.ReportStatus("🧠", "think", "Processing request...")

	// Early check for model
	if b.model == nil {
		tooling.ReportStatus("❌", "error", "No AI model configured")
		return Response{}, fmt.Errorf("no AI model configured. Run 'vibeaura auth' to set up a provider")
	}

	// 1. Session & Thread Management
	sessionID := "default" // In a real app, this would come from the request
	session, ok := b.sessions[sessionID]
	if !ok {
		session = tooling.NewSession(sessionID)
		b.sessions[sessionID] = session
	}

	// 2. Perceive: Receive request + SystemSnapshot
	snapshot, _ := b.monitor.GetSnapshot()
	tooling.ReportStatus("👁️", "perceive", fmt.Sprintf("CWD: %s", snapshot.WorkingDir))

	// 3. Tool Awareness (Smart Handshake)
	toolDefs := b.tools.GetPromptDefinitions(tooling.CoreTools())
	tooling.ReportStatus("🔧", "tools", fmt.Sprintf("Loaded %d core tools", len(tooling.CoreTools())))

	// 4. Update Rolling Context Window
	b.memory.AddToWindow(req.ID, req.Content, "user_prompt")

	// 5. Prompt System: classify + layer instructions + inject recall + build final prompt
	augmentedPrompt := ""
	var recs []prompt.Recommendation
	var promptIntent prompt.Intent

	if b.config.Prompt.Enabled && b.prompts != nil {
		tooling.ReportStatus("📝", "prompt", "Building augmented prompt...")
		env, builtRecs, err := b.prompts.Build(ctx, req.Content, snapshot, toolDefs)
		if err != nil {
			tooling.ReportStatus("❌", "error", fmt.Sprintf("Prompt build failed: %v", err))
			return Response{}, fmt.Errorf("building prompt: %w", err)
		}
		if ignored, ok := env.Metadata["ignored"].(bool); ok && ignored {
			tooling.ReportStatus("⏭️", "skip", "Empty/invalid prompt ignored")
			return Response{Content: "(ignored empty/invalid prompt)"}, nil
		}
		augmentedPrompt = env.Prompt
		recs = builtRecs
		promptIntent = env.Intent
		tooling.ReportStatus("✅", "prompt", fmt.Sprintf("Intent: %s", promptIntent))
	} else {
		// Fallback...
		tooling.ReportStatus("📝", "prompt", "Using fallback prompt builder")
		snippets, _ := b.memory.Recall(req.Content)
		contextStr := strings.Join(snippets, "\n")

		augmentedPrompt = fmt.Sprintf(`System Context:
%s

System CWD: %s
Available Tools (JSON-RPC 2.0 Style):
%s

User Request (Thread ID: %s):
%s`, contextStr, snapshot.WorkingDir, toolDefs, req.ID, req.Content)
	}

	// EXECUTION LOOP (Agentic)
	maxTurns := 5
	history := augmentedPrompt

	for i := 0; i < maxTurns; i++ {
		tooling.ReportStatus("🔄", "loop", fmt.Sprintf("Turn %d/%d: Generating...", i+1, maxTurns))

		// 1. Generate
		resp, err := b.model.Generate(ctx, history)
		if err != nil {
			tooling.ReportStatus("❌", "error", fmt.Sprintf("Model error: %v", err))
			return Response{}, fmt.Errorf("generating response: %w", err)
		}

		// Show first 100 chars of response
		preview := resp
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		tooling.ReportStatus("💬", "response", preview)

		// 2. Parse & Execute Tools
		executed, resultVal, interventionErr, execErr := b.executeToolCalls(ctx, resp)

		// Bubble up intervention immediately so UI can handle it
		if interventionErr != nil {
			tooling.ReportStatus("⚠️", "intervention", "User approval required")
			return Response{}, interventionErr
		}

		if !executed {
			tooling.ReportStatus("✅", "done", "No tool call, returning response")
			// No tool calls? We are done.
			session.AddThread(&tooling.Thread{
				ID:       req.ID,
				Prompt:   req.Content,
				Response: resp,
				Metadata: map[string]interface{}{
					"prompt_intent":    promptIntent,
					"recommendations":  recs,
					"response_raw_len": len(resp),
				},
			})
			_ = b.memory.Store(req.ID, resp)
			return Response{Content: resp}, nil
		}

		// 3. Observation (feed back into history)
		if execErr != nil {
			tooling.ReportStatus("❌", "tool", fmt.Sprintf("Tool error: %v", execErr))
			history += fmt.Sprintf("\n\nUser: Tool Execution Failed: %v\nSystem:", execErr)
		} else {
			resultPreview := resultVal
			if len(resultPreview) > 80 {
				resultPreview = resultPreview[:80] + "..."
			}
			tooling.ReportStatus("✅", "tool", fmt.Sprintf("Result: %s", resultPreview))
			history += fmt.Sprintf("\n\nUser: Tool Output: %s\nSystem:", resultVal)
		}

		// 4. Record intermediate step
		_ = b.memory.Store(req.ID+"_step_"+fmt.Sprint(i), resultVal)
	}

	tooling.ReportStatus("⚠️", "limit", "Agent loop limit reached")
	return Response{Content: "Agent loop limit reached."}, nil
}

// executeToolCalls parses the response for JSON tool invocations and executes them.
func (b *Brain) executeToolCalls(ctx context.Context, input string) (bool, string, error, error) {
	// Simple JSON block parser: Look for ```json { "tool": ... } ```
	start := strings.Index(input, "```json")
	if start == -1 {
		return false, "", nil, nil
	}

	// Find closing block logic
	// We start searching AFTER the "```json" (length 7)
	contentStart := start + 7
	blockContent := input[contentStart:]

	end := strings.Index(blockContent, "```")
	if end == -1 {
		return false, "", nil, nil
	}

	jsonStr := strings.TrimSpace(blockContent[:end])

	// Attempt to parse tool call
	var call struct {
		Tool string          `json:"tool"`
		Args json.RawMessage `json:"parameters"`
	}
	// Try parsing. If it fails, maybe it's not a tool call.
	if err := json.Unmarshal([]byte(jsonStr), &call); err != nil {
		return false, "", nil, nil
	}

	if call.Tool == "" {
		return false, "", nil, nil
	}

	// Found a tool call!
	t, found := b.tools.Get(call.Tool)
	if !found {
		return true, "", nil, fmt.Errorf("tool '%s' not found", call.Tool)
	}

	res, err := t.Execute(ctx, call.Args)
	if err != nil {
		return true, "", err, err
	}

	return true, res.Content, nil, nil
}

// PullModel requests a model download (currently only supported by Ollama)
func (b *Brain) PullModel(ctx context.Context, name string) error {
	// Re-initialize provider to ensure we have the latest endpoint
	configMap := map[string]string{
		"endpoint": b.config.Model.Endpoint,
		"model":    name,
	}

	p, err := model.GetProvider("ollama", configMap)
	if err != nil {
		return err
	}

	// Dynamic check for PullModel capability
	if puller, ok := p.(interface {
		PullModel(ctx context.Context, name string, cb func(any)) error
	}); ok {
		return puller.PullModel(ctx, name, nil)
	}

	return fmt.Errorf("provider '%s' does not support pulling models", p.Name())
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

func (b *Brain) autodetectBestModel() {
	// Only autodetect if we are using the default "llama3" which might not exist,
	// or if the model name is empty/none.
	if b.config.Model.Name != "llama3" && b.config.Model.Name != "" && b.config.Model.Name != "none" {
		return
	}

	ctx := context.Background()
	discoveries, err := b.DiscoverModels(ctx)
	if err != nil || len(discoveries) == 0 {
		return
	}

	// 1. Try to find if LLAMA-3 or 3.2 is actually there (better matching than just 'llama3')
	for _, d := range discoveries {
		name := strings.ToLower(d.Name)
		if strings.Contains(name, "llama") || strings.Contains(name, "gpt-4o") || strings.Contains(name, "phi-3") {
			b.SetModel(d.Provider, d.Name)
			return
		}
	}

	// 2. Fallback to the first available model from any provider
	if len(discoveries) > 0 {
		b.SetModel(discoveries[0].Provider, discoveries[0].Name)
	}
}

// GetSecret retrieves a secret from the vault
func (b *Brain) GetSecret(key string) (string, error) {
	if b.vault == nil {
		return "", fmt.Errorf("vault not initialized")
	}
	return b.vault.Get(key)
}
