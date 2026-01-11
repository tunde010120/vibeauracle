package prompt

import (
	"context"
	"time"
)

// Intent is the prompt system's classification of what the user is trying to do.
type Intent string

const (
	IntentAuto Intent = "auto"
	IntentAsk  Intent = "ask"  // Q&A / explanation
	IntentCRUD Intent = "crud" // file/system changes, debugging, implementation
	IntentPlan Intent = "plan" // project planning, architecture, breakdown
	IntentChat Intent = "chat" // general conversation
)

// Envelope is the final payload sent to the model.
type Envelope struct {
	Intent       Intent
	Prompt       string
	Instructions []string
	Metadata     map[string]any
}

// PartType is a parsed response segment kind.
type PartType string

const (
	PartText PartType = "text"
	PartCode PartType = "code"
)

// ResponsePart is a piece of model output.
type ResponsePart struct {
	Type     PartType
	Lang     string
	Content  string
	StartPos int
	EndPos   int
}

// ParsedResponse is the model output parsed into semantic chunks.
type ParsedResponse struct {
	Raw   string
	Parts []ResponsePart
}

// Recommendation is an optional, low-frequency hint layer.
type Recommendation struct {
	Title       string
	Description string
	Confidence  float64
}

// Recommender can generate suggested follow-up actions based on prompt context.
// It should be called sparingly (budgeted) by the prompt system.
type Recommender interface {
	Recommend(ctx context.Context, in RecommendInput) ([]Recommendation, error)
}

// Model allows the prompt system to query an LLM for background tasks (like recommendations).
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// RecommendInput is intentionally small; we can grow it as we add richer signals.
type RecommendInput struct {
	Intent     Intent
	UserText   string
	WorkingDir string
	Time       time.Time
}

// Memory is a thin interface over the local learning system.
type Memory interface {
	Store(key string, value string) error
	Recall(query string) ([]string, error)
}
