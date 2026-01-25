package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	// Layout constants
	minWidth         = 80
	storiesPanelPct  = 35 // Stories panel takes 35% of width
	detailsPanelPct  = 65 // Details panel takes 65% of width
	headerHeight     = 3
	footerHeight     = 3  // Increased to accommodate activity line
	activityHeight   = 1
	progressBarWidth = 20
)

// renderDashboard renders the full dashboard view.
func (a *App) renderDashboard() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	header := a.renderHeader()
	footer := a.renderFooter()

	// Calculate content area height
	contentHeight := a.height - headerHeight - footerHeight - 2 // -2 for panel borders

	// Render panels
	storiesWidth := (a.width * storiesPanelPct / 100) - 2
	detailsWidth := a.width - storiesWidth - 4 // -4 for borders and gap

	storiesPanel := a.renderStoriesPanel(storiesWidth, contentHeight)
	detailsPanel := a.renderDetailsPanel(detailsWidth, contentHeight)

	// Join panels horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, storiesPanel, detailsPanel)

	// Stack header, content, and footer
	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// renderHeader renders the header with branding, state, iteration, and elapsed time.
func (a *App) renderHeader() string {
	// Branding
	brand := headerStyle.Render("chief")

	// State indicator - use the centralized style system
	stateStyle := GetStateStyle(a.state)
	state := stateStyle.Render(fmt.Sprintf("[%s]", a.state.String()))

	// Iteration count
	iteration := SubtitleStyle.Render(fmt.Sprintf("Iteration: %d", a.iteration))

	// Elapsed time
	elapsed := a.GetElapsedTime()
	elapsedStr := SubtitleStyle.Render(fmt.Sprintf("Time: %s", formatDuration(elapsed)))

	// Combine elements
	leftPart := lipgloss.JoinHorizontal(lipgloss.Center, brand, "  ", state)
	rightPart := lipgloss.JoinHorizontal(lipgloss.Center, iteration, "  ", elapsedStr)

	// Create the full header line with proper spacing
	spacing := strings.Repeat(" ", max(0, a.width-lipgloss.Width(leftPart)-lipgloss.Width(rightPart)-2))
	headerLine := lipgloss.JoinHorizontal(lipgloss.Center, leftPart, spacing, rightPart)

	// Add a border below
	border := DividerStyle.Render(strings.Repeat("─", a.width))

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, border)
}

// renderFooter renders the footer with keyboard shortcuts, PRD name, and activity line.
func (a *App) renderFooter() string {
	// Keyboard shortcuts (context-sensitive based on view and state)
	var shortcuts []string

	if a.viewMode == ViewLog {
		// Log view shortcuts
		shortcuts = []string{"t: dashboard", "l: prds", "j/k: scroll", "Ctrl+D/U: page", "g/G: top/bottom", "q: quit"}
	} else {
		// Dashboard view shortcuts
		switch a.state {
		case StateReady, StatePaused:
			shortcuts = []string{"s: start", "t: log", "l: prds", "↑/k: up", "↓/j: down", "q: quit"}
		case StateRunning:
			shortcuts = []string{"p: pause", "x: stop", "t: log", "l: prds", "↑/k: up", "↓/j: down", "q: quit"}
		case StateStopped, StateError:
			shortcuts = []string{"s: retry", "t: log", "l: prds", "↑/k: up", "↓/j: down", "q: quit"}
		default:
			shortcuts = []string{"t: log", "l: prds", "↑/k: up", "↓/j: down", "q: quit"}
		}
	}
	shortcutsStr := footerStyle.Render(strings.Join(shortcuts, "  │  "))

	// PRD name
	prdInfo := footerStyle.Render(fmt.Sprintf("PRD: %s", a.prdName))

	// Create footer line with proper spacing
	spacing := strings.Repeat(" ", max(0, a.width-lipgloss.Width(shortcutsStr)-lipgloss.Width(prdInfo)-2))
	footerLine := lipgloss.JoinHorizontal(lipgloss.Center, shortcutsStr, spacing, prdInfo)

	// Activity line
	activityLine := a.renderActivityLine()

	// Add border above
	border := DividerStyle.Render(strings.Repeat("─", a.width))

	return lipgloss.JoinVertical(lipgloss.Left, border, activityLine, footerLine)
}

// renderActivityLine renders the current activity status line.
func (a *App) renderActivityLine() string {
	activity := a.lastActivity
	if activity == "" {
		activity = "Ready to start"
	}

	// Truncate if too long
	maxLen := a.width - 4
	if len(activity) > maxLen && maxLen > 3 {
		activity = activity[:maxLen-3] + "..."
	}

	// Use the centralized activity style system
	activityStyle := GetActivityStyle(a.state)

	return activityStyle.Render(activity)
}

// renderStoriesPanel renders the stories list panel.
func (a *App) renderStoriesPanel(width, height int) string {
	var content strings.Builder

	// Panel title using centralized style
	title := PanelTitleStyle.Render("Stories")
	content.WriteString(title)
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", width-2)))
	content.WriteString("\n")

	// Story list
	listHeight := height - 5 // Account for title, border, and progress bar
	for i, story := range a.prd.UserStories {
		if i >= listHeight {
			// Show indicator that there are more stories
			moreStyle := lipgloss.NewStyle().Foreground(mutedColor)
			content.WriteString(moreStyle.Render(fmt.Sprintf("... and %d more", len(a.prd.UserStories)-i)))
			break
		}

		icon := GetStatusIcon(story.Passes, story.InProgress)

		// Truncate title to fit
		maxTitleLen := width - 12 // Account for icon, ID, and spacing
		displayTitle := story.Title
		if len(displayTitle) > maxTitleLen {
			displayTitle = displayTitle[:maxTitleLen-3] + "..."
		}

		line := fmt.Sprintf("%s %s %s", icon, story.ID, displayTitle)

		if i == a.selectedIndex {
			line = selectedStyle.Width(width - 2).Render(line)
		}

		content.WriteString(line)
		content.WriteString("\n")
	}

	// Pad remaining space
	linesWritten := min(len(a.prd.UserStories), listHeight) + 2 // +2 for title and divider
	for i := linesWritten; i < height-3; i++ {
		content.WriteString("\n")
	}

	// Progress bar
	content.WriteString(DividerStyle.Render(strings.Repeat("─", width-2)))
	content.WriteString("\n")
	progressBar := a.renderProgressBar(width - 4)
	content.WriteString(progressBar)

	return panelStyle.Width(width).Height(height).Render(content.String())
}

// renderDetailsPanel renders the details panel for the selected story.
func (a *App) renderDetailsPanel(width, height int) string {
	story := a.GetSelectedStory()
	if story == nil {
		return panelStyle.Width(width).Height(height).Render("No stories in PRD")
	}

	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render(story.Title))
	content.WriteString("\n\n")

	// Status and Priority with proper styling
	statusIcon := GetStatusIcon(story.Passes, story.InProgress)
	var statusText string
	var statusStyle lipgloss.Style
	if story.Passes {
		statusText = "Passed"
		statusStyle = statusPassedStyle
	} else if story.InProgress {
		statusText = "In Progress"
		statusStyle = statusInProgressStyle
	} else {
		statusText = "Pending"
		statusStyle = statusPendingStyle
	}
	content.WriteString(fmt.Sprintf("%s %s  │  Priority: %d\n", statusIcon, statusStyle.Render(statusText), story.Priority))
	content.WriteString(DividerStyle.Render(strings.Repeat("─", width-4)))
	content.WriteString("\n\n")

	// Description
	content.WriteString(labelStyle.Render("Description"))
	content.WriteString("\n")
	content.WriteString(wrapText(story.Description, width-4))
	content.WriteString("\n\n")

	// Acceptance Criteria
	content.WriteString(labelStyle.Render("Acceptance Criteria"))
	content.WriteString("\n")
	for _, criterion := range story.AcceptanceCriteria {
		wrapped := wrapText("• "+criterion, width-6)
		content.WriteString(wrapped)
		content.WriteString("\n")
	}

	return panelStyle.Width(width).Height(height).Render(content.String())
}

// renderProgressBar renders a progress bar showing completion percentage.
func (a *App) renderProgressBar(width int) string {
	percentage := a.GetCompletionPercentage()
	completedStories := 0
	totalStories := len(a.prd.UserStories)
	for _, s := range a.prd.UserStories {
		if s.Passes {
			completedStories++
		}
	}

	// Calculate bar width
	barWidth := width - 15 // Space for percentage and count
	if barWidth < 10 {
		barWidth = 10
	}

	filledWidth := int(float64(barWidth) * percentage / 100.0)
	emptyWidth := barWidth - filledWidth

	bar := progressBarFillStyle.Render(strings.Repeat("█", filledWidth)) +
		progressBarEmptyStyle.Render(strings.Repeat("░", emptyWidth))

	return fmt.Sprintf("%s %3.0f%% %d/%d", bar, percentage, completedStories, totalStories)
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// wrapText wraps text to fit within a given width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		if lineLen+wordLen+1 > width && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		}

		if lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}

		result.WriteString(word)
		lineLen += wordLen

		// Handle very long words
		if wordLen > width && i < len(words)-1 {
			result.WriteString("\n")
			lineLen = 0
		}
	}

	return result.String()
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// renderLogView renders the full-screen log view.
func (a *App) renderLogView() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	header := a.renderLogHeader()
	footer := a.renderFooter()

	// Calculate content area height
	contentHeight := a.height - headerHeight - footerHeight - 2

	// Render log content
	a.logViewer.SetSize(a.width-4, contentHeight)
	logContent := a.logViewer.Render()

	// Wrap in a panel
	logPanel := panelStyle.Width(a.width - 2).Height(contentHeight).Render(logContent)

	// Stack header, content, and footer
	return lipgloss.JoinVertical(lipgloss.Left, header, logPanel, footer)
}

// renderLogHeader renders the header for the log view.
func (a *App) renderLogHeader() string {
	// Branding
	brand := headerStyle.Render("chief")

	// View indicator
	viewIndicator := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		Render("[Log View]")

	// State indicator
	stateStyle := GetStateStyle(a.state)
	state := stateStyle.Render(fmt.Sprintf("[%s]", a.state.String()))

	// Iteration count
	iteration := SubtitleStyle.Render(fmt.Sprintf("Iteration: %d", a.iteration))

	// Auto-scroll indicator
	var scrollIndicator string
	if a.logViewer.IsAutoScrolling() {
		scrollIndicator = lipgloss.NewStyle().Foreground(SuccessColor).Render("[Auto-scroll]")
	} else {
		scrollIndicator = lipgloss.NewStyle().Foreground(MutedColor).Render("[Manual scroll]")
	}

	// Combine elements
	leftPart := lipgloss.JoinHorizontal(lipgloss.Center, brand, "  ", viewIndicator, "  ", state)
	rightPart := lipgloss.JoinHorizontal(lipgloss.Center, iteration, "  ", scrollIndicator)

	// Create the full header line with proper spacing
	spacing := strings.Repeat(" ", max(0, a.width-lipgloss.Width(leftPart)-lipgloss.Width(rightPart)-2))
	headerLine := lipgloss.JoinHorizontal(lipgloss.Center, leftPart, spacing, rightPart)

	// Add a border below
	border := DividerStyle.Render(strings.Repeat("─", a.width))

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, border)
}
