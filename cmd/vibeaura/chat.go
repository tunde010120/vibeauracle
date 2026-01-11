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

type chatState struct {
	Messages []string `json:"messages"`
	Input    string   `json:"input"`
}

func initialModel(b *brain.Brain) model {
	ta := textarea.New()
	ta.Placeholder = "Send a message or type / for commands..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 1000

	ta.SetWidth(60)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(60, 15)
	
	m := model{
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
		vp.SetContent(`Welcome to vibeauracle.
The Alpha & Omega of AI Engineering.
type /help for commands.`)
	}

	return m
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) saveState() {
	state := chatState{
		Messages: m.messages,
		Input:    m.textarea.Value(),
	}
	m.brain.StoreState("chat_session", state)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.viewport.Height = msg.Height - m.textarea.Height() - 5
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
			m.messages = append(m.messages, fmt.Sprintf("You: %s", v))
			m.textarea.Reset()
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			
			m.saveState()

			// Process via brain
			return m, m.processRequest(v)

		}
	case brain.Response:
		if msg.Error != nil {
			m.messages = append(m.messages, fmt.Sprintf("Error: %v", msg.Error))
		} else {
			m.messages = append(m.messages, fmt.Sprintf("AI: %s", msg.Content))
		}
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()
		m.saveState()
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) processRequest(content string) tea.Cmd {
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

func (m model) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "/help":
		m.messages = append(m.messages, "Available commands:\n/help - Show this\n/status - System resource snapshot\n/cwd - Show current directory\n/clear - Clear chat\n/exit - Quit")
	case "/status":
		snapshot, _ := m.brain.GetSnapshot()
		m.messages = append(m.messages, fmt.Sprintf("System Status:\nCPU: %.1f%%\nMem: %.1f%%", snapshot.CPUUsage, snapshot.MemoryUsage))
	case "/cwd":
		snapshot, _ := m.brain.GetSnapshot()
		m.messages = append(m.messages, fmt.Sprintf("Current Directory: %s", snapshot.WorkingDir))
	case "/clear":
		m.messages = []string{}
		m.viewport.SetContent("Chat cleared.")
	case "/exit":
		return m, tea.Quit
	default:
		m.messages = append(m.messages, fmt.Sprintf("Unknown command: %s", parts[0]))
	}
	m.textarea.Reset()
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render("vibeauracle"),
		m.viewport.View(),
		m.textarea.View(),
	) + "\n\n"
}

