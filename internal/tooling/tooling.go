package tooling

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool represents a programmable interface that can be exposed to a model.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage // JSON Schema
	Execute(ctx context.Context, args json.RawMessage) (interface{}, error)
}

// Registry manages the set of available tools.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	var list []Tool
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// GetPromptDefinitions returns a human-readable or machine-parsable definition
// of all tools to be injected into a model's prompt.
func (r *Registry) GetPromptDefinitions() string {
	var defs string
	for _, t := range r.tools {
		defs += fmt.Sprintf("- %s: %s\n", t.Name(), t.Description())
	}
	return defs
}

