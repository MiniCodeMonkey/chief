package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/minicodemonkey/chief/internal/loop"
)

// LogEntry represents a single entry in the log viewer.
type LogEntry struct {
	Type      loop.EventType
	Text      string
	Tool      string
	ToolInput map[string]interface{}
	StoryID   string
}

// LogViewer manages the log viewport state.
type LogViewer struct {
	entries    []LogEntry
	scrollPos  int  // Current scroll position (top line index)
	height     int  // Viewport height (lines)
	width      int  // Viewport width
	autoScroll bool // Auto-scroll to bottom when new content arrives
}

// NewLogViewer creates a new log viewer.
func NewLogViewer() *LogViewer {
	return &LogViewer{
		entries:    make([]LogEntry, 0),
		scrollPos:  0,
		autoScroll: true,
	}
}

// AddEvent adds a loop event to the log.
func (l *LogViewer) AddEvent(event loop.Event) {
	entry := LogEntry{
		Type:      event.Type,
		Text:      event.Text,
		Tool:      event.Tool,
		ToolInput: event.ToolInput,
		StoryID:   event.StoryID,
	}

	// Filter out events we don't want to display
	switch event.Type {
	case loop.EventAssistantText, loop.EventToolStart, loop.EventToolResult,
		loop.EventStoryStarted, loop.EventComplete, loop.EventError, loop.EventRetrying:
		l.entries = append(l.entries, entry)
	default:
		// Skip iteration start, unknown events, etc.
		return
	}

	// Auto-scroll to bottom if enabled
	if l.autoScroll && l.height > 0 {
		l.scrollToBottom()
	}
}

// SetSize sets the viewport dimensions.
func (l *LogViewer) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// ScrollUp scrolls up by one line.
func (l *LogViewer) ScrollUp() {
	if l.scrollPos > 0 {
		l.scrollPos--
		l.autoScroll = false
	}
}

// ScrollDown scrolls down by one line.
func (l *LogViewer) ScrollDown() {
	maxScroll := l.maxScrollPos()
	if l.scrollPos < maxScroll {
		l.scrollPos++
	}
	// Re-enable auto-scroll if at bottom
	if l.scrollPos >= maxScroll {
		l.autoScroll = true
	}
}

// PageUp scrolls up by half a page.
func (l *LogViewer) PageUp() {
	halfPage := l.height / 2
	if halfPage < 1 {
		halfPage = 1
	}
	l.scrollPos -= halfPage
	if l.scrollPos < 0 {
		l.scrollPos = 0
	}
	l.autoScroll = false
}

// PageDown scrolls down by half a page.
func (l *LogViewer) PageDown() {
	halfPage := l.height / 2
	if halfPage < 1 {
		halfPage = 1
	}
	l.scrollPos += halfPage
	maxScroll := l.maxScrollPos()
	if l.scrollPos > maxScroll {
		l.scrollPos = maxScroll
	}
	// Re-enable auto-scroll if at bottom
	if l.scrollPos >= maxScroll {
		l.autoScroll = true
	}
}

// ScrollToTop scrolls to the top.
func (l *LogViewer) ScrollToTop() {
	l.scrollPos = 0
	l.autoScroll = false
}

// ScrollToBottom scrolls to the bottom.
func (l *LogViewer) scrollToBottom() {
	l.scrollPos = l.maxScrollPos()
	l.autoScroll = true
}

// ScrollToBottom (exported) scrolls to the bottom.
func (l *LogViewer) ScrollToBottom() {
	l.scrollToBottom()
}

// maxScrollPos returns the maximum scroll position.
func (l *LogViewer) maxScrollPos() int {
	totalLines := l.totalLines()
	maxPos := totalLines - l.height
	if maxPos < 0 {
		return 0
	}
	return maxPos
}

// totalLines calculates the total number of rendered lines.
func (l *LogViewer) totalLines() int {
	if l.width <= 0 {
		return len(l.entries)
	}

	total := 0
	for _, entry := range l.entries {
		total += l.entryHeight(entry)
	}
	return total
}

// entryHeight calculates how many lines an entry takes.
func (l *LogViewer) entryHeight(entry LogEntry) int {
	switch entry.Type {
	case loop.EventToolStart:
		// Tool display is now a single line
		return 1
	case loop.EventToolResult:
		// Tool result is typically compact
		return 1
	default:
		// Text entries: count wrapped lines
		if entry.Text == "" {
			return 1
		}
		wrapped := wrapText(entry.Text, l.width-4)
		return strings.Count(wrapped, "\n") + 1
	}
}

// getToolIcon returns an emoji icon for a tool name.
func getToolIcon(toolName string) string {
	switch toolName {
	case "Read":
		return "📖"
	case "Edit":
		return "✏️"
	case "Write":
		return "📝"
	case "Bash":
		return "🔨"
	case "Glob":
		return "🔍"
	case "Grep":
		return "🔎"
	case "Task":
		return "🤖"
	case "WebFetch":
		return "🌐"
	case "WebSearch":
		return "🌐"
	default:
		return "⚙️"
	}
}

// getToolArgument extracts the main argument from tool input for display.
func getToolArgument(toolName string, input map[string]interface{}) string {
	if input == nil {
		return ""
	}

	switch toolName {
	case "Read", "Edit", "Write":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			// Truncate long commands
			if len(cmd) > 60 {
				return cmd[:57] + "..."
			}
			return cmd
		}
	case "Glob":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "WebFetch", "WebSearch":
		if url, ok := input["url"].(string); ok {
			return url
		}
		if query, ok := input["query"].(string); ok {
			return query
		}
	case "Task":
		if desc, ok := input["description"].(string); ok {
			return desc
		}
	}

	return ""
}

// IsAutoScrolling returns whether auto-scroll is enabled.
func (l *LogViewer) IsAutoScrolling() bool {
	return l.autoScroll
}

// Clear clears all log entries.
func (l *LogViewer) Clear() {
	l.entries = make([]LogEntry, 0)
	l.scrollPos = 0
	l.autoScroll = true
}

// Render renders the log viewer content.
func (l *LogViewer) Render() string {
	if len(l.entries) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(1, 2)
		return emptyStyle.Render("No log entries yet. Start the loop to see Claude's activity.")
	}

	// Build all lines
	var allLines []string
	for _, entry := range l.entries {
		lines := l.renderEntry(entry)
		allLines = append(allLines, lines...)
	}

	// Apply scrolling
	startLine := l.scrollPos
	if startLine < 0 {
		startLine = 0
	}
	if startLine >= len(allLines) {
		startLine = len(allLines) - 1
		if startLine < 0 {
			startLine = 0
		}
	}

	endLine := startLine + l.height
	if endLine > len(allLines) {
		endLine = len(allLines)
	}

	visibleLines := allLines[startLine:endLine]

	// Add cursor indicator at bottom if streaming
	content := strings.Join(visibleLines, "\n")
	if l.autoScroll && len(l.entries) > 0 {
		lastEntry := l.entries[len(l.entries)-1]
		if lastEntry.Type == loop.EventAssistantText || lastEntry.Type == loop.EventToolStart {
			cursorStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Blink(true)
			content += "\n" + cursorStyle.Render("▌")
		}
	}

	return content
}

// renderEntry renders a single log entry as lines.
func (l *LogViewer) renderEntry(entry LogEntry) []string {
	switch entry.Type {
	case loop.EventToolStart:
		return l.renderToolCard(entry)
	case loop.EventToolResult:
		return l.renderToolResult(entry)
	case loop.EventStoryStarted:
		return l.renderStoryStarted(entry)
	case loop.EventComplete:
		return l.renderComplete(entry)
	case loop.EventError:
		return l.renderError(entry)
	case loop.EventRetrying:
		return l.renderRetrying(entry)
	default:
		return l.renderText(entry)
	}
}

// renderText renders an assistant text entry.
func (l *LogViewer) renderText(entry LogEntry) []string {
	if entry.Text == "" {
		return []string{}
	}

	textStyle := lipgloss.NewStyle().Foreground(TextColor)
	wrapped := wrapText(entry.Text, l.width-4)
	lines := strings.Split(wrapped, "\n")

	var result []string
	for _, line := range lines {
		result = append(result, textStyle.Render(line))
	}
	return result
}

// renderToolCard renders a tool call as a single styled line with icon and argument.
func (l *LogViewer) renderToolCard(entry LogEntry) []string {
	toolName := entry.Tool
	if toolName == "" {
		toolName = "unknown"
	}

	// Get icon and argument
	icon := getToolIcon(toolName)
	arg := getToolArgument(toolName, entry.ToolInput)

	// Style the output
	toolNameStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	argStyle := lipgloss.NewStyle().Foreground(TextColor)

	// Build the line: icon + tool name + argument
	var line string
	if arg != "" {
		// Truncate argument if too long
		maxArgLen := l.width - len(toolName) - 8
		if maxArgLen > 0 && len(arg) > maxArgLen {
			arg = arg[:maxArgLen-3] + "..."
		}
		line = fmt.Sprintf("%s %s %s", icon, toolNameStyle.Render(toolName), argStyle.Render(arg))
	} else {
		line = fmt.Sprintf("%s %s", icon, toolNameStyle.Render(toolName))
	}

	return []string{line}
}

// renderToolResult renders a tool result.
func (l *LogViewer) renderToolResult(entry LogEntry) []string {
	resultStyle := lipgloss.NewStyle().Foreground(MutedColor)
	checkStyle := lipgloss.NewStyle().Foreground(SuccessColor)

	// Show a compact result indicator, truncate to available width
	text := entry.Text
	maxLen := l.width - 8 // Account for "  ↳ " prefix and padding
	if maxLen < 20 {
		maxLen = 20
	}
	if len(text) > maxLen {
		text = text[:maxLen-3] + "..."
	}
	if text == "" {
		text = "(no output)"
	}

	return []string{resultStyle.Render(checkStyle.Render("  ↳ ") + text)}
}

// renderStoryStarted renders a story started marker.
func (l *LogViewer) renderStoryStarted(entry LogEntry) []string {
	storyStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		Padding(0, 1)

	dividerStyle := lipgloss.NewStyle().Foreground(PrimaryColor)
	divider := dividerStyle.Render(strings.Repeat("─", l.width-4))

	return []string{
		"",
		divider,
		storyStyle.Render(fmt.Sprintf("▶ Working on: %s", entry.StoryID)),
		divider,
		"",
	}
}

// renderComplete renders a completion message.
func (l *LogViewer) renderComplete(entry LogEntry) []string {
	completeStyle := lipgloss.NewStyle().
		Foreground(SuccessColor).
		Bold(true).
		Padding(0, 1)

	dividerStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	divider := dividerStyle.Render(strings.Repeat("═", l.width-4))

	return []string{
		"",
		divider,
		completeStyle.Render("✓ All stories complete!"),
		divider,
	}
}

// renderError renders an error message.
func (l *LogViewer) renderError(entry LogEntry) []string {
	errorStyle := lipgloss.NewStyle().
		Foreground(ErrorColor).
		Bold(true)

	text := entry.Text
	if text == "" {
		text = "An error occurred"
	}

	return []string{errorStyle.Render("✗ Error: " + text)}
}

// renderRetrying renders a retry message.
func (l *LogViewer) renderRetrying(entry LogEntry) []string {
	retryStyle := lipgloss.NewStyle().
		Foreground(WarningColor).
		Bold(true)

	text := entry.Text
	if text == "" {
		text = "Retrying..."
	}

	return []string{retryStyle.Render("🔄 " + text)}
}
