package prompt

import (
	"context"
	"testing"

	"github.com/nathfavour/vibeauracle/sys"
)

type memStub struct{}

func (m *memStub) Store(key string, value string) error { return nil }
func (m *memStub) Recall(query string) ([]string, error) {
	return []string{"previous hint"}, nil
}

func TestBuild_ClassifiesAsk(t *testing.T) {
	cfg := sys.Config{}
	cfg.Prompt.Mode = "auto"
	cfg.Prompt.LearningEnabled = true

	s := New(&cfg, &memStub{}, &NoopRecommender{})
	env, _, err := s.Build(context.Background(), "why does this happen?", sys.Snapshot{WorkingDir: "/tmp"}, "")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if env.Intent != IntentAsk {
		t.Fatalf("got intent %q, want %q", env.Intent, IntentAsk)
	}
	if env.Prompt == "" {
		t.Fatal("expected prompt to be non-empty")
	}
}

func TestParseModelResponse_CodeFence(t *testing.T) {
	raw := "hello\n```go\nfmt.Println(\"x\")\n```\nbye"
	parsed := ParseModelResponse(raw)
	if len(parsed.Parts) != 3 {
		t.Fatalf("got %d parts, want 3", len(parsed.Parts))
	}
	if parsed.Parts[1].Type != PartCode || parsed.Parts[1].Lang != "go" {
		t.Fatalf("expected middle part to be go code")
	}
}
