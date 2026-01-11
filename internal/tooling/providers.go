package tooling

import (
	"context"

	"github.com/nathfavour/vibeauracle/sys"
)

// SystemProvider provides core tools built into the system.
type SystemProvider struct {
	fs      sys.FS
	monitor *sys.Monitor
	guard   *SecurityGuard
}

func NewSystemProvider(f sys.FS, m *sys.Monitor, guard *SecurityGuard) *SystemProvider {
	return &SystemProvider{fs: f, monitor: m, guard: guard}
}

func (p *SystemProvider) Name() string { return "system" }

func (p *SystemProvider) Provide(ctx context.Context) ([]Tool, error) {
	tools := []Tool{
		NewReadFileTool(p.fs),
		NewWriteFileTool(p.fs),
		NewListFilesTool(p.fs),
		NewListDirTool(p.fs),
		NewFileStatsTool(p.fs),
		NewTraversalTool(p.fs),
		&ShellExecTool{},
		&GrepTool{},
		NewSystemInfoTool(p.monitor),
		&FetchURLTool{},
	}

	var secured []Tool
	for _, t := range tools {
		if p.guard != nil {
			secured = append(secured, WrapWithSecurity(t, p.guard))
		} else {
			secured = append(secured, t)
		}
	}

	return secured, nil
}

// VibeProvider provides community-contributed tools.
type VibeProvider struct{}

func NewVibeProvider() *VibeProvider {
	return &VibeProvider{}
}

func (p *VibeProvider) Name() string { return "vibes" }

func (p *VibeProvider) Provide(ctx context.Context) ([]Tool, error) {
	// In a real implementation, this would scan the vibes/ directory,
	// load shared objects, or communicate with vibe processes.
	// For now, we'll return a placeholder or adapt the existing hello-world.
	return []Tool{}, nil
}

// Global Registry Setup
func Setup(f sys.FS, m *sys.Monitor, guard *SecurityGuard) *Registry {
	r := NewRegistry()
	r.RegisterProvider(NewSystemProvider(f, m, guard))
	r.RegisterProvider(NewVibeProvider())

	// Example MCP Provider (can be loaded from config in the future)
	// r.RegisterProvider(NewMCPProvider(MCPConfig{
	// 	Name:    "github",
	// 	Command: "npx",
	// 	Args:    []string{"-y", "@modelcontextprotocol/server-github"},
	// }))

	_ = r.Sync(context.Background())
	return r
}
