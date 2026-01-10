package context

import (
	"fmt"
)

// Memory handles long-term and short-term memory
type Memory struct {
	// sqlite connection, etc.
}

func NewMemory() *Memory {
	return &Memory{}
}

// Store adds a fact or snippet to the memory
func (m *Memory) Store(key string, value string) error {
	fmt.Printf("Storing in context: %s -> %s\n", key, value)
	return nil
}

// Recall retrieves relevant snippets from memory
func (m *Memory) Recall(query string) ([]string, error) {
	fmt.Printf("Recalling from context for: %s\n", query)
	return []string{"relevant snippet 1", "relevant snippet 2"}, nil
}

