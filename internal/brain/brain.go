package brain

import (
	"context"
	"fmt"
	"strings"

	vcontext "github.com/nathfavour/vibeauracle/context"
	"github.com/nathfavour/vibeauracle/model"
	"github.com/nathfavour/vibeauracle/sys"
	"github.com/nathfavour/vibeauracle/auth"
	"github.com/nathfavour/vibeauracle/tooling"
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
	memory   *vcontext.Memory
	tools    *tooling.Registry
	security *tooling.SecurityGuard
	sessions map[string]*tooling.Session
}

func New() *Brain {
	// Initialize config
	cm, _ := sys.NewConfigManager()
	cfg, _ := cm.Load()

	// Initialize model provider based on config
	var provider model.Provider
	if cfg.Model.Provider == "openai" {
		provider, _ = model.NewOpenAIProvider(cfg.Model.Endpoint, cfg.Model.Name)
	} else {
		provider, _ = model.NewOllamaProvider(cfg.Model.Endpoint, cfg.Model.Name)
	}

	fs := sys.NewLocalFS("")
	registry := tooling.NewRegistry()
	registry.Register(tooling.NewTraversalTool(fs))

	return &Brain{
		model:    model.New(provider),
		monitor:  sys.NewMonitor(),
		fs:       fs,
		config:   cfg,
		auth:     auth.NewHandler(),
		memory:   vcontext.NewMemory(),
		tools:    registry,
		security: tooling.NewSecurityGuard(),
		sessions: make(map[string]*tooling.Session),
	}
}

// Process handles the "Plan-Execute-Reflect" loop
func (b *Brain) Process(ctx context.Context, req Request) (Response, error) {
	// 1. Perceive: Receive request + SystemSnapshot
	snapshot, _ := b.monitor.GetSnapshot()
	fmt.Printf("Perceiving request: %s (System: CPU %.2f%%, Mem %.2f%%, CWD: %s)\n",
		req.Content, snapshot.CPUUsage, snapshot.MemoryUsage, snapshot.WorkingDir)

	// 2. Recall (RAG/Context)
	snippets, _ := b.memory.Recall(req.Content)
	contextStr := strings.Join(snippets, "\n")

	// 3. Plan & Execute via Model
	augmentedPrompt := fmt.Sprintf("Context:\n%s\n\nSystem CWD: %s\nCapabilities: File CRUD (Read, Write, Delete, List)\nUser Request: %s", 
		contextStr, snapshot.WorkingDir, req.Content)
	
	// Pre-execution Security Check
	decision := b.auth.Check(auth.Request{
		Action:   auth.ActionFSWrite,
		Resource: "project_files",
		Context:  req.Content,
	})
	
	if decision == auth.DecisionDeny {
		return Response{Content: "Operation denied by security policy."}, nil
	}

	resp, err := b.model.Generate(ctx, augmentedPrompt)
	if err != nil {
		return Response{}, fmt.Errorf("generating response: %w", err)
	}
	
	// Store result in memory
	_ = b.memory.Store(req.ID, resp)
	
	// 4. Reflect - Placeholder

	return Response{
		Content: resp,
	}, nil
}

