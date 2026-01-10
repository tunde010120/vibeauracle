package sys

import (
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

// resolvePath ensures paths are handled relative to the base directory
func (l *LocalFS) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(l.baseDir, path)
}

