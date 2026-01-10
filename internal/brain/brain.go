package brain

import (
	"context"
	"fmt"

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
}

func New() *Brain {
	return &Brain{
		model:   model.New(&model.OllamaProvider{}),
		monitor: sys.NewMonitor(),
	}
}

// Process handles the "Plan-Execute-Reflect" loop
func (b *Brain) Process(ctx context.Context, req Request) (Response, error) {
	// 1. Perceive: Receive request + SystemSnapshot
	snapshot, _ := b.monitor.GetSnapshot()
	fmt.Printf("Perceiving request: %s (System: CPU %.2f%%, Mem %.2f%%)\n", 
		req.Content, snapshot.CPUUsage, snapshot.MemoryUsage)

	// 2. Recall (RAG/Context) - Placeholder
	
	// 3. Plan & Execute via Model
	resp, err := b.model.Generate(ctx, req.Content)
	if err != nil {
		return Response{}, fmt.Errorf("generating response: %w", err)
	}
	
	// 4. Reflect - Placeholder

	return Response{
		Content: resp,
	}, nil
}

