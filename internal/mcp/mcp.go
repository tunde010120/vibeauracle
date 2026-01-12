package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nathfavour/vibeauracle/tooling"
)

// Bridge manages connections to various MCP-compliant tools and registries.
type Bridge struct {
	registry *tooling.Registry
}

func NewBridge(r *tooling.Registry) *Bridge {
	return &Bridge{
		registry: r,
	}
}

// ListTools returns all tools available through the bridge in an MCP-compliant format.
func (b *Bridge) ListTools() []tooling.MCPTool {
	tools := b.registry.List()
	mcpTools := make([]tooling.MCPTool, len(tools))
	for i, t := range tools {
		mcpTools[i] = tooling.ToMCP(t)
	}
	return mcpTools
}

// Execute runs a tool from the registry.
func (b *Bridge) Execute(ctx context.Context, toolName string, args json.RawMessage) (interface{}, error) {
	t, ok := b.registry.Get(toolName)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	return t.Execute(ctx, args)
}


