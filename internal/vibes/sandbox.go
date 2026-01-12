package vibes

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Sandbox provides isolated execution for Vibe actions.
type Sandbox struct {
	vibe        *Vibe
	timeout     time.Duration
	maxMemory   int64 // bytes
	allowedEnv  []string
	blockedCmds []string
	workDir     string
}

// SandboxConfig configures sandbox behavior.
type SandboxConfig struct {
	Timeout     time.Duration
	MaxMemory   int64
	AllowedEnv  []string
	BlockedCmds []string
	WorkDir     string
}

// DefaultSandboxConfig returns sensible defaults.
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		Timeout:     30 * time.Second,
		MaxMemory:   256 * 1024 * 1024, // 256MB
		AllowedEnv:  []string{"PATH", "HOME", "USER", "TERM"},
		BlockedCmds: []string{"rm", "sudo", "su", "dd", "mkfs", "fdisk", "shutdown", "reboot"},
		WorkDir:     "",
	}
}

// NewSandbox creates a new sandbox for a Vibe.
func NewSandbox(vibe *Vibe, config *SandboxConfig) *Sandbox {
	if config == nil {
		config = DefaultSandboxConfig()
	}

	return &Sandbox{
		vibe:        vibe,
		timeout:     config.Timeout,
		maxMemory:   config.MaxMemory,
		allowedEnv:  config.AllowedEnv,
		blockedCmds: config.BlockedCmds,
		workDir:     config.WorkDir,
	}
}

// Execute runs a shell command in the sandbox.
func (s *Sandbox) Execute(cmd string) (string, error) {
	// Check for blocked commands
	if s.isBlocked(cmd) {
		return "", fmt.Errorf("command blocked by sandbox policy")
	}

	// Check permissions
	if !s.vibe.HasPermission(PermSystemShell) && !s.vibe.HasPermission(PermSandboxEscape) {
		return "", fmt.Errorf("vibe lacks permission for shell execution")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	shell := exec.CommandContext(ctx, "sh", "-c", cmd)
	if s.workDir != "" {
		shell.Dir = s.workDir
	}

	// Restrict environment
	shell.Env = s.filteredEnv()

	output, err := shell.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %v", s.timeout)
	}

	return string(output), err
}

func (s *Sandbox) isBlocked(cmd string) bool {
	// Skip blocking if vibe has sandbox escape
	if s.vibe.HasPermission(PermSandboxEscape) {
		return false
	}

	cmdLower := strings.ToLower(cmd)
	for _, blocked := range s.blockedCmds {
		if strings.Contains(cmdLower, blocked) {
			return true
		}
	}
	return false
}

func (s *Sandbox) filteredEnv() []string {
	// If sandbox escape, allow all env
	if s.vibe.HasPermission(PermSandboxEscape) {
		return nil // nil means inherit all
	}

	var filtered []string
	for _, key := range s.allowedEnv {
		filtered = append(filtered, key+"="+getEnv(key))
	}
	return filtered
}

func getEnv(key string) string {
	// This would normally use os.Getenv
	// Simplified for now
	return ""
}

// Executor manages sandboxed execution across all Vibes.
type Executor struct {
	mu        sync.RWMutex
	sandboxes map[string]*Sandbox
	config    *SandboxConfig
	logger    *Logger
	telemetry *Telemetry
	security  *SecurityManager
}

// NewExecutor creates a new Vibe executor.
func NewExecutor(logger *Logger, telemetry *Telemetry, security *SecurityManager) *Executor {
	return &Executor{
		sandboxes: make(map[string]*Sandbox),
		config:    DefaultSandboxConfig(),
		logger:    logger,
		telemetry: telemetry,
		security:  security,
	}
}

// SetConfig updates the sandbox configuration.
func (e *Executor) SetConfig(config *SandboxConfig) {
	e.mu.Lock()
	e.config = config
	e.mu.Unlock()
}

// GetSandbox returns or creates a sandbox for a Vibe.
func (e *Executor) GetSandbox(vibe *Vibe) *Sandbox {
	e.mu.Lock()
	defer e.mu.Unlock()

	if sb, ok := e.sandboxes[vibe.Spec.Name]; ok {
		return sb
	}

	sb := NewSandbox(vibe, e.config)
	e.sandboxes[vibe.Spec.Name] = sb
	return sb
}

// ExecuteAction runs a tool action for a Vibe.
func (e *Executor) ExecuteAction(vibe *Vibe, action string) (string, error) {
	// Check if agent is locked
	if e.security.IsLocked() {
		return "", fmt.Errorf("agent is locked")
	}

	// Record activity
	e.security.RecordActivity()

	start := time.Now()
	sandbox := e.GetSandbox(vibe)

	e.logger.Log(LogDebug, vibe.Spec.Name, fmt.Sprintf("Executing action: %s", truncate(action, 50)))

	output, err := sandbox.Execute(action)
	duration := time.Since(start)

	if err != nil {
		e.logger.LogError(vibe.Spec.Name, "", err)
		e.telemetry.RecordFailure(vibe.Spec.Name, duration, err)
		return output, err
	}

	e.logger.LogHook(LogInfo, vibe.Spec.Name, "", "Action completed", duration)
	e.telemetry.RecordSuccess(vibe.Spec.Name, duration)

	return output, nil
}

// ExecuteTool runs a custom tool defined by a Vibe.
func (e *Executor) ExecuteTool(vibe *Vibe, tool ToolDefinition, params map[string]string) (string, error) {
	// Substitute parameters in action
	action := tool.Action
	for key, value := range params {
		action = strings.ReplaceAll(action, "${"+key+"}", value)
	}

	return e.ExecuteAction(vibe, action)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// RecoveryHandler handles Vibe execution failures.
type RecoveryHandler struct {
	maxRetries  int
	retryDelay  time.Duration
	failedVibes map[string]int
	mu          sync.Mutex
}

// NewRecoveryHandler creates a new recovery handler.
func NewRecoveryHandler(maxRetries int, retryDelay time.Duration) *RecoveryHandler {
	return &RecoveryHandler{
		maxRetries:  maxRetries,
		retryDelay:  retryDelay,
		failedVibes: make(map[string]int),
	}
}

// RecordFailure records a Vibe failure.
func (rh *RecoveryHandler) RecordFailure(vibeName string) bool {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	rh.failedVibes[vibeName]++
	return rh.failedVibes[vibeName] <= rh.maxRetries
}

// RecordSuccess clears failure count for a Vibe.
func (rh *RecoveryHandler) RecordSuccess(vibeName string) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	delete(rh.failedVibes, vibeName)
}

// GetRetryDelay returns the delay before retrying.
func (rh *RecoveryHandler) GetRetryDelay(vibeName string) time.Duration {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	failures := rh.failedVibes[vibeName]
	// Exponential backoff
	return rh.retryDelay * time.Duration(1<<failures)
}

// IsDisabled checks if a Vibe has exceeded max retries.
func (rh *RecoveryHandler) IsDisabled(vibeName string) bool {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	return rh.failedVibes[vibeName] > rh.maxRetries
}
