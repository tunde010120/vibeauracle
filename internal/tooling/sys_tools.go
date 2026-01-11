package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/nathfavour/vibeauracle/sys"
)

// ReadFileTool reads the content of a file.
type ReadFileTool struct {
	fs sys.FS
}

func NewReadFileTool(f sys.FS) *ReadFileTool {
	return &ReadFileTool{fs: f}
}

func (t *ReadFileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "sys_read_file",
		Description: "Read the content of a file from the filesystem.",
		Source:      "system",
		Category:    CategoryFileSystem,
		Roles:       []AgentRole{RoleCoder, RoleEngineer},
		Complexity:  2,
		Permissions: []Permission{PermRead},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute or relative path to the file"}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	content, err := t.fs.ReadFile(input.Path)
	if err != nil {
		return &ToolResult{Status: "error", Error: err}, err
	}
	return &ToolResult{
		Status:  "success",
		Content: string(content),
		Data:    map[string]interface{}{"size": len(content)},
	}, nil
}

// WriteFileTool creates or overwrites a file.
type WriteFileTool struct {
	fs sys.FS
}

func NewWriteFileTool(f sys.FS) *WriteFileTool {
	return &WriteFileTool{fs: f}
}

func (t *WriteFileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "sys_write_file",
		Description: "Create or overwrite a file with specific content.",
		Source:      "system",
		Category:    CategoryFileSystem,
		Roles:       []AgentRole{RoleCoder, RoleEngineer},
		Complexity:  5,
		Permissions: []Permission{PermWrite},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file to write"},
				"content": {"type": "string", "description": "Content to write to the file"}
			},
			"required": ["path", "content"]
		}`),
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	err := t.fs.WriteFile(input.Path, []byte(input.Content))
	if err != nil {
		return &ToolResult{Status: "error", Error: err}, err
	}
	return &ToolResult{
		Status:    "success",
		Content:   "File written successfully",
		Artifacts: []string{input.Path},
	}, nil
}

// ShellExecTool runs a shell command.
type ShellExecTool struct{}

func (t *ShellExecTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "sys_shell_exec",
		Description: "Execute a shell command.",
		Source:      "system",
		Category:    CategorySystem,
		Roles:       []AgentRole{RoleEngineer},
		Complexity:  8,
		Permissions: []Permission{PermExecute},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {"type": "string", "description": "The command to execute"},
				"args": {"type": "array", "items": {"type": "string"}, "description": "Arguments for the command"}
			},
			"required": ["command"]
		}`),
	}
}

func (t *ShellExecTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, input.Command, input.Args...)
	output, err := cmd.CombinedOutput()
	status := "success"
	if err != nil {
		status = "error"
	}

	return &ToolResult{
		Status:  status,
		Content: string(output),
		Meta:    map[string]interface{}{"command": input.Command},
		Error:   err,
	}, nil // We return nil error here because the *execution* succeeded, even if the command failed, but we populate Error in struct
}

// SystemInfoTool provides a snapshot of system resources.
type SystemInfoTool struct {
	monitor *sys.Monitor
}

func NewSystemInfoTool(m *sys.Monitor) *SystemInfoTool {
	return &SystemInfoTool{monitor: m}
}

func (t *SystemInfoTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "sys_info",
		Description: "Get a snapshot of current system resource usage.",
		Source:      "system",
		Category:    CategorySystem,
		Roles:       []AgentRole{RoleAll},
		Complexity:  1,
		Permissions: []Permission{PermRead},
		Parameters:  json.RawMessage(`{"type": "object"}`),
	}
}

func (t *SystemInfoTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	snap, err := t.monitor.GetSnapshot()
	if err != nil {
		return nil, err
	}
	return &ToolResult{
		Status:  "success",
		Content: fmt.Sprintf("CPU: %.1f%%, RAM: %.1f%%, CWD: %s", snap.CPUUsage, snap.MemoryUsage, snap.WorkingDir),
		Data:    snap,
	}, nil
}

// ListFilesTool lists files in a directory.
type ListFilesTool struct {
	fs sys.FS
}

func NewListFilesTool(f sys.FS) *ListFilesTool {
	return &ListFilesTool{fs: f}
}

func (t *ListFilesTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "sys_list_files",
		Description: "List files and directories in a given path.",
		Source:      "system",
		Category:    CategoryFileSystem,
		Roles:       []AgentRole{RoleCoder, RoleEngineer},
		Complexity:  2,
		Permissions: []Permission{PermRead},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to list files from"}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ListFilesTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	files, err := t.fs.ListFiles(input.Path)
	if err != nil {
		return &ToolResult{Status: "error", Error: err}, err
	}
	return &ToolResult{
		Status:  "success",
		Content: fmt.Sprintf("Found %d files", len(files)),
		Data:    files,
	}, nil
}

// FetchURLTool fetches content from a URL.
type FetchURLTool struct{}

func (t *FetchURLTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "http_fetch",
		Description: "Fetch the content of a public URL (HTTP/HTTPS).",
		Source:      "system",
		Category:    CategoryNetwork,
		Roles:       []AgentRole{RoleEngineer, RoleArchitect},
		Complexity:  4,
		Permissions: []Permission{PermNetwork},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {"type": "string", "description": "The URL to fetch"}
			},
			"required": ["url"]
		}`),
	}
}

func (t *FetchURLTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", input.URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ToolResult{Status: "error", Error: err}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{Status: "error", Error: err}, err
	}

	return &ToolResult{
		Status:  "success",
		Content: string(body),
		Meta:    map[string]interface{}{"status_code": resp.StatusCode},
	}, nil
}
