package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
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
	audit *AuditLogger

	mu           sync.Mutex
	sessionAllow map[string]bool
	sessionDeny  map[string]bool
}

func NewEnclave(appDataDir string) (*Enclave, error) {
	storePath := filepath.Join(appDataDir, "enclave", "approvals.json")
	auditPath := filepath.Join(appDataDir, "enclave", "audit.log")

	// Ensure dir exists
	os.MkdirAll(filepath.Dir(storePath), 0755)

	s, err := NewApprovalStore(storePath)
	if err != nil {
		return nil, err
	}
	return &Enclave{
		store:        s,
		audit:        NewAuditLogger(auditPath),
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
		e.audit.Log(req.ToolName, args, risk, "Blocked", resolveScope(args))
		return false, fmt.Errorf("security: blocked action: %s", req.Summary)
	}

	scope := resolveScope(args)

	// Session checks
	e.mu.Lock()
	if e.sessionDeny[key] {
		e.mu.Unlock()
		e.audit.Log(req.ToolName, args, risk, "Denied (Session)", scope)
		return false, fmt.Errorf("security: denied for session: %s", req.Summary)
	}
	if e.sessionAllow[key] {
		e.mu.Unlock()
		e.audit.Log(req.ToolName, args, risk, "Approved (Session)", scope)
		return true, nil
	}
	e.mu.Unlock()

	// Persisted checks
	if rec, ok := e.store.Get(key); ok {
		switch rec.Decision {
		case decisionAllow:
			e.audit.Log(req.ToolName, args, risk, "Approved (Persisted)", scope)
			return true, nil
		case decisionDeny:
			e.audit.Log(req.ToolName, args, risk, "Denied (Persisted)", scope)
			return false, fmt.Errorf("security: denied (persisted): %s", req.Summary)
		}
	}

	// Create resumption closure
	resumeFunc := func(choice string) (*ToolResult, error) {
		switch choice {
		case "Approve Once":
			e.audit.Log(req.ToolName, args, risk, "Approved (Once)", scope)
			return tool.Execute(context.TODO(), args) // Execute directly
		case "Approve Session":
			e.ApproveSession(key)
			e.audit.Log(req.ToolName, args, risk, "Approved (Session)", scope)
			return tool.Execute(context.TODO(), args)
		case "Approve Forever":
			e.ApproveForever(key)
			e.audit.Log(req.ToolName, args, risk, "Approved (Forever)", scope)
			return tool.Execute(context.TODO(), args)
		default:
			e.audit.Log(req.ToolName, args, risk, "Denied (User)", scope)
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

// --- Audit Logging ---

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Tool      string `json:"tool"`
	Args      string `json:"args"`
	Risk      string `json:"risk"`
	Decision  string `json:"decision"` // Approved, Denied
	Scope     string `json:"scope"`    // Local, System
}

// AuditLogger maintains a secure ledger of all agent actions
type AuditLogger struct {
	path string
	mu   sync.Mutex
}

func NewAuditLogger(path string) *AuditLogger {
	return &AuditLogger{path: path}
}

func (l *AuditLogger) Log(tool string, args json.RawMessage, risk, decision, scope string) {
	entry := AuditEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Tool:      tool,
		Args:      stableJSON(args),
		Risk:      risk,
		Decision:  decision,
		Scope:     scope,
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // 0600 = Secure
	if err == nil {
		bytes, _ := json.Marshal(entry)
		f.WriteString(string(bytes) + "\n")
		f.Close()
	}
}

// --- Scoped Security ---

// resolveScope determines if an operation is Local (safe-ish) or System (dangerous)
func resolveScope(args json.RawMessage) string {
	s := string(args)
	// Heuristic: absolute paths outside CWD are system.
	// This is imperfect but a good baseline.
	if strings.Contains(s, "\"/etc/") || strings.Contains(s, "\"/usr/") || strings.Contains(s, "\"/var/") {
		return "System"
	}
	cwd, _ := os.Getwd()
	if strings.Contains(s, cwd) {
		return "Local"
	}
	// Default to system if ambiguous for safety, or Local if it looks like a relative path
	// For now, let's assume Local unless it looks like a root path
	if strings.Contains(s, "\"/") && !strings.Contains(s, "\"/home/") {
		return "System"
	}
	return "Local"
}
