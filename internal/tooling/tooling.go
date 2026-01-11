package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nathfavour/vibeauracle/sys"
)

// Tool represents a programmable interface that can be exposed to a model.
type Tool interface {
	Metadata() ToolMetadata
	Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error)
}

// ToolCategory defines the operational domain of a tool.
type ToolCategory string

const (
	CategoryFileSystem ToolCategory = "filesystem" // CRUD operations on files
	CategoryAnalysis   ToolCategory = "analysis"   // Static analysis, linting, reading
	CategorySystem     ToolCategory = "system"     // OS-level interaction (shell, env)
	CategoryNetwork    ToolCategory = "network"    // HTTP, P2P, Sockets
	CategoryCoding     ToolCategory = "coding"     // AST manipulation, refactoring
	CategorySecurity   ToolCategory = "security"   // Keys, encryption, permissions
	CategoryMemory     ToolCategory = "memory"     // RAG, Vector Search, Recall
	CategoryDevOps     ToolCategory = "devops"     // Docker, Git, CI/CD
)

// AgentRole defines the persona/stage for which this tool is relevant.
type AgentRole string

const (
	RoleArchitect       AgentRole = "architect"        // High-level planning, discovery
	RoleEngineer        AgentRole = "engineer"         // Implementation, debugging, scripting
	RoleCoder           AgentRole = "coder"            // Low-level file editing, syntax fixing
	RoleSecurityOfficer AgentRole = "security_officer" // Auditing, sensitive file access
	RoleQA              AgentRole = "qa"               // Testing, validation, linting
	RoleResearcher      AgentRole = "researcher"       // Web search, doc reading
	RoleAll             AgentRole = "*"                // Universal access
)

// ToolMetadata holds detailed information about a tool for agentic reasoning.
type ToolMetadata struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
	Permissions []Permission    `json:"permissions"`
	Source      string          `json:"source"`

	// Enhanced fields for multi-layered agents
	Category   ToolCategory `json:"category"`
	Roles      []AgentRole  `json:"roles"`      // Which agent personas should see this?
	Complexity int          `json:"complexity"` // 1-10 estimation of cognitive load
}

// ToolResult is a structured response enabling agentic reflection.
type ToolResult struct {
	Content   string                 `json:"content"`             // The primary textual output
	Data      interface{}            `json:"data,omitempty"`      // Structured data for programmatical use
	Status    string                 `json:"status"`              // "success", "error", "partial"
	Artifacts []string               `json:"artifacts,omitempty"` // List of files created/modified
	Error     error                  `json:"-"`                   // Go error for internal handling
	Meta      map[string]interface{} `json:"meta,omitempty"`      // Extra context (latency, confidence)
}

// Permission represents a capability required by a tool.
type Permission string

const (
	PermRead      Permission = "read"
	PermWrite     Permission = "write"
	PermExecute   Permission = "execute"
	PermNetwork   Permission = "network"
	PermSensitive Permission = "sensitive" // Access to passwords, keys, etc.
)

// ToolProvider is an interface for sources that provide a set of tools.
type ToolProvider interface {
	Name() string
	Provide(ctx context.Context) ([]Tool, error)
}

// Registry manages the set of available tools from various providers.
type Registry struct {
	providers []ToolProvider
	tools     map[string]Tool
	mu        sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) RegisterProvider(p ToolProvider) {
	r.providers = append(r.providers, p)
}

func (r *Registry) Sync(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing tools or intelligently update them
	r.tools = make(map[string]Tool)

	for _, p := range r.providers {
		tools, err := p.Provide(ctx)
		if err != nil {
			return fmt.Errorf("provider %s failed: %w", p.Name(), err)
		}
		for _, t := range tools {
			r.tools[t.Metadata().Name] = t
		}
	}
	return nil
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Metadata().Name] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []Tool
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// MCPTool matches the official Model Context Protocol tool definition.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToMCP converts a tool to an MCP-compliant structure.
func ToMCP(t Tool) MCPTool {
	m := t.Metadata()
	return MCPTool{
		Name:        m.Name,
		Description: m.Description,
		InputSchema: m.Parameters,
	}
}

// DefaultRegistry creates a registry populated with core system tools.
func DefaultRegistry(f sys.FS, m *sys.Monitor, guard *SecurityGuard) *Registry {
	r := NewRegistry()

	tools := []Tool{
		NewReadFileTool(f),
		NewWriteFileTool(f),
		NewListFilesTool(f),
		NewTraversalTool(f),
		&ShellExecTool{},
		NewSystemInfoTool(m),
		&FetchURLTool{},
	}

	for _, t := range tools {
		if guard != nil {
			r.Register(WrapWithSecurity(t, guard))
		} else {
			r.Register(t)
		}
	}

	return r
}

// GetPromptDefinitions returns a human-readable or machine-parsable definition
// of all tools to be injected into a model's prompt.
func (r *Registry) GetPromptDefinitions() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var defs string
	for _, t := range r.tools {
		m := t.Metadata()
		defs += fmt.Sprintf("- %s: %s\n", m.Name, m.Description)
	}
	return defs
}
