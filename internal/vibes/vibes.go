// Package vibes provides the extension API for vibeauracle.
// Vibes are natural-language-powered, markdown-defined extensions that can
// modify any aspect of the system.
package vibes

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Hook represents a lifecycle event that Vibes can attach to.
type Hook string

const (
	HookOnStartup       Hook = "on_startup"
	HookOnShutdown      Hook = "on_shutdown"
	HookOnFileChange    Hook = "on_file_change"
	HookOnCommand       Hook = "on_command"
	HookOnToolCall      Hook = "on_tool_call"
	HookOnSchedule      Hook = "on_schedule"
	HookOnConfigChange  Hook = "on_config_change"
	HookOnModelResponse Hook = "on_model_response"
	HookOnUpdate        Hook = "on_update"
)

// Permission represents what a Vibe is allowed to access.
type Permission string

const (
	PermConfigRead      Permission = "config.read"
	PermConfigWrite     Permission = "config.write"
	PermUITheme         Permission = "ui.theme"
	PermUILayout        Permission = "ui.layout"
	PermSchedulerCreate Permission = "scheduler.create"
	PermSchedulerCancel Permission = "scheduler.cancel"
	PermAgentPrompt     Permission = "agent.prompt"
	PermAgentTools      Permission = "agent.tools"
	PermAgentLock       Permission = "agent.lock"
	PermUpdateFrequency Permission = "update.frequency"
	PermUpdateChannel   Permission = "update.channel"
	PermBinarySelfMod   Permission = "binary.self_modify"
	PermSystemShell     Permission = "system.shell"
	PermSystemFS        Permission = "system.fs"
	PermSandboxEscape   Permission = "sandbox.escape"
)

// ToolDefinition describes a custom tool a Vibe can register.
type ToolDefinition struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Parameters  map[string]ToolParameter `yaml:"parameters"`
	Action      string                   `yaml:"action"` // Shell command or script
}

// ToolParameter describes a parameter for a custom tool.
type ToolParameter struct {
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default,omitempty"`
}

// UIConfig holds UI customization settings.
type UIConfig struct {
	Theme  ThemeConfig  `yaml:"theme,omitempty"`
	Layout LayoutConfig `yaml:"layout,omitempty"`
}

// ThemeConfig defines color overrides.
type ThemeConfig struct {
	Primary    string `yaml:"primary,omitempty"`
	Secondary  string `yaml:"secondary,omitempty"`
	Accent     string `yaml:"accent,omitempty"`
	Background string `yaml:"background,omitempty"`
	Foreground string `yaml:"foreground,omitempty"`
	Success    string `yaml:"success,omitempty"`
	Warning    string `yaml:"warning,omitempty"`
	Error      string `yaml:"error,omitempty"`
}

// LayoutConfig defines UI layout overrides.
type LayoutConfig struct {
	Sidebar   string `yaml:"sidebar,omitempty"`    // left, right, hidden
	TreeWidth string `yaml:"tree_width,omitempty"` // percentage or px
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	RequirePassword bool   `yaml:"require_password,omitempty"`
	PasswordHash    string `yaml:"password_hash,omitempty"`
	LockAfter       string `yaml:"lock_after,omitempty"` // Duration string
}

// BinaryConfig holds binary self-modification settings.
type BinaryConfig struct {
	LDFlags   []string `yaml:"ldflags,omitempty"`
	RebuildOn string   `yaml:"rebuild_on,omitempty"`
}

// Spec is the parsed YAML front matter of a Vibe.
type Spec struct {
	Name         string           `yaml:"name"`
	Version      string           `yaml:"version"`
	Author       string           `yaml:"author,omitempty"`
	Description  string           `yaml:"description,omitempty"`
	Hooks        []Hook           `yaml:"hooks,omitempty"`
	Permissions  []Permission     `yaml:"permissions,omitempty"`
	Schedule     string           `yaml:"schedule,omitempty"`      // Cron expression
	ScheduleOnce string           `yaml:"schedule_once,omitempty"` // ISO 8601 timestamp
	Tools        []ToolDefinition `yaml:"tools,omitempty"`
	UI           UIConfig         `yaml:"ui,omitempty"`
	Security     SecurityConfig   `yaml:"security,omitempty"`
	Binary       BinaryConfig     `yaml:"binary,omitempty"`
}

// Vibe represents a loaded extension.
type Vibe struct {
	Spec         Spec
	Instructions string // The Markdown body (natural language instructions)
	FilePath     string
	Enabled      bool
}

// Parse reads a .vibe.md file and extracts the Spec and Instructions.
func Parse(path string) (*Vibe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading vibe file: %w", err)
	}

	// Split front matter from body
	frontMatter, body, err := splitFrontMatter(data)
	if err != nil {
		return nil, fmt.Errorf("parsing front matter: %w", err)
	}

	var spec Spec
	if err := yaml.Unmarshal(frontMatter, &spec); err != nil {
		return nil, fmt.Errorf("parsing YAML spec: %w", err)
	}

	return &Vibe{
		Spec:         spec,
		Instructions: string(body),
		FilePath:     path,
		Enabled:      true,
	}, nil
}

// splitFrontMatter separates the YAML front matter from the Markdown body.
func splitFrontMatter(data []byte) ([]byte, []byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var frontMatter bytes.Buffer
	var body bytes.Buffer
	inFrontMatter := false
	frontMatterDone := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 && strings.TrimSpace(line) == "---" {
			inFrontMatter = true
			continue
		}

		if inFrontMatter && strings.TrimSpace(line) == "---" {
			inFrontMatter = false
			frontMatterDone = true
			continue
		}

		if inFrontMatter {
			frontMatter.WriteString(line)
			frontMatter.WriteString("\n")
		} else if frontMatterDone {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	if !frontMatterDone {
		return nil, nil, fmt.Errorf("no valid front matter found")
	}

	return frontMatter.Bytes(), body.Bytes(), nil
}

// Registry manages all loaded Vibes.
type Registry struct {
	mu    sync.RWMutex
	vibes map[string]*Vibe
	dirs  []string
}

// NewRegistry creates a new Vibe registry.
func NewRegistry() *Registry {
	return &Registry{
		vibes: make(map[string]*Vibe),
		dirs:  make([]string, 0),
	}
}

// AddDirectory registers a directory to scan for Vibes.
func (r *Registry) AddDirectory(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirs = append(r.dirs, dir)
}

// Scan discovers and loads all Vibes from registered directories.
func (r *Registry) Scan() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, dir := range r.dirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			if d.IsDir() {
				return nil
			}

			if strings.HasSuffix(path, ".vibe.md") {
				vibe, err := Parse(path)
				if err != nil {
					// Log but don't fail entire scan
					fmt.Fprintf(os.Stderr, "Warning: failed to parse vibe %s: %v\n", path, err)
					return nil
				}
				r.vibes[vibe.Spec.Name] = vibe
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("scanning directory %s: %w", dir, err)
		}
	}

	return nil
}

// Get retrieves a Vibe by name.
func (r *Registry) Get(name string) (*Vibe, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.vibes[name]
	return v, ok
}

// List returns all loaded Vibes.
func (r *Registry) List() []*Vibe {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Vibe, 0, len(r.vibes))
	for _, v := range r.vibes {
		result = append(result, v)
	}
	return result
}

// ByHook returns all Vibes that are attached to a specific hook.
func (r *Registry) ByHook(hook Hook) []*Vibe {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Vibe
	for _, v := range r.vibes {
		if !v.Enabled {
			continue
		}
		for _, h := range v.Spec.Hooks {
			if h == hook {
				result = append(result, v)
				break
			}
		}
	}
	return result
}

// Enable enables a Vibe by name.
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.vibes[name]
	if !ok {
		return fmt.Errorf("vibe not found: %s", name)
	}
	v.Enabled = true
	return nil
}

// Disable disables a Vibe by name.
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.vibes[name]
	if !ok {
		return fmt.Errorf("vibe not found: %s", name)
	}
	v.Enabled = false
	return nil
}

// HasPermission checks if a Vibe has a specific permission.
func (v *Vibe) HasPermission(perm Permission) bool {
	for _, p := range v.Spec.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}
