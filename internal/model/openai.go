package model

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func init() {
	Register("openai", func(config map[string]string) (Provider, error) {
		return NewOpenAIProvider(config["api_key"], config["model"])
	})
}

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	llm llms.Model
}

func (p *OpenAIProvider) Name() string { return "openai" }

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string, modelName string) (*OpenAIProvider, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithModel(modelName),
	)
	if err != nil {
		return nil, fmt.Errorf("openai init: %w", err)
	}

	return &OpenAIProvider{
		llm: llm,
	}, nil
}

// Generate sends a prompt to OpenAI and returns the response
func (p *OpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := llms.GenerateFromSinglePrompt(ctx, p.llm, prompt)
	if err != nil {
		return "", fmt.Errorf("openai generate: %w", err)
	}

	return resp, nil
}

