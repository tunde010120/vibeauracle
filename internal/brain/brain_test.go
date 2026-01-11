package brain

import (
	"context"
	"testing"

	"github.com/nathfavour/vibeauracle/model"
)

type MockProvider struct{}

func (m *MockProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return "Mocked AI Response", nil
}

func (m *MockProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{"mock-model"}, nil
}

func (m *MockProvider) Name() string {
	return "mock"
}

func TestBrain_Process(t *testing.T) {
	b := New()
	// Inject mock provider to avoid network/ollama dependency in tests
	b.model = model.New(&MockProvider{})

	req := Request{
		ID:      "test-1",
		Content: "Hello Brain",
	}

	resp, err := b.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Brain processing failed: %v", err)
	}

	if resp.Content != "Mocked AI Response" {
		t.Errorf("Unexpected brain response: %s", resp.Content)
	}
}

