package vibes

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a spec validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult holds all validation errors for a Vibe.
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

func (vr *ValidationResult) IsValid() bool {
	return len(vr.Errors) == 0
}

func (vr *ValidationResult) AddError(field, msg string) {
	vr.Errors = append(vr.Errors, ValidationError{Field: field, Message: msg})
}

func (vr *ValidationResult) AddWarning(field, msg string) {
	vr.Warnings = append(vr.Warnings, ValidationError{Field: field, Message: msg})
}

// Validate checks a Vibe spec for correctness.
func Validate(vibe *Vibe) *ValidationResult {
	result := &ValidationResult{}

	// Name validation
	if vibe.Spec.Name == "" {
		result.AddError("name", "required field is missing")
	} else if !isValidName(vibe.Spec.Name) {
		result.AddError("name", "must be lowercase alphanumeric with hyphens only")
	}

	// Version validation
	if vibe.Spec.Version == "" {
		result.AddWarning("version", "missing version, defaulting to 1.0.0")
	} else if !isValidVersion(vibe.Spec.Version) {
		result.AddWarning("version", "should follow semver format (e.g., 1.0.0)")
	}

	// Hooks validation
	for _, hook := range vibe.Spec.Hooks {
		if !isValidHook(hook) {
			result.AddError("hooks", fmt.Sprintf("unknown hook: %s", hook))
		}
	}

	// Permissions validation
	for _, perm := range vibe.Spec.Permissions {
		if !isValidPermission(perm) {
			result.AddError("permissions", fmt.Sprintf("unknown permission: %s", perm))
		}
	}

	// Schedule validation
	if vibe.Spec.Schedule != "" {
		if !isValidCron(vibe.Spec.Schedule) {
			result.AddError("schedule", "invalid cron expression")
		}
	}

	// Tools validation
	for i, tool := range vibe.Spec.Tools {
		if tool.Name == "" {
			result.AddError(fmt.Sprintf("tools[%d].name", i), "required field is missing")
		}
		if tool.Action == "" {
			result.AddError(fmt.Sprintf("tools[%d].action", i), "required field is missing")
		}
	}

	// UI validation
	if vibe.Spec.UI.Theme.Primary != "" && !isValidColor(vibe.Spec.UI.Theme.Primary) {
		result.AddWarning("ui.theme.primary", "should be a valid hex color")
	}
	if vibe.Spec.UI.Theme.Secondary != "" && !isValidColor(vibe.Spec.UI.Theme.Secondary) {
		result.AddWarning("ui.theme.secondary", "should be a valid hex color")
	}

	// Security validation
	if vibe.Spec.Security.RequirePassword && vibe.Spec.Security.PasswordHash == "" {
		result.AddError("security.password_hash", "required when require_password is true")
	}

	// Instructions validation
	if strings.TrimSpace(vibe.Instructions) == "" {
		result.AddWarning("instructions", "empty instructions body")
	}

	return result
}

func isValidName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`, name)
	return matched
}

func isValidVersion(version string) bool {
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$`, version)
	return matched
}

func isValidHook(hook Hook) bool {
	validHooks := []Hook{
		HookOnStartup, HookOnShutdown, HookOnFileChange, HookOnCommand,
		HookOnToolCall, HookOnSchedule, HookOnConfigChange, HookOnModelResponse, HookOnUpdate,
	}
	for _, h := range validHooks {
		if h == hook {
			return true
		}
	}
	return false
}

func isValidPermission(perm Permission) bool {
	validPerms := []Permission{
		PermConfigRead, PermConfigWrite, PermUITheme, PermUILayout,
		PermSchedulerCreate, PermSchedulerCancel, PermAgentPrompt, PermAgentTools,
		PermAgentLock, PermUpdateFrequency, PermUpdateChannel, PermBinarySelfMod,
		PermSystemShell, PermSystemFS, PermSandboxEscape,
	}
	for _, p := range validPerms {
		if p == perm {
			return true
		}
	}
	return false
}

func isValidCron(expr string) bool {
	// Basic cron validation (5 or 6 fields)
	fields := strings.Fields(expr)
	return len(fields) >= 5 && len(fields) <= 6
}

func isValidColor(color string) bool {
	matched, _ := regexp.MatchString(`^#[0-9A-Fa-f]{6}$`, color)
	return matched
}
