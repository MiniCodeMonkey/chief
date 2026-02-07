package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// WorktreeSpinnerStep represents a step in the worktree setup process.
type WorktreeSpinnerStep int

const (
	SpinnerStepCreateBranch WorktreeSpinnerStep = iota
	SpinnerStepCreateWorktree
	SpinnerStepRunSetup
	SpinnerStepDone
)

// stepInfo holds the display info for each setup step.
type stepInfo struct {
	label    string
	complete bool
	active   bool
	errMsg   string
}

// WorktreeSpinner manages the worktree setup spinner overlay state.
type WorktreeSpinner struct {
	width  int
	height int

	prdName       string
	branchName    string
	defaultBranch string
	worktreePath  string // Relative path for display (e.g., ".chief/worktrees/auth/")
	setupCommand  string // Empty if no setup command configured

	currentStep  WorktreeSpinnerStep
	spinnerFrame int
	steps        []stepInfo
	errMsg       string // Overall error message
	cancelled    bool
}

// NewWorktreeSpinner creates a new worktree setup spinner.
func NewWorktreeSpinner() *WorktreeSpinner {
	return &WorktreeSpinner{}
}

// Configure sets up the spinner with the given parameters.
func (w *WorktreeSpinner) Configure(prdName, branchName, defaultBranch, worktreePath, setupCommand string) {
	w.prdName = prdName
	w.branchName = branchName
	w.defaultBranch = defaultBranch
	w.worktreePath = worktreePath
	w.setupCommand = setupCommand
	w.currentStep = SpinnerStepCreateBranch
	w.spinnerFrame = 0
	w.errMsg = ""
	w.cancelled = false

	// Build steps list
	w.steps = []stepInfo{
		{label: fmt.Sprintf("Creating branch '%s' from '%s'", branchName, defaultBranch)},
		{label: fmt.Sprintf("Creating worktree at %s", worktreePath)},
	}
	if setupCommand != "" {
		w.steps = append(w.steps, stepInfo{label: fmt.Sprintf("Running setup: %s", setupCommand)})
	}

	// Mark first step as active
	if len(w.steps) > 0 {
		w.steps[0].active = true
	}
}

// SetSize sets the spinner dimensions.
func (w *WorktreeSpinner) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// AdvanceStep marks the current step as complete and moves to the next.
func (w *WorktreeSpinner) AdvanceStep() {
	idx := int(w.currentStep)
	if idx < len(w.steps) {
		w.steps[idx].complete = true
		w.steps[idx].active = false
	}

	w.currentStep++

	// Skip setup step if no setup command
	if w.currentStep == SpinnerStepRunSetup && w.setupCommand == "" {
		w.currentStep = SpinnerStepDone
	}

	nextIdx := int(w.currentStep)
	if nextIdx < len(w.steps) {
		w.steps[nextIdx].active = true
	}
}

// SetError sets an error on the current step.
func (w *WorktreeSpinner) SetError(err string) {
	w.errMsg = err
	idx := int(w.currentStep)
	if idx < len(w.steps) {
		w.steps[idx].errMsg = err
		w.steps[idx].active = false
	}
}

// HasError returns true if there is an error.
func (w *WorktreeSpinner) HasError() bool {
	return w.errMsg != ""
}

// IsDone returns true if all steps are complete.
func (w *WorktreeSpinner) IsDone() bool {
	return w.currentStep >= SpinnerStepDone
}

// GetCurrentStep returns the current step.
func (w *WorktreeSpinner) GetCurrentStep() WorktreeSpinnerStep {
	return w.currentStep
}

// HasSetupCommand returns true if a setup command is configured.
func (w *WorktreeSpinner) HasSetupCommand() bool {
	return w.setupCommand != ""
}

// IsCancelled returns true if the user cancelled.
func (w *WorktreeSpinner) IsCancelled() bool {
	return w.cancelled
}

// Cancel marks the spinner as cancelled.
func (w *WorktreeSpinner) Cancel() {
	w.cancelled = true
}

// Tick advances the spinner animation frame.
func (w *WorktreeSpinner) Tick() {
	w.spinnerFrame++
}

// completedStepLabels returns the labels for completed steps (for display after done).
func (w *WorktreeSpinner) completedStepLabels() []string {
	var labels []string
	labels = append(labels, fmt.Sprintf("Created branch '%s' from '%s'", w.branchName, w.defaultBranch))
	labels = append(labels, fmt.Sprintf("Created worktree at %s", w.worktreePath))
	if w.setupCommand != "" {
		labels = append(labels, fmt.Sprintf("Ran setup: %s", w.setupCommand))
	}
	return labels
}

// Render renders the spinner overlay.
func (w *WorktreeSpinner) Render() string {
	modalWidth := min(65, w.width-10)
	if modalWidth < 40 {
		modalWidth = 40
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor)
	content.WriteString(titleStyle.Render("Setting up worktree"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerStyle := lipgloss.NewStyle().Foreground(PrimaryColor)
	checkStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	errorStyle := lipgloss.NewStyle().Foreground(ErrorColor)
	textStyle := lipgloss.NewStyle().Foreground(TextColor)
	mutedStyle := lipgloss.NewStyle().Foreground(MutedColor)

	// Render steps
	for _, step := range w.steps {
		if step.complete {
			content.WriteString(checkStyle.Render("✓"))
			content.WriteString(" ")
			// Show completed label
			completedLabel := strings.Replace(step.label, "Creating branch", "Created branch", 1)
			completedLabel = strings.Replace(completedLabel, "Creating worktree", "Created worktree", 1)
			completedLabel = strings.Replace(completedLabel, "Running setup", "Ran setup", 1)
			content.WriteString(textStyle.Render(completedLabel))
		} else if step.errMsg != "" {
			content.WriteString(errorStyle.Render("✗"))
			content.WriteString(" ")
			content.WriteString(errorStyle.Render(step.label))
			content.WriteString("\n")
			content.WriteString("  ")
			content.WriteString(errorStyle.Render(step.errMsg))
		} else if step.active {
			frame := spinnerFrames[w.spinnerFrame%len(spinnerFrames)]
			content.WriteString(spinnerStyle.Render(frame))
			content.WriteString(" ")
			content.WriteString(textStyle.Render(step.label))
		} else {
			content.WriteString(mutedStyle.Render("○"))
			content.WriteString(" ")
			content.WriteString(mutedStyle.Render(step.label))
		}
		content.WriteString("\n")
	}

	// Done state - show "Starting loop..."
	if w.IsDone() {
		content.WriteString("\n")
		content.WriteString(checkStyle.Render("Starting loop..."))
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().Foreground(MutedColor)
	if w.HasError() {
		content.WriteString(footerStyle.Render("Esc: Cancel and clean up"))
	} else if w.IsDone() {
		// No footer needed when transitioning
	} else {
		content.WriteString(footerStyle.Render("Esc: Cancel"))
	}

	// Modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth)

	modal := modalStyle.Render(content.String())

	return w.centerModal(modal)
}

// centerModal centers the modal on the screen.
func (w *WorktreeSpinner) centerModal(modal string) string {
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := 0
	for _, line := range lines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	topPadding := (w.height - modalHeight) / 2
	leftPadding := (w.width - modalWidth) / 2

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
