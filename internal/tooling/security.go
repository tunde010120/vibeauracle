package tooling

import (
	"context"
	"encoding/json"
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
	blockedPaths    []string
	allowEnv        bool
	autoApproveRead bool

	// Policy-based controls
	allowedPermissions map[Permission]bool
	deniedPermissions  map[Permission]bool

	interceptor func(tool Tool, args json.RawMessage) (bool, error)
	mu          sync.RWMutex
}

func NewSecurityGuard() *SecurityGuard {
	return &SecurityGuard{
		blockedPaths:    []string{".env", ".key", "id_rsa", "credentials", "id_ed25519"},
		autoApproveRead: true,
		allowedPermissions: map[Permission]bool{
			PermRead: true,
		},
		deniedPermissions: make(map[Permission]bool),
	}
}

// SetAllowEnv allows or blocks access to environment/sensitive files for the current scope.
func (s *SecurityGuard) SetAllowEnv(allow bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowEnv = allow
}

// SetInterceptor installs a manual authorization hook.
// The interceptor can return (false, *NeedsApprovalError) to request user input.
func (s *SecurityGuard) SetInterceptor(fn func(tool Tool, args json.RawMessage) (bool, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interceptor = fn
}

// SetPermissionPolicy sets whether a specific permission is globally allowed or denied.
func (s *SecurityGuard) SetPermissionPolicy(p Permission, allowed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if allowed {
		s.allowedPermissions[p] = true
		delete(s.deniedPermissions, p)
	} else {
		s.deniedPermissions[p] = true
		delete(s.allowedPermissions, p)
	}
}

// ValidateRequest checks if a tool execution is allowed based on its permissions and arguments.
func (s *SecurityGuard) ValidateRequest(t Tool, args json.RawMessage) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := t.Metadata()
	perms := m.Permissions
	requiresManualApproval := false

	for _, p := range perms {
		// 1. Check if explicitly denied
		if s.deniedPermissions[p] {
			return fmt.Errorf("%w: permission %s is explicitly denied", ErrBlockedAccess, p)
		}

		// 2. Sensitive data check
		if p == PermSensitive && !s.allowEnv {
			return fmt.Errorf("%w: sensitive data access is disabled", ErrBlockedAccess)
		}

		// 3. Check if NOT explicitly allowed
		if !s.allowedPermissions[p] {
			requiresManualApproval = true
		}
	}

	// If all permissions are allowed, we're good
	if !requiresManualApproval {
		return nil
	}

	// If we need manual approval and have an interceptor, use it
	if s.interceptor != nil {
		approved, err := s.interceptor(t, args)
		if err != nil {
			return err
		}
		if !approved {
			return errors.New("security: user declined the operation")
		}
		return nil
	}

	// If no interceptor and not explicitly allowed, block for safety
	return fmt.Errorf("security: operation requires manual authorization for permissions %v", perms)
}

// CheckPath verifies if a path is safe to access (remains for compatibility or internal checks).
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

// Execute performs security validation before delegating to the underlying Tool.
func (st *SecureTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if err := st.guard.ValidateRequest(st.Tool, args); err != nil {
		return &ToolResult{Status: "error", Error: err}, err
	}
	return st.Tool.Execute(ctx, args)
}
