package auth

import (
	"fmt"
	"sync"
)

// Duration defines how long a permission lasts
type Duration string

const (
	DurationOnce      Duration = "once"
	DurationSession   Duration = "session"
	DurationPermanent Duration = "permanent"
)

// Decision represents the result of a permission check
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
	DecisionAsk   Decision = "ask"
)

// Action defines the type of operation being performed
type Action string

const (
	ActionFSRead    Action = "fs:read"
	ActionFSWrite   Action = "fs:write"
	ActionFSDelete  Action = "fs:delete"
	ActionShellExec Action = "shell:exec"
	ActionNetAccess Action = "net:access"
)

// Request represents a permission request
type Request struct {
	Action   Action
	Resource string
	Context  string // Additional info for the user
}

// Policy defines a rule for permissions
type Policy struct {
	Action   Action   `json:"action"`
	Resource string   `json:"resource"` // Can be a regex or glob in future
	Decision Decision `json:"decision"`
	Duration Duration `json:"duration"`
}

// Handler manages permissions and policies
type Handler struct {
	mu              sync.RWMutex
	policies        []Policy
	sessionGrants   map[string]Decision // key: action+resource
	permanentGrants map[string]Decision // managed via config later
}

// NewHandler creates a new permission handler
func NewHandler() *Handler {
	return &Handler{
		sessionGrants:   make(map[string]Decision),
		permanentGrants: make(map[string]Decision),
	}
}

// Check verifies if an action is permitted
func (h *Handler) Check(req Request) Decision {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := h.key(req.Action, req.Resource)

	// 1. Check session grants
	if decision, ok := h.sessionGrants[key]; ok {
		return decision
	}

	// 2. Check permanent grants
	if decision, ok := h.permanentGrants[key]; ok {
		return decision
	}

	// 3. Check static policies (if any)
	for _, p := range h.policies {
		if p.Action == req.Action && (p.Resource == "*" || p.Resource == req.Resource) {
			return p.Decision
		}
	}

	return DecisionAsk
}

// Grant records a user's permission decision
func (h *Handler) Grant(req Request, decision Decision, duration Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := h.key(req.Action, req.Resource)

	switch duration {
	case DurationOnce:
		// We don't store "once" grants, we just return the decision for that call
	case DurationSession:
		h.sessionGrants[key] = decision
	case DurationPermanent:
		h.permanentGrants[key] = decision
		// In a real app, this would be saved to the ~/.vibe auracle/config.yaml
	}
}

func (h *Handler) key(action Action, resource string) string {
	return fmt.Sprintf("%s:%s", action, resource)
}
