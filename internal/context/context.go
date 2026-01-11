package context

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

// ContextItem represents a granular unit of information.
type ContextItem struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`      // "file", "user_prompt", "agent_reply", "system_state"
	Frequency int       `json:"frequency"` // How often this item is requested/referenced
	LastUsed  time.Time `json:"last_used"`
	Pinned    bool      `json:"pinned"` // Critical info that never leaves the window
}

// Window manages the rolling context of information.
type Window struct {
	Items     map[string]*ContextItem
	MaxLength int // Max tokens or items (simplified as item count for now)
	mu        sync.RWMutex
}

func NewWindow(maxItems int) *Window {
	return &Window{
		Items:     make(map[string]*ContextItem),
		MaxLength: maxItems,
	}
}

// Add inserts or updates an item in the context window.
func (w *Window) Add(id, content, itemType string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if item, exists := w.Items[id]; exists {
		item.Frequency++
		item.LastUsed = time.Now()
		item.Content = content // Update content if it changed
		return
	}

	w.Items[id] = &ContextItem{
		ID:        id,
		Content:   content,
		Type:      itemType,
		Frequency: 1,
		LastUsed:  time.Now(),
	}

	w.prune()
}

// prune enforces the window size by removing ensuring least relevant items are dropped.
func (w *Window) prune() {
	if len(w.Items) <= w.MaxLength {
		return
	}

	type rankedItem struct {
		ID    string
		Score float64 // Higher is better
	}

	var ranked []rankedItem
	now := time.Now()

	for id, item := range w.Items {
		if item.Pinned {
			continue // Never prune pinned items
		}
		// Recency bias + Frequency weight
		hoursUnused := now.Sub(item.LastUsed).Hours()
		score := float64(item.Frequency) - (hoursUnused * 0.5)
		ranked = append(ranked, rankedItem{ID: id, Score: score})
	}

	// Sort by score ascending (lowest first)
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score < ranked[j].Score
	})

	// Remove items until we fit
	excess := len(w.Items) - w.MaxLength
	for i := 0; i < excess && i < len(ranked); i++ {
		delete(w.Items, ranked[i].ID)
	}
}

// GetContext returns the formatted context string, sorted by relevance.
func (w *Window) GetContext() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var activeItems []*ContextItem
	for _, item := range w.Items {
		activeItems = append(activeItems, item)
	}

	// Sort: Pinned first, then by recency/frequency
	sort.Slice(activeItems, func(i, j int) bool {
		if activeItems[i].Pinned != activeItems[j].Pinned {
			return activeItems[i].Pinned
		}
		return activeItems[i].LastUsed.After(activeItems[j].LastUsed)
	})

	var sb strings.Builder
	for _, item := range activeItems {
		sb.WriteString(fmt.Sprintf("[%s] (%s):\n%s\n---\n", item.Type, item.ID, item.Content))
	}
	return sb.String()
}

// Memory now wraps the Window system + DB persistence
type Memory struct {
	db     *sql.DB
	Window *Window
}

func NewMemory() *Memory {
	// ... (DB Init logic remains same) ...
	home, _ := os.UserHomeDir()
	dbDir := filepath.Join(home, ".vibeauracle")
	os.MkdirAll(dbDir, 0755)

	dbPath := filepath.Join(dbDir, "vibe.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return &Memory{Window: NewWindow(50)} // Safe fallback
	}

	// Initialize tables (same as before)
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

	return &Memory{
		db:     db,
		Window: NewWindow(50), // Standard context size
	}
}

// AddToWindow pushes content into the short-term rolling context.
func (m *Memory) AddToWindow(id, content, itemType string) {
	if m.Window != nil {
		m.Window.Add(id, content, itemType)
	}
}

// Store adds a fact or snippet to the long-term db memory.
func (m *Memory) Store(key string, value string) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := m.db.Exec("INSERT OR REPLACE INTO memory (key, value) VALUES (?, ?)", key, value)
	return err
}

// Recall retrieves relevant snippets from both short-term window and long-term DB.
func (m *Memory) Recall(query string) ([]string, error) {
	var results []string

	// 1. Get highly relevant short-term context
	if m.Window != nil {
		results = append(results, "--- Current Context Window ---")
		results = append(results, m.Window.GetContext())
	}

	// 2. Query long-term memory
	if m.db != nil {
		rows, err := m.db.Query("SELECT value FROM memory WHERE value LIKE ? LIMIT 5", "%"+query+"%")
		if err == nil {
			defer rows.Close()
			results = append(results, "--- Long-Term Memory ---")
			for rows.Next() {
				var s string
				if err := rows.Scan(&s); err == nil {
					results = append(results, s)
				}
			}
		}
	}

	return results, nil
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
