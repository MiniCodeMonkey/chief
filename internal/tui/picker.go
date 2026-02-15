package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/minicodemonkey/chief/internal/git"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
)

// PRDEntry represents a PRD in the picker list.
type PRDEntry struct {
	Name        string         // Directory name (e.g., "main", "feature-x")
	Path        string         // Full path to prd.json
	PRD         *prd.PRD       // Loaded PRD data
	LoadError   error          // Error if PRD couldn't be loaded
	Completed   int            // Number of completed stories
	Total       int            // Total number of stories
	InProgress  bool           // Whether any story is in progress
	LoopState   loop.LoopState // Current loop state from manager
	Iteration   int            // Current iteration if running
	Branch      string         // Git branch for this PRD (empty = no branch)
	WorktreeDir string         // Worktree directory (empty = current directory)
	Orphaned    bool           // True if worktree exists on disk but no running PRD tracks it
}

// MergeResult holds the result of a merge operation for display.
type MergeResult struct {
	Success   bool     // Whether the merge succeeded
	Message   string   // Success message or error summary
	Conflicts []string // Conflicting file list (empty on success)
	Branch    string   // The branch that was merged
}

// CleanOption represents the user's choice in the clean confirmation dialog.
type CleanOption int

const (
	CleanOptionRemoveAll    CleanOption = iota // Remove worktree + delete branch
	CleanOptionWorktreeOnly                    // Remove worktree only (keep branch)
	CleanOptionCancel                          // Cancel
)

// CleanConfirmation holds the state of the clean confirmation dialog.
type CleanConfirmation struct {
	EntryName    string // Name of the PRD being cleaned
	Branch       string // Branch name to display
	WorktreeDir  string // Worktree path to display
	SelectedIdx  int    // Selected option index (0-2)
}

// CleanResult holds the result of a clean operation for display.
type CleanResult struct {
	Success bool   // Whether the clean succeeded
	Message string // Success or error message
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
	manager            *loop.Manager      // Reference to the loop manager for status updates
	mergeResult        *MergeResult       // Result of the last merge operation (nil = none)
	cleanConfirmation  *CleanConfirmation // Active clean confirmation dialog (nil = none)
	cleanResult        *CleanResult       // Result of the last clean operation (nil = none)
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

	// Detect orphaned worktrees - worktrees on disk not tracked by any manager instance
	diskWorktrees := git.DetectOrphanedWorktrees(p.basePath)
	if len(diskWorktrees) > 0 {
		// Build set of tracked worktree dirs from manager
		trackedDirs := make(map[string]bool)
		if p.manager != nil {
			for _, inst := range p.manager.GetAllInstances() {
				if inst.WorktreeDir != "" {
					trackedDirs[inst.WorktreeDir] = true
				}
			}
		}

		for prdName, absPath := range diskWorktrees {
			if trackedDirs[absPath] {
				continue // This worktree is tracked by a running/registered PRD
			}
			// Mark the matching entry as orphaned, or note it on existing entries
			found := false
			for i := range p.entries {
				if p.entries[i].Name == prdName {
					p.entries[i].Orphaned = true
					// Also set WorktreeDir if not already set (so CanClean works)
					if p.entries[i].WorktreeDir == "" {
						p.entries[i].WorktreeDir = absPath
					}
					found = true
					break
				}
			}
			// If no matching PRD entry exists, the worktree is truly orphaned
			// (no prd.json at all). Still show it so the user knows it exists.
			if !found {
				p.entries = append(p.entries, PRDEntry{
					Name:        prdName,
					Path:        filepath.Join(p.basePath, ".chief", "prds", prdName, "prd.json"),
					LoopState:   loop.LoopStateReady,
					WorktreeDir: absPath,
					Orphaned:    true,
					LoadError:   fmt.Errorf("orphaned worktree (no prd.json)"),
				})
			}
		}
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

	// Get loop state and worktree info from manager if available
	if p.manager != nil {
		if state, iteration, _ := p.manager.GetState(name); state != 0 || iteration != 0 {
			prdEntry.LoopState = state
			prdEntry.Iteration = iteration
		}
		if instance := p.manager.GetInstance(name); instance != nil {
			prdEntry.Branch = instance.Branch
			prdEntry.WorktreeDir = instance.WorktreeDir
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

// CanMerge returns true if the selected entry is a completed PRD with a branch set.
func (p *PRDPicker) CanMerge() bool {
	entry := p.GetSelectedEntry()
	if entry == nil || entry.Branch == "" {
		return false
	}
	// Allow merge for completed loop state or all stories passed
	return entry.LoopState == loop.LoopStateComplete || (entry.Completed == entry.Total && entry.Total > 0)
}

// SetMergeResult sets the merge result for display.
func (p *PRDPicker) SetMergeResult(result *MergeResult) {
	p.mergeResult = result
}

// ClearMergeResult clears any displayed merge result.
func (p *PRDPicker) ClearMergeResult() {
	p.mergeResult = nil
}

// HasMergeResult returns true if there is a merge result to display.
func (p *PRDPicker) HasMergeResult() bool {
	return p.mergeResult != nil
}

// CanClean returns true if the selected entry is a non-running PRD with a worktree.
func (p *PRDPicker) CanClean() bool {
	entry := p.GetSelectedEntry()
	if entry == nil || entry.WorktreeDir == "" {
		return false
	}
	// Disabled for running PRDs - user must stop first
	return entry.LoopState != loop.LoopStateRunning
}

// StartCleanConfirmation opens the clean confirmation dialog for the selected entry.
func (p *PRDPicker) StartCleanConfirmation() {
	entry := p.GetSelectedEntry()
	if entry == nil {
		return
	}
	p.cleanConfirmation = &CleanConfirmation{
		EntryName:   entry.Name,
		Branch:      entry.Branch,
		WorktreeDir: p.worktreeDisplayPath(*entry),
		SelectedIdx: 0,
	}
}

// CancelCleanConfirmation closes the clean confirmation dialog.
func (p *PRDPicker) CancelCleanConfirmation() {
	p.cleanConfirmation = nil
}

// HasCleanConfirmation returns true if the clean confirmation dialog is active.
func (p *PRDPicker) HasCleanConfirmation() bool {
	return p.cleanConfirmation != nil
}

// GetCleanConfirmation returns the current clean confirmation state.
func (p *PRDPicker) GetCleanConfirmation() *CleanConfirmation {
	return p.cleanConfirmation
}

// CleanConfirmMoveUp moves the selection up in the clean confirmation dialog.
func (p *PRDPicker) CleanConfirmMoveUp() {
	if p.cleanConfirmation != nil && p.cleanConfirmation.SelectedIdx > 0 {
		p.cleanConfirmation.SelectedIdx--
	}
}

// CleanConfirmMoveDown moves the selection down in the clean confirmation dialog.
func (p *PRDPicker) CleanConfirmMoveDown() {
	if p.cleanConfirmation != nil && p.cleanConfirmation.SelectedIdx < 2 {
		p.cleanConfirmation.SelectedIdx++
	}
}

// GetCleanOption returns the selected clean option.
func (p *PRDPicker) GetCleanOption() CleanOption {
	if p.cleanConfirmation == nil {
		return CleanOptionCancel
	}
	switch p.cleanConfirmation.SelectedIdx {
	case 0:
		return CleanOptionRemoveAll
	case 1:
		return CleanOptionWorktreeOnly
	case 2:
		return CleanOptionCancel
	default:
		return CleanOptionCancel
	}
}

// SetCleanResult sets the clean result for display.
func (p *PRDPicker) SetCleanResult(result *CleanResult) {
	p.cleanResult = result
}

// ClearCleanResult clears any displayed clean result.
func (p *PRDPicker) ClearCleanResult() {
	p.cleanResult = nil
}

// HasCleanResult returns true if there is a clean result to display.
func (p *PRDPicker) HasCleanResult() bool {
	return p.cleanResult != nil
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

	// If there's a clean result, render that instead
	if p.cleanResult != nil {
		return p.renderCleanResult(modalWidth, modalHeight)
	}

	// If there's a clean confirmation, render that instead
	if p.cleanConfirmation != nil {
		return p.renderCleanConfirmation(modalWidth, modalHeight)
	}

	// If there's a merge result, render that instead
	if p.mergeResult != nil {
		return p.renderMergeResult(modalWidth, modalHeight)
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

	if entry.Orphaned && entry.LoadError != nil {
		// Orphaned worktree with no PRD - show orphaned indicator
		orphanedStyle := lipgloss.NewStyle().Foreground(WarningColor)
		line.WriteString(orphanedStyle.Render("[orphaned worktree]"))
		// Show worktree path if space allows
		remaining := width - 32 - 18 // account for indicator + name + orphaned label
		if remaining > 10 && entry.WorktreeDir != "" {
			pathStyle := lipgloss.NewStyle().Foreground(MutedColor)
			displayPath := p.worktreeDisplayPath(entry)
			line.WriteString(pathStyle.Render("  " + displayPath))
		}
	} else if entry.LoadError != nil {
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

		// Orphaned worktree indicator (for entries with PRD but orphaned worktree)
		if entry.Orphaned {
			orphanedStyle := lipgloss.NewStyle().Foreground(WarningColor)
			line.WriteString(" ")
			line.WriteString(orphanedStyle.Render("[orphaned]"))
		}

		// Branch and worktree path (only if branch is set)
		if entry.Branch != "" {
			branchPathStyle := lipgloss.NewStyle().Foreground(MutedColor)
			// Calculate remaining space for branch and path info
			// Base content uses: 2 (indicator) + 12 (name) + 1 (space) + 8 (progress) + 1 (space) + ~3 (count) + 1 (space) + ~2 (state) = ~30
			remaining := width - 32
			if remaining > 10 {
				branchStr := entry.Branch
				pathStr := p.worktreeDisplayPath(entry)
				// Truncate to fit within remaining space: "  branch  path"
				infoStr := p.formatBranchPath(branchStr, pathStr, remaining)
				line.WriteString(branchPathStyle.Render(infoStr))
			}
		}
	}

	result := line.String()

	// Apply selection highlight
	if selected {
		result = selectedStyle.Width(width).Render(result)
	}

	return result
}

// worktreeDisplayPath returns a display-friendly worktree path.
func (p *PRDPicker) worktreeDisplayPath(entry PRDEntry) string {
	if entry.WorktreeDir == "" {
		return "(current directory)"
	}
	// Show relative path from base dir
	rel, err := filepath.Rel(p.basePath, entry.WorktreeDir)
	if err != nil {
		return entry.WorktreeDir
	}
	return rel + "/"
}

// formatBranchPath formats branch and path info to fit within maxWidth.
// maxWidth is in display characters (runes).
func (p *PRDPicker) formatBranchPath(branch, path string, maxWidth int) string {
	// Format: "  <branch>  <path>"
	prefix := "  "
	separator := "  "
	prefixLen := 2
	sepLen := 2

	branchRunes := []rune(branch)
	pathRunes := []rune(path)

	fullLen := prefixLen + len(branchRunes) + sepLen + len(pathRunes)
	if fullLen <= maxWidth {
		return prefix + branch + separator + path
	}

	// Try truncating path first
	availForPath := maxWidth - prefixLen - len(branchRunes) - sepLen
	if availForPath > 5 {
		if len(pathRunes) > availForPath {
			// "…" takes 1 display character
			keep := availForPath - 1
			pathRunes = append([]rune("…"), pathRunes[len(pathRunes)-keep:]...)
		}
		return prefix + branch + separator + string(pathRunes)
	}

	// Not enough room for path, just show branch (truncated if needed)
	availForBranch := maxWidth - prefixLen
	if availForBranch > 3 && len(branchRunes) > availForBranch {
		branchRunes = append(branchRunes[:availForBranch-1], '…')
	}
	return prefix + string(branchRunes)
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

	// Add merge shortcut for completed PRDs with a branch
	mergeHint := ""
	if p.CanMerge() {
		mergeHint = "m: merge  │  "
	}

	// Add clean shortcut for non-running PRDs with a worktree
	cleanHint := ""
	if p.CanClean() {
		cleanHint = "c: clean  │  "
	}

	// Add state-specific controls
	switch entry.LoopState {
	case loop.LoopStateReady, loop.LoopStatePaused, loop.LoopStateStopped, loop.LoopStateError:
		return "s: start  │  " + mergeHint + cleanHint + base
	case loop.LoopStateRunning:
		return "p: pause  │  x: stop  │  " + base
	case loop.LoopStateComplete:
		return mergeHint + cleanHint + base
	default:
		return "s: start  │  " + mergeHint + cleanHint + base
	}
}

// renderMergeResult renders the merge result dialog.
func (p *PRDPicker) renderMergeResult(modalWidth, modalHeight int) string {
	var content strings.Builder

	if p.mergeResult.Success {
		// Success display
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(SuccessColor).
			Padding(0, 1)
		content.WriteString(titleStyle.Render("Merge Successful"))
		content.WriteString("\n")
		content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
		content.WriteString("\n\n")

		msgStyle := lipgloss.NewStyle().
			Foreground(TextColor).
			Padding(0, 1)
		content.WriteString(msgStyle.Render(p.mergeResult.Message))
		content.WriteString("\n")
	} else {
		// Error/conflict display
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ErrorColor).
			Padding(0, 1)
		content.WriteString(titleStyle.Render("Merge Conflict"))
		content.WriteString("\n")
		content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
		content.WriteString("\n\n")

		msgStyle := lipgloss.NewStyle().
			Foreground(TextColor).
			Padding(0, 1)
		content.WriteString(msgStyle.Render(p.mergeResult.Message))
		content.WriteString("\n\n")

		if len(p.mergeResult.Conflicts) > 0 {
			conflictHeaderStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(WarningColor).
				Padding(0, 1)
			content.WriteString(conflictHeaderStyle.Render("Conflicting files:"))
			content.WriteString("\n")

			conflictStyle := lipgloss.NewStyle().
				Foreground(MutedColor).
				Padding(0, 2)
			maxFiles := modalHeight - 12
			if maxFiles < 3 {
				maxFiles = 3
			}
			for i, f := range p.mergeResult.Conflicts {
				if i >= maxFiles {
					content.WriteString(conflictStyle.Render(fmt.Sprintf("  ... and %d more", len(p.mergeResult.Conflicts)-maxFiles)))
					content.WriteString("\n")
					break
				}
				content.WriteString(conflictStyle.Render("  " + f))
				content.WriteString("\n")
			}

			content.WriteString("\n")
			hintStyle := lipgloss.NewStyle().
				Foreground(MutedColor).
				Padding(0, 1)
			content.WriteString(hintStyle.Render("To resolve manually:"))
			content.WriteString("\n")
			content.WriteString(hintStyle.Render(fmt.Sprintf("  cd <project-root>")))
			content.WriteString("\n")
			content.WriteString(hintStyle.Render(fmt.Sprintf("  git merge %s", p.mergeResult.Branch)))
			content.WriteString("\n")
			content.WriteString(hintStyle.Render("  # resolve conflicts, then git commit"))
			content.WriteString("\n")
		}
	}

	// Footer
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)
	content.WriteString(footerStyle.Render("Press any key to continue"))

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())
	return p.centerModal(modal)
}

// renderCleanConfirmation renders the clean confirmation dialog.
func (p *PRDPicker) renderCleanConfirmation(modalWidth, modalHeight int) string {
	var content strings.Builder
	cc := p.cleanConfirmation

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(WarningColor).
		Padding(0, 1)
	content.WriteString(titleStyle.Render("Clean Worktree"))
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// Show what will be removed
	infoStyle := lipgloss.NewStyle().
		Foreground(TextColor).
		Padding(0, 1)
	content.WriteString(infoStyle.Render(fmt.Sprintf("PRD: %s", cc.EntryName)))
	content.WriteString("\n")
	content.WriteString(infoStyle.Render(fmt.Sprintf("Worktree: %s", cc.WorktreeDir)))
	content.WriteString("\n")
	if cc.Branch != "" {
		content.WriteString(infoStyle.Render(fmt.Sprintf("Branch: %s", cc.Branch)))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Options
	options := []struct {
		label string
		hint  string
	}{
		{"Remove worktree + delete branch (Recommended)", "Removes worktree directory and deletes the local branch"},
		{"Remove worktree only (keep branch)", "Removes worktree directory but keeps the branch for later use"},
		{"Cancel", ""},
	}

	for i, opt := range options {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(TextColor)
		if i == cc.SelectedIdx {
			prefix = "▸ "
			style = style.Bold(true).Foreground(TextBrightColor)
		}
		content.WriteString(style.Render(prefix + opt.label))
		content.WriteString("\n")
		if opt.hint != "" && i == cc.SelectedIdx {
			hintStyle := lipgloss.NewStyle().Foreground(MutedColor).Padding(0, 2)
			content.WriteString(hintStyle.Render("  " + opt.hint))
			content.WriteString("\n")
		}
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)
	content.WriteString(footerStyle.Render("↑/k ↓/j: nav  │  Enter: confirm  │  Esc: cancel"))

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(WarningColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())
	return p.centerModal(modal)
}

// renderCleanResult renders the clean result dialog.
func (p *PRDPicker) renderCleanResult(modalWidth, modalHeight int) string {
	var content strings.Builder

	if p.cleanResult.Success {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(SuccessColor).
			Padding(0, 1)
		content.WriteString(titleStyle.Render("Clean Successful"))
	} else {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ErrorColor).
			Padding(0, 1)
		content.WriteString(titleStyle.Render("Clean Failed"))
	}
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	msgStyle := lipgloss.NewStyle().
		Foreground(TextColor).
		Padding(0, 1)
	content.WriteString(msgStyle.Render(p.cleanResult.Message))
	content.WriteString("\n")

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)
	content.WriteString(footerStyle.Render("Press any key to continue"))

	// Modal box style
	borderColor := SuccessColor
	if !p.cleanResult.Success {
		borderColor = ErrorColor
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())
	return p.centerModal(modal)
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
