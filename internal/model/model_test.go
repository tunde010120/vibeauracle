package model

import (
	"context"
	"testing"
)

type MockProvider struct {
	Response string
	Err      error
}

func (m *MockProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func TestModel_Generate(t *testing.T) {
	mock := &MockProvider{Response: "Test Response"}
	m := New(mock)

	resp, err := m.Generate(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != "Test Response" {
		t.Errorf("Expected 'Test Response', got '%s'", resp)
	}
}

