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

// GithubProvider implements the Provider interface for GitHub Models
// Since GitHub Models is OpenAI-compatible, we wrap the OpenAI provider
// but point it to the GitHub inference endpoint.
type GithubProvider struct {
	llm   llms.Model
	token string
}

const (
	GithubModelsBaseURL = "https://models.inference.ai.azure.com"
)

func init() {
	Register("github-models", func(config map[string]string) (Provider, error) {
		return NewGithubProvider(config["token"], config["model"])
	})
}

func (p *GithubProvider) Name() string { return "github-models" }

// NewGithubProvider creates a new GitHub Models provider
func NewGithubProvider(token string, modelName string) (*GithubProvider, error) {
	if modelName == "" {
		modelName = "gpt-4o" // Sensible default for GitHub Models
	}
	
	llm, err := openai.New(
		openai.WithToken(token),
		openai.WithBaseURL(GithubModelsBaseURL),
		openai.WithModel(modelName),
	)
	if err != nil {
		return nil, fmt.Errorf("github models init: %w", err)
	}

	return &GithubProvider{
		llm:   llm,
		token: token,
	}, nil
}

// Generate sends a prompt to GitHub Models and returns the response
func (p *GithubProvider) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := llms.GenerateFromSinglePrompt(ctx, p.llm, prompt)
	if err != nil {
		return "", fmt.Errorf("github models generate: %w", err)
	}

	return resp, nil
}

// ListModels returns a list of available models from GitHub Models
func (p *GithubProvider) ListModels(ctx context.Context) ([]string, error) {
	// GitHub Models uses the standard OpenAI /models endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", GithubModelsBaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github models list failed: %s", resp.Status)
	}

	// GitHub Models API can return either a top-level array or an object with a "data" field (OpenAI style)
	// We use a flexible map to decode first, then extract correctly.
	var raw interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding github models: %w", err)
	}

	var models []string
	
	processEntry := func(m map[string]interface{}) {
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		
		target := id
		if target == "" {
			target = name
		}

		if target != "" {
			// Filter for chat-friendly models if it's GitHub
			isChat := strings.Contains(strings.ToLower(target), "gpt") || 
				strings.Contains(strings.ToLower(target), "llama") || 
				strings.Contains(strings.ToLower(target), "phi") || 
				strings.Contains(strings.ToLower(target), "mistral") ||
				strings.Contains(strings.ToLower(target), "codellama")
			
			if isChat {
				models = append(models, target)
			}
		}
	}

	switch v := raw.(type) {
	case []interface{}:
		// Top-level array format
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				processEntry(m)
			}
		}
	case map[string]interface{}:
		// Object format (check for "data" field)
		if data, ok := v["data"].([]interface{}); ok {
			for _, item := range data {
				if m, ok := item.(map[string]interface{}); ok {
					processEntry(m)
				}
			}
		} else {
			// Maybe it's just a single object? (unlikely but safe)
			processEntry(v)
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no models found in github response")
	}

	return models, nil
}
