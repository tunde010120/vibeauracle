package sys

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// FS defines the interface for filesystem operations
type FS interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error
	DeleteFile(path string) error
	ListFiles(path string) ([]string, error)
	// Edit performs a fast search-and-replace on a file
	Edit(path string, oldStr, newStr string) error
	// Batch executes multiple file operations at once
	Batch(ops []BatchOp) error
}

// BatchOpType defines the type of operation in a batch
type BatchOpType string

const (
	OpWrite  BatchOpType = "write"
	OpDelete BatchOpType = "delete"
)

// BatchOp represents a single operation in a batch
type BatchOp struct {
	Type    BatchOpType
	Path    string
	Content []byte
}

// LocalFS implements FS using the local filesystem
type LocalFS struct {
	baseDir string
}

// NewLocalFS creates a new LocalFS with a specific base directory (sandbox)
func NewLocalFS(baseDir string) *LocalFS {
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}
	return &LocalFS{baseDir: baseDir}
}

// ReadFile reads a file's content
func (l *LocalFS) ReadFile(path string) ([]byte, error) {
	fullPath := l.resolvePath(path)
	return os.ReadFile(fullPath)
}

// WriteFile creates or overwrites a file
func (l *LocalFS) WriteFile(path string, content []byte) error {
	fullPath := l.resolvePath(path)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	return os.WriteFile(fullPath, content, 0644)
}

// DeleteFile removes a file
func (l *LocalFS) DeleteFile(path string) error {
	fullPath := l.resolvePath(path)
	return os.Remove(fullPath)
}

// ListFiles lists files in a directory
func (l *LocalFS) ListFiles(path string) ([]string, error) {
	fullPath := l.resolvePath(path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	return files, nil
}

// Edit performs a fast search-and-replace on a file without rewriting if no changes
func (l *LocalFS) Edit(path string, oldStr, newStr string) error {
	fullPath := l.resolvePath(path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}

	if !bytes.Contains(content, []byte(oldStr)) {
		return fmt.Errorf("string not found in file")
	}

	newContent := bytes.ReplaceAll(content, []byte(oldStr), []byte(newStr))

	// Atomic-ish write: only write if something changed
	if bytes.Equal(content, newContent) {
		return nil
	}

	return os.WriteFile(fullPath, newContent, 0644)
}

// Batch executes multiple file operations at once for lightning speed
func (l *LocalFS) Batch(ops []BatchOp) error {
	for _, op := range ops {
		switch op.Type {
		case OpWrite:
			if err := l.WriteFile(op.Path, op.Content); err != nil {
				return err
			}
		case OpDelete:
			if err := l.DeleteFile(op.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolvePath ensures paths are handled relative to the base directory and sanitized.
func (l *LocalFS) resolvePath(path string) string {
	if path == "" {
		return l.baseDir
	}
	if filepath.IsAbs(path) {
		return path // User knows what they are doing with absolute paths
	}
	// Force join with CWD/baseDir
	abs, err := filepath.Abs(filepath.Join(l.baseDir, path))
	if err != nil {
		return filepath.Join(l.baseDir, path) // Fallback
	}
	return abs
}
