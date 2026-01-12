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
	// GitHub Models uses the standard OpenAI /models endpoint or its own models API
	req, err := http.NewRequestWithContext(ctx, "GET", GithubModelsBaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	// As per AI.md: Use specific headers for GitHub Models API
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github models list failed: %s", resp.Status)
	}

	// GitHub Models API can return either a top-level array or an object with a "data" field (OpenAI style)
	var raw interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding github models: %w", err)
	}

	var models []string
	
	processEntry := func(m map[string]interface{}) {
		// Use "name" as the primary identifier if available, per AI.md example
		name, _ := m["name"].(string)
		id, _ := m["id"].(string)
		
		target := name
		if target == "" {
			target = id
		}

		if target != "" {
			// Per AI.md: Filter for chat-friendly models. 
			// We check the "task" field, but we also check "type" and name patterns 
			// to ensure we don't miss anything that could be used for chat.
			task, _ := m["task"].(string)
			lTask := strings.ToLower(task)
			isChat := strings.Contains(lTask, "chat") || strings.Contains(lTask, "completion")
			
			// Fallback: name-based filtering if task info is missing or generic
			if !isChat || lTask == "" {
				lTarget := strings.ToLower(target)
				isChat = isChat || strings.Contains(lTarget, "gpt") || 
					strings.Contains(lTarget, "llama") || 
					strings.Contains(lTarget, "phi") || 
					strings.Contains(lTarget, "mistral") ||
					strings.Contains(lTarget, "mixtral") ||
					strings.Contains(lTarget, "command") ||
					strings.Contains(lTarget, "claude")
			}
			
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
