package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message role constants
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Message represents a single message in the chat history.
type Message struct {
	Role    string
	Content string
}

// PRDCreationChat represents the TUI component for interactive PRD creation.
type PRDCreationChat struct {
	prdName    string
	prdDir     string
	baseDir    string
	context    string
	messages   []Message
	sessionID  string
	input      textinput.Model
	viewport   viewport.Model
	width      int
	height     int
	loading    bool
	done       bool
	err        error
	
	// Track if Gemini has saved the PRD
	prdSaved bool
}

// NewPRDCreationChat creates a new PRDCreationChat component.
func NewPRDCreationChat(baseDir, prdName, context string) *PRDCreationChat {
	ti := textinput.New()
	ti.Placeholder = "Type your response..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 50

	vp := viewport.New(0, 0)

	return &PRDCreationChat{
		prdName:   prdName,
		prdDir:    filepath.Join(baseDir, ".melliza", "prds", prdName),
		baseDir:   baseDir,
		context:   context,
		messages:  make([]Message, 0),
		input:     ti,
		viewport:  vp,
		loading:   false,
		done:      false,
	}
}

// SetSize sets the dimensions for the chat component.
func (c *PRDCreationChat) SetSize(width, height int) {
	c.width = width
	c.height = height
	
	// Subtract header, footer, and borders
	vpWidth := width - 4
	vpHeight := height - 10 // Account for input field and borders
	
	c.viewport.Width = vpWidth
	c.viewport.Height = vpHeight
	c.input.Width = vpWidth - 10
	
	c.renderViewport()
}

// ChatEventMsg is sent when a chat event occurs.
type ChatEventMsg struct {
	Type      string // "init", "delta", "message", "done", "error"
	Content   string
	SessionID string
}

func (c *PRDCreationChat) Init() tea.Cmd {
	return nil // Actual initialization is triggered via a specific command
}

// StartSession initiates the chat session with Gemini.
func (c *PRDCreationChat) StartSession(prompt string) tea.Cmd {
	c.loading = true
	c.messages = append(c.messages, Message{Role: RoleSystem, Content: "Initializing PRD creation session..."})
	c.renderViewport()

	return c.runGemini(prompt, "")
}

// SendMessage sends a user message to Gemini.
func (c *PRDCreationChat) SendMessage() tea.Cmd {
	if c.loading || c.done {
		return nil
	}

	content := c.input.Value()
	if strings.TrimSpace(content) == "" {
		return nil
	}

	if content == "/exit" {
		c.done = true
		c.messages = append(c.messages, Message{Role: RoleUser, Content: content})
		c.input.SetValue("")
		c.renderViewport()
		return nil
	}

	c.messages = append(c.messages, Message{Role: RoleUser, Content: content})
	c.input.SetValue("")
	c.loading = true
	c.renderViewport()

	return c.runGemini(content, c.sessionID)
}

// runGemini executes Gemini in one-shot mode (with prompt or resume).
func (c *PRDCreationChat) runGemini(prompt string, sessionID string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"--output-format", "stream-json"}
		if sessionID != "" {
			args = append(args, "--resume", sessionID, "-p", prompt)
		} else {
			args = append(args, "-p", prompt)
		}

		cmd := exec.Command("gemini", args...)
		cmd.Dir = c.baseDir
		cmd.Stdin = nil // Ensure no stdin attachment

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return ChatEventMsg{Type: "error", Content: err.Error()}
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return ChatEventMsg{Type: "error", Content: err.Error()}
		}

		if err := cmd.Start(); err != nil {
			return ChatEventMsg{Type: "error", Content: err.Error()}
		}

		// Process stderr in a goroutine to avoid blocking
		go func() {
			// Log stderr to gemini.log for debugging
			prdDir := filepath.Join(c.baseDir, ".melliza", "prds", c.prdName)
			logPath := filepath.Join(prdDir, "gemini.log")
			logFile, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if logFile != nil {
				defer logFile.Close()
				scanner := bufio.NewScanner(stderr)
				for scanner.Scan() {
					logFile.WriteString("[stderr] " + scanner.Text() + "\n")
				}
			} else {
				// Sink stderr if log file can't be opened
				scanner := bufio.NewScanner(stderr)
				for scanner.Scan() {}
			}
		}()

		scanner := bufio.NewScanner(stdout)
		var lastAssistantMsg string
		var capturedSessionID string

		for scanner.Scan() {
			line := scanner.Text()
			var msg map[string]interface{}
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}

			msgType, _ := msg["type"].(string)
			switch msgType {
			case "init":
				sid, _ := msg["session_id"].(string)
				capturedSessionID = sid
			case "message":
				role, _ := msg["role"].(string)
				content, _ := msg["content"].(string)
				if role == "assistant" {
					lastAssistantMsg += content
					// We could emit deltas here for real-time streaming,
					// but let's stick to full messages for now to keep it simple
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			return ChatEventMsg{Type: "error", Content: err.Error()}
		}

		return ChatEventMsg{
			Type:      "message",
			Content:   lastAssistantMsg,
			SessionID: capturedSessionID,
		}
	}
}

// Update handles messages and updates the component state.
func (c *PRDCreationChat) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ChatEventMsg:
		switch msg.Type {
		case "message":
			c.loading = false
			if msg.SessionID != "" {
				c.sessionID = msg.SessionID
			}
			c.messages = append(c.messages, Message{Role: RoleAssistant, Content: msg.Content})
			
			// Check if PRD was saved (heuristic: check if prd.md exists)
			if strings.Contains(msg.Content, "prd.md") || strings.Contains(msg.Content, "saved") {
				c.prdSaved = true
			}

			// Check if Gemini is finished
			if strings.Contains(msg.Content, "/exit") || strings.Contains(msg.Content, "<melliza-complete/>") {
				c.done = true
			}
			
			c.renderViewport()
			c.viewport.GotoBottom()
		case "error":
			c.loading = false
			c.err = fmt.Errorf("%s", msg.Content)
			c.messages = append(c.messages, Message{Role: RoleSystem, Content: "Error: " + msg.Content})
			c.renderViewport()
			c.viewport.GotoBottom()
		}

	case tea.KeyMsg:
		if c.loading || c.done {
			return c, nil
		}

		switch msg.String() {
		case "enter":
			return c, c.SendMessage()
		}

		c.input, cmd = c.input.Update(msg)
		return c, cmd
	}

	return c, nil
}

// renderViewport prepares the content for the viewport.
func (c *PRDCreationChat) renderViewport() {
	var b strings.Builder

	for _, m := range c.messages {
		switch m.Role {
		case RoleUser:
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("You: "))
			b.WriteString("\n")
			b.WriteString(m.Content)
		case RoleAssistant:
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render("Gemini: "))
			b.WriteString("\n")
			b.WriteString(renderGlamour(m.Content, c.viewport.Width-2))
		case RoleSystem:
			b.WriteString(lipgloss.NewStyle().Italic(true).Foreground(MutedColor).Render(m.Content))
		}
		b.WriteString("\n\n")
	}

	if c.loading {
		b.WriteString(lipgloss.NewStyle().Italic(true).Foreground(MutedColor).Render("Gemini is thinking..."))
	}

	c.viewport.SetContent(b.String())
}

// View renders the component.
func (c *PRDCreationChat) View() string {
	var b strings.Builder

	// Header
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("PRD Creation Chat"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(BorderColor).Render(strings.Repeat("─", c.width-4)))
	b.WriteString("\n\n")

	// Chat history
	b.WriteString(c.viewport.View())
	b.WriteString("\n\n")

	// Input field
	if !c.done {
		b.WriteString(lipgloss.NewStyle().Foreground(BorderColor).Render(strings.Repeat("─", c.width-4)))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(PrimaryColor).Render(" > "))
		b.WriteString(c.input.View())
	} else {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render("PRD completed! Press Enter to start implementation."))
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
