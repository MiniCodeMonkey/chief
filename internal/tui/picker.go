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

// PRDEntry represents a PRD in the picker list.
type PRDEntry struct {
	Name       string         // Directory name (e.g., "main", "feature-x")
	Path       string         // Full path to prd.json
	PRD        *prd.PRD       // Loaded PRD data
	LoadError  error          // Error if PRD couldn't be loaded
	Completed  int            // Number of completed stories
	Total      int            // Total number of stories
	InProgress bool           // Whether any story is in progress
	LoopState  loop.LoopState // Current loop state from manager
	Iteration  int            // Current iteration if running
}

// PRDPicker manages the PRD picker modal state.
type PRDPicker struct {
	entries       []PRDEntry
	selectedIndex int
	width         int
	height        int
	basePath      string        // Base path where .chief/prds/ is located
	currentPRD    string        // Name of the currently active PRD
	inputMode     bool          // Whether we're in input mode for new PRD name
	inputValue    string        // The current input value for new PRD name
	manager       *loop.Manager // Reference to the loop manager for status updates
}

// NewPRDPicker creates a new PRD picker.
func NewPRDPicker(basePath string, currentPRDName string, manager *loop.Manager) *PRDPicker {
	p := &PRDPicker{
		entries:       make([]PRDEntry, 0),
		selectedIndex: 0,
		basePath:      basePath,
		currentPRD:    currentPRDName,
		inputMode:     false,
		inputValue:    "",
		manager:       manager,
	}
	p.Refresh()
	return p
}

// SetManager sets the loop manager reference.
func (p *PRDPicker) SetManager(manager *loop.Manager) {
	p.manager = manager
}

// Refresh reloads the list of PRDs from the .chief/prds/ directory.
func (p *PRDPicker) Refresh() {
	p.entries = make([]PRDEntry, 0)

	prdsDir := filepath.Join(p.basePath, ".chief", "prds")

	// Read the prds directory
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		// Directory might not exist - that's okay, but still check for current PRD
		entries = nil
	}

	// Track names we've added to avoid duplicates
	addedNames := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		prdPath := filepath.Join(prdsDir, name, "prd.json")

		prdEntry := p.loadPRDEntry(name, prdPath)
		p.entries = append(p.entries, prdEntry)
		addedNames[name] = true
	}

	// Also check if there's a "main" PRD directly in .chief/ (legacy location)
	mainPrdPath := filepath.Join(p.basePath, ".chief", "prd.json")
	if _, err := os.Stat(mainPrdPath); err == nil && !addedNames["main"] {
		prdEntry := p.loadPRDEntry("main", mainPrdPath)
		p.entries = append(p.entries, prdEntry)
		addedNames["main"] = true
	}

	// Ensure selected index is valid
	if p.selectedIndex >= len(p.entries) {
		p.selectedIndex = len(p.entries) - 1
		if p.selectedIndex < 0 {
			p.selectedIndex = 0
		}
	}
}

// loadPRDEntry creates a PRDEntry for a given name and path.
func (p *PRDPicker) loadPRDEntry(name, prdPath string) PRDEntry {
	prdEntry := PRDEntry{
		Name:      name,
		Path:      prdPath,
		LoopState: loop.LoopStateReady,
	}

	// Try to load the PRD
	loadedPRD, err := prd.LoadPRD(prdPath)
	if err != nil {
		prdEntry.LoadError = err
	} else {
		prdEntry.PRD = loadedPRD
		prdEntry.Total = len(loadedPRD.UserStories)
		for _, story := range loadedPRD.UserStories {
			if story.Passes {
				prdEntry.Completed++
			}
			if story.InProgress {
				prdEntry.InProgress = true
			}
		}
	}

	// Get loop state from manager if available
	if p.manager != nil {
		if state, iteration, _ := p.manager.GetState(name); state != 0 || iteration != 0 {
			prdEntry.LoopState = state
			prdEntry.Iteration = iteration
		}
	}

	return prdEntry
}

// SetSize sets the modal dimensions.
func (p *PRDPicker) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// MoveUp moves the selection up.
func (p *PRDPicker) MoveUp() {
	if p.inputMode {
		return
	}
	if p.selectedIndex > 0 {
		p.selectedIndex--
	}
}

// MoveDown moves the selection down.
func (p *PRDPicker) MoveDown() {
	if p.inputMode {
		return
	}
	if p.selectedIndex < len(p.entries)-1 {
		p.selectedIndex++
	}
}

// GetSelectedEntry returns the currently selected PRD entry.
func (p *PRDPicker) GetSelectedEntry() *PRDEntry {
	if p.selectedIndex >= 0 && p.selectedIndex < len(p.entries) {
		return &p.entries[p.selectedIndex]
	}
	return nil
}

// IsEmpty returns true if there are no PRDs.
func (p *PRDPicker) IsEmpty() bool {
	return len(p.entries) == 0
}

// IsInputMode returns true if the picker is in input mode for new PRD name.
func (p *PRDPicker) IsInputMode() bool {
	return p.inputMode
}

// StartInputMode enters input mode for creating a new PRD.
func (p *PRDPicker) StartInputMode() {
	p.inputMode = true
	p.inputValue = ""
}

// CancelInputMode exits input mode without creating a PRD.
func (p *PRDPicker) CancelInputMode() {
	p.inputMode = false
	p.inputValue = ""
}

// GetInputValue returns the current input value.
func (p *PRDPicker) GetInputValue() string {
	return p.inputValue
}

// AddInputChar adds a character to the input.
func (p *PRDPicker) AddInputChar(ch rune) {
	// Only allow valid directory name characters
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
		p.inputValue += string(ch)
	}
}

// DeleteInputChar removes the last character from the input.
func (p *PRDPicker) DeleteInputChar() {
	if len(p.inputValue) > 0 {
		p.inputValue = p.inputValue[:len(p.inputValue)-1]
	}
}

// SetCurrentPRD sets the current PRD name for highlighting.
func (p *PRDPicker) SetCurrentPRD(name string) {
	p.currentPRD = name
}

// Render renders the PRD picker modal.
func (p *PRDPicker) Render() string {
	// Modal dimensions
	modalWidth := min(60, p.width-10)
	modalHeight := min(20, p.height-6)

	if modalWidth < 30 {
		modalWidth = 30
	}
	if modalHeight < 10 {
		modalHeight = 10
	}

	// Build modal content
	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Padding(0, 1)
	content.WriteString(titleStyle.Render("Select PRD"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	if p.inputMode {
		// Input mode for new PRD name
		content.WriteString(p.renderInputMode(modalWidth - 4))
	} else if p.IsEmpty() {
		// Empty state
		emptyStyle := lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(1, 2)
		content.WriteString(emptyStyle.Render("No PRDs found in .chief/prds/"))
		content.WriteString("\n")
		content.WriteString(emptyStyle.Render("Press 'n' to create a new PRD"))
	} else {
		// PRD list
		listHeight := modalHeight - 7 // Account for title, borders, footer
		startIdx := 0
		if p.selectedIndex >= listHeight {
			startIdx = p.selectedIndex - listHeight + 1
		}

		for i := startIdx; i < len(p.entries) && i < startIdx+listHeight; i++ {
			entry := p.entries[i]
			line := p.renderEntry(entry, i == p.selectedIndex, modalWidth-6)
			content.WriteString(line)
			content.WriteString("\n")
		}

		// Pad remaining space
		renderedLines := min(len(p.entries)-startIdx, listHeight)
		for i := renderedLines; i < listHeight; i++ {
			content.WriteString("\n")
		}
	}

	// Footer with shortcuts
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	var shortcuts string
	if p.inputMode {
		shortcuts = "Enter: create  │  Esc: cancel"
	} else {
		// Build context-sensitive shortcuts based on selected entry's state
		shortcuts = p.buildFooterShortcuts()
	}
	footerStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)
	content.WriteString(footerStyle.Render(shortcuts))

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())

	// Center the modal on screen
	return p.centerModal(modal)
}

// renderEntry renders a single PRD entry line.
func (p *PRDPicker) renderEntry(entry PRDEntry, selected bool, width int) string {
	var line strings.Builder

	// Current indicator
	if entry.Name == p.currentPRD {
		line.WriteString(lipgloss.NewStyle().Foreground(SuccessColor).Render("● "))
	} else {
		line.WriteString("  ")
	}

	// Name
	nameStyle := lipgloss.NewStyle().Foreground(TextColor)
	if selected {
		nameStyle = nameStyle.Bold(true).Foreground(TextBrightColor)
	}
	name := entry.Name
	maxNameLen := 12
	if len(name) > maxNameLen {
		name = name[:maxNameLen-2] + ".."
	}
	line.WriteString(nameStyle.Render(fmt.Sprintf("%-12s", name)))
	line.WriteString(" ")

	if entry.LoadError != nil {
		// Show error indicator
		errorStyle := lipgloss.NewStyle().Foreground(ErrorColor)
		line.WriteString(errorStyle.Render("[error]"))
	} else {
		// Progress bar
		progressWidth := 8
		percentage := float64(0)
		if entry.Total > 0 {
			percentage = float64(entry.Completed) / float64(entry.Total) * 100
		}
		filledWidth := int(float64(progressWidth) * percentage / 100)
		emptyWidth := progressWidth - filledWidth

		progressBar := progressBarFillStyle.Render(strings.Repeat("█", filledWidth)) +
			progressBarEmptyStyle.Render(strings.Repeat("░", emptyWidth))
		line.WriteString(progressBar)
		line.WriteString(" ")

		// Count
		countStyle := lipgloss.NewStyle().Foreground(MutedColor)
		line.WriteString(countStyle.Render(fmt.Sprintf("%d/%d", entry.Completed, entry.Total)))

		// Loop state indicator
		line.WriteString(" ")
		line.WriteString(p.renderLoopStateIndicator(entry))
	}

	result := line.String()

	// Apply selection highlight
	if selected {
		result = selectedStyle.Width(width).Render(result)
	}

	return result
}

// renderLoopStateIndicator renders a visual indicator for the loop state.
func (p *PRDPicker) renderLoopStateIndicator(entry PRDEntry) string {
	switch entry.LoopState {
	case loop.LoopStateRunning:
		// Show spinning indicator with iteration count
		runningStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
		return runningStyle.Render(fmt.Sprintf("▶ %d", entry.Iteration))
	case loop.LoopStatePaused:
		pausedStyle := lipgloss.NewStyle().Foreground(WarningColor)
		return pausedStyle.Render("⏸")
	case loop.LoopStateComplete:
		completeStyle := lipgloss.NewStyle().Foreground(SuccessColor)
		return completeStyle.Render("✓")
	case loop.LoopStateError:
		errorStyle := lipgloss.NewStyle().Foreground(ErrorColor)
		return errorStyle.Render("✗")
	case loop.LoopStateStopped:
		stoppedStyle := lipgloss.NewStyle().Foreground(MutedColor)
		return stoppedStyle.Render("■")
	default:
		// Ready state - show story status
		if entry.InProgress {
			inProgressStyle := lipgloss.NewStyle().Foreground(PrimaryColor)
			return inProgressStyle.Render("●")
		} else if entry.Completed == entry.Total && entry.Total > 0 {
			completeStyle := lipgloss.NewStyle().Foreground(SuccessColor)
			return completeStyle.Render("✓")
		}
		return ""
	}
}

// renderInputMode renders the input mode for new PRD name.
func (p *PRDPicker) renderInputMode(width int) string {
	var content strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)
	content.WriteString(labelStyle.Render("New PRD name:"))
	content.WriteString("\n\n")

	// Input field
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0, 1).
		Width(width - 4)

	inputValue := p.inputValue
	if inputValue == "" {
		inputValue = lipgloss.NewStyle().Foreground(MutedColor).Render("(type a name...)")
	}
	// Add cursor
	cursorStyle := lipgloss.NewStyle().Foreground(PrimaryColor).Blink(true)
	inputValue += cursorStyle.Render("▌")

	content.WriteString(inputStyle.Render(inputValue))
	content.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(MutedColor)
	content.WriteString(hintStyle.Render("Only letters, numbers, - and _ allowed"))

	return content.String()
}

// buildFooterShortcuts builds context-sensitive shortcuts based on selected entry's state.
func (p *PRDPicker) buildFooterShortcuts() string {
	entry := p.GetSelectedEntry()
	if entry == nil {
		return "↑/k ↓/j: nav  │  n: new  │  Esc/l: close"
	}

	// Base shortcuts
	base := "Enter: select  │  n: new  │  e: edit  │  Esc/l: close"

	// Add state-specific controls
	switch entry.LoopState {
	case loop.LoopStateReady, loop.LoopStatePaused, loop.LoopStateStopped, loop.LoopStateError:
		return "s: start  │  " + base
	case loop.LoopStateRunning:
		return "p: pause  │  x: stop  │  " + base
	case loop.LoopStateComplete:
		return base
	default:
		return "s: start  │  " + base
	}
}

// centerModal centers the modal on the screen.
func (p *PRDPicker) centerModal(modal string) string {
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := 0
	for _, line := range lines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	// Calculate padding
	topPadding := (p.height - modalHeight) / 2
	leftPadding := (p.width - modalWidth) / 2

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
