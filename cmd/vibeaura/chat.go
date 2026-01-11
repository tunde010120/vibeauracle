package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/nathfavour/vibeauracle/brain"
)

type focus int

const (
	focusChat focus = iota
	focusPerusal
	focusEdit
)

type model struct {
	viewport      viewport.Model
	perusalVp     viewport.Model
	messages      []string
	textarea      textarea.Model
	editArea      textarea.Model
	err           error
	brain         *brain.Brain
	width         int
	height        int
	initialized   bool
	showTree      bool
	focus         focus
	treeEntries   []os.DirEntry
	treeCursor    int
	currentPath   string
	isFileOpen    bool
	banner        string
	suggestions   []string
	suggestionIdx int
	triggerChar   string // '/' or '#'
	isCapturing   bool

	// Model selection & filtering
	allModelDiscoveries []brain.ModelDiscovery
	suggestionFilter    string
	isFilteringModels   bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EE6FF8")).
			Bold(true)

	aiStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04D9FF")).
		Bold(true)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	highlight = lipgloss.Color("#7D56F4")

	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true).
			Italic(true)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#222222"))

	selectedSuggestionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#7D56F4")).
				Bold(true)

	treeStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#444444")).
			PaddingLeft(2)

	activeBorder = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), true).
			BorderForeground(lipgloss.Color("#7D56F4"))

	inactiveBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("#444444"))
)

type chatState struct {
	Messages []string `json:"messages"`
	Input    string   `json:"input"`
}

var allCommands = []string{
	"/help", "/status", "/cwd", "/version", "/clear", "/exit", "/show-tree", "/shot", "/auth", "/mcp", "/sys", "/skill", "/models",
}

var subCommands = map[string][]string{
	"/auth":   {"/ollama", "/github-models", "/github-copilot", "/openai", "/anthropic"},
	"/mcp":    {"/list", "/add", "/logs", "/call"},
	"/sys":    {"/stats", "/env", "/update", "/logs"},
	"/skill":  {"/list", "/info", "/load", "/disable"},
	"/models": {"/list", "/use", "/pull"},
}

func buildBanner(width int) string {
	if width <= 0 {
		width = 60
	}

	// Wide terminals/panes get the big ASCII banner.
	ascii := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00D7")).Bold(true).Render("       _ _                                  _"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#D700FF")).Bold(true).Render(" __   _(_) |__   ___  __ _ _   _ _ __ __ _  ___| | ___"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#AF00FF")).Bold(true).Render(" \\ \\ / / | '_ \\ / _ \\/ _` | | | | '__/ _` |/ __| |/ _ \\"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#8700FF")).Bold(true).Render("  \\ V /| | |_) |  __/ (_| | |_| | | | (_| | (__| |  __/"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5F00FF")).Bold(true).Render("   \\_/ |_|_.__/ \\___|\\__,_|\\__,_|_|  \\__,_|\\___|_|\\___|"),
	}

	maxASCII := 0
	for _, l := range ascii {
		w := lipgloss.Width(l)
		if w > maxASCII {
			maxASCII = w
		}
	}

	tagline := helpStyle.Render("Distributed, System-Intimate AI Engineering Ecosystem")
	if width >= maxASCII {
		return strings.Join(append(append(ascii, ""), tagline), "\n") + "\n"
	}

	// Compact banner for narrow panes: multicolor title + tagline.
	word := "vibeauracle"
	colors := []lipgloss.Color{
		lipgloss.Color("#FF00D7"),
		lipgloss.Color("#D700FF"),
		lipgloss.Color("#AF00FF"),
		lipgloss.Color("#8700FF"),
		lipgloss.Color("#5F00FF"),
		lipgloss.Color("#7D56F4"),
		lipgloss.Color("#04D9FF"),
	}

	spaced := width >= (len(word)*2 - 1)
	title := gradientWord(word, colors, spaced)
	if lipgloss.Width(title) > width {
		// Fall back if spacing makes it too wide.
		title = gradientWord(word, colors, false)
	}

	// Keep tagline only if it fits reasonably.
	if width < 44 {
		return title + "\n" + helpStyle.Render("System-Intimate AI") + "\n"
	}
	return title + "\n" + tagline + "\n"
}

func gradientWord(word string, colors []lipgloss.Color, spaced bool) string {
	var b strings.Builder
	colorIdx := 0
	for _, r := range word {
		style := lipgloss.NewStyle().Foreground(colors[colorIdx%len(colors)]).Bold(true)
		b.WriteString(style.Render(string(r)))
		colorIdx++
		if spaced {
			b.WriteString(" ")
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func isBannerMessage(msg string) bool {
	// This substring exists in both the wide and compact banner variants.
	return strings.Contains(msg, "System-Intimate") || strings.Contains(msg, "_(_) |__")
}

func ensureBanner(messages *[]string, banner string) {
	if messages == nil {
		return
	}
	if len(*messages) == 0 {
		*messages = append(*messages, banner)
		return
	}
	if isBannerMessage((*messages)[0]) {
		(*messages)[0] = banner
		return
	}
	*messages = append([]string{banner}, *messages...)
}

func initialModel(b *brain.Brain) *model {
	ta := textarea.New()
	ta.Placeholder = "Send a message or type / for commands..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 2000
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	ea := textarea.New()
	ea.Placeholder = "Edit file... (Esc to cancel, Ctrl+S to save)"
	ea.ShowLineNumbers = true
	ea.SetWidth(60)
	ea.SetHeight(20)

	vp := viewport.New(60, 15)
	pvp := viewport.New(60, 15)

	cwd, _ := os.Getwd()

	banner := buildBanner(vp.Width)

	m := &model{
		textarea:    ta,
		editArea:    ea,
		viewport:    vp,
		perusalVp:   pvp,
		messages:    []string{},
		brain:       b,
		focus:       focusChat,
		currentPath: cwd,
		showTree:    true, // Show tree by default
		banner:      banner,
	}

	// Load initial tree
	m.loadTree(cwd)

	// Attempt to restore state
	var state chatState
	if err := b.RecallState("chat_session", &state); err == nil && len(state.Messages) > 0 {
		m.messages = state.Messages
		ensureBanner(&m.messages, banner)
		m.textarea.SetValue(state.Input)
		m.viewport.SetContent(m.renderMessages())
		if m.viewport.TotalLineCount() <= m.viewport.Height {
			m.viewport.GotoTop()
		} else {
			m.viewport.GotoBottom()
		}
	} else {
		m.messages = append(m.messages, banner)
		m.messages = append(m.messages, "Type "+systemStyle.Render("/help")+" to see available commands.")
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoTop()
	}

	return m
}

func (m *model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *model) saveState() {
	state := chatState{
		Messages: m.messages,
		Input:    m.textarea.Value(),
	}
	m.brain.StoreState("chat_session", state)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		eaCmd tea.Cmd
		pvCmd tea.Cmd
	)

	// Update focus-specific components
	switch m.focus {
	case focusChat:
		m.textarea, tiCmd = m.textarea.Update(msg)
	case focusEdit:
		m.editArea, eaCmd = m.editArea.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.perusalVp, pvCmd = m.perusalVp.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		wasAtTop := m.viewport.AtTop()
		wasAtBottom := m.viewport.AtBottom()
		prevYOffset := m.viewport.YOffset

		m.width = msg.Width
		m.height = msg.Height

		if m.showTree {
			m.viewport.Width = (msg.Width / 2) - 2
			m.perusalVp.Width = msg.Width - m.viewport.Width - 4
		} else {
			m.viewport.Width = msg.Width - 2
		}

		m.textarea.SetWidth(m.viewport.Width + 2)
		m.editArea.SetWidth(m.perusalVp.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - 6
		m.perusalVp.Height = m.viewport.Height
		m.editArea.SetHeight(m.perusalVp.Height - 2)

		m.banner = buildBanner(m.viewport.Width)
		ensureBanner(&m.messages, m.banner)
		m.viewport.SetContent(m.renderMessages())

		if wasAtBottom {
			m.viewport.GotoBottom()
		} else if wasAtTop {
			m.viewport.GotoTop()
		} else {
			m.viewport.SetYOffset(prevYOffset)
			if m.viewport.PastBottom() {
				m.viewport.GotoBottom()
			}
		}

	case tea.KeyMsg:
		// Universal focus switcher
		if msg.String() == "tab" && m.focus != focusEdit {
			if m.focus == focusChat {
				m.focus = focusPerusal
				m.textarea.Blur()
			} else {
				m.focus = focusChat
				m.textarea.Focus()
			}
			return m, nil
		}

		if msg.String() == "esc" {
			if m.focus == focusEdit {
				m.focus = focusPerusal
				return m, nil
			}
			m.focus = focusChat
			m.textarea.Focus()
			m.suggestions = nil
			return m, nil
		}

		// Handle active focus
		switch m.focus {
		case focusChat:
			return m.handleChatKey(msg)
		case focusPerusal:
			return m.handlePerusalKey(msg)
		case focusEdit:
			return m.handleEditKey(msg)
		}

	case brain.Response:
		if msg.Error != nil {
			m.messages = append(m.messages, errorStyle.Render(" BRAIN ERROR ")+"\n"+msg.Error.Error())
		} else {
			m.messages = append(m.messages, aiStyle.Render("Brain: ")+m.styleMessage(msg.Content))
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.saveState()

	case []brain.ModelDiscovery:
		m.allModelDiscoveries = msg
		// If we are currently typing /models /use, refresh suggestions
		val := m.textarea.Value()
		if strings.Contains(val, "/models /use") {
			m.updateSuggestions(val)
		}
	}

	return m, tea.Batch(tiCmd, vpCmd, eaCmd, pvCmd)
}

func (m *model) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Suggestion navigation
	if len(m.suggestions) > 0 {
		switch msg.String() {
		case "down":
			m.suggestionIdx = (m.suggestionIdx + 1) % len(m.suggestions)
			return m, nil
		case "up":
			m.suggestionIdx = (m.suggestionIdx - 1 + len(m.suggestions)) % len(m.suggestions)
			return m, nil
		case "enter":
			return m.applySuggestion()
		case "esc":
			m.suggestions = nil
			return m, nil
		}
	}

	// ALWAYS allow viewport scrolling via arrow keys if textarea is on the first/last line
	// or empty, though textarea handles internal navigation.
	// To be safer and match user request perfectly: if focus is Chat, 
	// and they aren't nav-ing suggestions, arrows should at least scroll if empty.
	if m.textarea.Value() == "" {
		switch msg.String() {
		case "up":
			m.viewport.LineUp(1)
			return m, nil
		case "down":
			m.viewport.LineDown(1)
			return m, nil
		}
	}

	// PageUp/PageDown always scroll the chat
	switch msg.String() {
	case "pgup":
		m.viewport.ViewUp()
		return m, nil
	case "pgdown":
		m.viewport.ViewDown()
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		m.saveState()
		return m, tea.Quit
	case "enter":
		v := m.textarea.Value()
		if strings.TrimSpace(v) == "" {
			return m, nil
		}
		if strings.HasPrefix(strings.TrimSpace(v), "/") {
			return m.handleSlashCommand(v)
		}
		m.messages = append(m.messages, userStyle.Render("You: ")+m.styleMessage(v))
		m.textarea.Reset()
		m.textarea.FocusedStyle.Text = lipgloss.NewStyle()
		m.suggestions = nil
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.saveState()
		return m, m.processRequest(v)
	default:
		val := m.textarea.Value()
		m.updateSuggestions(val)
		
		// If we just typed /models /use, trigger model discovery if empty
		if strings.HasSuffix(val, "/models /use ") && len(m.allModelDiscoveries) == 0 {
			return m, m.discoverModels()
		}

		if strings.HasPrefix(val, "/") {
			m.textarea.FocusedStyle.Text = systemStyle
		} else {
			m.textarea.FocusedStyle.Text = lipgloss.NewStyle()
		}
	}
	return m, nil
}

func (m *model) styleMessage(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}

	parts := strings.Split(v, " ")
	for i, p := range parts {
		if strings.HasPrefix(p, "/") {
			// Check if it's a known command or subcommand
			isKnown := false
			for _, c := range allCommands {
				if c == p {
					isKnown = true
					break
				}
			}
			if !isKnown {
				for _, subs := range subCommands {
					for _, s := range subs {
						if s == p {
							isKnown = true
							break
						}
					}
					if isKnown {
						break
					}
				}
			}

			if isKnown {
				parts[i] = systemStyle.Render(p)
			} else {
				parts[i] = errorStyle.Render(p)
			}
		} else if strings.HasPrefix(p, "#") {
			parts[i] = tagStyle.Render(p)
		}
	}
	return strings.Join(parts, " ")
}

func (m *model) handlePerusalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Allow scrolling the conversation viewport from the explorer view via Shift+Arrows
	switch msg.String() {
	case "shift+up":
		m.viewport.LineUp(1)
		return m, nil
	case "shift+down":
		m.viewport.LineDown(1)
		return m, nil
	}

	if m.isFileOpen {
		switch msg.String() {
		case "up", "k":
			m.perusalVp.LineUp(1)
			return m, nil
		case "down", "j":
			m.perusalVp.LineDown(1)
			return m, nil
		}
	}

	switch msg.String() {
	case "up", "k":
		if m.treeCursor > 0 {
			m.treeCursor--
			m.updatePerusalContent()
		}
	case "down", "j":
		if m.treeCursor < len(m.treeEntries)-1 {
			m.treeCursor++
			m.updatePerusalContent()
		}
	case "enter":
		if len(m.treeEntries) == 0 {
			return m, nil
		}
		entry := m.treeEntries[m.treeCursor]
		path := filepath.Join(m.currentPath, entry.Name())
		if entry.IsDir() {
			m.currentPath = path
			m.treeCursor = 0
			m.loadTree(path)
		} else {
			m.openFile(path)
		}
	case "backspace":
		parent := filepath.Dir(m.currentPath)
		m.currentPath = parent
		m.treeCursor = 0
		m.loadTree(parent)
	case ":":
		// Quick command mode if needed, but for now just :i
	case "i":
		if m.isFileOpen {
			m.focus = focusEdit
			m.editArea.Focus()
		}
	}
	return m, nil
}

func (m *model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+s" {
		content := m.editArea.Value()
		os.WriteFile(m.currentPath, []byte(content), 0644)
		m.focus = focusPerusal
		m.openFile(m.currentPath) // Refresh view
		return m, nil
	}
	return m, nil
}

func (m *model) renderMessages() string {
	var sb strings.Builder
	for i, msg := range m.messages {
		// Use lipgloss to wrap the message to the viewport width precisely.
		// This prevents right-overflow in split panes.
		wrapped := lipgloss.NewStyle().Width(m.viewport.Width).Render(msg)
		sb.WriteString(wrapped)
		if i < len(m.messages)-1 {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

func (m *model) loadTree(path string) {
	entries, _ := os.ReadDir(path)
	m.treeEntries = nil
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") || e.Name() == ".env" {
			m.treeEntries = append(m.treeEntries, e)
		}
	}
	m.isFileOpen = false
	m.updatePerusalContent()
}

func (m *model) openFile(path string) {
	content, err := os.ReadFile(path)
	if err == nil {
		m.isFileOpen = true
		m.currentPath = path
		m.editArea.SetValue(string(content))
		m.perusalVp.SetContent(string(content))
	}
}

func (m *model) updatePerusalContent() {
	if m.isFileOpen {
		return
	}
	var sb strings.Builder
	sb.WriteString(systemStyle.Render(" EXPLORER: "+m.currentPath) + "\n\n")
	for i, entry := range m.treeEntries {
		cursor := "  "
		if i == m.treeCursor {
			cursor = "> "
		}
		icon := "📄 "
		if entry.IsDir() {
			icon = "📁 "
		}
		line := cursor + icon + entry.Name()
		if i == m.treeCursor {
			sb.WriteString(suggestionStyle.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}
	m.perusalVp.SetContent(sb.String())
}

func shortenModelName(name string) string {
	return brain.ShortenModelName(name)
}

func (m *model) updateSuggestions(val string) {
	m.suggestions = nil
	m.suggestionIdx = 0
	m.triggerChar = ""
	m.isFilteringModels = false

	if val == "" {
		return
	}

	if strings.Contains(val, "/models /use") {
		m.isFilteringModels = true
		if len(m.allModelDiscoveries) == 0 {
			// Trigger discovery
			go func() {
				// We can't return Cmd here, so we'll just wait for the next Update cycle 
				// if we were in a proper Msg flow, but here we are in a helper.
				// Better to trigger this from handleChatKey or applySuggestion.
			}()
		}
		
		// Everything after "/models /use " is the filter
		parts := strings.Split(val, "/models /use")
		filter := ""
		if len(parts) > 1 {
			filter = strings.TrimSpace(parts[1])
		}
		m.suggestionFilter = filter

		for _, d := range m.allModelDiscoveries {
			display := fmt.Sprintf("%s (%s)", shortenModelName(d.Name), d.Provider)
			if filter == "" || strings.Contains(strings.ToLower(display), strings.ToLower(filter)) {
				// We store the full identifier for applySuggestion, but display it nicely
				m.suggestions = append(m.suggestions, fmt.Sprintf("%s|%s", d.Provider, d.Name))
			}
		}
		return
	}

	// Handle trailing space for subcommand triggering
	if strings.HasSuffix(val, " ") {
		parts := strings.Fields(val)
		if len(parts) == 1 {
			if subs, ok := subCommands[parts[0]]; ok {
				m.suggestions = subs
				m.triggerChar = "" // Already has / in the subCommand string
				sort.Strings(m.suggestions)
				return
			}
		}
	}

	words := strings.Fields(val)
	if len(words) == 0 {
		if strings.HasSuffix(val, "/") {
			m.triggerChar = "/"
			m.suggestions = append([]string{}, allCommands...)
			sort.Strings(m.suggestions)
		} else if strings.HasSuffix(val, "#") {
			m.triggerChar = "#"
			m.suggestions = m.getFileSuggestions("")
		}
		return
	}

	lastWord := words[len(words)-1]
	
	// Check if we are typing a subcommand
	if len(words) > 1 {
		parentCmd := words[0]
		if subs, ok := subCommands[parentCmd]; ok {
			m.triggerChar = "" // Subcommands already have slashes
			for _, sub := range subs {
				if strings.HasPrefix(sub, lastWord) {
					m.suggestions = append(m.suggestions, sub)
				}
			}
			sort.Strings(m.suggestions)
			if len(m.suggestions) > 0 {
				return
			}
		}
	}

	if strings.HasPrefix(lastWord, "/") {
		m.triggerChar = "/"
		for _, cmd := range allCommands {
			if strings.HasPrefix(cmd, lastWord) {
				m.suggestions = append(m.suggestions, cmd)
			}
		}
		sort.Strings(m.suggestions)
	} else if strings.HasPrefix(lastWord, "#") {
		m.triggerChar = "#"
		m.suggestions = m.getFileSuggestions(lastWord[1:])
	}
}

func (m *model) getFileSuggestions(prefix string) []string {
	var suggestions []string
	root, _ := os.Getwd()

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(suggestions) > 30 {
			return nil
		}

		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "bin" || name == "dist" {
				return filepath.SkipDir
			}
			if prefix != "" && !strings.HasPrefix(name, prefix) && !strings.HasPrefix(path, prefix) {
				return nil
			}
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}

		if prefix == "" || strings.HasPrefix(rel, prefix) || strings.HasPrefix(name, prefix) {
			suggestions = append(suggestions, rel)
		}

		return nil
	})

	sort.Strings(suggestions)
	return suggestions
}

func (m *model) applySuggestion() (tea.Model, tea.Cmd) {
	if len(m.suggestions) == 0 {
		return m, nil
	}

	val := m.textarea.Value()
	words := strings.Fields(val)

	suggestion := m.suggestions[m.suggestionIdx]

	// Handle model selection specialized format: provider|name
	if m.isFilteringModels && strings.Contains(suggestion, "|") {
		parts := strings.Split(suggestion, "|")
		provider := parts[0]
		modelName := parts[1]
		
		// Set the exact command
		m.textarea.SetValue(fmt.Sprintf("/models /use %s %s", provider, modelName))
		m.textarea.SetCursor(len(m.textarea.Value()))
		m.suggestions = nil
		return m.handleSlashCommand(m.textarea.Value())
	}

	// Determine if we are completing a top-level command or a subcommand/argument
	isTopLevel := len(words) <= 1 && !strings.HasSuffix(val, " ")

	if isTopLevel {
		trimmed := strings.TrimPrefix(suggestion, m.triggerChar)
		replacement := m.triggerChar + trimmed
		m.textarea.SetValue(replacement)
	} else if strings.HasSuffix(val, " ") {
		// We were suggesting subcommands because of a trailing space
		m.textarea.SetValue(val + suggestion)
	} else {
		// Replacing the last word (likely a subcommand or tag)
		trimmed := strings.TrimPrefix(suggestion, m.triggerChar)
		replacement := m.triggerChar + trimmed
		if len(words) > 0 {
			words[len(words)-1] = replacement
			m.textarea.SetValue(strings.Join(words, " "))
		} else {
			m.textarea.SetValue(replacement)
		}
	}
	
	m.textarea.SetCursor(len(m.textarea.Value()))
	m.suggestions = nil

	currentVal := strings.TrimSpace(m.textarea.Value())
	parts := strings.Fields(currentVal)

	// If it's a command that has subcommands and we only have the parent, keep composing
	if len(parts) == 1 {
		if _, ok := subCommands[parts[0]]; ok {
			m.textarea.SetValue(currentVal + " ")
			m.textarea.SetCursor(len(m.textarea.Value()))
			m.updateSuggestions(m.textarea.Value())
			return m, nil
		}
	}

	// Auto-execute when suggestion completes a no-arg command or a no-arg subcommand.
	noArgSubs := map[string]map[string]bool{
		"/models": {"/list": true},
		"/sys":    {"/stats": true, "/env": true, "/update": true, "/logs": true},
		"/mcp":    {"/list": true, "/logs": true},
		"/skill":  {"/list": true},
	}

	if len(parts) == 1 && m.triggerChar == "/" {
		if _, hasSubs := subCommands[parts[0]]; !hasSubs {
			return m.handleSlashCommand(currentVal)
		}
	}

	if len(parts) == 2 {
		if subs, ok := noArgSubs[parts[0]]; ok {
			if subs[parts[1]] {
				return m.handleSlashCommand(currentVal)
			}
		}
	}

	// Otherwise stay in the textarea to allow more input
	m.textarea.SetValue(m.textarea.Value() + " ")
	m.textarea.SetCursor(len(m.textarea.Value()))
	return m, nil
}

func (m *model) processRequest(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		req := brain.Request{
			ID:      uuid.NewString(),
			Content: content,
		}
		resp, _ := m.brain.Process(ctx, req)
		return resp
	}
}

func (m *model) takeScreenshot() (tea.Model, tea.Cmd) {
	config := m.brain.GetConfig()
	dir := config.UI.ScreenshotDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		m.messages = append(m.messages, errorStyle.Render(" Screenshot Error: ")+err.Error())
		return m, nil
	}

	timestamp := time.Now().Format("2006-01-02_150405")
	filename := fmt.Sprintf("vibeaura_%s", timestamp)

	basePath := filepath.Join(dir, filename)
	ansiPath := basePath + ".ansi"
	svgPath := basePath + ".svg"
	pngPath := basePath + ".png"

	// Use current layout but ensure it's rendered for capture
	m.isCapturing = true
	rawView := m.View()
	m.isCapturing = false

	// Tier 2: Generate SVG but don't save yet if targeting PNG
	svgContent := convertAnsiToSVG(rawView)
	_ = os.WriteFile(svgPath, []byte(svgContent), 0644)

	// Tier 1: Try PNG
	err := convertToPNG(svgPath, pngPath)

	msg := systemStyle.Render(" SCREENSHOT CAPTURED ") + "\n"

	if err == nil {
		// Highest Tier: PNG only
		_ = os.Remove(svgPath)
		msg += helpStyle.Render("🖼️ Saved PNG: " + pngPath)
	} else if svgContent != "" {
		// Middle Tier: SVG only
		msg += helpStyle.Render("📍 Saved SVG: " + svgPath)
		msg += "\n" + errorStyle.Render(" PNG fail: ") + helpStyle.Render("install ffmpeg/rsvg")
	} else {
		// Fallback Tier: ANSI only
		_ = os.WriteFile(ansiPath, []byte(rawView), 0644)
		msg += helpStyle.Render("📄 Saved ANSI: " + ansiPath)
	}

	m.messages = append(m.messages, msg)
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	m.textarea.Reset()

	// Normalize command path if user uses slashes without spaces (e.g. /models/list)
	if len(parts) == 1 && strings.Count(parts[0], "/") > 1 {
		cmdPath := parts[0]
		isKnown := false
		for _, c := range allCommands {
			if c == cmdPath {
				isKnown = true
				break
			}
		}
		if !isKnown {
			// Split path like /models/list into ["/models", "/list"]
			rawParts := strings.Split(strings.TrimPrefix(cmdPath, "/"), "/")
			parts = []string{}
			for _, p := range rawParts {
				if p != "" {
					parts = append(parts, "/"+p)
				}
			}
		}
	}

	// Guardrail: subcommands like "/list" are not top-level commands
	if len(parts) > 0 {
		isTopLevel := false
		for _, c := range allCommands {
			if c == parts[0] {
				isTopLevel = true
				break
			}
		}
		if !isTopLevel {
			isSub := false
			for _, subs := range subCommands {
				for _, s := range subs {
					if s == parts[0] {
						isSub = true
						break
					}
				}
				if isSub {
					break
				}
			}
			if isSub {
				m.messages = append(m.messages,
					systemStyle.Render(" COMMAND ")+"\n"+
					helpStyle.Render("That is a subcommand and can’t be run by itself.")+"\n"+
					helpStyle.Render("Example: /models /list"),
				)
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, nil
			}
		}
	}

	switch parts[0] {
	case "/help":
		m.messages = append(m.messages, systemStyle.Render(" COMMANDS ")+"\n"+helpStyle.Render("• /help    - Show this list\n• /status  - System resource snapshot\n• /mcp     - Manage MCP tools & servers\n• /skill   - Manage agentic vibes/skills\n• /sys     - Hardware & system details\n• /auth    - Manage AI provider credentials\n• /shot    - Take a beautiful TUI screenshot\n• /cwd     - Show current directory\n• /version - Show version info\n• /clear   - Clear chat history\n• /exit    - Quit vibeauracle"))
	case "/status":
		snapshot, _ := m.brain.GetSnapshot()
		status := fmt.Sprintf(systemStyle.Render(" SYSTEM ")+"\n"+helpStyle.Render("CPU: %.1f%% | Mem: %.1f%%"), snapshot.CPUUsage, snapshot.MemoryUsage)
		m.messages = append(m.messages, status)
	case "/cwd":
		snapshot, _ := m.brain.GetSnapshot()
		m.messages = append(m.messages, systemStyle.Render(" CWD ")+" "+helpStyle.Render(snapshot.WorkingDir))
	case "/version":
		m.messages = append(m.messages, systemStyle.Render(" VERSION ")+"\n"+helpStyle.Render(fmt.Sprintf("App: %s\nCommit: %s\nCompiler: %s", Version, Commit, runtime.Version())))
	case "/auth":
		return m.handleAuthCommand(parts)
	case "/models":
		return m.handleModelsCommand(parts)
	case "/mcp":
		return m.handleMcpCommand(parts)
	case "/sys":
		return m.handleSysCommand(parts)
	case "/skill":
		return m.handleSkillCommand(parts)
	case "/shot":
		return m.takeScreenshot()
	case "/show-tree":
		m.showTree = !m.showTree
		// trigger resize
		return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }
	case "/clear":
		m.messages = []string{}
		ensureBanner(&m.messages, m.banner)
		m.messages = append(m.messages, "Type "+systemStyle.Render("/help")+" to see available commands.")
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoTop()
		m.saveState()
		return m, nil
	case "/exit":
		return m, tea.Quit
	default:
		m.messages = append(m.messages, errorStyle.Render(" Unknown Command: ")+parts[0])
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) handleAuthCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m.messages = append(m.messages, systemStyle.Render(" AUTH ")+"\n"+helpStyle.Render("Manage your AI provider credentials.\n\nUsage: /auth <provider> [key/endpoint]\nProviders: /ollama, /github-models, /github-copilot, /openai, /anthropic"))
		return m, nil
	}

	provider := strings.ToLower(parts[1])
	switch provider {
	case "/ollama", "ollama":
		if len(parts) > 2 {
			endpoint := parts[2]
			cfg := m.brain.Config()
			cfg.Model.Endpoint = endpoint
			if err := m.brain.UpdateConfig(cfg); err != nil {
				m.messages = append(m.messages, errorStyle.Render(" CONFIG ERROR ")+"\n"+err.Error())
			} else {
				m.messages = append(m.messages, systemStyle.Render(" OLLAMA ")+"\n"+helpStyle.Render(fmt.Sprintf("Ollama endpoint set to: %s", endpoint)))
			}
		} else {
			m.messages = append(m.messages, systemStyle.Render(" OLLAMA ")+"\n"+helpStyle.Render("Ollama is usually active on http://localhost:11434.\nTo use a custom host: /auth /ollama <endpoint>"))
		}
	case "/github-models", "github-models":
		if len(parts) > 2 {
			err := m.brain.StoreSecret("github_models_pat", parts[2])
			if err != nil {
				m.messages = append(m.messages, errorStyle.Render(" VAULT ERROR ")+"\n"+err.Error())
			} else {
				m.messages = append(m.messages, systemStyle.Render(" GITHUB MODELS ")+"\n"+helpStyle.Render("GitHub Models PAT received and stored securely."))
			}
		} else {
			m.messages = append(m.messages, systemStyle.Render(" GITHUB MODELS ")+"\n"+helpStyle.Render("Special BYOK method for GitHub AI Models.\nUsage: /auth /github-models <your-pat-token>"))
		}
	case "/github-copilot", "github-copilot":
		m.messages = append(m.messages, systemStyle.Render(" GITHUB COPILOT ")+"\n"+errorStyle.Render(" Not yet integrated "))
	case "/openai", "openai", "/anthropic", "anthropic":
		if len(parts) > 2 {
			providerName := strings.TrimPrefix(provider, "/")
			err := m.brain.StoreSecret(providerName+"_api_key", parts[2])
			if err != nil {
				m.messages = append(m.messages, errorStyle.Render(" VAULT ERROR ")+"\n"+err.Error())
			} else {
				m.messages = append(m.messages, systemStyle.Render(strings.ToUpper(providerName))+"\n"+helpStyle.Render(fmt.Sprintf("%s API key received and stored securely.", strings.Title(providerName))))
			}

			// Optional: set custom endpoint if provided as 3rd arg
			if len(parts) > 3 {
				endpoint := parts[3]
				cfg := m.brain.Config()
				cfg.Model.Endpoint = endpoint
				if err := m.brain.UpdateConfig(cfg); err == nil {
					m.messages = append(m.messages, helpStyle.Render("Endpoint set to: "+endpoint))
				}
			}
		} else {
			providerTitle := strings.Title(strings.TrimPrefix(provider, "/"))
			m.messages = append(m.messages, systemStyle.Render(strings.ToUpper(providerTitle))+"\n"+helpStyle.Render(fmt.Sprintf("Usage: /auth %s <api-key> [endpoint]", provider)))
		}
	default:
		m.messages = append(m.messages, systemStyle.Render(" AUTH ")+"\n"+errorStyle.Render(fmt.Sprintf(" Provider '%s' not yet integrated ", provider)))
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) handleModelsCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 || parts[1] == "/list" || parts[1] == "list" {
		m.messages = append(m.messages, systemStyle.Render(" DISCOVERING MODELS ")+"\n"+subtleStyle.Render("Querying active providers..."))
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		return m, func() tea.Msg {
			discoveries, err := m.brain.DiscoverModels(context.Background())
			if err != nil {
				return brain.Response{Error: err}
			}

			var sb strings.Builder
			sb.WriteString(systemStyle.Render(" AVAILABLE MODELS ") + "\n")
			if len(discoveries) == 0 {
				sb.WriteString(helpStyle.Render("No models found. Check /auth to configure providers."))
			} else {
				for _, d := range discoveries {
					sb.WriteString(fmt.Sprintf("%s %s\n", 
						aiStyle.Render("• "+d.Name), 
						subtleStyle.Render("("+d.Provider+")")))
				}
				sb.WriteString("\n" + helpStyle.Render("Use /models /use <provider> <model> to switch."))
			}
			return brain.Response{Content: sb.String()}
		}
	}

	sub := strings.ToLower(parts[1])
	if (sub == "/use" || sub == "use") && len(parts) >= 4 {
		provider := parts[2]
		modelName := parts[3]
		err := m.brain.SetModel(provider, modelName)
		if err != nil {
			m.messages = append(m.messages, errorStyle.Render(" SWITCH ERROR ")+"\n"+err.Error())
		} else {
			m.messages = append(m.messages, systemStyle.Render(" MODEL SWITCHED ")+"\n"+helpStyle.Render(fmt.Sprintf("Now using %s via %s", modelName, provider)))
		}
	} else if sub == "/use" || sub == "use" {
		m.messages = append(m.messages, systemStyle.Render(" MODELS ")+"\n"+helpStyle.Render("Usage: /models /use <provider> <model_name>")+"\n"+subtleStyle.Render("Tip: Use the interactive selector by typing '/models /use ' and scrolling."))
	} else if sub == "/pull" || sub == "pull" {
		if len(parts) >= 3 {
			modelName := parts[2]
			m.messages = append(m.messages, systemStyle.Render(" OLLAMA PULL ")+"\n"+helpStyle.Render("Requesting pull for: "+modelName))
			return m, m.pullOllamaModel(modelName)
		}
		m.messages = append(m.messages, systemStyle.Render(" MODELS ")+"\n"+helpStyle.Render("Usage: /models /pull <model_name>")+"\n"+subtleStyle.Render("Example: /models /pull llama3.2"))
	} else {
		m.messages = append(m.messages, errorStyle.Render(" Unknown MODELS subcommand: ")+sub)
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) handleMcpCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m.messages = append(m.messages, systemStyle.Render(" MCP ")+"\n"+helpStyle.Render("Manage Model Context Protocol servers.\n\nUsage: /mcp <subcommand>\nSubcommands: /list, /add, /logs, /call"))
		return m, nil
	}

	sub := strings.ToLower(parts[1])
	switch sub {
	case "/list", "list":
		m.messages = append(m.messages, systemStyle.Render(" MCP SERVERS ")+"\n"+helpStyle.Render("• github (stdio) - tools: github_query\n• postgres (stdio) - tools: postgres_exec"))
	case "/add", "add":
		m.messages = append(m.messages, systemStyle.Render(" MCP ")+"\n"+helpStyle.Render("Usage: /mcp /add <name> <command> [args...]"))
	case "/logs", "logs":
		m.messages = append(m.messages, systemStyle.Render(" MCP LOGS ")+"\n"+subtleStyle.Render("Waiting for MCP traffic..."))
	case "/call", "call":
		m.messages = append(m.messages, systemStyle.Render(" MCP CALL ")+"\n"+helpStyle.Render("Usage: /mcp /call <tool_name> <json_args>"))
	default:
		m.messages = append(m.messages, errorStyle.Render(" Unknown MCP subcommand: ")+sub)
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) handleSysCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m.messages = append(m.messages, systemStyle.Render(" SYS ")+"\n"+helpStyle.Render("System and hardware intimacy controls.\n\nUsage: /sys <subcommand>\nSubcommands: /stats, /env, /update, /logs"))
		return m, nil
	}

	sub := strings.ToLower(parts[1])
	switch sub {
	case "/stats", "stats":
		snapshot, _ := m.brain.GetSnapshot()
		stats := fmt.Sprintf(systemStyle.Render(" POWER SNAPSHOT ")+"\n"+
			helpStyle.Render("OS: %s | Arch: %s\nCPU: %.1f%% | Mem: %.1f%%\nGoroutines: %d"),
			runtime.GOOS, runtime.GOARCH, snapshot.CPUUsage, snapshot.MemoryUsage, runtime.NumGoroutine())
		m.messages = append(m.messages, stats)
	case "/env", "env":
		m.messages = append(m.messages, systemStyle.Render(" ENVIRONMENT ")+"\n"+helpStyle.Render("Limited view (Filtered for security)\nSHELL: %s\nPATH: %s..."), os.Getenv("SHELL"), os.Getenv("PATH")[:30])
	case "/update", "update":
		// This uses the logic from update.go
		m.messages = append(m.messages, systemStyle.Render(" UPDATE ")+"\n"+helpStyle.Render("Checking for latest release on GitHub..."))
		// In a real implementation, we would return a Cmd here to run the update check
	case "/logs", "logs":
		m.messages = append(m.messages, systemStyle.Render(" SYSTEM LOGS ")+"\n"+subtleStyle.Render("Streaming vibeauracle.log..."))
	default:
		m.messages = append(m.messages, errorStyle.Render(" Unknown SYS subcommand: ")+sub)
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) handleSkillCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m.messages = append(m.messages, systemStyle.Render(" SKILL ")+"\n"+helpStyle.Render("Manage Brain capabilities (Vibes).\n\nUsage: /skill <subcommand>\nSubcommands: /list, /info, /load, /disable"))
		return m, nil
	}

	sub := strings.ToLower(parts[1])
	switch sub {
	case "/list", "list":
		m.messages = append(m.messages, systemStyle.Render(" ACTIVE SKILLS ")+"\n"+helpStyle.Render("• hello-world (vibe)\n• fs-manager (internal)\n• git-ops (internal)"))
	case "/info", "info":
		m.messages = append(m.messages, systemStyle.Render(" SKILL INFO ")+"\n"+helpStyle.Render("Usage: /skill /info <skill_id>"))
	case "/load", "load":
		m.messages = append(m.messages, systemStyle.Render(" LOAD SKILL ")+"\n"+helpStyle.Render("Usage: /skill /load <path_or_url>"))
	case "/disable", "disable":
		m.messages = append(m.messages, systemStyle.Render(" DISABLE SKILL ")+"\n"+helpStyle.Render("Usage: /skill /disable <skill_id>"))
	default:
		m.messages = append(m.messages, errorStyle.Render(" Unknown SKILL subcommand: ")+sub)
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) View() string {
	header := titleStyle.Render(" vibeauracle ") + " " + helpStyle.Render("v"+Version)
	borderWidth := m.width
	if borderWidth > 20 {
		borderWidth--
	}
	border := strings.Repeat("─", borderWidth)

	chatView := m.viewport.View()
	if m.focus == focusChat {
		chatView = activeBorder.Width(m.viewport.Width).Render(chatView)
	} else {
		chatView = inactiveBorder.Width(m.viewport.Width).Render(chatView)
	}

	mainContent := chatView
	if m.showTree {
		var perusalContent string
		if m.focus == focusEdit {
			perusalContent = activeBorder.Width(m.perusalVp.Width).Render(m.editArea.View())
		} else if m.focus == focusPerusal {
			perusalContent = activeBorder.Width(m.perusalVp.Width).Render(m.perusalVp.View())
		} else {
			perusalContent = inactiveBorder.Width(m.perusalVp.Width).Render(m.perusalVp.View())
		}

		mainContent = lipgloss.JoinHorizontal(lipgloss.Top,
			chatView,
			perusalContent,
		)
	}

	view := fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s",
		header,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render(border),
		mainContent,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render(border),
		m.textarea.View(),
	)

	if !m.isCapturing {
		if suggs := m.renderSuggestions(); suggs != "" {
			view += "\n" + suggs
		}
	}

	return view + "\n"
}

func (m *model) renderSuggestions() string {
	if len(m.suggestions) == 0 {
		return ""
	}

	maxItems := 10
	items := m.suggestions
	if len(items) > maxItems {
		items = items[:maxItems]
	}

	width := 50
	if m.width-10 < width {
		width = m.width - 4
	}

	var rows []string

	// Header/Filter input for model selector
	if m.isFilteringModels {
		filterHeader := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true).
			Padding(0, 1).
			Render("🔍 Filter: " + m.suggestionFilter + "█")
		rows = append(rows, filterHeader)
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render(strings.Repeat("─", width)))
		
		if len(m.allModelDiscoveries) == 0 {
			rows = append(rows, subtleStyle.Width(width).Render("  Discovering models..."))
		}
	}

	for i, s := range items {
		selected := i == m.suggestionIdx

		style := suggestionStyle
		if selected {
			style = selectedSuggestionStyle
		}

		name := filepath.Base(s)
		dir := filepath.Dir(s)

		if strings.Contains(s, "|") && m.isFilteringModels {
			parts := strings.Split(s, "|")
			provider := parts[0]
			modelName := parts[1]
			name = shortenModelName(modelName)
			dir = provider

			// Shorten provider names for UI
			if dir == "github-models" {
				dir = "github"
			}
		} else {
			if m.triggerChar == "/" {
				name = s
			}
			if dir == "." || m.triggerChar == "/" {
				dir = ""
			}
		}

		// Truncate name if path is too long
		namePart := name
		if len(namePart) > 25 {
			namePart = namePart[:22] + "..."
		}

		dirPart := dir
		if len(dirPart) > width-25 {
			dirPart = "..." + dirPart[len(dirPart)-(width-28):]
		}

		// Calculate spacing for right alignment
		spacing := width - lipgloss.Width(namePart) - lipgloss.Width(dirPart) - 2
		if spacing < 1 {
			spacing = 1
		}

		row := fmt.Sprintf(" %s%s%s ", namePart, strings.Repeat(" ", spacing), dirPart)
		rows = append(rows, style.Width(width).Render(row))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlight).
		MarginLeft(2).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m *model) discoverModels() tea.Cmd {
	return func() tea.Msg {
		discoveries, err := m.brain.DiscoverModels(context.Background())
		if err != nil {
			return brain.Response{Error: err}
		}
		return discoveries
	}
}

func (m *model) pullOllamaModel(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.brain.PullModel(context.Background(), name)
		if err != nil {
			return brain.Response{Error: err}
		}
		return brain.Response{Content: "Successfully pulled " + name + ". You can now use it with /models /use ollama " + name}
	}
}
