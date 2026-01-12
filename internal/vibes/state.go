package vibes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State represents persistent state for a Vibe.
type State struct {
	VibeName    string                 `json:"vibe_name"`
	Enabled     bool                   `json:"enabled"`
	LastRun     *time.Time             `json:"last_run,omitempty"`
	RunCount    int                    `json:"run_count"`
	Data        map[string]interface{} `json:"data,omitempty"`
	ApprovedAt  *time.Time             `json:"approved_at,omitempty"`
	InstalledAt time.Time              `json:"installed_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// StateManager handles persistence of Vibe state.
type StateManager struct {
	mu       sync.RWMutex
	states   map[string]*State
	dataDir  string
	dirty    bool
	saveChan chan struct{}
}

// NewStateManager creates a new state manager.
func NewStateManager(dataDir string) *StateManager {
	sm := &StateManager{
		states:   make(map[string]*State),
		dataDir:  dataDir,
		saveChan: make(chan struct{}, 1),
	}

	// Load existing state
	sm.load()

	// Start background save loop
	go sm.saveLoop()

	return sm
}

// load reads state from disk.
func (sm *StateManager) load() {
	statePath := filepath.Join(sm.dataDir, "vibes_state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return // No state file yet
	}

	var states map[string]*State
	if err := json.Unmarshal(data, &states); err != nil {
		return
	}

	sm.mu.Lock()
	sm.states = states
	sm.mu.Unlock()
}

// saveLoop periodically saves dirty state to disk.
func (sm *StateManager) saveLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.saveIfDirty()
		case <-sm.saveChan:
			sm.saveIfDirty()
		}
	}
}

func (sm *StateManager) saveIfDirty() {
	sm.mu.Lock()
	if !sm.dirty {
		sm.mu.Unlock()
		return
	}
	sm.dirty = false
	stateCopy := make(map[string]*State)
	for k, v := range sm.states {
		stateCopy[k] = v
	}
	sm.mu.Unlock()

	data, err := json.MarshalIndent(stateCopy, "", "  ")
	if err != nil {
		return
	}

	statePath := filepath.Join(sm.dataDir, "vibes_state.json")
	os.WriteFile(statePath, data, 0644)
}

// ForceSave immediately saves state to disk.
func (sm *StateManager) ForceSave() {
	select {
	case sm.saveChan <- struct{}{}:
	default:
	}
}

// Get retrieves state for a Vibe.
func (sm *StateManager) Get(vibeName string) *State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.states[vibeName]
}

// GetOrCreate retrieves state or creates a new one.
func (sm *StateManager) GetOrCreate(vibeName string) *State {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if state, ok := sm.states[vibeName]; ok {
		return state
	}

	now := time.Now()
	state := &State{
		VibeName:    vibeName,
		Enabled:     true,
		Data:        make(map[string]interface{}),
		InstalledAt: now,
		UpdatedAt:   now,
	}
	sm.states[vibeName] = state
	sm.dirty = true
	return state
}

// SetEnabled updates the enabled state.
func (sm *StateManager) SetEnabled(vibeName string, enabled bool) {
	state := sm.GetOrCreate(vibeName)
	sm.mu.Lock()
	state.Enabled = enabled
	state.UpdatedAt = time.Now()
	sm.dirty = true
	sm.mu.Unlock()
}

// RecordRun updates the last run time and count.
func (sm *StateManager) RecordRun(vibeName string) {
	state := sm.GetOrCreate(vibeName)
	sm.mu.Lock()
	now := time.Now()
	state.LastRun = &now
	state.RunCount++
	state.UpdatedAt = now
	sm.dirty = true
	sm.mu.Unlock()
}

// SetData stores arbitrary data for a Vibe.
func (sm *StateManager) SetData(vibeName, key string, value interface{}) {
	state := sm.GetOrCreate(vibeName)
	sm.mu.Lock()
	if state.Data == nil {
		state.Data = make(map[string]interface{})
	}
	state.Data[key] = value
	state.UpdatedAt = time.Now()
	sm.dirty = true
	sm.mu.Unlock()
}

// GetData retrieves arbitrary data for a Vibe.
func (sm *StateManager) GetData(vibeName, key string) (interface{}, bool) {
	state := sm.Get(vibeName)
	if state == nil || state.Data == nil {
		return nil, false
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := state.Data[key]
	return val, ok
}

// RecordApproval marks a Vibe as approved.
func (sm *StateManager) RecordApproval(vibeName string) {
	state := sm.GetOrCreate(vibeName)
	sm.mu.Lock()
	now := time.Now()
	state.ApprovedAt = &now
	state.UpdatedAt = now
	sm.dirty = true
	sm.mu.Unlock()
}

// IsApprovedRecently checks if a Vibe was approved within a duration.
func (sm *StateManager) IsApprovedRecently(vibeName string, within time.Duration) bool {
	state := sm.Get(vibeName)
	if state == nil || state.ApprovedAt == nil {
		return false
	}
	return time.Since(*state.ApprovedAt) < within
}

// Delete removes state for a Vibe.
func (sm *StateManager) Delete(vibeName string) {
	sm.mu.Lock()
	delete(sm.states, vibeName)
	sm.dirty = true
	sm.mu.Unlock()
}

// Stats returns aggregate statistics.
func (sm *StateManager) Stats() (total, enabled, disabled int) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, state := range sm.states {
		total++
		if state.Enabled {
			enabled++
		} else {
			disabled++
		}
	}
	return
}

// Export returns all state as JSON.
func (sm *StateManager) Export() ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return json.MarshalIndent(sm.states, "", "  ")
}

// Import loads state from JSON.
func (sm *StateManager) Import(data []byte) error {
	var states map[string]*State
	if err := json.Unmarshal(data, &states); err != nil {
		return fmt.Errorf("invalid state format: %w", err)
	}

	sm.mu.Lock()
	sm.states = states
	sm.dirty = true
	sm.mu.Unlock()

	return nil
}
