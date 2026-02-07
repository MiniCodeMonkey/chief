package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CompletionScreen manages the completion screen state shown when a PRD finishes.
type CompletionScreen struct {
	width  int
	height int

	prdName    string
	completed  int
	total      int
	branch     string
	commitCount int
	hasAutoActions bool // Whether push/PR auto-actions are configured
}

// NewCompletionScreen creates a new completion screen.
func NewCompletionScreen() *CompletionScreen {
	return &CompletionScreen{}
}

// Configure sets up the completion screen with PRD completion data.
func (c *CompletionScreen) Configure(prdName string, completed, total int, branch string, commitCount int, hasAutoActions bool) {
	c.prdName = prdName
	c.completed = completed
	c.total = total
	c.branch = branch
	c.commitCount = commitCount
	c.hasAutoActions = hasAutoActions
}

// SetSize sets the screen dimensions.
func (c *CompletionScreen) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// PRDName returns the PRD name shown on the completion screen.
func (c *CompletionScreen) PRDName() string {
	return c.prdName
}

// Branch returns the branch shown on the completion screen.
func (c *CompletionScreen) Branch() string {
	return c.branch
}

// HasBranch returns true if the completion screen has a branch set.
func (c *CompletionScreen) HasBranch() bool {
	return c.branch != ""
}

// Render renders the completion screen.
func (c *CompletionScreen) Render() string {
	modalWidth := min(60, c.width-10)
	modalHeight := min(20, c.height-6)

	if modalWidth < 30 {
		modalWidth = 30
	}
	if modalHeight < 10 {
		modalHeight = 10
	}

	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(SuccessColor).
		Padding(0, 1)
	content.WriteString(headerStyle.Render(fmt.Sprintf("PRD Complete! %s %d/%d stories", c.prdName, c.completed, c.total)))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Branch and commit info
	infoStyle := lipgloss.NewStyle().
		Foreground(TextColor).
		Padding(0, 1)

	if c.branch != "" {
		content.WriteString(infoStyle.Render(fmt.Sprintf("Branch: %s", c.branch)))
		content.WriteString("\n")

		commitLabel := "commit"
		if c.commitCount != 1 {
			commitLabel = "commits"
		}
		content.WriteString(infoStyle.Render(fmt.Sprintf("Commits: %d %s on branch", c.commitCount, commitLabel)))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Auto-actions hint
	if !c.hasAutoActions {
		hintStyle := lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(0, 1)
		content.WriteString(hintStyle.Render("Configure auto-push and PR in settings (,)"))
		content.WriteString("\n\n")
	}

	// Footer
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)

	var shortcuts []string
	if c.branch != "" {
		shortcuts = append(shortcuts, "m: merge")
		shortcuts = append(shortcuts, "c: clean")
	}
	shortcuts = append(shortcuts, "l: switch PRD")
	shortcuts = append(shortcuts, "q: quit")
	content.WriteString(footerStyle.Render(strings.Join(shortcuts, "  │  ")))

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(SuccessColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())

	// Center the modal on screen
	return centerModal(modal, c.width, c.height)
}

// centerModal centers a modal string on the screen.
func centerModal(modal string, screenWidth, screenHeight int) string {
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := 0
	for _, line := range lines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	topPadding := (screenHeight - modalHeight) / 2
	leftPadding := (screenWidth - modalWidth) / 2

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
