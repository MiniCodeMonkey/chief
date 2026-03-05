package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ChatMode distinguishes between creating a new PRD and editing an existing one.
type ChatMode int

const (
	ChatModeCreate ChatMode = iota
	ChatModeEdit
)

// chatSpinnerTickMsg is sent to animate the waiting display in the creation chat.
type chatSpinnerTickMsg struct{}

// spinnerFrames are the braille spinner characters.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// chatRobotFrames are ASCII art frames for the waiting animation.
var chatRobotFrames = []string{
	"   ╭─────╮\n   │ ◉ ◉ │\n   │  ▽  │\n   ╰──┬──╯\n      │\n   ╭──┴──╮\n   │     │\n   ╰─────╯",
	"   ╭─────╮\n   │ ◉ ◉ │\n   │  ◇  │\n   ╰──┬──╯\n      │\n   ╭──┴──╮\n   │ ░░░ │\n   ╰─────╯",
	"   ╭─────╮\n   │ ◑ ◑ │\n   │  ▽  │\n   ╰──┬──╯\n      │\n   ╭──┴──╮\n   │ ▓▓▓ │\n   ╰─────╯",
	"   ╭─────╮\n   │ ◉ ◉ │\n   │  △  │\n   ╰──┬──╯\n      │\n   ╭──┴──╮\n   │ ███ │\n   ╰─────╯",
}

// chatWaitingJokes are shown while waiting for Gemini to respond.
var chatWaitingJokes = []string{
	"Why do programmers prefer dark mode? Because light attracts bugs.",
	"There are only 10 types of people: those who understand binary and those who don't.",
	"A SQL query walks into a bar, sees two tables and asks... 'Can I JOIN you?'",
	"!false — it's funny because it's true.",
	"Why do Java developers wear glasses? Because they can't C#.",
	"There's no place like 127.0.0.1.",
	"Algorithm: a word used by programmers when they don't want to explain what they did.",
	"It works on my machine. Ship it!",
	"99 little bugs in the code, 99 little bugs. Take one down, patch it around... 127 little bugs in the code.",
	"Debugging is like being the detective in a crime movie where you are also the murderer.",
	"I asked the AI to write a PRD. It wrote a PRD about writing PRDs.",
	"You're absolutely right. That's a great point. I completely agree.\n— Gemini, before doing what it was already going to do",
	"The AI said it was 95% confident. It was not.",
	"Prompt engineering: the art of saying 'no really, do what I said' in 47 different ways.",
	"The LLM hallucinated a library that doesn't exist.\nHonestly, the API looked pretty good though.",
	"AI will replace programmers any day now.\n— programmers, every year since 2022",
	"The code works and nobody knows why. The code breaks and nobody knows why.",
	"Homer: 'To start, press any key.' Where's the ANY key?!",
}

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
	mode       ChatMode
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

	// Waiting animation state
	spinnerFrame   int
	robotFrame     int
	jokeIndex      int
	lastJokeChange time.Time
	loadingStart   time.Time
}

// NewPRDCreationChat creates a new PRDCreationChat component.
func NewPRDCreationChat(baseDir, prdName, context string) *PRDCreationChat {
	ti := textinput.New()
	ti.Placeholder = "Type your response..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.SetWidth(50)

	vp := viewport.New()

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
		jokeIndex: rand.Intn(len(chatWaitingJokes)),
	}
}

// SetMode sets the chat mode (create or edit).
func (c *PRDCreationChat) SetMode(mode ChatMode) {
	c.mode = mode
}

// SetSize sets the dimensions for the chat component.
func (c *PRDCreationChat) SetSize(width, height int) {
	c.width = width
	c.height = height
	
	// Subtract header, footer, and borders
	vpWidth := width - 4
	vpHeight := height - 13 // Account for header, input field, footer, and borders

	if vpWidth < 1 {
		vpWidth = 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}

	c.viewport.SetWidth(vpWidth)
	c.viewport.SetHeight(vpHeight)
	c.input.SetWidth(vpWidth - 10)

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
	c.loadingStart = time.Now()
	c.lastJokeChange = time.Now()
	c.messages = append(c.messages, Message{Role: RoleSystem, Content: "Initializing PRD creation session..."})
	c.renderViewport()
	c.viewport.GotoBottom()

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
		c.viewport.GotoBottom()
		return nil
	}

	c.messages = append(c.messages, Message{Role: RoleUser, Content: content})
	c.input.SetValue("")
	c.loading = true
	c.loadingStart = time.Now()
	c.lastJokeChange = time.Now()
	c.renderViewport()
	c.viewport.GotoBottom()

	return c.runGemini(content, c.sessionID)
}

// runGemini executes Gemini in headless mode with stream-json output.
// Uses -p (headless) so Gemini completes the full agent loop including tool calls.
// Uses -e none to skip loading extensions (much faster startup).
func (c *PRDCreationChat) runGemini(prompt string, sessionID string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"--yolo", "--output-format", "stream-json", "-e", "none"}
		if sessionID != "" {
			args = append(args, "-r", sessionID, "-p", prompt)
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

		waitErr := cmd.Wait()

		// If we captured output, return it even if Gemini exited non-zero.
		// Gemini often exits with code 1 due to stdin closing, extension errors,
		// or API timeouts — but may have still produced useful output and written files.
		if lastAssistantMsg != "" || capturedSessionID != "" {
			return ChatEventMsg{
				Type:      "message",
				Content:   lastAssistantMsg,
				SessionID: capturedSessionID,
			}
		}

		if waitErr != nil {
			return ChatEventMsg{Type: "error", Content: waitErr.Error()}
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

	case tea.KeyPressMsg:
		// Viewport scrolling — always available
		switch msg.String() {
		case "pgup", "ctrl+u":
			c.viewport.HalfPageUp()
			return c, nil
		case "pgdown", "ctrl+d":
			c.viewport.HalfPageDown()
			return c, nil
		case "up":
			c.viewport.ScrollUp(1)
			return c, nil
		case "down":
			c.viewport.ScrollDown(1)
			return c, nil
		}

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

// advanceAnimation advances the waiting animation frames.
func (c *PRDCreationChat) advanceAnimation() {
	c.spinnerFrame = (c.spinnerFrame + 1) % len(spinnerFrames)
	// Robot animates slower — every 3rd tick
	if c.spinnerFrame%3 == 0 {
		c.robotFrame = (c.robotFrame + 1) % len(chatRobotFrames)
	}
	// Rotate joke every 8 seconds
	if time.Since(c.lastJokeChange) > 8*time.Second {
		c.jokeIndex = (c.jokeIndex + 1) % len(chatWaitingJokes)
		c.lastJokeChange = time.Now()
	}
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
			b.WriteString(renderGlamour(m.Content, c.viewport.Width()-2))
		case RoleSystem:
			b.WriteString(lipgloss.NewStyle().Italic(true).Foreground(MutedColor).Render(m.Content))
		}
		b.WriteString("\n\n")
	}

	if c.loading {
		elapsed := time.Since(c.loadingStart).Truncate(time.Second)
		spinner := spinnerFrames[c.spinnerFrame]
		robot := chatRobotFrames[c.robotFrame]
		joke := chatWaitingJokes[c.jokeIndex]

		// Spinner + status line
		statusLine := fmt.Sprintf("%s Gemini is thinking... (%s)", spinner, elapsed)
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render(statusLine))
		b.WriteString("\n\n")

		// Robot ASCII art
		b.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Render(robot))
		b.WriteString("\n\n")

		// Joke in a styled box
		jokeStyle := lipgloss.NewStyle().
			Italic(true).
			Foreground(WarningColor).
			PaddingLeft(3)
		b.WriteString(jokeStyle.Render("  " + joke))
	}

	c.viewport.SetContent(b.String())
}

// View renders the component.
func (c *PRDCreationChat) View() tea.View {
	if c.width < 5 {
		return tea.NewView("Initializing...")
	}

	var b strings.Builder

	// Header
	headerText := "PRD Creation Chat"
	if c.mode == ChatModeEdit {
		headerText = "PRD Edit Chat"
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render(headerText))
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
		doneText := "PRD completed! Press Enter to start implementation."
		if c.mode == ChatModeEdit {
			doneText = "PRD updated! Press Enter to convert and return to dashboard."
		}
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render(doneText))
	}

	// Footer with shortcuts
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(BorderColor).Render(strings.Repeat("─", c.width-4)))
	b.WriteString("\n")
	var shortcuts string
	if c.done {
		shortcuts = "Enter: convert  │  Esc: back  │  q: quit"
	} else if c.loading {
		shortcuts = "Esc: back  │  q: quit  │  pgup/pgdn: scroll"
	} else {
		shortcuts = "Enter: send  │  /exit: finish  │  Esc: back  │  q: quit  │  pgup/pgdn: scroll"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Padding(0, 1).Render(shortcuts))

	return tea.NewView(lipgloss.NewStyle().Padding(1, 2).Render(b.String()))
}
