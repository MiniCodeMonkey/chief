package tui

import (
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// confettiChars are the characters used for confetti particles.
var confettiChars = []string{"✦", "★", "●", "◆", "♦", "▲", "■", "♥", "✧", "⬥"}

// confettiColors are the colors used for confetti particles.
var confettiColors = []lipgloss.Color{
	SuccessColor,
	PrimaryColor,
	WarningColor,
	ErrorColor,
	lipgloss.Color("#FF6AC1"), // Pink
	lipgloss.Color("#FFD700"), // Gold
	lipgloss.Color("#FF8C00"), // Dark orange
}

// Particle represents a single confetti particle.
type Particle struct {
	x, y   float64
	vx, vy float64
	char   string
	color  lipgloss.Color
	life   int
}

// Confetti manages a collection of animated confetti particles.
type Confetti struct {
	particles []Particle
	width     int
	height    int
}

// NewConfetti creates a new confetti system with particles spread across the screen.
func NewConfetti(width, height int) *Confetti {
	c := &Confetti{
		width:  width,
		height: height,
	}

	count := 80 + rand.Intn(40) // 80-120 particles
	c.particles = make([]Particle, count)

	for i := range c.particles {
		c.particles[i] = Particle{
			x:    rand.Float64() * float64(width),
			y:    rand.Float64()*float64(height+10) - float64(height/2), // stagger: some above screen, some mid
			vx:   (rand.Float64() - 0.5) * 0.6,                         // lateral drift -0.3 to 0.3
			vy:   0.2 + rand.Float64()*0.4,                              // falling 0.2-0.6
			char: confettiChars[rand.Intn(len(confettiChars))],
			color: confettiColors[rand.Intn(len(confettiColors))],
			life: 80 + rand.Intn(120), // 80-200 ticks
		}
	}

	return c
}

// Tick advances all particles by one frame.
func (c *Confetti) Tick() {
	alive := c.particles[:0]
	for i := range c.particles {
		p := &c.particles[i]
		p.x += p.vx
		p.y += p.vy
		p.life--

		if p.life > 0 && p.y < float64(c.height) {
			alive = append(alive, *p)
		}
	}
	c.particles = alive
}

// Render draws all particles onto a character grid and returns it as a string.
func (c *Confetti) Render(width, height int) string {
	// Build a grid of cells
	grid := make([][]string, height)
	for i := range grid {
		grid[i] = make([]string, width)
		for j := range grid[i] {
			grid[i][j] = " "
		}
	}

	// Place particles
	for i := range c.particles {
		p := &c.particles[i]
		px := int(p.x)
		py := int(p.y)
		if px >= 0 && px < width && py >= 0 && py < height {
			style := lipgloss.NewStyle().Foreground(p.color)
			grid[py][px] = style.Render(p.char)
		}
	}

	// Build output
	var b strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			b.WriteString(grid[y][x])
		}
		if y < height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// HasParticles returns true if there are still active particles.
func (c *Confetti) HasParticles() bool {
	return len(c.particles) > 0
}
