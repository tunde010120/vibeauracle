package tooling

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type approvalDecision string

const (
	decisionAllow approvalDecision = "allow"
	decisionDeny  approvalDecision = "deny"
)

type approvalRecord struct {
	Decision  approvalDecision `json:"decision"`
	UpdatedAt time.Time        `json:"updated_at"`
	Count     int              `json:"count"`
}

// ApprovalStore persists allow/deny rules across runs.
// Stored as a single JSON file in the app data dir.
type ApprovalStore struct {
	path string
	mu   sync.Mutex
	m    map[string]approvalRecord
}

func NewApprovalStore(path string) (*ApprovalStore, error) {
	if path == "" {
		return nil, fmt.Errorf("approval store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating approvals dir: %w", err)
	}

	s := &ApprovalStore{path: path, m: map[string]approvalRecord{}}
	_ = s.load()
	return s, nil
}

func (s *ApprovalStore) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, &s.m)
}

func (s *ApprovalStore) save() error {
	b, err := json.MarshalIndent(s.m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0644)
}

func (s *ApprovalStore) Get(key string) (approvalRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[key]
	return rec, ok
}

func (s *ApprovalStore) Set(key string, decision approvalDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.m[key]
	rec.Decision = decision
	rec.UpdatedAt = time.Now()
	rec.Count++
	s.m[key] = rec
	return s.save()
}
