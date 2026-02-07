package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BranchWarningOption represents the user's choice in the branch warning dialog.
type BranchWarningOption int

const (
	BranchOptionCreateWorktree   BranchWarningOption = iota // Create worktree + branch
	BranchOptionCreateBranch                                // Create branch only (no worktree)
	BranchOptionContinue                                    // Continue on current branch / run in same directory
	BranchOptionCancel                                      // Cancel
)

// DialogContext determines which set of options to show.
type DialogContext int

const (
	// DialogProtectedBranch: on a protected branch (main/master)
	DialogProtectedBranch DialogContext = iota
	// DialogAnotherPRDRunning: another PRD is already running in the same directory
	DialogAnotherPRDRunning
	// DialogNoConflicts: not protected, nothing else running in same dir
	DialogNoConflicts
)

// dialogOption represents a single option in the dialog.
type dialogOption struct {
	label       string              // Display label
	hint        string              // Path hint (e.g., ".chief/worktrees/auth/")
	recommended bool                // Whether this is the recommended option
	option      BranchWarningOption // The option value this maps to
}

// BranchWarning manages the branch warning dialog state.
type BranchWarning struct {
	width         int
	height        int
	currentBranch string
	prdName       string
	worktreePath  string // Relative worktree path (e.g., ".chief/worktrees/auth/")
	selectedIndex int
	editMode      bool   // Whether we're editing the branch name
	branchName    string // The current branch name (editable)
	context       DialogContext
	options       []dialogOption
}

// NewBranchWarning creates a new branch warning dialog.
func NewBranchWarning() *BranchWarning {
	return &BranchWarning{
		selectedIndex: 0,
	}
}

// SetSize sets the dialog dimensions.
func (b *BranchWarning) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// SetContext sets the branch, PRD context, and worktree path for the warning.
func (b *BranchWarning) SetContext(currentBranch, prdName, worktreePath string) {
	b.currentBranch = currentBranch
	b.prdName = prdName
	b.branchName = fmt.Sprintf("chief/%s", prdName)
	b.worktreePath = worktreePath
}

// SetDialogContext sets which context mode the dialog should display.
func (b *BranchWarning) SetDialogContext(ctx DialogContext) {
	b.context = ctx
	b.buildOptions()
}

// buildOptions creates the option list based on the dialog context.
func (b *BranchWarning) buildOptions() {
	switch b.context {
	case DialogProtectedBranch:
		b.options = []dialogOption{
			{
				label:       "Create worktree + branch",
				hint:        b.worktreePath,
				recommended: true,
				option:      BranchOptionCreateWorktree,
			},
			{
				label:  "Create branch only",
				hint:   "./ (current directory)",
				option: BranchOptionCreateBranch,
			},
			{
				label:  fmt.Sprintf("Continue on %s", b.currentBranch),
				hint:   "./ (current directory)",
				option: BranchOptionContinue,
			},
			{
				label:  "Cancel",
				option: BranchOptionCancel,
			},
		}
	case DialogAnotherPRDRunning:
		b.options = []dialogOption{
			{
				label:       "Create worktree",
				hint:        b.worktreePath,
				recommended: true,
				option:      BranchOptionCreateWorktree,
			},
			{
				label:  "Run in same directory",
				hint:   "./ (current directory)",
				option: BranchOptionContinue,
			},
			{
				label:  "Cancel",
				option: BranchOptionCancel,
			},
		}
	case DialogNoConflicts:
		b.options = []dialogOption{
			{
				label:       "Run in current directory",
				hint:        "./ (current directory)",
				recommended: true,
				option:      BranchOptionContinue,
			},
			{
				label:  "Create worktree + branch",
				hint:   b.worktreePath,
				option: BranchOptionCreateWorktree,
			},
			{
				label:  "Cancel",
				option: BranchOptionCancel,
			},
		}
	}
}

// GetSuggestedBranch returns the branch name (may be edited by user).
func (b *BranchWarning) GetSuggestedBranch() string {
	return b.branchName
}

// MoveUp moves selection up.
func (b *BranchWarning) MoveUp() {
	if b.selectedIndex > 0 {
		b.selectedIndex--
	}
}

// MoveDown moves selection down.
func (b *BranchWarning) MoveDown() {
	if b.selectedIndex < len(b.options)-1 {
		b.selectedIndex++
	}
}

// GetSelectedOption returns the currently selected option.
func (b *BranchWarning) GetSelectedOption() BranchWarningOption {
	if b.selectedIndex >= 0 && b.selectedIndex < len(b.options) {
		return b.options[b.selectedIndex].option
	}
	return BranchOptionCancel
}

// GetDialogContext returns the current dialog context.
func (b *BranchWarning) GetDialogContext() DialogContext {
	return b.context
}

// Reset resets the dialog state.
func (b *BranchWarning) Reset() {
	b.selectedIndex = 0
	b.editMode = false
	b.branchName = fmt.Sprintf("chief/%s", b.prdName)
}

// IsEditMode returns true if the branch name is being edited.
func (b *BranchWarning) IsEditMode() bool {
	return b.editMode
}

// StartEditMode enters edit mode for the branch name.
func (b *BranchWarning) StartEditMode() {
	b.editMode = true
}

// CancelEditMode exits edit mode.
func (b *BranchWarning) CancelEditMode() {
	b.editMode = false
}

// AddInputChar adds a character to the branch name.
func (b *BranchWarning) AddInputChar(ch rune) {
	// Only allow valid git branch name characters
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '/' {
		b.branchName += string(ch)
	}
}

// DeleteInputChar removes the last character from the branch name.
func (b *BranchWarning) DeleteInputChar() {
	if len(b.branchName) > 0 {
		b.branchName = b.branchName[:len(b.branchName)-1]
	}
}

// selectedOptionHasBranch returns true if the currently selected option involves branch creation.
func (b *BranchWarning) selectedOptionHasBranch() bool {
	opt := b.GetSelectedOption()
	return opt == BranchOptionCreateWorktree || opt == BranchOptionCreateBranch
}

// Render renders the branch warning dialog.
func (b *BranchWarning) Render() string {
	// Modal dimensions
	modalWidth := min(65, b.width-10)
	modalHeight := min(20, b.height-6)

	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalHeight < 14 {
		modalHeight = 14
	}

	// Build modal content
	var content strings.Builder

	// Title and message based on context
	b.renderHeader(&content, modalWidth)

	// Branch name (shown when any option involves a branch)
	b.renderBranchName(&content)

	// Options
	b.renderOptions(&content)

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().Foreground(MutedColor)
	if b.editMode {
		content.WriteString(footerStyle.Render("Enter: confirm  Esc: cancel edit"))
	} else {
		content.WriteString(footerStyle.Render("↑/↓: Navigate  Enter: Select  e: Edit branch  Esc: Cancel"))
	}

	// Modal box style - use warning color for protected branch, primary for others
	borderColor := PrimaryColor
	if b.context == DialogProtectedBranch {
		borderColor = WarningColor
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())

	// Center the modal on screen
	return b.centerModal(modal)
}

// renderHeader renders the dialog title and message.
func (b *BranchWarning) renderHeader(content *strings.Builder, modalWidth int) {
	titleStyle := lipgloss.NewStyle().Bold(true)

	switch b.context {
	case DialogProtectedBranch:
		content.WriteString(titleStyle.Foreground(WarningColor).Render("⚠️  Protected Branch Warning"))
		content.WriteString("\n")
		content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
		content.WriteString("\n\n")

		messageStyle := lipgloss.NewStyle().Foreground(TextColor)
		content.WriteString(messageStyle.Render(fmt.Sprintf("You are on the '%s' branch.", b.currentBranch)))
		content.WriteString("\n")
		content.WriteString(messageStyle.Render("It's recommended to isolate work in a worktree."))
		content.WriteString("\n\n")

	case DialogAnotherPRDRunning:
		content.WriteString(titleStyle.Foreground(PrimaryColor).Render("Directory In Use"))
		content.WriteString("\n")
		content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
		content.WriteString("\n\n")

		messageStyle := lipgloss.NewStyle().Foreground(TextColor)
		content.WriteString(messageStyle.Render("Another PRD is already running in this directory."))
		content.WriteString("\n")
		content.WriteString(messageStyle.Render("A worktree will avoid file conflicts."))
		content.WriteString("\n\n")

	case DialogNoConflicts:
		content.WriteString(titleStyle.Foreground(PrimaryColor).Render("Start PRD"))
		content.WriteString("\n")
		content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
		content.WriteString("\n\n")

		messageStyle := lipgloss.NewStyle().Foreground(TextColor)
		content.WriteString(messageStyle.Render("Choose where Claude should work:"))
		content.WriteString("\n\n")
	}
}

// renderBranchName renders the branch name display/editor.
func (b *BranchWarning) renderBranchName(content *strings.Builder) {
	branchLabelStyle := lipgloss.NewStyle().Foreground(MutedColor)

	if b.editMode {
		content.WriteString(branchLabelStyle.Render("Branch: "))
		inputStyle := lipgloss.NewStyle().
			Foreground(TextBrightColor).
			Background(lipgloss.Color("237"))
		cursorStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Blink(true)
		content.WriteString(inputStyle.Render(b.branchName))
		content.WriteString(cursorStyle.Render("▌"))
		content.WriteString("\n\n")
	} else {
		content.WriteString(branchLabelStyle.Render(fmt.Sprintf("Branch: %s", b.branchName)))
		content.WriteString("\n\n")
	}
}

// renderOptions renders the selectable options list.
func (b *BranchWarning) renderOptions(content *strings.Builder) {
	optionStyle := lipgloss.NewStyle().Foreground(TextColor)
	selectedStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(MutedColor)
	recommendedStyle := lipgloss.NewStyle().Foreground(SuccessColor)

	for i, opt := range b.options {
		isSelected := i == b.selectedIndex

		// Render option label
		if isSelected {
			content.WriteString(selectedStyle.Render(fmt.Sprintf("▶ %s", opt.label)))
		} else {
			content.WriteString(optionStyle.Render(fmt.Sprintf("  %s", opt.label)))
		}

		// Render recommended tag
		if opt.recommended {
			content.WriteString(" ")
			content.WriteString(recommendedStyle.Render("(Recommended)"))
		}

		content.WriteString("\n")

		// Render path hint (indented under the option)
		if opt.hint != "" {
			content.WriteString(hintStyle.Render(fmt.Sprintf("    → %s", opt.hint)))
			content.WriteString("\n")
		}
	}
}

// centerModal centers the modal on the screen.
func (b *BranchWarning) centerModal(modal string) string {
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := 0
	for _, line := range lines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	// Calculate padding
	topPadding := (b.height - modalHeight) / 2
	leftPadding := (b.width - modalWidth) / 2

	if topPadding < 0 {
		topPadding = 0
	}
	if leftPadding < 0 {
		leftPadding = 0
	}

	// Build centered content
	var result strings.Builder

	// Top padding
	for i := 0; i < topPadding; i++ {
		result.WriteString("\n")
	}

	// Modal lines with left padding
	leftPad := strings.Repeat(" ", leftPadding)
	for _, line := range lines {
		result.WriteString(leftPad)
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
