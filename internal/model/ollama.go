package model

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ollama/ollama/api"
)

func init() {
	Register("ollama", func(config map[string]string) (Provider, error) {
		return NewOllamaProvider(config["endpoint"], config["model"])
	})
}

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	client *api.Client
	model  string
}

func (p *OllamaProvider) Name() string { return "ollama" }

// NewOllamaProvider creates a new Ollama provider
// host is the Ollama server URL (e.g., "http://localhost:11434")
// modelName is the model to use (e.g., "llama3")
func NewOllamaProvider(host string, modelName string) (*OllamaProvider, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		// Fallback to manual client creation if env vars are not set
		client = api.NewClient(&url.URL{Scheme: "http", Host: host}, http.DefaultClient)
	}

	return &OllamaProvider{
		client: client,
		model:  modelName,
	}, nil
}

// Generate sends a prompt to Ollama and returns the response
func (p *OllamaProvider) Generate(ctx context.Context, prompt string) (string, error) {
	var response string
	
	req := &api.GenerateRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: new(bool), // false
	}

	fn := func(resp api.GenerateResponse) error {
		response += resp.Response
		return nil
	}

	err := p.client.Generate(ctx, req, fn)
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}

	return response, nil
}

