package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/nathfavour/vibeauracle/brain"
)

type model struct {
	viewport      viewport.Model
	messages      []string
	textarea      textarea.Model
	err           error
	brain         *brain.Brain
	width         int
	height        int
	initialized   bool
	showTree      bool
	treeView      string
	suggestions   []string
	suggestionIdx int
	triggerChar   string // '/' or '#'
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
)

type chatState struct {
	Messages []string `json:"messages"`
	Input    string   `json:"input"`
}

var allCommands = []string{
	"/help", "/status", "/cwd", "/version", "/clear", "/exit", "/show-tree",
}

func initialModel(b *brain.Brain) *model {
	ta := textarea.New()
	ta.Placeholder = "Send a message or type / for commands..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 2000

	ta.SetWidth(60)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(60, 15)
	
	m := &model{
		textarea: ta,
		viewport: vp,
		messages: []string{},
		brain:    b,
	}

	// Attempt to restore state
	var state chatState
	if err := b.RecallState("chat_session", &state); err == nil {
		m.messages = state.Messages
		m.textarea.SetValue(state.Input)
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()
	} else {
		welcome := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("vibeauracle: The Alpha & Omega"),
			helpStyle.Render("Distributed, System-Intimate AI Engineering Ecosystem"),
			"",
			"Type "+systemStyle.Render("/help")+" to see available commands.",
		)
		m.viewport.SetContent(welcome)
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
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		if m.showTree {
			m.viewport.Width = msg.Width / 2
		}
		m.textarea.SetWidth(m.viewport.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - 6
	case tea.KeyMsg:
		// Suggestion navigation
		if len(m.suggestions) > 0 {
			switch msg.String() {
			case "tab", "down":
				m.suggestionIdx = (m.suggestionIdx + 1) % len(m.suggestions)
				return m, nil
			case "shift+tab", "up":
				m.suggestionIdx = (m.suggestionIdx - 1 + len(m.suggestions)) % len(m.suggestions)
				return m, nil
			case "enter":
				// Accept suggestion
				m.applySuggestion()
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.saveState()
			return m, tea.Quit
		case "enter":
			v := m.textarea.Value()
			if v == "" {
				return m, nil
			}

			// Handle slash commands
			if strings.HasPrefix(v, "/") {
				return m.handleSlashCommand(v)
			}

			// Add user message via a response update
			m.messages = append(m.messages, userStyle.Render("You: ")+v)
			m.textarea.Reset()
			m.updateSuggestions("") // Clear suggestions
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			
			m.saveState()

			// Process via brain
			return m, m.processRequest(v)
		default:
			// After normal keypress, update suggestions
			m.updateSuggestions(m.textarea.Value())
			m.updateDynamicPreview()
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

	return m, tea.Batch(tiCmd, vpCmd)
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

func (m *model) applySuggestion() {
	if len(m.suggestions) == 0 {
		return
	}
	
	val := m.textarea.Value()
	words := strings.Fields(val)
	
	suggestion := m.suggestions[m.suggestionIdx]
	// Avoid doubling trigger characters (e.g. //help or ##file)
	trimmed := strings.TrimPrefix(suggestion, m.triggerChar)
	replacement := m.triggerChar + trimmed

	if len(words) == 0 {
		m.textarea.SetValue(replacement + " ")
	} else {
		words[len(words)-1] = replacement
		m.textarea.SetValue(strings.Join(words, " ") + " ")
	}
	m.textarea.SetCursor(len(m.textarea.Value()))
	m.suggestions = nil
	m.updateDynamicPreview()
}

func (m *model) updateDynamicPreview() {
	if !m.showTree {
		return
	}

	val := m.textarea.Value()
	tags := m.extractTags(val)
	
	if len(tags) > 0 {
		// Show the last tag's content
		lastTag := tags[len(tags)-1]
		content, err := os.ReadFile(lastTag)
		if err == nil {
			m.treeView = string(content)
			return
		}
		
		// If it's a directory, show tree
		info, err := os.Stat(lastTag)
		if err == nil && info.IsDir() {
			m.treeView = m.renderTree(lastTag)
			return
		}
	}

	// Default to workspace tree
	m.treeView = m.renderTree(".")
}

func (m *model) extractTags(val string) []string {
	var tags []string
	words := strings.Fields(val)
	for _, w := range words {
		if strings.HasPrefix(w, "#") {
			path := strings.TrimPrefix(w, "#")
			if _, err := os.Stat(path); err == nil {
				tags = append(tags, path)
			}
		}
	}
	return tags
}

func (m *model) renderTree(root string) string {
	var sb strings.Builder
	sb.WriteString(systemStyle.Render(" EXPLORER: " + root) + "\n\n")
	
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != ".env" {
			continue
		}
		
		icon := "📄 "
		if entry.IsDir() {
			icon = "📁 "
		}
		sb.WriteString(icon + name + "\n")
	}
	return sb.String()
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

func (m *model) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	m.textarea.Reset()
	
	switch parts[0] {
	case "/help":
		m.messages = append(m.messages, systemStyle.Render(" COMMANDS ") + "\n" + helpStyle.Render("• /help    - Show this list\n• /status  - System resource snapshot\n• /cwd     - Show current directory\n• /version - Show current version info\n• /clear   - Clear chat history\n• /exit    - Quit vibeauracle"))
	case "/status":
		snapshot, _ := m.brain.GetSnapshot()
		status := fmt.Sprintf(systemStyle.Render(" SYSTEM ") + "\n" + helpStyle.Render("CPU: %.1f%% | Mem: %.1f%%"), snapshot.CPUUsage, snapshot.MemoryUsage)
		m.messages = append(m.messages, status)
	case "/cwd":
		snapshot, _ := m.brain.GetSnapshot()
		m.messages = append(m.messages, systemStyle.Render(" CWD ") + " " + helpStyle.Render(snapshot.WorkingDir))
	case "/version":
		m.messages = append(m.messages, systemStyle.Render(" VERSION ") + "\n" + helpStyle.Render(fmt.Sprintf("App: %s\nCommit: %s\nCompiler: %s", Version, Commit, runtime.Version())))
	case "/show-tree":
		m.showTree = !m.showTree
		if m.showTree {
			m.viewport.Width = m.width / 2
			m.treeView = m.renderTree(".")
		} else {
			m.viewport.Width = m.width
		}
		m.textarea.SetWidth(m.viewport.Width)
		m.messages = append(m.messages, systemStyle.Render(" Sideview Toggled "))
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
	border := strings.Repeat("─", m.width)
	if m.width > 20 {
		border = strings.Repeat("─", m.width-1)
	}

	mainContent := m.viewport.View()
	if m.showTree {
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top,
			m.viewport.View(),
			treeStyle.Render(m.treeView),
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

	if suggs := m.renderSuggestions(); suggs != "" {
		view += "\n" + suggs
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

