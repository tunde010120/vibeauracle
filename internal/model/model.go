package model

import (
	"context"
	"fmt"
)

// Provider represents an AI model provider (e.g., Ollama, OpenAI)
type Provider interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Model handles AI interactions
type Model struct {
	provider Provider
}

// New creates a new Model with the given provider
func New(p Provider) *Model {
	return &Model{provider: p}
}

// Generate uses the configured provider to generate a response
func (m *Model) Generate(ctx context.Context, prompt string) (string, error) {
	if m.provider == nil {
		return "", fmt.Errorf("no provider configured")
	}
	return m.provider.Generate(ctx, prompt)
}
