package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// AutoActionState represents the progress of an auto-action (push or PR).
type AutoActionState int

const (
	AutoActionIdle       AutoActionState = iota // Not configured or not started
	AutoActionInProgress                        // Currently running
	AutoActionSuccess                           // Completed successfully
	AutoActionError                             // Failed with error
)

// StoryTiming records the duration of a completed story.
type StoryTiming struct {
	StoryID  string
	Title    string
	Duration time.Duration
}

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

	// Duration data
	totalDuration time.Duration
	storyTimings  []StoryTiming

	// Confetti animation
	confetti *Confetti

	// Auto-action state
	pushState    AutoActionState
	pushError    string
	prState      AutoActionState
	prError      string
	prURL        string
	prTitle      string
	spinnerFrame int
}

// NewCompletionScreen creates a new completion screen.
func NewCompletionScreen() *CompletionScreen {
	return &CompletionScreen{}
}

// Configure sets up the completion screen with PRD completion data.
func (c *CompletionScreen) Configure(prdName string, completed, total int, branch string, commitCount int, hasAutoActions bool, totalDuration time.Duration, storyTimings []StoryTiming) {
	c.prdName = prdName
	c.completed = completed
	c.total = total
	c.branch = branch
	c.commitCount = commitCount
	c.hasAutoActions = hasAutoActions
	c.totalDuration = totalDuration
	c.storyTimings = storyTimings
	// Reset auto-action state
	c.pushState = AutoActionIdle
	c.pushError = ""
	c.prState = AutoActionIdle
	c.prError = ""
	c.prURL = ""
	c.prTitle = ""
	c.spinnerFrame = 0
	// Initialize confetti
	c.confetti = NewConfetti(c.width, c.height)
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

// SetPushInProgress marks the push as in progress.
func (c *CompletionScreen) SetPushInProgress() {
	c.pushState = AutoActionInProgress
}

// SetPushSuccess marks the push as successful.
func (c *CompletionScreen) SetPushSuccess() {
	c.pushState = AutoActionSuccess
}

// SetPushError marks the push as failed with an error message.
func (c *CompletionScreen) SetPushError(errMsg string) {
	c.pushState = AutoActionError
	c.pushError = errMsg
}

// SetPRInProgress marks the PR creation as in progress.
func (c *CompletionScreen) SetPRInProgress() {
	c.prState = AutoActionInProgress
}

// SetPRSuccess marks the PR creation as successful.
func (c *CompletionScreen) SetPRSuccess(url, title string) {
	c.prState = AutoActionSuccess
	c.prURL = url
	c.prTitle = title
}

// SetPRError marks the PR creation as failed with an error message.
func (c *CompletionScreen) SetPRError(errMsg string) {
	c.prState = AutoActionError
	c.prError = errMsg
}

// Tick advances the spinner animation frame.
func (c *CompletionScreen) Tick() {
	c.spinnerFrame++
}

// TickConfetti advances the confetti animation by one frame.
func (c *CompletionScreen) TickConfetti() {
	if c.confetti != nil {
		c.confetti.Tick()
	}
}

// HasConfetti returns true if confetti is still animating.
func (c *CompletionScreen) HasConfetti() bool {
	return c.confetti != nil && c.confetti.HasParticles()
}

// IsAutoActionRunning returns true if any auto-action is currently in progress.
func (c *CompletionScreen) IsAutoActionRunning() bool {
	return c.pushState == AutoActionInProgress || c.prState == AutoActionInProgress
}

// Render renders the completion screen with confetti background.
func (c *CompletionScreen) Render() string {
	modalWidth := min(70, c.width-6)
	if modalWidth < 30 {
		modalWidth = 30
	}

	// Inner content width (inside padding and border)
	innerWidth := modalWidth - 6 // 2 padding each side + 2 border

	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(SuccessColor)
	content.WriteString(headerStyle.Render("🎉 PRD Complete!"))
	content.WriteString("\n")

	// Subtitle
	subtitleStyle := lipgloss.NewStyle().Foreground(TextColor)
	prdTitle := formatPRDTitle(c.prdName)
	content.WriteString(subtitleStyle.Render(fmt.Sprintf("%s — %d/%d stories", prdTitle, c.completed, c.total)))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", innerWidth)))
	content.WriteString("\n")

	// Total duration
	if c.totalDuration > 0 {
		content.WriteString("\n")
		durationStyle := lipgloss.NewStyle().Foreground(SuccessColor)
		content.WriteString(durationStyle.Render(fmt.Sprintf("Completed in %s", formatDuration(c.totalDuration))))
		content.WriteString("\n")
	}

	// Per-story timings
	if len(c.storyTimings) > 0 {
		content.WriteString("\n")
		content.WriteString(c.renderStoryTimings(innerWidth))
	}

	// Branch and commit info (combined to single line)
	content.WriteString("\n")
	if c.branch != "" {
		infoStyle := lipgloss.NewStyle().Foreground(TextColor)
		commitLabel := "commit"
		if c.commitCount != 1 {
			commitLabel = "commits"
		}
		content.WriteString(infoStyle.Render(fmt.Sprintf("Branch: %s  •  %d %s", c.branch, c.commitCount, commitLabel)))
		content.WriteString("\n")
	}

	// Auto-actions progress or hint
	if c.pushState != AutoActionIdle || c.prState != AutoActionIdle {
		content.WriteString(c.renderAutoActions(innerWidth))
	} else if !c.hasAutoActions {
		hintStyle := lipgloss.NewStyle().Foreground(MutedColor)
		content.WriteString(hintStyle.Render("Configure auto-push and PR in settings (,)"))
		content.WriteString("\n")
	}

	// Footer
	content.WriteString(DividerStyle.Render(strings.Repeat("─", innerWidth)))
	content.WriteString("\n")

	fStyle := lipgloss.NewStyle().Foreground(MutedColor)
	var shortcuts []string
	if c.branch != "" {
		shortcuts = append(shortcuts, "m: merge")
		shortcuts = append(shortcuts, "c: clean")
	}
	shortcuts = append(shortcuts, "l: switch PRD")
	shortcuts = append(shortcuts, "q: quit")
	content.WriteString(fStyle.Render(strings.Join(shortcuts, "  │  ")))

	// Calculate dynamic height
	modalHeight := c.calculateModalHeight()

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(SuccessColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())

	// Render confetti background and overlay modal
	if c.confetti != nil && c.confetti.HasParticles() {
		background := c.confetti.Render(c.width, c.height)
		return overlayModal(background, modal, c.width, c.height)
	}

	return centerModal(modal, c.width, c.height)
}

// calculateModalHeight determines the dynamic modal height based on content.
func (c *CompletionScreen) calculateModalHeight() int {
	// Base: header(1) + subtitle(1) + divider(1) + blank(1) + duration(1) + blank(1)
	//       + branch(1) + blank(1) + divider(1) + footer(1) + padding(2) = ~12
	base := 12

	// Story timings
	storyLines := len(c.storyTimings)
	maxStoryLines := c.height - base - 6
	if maxStoryLines < 3 {
		maxStoryLines = 3
	}
	if storyLines > maxStoryLines {
		storyLines = maxStoryLines + 1 // +1 for "... and N more"
	}
	if storyLines > 0 {
		storyLines++ // blank line before stories
	}

	// Auto-action lines
	autoLines := 0
	if c.pushState != AutoActionIdle {
		autoLines++
	}
	if c.prState != AutoActionIdle {
		autoLines++
		if c.prState == AutoActionSuccess {
			autoLines++ // URL line
		}
	}
	if !c.hasAutoActions && c.pushState == AutoActionIdle && c.prState == AutoActionIdle {
		autoLines++ // hint line
	}

	// No duration line if zero
	durationLine := 0
	if c.totalDuration > 0 {
		durationLine = 2 // blank + duration text
	}

	calculated := base + storyLines + autoLines + durationLine
	maxHeight := c.height - 4
	if maxHeight < 10 {
		maxHeight = 10
	}
	if calculated > maxHeight {
		calculated = maxHeight
	}
	if calculated < 10 {
		calculated = 10
	}
	return calculated
}

// renderStoryTimings renders the per-story timing list with mini bar charts.
func (c *CompletionScreen) renderStoryTimings(innerWidth int) string {
	var b strings.Builder

	checkStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	titleStyle := lipgloss.NewStyle().Foreground(TextColor)
	dotStyle := lipgloss.NewStyle().Foreground(MutedColor)
	durStyle := lipgloss.NewStyle().Foreground(TextColor)
	barStyle := lipgloss.NewStyle().Foreground(SuccessColor)

	// Find max duration for proportional bars
	var maxDur time.Duration
	for _, st := range c.storyTimings {
		if st.Duration > maxDur {
			maxDur = st.Duration
		}
	}

	maxBarWidth := 10
	// Layout: "✓ " + title + " " + dots + " " + duration + "  " + bar
	// Reserve: 2 (check+space) + 1 (space before dots) + 1 (space after dots) + 8 (duration) + 2 (gap) + bar
	fixedWidth := 2 + 1 + 1 + 8 + 2 + maxBarWidth
	maxTitleWidth := innerWidth - fixedWidth
	if maxTitleWidth < 10 {
		maxTitleWidth = 10
	}

	// Limit visible stories
	maxVisible := c.height - 16
	if maxVisible < 3 {
		maxVisible = 3
	}
	visible := c.storyTimings
	truncated := 0
	if len(visible) > maxVisible {
		truncated = len(visible) - maxVisible
		visible = visible[:maxVisible]
	}

	for _, st := range visible {
		// Truncate title if needed
		title := st.Title
		titleLen := lipgloss.Width(title)
		if titleLen > maxTitleWidth {
			title = title[:maxTitleWidth-1] + "…"
			titleLen = maxTitleWidth
		}

		// Duration string (right-aligned in 8 chars)
		durStr := formatDuration(st.Duration)
		if len(durStr) > 8 {
			durStr = durStr[:8]
		}

		// Dot leaders
		dotCount := innerWidth - 2 - titleLen - 1 - len(durStr) - 2 - maxBarWidth - 1
		if dotCount < 2 {
			dotCount = 2
		}
		dots := strings.Repeat(".", dotCount)

		// Mini bar
		barWidth := 0
		if maxDur > 0 {
			barWidth = int(float64(maxBarWidth) * float64(st.Duration) / float64(maxDur))
			if barWidth < 1 && st.Duration > 0 {
				barWidth = 1
			}
		}
		bar := strings.Repeat("█", barWidth)

		b.WriteString(checkStyle.Render("✓"))
		b.WriteString(" ")
		b.WriteString(titleStyle.Render(title))
		b.WriteString(" ")
		b.WriteString(dotStyle.Render(dots))
		b.WriteString(" ")
		b.WriteString(durStyle.Render(durStr))
		b.WriteString("  ")
		b.WriteString(barStyle.Render(bar))
		b.WriteString("\n")
	}

	if truncated > 0 {
		moreStyle := lipgloss.NewStyle().Foreground(MutedColor)
		b.WriteString(moreStyle.Render(fmt.Sprintf("  ... and %d more", truncated)))
		b.WriteString("\n")
	}

	return b.String()
}

// spinnerChars are the animation frames for the completion screen spinner.
var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// renderAutoActions renders the auto-action progress section.
func (c *CompletionScreen) renderAutoActions(innerWidth int) string {
	var lines strings.Builder

	infoStyle := lipgloss.NewStyle().Foreground(TextColor)
	successStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	errorStyle := lipgloss.NewStyle().Foreground(ErrorColor)
	spinnerStyle := lipgloss.NewStyle().Foreground(PrimaryColor)

	// Push status
	if c.pushState != AutoActionIdle {
		switch c.pushState {
		case AutoActionInProgress:
			frame := spinnerChars[c.spinnerFrame%len(spinnerChars)]
			lines.WriteString(spinnerStyle.Render(fmt.Sprintf("%s Pushing branch to remote...", frame)))
		case AutoActionSuccess:
			lines.WriteString(successStyle.Render("✓ Pushed branch to remote"))
		case AutoActionError:
			lines.WriteString(errorStyle.Render(fmt.Sprintf("✗ Push failed: %s", c.pushError)))
		}
		lines.WriteString("\n")
	}

	// PR status
	if c.prState != AutoActionIdle {
		switch c.prState {
		case AutoActionInProgress:
			frame := spinnerChars[c.spinnerFrame%len(spinnerChars)]
			lines.WriteString(spinnerStyle.Render(fmt.Sprintf("%s Creating pull request...", frame)))
		case AutoActionSuccess:
			lines.WriteString(successStyle.Render(fmt.Sprintf("✓ Created PR: %s", c.prTitle)))
			lines.WriteString("\n")
			lines.WriteString(infoStyle.Render(fmt.Sprintf("  %s", c.prURL)))
		case AutoActionError:
			lines.WriteString(errorStyle.Render(fmt.Sprintf("✗ PR creation failed: %s", c.prError)))
		}
		lines.WriteString("\n")
	}

	_ = innerWidth
	return lines.String()
}

// formatPRDTitle converts a kebab-case PRD name to title case.
func formatPRDTitle(name string) string {
	words := strings.Split(name, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// overlayModal composites a modal on top of a background, centering the modal.
func overlayModal(background, modal string, screenWidth, screenHeight int) string {
	bgLines := strings.Split(background, "\n")
	modalLines := strings.Split(modal, "\n")

	// Measure modal dimensions
	modalHeight := len(modalLines)
	modalWidth := 0
	for _, line := range modalLines {
		w := lipgloss.Width(line)
		if w > modalWidth {
			modalWidth = w
		}
	}

	// Calculate centering offsets
	offsetY := (screenHeight - modalHeight) / 2
	offsetX := (screenWidth - modalWidth) / 2
	if offsetY < 0 {
		offsetY = 0
	}
	if offsetX < 0 {
		offsetX = 0
	}

	// Pad background to full screen height
	for len(bgLines) < screenHeight {
		bgLines = append(bgLines, strings.Repeat(" ", screenWidth))
	}

	// Overlay modal lines onto background
	for i, mLine := range modalLines {
		bgIdx := offsetY + i
		if bgIdx >= len(bgLines) {
			break
		}

		mWidth := lipgloss.Width(mLine)
		if mWidth == 0 {
			continue
		}

		// Build the composited line: bg prefix + modal + bg suffix
		bgLine := bgLines[bgIdx]
		bgRunes := []rune(bgLine)

		// Pad bg line if needed
		bgVisualWidth := lipgloss.Width(bgLine)
		for bgVisualWidth < screenWidth {
			bgRunes = append(bgRunes, ' ')
			bgVisualWidth++
		}

		var result strings.Builder

		// Write background chars before modal
		prefixWidth := 0
		runeIdx := 0
		for runeIdx < len(bgRunes) && prefixWidth < offsetX {
			result.WriteRune(bgRunes[runeIdx])
			prefixWidth++
			runeIdx++
		}

		// Pad if background was shorter than offsetX
		for prefixWidth < offsetX {
			result.WriteByte(' ')
			prefixWidth++
		}

		// Write the modal line
		result.WriteString(mLine)

		bgLines[bgIdx] = result.String()
	}

	return strings.Join(bgLines[:screenHeight], "\n")
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
