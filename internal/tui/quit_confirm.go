package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// QuitConfirmOption represents the user's choice in the quit confirmation dialog.
type QuitConfirmOption int

const (
	QuitOptionQuit   QuitConfirmOption = iota // Quit and stop loop
	QuitOptionCancel                          // Cancel
)

// QuitConfirmation manages the quit confirmation dialog state.
type QuitConfirmation struct {
	width       int
	height      int
	selectedIdx int
	message     string // Context-specific message
	quitLabel   string // Label for the quit option
	leaveOnly   bool   // If true, confirm leaves the view instead of quitting the app
}

// NewQuitConfirmation creates a new quit confirmation dialog.
func NewQuitConfirmation() *QuitConfirmation {
	return &QuitConfirmation{
		selectedIdx: 1, // Default to Cancel (safe choice)
	}
}

// SetSize sets the dialog dimensions.
func (q *QuitConfirmation) SetSize(width, height int) {
	q.width = width
	q.height = height
}

// MoveUp moves selection up.
func (q *QuitConfirmation) MoveUp() {
	if q.selectedIdx > 0 {
		q.selectedIdx--
	}
}

// MoveDown moves selection down.
func (q *QuitConfirmation) MoveDown() {
	if q.selectedIdx < 1 {
		q.selectedIdx++
	}
}

// GetSelected returns the currently selected option.
func (q *QuitConfirmation) GetSelected() QuitConfirmOption {
	if q.selectedIdx == 0 {
		return QuitOptionQuit
	}
	return QuitOptionCancel
}

// Reset resets the dialog state to defaults.
func (q *QuitConfirmation) Reset() {
	q.selectedIdx = 1 // Default to Cancel
	q.message = "A loop is currently running.\nExiting will stop the loop."
	q.quitLabel = "Quit and stop loop"
	q.leaveOnly = false
}

// SetContext sets context-specific message and quit label.
// If leaveOnly is true, confirming returns to the previous view instead of quitting.
func (q *QuitConfirmation) SetContext(message, quitLabel string, leaveOnly bool) {
	q.message = message
	q.quitLabel = quitLabel
	q.leaveOnly = leaveOnly
}

// IsLeaveOnly returns whether this dialog is for leaving a view (not quitting).
func (q *QuitConfirmation) IsLeaveOnly() bool {
	return q.leaveOnly
}

// Render renders the quit confirmation dialog.
func (q *QuitConfirmation) Render() string {
	modalWidth := min(55, q.width-10)
	if modalWidth < 40 {
		modalWidth = 40
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(WarningColor)
	title := "Quit Melliza?"
	if q.leaveOnly {
		title = "Leave PRD creation?"
	}
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Message
	messageStyle := lipgloss.NewStyle().Foreground(TextColor)
	for _, line := range strings.Split(q.message, "\n") {
		content.WriteString(messageStyle.Render(line))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Options
	optionStyle := lipgloss.NewStyle().Foreground(TextColor)
	selectedStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)

	options := []string{q.quitLabel, "Cancel"}
	for i, opt := range options {
		if i == q.selectedIdx {
			content.WriteString(selectedStyle.Render("▶ " + opt))
		} else {
			content.WriteString(optionStyle.Render("  " + opt))
		}
		content.WriteString("\n")
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(footerStyle.Render("↑/↓: Navigate  Enter: Select  Esc: Cancel"))

	// Modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(WarningColor).
		Padding(1, 2).
		Width(modalWidth)

	modal := modalStyle.Render(content.String())

	// Center on screen
	return q.centerModal(modal)
}

// centerModal centers the modal on the screen.
func (q *QuitConfirmation) centerModal(modal string) string {
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := 0
	for _, line := range lines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	topPadding := (q.height - modalHeight) / 2
	leftPadding := (q.width - modalWidth) / 2

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
