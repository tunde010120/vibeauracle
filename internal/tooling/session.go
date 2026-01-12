package tooling

import (
	"time"
)

// ToolCall represents a specific tool invocation within a thread.
type ToolCall struct {
	ToolName  string      `json:"tool_name"`
	Args      interface{} `json:"args"`
	Result    interface{} `json:"result"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Thread represents a single interaction or "prompt" in a session.
type Thread struct {
	ID        string                 `json:"id"`
	Prompt    string                 `json:"prompt"`
	Response  string                 `json:"response"`
	ToolCalls []ToolCall             `json:"tool_calls"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// Session represents a "process" containing multiple threads.
type Session struct {
	ID        string    `json:"id"`
	Threads   []*Thread `json:"threads"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewSession(id string) *Session {
	return &Session{
		ID:        id,
		Threads:   make([]*Thread, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *Session) AddThread(t *Thread) {
	s.Threads = append(s.Threads, t)
	s.UpdatedAt = time.Now()
}

func (s *Session) Export() map[string]interface{} {
	return map[string]interface{}{
		"id":         s.ID,
		"threads":    s.Threads,
		"created_at": s.CreatedAt,
		"updated_at": s.UpdatedAt,
	}
}
