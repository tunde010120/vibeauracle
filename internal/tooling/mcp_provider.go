package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// MCPProvider connects to an external Model Context Protocol server.
type MCPProvider struct {
	config MCPConfig
	client *MCPClient
}

type MCPConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env"`
}

func NewMCPProvider(cfg MCPConfig) *MCPProvider {
	return &MCPProvider{
		config: cfg,
	}
}

func (p *MCPProvider) Name() string { return "mcp:" + p.config.Name }

func (p *MCPProvider) Provide(ctx context.Context) ([]Tool, error) {
	if p.client == nil {
		p.client = NewMCPClient(p.config)
		if err := p.client.Start(); err != nil {
			return nil, err
		}
	}

	mcpTools, err := p.client.ListTools(ctx)
	if err != nil {
		return nil, err
	}

	var tools []Tool
	for _, mt := range mcpTools {
		tools = append(tools, &ExternalMCPTool{
			client: p.client,
			meta: ToolMetadata{
				Name:        mt.Name,
				Description: mt.Description,
				Parameters:  mt.InputSchema,
				Source:      p.Name(),
				Permissions: []Permission{PermNetwork, PermRead, PermWrite}, // Conservative default for MCP
			},
		})
	}

	return tools, nil
}

// ExternalMCPTool represents a tool hosted on a remote MCP server.
type ExternalMCPTool struct {
	client *MCPClient
	meta   ToolMetadata
}

func (t *ExternalMCPTool) Metadata() ToolMetadata { return t.meta }

func (t *ExternalMCPTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	return t.client.CallTool(ctx, t.meta.Name, args)
}

// MCPClient handles the low-level communication with an MCP server via stdio.
type MCPClient struct {
	config MCPConfig
	cmd    *exec.Cmd
	stdin  *json.Encoder
	stdout *json.Decoder
	mu     sync.Mutex
	id     int
}

func NewMCPClient(cfg MCPConfig) *MCPClient {
	return &MCPClient{config: cfg}
}

func (c *MCPClient) Start() error {
	c.cmd = exec.Command(c.config.Command, c.config.Args...)
	c.cmd.Env = append(os.Environ(), c.config.Env...)

	in, err := c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	c.stdin = json.NewEncoder(in)
	c.stdout = json.NewDecoder(out)

	return c.cmd.Start()
}

func (c *MCPClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.id++
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.id,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	if err := c.stdin.Encode(req); err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Tools []MCPTool `json:"tools"`
		} `json:"result"`
		Error interface{} `json:"error"`
	}

	if err := c.stdout.Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error: %v", resp.Error)
	}

	return resp.Result.Tools, nil
}

func (c *MCPClient) CallTool(ctx context.Context, name string, args json.RawMessage) (*ToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.id++
	var params map[string]interface{}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.id,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": params,
		},
	}

	if err := c.stdin.Encode(req); err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error interface{} `json:"error"`
	}

	if err := c.stdout.Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return &ToolResult{Status: "error", Error: fmt.Errorf("%v", resp.Error)}, fmt.Errorf("mcp error: %v", resp.Error)
	}

	// Concatenate text content
	var content string
	for _, part := range resp.Result.Content {
		if part.Type == "text" {
			content += part.Text + "\n"
		}
	}

	status := "success"
	if resp.Result.IsError {
		status = "error"
	}

	return &ToolResult{
		Status:  status,
		Content: content,
		Data:    resp.Result,
	}, nil
}
