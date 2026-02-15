package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/minicodemonkey/chief/internal/git"
)

// DiffViewer displays git diffs with syntax highlighting and scrolling.
type DiffViewer struct {
	lines      []string
	offset     int
	width      int
	height     int
	stats      string
	baseDir    string
	err        error
	loaded     bool
}

// NewDiffViewer creates a new diff viewer.
func NewDiffViewer(baseDir string) *DiffViewer {
	return &DiffViewer{
		baseDir: baseDir,
	}
}

// SetSize sets the viewport dimensions.
func (d *DiffViewer) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// Load fetches the latest git diff.
func (d *DiffViewer) Load() {
	d.offset = 0
	d.loaded = true

	diff, err := git.GetDiff(d.baseDir)
	if err != nil {
		d.err = err
		d.lines = nil
		d.stats = ""
		return
	}

	d.err = nil

	if strings.TrimSpace(diff) == "" {
		d.lines = nil
		d.stats = ""
		return
	}

	d.lines = strings.Split(diff, "\n")

	stats, err := git.GetDiffStats(d.baseDir)
	if err == nil {
		d.stats = stats
	}
}

// ScrollUp scrolls up one line.
func (d *DiffViewer) ScrollUp() {
	if d.offset > 0 {
		d.offset--
	}
}

// ScrollDown scrolls down one line.
func (d *DiffViewer) ScrollDown() {
	maxOffset := d.maxOffset()
	if d.offset < maxOffset {
		d.offset++
	}
}

// PageUp scrolls up half a page.
func (d *DiffViewer) PageUp() {
	d.offset -= d.height / 2
	if d.offset < 0 {
		d.offset = 0
	}
}

// PageDown scrolls down half a page.
func (d *DiffViewer) PageDown() {
	d.offset += d.height / 2
	maxOffset := d.maxOffset()
	if d.offset > maxOffset {
		d.offset = maxOffset
	}
}

// ScrollToTop scrolls to the top.
func (d *DiffViewer) ScrollToTop() {
	d.offset = 0
}

// ScrollToBottom scrolls to the bottom.
func (d *DiffViewer) ScrollToBottom() {
	d.offset = d.maxOffset()
}

func (d *DiffViewer) maxOffset() int {
	if len(d.lines) <= d.height {
		return 0
	}
	return len(d.lines) - d.height
}

// Render renders the diff view.
func (d *DiffViewer) Render() string {
	if !d.loaded {
		return lipgloss.NewStyle().Foreground(MutedColor).Render("Loading diff...")
	}

	if d.err != nil {
		return lipgloss.NewStyle().Foreground(ErrorColor).Render("Error loading diff: " + d.err.Error())
	}

	if len(d.lines) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Render("No changes detected")
	}

	var content strings.Builder

	// Render visible lines with syntax highlighting
	visibleEnd := d.offset + d.height
	if visibleEnd > len(d.lines) {
		visibleEnd = len(d.lines)
	}

	for i := d.offset; i < visibleEnd; i++ {
		line := d.lines[i]
		styled := d.styleLine(line)

		// Truncate to width
		if lipgloss.Width(styled) > d.width {
			// Re-style the truncated raw line
			if len(line) > d.width-3 {
				line = line[:d.width-3] + "..."
			}
			styled = d.styleLine(line)
		}

		content.WriteString(styled)
		if i < visibleEnd-1 {
			content.WriteString("\n")
		}
	}

	return content.String()
}

// styleLine applies diff syntax highlighting to a single line.
func (d *DiffViewer) styleLine(line string) string {
	addStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	removeStyle := lipgloss.NewStyle().Foreground(ErrorColor)
	hunkStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(TextBrightColor).Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(MutedColor)

	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return fileStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return hunkStyle.Render(line)
	case strings.HasPrefix(line, "+"):
		return addStyle.Render(line)
	case strings.HasPrefix(line, "-"):
		return removeStyle.Render(line)
	case strings.HasPrefix(line, "diff "):
		return fileStyle.Render(line)
	case strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file"):
		return metaStyle.Render(line)
	default:
		return line
	}
}

