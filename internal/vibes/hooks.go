package vibes

import (
	"sync"
)

// HookHandler is a function that processes a hook event with context.
type HookHandler func(ctx *HookContext)

// HookContext provides data to hook handlers.
type HookContext struct {
	Hook     Hook
	Vibe     *Vibe
	Data     map[string]interface{}
	Cancel   bool // Set to true to prevent default behavior
	Response interface{}
}

// HookDispatcher manages hook subscriptions and dispatching.
type HookDispatcher struct {
	mu       sync.RWMutex
	handlers map[Hook][]HookHandler
	registry *Registry
}

// NewHookDispatcher creates a new hook dispatcher.
func NewHookDispatcher(registry *Registry) *HookDispatcher {
	return &HookDispatcher{
		handlers: make(map[Hook][]HookHandler),
		registry: registry,
	}
}

// RegisterHandler adds a handler for a specific hook.
func (hd *HookDispatcher) RegisterHandler(hook Hook, handler HookHandler) {
	hd.mu.Lock()
	defer hd.mu.Unlock()
	hd.handlers[hook] = append(hd.handlers[hook], handler)
}

// Dispatch triggers all handlers for a hook.
// Returns true if any handler set Cancel to true.
func (hd *HookDispatcher) Dispatch(hook Hook, data map[string]interface{}) bool {
	hd.mu.RLock()
	handlers := hd.handlers[hook]
	hd.mu.RUnlock()

	// Get all vibes attached to this hook
	vibes := hd.registry.ByHook(hook)

	cancelled := false

	// First, run vibe-specific handlers
	for _, vibe := range vibes {
		ctx := &HookContext{
			Hook:   hook,
			Vibe:   vibe,
			Data:   data,
			Cancel: false,
		}

		// Run any registered handlers
		for _, handler := range handlers {
			handler(ctx)
			if ctx.Cancel {
				cancelled = true
			}
		}
	}

	// Run global handlers (not tied to a specific vibe)
	for _, handler := range handlers {
		ctx := &HookContext{
			Hook:   hook,
			Vibe:   nil,
			Data:   data,
			Cancel: false,
		}
		handler(ctx)
		if ctx.Cancel {
			cancelled = true
		}
	}

	return cancelled
}

// DispatchAsync triggers handlers asynchronously.
func (hd *HookDispatcher) DispatchAsync(hook Hook, data map[string]interface{}) {
	go hd.Dispatch(hook, data)
}

// Connector represents a point where Vibes can plug into the system.
// Every "female connector" in the architecture exposes one or more hooks.
type Connector struct {
	Name        string
	Description string
	Hooks       []Hook
	Permissions []Permission
}

// DefaultConnectors returns the built-in system connectors.
func DefaultConnectors() []Connector {
	return []Connector{
		{
			Name:        "Lifecycle",
			Description: "Application startup and shutdown",
			Hooks:       []Hook{HookOnStartup, HookOnShutdown},
			Permissions: nil,
		},
		{
			Name:        "FileSystem",
			Description: "File watcher events",
			Hooks:       []Hook{HookOnFileChange},
			Permissions: []Permission{PermSystemFS},
		},
		{
			Name:        "Commands",
			Description: "CLI command execution",
			Hooks:       []Hook{HookOnCommand},
			Permissions: nil,
		},
		{
			Name:        "Tools",
			Description: "Agent tool execution",
			Hooks:       []Hook{HookOnToolCall},
			Permissions: []Permission{PermAgentTools},
		},
		{
			Name:        "Scheduler",
			Description: "Scheduled task execution",
			Hooks:       []Hook{HookOnSchedule},
			Permissions: []Permission{PermSchedulerCreate},
		},
		{
			Name:        "Config",
			Description: "Configuration changes",
			Hooks:       []Hook{HookOnConfigChange},
			Permissions: []Permission{PermConfigRead, PermConfigWrite},
		},
		{
			Name:        "Agent",
			Description: "AI model responses",
			Hooks:       []Hook{HookOnModelResponse},
			Permissions: []Permission{PermAgentPrompt},
		},
		{
			Name:        "Update",
			Description: "Update lifecycle",
			Hooks:       []Hook{HookOnUpdate},
			Permissions: []Permission{PermUpdateFrequency, PermUpdateChannel},
		},
	}
}
