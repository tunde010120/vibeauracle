package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/nathfavour/vibeauracle/brain"
)

type model struct {
	viewport    viewport.Model
	messages    []string
	textarea    textarea.Model
	err         error
	brain       *brain.Brain
	width       int
	height      int
	initialized bool
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
)

type chatState struct {
	Messages []string `json:"messages"`
	Input    string   `json:"input"`
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
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - 6
	case tea.KeyMsg:
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
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			
			m.saveState()

			// Process via brain
			return m, m.processRequest(v)

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
		m.messages = append(m.messages, systemStyle.Render(" VERSION ") + " " + helpStyle.Render(Version+" ("+Commit+")"))
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
	header := titleStyle.Render(" vibeauracle ") + " " + helpStyle.Render("v" + Version)
	border := strings.Repeat("─", m.width)
	if m.width > 20 {
		border = strings.Repeat("─", m.width-1)
	}

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s",
		header,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render(border),
		m.viewport.View(),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render(border),
		m.textarea.View(),
	) + "\n"
}

