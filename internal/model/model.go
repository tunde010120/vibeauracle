package model

import (
	"context"
	"fmt"
)

// Provider represents an AI model provider (e.g., Ollama, OpenAI)
type Provider interface {
	Generate(ctx context.Context, prompt string) (string, error)
	ListModels(ctx context.Context) ([]string, error)
	Name() string
}

type ProviderFactory func(config map[string]string) (Provider, error)

var (
	registry = make(map[string]ProviderFactory)
)

// Register adds a new provider factory to the registry
func Register(name string, factory ProviderFactory) {
	registry[name] = factory
}

// GetProvider creates a provider instance using the registry
func GetProvider(name string, config map[string]string) (Provider, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return factory(config)
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
