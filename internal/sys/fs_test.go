package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibeaura-fs-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)
	testFile := "test.txt"
	content := []byte("hello vibeaura")

	// Test Write
	if err := fs.WriteFile(testFile, content); err != nil {
		t.Errorf("WriteFile failed: %v", err)
	}

	// Test Read
	got, err := fs.ReadFile(testFile)
	if err != nil {
		t.Errorf("ReadFile failed: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", got, content)
	}

	// Test List
	files, err := fs.ListFiles(".")
	if err != nil {
		t.Errorf("ListFiles failed: %v", err)
	}
	found := false
	for _, f := range files {
		if f == testFile {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("file %q not found in list %v", testFile, files)
	}

	// Test Delete
	if err := fs.DeleteFile(testFile); err != nil {
		t.Errorf("DeleteFile failed: %v", err)
	}

	// Verify Delete
	if _, err := fs.ReadFile(testFile); err == nil {
		t.Error("file still exists after deletion")
	}
}

func TestLocalFS_Subdir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibeaura-fs-test-subdir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)
	testFile := filepath.Join("subdir", "nested.txt")
	content := []byte("nested content")

	if err := fs.WriteFile(testFile, content); err != nil {
		t.Errorf("WriteFile in subdir failed: %v", err)
	}

	got, err := fs.ReadFile(testFile)
	if err != nil {
		t.Errorf("ReadFile in subdir failed: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

