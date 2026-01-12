package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// InterventionError is returned when a tool needs user selection/approval.
// The UI should render the choices and then call Resume(selectedOption).
type InterventionError struct {
	Title   string
	Choices []string
	Resume  func(choice string) (*ToolResult, error)
}

func (e *InterventionError) Error() string {
	return "intervention required: " + e.Title
}

// Enclave provides a secure policy layer for tool execution.
// It supports approvals scoped to: once (caller-handled), session, or forever (persisted).
type Enclave struct {
	store *ApprovalStore

	mu           sync.Mutex
	sessionAllow map[string]bool
	sessionDeny  map[string]bool
}

func NewEnclave(appDataDir string) (*Enclave, error) {
	storePath := filepath.Join(appDataDir, "enclave", "approvals.json")
	s, err := NewApprovalStore(storePath)
	if err != nil {
		return nil, err
	}
	return &Enclave{
		store:        s,
		sessionAllow: map[string]bool{},
		sessionDeny:  map[string]bool{},
	}, nil
}

// ApproveSession allows a request key for the rest of the current session.
func (e *Enclave) ApproveSession(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionAllow[key] = true
	delete(e.sessionDeny, key)
}

// DenySession denies a request key for the rest of the current session.
func (e *Enclave) DenySession(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionDeny[key] = true
	delete(e.sessionAllow, key)
}

// ApproveForever persists an allow decision.
func (e *Enclave) ApproveForever(key string) error {
	return e.store.Set(key, decisionAllow)
}

// DenyForever persists a deny decision.
func (e *Enclave) DenyForever(key string) error {
	return e.store.Set(key, decisionDeny)
}

// Interceptor is meant to be installed into SecurityGuard.SetInterceptor.
// It returns true if approved; otherwise returns a NeedsApprovalError.
func (e *Enclave) Interceptor(tool Tool, args json.RawMessage) (bool, error) {
	// Normalize and build a stable key.
	key, req, risk, err := buildApprovalRequest(tool, args)
	if err != nil {
		return false, err
	}
	req.Key = key
	req.Risk = risk

	// Hard-block rules
	if risk == "blocked" {
		return false, fmt.Errorf("security: blocked action: %s", req.Summary)
	}

	// Session checks
	e.mu.Lock()
	if e.sessionDeny[key] {
		e.mu.Unlock()
		return false, fmt.Errorf("security: denied for session: %s", req.Summary)
	}
	if e.sessionAllow[key] {
		e.mu.Unlock()
		return true, nil
	}
	e.mu.Unlock()

	// Persisted checks
	if rec, ok := e.store.Get(key); ok {
		switch rec.Decision {
		case decisionAllow:
			return true, nil
		case decisionDeny:
			return false, fmt.Errorf("security: denied (persisted): %s", req.Summary)
		}
	}

	// Create resumption closure
	resumeFunc := func(choice string) (*ToolResult, error) {
		switch choice {
		case "Approve Once":
			return tool.Execute(context.TODO(), args) // Execute directly
		case "Approve Session":
			e.ApproveSession(key)
			return tool.Execute(context.TODO(), args)
		case "Approve Forever":
			e.ApproveForever(key)
			return tool.Execute(context.TODO(), args)
		default:
			return nil, fmt.Errorf("security: user denied %s", req.Summary)
		}
	}

	return false, &InterventionError{
		Title:   fmt.Sprintf("Allow action? %s", req.Summary),
		Choices: []string{"Approve Once", "Approve Session", "Approve Forever", "Deny"},
		Resume:  resumeFunc,
	}
}

// buildApprovalRequest inspects a tool call and returns a stable key and description.
func buildApprovalRequest(tool Tool, args json.RawMessage) (string, ApprovalRequest, string, error) {
	m := tool.Metadata()
	name := m.Name
	req := ApprovalRequest{ToolName: name}

	// Default risk based on permissions
	risk := "medium"
	for _, p := range m.Permissions {
		switch p {
		case PermRead:
			// keep low unless mixed with others
			if risk == "medium" {
				risk = "low"
			}
		case PermNetwork:
			risk = "medium"
		case PermWrite, PermExecute, PermSensitive:
			risk = "high"
		}
	}

	// Tool-specific formatting and sanitization
	summary := name
	preview := string(args)
	if len(preview) > 180 {
		preview = preview[:180] + "…"
	}

	key := name + ":" + stableJSON(args)

	if name == "sys_shell_exec" {
		var input struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", ApprovalRequest{}, "", err
		}
		cmdline := strings.TrimSpace(input.Command + " " + strings.Join(input.Args, " "))
		summary = "exec: " + cmdline
		preview = cmdline
		key = "sys_shell_exec:" + normalizeCmdKey(input.Command, input.Args)

		// Sanitization: block truly dangerous commands.
		if r := commandRisk(input.Command, input.Args); r == "blocked" {
			risk = "blocked"
		}
	}

	req.Summary = summary
	req.ArgsPreview = preview
	return key, req, risk, nil
}

func stableJSON(b json.RawMessage) string {
	// Already stable enough for our use; callers pass struct->json with stable key order usually.
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "{}"
	}
	return s
}

func normalizeCmdKey(command string, args []string) string {
	c := strings.ToLower(strings.TrimSpace(command))
	parts := []string{c}
	for _, a := range args {
		parts = append(parts, strings.TrimSpace(a))
	}
	return strings.Join(parts, "\u0000")
}

var dangerousExact = map[string]bool{
	"mkfs":      true,
	"mkfs.ext4": true,
	"mkfs.xfs":  true,
	"dd":        true,
	"shutdown":  true,
	"reboot":    true,
	"poweroff":  true,
}

func commandRisk(command string, args []string) string {
	c := strings.ToLower(strings.TrimSpace(command))
	if dangerousExact[c] {
		return "blocked"
	}

	// Block shells that execute arbitrary strings.
	if (c == "sh" || c == "bash" || c == "zsh") && contains(args, "-c") {
		return "blocked"
	}

	// Block curl|sh patterns.
	joined := strings.ToLower(strings.Join(args, " "))
	if strings.Contains(joined, "| sh") || strings.Contains(joined, "|bash") {
		return "blocked"
	}

	// Block rm -rf / (and close variants)
	if c == "rm" {
		if contains(args, "-rf") || contains(args, "-fr") {
			for _, a := range args {
				if strings.TrimSpace(a) == "/" {
					return "blocked"
				}
			}
		}
	}

	// Block writing raw to block devices
	if c == "dd" {
		// dd is already blocked above, but keep defense-in-depth
		return "blocked"
	}

	// Detect obvious device paths
	devRe := regexp.MustCompile(`^/dev/(sd|nvme|mmcblk|loop)`) // conservative
	for _, a := range args {
		if devRe.MatchString(strings.TrimSpace(a)) {
			return "blocked"
		}
	}

	return "ok"
}

func contains(xs []string, target string) bool {
	for _, x := range xs {
		if strings.TrimSpace(x) == target {
			return true
		}
	}
	return false
}

// Ensure Enclave can be used where context is needed (future).
var _ = context.Background
