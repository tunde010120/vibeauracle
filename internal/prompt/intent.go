package prompt

import "strings"

// ClassifyIntent uses lightweight heuristics to pick a mode.
// This should remain cheap and offline.
func ClassifyIntent(userText string) Intent {
	text := strings.TrimSpace(strings.ToLower(userText))
	if text == "" {
		return IntentChat
	}

	// Explicit mode directives (power-user)
	if strings.HasPrefix(text, "/ask") || strings.HasPrefix(text, "ask:") {
		return IntentAsk
	}
	if strings.HasPrefix(text, "/plan") || strings.HasPrefix(text, "plan:") {
		return IntentPlan
	}
	if strings.HasPrefix(text, "/do") || strings.HasPrefix(text, "do:") {
		return IntentCRUD
	}

	// Question / explanation intent
	if strings.HasSuffix(text, "?") || strings.HasPrefix(text, "why ") || strings.HasPrefix(text, "what ") || strings.HasPrefix(text, "how ") {
		return IntentAsk
	}

	// Planning intent
	if strings.Contains(text, "architecture") || strings.Contains(text, "design") || strings.Contains(text, "roadmap") || strings.Contains(text, "plan") || strings.Contains(text, "scaffold") {
		return IntentPlan
	}

	// CRUD / implementation intent
	crudWords := []string{"implement", "fix", "refactor", "create file", "add", "remove", "update", "write", "generate", "debug", "build", "test"}
	for _, w := range crudWords {
		if strings.Contains(text, w) {
			return IntentCRUD
		}
	}

	return IntentChat
}

// LooksLikePrompt determines whether input should be treated as an actual prompt.
// This helps ignore accidental keystrokes or empty messages.
func LooksLikePrompt(userText string) bool {
	text := strings.TrimSpace(userText)
	if text == "" {
		return false
	}
	if len(text) == 1 {
		// avoid treating single characters as prompts, except common confirmations
		switch strings.ToLower(text) {
		case "y", "n", "?":
			return true
		default:
			return false
		}
	}
	return true
}
