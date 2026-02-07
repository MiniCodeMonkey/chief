package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/minicodemonkey/chief/internal/git"
)

// ghCheckResultMsg is sent when the gh CLI check completes.
type ghCheckResultMsg struct {
	installed     bool
	authenticated bool
	err           error
}

// FirstTimeSetupResult contains the result of the first-time setup flow.
type FirstTimeSetupResult struct {
	PRDName           string
	AddedGitignore    bool
	Cancelled         bool
	PushOnComplete    bool
	CreatePROnComplete bool
}

// FirstTimeSetupStep represents the current step in the setup flow.
type FirstTimeSetupStep int

const (
	StepGitignore FirstTimeSetupStep = iota
	StepPRDName
	StepPostCompletion
	StepGHError
)

// FirstTimeSetup is a TUI for first-time project setup.
type FirstTimeSetup struct {
	width  int
	height int

	step          FirstTimeSetupStep
	showGitignore bool // Whether to show the gitignore step

	// Gitignore step
	gitignoreSelected int // 0 = Yes, 1 = No

	// PRD name step
	prdName      string
	prdNameError string

	// Post-completion config step
	pushSelected     int // 0 = Yes, 1 = No
	createPRSelected int // 0 = Yes, 1 = No
	postCompField    int // 0 = push toggle, 1 = PR toggle

	// GH CLI error step
	ghErrorMsg      string
	ghErrorSelected int // 0 = Continue without PR, 1 = Try again

	// Result
	result FirstTimeSetupResult

	baseDir string
}

// NewFirstTimeSetup creates a new first-time setup TUI.
func NewFirstTimeSetup(baseDir string, showGitignore bool) *FirstTimeSetup {
	step := StepPRDName
	if showGitignore {
		step = StepGitignore
	}
	return &FirstTimeSetup{
		baseDir:           baseDir,
		showGitignore:     showGitignore,
		step:              step,
		gitignoreSelected: 0, // Default to "Yes"
		prdName:           "main",
		pushSelected:      0, // Default to "Yes"
		createPRSelected:  0, // Default to "Yes"
	}
}

// Init initializes the model.
func (f FirstTimeSetup) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// Update handles messages.
func (f FirstTimeSetup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height
		return f, nil

	case ghCheckResultMsg:
		return f.handleGHCheckResult(msg)

	case tea.KeyMsg:
		switch f.step {
		case StepGitignore:
			return f.handleGitignoreKeys(msg)
		case StepPRDName:
			return f.handlePRDNameKeys(msg)
		case StepPostCompletion:
			return f.handlePostCompletionKeys(msg)
		case StepGHError:
			return f.handleGHErrorKeys(msg)
		}
	}
	return f, nil
}

func (f FirstTimeSetup) handleGitignoreKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		f.result.Cancelled = true
		return f, tea.Quit

	case "up", "k", "left", "h":
		if f.gitignoreSelected > 0 {
			f.gitignoreSelected--
		}
		return f, nil

	case "down", "j", "right", "l":
		if f.gitignoreSelected < 1 {
			f.gitignoreSelected++
		}
		return f, nil

	case "y", "Y":
		f.gitignoreSelected = 0
		return f.confirmGitignore()

	case "n", "N":
		f.gitignoreSelected = 1
		return f.confirmGitignore()

	case "enter":
		return f.confirmGitignore()
	}
	return f, nil
}

func (f FirstTimeSetup) confirmGitignore() (tea.Model, tea.Cmd) {
	if f.gitignoreSelected == 0 {
		// User wants to add .chief to gitignore
		if err := git.AddChiefToGitignore(f.baseDir); err != nil {
			// Show error but continue
			f.prdNameError = "Warning: failed to add .chief to .gitignore"
		} else {
			f.result.AddedGitignore = true
		}
	}
	f.step = StepPRDName
	return f, nil
}

func (f FirstTimeSetup) handlePRDNameKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		f.result.Cancelled = true
		return f, tea.Quit

	case "esc":
		if f.showGitignore {
			// Go back to gitignore step
			f.step = StepGitignore
			f.prdNameError = ""
			return f, nil
		}
		f.result.Cancelled = true
		return f, tea.Quit

	case "enter":
		// Validate PRD name
		name := strings.TrimSpace(f.prdName)
		if name == "" {
			f.prdNameError = "Name cannot be empty"
			return f, nil
		}
		if !isValidPRDName(name) {
			f.prdNameError = "Name can only contain letters, numbers, hyphens, and underscores"
			return f, nil
		}
		f.result.PRDName = name
		f.step = StepPostCompletion
		return f, nil

	case "backspace":
		if len(f.prdName) > 0 {
			f.prdName = f.prdName[:len(f.prdName)-1]
			f.prdNameError = ""
		}
		return f, nil

	default:
		// Handle character input
		if len(msg.String()) == 1 {
			r := rune(msg.String()[0])
			// Only allow valid characters
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
				(r >= '0' && r <= '9') || r == '-' || r == '_' {
				f.prdName += string(r)
				f.prdNameError = ""
			}
		}
		return f, nil
	}
}

// isValidPRDName checks if a name is valid for a PRD.
func isValidPRDName(name string) bool {
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return validName.MatchString(name)
}

func (f FirstTimeSetup) handlePostCompletionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		f.result.Cancelled = true
		return f, tea.Quit

	case "esc":
		// Go back to PRD name step
		f.step = StepPRDName
		return f, nil

	case "up", "k":
		if f.postCompField > 0 {
			f.postCompField--
		}
		return f, nil

	case "down", "j":
		if f.postCompField < 1 {
			f.postCompField++
		}
		return f, nil

	case "left", "h":
		// Toggle to Yes (0)
		if f.postCompField == 0 {
			f.pushSelected = 0
		} else {
			f.createPRSelected = 0
		}
		return f, nil

	case "right", "l":
		// Toggle to No (1)
		if f.postCompField == 0 {
			f.pushSelected = 1
		} else {
			f.createPRSelected = 1
		}
		return f, nil

	case " ", "tab":
		// Toggle the current field
		if f.postCompField == 0 {
			f.pushSelected = 1 - f.pushSelected
		} else {
			f.createPRSelected = 1 - f.createPRSelected
		}
		return f, nil

	case "y", "Y":
		if f.postCompField == 0 {
			f.pushSelected = 0
		} else {
			f.createPRSelected = 0
		}
		return f, nil

	case "n", "N":
		if f.postCompField == 0 {
			f.pushSelected = 1
		} else {
			f.createPRSelected = 1
		}
		return f, nil

	case "enter":
		return f.confirmPostCompletion()
	}
	return f, nil
}

func (f FirstTimeSetup) confirmPostCompletion() (tea.Model, tea.Cmd) {
	f.result.PushOnComplete = f.pushSelected == 0
	f.result.CreatePROnComplete = f.createPRSelected == 0

	// If PR creation is enabled, validate gh CLI
	if f.result.CreatePROnComplete {
		return f, func() tea.Msg {
			installed, authenticated, err := git.CheckGHCLI()
			return ghCheckResultMsg{installed: installed, authenticated: authenticated, err: err}
		}
	}

	return f, tea.Quit
}

func (f FirstTimeSetup) handleGHCheckResult(msg ghCheckResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		f.ghErrorMsg = fmt.Sprintf("Error checking gh CLI: %s", msg.err.Error())
		f.ghErrorSelected = 0
		f.step = StepGHError
		return f, nil
	}

	if !msg.installed {
		f.ghErrorMsg = "GitHub CLI (gh) is not installed.\nInstall it from: https://cli.github.com"
		f.ghErrorSelected = 0
		f.step = StepGHError
		return f, nil
	}

	if !msg.authenticated {
		f.ghErrorMsg = "GitHub CLI (gh) is not authenticated.\nRun: gh auth login"
		f.ghErrorSelected = 0
		f.step = StepGHError
		return f, nil
	}

	// gh is installed and authenticated - done
	return f, tea.Quit
}

func (f FirstTimeSetup) handleGHErrorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		f.result.Cancelled = true
		return f, tea.Quit

	case "esc":
		// Go back to post-completion step
		f.step = StepPostCompletion
		return f, nil

	case "up", "k":
		if f.ghErrorSelected > 0 {
			f.ghErrorSelected--
		}
		return f, nil

	case "down", "j":
		if f.ghErrorSelected < 1 {
			f.ghErrorSelected++
		}
		return f, nil

	case "enter":
		if f.ghErrorSelected == 0 {
			// Continue without PR creation
			f.result.CreatePROnComplete = false
			return f, tea.Quit
		}
		// Try again
		return f, func() tea.Msg {
			installed, authenticated, err := git.CheckGHCLI()
			return ghCheckResultMsg{installed: installed, authenticated: authenticated, err: err}
		}
	}
	return f, nil
}

// View renders the TUI.
func (f FirstTimeSetup) View() string {
	switch f.step {
	case StepGitignore:
		return f.renderGitignoreStep()
	case StepPRDName:
		return f.renderPRDNameStep()
	case StepPostCompletion:
		return f.renderPostCompletionStep()
	case StepGHError:
		return f.renderGHErrorStep()
	default:
		return ""
	}
}

func (f FirstTimeSetup) renderGitignoreStep() string {
	modalWidth := min(65, f.width-10)
	if modalWidth < 45 {
		modalWidth = 45
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)
	content.WriteString(titleStyle.Render("Welcome to Chief!"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Message
	messageStyle := lipgloss.NewStyle().Foreground(TextColor)
	content.WriteString(messageStyle.Render("Would you like to add .chief to .gitignore?"))
	content.WriteString("\n\n")

	descStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(descStyle.Render("This keeps your PRD plans local and out of version control."))
	content.WriteString("\n")
	content.WriteString(descStyle.Render("Not required, but recommended if you prefer local-only plans."))
	content.WriteString("\n\n")

	// Options
	optionStyle := lipgloss.NewStyle().Foreground(TextColor)
	selectedStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	options := []struct {
		label string
		desc  string
	}{
		{"Yes, add .chief to .gitignore", "(Recommended)"},
		{"No, keep .chief in version control", ""},
	}

	for i, opt := range options {
		var line string
		if i == f.gitignoreSelected {
			line = selectedStyle.Render(fmt.Sprintf("▶ %s", opt.label))
			if opt.desc != "" {
				line += " " + lipgloss.NewStyle().Foreground(SuccessColor).Render(opt.desc)
			}
		} else {
			line = optionStyle.Render(fmt.Sprintf("  %s", opt.label))
			if opt.desc != "" {
				line += " " + lipgloss.NewStyle().Foreground(MutedColor).Render(opt.desc)
			}
		}
		content.WriteString(line)
		content.WriteString("\n")
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(footerStyle.Render("↑/↓: Navigate  Enter: Select  y/n: Quick select  Esc: Cancel"))

	// Modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth)

	modal := modalStyle.Render(content.String())

	return f.centerModal(modal)
}

func (f FirstTimeSetup) renderPRDNameStep() string {
	modalWidth := min(60, f.width-10)
	if modalWidth < 45 {
		modalWidth = 45
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)

	if f.showGitignore && f.result.AddedGitignore {
		content.WriteString(lipgloss.NewStyle().Foreground(SuccessColor).Render("✓ Added .chief to .gitignore"))
		content.WriteString("\n\n")
	}

	content.WriteString(titleStyle.Render("Create Your First PRD"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Message
	messageStyle := lipgloss.NewStyle().Foreground(TextColor)
	content.WriteString(messageStyle.Render("Enter a name for your PRD:"))
	content.WriteString("\n\n")

	// Input field
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0, 1).
		Width(modalWidth - 8)

	displayName := f.prdName
	if displayName == "" {
		displayName = " " // Show cursor position
	}
	content.WriteString(inputStyle.Render(displayName + "█"))
	content.WriteString("\n")

	// Error message
	if f.prdNameError != "" {
		errorStyle := lipgloss.NewStyle().Foreground(ErrorColor)
		content.WriteString("\n")
		content.WriteString(errorStyle.Render(f.prdNameError))
	}

	// Hint
	content.WriteString("\n")
	hintStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(hintStyle.Render("PRD will be created at: .chief/prds/" + f.prdName + "/"))

	// Footer
	content.WriteString("\n\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().Foreground(MutedColor)
	if f.showGitignore {
		content.WriteString(footerStyle.Render("Enter: Create PRD  Esc: Back  Ctrl+C: Cancel"))
	} else {
		content.WriteString(footerStyle.Render("Enter: Create PRD  Esc/Ctrl+C: Cancel"))
	}

	// Modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth)

	modal := modalStyle.Render(content.String())

	return f.centerModal(modal)
}

func (f FirstTimeSetup) renderPostCompletionStep() string {
	modalWidth := min(65, f.width-10)
	if modalWidth < 45 {
		modalWidth = 45
	}

	var content strings.Builder

	// Success indicators for previous steps
	successStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	if f.result.AddedGitignore {
		content.WriteString(successStyle.Render("✓ Added .chief to .gitignore"))
		content.WriteString("\n")
	}
	content.WriteString(successStyle.Render(fmt.Sprintf("✓ PRD: %s", f.result.PRDName)))
	content.WriteString("\n\n")

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)
	content.WriteString(titleStyle.Render("Post-Completion Settings"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Description
	descStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(descStyle.Render("When a PRD completes, Chief can automatically push"))
	content.WriteString("\n")
	content.WriteString(descStyle.Render("the branch and create a pull request for you."))
	content.WriteString("\n\n")

	// Toggle styles
	activeFieldStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	inactiveFieldStyle := lipgloss.NewStyle().Foreground(TextColor)
	yesStyle := lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	noStyle := lipgloss.NewStyle().Foreground(MutedColor)
	recommendedStyle := lipgloss.NewStyle().Foreground(SuccessColor)

	// Push toggle
	pushLabel := "Push branch to remote?"
	if f.postCompField == 0 {
		content.WriteString(activeFieldStyle.Render("▶ " + pushLabel))
	} else {
		content.WriteString(inactiveFieldStyle.Render("  " + pushLabel))
	}
	content.WriteString("  ")
	if f.pushSelected == 0 {
		content.WriteString(yesStyle.Render("[Yes]"))
		content.WriteString(" ")
		content.WriteString(noStyle.Render(" No "))
		content.WriteString(" ")
		content.WriteString(recommendedStyle.Render("(Recommended)"))
	} else {
		content.WriteString(noStyle.Render(" Yes "))
		content.WriteString(" ")
		content.WriteString(yesStyle.Render("[No]"))
	}
	content.WriteString("\n\n")

	// PR toggle
	prLabel := "Automatically create a pull request?"
	if f.postCompField == 1 {
		content.WriteString(activeFieldStyle.Render("▶ " + prLabel))
	} else {
		content.WriteString(inactiveFieldStyle.Render("  " + prLabel))
	}
	content.WriteString("  ")
	if f.createPRSelected == 0 {
		content.WriteString(yesStyle.Render("[Yes]"))
		content.WriteString(" ")
		content.WriteString(noStyle.Render(" No "))
		content.WriteString(" ")
		content.WriteString(recommendedStyle.Render("(Recommended)"))
	} else {
		content.WriteString(noStyle.Render(" Yes "))
		content.WriteString(" ")
		content.WriteString(yesStyle.Render("[No]"))
	}
	content.WriteString("\n\n")

	// Hint
	hintStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(hintStyle.Render("You can change these later with ,"))

	// Footer
	content.WriteString("\n\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(footerStyle.Render("↑/↓: Navigate  ←/→/Space: Toggle  y/n: Quick set  Enter: Continue  Esc: Back"))

	// Modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth)

	modal := modalStyle.Render(content.String())

	return f.centerModal(modal)
}

func (f FirstTimeSetup) renderGHErrorStep() string {
	modalWidth := min(60, f.width-10)
	if modalWidth < 45 {
		modalWidth = 45
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ErrorColor)
	content.WriteString(titleStyle.Render("GitHub CLI Issue"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Error message
	errorStyle := lipgloss.NewStyle().Foreground(ErrorColor)
	for _, line := range strings.Split(f.ghErrorMsg, "\n") {
		content.WriteString(errorStyle.Render(line))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Options
	optionStyle := lipgloss.NewStyle().Foreground(TextColor)
	selectedOptionStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	options := []string{
		"Continue without PR creation",
		"Try again",
	}

	for i, opt := range options {
		if i == f.ghErrorSelected {
			content.WriteString(selectedOptionStyle.Render(fmt.Sprintf("▶ %s", opt)))
		} else {
			content.WriteString(optionStyle.Render(fmt.Sprintf("  %s", opt)))
		}
		content.WriteString("\n")
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(footerStyle.Render("↑/↓: Navigate  Enter: Select  Esc: Back"))

	// Modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ErrorColor).
		Padding(1, 2).
		Width(modalWidth)

	modal := modalStyle.Render(content.String())

	return f.centerModal(modal)
}

func (f FirstTimeSetup) centerModal(modal string) string {
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := 0
	for _, line := range lines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	topPadding := (f.height - modalHeight) / 2
	leftPadding := (f.width - modalWidth) / 2

	if topPadding < 0 {
		topPadding = 0
	}
	if leftPadding < 0 {
		leftPadding = 0
	}

	var result strings.Builder

	for i := 0; i < topPadding; i++ {
		result.WriteString("\n")
	}

	leftPad := strings.Repeat(" ", leftPadding)
	for _, line := range lines {
		result.WriteString(leftPad)
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}

// GetResult returns the setup result.
func (f FirstTimeSetup) GetResult() FirstTimeSetupResult {
	return f.result
}

// RunFirstTimeSetup runs the first-time setup TUI and returns the result.
func RunFirstTimeSetup(baseDir string, showGitignore bool) (FirstTimeSetupResult, error) {
	setup := NewFirstTimeSetup(baseDir, showGitignore)
	p := tea.NewProgram(setup, tea.WithAltScreen())

	model, err := p.Run()
	if err != nil {
		return FirstTimeSetupResult{Cancelled: true}, err
	}

	if finalSetup, ok := model.(FirstTimeSetup); ok {
		return finalSetup.GetResult(), nil
	}

	return FirstTimeSetupResult{Cancelled: true}, nil
}
