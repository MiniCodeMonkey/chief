package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/git"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
)

// PRDUpdateMsg is sent when the PRD file changes.
type PRDUpdateMsg struct {
	PRD   *prd.PRD
	Error error
}

// AppState represents the current state of the application.
type AppState int

const (
	StateReady AppState = iota
	StateRunning
	StatePaused
	StateStopped
	StateComplete
	StateError
)

func (s AppState) String() string {
	switch s {
	case StateReady:
		return "Ready"
	case StateRunning:
		return "Running"
	case StatePaused:
		return "Paused"
	case StateStopped:
		return "Stopped"
	case StateComplete:
		return "Complete"
	case StateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// LoopEventMsg wraps a loop event for the Bubble Tea model.
type LoopEventMsg struct {
	PRDName string
	Event   loop.Event
}

// LoopFinishedMsg is sent when the loop finishes (complete, paused, stopped, or error).
type LoopFinishedMsg struct {
	PRDName string
	Err     error
}

// PRDCompletedMsg is sent when any PRD completes all stories.
type PRDCompletedMsg struct {
	PRDName string
}

// LaunchInitMsg signals the TUI should exit to launch the init flow.
type LaunchInitMsg struct {
	Name string
}

// LaunchEditMsg signals the TUI should exit to launch the edit flow.
type LaunchEditMsg struct {
	Name string
}

// ViewMode represents which view is currently active.
type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewLog
	ViewPicker
	ViewHelp
	ViewBranchWarning
)

// App is the main Bubble Tea model for the Chief TUI.
type App struct {
	prd           *prd.PRD
	prdPath       string
	prdName       string
	state         AppState
	iteration     int
	startTime     time.Time
	selectedIndex int
	width         int
	height        int
	err           error

	// Loop manager for parallel PRD execution
	manager *loop.Manager
	maxIter int

	// Activity tracking
	lastActivity string

	// File watching
	watcher *prd.Watcher

	// View mode
	viewMode  ViewMode
	logViewer *LogViewer

	// PRD tab bar (always visible)
	tabBar *TabBar

	// PRD picker (for creating new PRDs)
	picker  *PRDPicker
	baseDir string // Base directory for .chief/prds/

	// Project config
	config *config.Config

	// Help overlay
	helpOverlay      *HelpOverlay
	previousViewMode ViewMode // View to return to when closing help

	// Branch warning dialog
	branchWarning      *BranchWarning
	pendingStartPRD    string // PRD name waiting to start after branch decision
	pendingWorktreePath string // Absolute worktree path for pending PRD

	// Completion notification callback
	onCompletion func(prdName string)

	// Verbose mode - show raw Claude output
	verbose bool

	// Post-exit action - what to do after TUI exits
	PostExitAction PostExitAction
	PostExitPRD    string // PRD name for post-exit action
}

// PostExitAction represents an action to take after the TUI exits.
type PostExitAction int

const (
	PostExitNone PostExitAction = iota
	PostExitInit
	PostExitEdit
)

// NewApp creates a new App with the given PRD.
func NewApp(prdPath string) (*App, error) {
	return NewAppWithOptions(prdPath, 10) // default max iterations
}

// NewAppWithOptions creates a new App with the given PRD and options.
// If maxIter <= 0, it will be calculated dynamically based on remaining stories.
func NewAppWithOptions(prdPath string, maxIter int) (*App, error) {
	p, err := prd.LoadPRD(prdPath)
	if err != nil {
		return nil, err
	}

	// Calculate dynamic default if maxIter <= 0
	if maxIter <= 0 {
		remaining := 0
		for _, story := range p.UserStories {
			if !story.Passes {
				remaining++
			}
		}
		maxIter = remaining + 5
		if maxIter < 5 {
			maxIter = 5
		}
	}

	// Extract PRD name from path (directory name or filename without extension)
	prdName := filepath.Base(filepath.Dir(prdPath))
	if prdName == "." || prdName == "/" {
		prdName = filepath.Base(prdPath)
	}

	// Create file watcher
	watcher, err := prd.NewWatcher(prdPath)
	if err != nil {
		return nil, err
	}

	// Determine base directory for PRD picker
	// If path contains .chief/prds/, go up to the project root (4 levels up from prd.json)
	// .chief/prds/<name>/prd.json -> .chief/prds/<name> -> .chief/prds -> .chief -> project root
	baseDir := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(prdPath))))
	if !strings.Contains(prdPath, ".chief/prds/") {
		// Fallback to current working directory
		baseDir, _ = os.Getwd()
	}

	// Load project config
	cfg, err := config.Load(baseDir)
	if err != nil {
		cfg = config.Default()
	}

	// Create loop manager for parallel PRD execution
	manager := loop.NewManager(maxIter)
	manager.SetConfig(cfg)

	// Register the initial PRD with the manager
	manager.Register(prdName, prdPath)

	// Create tab bar for always-visible PRD tabs
	tabBar := NewTabBar(baseDir, prdName, manager)

	// Create picker with manager reference (for creating new PRDs)
	picker := NewPRDPicker(baseDir, prdName, manager)

	return &App{
		prd:           p,
		prdPath:       prdPath,
		prdName:       prdName,
		state:         StateReady,
		iteration:     0,
		selectedIndex: 0,
		maxIter:       maxIter,
		manager:       manager,
		watcher:       watcher,
		viewMode:      ViewDashboard,
		logViewer:     NewLogViewer(),
		tabBar:        tabBar,
		picker:        picker,
		baseDir:       baseDir,
		config:        cfg,
		helpOverlay:   NewHelpOverlay(),
		branchWarning: NewBranchWarning(),
	}, nil
}

// SetCompletionCallback sets a callback that is called when any PRD completes.
func (a *App) SetCompletionCallback(fn func(prdName string)) {
	a.onCompletion = fn
	if a.manager != nil {
		a.manager.SetCompletionCallback(fn)
	}
}

// SetVerbose enables or disables verbose mode (raw Claude output in log).
func (a *App) SetVerbose(v bool) {
	a.verbose = v
}

// DisableRetry disables automatic retry on Claude crashes.
func (a *App) DisableRetry() {
	if a.manager != nil {
		a.manager.DisableRetry()
	}
}

// Init initializes the App.
func (a App) Init() tea.Cmd {
	// Start the file watcher
	if a.watcher != nil {
		if err := a.watcher.Start(); err != nil {
			// Log error but don't fail - watcher is not critical
			a.lastActivity = "Warning: file watcher failed to start"
		}
	}

	return tea.Batch(
		tea.EnterAltScreen,
		a.listenForPRDChanges(),
		a.listenForManagerEvents(),
	)
}

// listenForManagerEvents listens for events from all managed loops.
func (a *App) listenForManagerEvents() tea.Cmd {
	if a.manager == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-a.manager.Events()
		if !ok {
			return nil
		}
		return LoopEventMsg{PRDName: event.PRDName, Event: event.Event}
	}
}

// Update handles messages and updates the model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Update log viewer size
		a.logViewer.SetSize(a.width, a.height-headerHeight-footerHeight-2)
		return a, nil

	case LoopEventMsg:
		return a.handleLoopEvent(msg.PRDName, msg.Event)

	case LoopFinishedMsg:
		return a.handleLoopFinished(msg.PRDName, msg.Err)

	case PRDCompletedMsg:
		// A PRD completed - trigger completion notification
		if a.onCompletion != nil {
			a.onCompletion(msg.PRDName)
		}
		// Refresh tab bar and picker to show updated status
		if a.tabBar != nil {
			a.tabBar.Refresh()
		}
		a.picker.Refresh()
		return a, nil

	case PRDUpdateMsg:
		return a.handlePRDUpdate(msg)

	case LaunchInitMsg:
		a.PostExitAction = PostExitInit
		a.PostExitPRD = msg.Name
		return a, tea.Quit

	case LaunchEditMsg:
		a.PostExitAction = PostExitEdit
		a.PostExitPRD = msg.Name
		return a, tea.Quit

	case tea.KeyMsg:
		// Handle help overlay first (can be opened/closed from any view)
		if msg.String() == "?" {
			if a.viewMode == ViewHelp {
				// Close help, return to previous view
				a.viewMode = a.previousViewMode
			} else {
				// Open help, remember current view
				a.previousViewMode = a.viewMode
				a.viewMode = ViewHelp
				a.helpOverlay.SetSize(a.width, a.height)
				a.helpOverlay.SetViewMode(a.previousViewMode)
			}
			return a, nil
		}

		// Handle help view (only Esc closes it besides ?)
		if a.viewMode == ViewHelp {
			if msg.String() == "esc" {
				a.viewMode = a.previousViewMode
			}
			// Ignore other keys in help view
			return a, nil
		}

		// Handle picker view separately (it has its own input mode)
		if a.viewMode == ViewPicker {
			return a.handlePickerKeys(msg)
		}

		// Handle branch warning view
		if a.viewMode == ViewBranchWarning {
			return a.handleBranchWarningKeys(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			a.stopAllLoops()
			a.stopWatcher()
			return a, tea.Quit

		// View switching
		case "t":
			if a.viewMode == ViewDashboard {
				a.viewMode = ViewLog
				a.logViewer.SetSize(a.width, a.height-headerHeight-footerHeight-2)
			} else {
				a.viewMode = ViewDashboard
			}
			return a, nil

		// New PRD (opens picker in input mode)
		case "n":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog {
				a.picker.Refresh()
				a.picker.SetSize(a.width, a.height)
				a.picker.StartInputMode()
				a.viewMode = ViewPicker
			}
			return a, nil

		// List PRDs (opens picker in selection mode)
		case "l":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog {
				a.picker.Refresh()
				a.picker.SetSize(a.width, a.height)
				a.viewMode = ViewPicker
			}
			return a, nil

		// Number keys 1-9 to switch PRDs
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog {
				index := int(msg.String()[0] - '1') // Convert "1" to 0, "2" to 1, etc.
				if entry := a.tabBar.GetEntry(index); entry != nil {
					return a.switchToPRD(entry.Name, entry.Path)
				}
			}
			return a, nil

		// Loop controls (work in both views)
		case "s":
			if a.state == StateReady || a.state == StatePaused || a.state == StateError || a.state == StateStopped {
				return a.startLoop()
			}
		case "p":
			if a.state == StateRunning {
				return a.pauseLoop()
			}
		case "x":
			if a.state == StateRunning || a.state == StatePaused {
				return a.stopLoopAndUpdate()
			}

		// Navigation - different behavior based on view
		case "up", "k":
			if a.viewMode == ViewLog {
				a.logViewer.ScrollUp()
			} else {
				if a.selectedIndex > 0 {
					a.selectedIndex--
				}
			}
		case "down", "j":
			if a.viewMode == ViewLog {
				a.logViewer.ScrollDown()
			} else {
				if a.selectedIndex < len(a.prd.UserStories)-1 {
					a.selectedIndex++
				}
			}

		// Log-specific scrolling
		case "ctrl+d":
			if a.viewMode == ViewLog {
				a.logViewer.PageDown()
			}
		case "ctrl+u":
			if a.viewMode == ViewLog {
				a.logViewer.PageUp()
			}
		case "g":
			if a.viewMode == ViewLog {
				a.logViewer.ScrollToTop()
			}
		case "G":
			if a.viewMode == ViewLog {
				a.logViewer.ScrollToBottom()
			}

		// Max iterations control
		case "+", "=":
			a.adjustMaxIterations(5)
		case "-", "_":
			a.adjustMaxIterations(-5)
		}
	}

	return a, nil
}

// startLoop starts the agent loop for the current PRD.
func (a App) startLoop() (tea.Model, tea.Cmd) {
	return a.startLoopForPRD(a.prdName)
}

// startLoopForPRD starts the agent loop for a specific PRD.
func (a App) startLoopForPRD(prdName string) (tea.Model, tea.Cmd) {
	// Get the PRD directory
	prdDir := filepath.Join(a.baseDir, ".chief", "prds", prdName)

	if !git.IsGitRepo(a.baseDir) {
		return a.doStartLoop(prdName, prdDir)
	}

	branch, err := git.GetCurrentBranch(a.baseDir)
	if err != nil {
		return a.doStartLoop(prdName, prdDir)
	}

	worktreePath := git.WorktreePathForPRD(a.baseDir, prdName)
	relWorktreePath := fmt.Sprintf(".chief/worktrees/%s/", prdName)

	// Determine dialog context
	isProtected := git.IsProtectedBranch(branch)
	anotherRunningInSameDir := a.isAnotherPRDRunningInSameDir(prdName)

	var dialogCtx DialogContext
	if isProtected {
		dialogCtx = DialogProtectedBranch
	} else if anotherRunningInSameDir {
		dialogCtx = DialogAnotherPRDRunning
	} else {
		dialogCtx = DialogNoConflicts
	}

	// Show the enhanced dialog
	a.branchWarning.SetSize(a.width, a.height)
	a.branchWarning.SetContext(branch, prdName, relWorktreePath)
	a.branchWarning.SetDialogContext(dialogCtx)
	a.branchWarning.Reset()
	a.pendingStartPRD = prdName
	a.pendingWorktreePath = worktreePath
	a.viewMode = ViewBranchWarning
	return a, nil
}

// isAnotherPRDRunningInSameDir checks if another PRD is running in the project root (no worktree).
func (a *App) isAnotherPRDRunningInSameDir(prdName string) bool {
	if a.manager == nil {
		return false
	}
	for _, inst := range a.manager.GetAllInstances() {
		if inst.Name != prdName && inst.State == loop.LoopStateRunning && inst.WorktreeDir == "" {
			return true
		}
	}
	return false
}

// doStartLoop actually starts the loop (after branch check).
func (a App) doStartLoop(prdName, prdDir string) (tea.Model, tea.Cmd) {
	// Check if this PRD is registered, if not register it
	if instance := a.manager.GetInstance(prdName); instance == nil {
		// Find the PRD path
		prdPath := filepath.Join(prdDir, "prd.json")
		a.manager.Register(prdName, prdPath)
	}

	// Start the loop via manager
	if err := a.manager.Start(prdName); err != nil {
		a.lastActivity = "Error starting loop: " + err.Error()
		return a, nil
	}

	// Update state if this is the current PRD
	if prdName == a.prdName {
		a.state = StateRunning
		a.startTime = time.Now()
		a.lastActivity = "Starting loop..."
	} else {
		a.lastActivity = "Started loop for: " + prdName
	}

	return a, nil
}

// pauseLoop sets the pause flag so the loop stops after the current iteration.
func (a App) pauseLoop() (tea.Model, tea.Cmd) {
	return a.pauseLoopForPRD(a.prdName)
}

// pauseLoopForPRD pauses the loop for a specific PRD.
func (a App) pauseLoopForPRD(prdName string) (tea.Model, tea.Cmd) {
	if a.manager != nil {
		a.manager.Pause(prdName)
	}
	if prdName == a.prdName {
		a.lastActivity = "Pausing after current iteration..."
	} else {
		a.lastActivity = "Pausing " + prdName + " after current iteration..."
	}
	return a, nil
}

// stopLoop stops the loop for the current PRD immediately.
func (a *App) stopLoop() {
	a.stopLoopForPRD(a.prdName)
}

// stopLoopForPRD stops the loop for a specific PRD immediately.
func (a *App) stopLoopForPRD(prdName string) {
	if a.manager != nil {
		a.manager.Stop(prdName)
	}
}

// stopLoopAndUpdate stops the loop and updates the state.
func (a App) stopLoopAndUpdate() (tea.Model, tea.Cmd) {
	return a.stopLoopAndUpdateForPRD(a.prdName)
}

// stopLoopAndUpdateForPRD stops the loop for a specific PRD and updates state.
func (a App) stopLoopAndUpdateForPRD(prdName string) (tea.Model, tea.Cmd) {
	a.stopLoopForPRD(prdName)
	if prdName == a.prdName {
		a.state = StateStopped
		a.lastActivity = "Stopped"
	} else {
		a.lastActivity = "Stopped " + prdName
	}
	return a, nil
}

// stopAllLoops stops all running loops.
func (a *App) stopAllLoops() {
	if a.manager != nil {
		a.manager.StopAll()
	}
}

// handleLoopEvent handles events from the manager.
func (a App) handleLoopEvent(prdName string, event loop.Event) (tea.Model, tea.Cmd) {
	// Only update iteration and log if this is the currently viewed PRD
	isCurrentPRD := prdName == a.prdName

	if isCurrentPRD {
		a.iteration = event.Iteration
		// Add event to log viewer
		a.logViewer.AddEvent(event)
	}

	switch event.Type {
	case loop.EventIterationStart:
		if isCurrentPRD {
			a.lastActivity = "Starting iteration..."
		}
	case loop.EventAssistantText:
		if isCurrentPRD {
			// Truncate long text for activity display
			text := event.Text
			if len(text) > 100 {
				text = text[:97] + "..."
			}
			a.lastActivity = text
		}
	case loop.EventToolStart:
		if isCurrentPRD {
			a.lastActivity = "Running tool: " + event.Tool
		}
	case loop.EventToolResult:
		if isCurrentPRD {
			a.lastActivity = "Tool completed"
		}
	case loop.EventStoryStarted:
		if isCurrentPRD {
			a.lastActivity = "Working on: " + event.StoryID
		}
	case loop.EventComplete:
		if isCurrentPRD {
			a.state = StateComplete
			a.lastActivity = "All stories complete!"
		}
		// Trigger completion callback for any PRD
		if a.onCompletion != nil {
			a.onCompletion(prdName)
		}
	case loop.EventMaxIterationsReached:
		if isCurrentPRD {
			a.state = StatePaused
			a.lastActivity = "Max iterations reached"
		}
	case loop.EventError:
		if isCurrentPRD {
			a.state = StateError
			a.err = event.Err
			if event.Err != nil {
				a.lastActivity = "Error: " + event.Err.Error()
			}
		}
	case loop.EventRetrying:
		if isCurrentPRD {
			a.lastActivity = event.Text
		}
	}

	// Reload PRD if this is the current one to reflect any changes made by Claude
	if isCurrentPRD {
		if p, err := prd.LoadPRD(a.prdPath); err == nil {
			a.prd = p
		}
	}

	// Refresh tab bar to show updated state
	if a.tabBar != nil {
		a.tabBar.Refresh()
	}

	// Continue listening for manager events
	return a, a.listenForManagerEvents()
}

// handleLoopFinished handles when a loop finishes.
func (a App) handleLoopFinished(prdName string, err error) (tea.Model, tea.Cmd) {
	// Only update state if this is the current PRD
	if prdName == a.prdName {
		// Get the actual state from the manager
		if state, _, _ := a.manager.GetState(prdName); state != 0 {
			switch state {
			case loop.LoopStateError:
				a.state = StateError
				a.err = err
				if err != nil {
					a.lastActivity = "Error: " + err.Error()
				}
			case loop.LoopStatePaused:
				a.state = StatePaused
				a.lastActivity = "Paused"
			case loop.LoopStateStopped:
				a.state = StateStopped
				a.lastActivity = "Stopped"
			case loop.LoopStateComplete:
				a.state = StateComplete
				a.lastActivity = "All stories complete!"
			}
		}

		// Reload PRD to reflect any changes
		if p, err := prd.LoadPRD(a.prdPath); err == nil {
			a.prd = p
		}
	}

	return a, nil
}

// View renders the TUI.
func (a App) View() string {
	switch a.viewMode {
	case ViewLog:
		return a.renderLogView()
	case ViewPicker:
		return a.renderPickerView()
	case ViewHelp:
		return a.renderHelpView()
	case ViewBranchWarning:
		return a.renderBranchWarningView()
	default:
		return a.renderDashboard()
	}
}

// renderBranchWarningView renders the branch warning dialog.
func (a *App) renderBranchWarningView() string {
	a.branchWarning.SetSize(a.width, a.height)
	return a.branchWarning.Render()
}

// handleBranchWarningKeys handles keyboard input for the branch warning dialog.
func (a App) handleBranchWarningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle edit mode input
	if a.branchWarning.IsEditMode() {
		switch msg.String() {
		case "esc":
			// Cancel edit mode
			a.branchWarning.CancelEditMode()
			return a, nil
		case "enter":
			// Confirm edit
			a.branchWarning.CancelEditMode()
			return a, nil
		case "backspace":
			a.branchWarning.DeleteInputChar()
			return a, nil
		default:
			// Add character to branch name
			if len(msg.String()) == 1 {
				a.branchWarning.AddInputChar(rune(msg.String()[0]))
			}
			return a, nil
		}
	}

	switch msg.String() {
	case "esc":
		a.viewMode = ViewDashboard
		a.pendingStartPRD = ""
		a.pendingWorktreePath = ""
		a.lastActivity = "Cancelled"
		return a, nil

	case "up", "k":
		a.branchWarning.MoveUp()
		return a, nil

	case "down", "j":
		a.branchWarning.MoveDown()
		return a, nil

	case "e":
		// Start editing branch name if on an option that involves a branch
		opt := a.branchWarning.GetSelectedOption()
		if opt == BranchOptionCreateWorktree || opt == BranchOptionCreateBranch {
			a.branchWarning.StartEditMode()
		}
		return a, nil

	case "enter":
		prdName := a.pendingStartPRD
		prdDir := filepath.Join(a.baseDir, ".chief", "prds", prdName)
		a.pendingStartPRD = ""
		a.pendingWorktreePath = ""
		a.viewMode = ViewDashboard

		switch a.branchWarning.GetSelectedOption() {
		case BranchOptionCreateWorktree:
			// Create worktree + branch - will be handled by US-009/US-018
			// For now, store the choice so app.go knows the user chose worktree
			branchName := a.branchWarning.GetSuggestedBranch()
			worktreePath := git.WorktreePathForPRD(a.baseDir, prdName)
			if err := git.CreateWorktree(a.baseDir, worktreePath, branchName); err != nil {
				a.lastActivity = "Error creating worktree: " + err.Error()
				return a, nil
			}
			a.lastActivity = fmt.Sprintf("Created worktree at %s on branch %s", worktreePath, branchName)

			// Register with worktree info and start
			prdPath := filepath.Join(prdDir, "prd.json")
			if instance := a.manager.GetInstance(prdName); instance == nil {
				a.manager.RegisterWithWorktree(prdName, prdPath, worktreePath, branchName)
			}
			return a.doStartLoop(prdName, prdDir)

		case BranchOptionCreateBranch:
			// Create the branch with (possibly edited) name
			branchName := a.branchWarning.GetSuggestedBranch()
			if err := git.CreateBranch(a.baseDir, branchName); err != nil {
				a.lastActivity = "Error creating branch: " + err.Error()
				return a, nil
			}
			a.lastActivity = "Created branch: " + branchName
			// Now start the loop
			return a.doStartLoop(prdName, prdDir)

		case BranchOptionContinue:
			// Continue on current branch / run in same directory
			return a.doStartLoop(prdName, prdDir)

		case BranchOptionCancel:
			a.lastActivity = "Cancelled"
			return a, nil
		}
	}

	return a, nil
}

// renderHelpView renders the help overlay.
func (a *App) renderHelpView() string {
	a.helpOverlay.SetSize(a.width, a.height)
	return a.helpOverlay.Render()
}

// handlePickerKeys handles keyboard input when the picker is active.
func (a App) handlePickerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle input mode (creating new PRD)
	if a.picker.IsInputMode() {
		switch msg.String() {
		case "esc":
			a.picker.CancelInputMode()
			return a, nil
		case "enter":
			name := a.picker.GetInputValue()
			if name != "" {
				// Launch interactive Claude session to create the PRD
				a.picker.CancelInputMode()
				a.stopAllLoops()
				a.stopWatcher()
				return a, func() tea.Msg {
					return LaunchInitMsg{Name: name}
				}
			}
			a.picker.CancelInputMode()
			return a, nil
		case "backspace":
			a.picker.DeleteInputChar()
			return a, nil
		default:
			// Handle character input
			if len(msg.String()) == 1 {
				a.picker.AddInputChar(rune(msg.String()[0]))
			}
			return a, nil
		}
	}

	// Normal picker mode
	switch msg.String() {
	case "esc", "l":
		a.viewMode = ViewDashboard
		return a, nil
	case "q", "ctrl+c":
		a.stopAllLoops()
		a.stopWatcher()
		return a, tea.Quit
	case "up", "k":
		a.picker.MoveUp()
		a.picker.Refresh() // Refresh to get latest state
		return a, nil
	case "down", "j":
		a.picker.MoveDown()
		a.picker.Refresh() // Refresh to get latest state
		return a, nil
	case "enter":
		entry := a.picker.GetSelectedEntry()
		if entry != nil && entry.LoadError == nil {
			return a.switchToPRD(entry.Name, entry.Path)
		}
		return a, nil
	case "n":
		a.picker.StartInputMode()
		return a, nil
	case "e":
		// Edit the selected PRD - launch interactive Claude session
		entry := a.picker.GetSelectedEntry()
		if entry != nil && entry.LoadError == nil {
			a.stopAllLoops()
			a.stopWatcher()
			return a, func() tea.Msg {
				return LaunchEditMsg{Name: entry.Name}
			}
		}
		return a, nil

	// Loop controls for the SELECTED PRD (not current)
	case "s":
		entry := a.picker.GetSelectedEntry()
		if entry != nil && entry.LoadError == nil {
			state := entry.LoopState
			if state == loop.LoopStateReady || state == loop.LoopStatePaused ||
				state == loop.LoopStateStopped || state == loop.LoopStateError {
				model, cmd := a.startLoopForPRD(entry.Name)
				a.picker.Refresh()
				return model, cmd
			}
		}
		return a, nil
	case "p":
		entry := a.picker.GetSelectedEntry()
		if entry != nil && entry.LoopState == loop.LoopStateRunning {
			model, cmd := a.pauseLoopForPRD(entry.Name)
			a.picker.Refresh()
			return model, cmd
		}
		return a, nil
	case "x":
		entry := a.picker.GetSelectedEntry()
		if entry != nil {
			state := entry.LoopState
			if state == loop.LoopStateRunning || state == loop.LoopStatePaused {
				model, cmd := a.stopLoopAndUpdateForPRD(entry.Name)
				a.picker.Refresh()
				return model, cmd
			}
		}
		return a, nil
	}

	return a, nil
}

// switchToPRD switches to a different PRD (view only - does not stop other loops).
func (a App) switchToPRD(name, prdPath string) (tea.Model, tea.Cmd) {
	// Stop current watcher (but NOT the loop - it can keep running)
	a.stopWatcher()

	// Load the new PRD
	newPRD, err := prd.LoadPRD(prdPath)
	if err != nil {
		a.lastActivity = "Error loading PRD: " + err.Error()
		a.viewMode = ViewDashboard
		return a, nil
	}

	// Register with manager if not already registered
	if instance := a.manager.GetInstance(name); instance == nil {
		a.manager.Register(name, prdPath)
	}

	// Create new watcher for the new PRD
	newWatcher, err := prd.NewWatcher(prdPath)
	if err != nil {
		a.lastActivity = "Warning: file watcher failed"
	} else {
		a.watcher = newWatcher
		if err := a.watcher.Start(); err != nil {
			a.lastActivity = "Warning: file watcher failed to start"
		}
	}

	// Get the state from the manager for this PRD
	loopState, iteration, loopErr := a.manager.GetState(name)
	appState := StateReady
	switch loopState {
	case loop.LoopStateRunning:
		appState = StateRunning
	case loop.LoopStatePaused:
		appState = StatePaused
	case loop.LoopStateStopped:
		appState = StateStopped
	case loop.LoopStateComplete:
		appState = StateComplete
	case loop.LoopStateError:
		appState = StateError
	}

	// Update app state
	a.prd = newPRD
	a.prdPath = prdPath
	a.prdName = name
	a.selectedIndex = 0
	a.state = appState
	a.iteration = iteration
	a.err = loopErr
	if appState == StateRunning {
		// Keep the existing start time if running
		if instance := a.manager.GetInstance(name); instance != nil {
			a.startTime = instance.StartTime
		}
	} else {
		a.startTime = time.Time{}
	}
	a.lastActivity = "Switched to PRD: " + name
	a.viewMode = ViewDashboard
	a.picker.SetCurrentPRD(name)
	a.tabBar.SetActiveByName(name)
	a.tabBar.Refresh()

	// Clear log viewer (each PRD has its own log)
	a.logViewer.Clear()

	// Return with new watcher listener
	return a, a.listenForPRDChanges()
}

// renderPickerView renders the PRD picker modal overlaid on the dashboard.
func (a *App) renderPickerView() string {
	// Render the dashboard in the background
	background := a.renderDashboard()

	// Overlay the picker
	a.picker.SetSize(a.width, a.height)
	picker := a.picker.Render()

	// For now, just return the picker (it handles centering)
	// In a more sophisticated implementation, we could overlay with transparency
	_ = background
	return picker
}

// GetPRD returns the current PRD.
func (a *App) GetPRD() *prd.PRD {
	return a.prd
}

// GetSelectedStory returns the currently selected story.
func (a *App) GetSelectedStory() *prd.UserStory {
	if a.selectedIndex >= 0 && a.selectedIndex < len(a.prd.UserStories) {
		return &a.prd.UserStories[a.selectedIndex]
	}
	return nil
}

// GetState returns the current app state.
func (a *App) GetState() AppState {
	return a.state
}

// GetIteration returns the current iteration count.
func (a *App) GetIteration() int {
	return a.iteration
}

// GetElapsedTime returns the elapsed time since the loop started.
func (a *App) GetElapsedTime() time.Duration {
	if a.startTime.IsZero() {
		return 0
	}
	return time.Since(a.startTime)
}

// GetCompletionPercentage returns the percentage of completed stories.
func (a *App) GetCompletionPercentage() float64 {
	if len(a.prd.UserStories) == 0 {
		return 100.0
	}
	var completed int
	for _, s := range a.prd.UserStories {
		if s.Passes {
			completed++
		}
	}
	return float64(completed) / float64(len(a.prd.UserStories)) * 100.0
}

// GetLastActivity returns the last activity message.
func (a *App) GetLastActivity() string {
	return a.lastActivity
}

// adjustMaxIterations adjusts the max iterations by delta.
func (a *App) adjustMaxIterations(delta int) {
	newMax := a.maxIter + delta
	if newMax < 1 {
		newMax = 1
	}
	a.maxIter = newMax

	// Update the manager's default
	if a.manager != nil {
		a.manager.SetMaxIterations(newMax)
		// Also update any running loop for the current PRD
		a.manager.SetMaxIterationsForInstance(a.prdName, newMax)
	}

	a.lastActivity = fmt.Sprintf("Max iterations: %d", newMax)
}

// listenForPRDChanges listens for PRD file changes and returns them as messages.
func (a *App) listenForPRDChanges() tea.Cmd {
	if a.watcher == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-a.watcher.Events()
		if !ok {
			return nil
		}
		return PRDUpdateMsg{PRD: event.PRD, Error: event.Error}
	}
}

// handlePRDUpdate handles PRD file change events.
func (a App) handlePRDUpdate(msg PRDUpdateMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		// File error - could be temporary, keep watching
		a.lastActivity = "PRD file error: " + msg.Error.Error()
	} else if msg.PRD != nil {
		// Update the PRD
		a.prd = msg.PRD

		// Adjust selected index if it's now out of bounds
		if a.selectedIndex >= len(a.prd.UserStories) {
			a.selectedIndex = len(a.prd.UserStories) - 1
			if a.selectedIndex < 0 {
				a.selectedIndex = 0
			}
		}
	}

	// Continue listening for changes
	return a, a.listenForPRDChanges()
}

// stopWatcher stops the file watcher.
func (a *App) stopWatcher() {
	if a.watcher != nil {
		a.watcher.Stop()
	}
}
