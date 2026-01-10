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

func New(p Provider) *Model {
	return &Model{provider: p}
}

func (m *Model) Generate(ctx context.Context, prompt string) (string, error) {
	if m.provider == nil {
		return "", fmt.Errorf("no provider configured")
	}
	return m.provider.Generate(ctx, prompt)
}

// OllamaProvider is a placeholder for Ollama integration
type OllamaProvider struct{}

func (p *OllamaProvider) Generate(ctx context.Context, prompt string) (string, error) {
	// Actual implementation would call Ollama API
	return "Ollama response to: " + prompt, nil
}

