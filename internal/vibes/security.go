package vibes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// SecurityManager handles authentication, permissions, and sandboxing.
type SecurityManager struct {
	mu            sync.RWMutex
	locked        bool
	passwordHash  string
	lockAfter     time.Duration
	lastActivity  time.Time
	approvedPerms map[string]map[Permission]bool // vibe name -> approved permissions
	lockTimer     *time.Timer
}

// NewSecurityManager creates a new security manager.
func NewSecurityManager() *SecurityManager {
	return &SecurityManager{
		approvedPerms: make(map[string]map[Permission]bool),
		lastActivity:  time.Now(),
	}
}

// SetPassword sets the password hash for agent locking.
func (sm *SecurityManager) SetPassword(password string) {
	hash := sha256.Sum256([]byte(password))
	sm.mu.Lock()
	sm.passwordHash = "sha256:" + hex.EncodeToString(hash[:])
	sm.mu.Unlock()
}

// SetLockAfter configures auto-lock duration.
func (sm *SecurityManager) SetLockAfter(d time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lockAfter = d
	sm.resetLockTimer()
}

func (sm *SecurityManager) resetLockTimer() {
	if sm.lockTimer != nil {
		sm.lockTimer.Stop()
	}

	if sm.lockAfter > 0 && sm.passwordHash != "" {
		sm.lockTimer = time.AfterFunc(sm.lockAfter, func() {
			sm.Lock()
		})
	}
}

// RecordActivity updates the last activity time and resets the lock timer.
func (sm *SecurityManager) RecordActivity() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastActivity = time.Now()
	sm.resetLockTimer()
}

// Lock locks the agent.
func (sm *SecurityManager) Lock() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.passwordHash != "" {
		sm.locked = true
	}
}

// Unlock unlocks the agent with the correct password.
func (sm *SecurityManager) Unlock(password string) error {
	hash := sha256.Sum256([]byte(password))
	attempt := "sha256:" + hex.EncodeToString(hash[:])

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.passwordHash != attempt {
		return fmt.Errorf("incorrect password")
	}

	sm.locked = false
	sm.lastActivity = time.Now()
	sm.resetLockTimer()
	return nil
}

// IsLocked returns whether the agent is currently locked.
func (sm *SecurityManager) IsLocked() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.locked
}

// RequiresPassword returns whether a password is configured.
func (sm *SecurityManager) RequiresPassword() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.passwordHash != ""
}

// ApprovePermission grants a permission to a Vibe.
func (sm *SecurityManager) ApprovePermission(vibeName string, perm Permission) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.approvedPerms[vibeName] == nil {
		sm.approvedPerms[vibeName] = make(map[Permission]bool)
	}
	sm.approvedPerms[vibeName][perm] = true
}

// RevokePermission removes a permission from a Vibe.
func (sm *SecurityManager) RevokePermission(vibeName string, perm Permission) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.approvedPerms[vibeName] != nil {
		delete(sm.approvedPerms[vibeName], perm)
	}
}

// IsApproved checks if a Vibe has an approved permission.
func (sm *SecurityManager) IsApproved(vibeName string, perm Permission) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.approvedPerms[vibeName] == nil {
		return false
	}
	return sm.approvedPerms[vibeName][perm]
}

// CheckPermission validates that a Vibe can use a permission.
// Returns an error if the permission requires approval and hasn't been granted.
func (sm *SecurityManager) CheckPermission(vibe *Vibe, perm Permission) error {
	if !vibe.HasPermission(perm) {
		return fmt.Errorf("vibe %s does not declare permission %s", vibe.Spec.Name, perm)
	}

	// Sensitive permissions require explicit approval
	if isSensitive(perm) && !sm.IsApproved(vibe.Spec.Name, perm) {
		return fmt.Errorf("permission %s requires approval for vibe %s", perm, vibe.Spec.Name)
	}

	return nil
}

// isSensitive returns true for permissions that require explicit approval.
func isSensitive(perm Permission) bool {
	switch perm {
	case PermBinarySelfMod, PermSandboxEscape, PermSystemShell, PermAgentLock:
		return true
	default:
		return false
	}
}

// SensitivePermissions returns the list of permissions that require approval.
func SensitivePermissions() []Permission {
	return []Permission{
		PermBinarySelfMod,
		PermSandboxEscape,
		PermSystemShell,
		PermAgentLock,
	}
}
