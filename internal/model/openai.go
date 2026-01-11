package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
	llm    llms.Model
	apiKey string
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
		llm:    llm,
		apiKey: apiKey,
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

// ListModels returns a list of available models from OpenAI
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai models list failed: %s", resp.Status)
	}

	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range data.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

