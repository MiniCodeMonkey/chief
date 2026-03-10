package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/lvcoi/melliza/internal/git"
)

// DiffViewer displays git diffs with syntax highlighting and scrolling.
type DiffViewer struct {
	vp        viewport.Model
	lines     []string
	width     int
	height    int
	stats     string
	baseDir   string
	storyID   string // Story ID whose commit diff is being shown (empty = full branch diff)
	noCommit  bool   // True when no commit was found for the selected story
	err       error
	loaded    bool
}

// NewDiffViewer creates a new diff viewer.
func NewDiffViewer(baseDir string) *DiffViewer {
	vp := viewport.New()
	vp.MouseWheelEnabled = false // app.go dispatches scroll externally
	vp.KeyMap = viewport.KeyMap{} // disable internal keybindings

	return &DiffViewer{
		vp:      vp,
		baseDir: baseDir,
	}
}

// SetSize sets the viewport dimensions.
func (d *DiffViewer) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.vp.SetWidth(width)
	d.vp.SetHeight(height)
}

// SetBaseDir updates the base directory used for loading diffs.
func (d *DiffViewer) SetBaseDir(dir string) {
	d.baseDir = dir
}

// Load fetches the latest git diff for the full branch.
func (d *DiffViewer) Load() {
	d.storyID = ""
	d.noCommit = false
	d.loadDiff("", "")
}

// LoadForStory fetches the git diff for a specific story's commit.
// If no commit is found, it shows a "not committed yet" message.
func (d *DiffViewer) LoadForStory(storyID, title string) {
	d.storyID = storyID

	// Find the commit for this story (match both ID and title to avoid
	// false positives from previous PRD runs with the same story IDs)
	commitHash, err := git.FindCommitForStory(d.baseDir, storyID, title)
	if err != nil || commitHash == "" {
		d.noCommit = true
		d.loaded = true
		d.err = nil
		d.lines = nil
		d.stats = ""
		d.vp.SetContent("")
		return
	}

	d.noCommit = false
	d.loadDiff(storyID, commitHash)
}

// loadDiff loads a diff, either for a specific commit or the full branch.
func (d *DiffViewer) loadDiff(storyID, commitHash string) {
	d.loaded = true

	var diff string
	var err error

	if commitHash != "" {
		diff, err = git.GetDiffForCommit(d.baseDir, commitHash)
	} else {
		diff, err = git.GetDiff(d.baseDir)
	}

	if err != nil {
		d.err = err
		d.lines = nil
		d.stats = ""
		d.vp.SetContent("")
		return
	}

	d.err = nil

	if strings.TrimSpace(diff) == "" {
		d.lines = nil
		d.stats = ""
		d.vp.SetContent("")
		return
	}

	d.lines = strings.Split(diff, "\n")
	d.vp.SetContent(d.renderStyledContent())

	if commitHash != "" {
		stats, err := git.GetDiffStatsForCommit(d.baseDir, commitHash)
		if err == nil {
			d.stats = stats
		}
	} else {
		stats, err := git.GetDiffStats(d.baseDir)
		if err == nil {
			d.stats = stats
		}
	}
}

// renderStyledContent pre-renders all diff lines with syntax coloring.
func (d *DiffViewer) renderStyledContent() string {
	var b strings.Builder
	for i, line := range d.lines {
		styled := d.styleLine(line)
		// Truncate to width
		if d.width > 0 && lipgloss.Width(styled) > d.width {
			if len(line) > d.width-3 {
				line = line[:d.width-3] + "..."
			}
			styled = d.styleLine(line)
		}
		b.WriteString(styled)
		if i < len(d.lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ScrollUp scrolls up one line.
func (d *DiffViewer) ScrollUp() {
	d.vp.ScrollUp(1)
}

// ScrollDown scrolls down one line.
func (d *DiffViewer) ScrollDown() {
	d.vp.ScrollDown(1)
}

// PageUp scrolls up half a page.
func (d *DiffViewer) PageUp() {
	d.vp.HalfPageUp()
}

// PageDown scrolls down half a page.
func (d *DiffViewer) PageDown() {
	d.vp.HalfPageDown()
}

// ScrollToTop scrolls to the top.
func (d *DiffViewer) ScrollToTop() {
	d.vp.GotoTop()
}

// ScrollToBottom scrolls to the bottom.
func (d *DiffViewer) ScrollToBottom() {
	d.vp.GotoBottom()
}

// ScrollPercent returns the current scroll percentage (0.0 to 1.0).
func (d *DiffViewer) ScrollPercent() float64 {
	return d.vp.ScrollPercent()
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
		if d.noCommit {
			return lipgloss.NewStyle().Foreground(WarningColor).Render("⚠ Not committed yet — " + d.storyID + " is still in progress")
		}
		if d.storyID != "" {
			return lipgloss.NewStyle().Foreground(MutedColor).Render("No changes for " + d.storyID)
		}
		return lipgloss.NewStyle().Foreground(MutedColor).Render("No changes detected")
	}

	return d.vp.View()
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
