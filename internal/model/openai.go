package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func init() {
	Register("openai", func(config map[string]string) (Provider, error) {
		return NewOpenAIProvider(config["api_key"], config["model"], config["base_url"])
	})
}

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	llm     llms.Model
	apiKey  string
	baseURL string
}

func (p *OpenAIProvider) Name() string { return "openai" }

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string, modelName string, baseURL string) (*OpenAIProvider, error) {
	if modelName == "" {
		modelName = "gpt-4o" // Default to a smart, modern model
	}

	opts := []openai.Option{
		openai.WithToken(apiKey),
		openai.WithModel(modelName),
	}

	if baseURL != "" {
		// Clean up common URL mistakes
		baseURL = strings.TrimSuffix(baseURL, "/")
		opts = append(opts, openai.WithBaseURL(baseURL))
	} else {
		baseURL = "https://api.openai.com/v1"
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("openai init: %w", err)
	}

	return &OpenAIProvider{
		llm:     llm,
		apiKey:  apiKey,
		baseURL: baseURL,
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
	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching openai models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("openai api key is invalid or expired")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai models list failed: %s", resp.Status)
	}

	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding openai models: %w", err)
	}

	var models []string
	for _, m := range data.Data {
		// Only include common chat models to avoid cluttering the UI
		id := m.ID
		isChatModel := strings.HasPrefix(id, "gpt") || 
			strings.HasPrefix(id, "o1-") || 
			id == "o3-mini"
		
		if isChatModel {
			models = append(models, id)
		}
	}
	
	if len(models) == 0 && len(data.Data) > 0 {
		// If we filtered out everything but there ARE models, 
		// maybe it's a custom provider, just return everything.
		for _, m := range data.Data {
			models = append(models, m.ID)
		}
	}

	return models, nil
}

