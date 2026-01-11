package context

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Memory handles long-term and short-term memory using SQLite
type Memory struct {
	db *sql.DB
}

func NewMemory() *Memory {
	home, _ := os.UserHomeDir()
	dbDir := filepath.Join(home, ".vibeauracle")
	os.MkdirAll(dbDir, 0755)
	
	dbPath := filepath.Join(dbDir, "vibe.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return &Memory{}
	}

	// Initialize tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memory (
			key TEXT PRIMARY KEY,
			value TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS app_state (
			id TEXT PRIMARY KEY,
			data TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		fmt.Printf("Error initializing database tables: %v\n", err)
	}

	return &Memory{db: db}
}

// Store adds a fact or snippet to the memory
func (m *Memory) Store(key string, value string) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := m.db.Exec("INSERT OR REPLACE INTO memory (key, value) VALUES (?, ?)", key, value)
	return err
}

// Recall retrieves relevant snippets from memory
func (m *Memory) Recall(query string) ([]string, error) {
	if m.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := m.db.Query("SELECT value FROM memory WHERE value LIKE ? LIMIT 5", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snippets []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			snippets = append(snippets, s)
		}
	}
	return snippets, nil
}

// SaveState persists arbitrary application state (JSON)
func (m *Memory) SaveState(id string, state interface{}) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = m.db.Exec("INSERT OR REPLACE INTO app_state (id, data) VALUES (?, ?)", id, string(data))
	return err
}

// LoadState retrieves persisted application state
func (m *Memory) LoadState(id string, target interface{}) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	var data string
	err := m.db.QueryRow("SELECT data FROM app_state WHERE id = ?", id).Scan(&data)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), target)
}

// ClearState removes a specific state entry
func (m *Memory) ClearState(id string) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := m.db.Exec("DELETE FROM app_state WHERE id = ?", id)
	return err
}

