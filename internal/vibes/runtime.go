package vibes

import (
	"os"
	"path/filepath"
	"time"
)

// Runtime is the central orchestrator for the Vibes extension system.
// It ties together the registry, scheduler, hooks, and security.
type Runtime struct {
	Registry   *Registry
	Scheduler  *Scheduler
	Dispatcher *HookDispatcher
	Security   *SecurityManager
	DataDir    string
}

// NewRuntime creates a fully initialized Vibes runtime.
func NewRuntime(dataDir string) (*Runtime, error) {
	vibesDir := filepath.Join(dataDir, "vibes")
	if err := os.MkdirAll(vibesDir, 0755); err != nil {
		return nil, err
	}

	registry := NewRegistry()
	registry.AddDirectory(vibesDir)

	runtime := &Runtime{
		Registry:   registry,
		Scheduler:  NewScheduler(),
		Dispatcher: NewHookDispatcher(registry),
		Security:   NewSecurityManager(),
		DataDir:    dataDir,
	}

	return runtime, nil
}

// Start initializes the runtime and activates all Vibes.
func (r *Runtime) Start() error {
	// Scan for vibes
	if err := r.Registry.Scan(); err != nil {
		return err
	}

	// Start the scheduler
	r.Scheduler.Start()

	// Schedule vibes with cron expressions
	for _, vibe := range r.Registry.List() {
		if vibe.Spec.Schedule != "" {
			v := vibe // Capture for closure
			_, err := r.Scheduler.Schedule(v.Spec.Name, v.Spec.Schedule, func() {
				r.Dispatcher.Dispatch(HookOnSchedule, map[string]interface{}{
					"vibe": v,
				})
			})
			if err != nil {
				// Log but don't fail
			}
		}

		if vibe.Spec.ScheduleOnce != "" {
			v := vibe
			t, err := time.Parse(time.RFC3339, v.Spec.ScheduleOnce)
			if err == nil {
				r.Scheduler.ScheduleOnce(v.Spec.Name, t, func() {
					r.Dispatcher.Dispatch(HookOnSchedule, map[string]interface{}{
						"vibe": v,
					})
				})
			}
		}
	}

	// Dispatch startup hook
	r.Dispatcher.Dispatch(HookOnStartup, nil)

	return nil
}

// Stop gracefully shuts down the runtime.
func (r *Runtime) Stop() {
	r.Dispatcher.Dispatch(HookOnShutdown, nil)
	r.Scheduler.Stop()
}

// Reload rescans vibes and reapplies configuration.
func (r *Runtime) Reload() error {
	// Cancel all scheduled tasks
	for _, vibe := range r.Registry.List() {
		r.Scheduler.Cancel(vibe.Spec.Name)
	}

	// Rescan
	if err := r.Registry.Scan(); err != nil {
		return err
	}

	// Reschedule
	for _, vibe := range r.Registry.List() {
		if vibe.Spec.Schedule != "" {
			v := vibe
			r.Scheduler.Schedule(v.Spec.Name, v.Spec.Schedule, func() {
				r.Dispatcher.Dispatch(HookOnSchedule, map[string]interface{}{
					"vibe": v,
				})
			})
		}
	}

	return nil
}

// InstallVibe copies a vibe file to the vibes directory.
func (r *Runtime) InstallVibe(sourcePath string) error {
	filename := filepath.Base(sourcePath)
	destPath := filepath.Join(r.DataDir, "vibes", filename)

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return err
	}

	return r.Reload()
}

// UninstallVibe removes a vibe file.
func (r *Runtime) UninstallVibe(name string) error {
	vibe, ok := r.Registry.Get(name)
	if !ok {
		return nil
	}

	r.Scheduler.Cancel(name)

	if err := os.Remove(vibe.FilePath); err != nil {
		return err
	}

	return r.Reload()
}

// GetTheme returns the merged theme configuration from all active vibes.
func (r *Runtime) GetTheme() ThemeConfig {
	merged := ThemeConfig{}

	for _, vibe := range r.Registry.List() {
		if !vibe.Enabled {
			continue
		}
		theme := vibe.Spec.UI.Theme

		// Merge: later vibes override earlier ones
		if theme.Primary != "" {
			merged.Primary = theme.Primary
		}
		if theme.Secondary != "" {
			merged.Secondary = theme.Secondary
		}
		if theme.Accent != "" {
			merged.Accent = theme.Accent
		}
		if theme.Background != "" {
			merged.Background = theme.Background
		}
		if theme.Foreground != "" {
			merged.Foreground = theme.Foreground
		}
		if theme.Success != "" {
			merged.Success = theme.Success
		}
		if theme.Warning != "" {
			merged.Warning = theme.Warning
		}
		if theme.Error != "" {
			merged.Error = theme.Error
		}
	}

	return merged
}

// GetLayout returns the merged layout configuration from all active vibes.
func (r *Runtime) GetLayout() LayoutConfig {
	merged := LayoutConfig{}

	for _, vibe := range r.Registry.List() {
		if !vibe.Enabled {
			continue
		}
		layout := vibe.Spec.UI.Layout

		if layout.Sidebar != "" {
			merged.Sidebar = layout.Sidebar
		}
		if layout.TreeWidth != "" {
			merged.TreeWidth = layout.TreeWidth
		}
	}

	return merged
}

// GetCustomTools returns all custom tools defined by active vibes.
func (r *Runtime) GetCustomTools() []ToolDefinition {
	var tools []ToolDefinition

	for _, vibe := range r.Registry.List() {
		if !vibe.Enabled {
			continue
		}
		tools = append(tools, vibe.Spec.Tools...)
	}

	return tools
}
