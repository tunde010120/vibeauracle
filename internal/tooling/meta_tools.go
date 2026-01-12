package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolDiscoveryTool allows the agent to search for and request additional tools.
// The "Wand".
type ToolDiscoveryTool struct {
	registry *Registry
}

func NewToolDiscoveryTool(r *Registry) *ToolDiscoveryTool {
	return &ToolDiscoveryTool{registry: r}
}

func (t *ToolDiscoveryTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "sys_tool_wand",
		Description: "The Magic Wand: Search for tools, list available capabilities, or request new features. Use this when you lack a tool to complete a task.",
		Source:      "system",
		Category:    CategorySystem,
		Roles:       []AgentRole{RoleAll},
		Complexity:  1,
		Permissions: []Permission{PermRead}, // Safe tool
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"action": {
					"type": "string",
					"enum": ["search", "list_categories", "wish"],
					"description": "The operation to perform"
				},
				"query": {
					"type": "string",
					"description": "Search term for 'search' action, or description of the desired tool for 'wish'"
				}
			},
			"required": ["action"]
		}`),
	}
}

func (t *ToolDiscoveryTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		Action string `json:"action"`
		Query  string `json:"query"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	ReportStatus("🪄", "exec", fmt.Sprintf("Waving wand: %s %s", input.Action, input.Query))

	switch input.Action {
	case "list_categories":
		// Return list of categories
		categories := []string{
			string(CategoryFileSystem),
			string(CategoryAnalysis),
			string(CategorySystem),
			string(CategoryNetwork),
			string(CategoryCoding),
			string(CategorySecurity),
			string(CategoryMemory),
			string(CategoryDevOps),
		}
		return &ToolResult{
			Status:  "success",
			Content: fmt.Sprintf("Available Categories:\n- %s", strings.Join(categories, "\n- ")),
		}, nil

	case "search":
		if input.Query == "" {
			return &ToolResult{Status: "error", Error: fmt.Errorf("query required for search")}, nil
		}
		// Search the registry
		matches := t.registry.Search(input.Query)
		if len(matches) == 0 {
			return &ToolResult{Status: "success", Content: "No matching tools found. Consider 'wish'ing for it?"}, nil
		}

		var sb strings.Builder
		sb.WriteString("Found Tools (definitions injection):\n")
		for _, tool := range matches {
			// Reuse the definition generator logic, but for individual tools
			// We manually format here to keep it distinct
			m := tool.Metadata()
			sb.WriteString(fmt.Sprintf("## %s\n%s\nUsage: %s\n---\n", m.Name, m.Description, string(m.Parameters)))
		}
		sb.WriteString("\nSystem Note: These tool definitions are now visible to you in this turn. usage is valid.")

		return &ToolResult{
			Status:  "success",
			Content: sb.String(),
		}, nil

	case "wish":
		ReportStatus("✨", "reflect", "Capture wish: "+input.Query)
		// Effectively a log for the developer, but we tell the agent it's acknowledged.
		return &ToolResult{
			Status:  "success",
			Content: fmt.Sprintf("Wish granted (logged). The system engineers have been notified of your need for: '%s'. Continue with best-effort alternatives.", input.Query),
		}, nil
	}

	return &ToolResult{Status: "error", Error: fmt.Errorf("unknown action: %s", input.Action)}, nil
}
