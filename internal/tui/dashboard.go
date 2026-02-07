package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/minicodemonkey/chief/internal/prd"
)

const (
	// Layout constants
	minWidth             = 80
	narrowWidthThreshold = 100 // Below this, switch to stacked layout
	storiesPanelPct      = 35  // Stories panel takes 35% of width
	detailsPanelPct      = 65  // Details panel takes 65% of width
	headerHeight         = 5   // Increased to accommodate tab bar (brand line + tab bar + border)
	footerHeight         = 3   // Increased to accommodate activity line
	activityHeight       = 1
	progressBarWidth     = 20
)

// isNarrowMode returns true if the terminal width is below the threshold for stacked layout.
func (a *App) isNarrowMode() bool {
	return a.width < narrowWidthThreshold
}

// renderDashboard renders the full dashboard view.
func (a *App) renderDashboard() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	// Use stacked layout for narrow terminals
	if a.isNarrowMode() {
		return a.renderStackedDashboard()
	}

	header := a.renderHeader()
	footer := a.renderFooter()

	// Calculate content area height
	contentHeight := a.height - a.effectiveHeaderHeight() - footerHeight - 2 // -2 for panel borders

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

// renderStackedDashboard renders the dashboard with stacked layout for narrow terminals.
func (a *App) renderStackedDashboard() string {
	header := a.renderNarrowHeader()
	footer := a.renderNarrowFooter()

	// Calculate content area height
	contentHeight := a.height - a.effectiveHeaderHeight() - footerHeight - 2 // -2 for panel borders

	// Split height between stories (40%) and details (60%)
	storiesHeight := max((contentHeight*40)/100, 5)
	detailsHeight := contentHeight - storiesHeight - 1 // -1 for gap between panels

	panelWidth := a.width - 2 // Account for borders

	storiesPanel := a.renderStoriesPanel(panelWidth, storiesHeight)
	detailsPanel := a.renderDetailsPanel(panelWidth, detailsHeight)

	// Join panels vertically
	content := lipgloss.JoinVertical(lipgloss.Left, storiesPanel, detailsPanel)

	// Stack header, content, and footer
	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// getWorktreeInfo returns the branch and directory info for the current PRD.
// Returns empty strings if no branch is set (backward compatible).
func (a *App) getWorktreeInfo() (branch, dir string) {
	if a.manager == nil {
		return "", ""
	}
	instance := a.manager.GetInstance(a.prdName)
	if instance == nil || instance.Branch == "" {
		return "", ""
	}
	branch = instance.Branch
	if instance.WorktreeDir != "" {
		// Convert absolute worktree path to relative for display
		dir = fmt.Sprintf(".chief/worktrees/%s/", a.prdName)
	} else {
		dir = "./ (current directory)"
	}
	return branch, dir
}

// hasWorktreeInfo returns true if the current PRD has branch info to display.
func (a *App) hasWorktreeInfo() bool {
	branch, _ := a.getWorktreeInfo()
	return branch != ""
}

// effectiveHeaderHeight returns the header height accounting for worktree info line.
func (a *App) effectiveHeaderHeight() int {
	if a.hasWorktreeInfo() {
		return headerHeight + 1
	}
	return headerHeight
}

// renderWorktreeInfoLine renders the branch and directory info line for the header.
func (a *App) renderWorktreeInfoLine() string {
	branch, dir := a.getWorktreeInfo()
	if branch == "" {
		return ""
	}

	branchLabel := SubtitleStyle.Render("branch:")
	branchValue := lipgloss.NewStyle().Foreground(PrimaryColor).Render(" " + branch)
	dirLabel := SubtitleStyle.Render("  dir:")
	dirValue := lipgloss.NewStyle().Foreground(TextColor).Render(" " + dir)

	return lipgloss.JoinHorizontal(lipgloss.Center, "  ", branchLabel, branchValue, dirLabel, dirValue)
}

// renderHeader renders the header with branding, state, iteration, and elapsed time.
func (a *App) renderHeader() string {
	// Branding
	brand := headerStyle.Render("chief")

	// State indicator - use the centralized style system
	stateStyle := GetStateStyle(a.state)
	state := stateStyle.Render(fmt.Sprintf("[%s]", a.state.String()))

	// Iteration count (current/max)
	iteration := SubtitleStyle.Render(fmt.Sprintf("Iteration: %d/%d", a.iteration, a.maxIter))

	// Elapsed time
	elapsed := a.GetElapsedTime()
	elapsedStr := SubtitleStyle.Render(fmt.Sprintf("Time: %s", formatDuration(elapsed)))

	// Combine elements
	leftPart := lipgloss.JoinHorizontal(lipgloss.Center, brand, "  ", state)
	rightPart := lipgloss.JoinHorizontal(lipgloss.Center, iteration, "  ", elapsedStr)

	// Create the full header line with proper spacing
	spacing := strings.Repeat(" ", max(0, a.width-lipgloss.Width(leftPart)-lipgloss.Width(rightPart)-2))
	headerLine := lipgloss.JoinHorizontal(lipgloss.Center, leftPart, spacing, rightPart)

	// Tab bar
	tabBarLine := a.renderTabBar()

	// Worktree info line (only shown when branch is set)
	worktreeInfoLine := a.renderWorktreeInfoLine()

	// Add a border below
	border := DividerStyle.Render(strings.Repeat("─", a.width))

	if worktreeInfoLine != "" {
		return lipgloss.JoinVertical(lipgloss.Left, headerLine, tabBarLine, worktreeInfoLine, border)
	}
	return lipgloss.JoinVertical(lipgloss.Left, headerLine, tabBarLine, border)
}

// renderTabBar renders the PRD tab bar.
func (a *App) renderTabBar() string {
	if a.tabBar == nil {
		return ""
	}
	a.tabBar.SetSize(a.width)
	if a.isNarrowMode() {
		return a.tabBar.RenderCompact()
	}
	return a.tabBar.Render()
}

// renderNarrowHeader renders a condensed header for narrow terminals.
func (a *App) renderNarrowHeader() string {
	// Branding
	brand := headerStyle.Render("chief")

	// State indicator - use the centralized style system
	stateStyle := GetStateStyle(a.state)
	state := stateStyle.Render(fmt.Sprintf("[%s]", a.state.String()))

	// Condensed iteration and time
	elapsed := a.GetElapsedTime()
	iterTime := SubtitleStyle.Render(fmt.Sprintf("#%d %s", a.iteration, formatDuration(elapsed)))

	// Combine elements
	leftPart := lipgloss.JoinHorizontal(lipgloss.Center, brand, " ", state)
	rightPart := iterTime

	// Create the full header line with proper spacing
	spacing := strings.Repeat(" ", max(0, a.width-lipgloss.Width(leftPart)-lipgloss.Width(rightPart)-2))
	headerLine := lipgloss.JoinHorizontal(lipgloss.Center, leftPart, spacing, rightPart)

	// Tab bar (compact)
	tabBarLine := a.renderTabBar()

	// Worktree info line (only shown when branch is set)
	worktreeInfoLine := a.renderWorktreeInfoLine()

	// Add a border below
	border := DividerStyle.Render(strings.Repeat("─", a.width))

	if worktreeInfoLine != "" {
		return lipgloss.JoinVertical(lipgloss.Left, headerLine, tabBarLine, worktreeInfoLine, border)
	}
	return lipgloss.JoinVertical(lipgloss.Left, headerLine, tabBarLine, border)
}

// renderFooter renders the footer with keyboard shortcuts, PRD name, and activity line.
func (a *App) renderFooter() string {
	// Keyboard shortcuts (context-sensitive based on view and state)
	var shortcuts []string

	if a.viewMode == ViewLog {
		// Log view shortcuts
		shortcuts = []string{"t: dashboard", "n: new", "l: list", "1-9: switch", "?: help", "j/k: scroll", "q: quit"}
	} else {
		// Dashboard view shortcuts
		switch a.state {
		case StateReady, StatePaused:
			shortcuts = []string{"s: start", "t: log", "n: new", "l: list", "1-9: switch", "?: help", "q: quit"}
		case StateRunning:
			shortcuts = []string{"p: pause", "x: stop", "t: log", "n: new", "l: list", "1-9: switch", "?: help", "q: quit"}
		case StateStopped, StateError:
			shortcuts = []string{"s: retry", "t: log", "n: new", "l: list", "1-9: switch", "?: help", "q: quit"}
		default:
			shortcuts = []string{"t: log", "n: new", "l: list", "1-9: switch", "?: help", "q: quit"}
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

// renderNarrowFooter renders a condensed footer for narrow terminals.
func (a *App) renderNarrowFooter() string {
	// Condensed keyboard shortcuts for narrow mode
	var shortcuts []string

	if a.viewMode == ViewLog {
		// Log view shortcuts - condensed
		shortcuts = []string{"t", "n", "1-9", "?", "q"}
	} else {
		// Dashboard view shortcuts - condensed
		switch a.state {
		case StateReady, StatePaused:
			shortcuts = []string{"s", "t", "n", "1-9", "?", "q"}
		case StateRunning:
			shortcuts = []string{"p", "x", "t", "n", "1-9", "?", "q"}
		case StateStopped, StateError:
			shortcuts = []string{"s", "t", "n", "1-9", "?", "q"}
		default:
			shortcuts = []string{"t", "n", "1-9", "?", "q"}
		}
	}
	shortcutsStr := footerStyle.Render(strings.Join(shortcuts, " "))

	// PRD name - truncate if needed
	prdName := a.prdName
	maxPRDLen := 12
	if len(prdName) > maxPRDLen {
		prdName = prdName[:maxPRDLen-2] + ".."
	}
	prdInfo := footerStyle.Render(prdName)

	// Create footer line with proper spacing
	spacing := strings.Repeat(" ", max(0, a.width-lipgloss.Width(shortcutsStr)-lipgloss.Width(prdInfo)-2))
	footerLine := lipgloss.JoinHorizontal(lipgloss.Center, shortcutsStr, spacing, prdInfo)

	// Activity line - use narrower truncation
	activityLine := a.renderNarrowActivityLine()

	// Add border above
	border := DividerStyle.Render(strings.Repeat("─", a.width))

	return lipgloss.JoinVertical(lipgloss.Left, border, activityLine, footerLine)
}

// renderNarrowActivityLine renders the activity line for narrow terminals.
func (a *App) renderNarrowActivityLine() string {
	activity := a.lastActivity
	if activity == "" {
		activity = "Ready"
	}

	// More aggressive truncation for narrow mode
	maxLen := a.width - 2
	if len(activity) > maxLen && maxLen > 3 {
		activity = activity[:maxLen-3] + "..."
	}

	// Use the centralized activity style system
	activityStyle := GetActivityStyle(a.state)

	return activityStyle.Render(activity)
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
			// Pad line to full width to ensure background fills the entire row
			lineWidth := lipgloss.Width(line)
			targetWidth := width - 2
			if lineWidth < targetWidth {
				line = line + strings.Repeat(" ", targetWidth-lineWidth)
			}
			line = selectedStyle.Render(line)
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
	// Check for empty PRD state first
	if len(a.prd.UserStories) == 0 {
		return a.renderEmptyPRDPanel(width, height)
	}

	// Check for error state - show error details instead of story details
	if a.state == StateError {
		return a.renderErrorPanel(width, height)
	}

	story := a.GetSelectedStory()
	if story == nil {
		return panelStyle.Width(width).Height(height).Render("No stories in PRD")
	}

	var content strings.Builder

	// Show interrupted story warning at the top if applicable
	if a.hasInterruptedStory() && a.state == StateReady {
		content.WriteString(a.renderInterruptedWarning(width - 4))
		content.WriteString("\n")
	}

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

// renderErrorPanel renders the error details panel when in error state.
func (a *App) renderErrorPanel(width, height int) string {
	var content strings.Builder

	// Error header
	errorIcon := statusFailedStyle.Render(IconFailed)
	errorTitle := StateErrorStyle.Render("ERROR")
	content.WriteString(fmt.Sprintf("%s %s\n", errorIcon, errorTitle))
	content.WriteString(DividerStyle.Render(strings.Repeat("─", width-4)))
	content.WriteString("\n\n")

	// Error message
	content.WriteString(labelStyle.Render("Error Details"))
	content.WriteString("\n")
	if a.err != nil {
		errorMsg := a.err.Error()
		content.WriteString(wrapText(errorMsg, width-4))
	} else {
		content.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Render("Unknown error occurred"))
	}
	content.WriteString("\n\n")

	// Log file hint
	content.WriteString(DividerStyle.Render(strings.Repeat("─", width-4)))
	content.WriteString("\n\n")
	hintStyle := lipgloss.NewStyle().Foreground(WarningColor)
	content.WriteString(hintStyle.Render("💡 Tip: Check claude.log in the PRD directory for full error details."))
	content.WriteString("\n\n")

	// Retry instructions
	content.WriteString(labelStyle.Render("What to do"))
	content.WriteString("\n")
	content.WriteString("• Press ")
	content.WriteString(ShortcutKeyStyle.Render("s"))
	content.WriteString(" to retry\n")
	content.WriteString("• Press ")
	content.WriteString(ShortcutKeyStyle.Render("t"))
	content.WriteString(" to view the log\n")
	content.WriteString("• Press ")
	content.WriteString(ShortcutKeyStyle.Render("q"))
	content.WriteString(" to quit")

	return panelStyle.Width(width).Height(height).Render(content.String())
}

// renderEmptyPRDPanel renders a panel when there are no stories in the PRD.
func (a *App) renderEmptyPRDPanel(width, height int) string {
	var content strings.Builder

	// Centered empty state message
	emptyIcon := lipgloss.NewStyle().Foreground(MutedColor).Render("📋")
	emptyTitle := titleStyle.Render("No User Stories")
	content.WriteString(fmt.Sprintf("%s %s\n", emptyIcon, emptyTitle))
	content.WriteString(DividerStyle.Render(strings.Repeat("─", width-4)))
	content.WriteString("\n\n")

	// Instructions
	content.WriteString(lipgloss.NewStyle().Foreground(TextColor).Render("This PRD has no user stories defined."))
	content.WriteString("\n\n")

	content.WriteString(labelStyle.Render("To add stories:"))
	content.WriteString("\n")
	content.WriteString("• Press ")
	content.WriteString(ShortcutKeyStyle.Render("l"))
	content.WriteString(" then ")
	content.WriteString(ShortcutKeyStyle.Render("e"))
	content.WriteString(" to edit this PRD with Claude\n")
	content.WriteString("• Press ")
	content.WriteString(ShortcutKeyStyle.Render("l"))
	content.WriteString(" then ")
	content.WriteString(ShortcutKeyStyle.Render("n"))
	content.WriteString(" to create a new PRD with Claude\n")
	content.WriteString("\n")

	content.WriteString(DividerStyle.Render(strings.Repeat("─", width-4)))
	content.WriteString("\n\n")

	content.WriteString(SubtitleStyle.Render("PRD Location:"))
	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(PrimaryColor).Render(a.prdPath))

	return panelStyle.Width(width).Height(height).Render(content.String())
}

// hasInterruptedStory returns true if there's a story with inProgress: true.
func (a *App) hasInterruptedStory() bool {
	for _, story := range a.prd.UserStories {
		if story.InProgress {
			return true
		}
	}
	return false
}

// getInterruptedStory returns the interrupted story if one exists.
func (a *App) getInterruptedStory() *prd.UserStory {
	for i := range a.prd.UserStories {
		if a.prd.UserStories[i].InProgress {
			return &a.prd.UserStories[i]
		}
	}
	return nil
}

// renderInterruptedWarning renders a warning banner for interrupted stories.
func (a *App) renderInterruptedWarning(width int) string {
	story := a.getInterruptedStory()
	if story == nil {
		return ""
	}

	var content strings.Builder

	// Warning box
	warningStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#3D3000")).
		Foreground(WarningColor).
		Padding(0, 1)

	warningIcon := "⚠"
	warningText := fmt.Sprintf("%s Interrupted Story: %s (%s)", warningIcon, story.ID, truncateWithEllipsis(story.Title, width-30))
	content.WriteString(warningStyle.Width(width).Render(warningText))
	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Render("A previous session was interrupted. Press 's' to resume."))

	return content.String()
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

// truncateWithEllipsis truncates text to maxLen characters, adding "..." if truncated.
func truncateWithEllipsis(text string, maxLen int) string {
	if maxLen <= 3 {
		if len(text) > maxLen {
			return text[:maxLen]
		}
		return text
	}
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// renderLogView renders the full-screen log view.
func (a *App) renderLogView() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	var header, footer string
	if a.isNarrowMode() {
		header = a.renderNarrowLogHeader()
		footer = a.renderNarrowFooter()
	} else {
		header = a.renderLogHeader()
		footer = a.renderFooter()
	}

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

	// Iteration count (current/max)
	iteration := SubtitleStyle.Render(fmt.Sprintf("Iteration: %d/%d", a.iteration, a.maxIter))

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

// renderNarrowLogHeader renders a condensed header for the log view in narrow mode.
func (a *App) renderNarrowLogHeader() string {
	// Branding
	brand := headerStyle.Render("chief")

	// Condensed view indicator
	viewIndicator := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		Render("[Log]")

	// State indicator
	stateStyle := GetStateStyle(a.state)
	state := stateStyle.Render(fmt.Sprintf("[%s]", a.state.String()))

	// Condensed iteration and scroll indicator
	var scrollIcon string
	if a.logViewer.IsAutoScrolling() {
		scrollIcon = lipgloss.NewStyle().Foreground(SuccessColor).Render("▼")
	} else {
		scrollIcon = lipgloss.NewStyle().Foreground(MutedColor).Render("▽")
	}
	rightPart := SubtitleStyle.Render(fmt.Sprintf("#%d", a.iteration)) + " " + scrollIcon

	// Combine elements
	leftPart := lipgloss.JoinHorizontal(lipgloss.Center, brand, " ", viewIndicator, " ", state)

	// Create the full header line with proper spacing
	spacing := strings.Repeat(" ", max(0, a.width-lipgloss.Width(leftPart)-lipgloss.Width(rightPart)-2))
	headerLine := lipgloss.JoinHorizontal(lipgloss.Center, leftPart, spacing, rightPart)

	// Add a border below
	border := DividerStyle.Render(strings.Repeat("─", a.width))

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, border)
}
