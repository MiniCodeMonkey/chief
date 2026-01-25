package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
)

// TabEntry represents a PRD tab in the tab bar.
type TabEntry struct {
	Name      string         // Directory name (e.g., "main", "feature-x")
	Path      string         // Full path to prd.json
	LoopState loop.LoopState // Current loop state from manager
	Completed int            // Number of completed stories
	Total     int            // Total number of stories
	Iteration int            // Current iteration if running
	IsActive  bool           // Whether this is the currently viewed PRD
}

// TabBar manages the always-visible PRD tab bar.
type TabBar struct {
	entries     []TabEntry
	activeIndex int
	width       int
	baseDir     string
	manager     *loop.Manager
	currentPRD  string
}

// NewTabBar creates a new tab bar.
func NewTabBar(baseDir, currentPRD string, manager *loop.Manager) *TabBar {
	t := &TabBar{
		entries:    make([]TabEntry, 0),
		baseDir:    baseDir,
		manager:    manager,
		currentPRD: currentPRD,
	}
	t.Refresh()
	return t
}

// Refresh reloads the list of PRDs from the .chief/prds/ directory.
func (t *TabBar) Refresh() {
	t.entries = make([]TabEntry, 0)

	prdsDir := filepath.Join(t.baseDir, ".chief", "prds")

	// Read the prds directory
	dirEntries, err := os.ReadDir(prdsDir)
	if err != nil {
		dirEntries = nil
	}

	// Track names we've added to avoid duplicates
	addedNames := make(map[string]bool)

	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		prdPath := filepath.Join(prdsDir, name, "prd.json")

		tabEntry := t.loadTabEntry(name, prdPath)
		t.entries = append(t.entries, tabEntry)
		addedNames[name] = true
	}

	// Also check if there's a "main" PRD directly in .chief/ (legacy location)
	mainPrdPath := filepath.Join(t.baseDir, ".chief", "prd.json")
	if _, err := os.Stat(mainPrdPath); err == nil && !addedNames["main"] {
		tabEntry := t.loadTabEntry("main", mainPrdPath)
		t.entries = append(t.entries, tabEntry)
	}

	// Update active index
	for i, entry := range t.entries {
		if entry.Name == t.currentPRD {
			t.activeIndex = i
			t.entries[i].IsActive = true
		}
	}
}

// loadTabEntry creates a TabEntry for a given name and path.
func (t *TabBar) loadTabEntry(name, prdPath string) TabEntry {
	tabEntry := TabEntry{
		Name:      name,
		Path:      prdPath,
		LoopState: loop.LoopStateReady,
		IsActive:  name == t.currentPRD,
	}

	// Try to load the PRD for progress info
	loadedPRD, err := prd.LoadPRD(prdPath)
	if err == nil {
		tabEntry.Total = len(loadedPRD.UserStories)
		for _, story := range loadedPRD.UserStories {
			if story.Passes {
				tabEntry.Completed++
			}
		}
	}

	// Get loop state from manager if available
	if t.manager != nil {
		if state, iteration, _ := t.manager.GetState(name); state != 0 || iteration != 0 {
			tabEntry.LoopState = state
			tabEntry.Iteration = iteration
		}
	}

	return tabEntry
}

// SetActiveByName sets the active tab by PRD name.
func (t *TabBar) SetActiveByName(name string) {
	t.currentPRD = name
	for i := range t.entries {
		t.entries[i].IsActive = t.entries[i].Name == name
		if t.entries[i].IsActive {
			t.activeIndex = i
		}
	}
}

// GetEntry returns the entry at the given 0-based index.
func (t *TabBar) GetEntry(index int) *TabEntry {
	if index >= 0 && index < len(t.entries) {
		return &t.entries[index]
	}
	return nil
}

// Count returns the number of PRD tabs (excludes "+ New").
func (t *TabBar) Count() int {
	return len(t.entries)
}

// SetSize sets the available width for the tab bar.
func (t *TabBar) SetSize(width int) {
	t.width = width
}

// Render renders the tab bar.
func (t *TabBar) Render() string {
	if len(t.entries) == 0 {
		// No PRDs - show just the "+ New" button
		newTab := TabNewStyle.Render("+ New")
		return newTab
	}

	var tabs []string

	for i, entry := range t.entries {
		tab := t.renderTab(entry, i+1)
		tabs = append(tabs, tab)
	}

	// Add the "+ New" tab
	newTab := TabNewStyle.Render("+ New")
	tabs = append(tabs, newTab)

	// Join tabs with small spacing
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderTab renders a single tab.
func (t *TabBar) renderTab(entry TabEntry, number int) string {
	var content strings.Builder

	// Build tab content: name + indicators
	name := entry.Name
	maxNameLen := 8
	if len(name) > maxNameLen {
		name = name[:maxNameLen-1] + "."
	}

	// State indicator
	var stateIndicator string
	switch entry.LoopState {
	case loop.LoopStateRunning:
		stateIndicator = fmt.Sprintf(" ▶ %d", entry.Iteration)
	case loop.LoopStatePaused:
		stateIndicator = " ⏸"
	case loop.LoopStateComplete:
		stateIndicator = " ✓"
	case loop.LoopStateError:
		stateIndicator = " ✗"
	default:
		// Show progress for ready state
		if entry.Total > 0 {
			stateIndicator = fmt.Sprintf(" [%d/%d]", entry.Completed, entry.Total)
			// Add checkmark if all complete
			if entry.Completed == entry.Total {
				stateIndicator += " ✓"
			}
		}
	}

	// Active indicator
	activeIndicator := ""
	if entry.IsActive {
		activeIndicator = "◉ "
	}

	content.WriteString(activeIndicator)
	content.WriteString(name)
	content.WriteString(stateIndicator)

	tabContent := content.String()

	// Choose style based on state
	var style lipgloss.Style
	if entry.IsActive {
		style = TabActiveStyle
	} else {
		switch entry.LoopState {
		case loop.LoopStateRunning:
			style = TabRunningStyle
		case loop.LoopStateError:
			style = TabErrorStyle
		default:
			style = TabStyle
		}
	}

	// Apply state-specific text colors
	switch entry.LoopState {
	case loop.LoopStateRunning:
		tabContent = lipgloss.NewStyle().Foreground(PrimaryColor).Render(tabContent)
	case loop.LoopStatePaused:
		tabContent = lipgloss.NewStyle().Foreground(WarningColor).Render(tabContent)
	case loop.LoopStateComplete:
		tabContent = lipgloss.NewStyle().Foreground(SuccessColor).Render(tabContent)
	case loop.LoopStateError:
		tabContent = lipgloss.NewStyle().Foreground(ErrorColor).Render(tabContent)
	default:
		if entry.IsActive {
			tabContent = lipgloss.NewStyle().Foreground(TextBrightColor).Render(tabContent)
		} else {
			tabContent = lipgloss.NewStyle().Foreground(TextColor).Render(tabContent)
		}
	}

	return style.Render(tabContent)
}

// RenderCompact renders a compact version of the tab bar for narrow terminals.
func (t *TabBar) RenderCompact() string {
	if len(t.entries) == 0 {
		return TabNewStyle.Render("+")
	}

	var tabs []string

	for i, entry := range t.entries {
		tab := t.renderCompactTab(entry, i+1)
		tabs = append(tabs, tab)
	}

	// Compact new tab
	newTab := TabNewStyle.Render("+")
	tabs = append(tabs, newTab)

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderCompactTab renders a compact version of a tab.
func (t *TabBar) renderCompactTab(entry TabEntry, number int) string {
	var content strings.Builder

	// Shorter name
	name := entry.Name
	maxNameLen := 5
	if len(name) > maxNameLen {
		name = name[:maxNameLen-1] + "."
	}

	// Minimal state indicator
	var stateIndicator string
	switch entry.LoopState {
	case loop.LoopStateRunning:
		stateIndicator = "▶"
	case loop.LoopStatePaused:
		stateIndicator = "⏸"
	case loop.LoopStateComplete:
		stateIndicator = "✓"
	case loop.LoopStateError:
		stateIndicator = "✗"
	}

	// Active indicator
	if entry.IsActive {
		content.WriteString("◉")
	}
	content.WriteString(name)
	if stateIndicator != "" {
		content.WriteString(stateIndicator)
	}

	tabContent := content.String()

	// Choose style
	var style lipgloss.Style
	if entry.IsActive {
		style = TabActiveStyle
	} else {
		switch entry.LoopState {
		case loop.LoopStateRunning:
			style = TabRunningStyle
		case loop.LoopStateError:
			style = TabErrorStyle
		default:
			style = TabStyle
		}
	}

	return style.Render(tabContent)
}
