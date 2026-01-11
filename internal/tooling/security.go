package tooling

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrBlockedAccess = errors.New("security: access to this resource is blocked by default")
)

// SecurityGuard manages access policies for tools.
type SecurityGuard struct {
	blockedPaths []string
	allowEnv     bool
	mu           sync.RWMutex
}

func NewSecurityGuard() *SecurityGuard {
	return &SecurityGuard{
		blockedPaths: []string{".env", ".key", "id_rsa", "credentials"},
	}
}

// SetAllowEnv allows or blocks access to environment/sensitive files for the current scope.
func (s *SecurityGuard) SetAllowEnv(allow bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowEnv = allow
}

// CheckPath verifies if a path is safe to access.
func (s *SecurityGuard) CheckPath(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.allowEnv {
		return nil
	}

	base := filepath.Base(path)
	for _, blocked := range s.blockedPaths {
		if strings.Contains(strings.ToLower(base), blocked) {
			return fmt.Errorf("%w: %s", ErrBlockedAccess, base)
		}
	}

	return nil
}

// SecureTool wraps a Tool with security checks.
type SecureTool struct {
	Tool
	guard *SecurityGuard
}

func WrapWithSecurity(t Tool, guard *SecurityGuard) Tool {
	return &SecureTool{Tool: t, guard: guard}
}

// Execute in SecureTool should ideally be overridden if we want to intercept specific arguments,
// but for now, we'll assume tools perform their own check using the guard provided in context or registry.
// Alternatively, we can inspect args if they contain "path" or "file" keys.

