package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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

	var data []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range data {
		id := m.ID
		if id == "" {
			id = m.Name
		}
		models = append(models, id)
	}
	return models, nil
}
