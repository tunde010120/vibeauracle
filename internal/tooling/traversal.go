package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nathfavour/vibeauracle/sys"
)

// TraversalTool is an intelligent file walker that respects ignore patterns.
type TraversalTool struct {
	fs sys.FS
}

func NewTraversalTool(f sys.FS) *TraversalTool {
	return &TraversalTool{fs: f}
}

func (t *TraversalTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "traverse_source",
		Description: "Intelligently traverses source code directory.",
		Source:      "system",
		Category:    CategoryAnalysis,
		Roles:       []AgentRole{RoleArchitect, RoleCoder},
		Complexity:  6,
		Permissions: []Permission{PermRead},
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Subdirectory to start traversal from"}
			}
		}`),
	}
}

func (t *TraversalTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	root, _ := os.Getwd()
	if input.Path != "" {
		root = filepath.Join(root, input.Path)
	}

	var results []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip common noise directories
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		// Add relative path to results
		rel, _ := filepath.Rel(root, path)
		results = append(results, rel)

		// Memory safety cap: don't return more than 500 files at once
		if len(results) > 500 {
			return fs.ErrInvalid // Or a specific signal to stop
		}

		return nil
	})

	if err != nil && err != fs.ErrInvalid {
		return &ToolResult{Status: "error", Error: err}, err
	}

	return &ToolResult{
		Status:  "success",
		Content: fmt.Sprintf("Found %d source files", len(results)),
		Data:    results,
	}, nil
}
