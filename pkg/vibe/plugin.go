package vibe

import "context"

// Vibe is the interface that community vibes must implement.
type Vibe interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// Skill represents an agentic capability that can be registered with the Brain.
type Skill struct {
	ID          string
	Name        string
	Description string
	Action      func(ctx context.Context, input string) (string, error)
}
