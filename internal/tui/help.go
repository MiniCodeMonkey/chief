package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ShortcutCategory represents a category of keyboard shortcuts.
type ShortcutCategory struct {
	Name      string
	Shortcuts []Shortcut
}

// Shortcut represents a single keyboard shortcut.
type Shortcut struct {
	Key         string
	Description string
}

// HelpOverlay manages the help overlay state.
type HelpOverlay struct {
	width    int
	height   int
	viewMode ViewMode
}

// NewHelpOverlay creates a new help overlay.
func NewHelpOverlay() *HelpOverlay {
	return &HelpOverlay{}
}

// SetSize sets the overlay dimensions.
func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// SetViewMode sets the current view mode for context-aware shortcuts.
func (h *HelpOverlay) SetViewMode(mode ViewMode) {
	h.viewMode = mode
}

// GetCategories returns the shortcut categories for the current view.
func (h *HelpOverlay) GetCategories() []ShortcutCategory {
	// Common categories
	loopControl := ShortcutCategory{
		Name: "Loop Control",
		Shortcuts: []Shortcut{
			{Key: "s", Description: "Start loop"},
			{Key: "p", Description: "Pause (after iteration)"},
			{Key: "x", Description: "Stop immediately"},
			{Key: "+/-", Description: "Adjust max iterations"},
		},
	}

	views := ShortcutCategory{
		Name: "Views",
		Shortcuts: []Shortcut{
			{Key: "t", Description: "Toggle log view"},
			{Key: "?", Description: "Help overlay"},
		},
	}

	prdControl := ShortcutCategory{
		Name: "PRD Control",
		Shortcuts: []Shortcut{
			{Key: "1-9", Description: "Switch to PRD"},
			{Key: "n", Description: "Create new PRD"},
		},
	}

	general := ShortcutCategory{
		Name: "General",
		Shortcuts: []Shortcut{
			{Key: "q", Description: "Quit"},
			{Key: "Ctrl+C", Description: "Quit"},
			{Key: "Esc", Description: "Close overlay/modal"},
		},
	}

	// View-specific categories
	switch h.viewMode {
	case ViewLog:
		scrolling := ShortcutCategory{
			Name: "Scrolling",
			Shortcuts: []Shortcut{
				{Key: "j / ↓", Description: "Scroll down"},
				{Key: "k / ↑", Description: "Scroll up"},
				{Key: "Ctrl+D", Description: "Page down"},
				{Key: "Ctrl+U", Description: "Page up"},
				{Key: "g", Description: "Go to top"},
				{Key: "G", Description: "Go to bottom"},
			},
		}
		return []ShortcutCategory{loopControl, prdControl, views, scrolling, general}

	case ViewPicker:
		navigation := ShortcutCategory{
			Name: "Navigation",
			Shortcuts: []Shortcut{
				{Key: "Enter", Description: "Create PRD"},
				{Key: "Esc", Description: "Cancel"},
			},
		}
		return []ShortcutCategory{navigation, general}

	default: // ViewDashboard
		navigation := ShortcutCategory{
			Name: "Navigation",
			Shortcuts: []Shortcut{
				{Key: "j / ↓", Description: "Next story"},
				{Key: "k / ↑", Description: "Previous story"},
			},
		}
		return []ShortcutCategory{loopControl, prdControl, views, navigation, general}
	}
}

// Render renders the help overlay.
func (h *HelpOverlay) Render() string {
	// Modal dimensions
	modalWidth := min(70, h.width-10)
	modalHeight := min(24, h.height-6)

	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalHeight < 14 {
		modalHeight = 14
	}

	// Build modal content
	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Padding(0, 1)
	content.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Get categories based on current view
	categories := h.GetCategories()

	// Render categories in two columns
	leftCol := &strings.Builder{}
	rightCol := &strings.Builder{}
	colWidth := (modalWidth - 8) / 2

	for i, cat := range categories {
		col := leftCol
		if i >= (len(categories)+1)/2 {
			col = rightCol
		}
		h.renderCategory(col, cat, colWidth)
	}

	// Join columns horizontally
	leftLines := strings.Split(leftCol.String(), "\n")
	rightLines := strings.Split(rightCol.String(), "\n")

	// Ensure both columns have the same number of lines
	maxLines := max(len(leftLines), len(rightLines))
	for len(leftLines) < maxLines {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < maxLines {
		rightLines = append(rightLines, "")
	}

	// Combine columns
	for i := 0; i < maxLines; i++ {
		leftLine := leftLines[i]
		rightLine := rightLines[i]

		// Pad left line to column width
		leftPadding := colWidth - lipgloss.Width(leftLine)
		if leftPadding < 0 {
			leftPadding = 0
		}
		content.WriteString(leftLine)
		content.WriteString(strings.Repeat(" ", leftPadding+4))
		content.WriteString(rightLine)
		content.WriteString("\n")
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)
	content.WriteString(footerStyle.Render("Press ? or Esc to close"))

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())

	// Center the modal on screen
	return h.centerModal(modal)
}

// renderCategory renders a single category of shortcuts.
func (h *HelpOverlay) renderCategory(w *strings.Builder, cat ShortcutCategory, width int) {
	// Category header
	catStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)
	w.WriteString(catStyle.Render(cat.Name))
	w.WriteString("\n")

	// Shortcuts
	keyStyle := lipgloss.NewStyle().
		Foreground(TextBrightColor).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(TextColor)

	for _, shortcut := range cat.Shortcuts {
		key := keyStyle.Render(shortcut.Key)
		desc := descStyle.Render(shortcut.Description)

		// Calculate padding for alignment
		keyWidth := lipgloss.Width(key)
		padding := 10 - keyWidth
		if padding < 1 {
			padding = 1
		}

		w.WriteString("  ")
		w.WriteString(key)
		w.WriteString(strings.Repeat(" ", padding))
		w.WriteString(desc)
		w.WriteString("\n")
	}
	w.WriteString("\n")
}

// centerModal centers the modal on the screen.
func (h *HelpOverlay) centerModal(modal string) string {
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := 0
	for _, line := range lines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	// Calculate padding
	topPadding := (h.height - modalHeight) / 2
	leftPadding := (h.width - modalWidth) / 2

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
