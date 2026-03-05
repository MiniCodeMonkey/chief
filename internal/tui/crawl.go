package tui

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CrawlPhase represents the current phase of the crawl animation.
type CrawlPhase int

const (
	PhaseTitle   CrawlPhase = iota // Title fades in, holds, fades out
	PhaseCrawl                     // Text scrolls upward
	PhaseFadeOut                   // Global fade to black
	PhaseDone                      // Animation complete
)

// CrawlStar represents a single star in the background.
type CrawlStar struct {
	x, y   float64
	speed  float64 // downward drift speed
	phase  float64 // twinkle phase offset
	bright float64 // base brightness 0.0-1.0
}

// crawlText is the narrative text that scrolls upward.
const crawlText = `It is a period of rapid iteration.
Autonomous agents, striking from
hidden terminals, have won their
first victory against the endless
backlog of unimplemented features.

During the sprint, agent scouts
managed to parse the sacred PRD,
an armored document with enough
power to mass-produce user stories
for an entire product roadmap.

Pursued by the organization's
sinister deadline, Melliza races
through the codebase aboard her
worktree, custodian of the commit
history that can save her team
and restore velocity to
the galaxy....`

// titleText is shown during the title phase.
const titleText = "MELLIZA"

// Crawl manages the Star Wars-style text crawl animation.
type Crawl struct {
	width, height int
	stars         []CrawlStar
	lines         []string // wrapped crawl text lines
	scrollY       float64  // vertical offset for text (starts below screen, decrements)
	phase         CrawlPhase
	frame         int
	titleAlpha    float64 // 0.0-1.0 for title fade
	globalAlpha   float64 // 1.0 normally, fades to 0 in PhaseFadeOut
	paused        bool
}

// NewCrawl creates a new crawl animation.
func NewCrawl(w, h int) *Crawl {
	c := &Crawl{
		width:       w,
		height:      h,
		phase:       PhaseTitle,
		titleAlpha:  0.0,
		globalAlpha: 1.0,
	}
	c.initStars()
	c.wrapText(w)
	c.scrollY = float64(h) + 4
	return c
}

func (c *Crawl) initStars() {
	count := 150
	c.stars = make([]CrawlStar, count)
	for i := range c.stars {
		c.stars[i] = CrawlStar{
			x:     rand.Float64() * float64(c.width),
			y:     rand.Float64() * float64(c.height),
			speed: 0.02 + rand.Float64()*0.08,
			phase: rand.Float64() * math.Pi * 2,
			bright: 0.3 + rand.Float64()*0.7,
		}
	}
}

func (c *Crawl) wrapText(width int) {
	maxWidth := width / 2
	if maxWidth < 30 {
		maxWidth = 30
	}
	if maxWidth > 50 {
		maxWidth = 50
	}

	c.lines = nil
	for _, paragraph := range strings.Split(crawlText, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			c.lines = append(c.lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		var line string
		for _, word := range words {
			if line == "" {
				line = word
			} else if len(line)+1+len(word) <= maxWidth {
				line += " " + word
			} else {
				c.lines = append(c.lines, line)
				line = word
			}
		}
		if line != "" {
			c.lines = append(c.lines, line)
		}
	}
}

// SetSize updates dimensions and rebuilds stars.
func (c *Crawl) SetSize(w, h int) {
	c.width = w
	c.height = h
	c.initStars()
	c.wrapText(w)
}

// Tick advances the animation by one frame.
func (c *Crawl) Tick() {
	if c.paused {
		return
	}
	c.frame++

	// Animate stars: drift down + twinkle
	for i := range c.stars {
		s := &c.stars[i]
		s.y += s.speed
		if s.y >= float64(c.height) {
			s.y = 0
			s.x = rand.Float64() * float64(c.width)
		}
		// Twinkle via phase rotation
		s.phase += 0.05 + rand.Float64()*0.02
	}

	switch c.phase {
	case PhaseTitle:
		// Fade in for 40 frames, hold for 60, fade out for 40
		if c.frame <= 40 {
			c.titleAlpha = float64(c.frame) / 40.0
		} else if c.frame <= 100 {
			c.titleAlpha = 1.0
		} else if c.frame <= 140 {
			c.titleAlpha = 1.0 - float64(c.frame-100)/40.0
		} else {
			c.phase = PhaseCrawl
			c.frame = 0
		}

	case PhaseCrawl:
		c.scrollY -= 0.15
		// Check if all text has scrolled past the top
		totalTextHeight := float64(len(c.lines))
		if c.scrollY+totalTextHeight < -2 {
			c.phase = PhaseFadeOut
			c.frame = 0
		}

	case PhaseFadeOut:
		c.globalAlpha = 1.0 - float64(c.frame)/40.0
		if c.globalAlpha <= 0 {
			c.globalAlpha = 0
			c.phase = PhaseDone
		}

	case PhaseDone:
		// nothing
	}
}

// Render draws the current frame to a string.
func (c *Crawl) Render() string {
	if c.width <= 0 || c.height <= 0 {
		return ""
	}

	w := c.width
	h := c.height

	// Build grid
	grid := make([][]string, h)
	for y := range grid {
		grid[y] = make([]string, w)
		for x := range grid[y] {
			grid[y][x] = " "
		}
	}

	// Place stars
	starGlyphs := []string{"·", "+", "*"}
	for i := range c.stars {
		s := &c.stars[i]
		px := int(s.x)
		py := int(s.y)
		if px < 0 || px >= w || py < 0 || py >= h {
			continue
		}

		// Twinkle brightness
		twinkle := (math.Sin(s.phase) + 1.0) / 2.0 // 0.0-1.0
		brightness := s.bright * twinkle * c.globalAlpha

		// Pick glyph based on brightness
		gi := int(brightness * 2.99)
		if gi > 2 {
			gi = 2
		}
		if gi < 0 {
			gi = 0
		}

		v := int(brightness * 255)
		if v < 20 {
			continue
		}
		if v > 255 {
			v = 255
		}
		color := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", v, v, v))
		style := lipgloss.NewStyle().Foreground(color)
		grid[py][px] = style.Render(starGlyphs[gi])
	}

	// Render title or crawl text
	switch c.phase {
	case PhaseTitle:
		c.renderTitle(grid, w, h)
	case PhaseCrawl, PhaseFadeOut:
		c.renderCrawlText(grid, w, h)
	}

	// Build output string
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			b.WriteString(grid[y][x])
		}
		if y < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (c *Crawl) renderTitle(grid [][]string, w, h int) {
	if c.titleAlpha <= 0 {
		return
	}

	v := int(c.titleAlpha * c.globalAlpha * 255)
	if v < 1 {
		return
	}
	if v > 255 {
		v = 255
	}

	color := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", v, v, v))
	style := lipgloss.NewStyle().Foreground(color).Bold(true)

	// Center the title
	titleY := h / 2
	titleX := (w - len(titleText)) / 2
	if titleX < 0 {
		titleX = 0
	}

	rendered := style.Render(titleText)
	// Place as a single styled string at the start position
	if titleY >= 0 && titleY < h && titleX < w {
		// Clear the cells the title will occupy
		for i := 0; i < len(titleText) && titleX+i < w; i++ {
			grid[titleY][titleX+i] = ""
		}
		grid[titleY][titleX] = rendered
	}
}

func (c *Crawl) renderCrawlText(grid [][]string, w, h int) {
	centerX := w / 2

	for lineIdx, line := range c.lines {
		screenY := int(c.scrollY) + lineIdx
		if screenY < 0 || screenY >= h {
			continue
		}

		// Brightness based on Y position: dim at top, bright at bottom
		yFrac := float64(screenY) / float64(h)
		brightness := yFrac * c.globalAlpha

		v := int(brightness * 255)
		if v < 10 {
			continue
		}
		if v > 255 {
			v = 255
		}

		// Use amber/gold color for crawl text
		r := v
		g := int(float64(v) * 0.85)
		bv := 0
		color := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, bv))
		style := lipgloss.NewStyle().Foreground(color)

		// Center the line
		startX := centerX - len(line)/2
		if startX < 0 {
			startX = 0
		}

		for i, ch := range line {
			col := startX + i
			if col >= 0 && col < w {
				grid[screenY][col] = style.Render(string(ch))
			}
		}
	}
}

// IsDone returns true when the animation has finished.
func (c *Crawl) IsDone() bool {
	return c.phase == PhaseDone
}

// TogglePause toggles the pause state.
func (c *Crawl) TogglePause() {
	c.paused = !c.paused
}

// --- Bubbletea standalone program ---

type crawlModel struct {
	crawl *Crawl
}

type crawlTickMsg time.Time

func tickCrawlAnimation() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return crawlTickMsg(t)
	})
}

func (m crawlModel) Init() tea.Cmd {
	return tickCrawlAnimation()
}

func (m crawlModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.crawl.SetSize(msg.Width, msg.Height)
		return m, nil

	case crawlTickMsg:
		m.crawl.Tick()
		if m.crawl.IsDone() {
			return m, tea.Quit
		}
		return m, tickCrawlAnimation()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "space":
			m.crawl.TogglePause()
			return m, nil
		}
	}
	return m, nil
}

func (m crawlModel) View() tea.View {
	v := tea.NewView(m.crawl.Render())
	v.AltScreen = true
	return v
}

// RunCrawl runs the Star Wars crawl animation as a standalone bubbletea program.
func RunCrawl() error {
	c := NewCrawl(80, 24)
	m := crawlModel{crawl: c}
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
