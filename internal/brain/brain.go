package brain

import (
	"context"
	"fmt"
	"strings"

	vcontext "github.com/nathfavour/vibeauracle/context"
	"github.com/nathfavour/vibeauracle/model"
	"github.com/nathfavour/vibeauracle/sys"
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
	model   *model.Model
	monitor *sys.Monitor
	memory  *vcontext.Memory
}

func New() *Brain {
	// For now, use defaults for Ollama. 
	// In production, these would come from config or environment variables.
	ollamaProvider, _ := model.NewOllamaProvider("localhost:11434", "llama3")
	
	return &Brain{
		model:   model.New(ollamaProvider),
		monitor: sys.NewMonitor(),
		memory:  vcontext.NewMemory(),
	}
}

// Process handles the "Plan-Execute-Reflect" loop
func (b *Brain) Process(ctx context.Context, req Request) (Response, error) {
	// 1. Perceive: Receive request + SystemSnapshot
	snapshot, _ := b.monitor.GetSnapshot()
	fmt.Printf("Perceiving request: %s (System: CPU %.2f%%, Mem %.2f%%)\n", 
		req.Content, snapshot.CPUUsage, snapshot.MemoryUsage)

	// 2. Recall (RAG/Context)
	snippets, _ := b.memory.Recall(req.Content)
	contextStr := strings.Join(snippets, "\n")
	
	// 3. Plan & Execute via Model
	augmentedPrompt := fmt.Sprintf("Context:\n%s\n\nUser Request: %s", contextStr, req.Content)
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

