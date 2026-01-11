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
	"/help", "/status", "/cwd", "/version", "/clear", "/exit", "/show-tree", "/shot",
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

	banner := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00D7")).Bold(true).Render("       _ _                                  _      "),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#D700FF")).Bold(true).Render(" __   _(_) |__   ___  __ _ _   _ _ __ __ _  ___| | ___ "),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#AF00FF")).Bold(true).Render(" \\ \\ / / | '_ \\ / _ \\/ _` | | | | '__/ _` |/ __| |/ _ \\"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#8700FF")).Bold(true).Render("  \\ V /| | |_) |  __/ (_| | |_| | | | (_| | (__| |  __/"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5F00FF")).Bold(true).Render("   \\_/ |_|_.__/ \\___|\\__,_|\\__,_|_|  \\__,_|\\___|_|\\___|"),
		"",
		helpStyle.Render("Distributed, System-Intimate AI Engineering Ecosystem"),
		"",
	)

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
		m.textarea.SetValue(state.Input)
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()
	} else {
		m.messages = append(m.messages, banner)
		m.messages = append(m.messages, "Type "+systemStyle.Render("/help")+" to see available commands.")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
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
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		if m.showTree {
			m.viewport.Width = msg.Width / 2
			m.perusalVp.Width = msg.Width / 2 - 4
		}
		m.textarea.SetWidth(m.viewport.Width)
		m.editArea.SetWidth(m.perusalVp.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - 6
		m.perusalVp.Height = m.viewport.Height
		m.editArea.SetHeight(m.perusalVp.Height - 2)

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
			m.messages = append(m.messages, errorStyle.Render("Error: ")+msg.Error.Error())
		} else {
			m.messages = append(m.messages, aiStyle.Render("AI: ")+msg.Content)
		}
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()
		m.saveState()
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

	// Viewport scrolling via arrows when text is empty
	if m.textarea.Value() == "" {
		switch msg.String() {
		case "up":
			m.viewport.LineUp(1)
			return m, nil
		case "down":
			m.viewport.LineDown(1)
			return m, nil
		}
	} else {
		// If text exists, maybe allow PgUp/PgDn for scrolling chat anyway?
		switch msg.String() {
		case "pgup":
			m.viewport.ViewUp()
			return m, nil
		case "pgdown":
			m.viewport.ViewDown()
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c":
		m.saveState()
		return m, tea.Quit
	case "enter":
		v := m.textarea.Value()
		if v == "" {
			return m, nil
		}
		if strings.HasPrefix(v, "/") {
			return m.handleSlashCommand(v)
		}
		m.messages = append(m.messages, userStyle.Render("You: ")+m.styleMessage(v))
		m.textarea.Reset()
		m.textarea.FocusedStyle.Text = lipgloss.NewStyle()
		m.suggestions = nil
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()
		m.saveState()
		return m, m.processRequest(v)
	default:
		val := m.textarea.Value()
		m.updateSuggestions(val)
		if strings.HasPrefix(val, "/") {
			m.textarea.FocusedStyle.Text = systemStyle
		} else {
			m.textarea.FocusedStyle.Text = lipgloss.NewStyle()
		}
	}
	return m, nil
}

func (m *model) styleMessage(v string) string {
	words := strings.Fields(v)
	for i, w := range words {
		if strings.HasPrefix(w, "/") {
			words[i] = systemStyle.Render(w)
		} else if strings.HasPrefix(w, "#") {
			words[i] = tagStyle.Render(w)
		}
	}
	return strings.Join(words, " ")
}

func (m *model) handlePerusalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m *model) updateSuggestions(val string) {
	m.suggestions = nil
	m.suggestionIdx = 0
	m.triggerChar = ""

	if val == "" {
		return
	}

	// Find the word being typed
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
	trimmed := strings.TrimPrefix(suggestion, m.triggerChar)
	replacement := m.triggerChar + trimmed

	if len(words) == 0 {
		m.textarea.SetValue(replacement)
	} else {
		words[len(words)-1] = replacement
		m.textarea.SetValue(strings.Join(words, " "))
	}
	m.textarea.SetCursor(len(m.textarea.Value()))
	m.suggestions = nil

	if m.triggerChar == "/" {
		return m.handleSlashCommand(m.textarea.Value())
	}
	
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
		msg += helpStyle.Render("🖼️ Saved PNG: "+pngPath)
	} else if svgContent != "" {
		// Middle Tier: SVG only
		msg += helpStyle.Render("📍 Saved SVG: "+svgPath)
		msg += "\n" + errorStyle.Render(" PNG fail: ") + helpStyle.Render("install ffmpeg/rsvg")
	} else {
		// Fallback Tier: ANSI only
		_ = os.WriteFile(ansiPath, []byte(rawView), 0644)
		msg += helpStyle.Render("📄 Saved ANSI: "+ansiPath)
	}

	m.messages = append(m.messages, msg)
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
	return m, nil
}

func (m *model) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	m.textarea.Reset()
	
	switch parts[0] {
	case "/help":
		m.messages = append(m.messages, systemStyle.Render(" COMMANDS ") + "\n" + helpStyle.Render("• /help    - Show this list\n• /status  - System resource snapshot\n• /cwd     - Show current directory\n• /version - Show current version info\n• /shot    - Take a beautiful TUI screenshot\n• /clear   - Clear chat history\n• /exit    - Quit vibeauracle"))
	case "/status":
		snapshot, _ := m.brain.GetSnapshot()
		status := fmt.Sprintf(systemStyle.Render(" SYSTEM ") + "\n" + helpStyle.Render("CPU: %.1f%% | Mem: %.1f%%"), snapshot.CPUUsage, snapshot.MemoryUsage)
		m.messages = append(m.messages, status)
	case "/cwd":
		snapshot, _ := m.brain.GetSnapshot()
		m.messages = append(m.messages, systemStyle.Render(" CWD ") + " " + helpStyle.Render(snapshot.WorkingDir))
	case "/version":
		m.messages = append(m.messages, systemStyle.Render(" VERSION ") + "\n" + helpStyle.Render(fmt.Sprintf("App: %s\nCommit: %s\nCompiler: %s", Version, Commit, runtime.Version())))
	case "/shot":
		return m.takeScreenshot()
	case "/show-tree":
		m.showTree = !m.showTree
		// trigger resize
		return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }
	case "/clear":
		m.messages = []string{}
		m.viewport.SetContent(systemStyle.Render(" Session Cleared "))
		return m, nil
	case "/exit":
		return m, tea.Quit
	default:
		m.messages = append(m.messages, errorStyle.Render(" Unknown Command: ") + parts[0])
	}
	
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
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
	for i, s := range items {
		selected := i == m.suggestionIdx
		
		style := suggestionStyle
		if selected {
			style = selectedSuggestionStyle
		}

		name := filepath.Base(s)
		if m.triggerChar == "/" {
			name = s
		}
		
		dir := filepath.Dir(s)
		if dir == "." || m.triggerChar == "/" {
			dir = ""
		}

		// Truncate name if path is too long
		namePart := name
		if len(namePart) > 20 {
			namePart = namePart[:17] + "..."
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

