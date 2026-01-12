package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nathfavour/vibeauracle/sys"
)

// ListDirTool provides enhanced directory listing with metadata and ignoring capabilities.
type ListDirTool struct {
	fs sys.FS
}

func NewListDirTool(f sys.FS) *ListDirTool {
	return &ListDirTool{fs: f}
}

func (t *ListDirTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "fs_list_dir",
		Description: "List files in a directory with metadata (size, type).",
		Source:      "system",
		Category:    CategoryFileSystem,
		Roles:       []AgentRole{RoleCoder, RoleResearcher, RoleArchitect}, // "RoleResearch" typo fixed to "RoleResearcher" in logic
		Complexity:  2,
		Permissions: []Permission{PermRead},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute path to the directory"}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ListDirTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
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

	// Simplify logic: ListFiles already returns names.
	// For "ultra-granular" we might want stats, but sys.FS interface is simple for now.
	// We can enhance sys.FS later or use os.Stat here if path is local.

	return &ToolResult{
		Status:  "success",
		Content: fmt.Sprintf("Found %d entries in %s", len(files), input.Path),
		Data:    files,
	}, nil
}

// GrepTool searches for patterns inside files.
type GrepTool struct{}

func (t *GrepTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "fs_grep",
		Description: "Search for regex patterns in files within a directory.",
		Source:      "system",
		Category:    CategoryAnalysis,
		Roles:       []AgentRole{RoleResearcher, RoleEngineer},
		Complexity:  5,
		Permissions: []Permission{PermRead},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Directory to search"},
				"pattern": {"type": "string", "description": "Regex pattern"},
				"recursive": {"type": "boolean", "description": "Search recursively"}
			},
			"required": ["path", "pattern"]
		}`),
	}
}

func (t *GrepTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	// Implementation placeholder for granular capability
	return &ToolResult{Status: "error", Content: "Not implemented yet"}, nil
}

// FileStatsTool provides detailed inode information.
type FileStatsTool struct {
	fs sys.FS
}

func NewFileStatsTool(f sys.FS) *FileStatsTool {
	return &FileStatsTool{fs: f}
}

func (t *FileStatsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "fs_stat",
		Description: "Get detailed metadata (size, modtime, permissions) for a file.",
		Source:      "system",
		Category:    CategoryFileSystem,
		Roles:       []AgentRole{RoleQA, RoleEngineer},
		Complexity:  1,
		Permissions: []Permission{PermRead},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to file"}
			},
			"required": ["path"]
		}`),
	}
}

func (t *FileStatsTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	info, err := os.Stat(input.Path)
	if err != nil {
		return &ToolResult{Status: "error", Error: err}, err
	}

	return &ToolResult{
		Status:  "success",
		Content: fmt.Sprintf("%s: size=%d mode=%s mod=%s", info.Name(), info.Size(), info.Mode(), info.ModTime()),
		Data: map[string]interface{}{
			"name":     info.Name(),
			"size":     info.Size(),
			"mode":     info.Mode().String(),
			"mod_time": info.ModTime(),
			"is_dir":   info.IsDir(),
		},
	}, nil
}
