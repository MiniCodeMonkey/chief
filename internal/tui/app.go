package tui

import (
	"fmt"
	"os"
	"os/exec"
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

// ProgressUpdateMsg is sent when progress.md changes.
type ProgressUpdateMsg struct {
	Entries map[string][]prd.ProgressEntry
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

// mergeResultMsg is sent when a merge operation completes.
type mergeResultMsg struct {
	branch    string
	conflicts []string
	output    string
	err       error
}

// cleanResultMsg is sent when a clean operation completes.
type cleanResultMsg struct {
	prdName      string
	success      bool
	message      string
	clearBranch  bool
}

// autoActionResultMsg is sent when a post-completion auto-action (push/PR) completes.
type autoActionResultMsg struct {
	action  string // "push" or "pr"
	err     error
	prURL   string // Only set for successful PR creation
	prTitle string // Only set for successful PR creation
}

// completionSpinnerTickMsg is sent to animate the completion screen spinner.
type completionSpinnerTickMsg struct{}

// confettiTickMsg is sent to animate confetti particles on the completion screen.
type confettiTickMsg struct{}

// worktreeStepResultMsg is sent when a worktree setup step completes.
type worktreeStepResultMsg struct {
	step WorktreeSpinnerStep
	err  error
}

// worktreeSpinnerTickMsg is sent to animate the worktree setup spinner.
type worktreeSpinnerTickMsg struct{}

// elapsedTickMsg is sent every second to update the elapsed time display.
type elapsedTickMsg struct{}

// settingsGHCheckResultMsg is sent when GH CLI validation completes in settings.
type settingsGHCheckResultMsg struct {
	installed     bool
	authenticated bool
	err           error
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
	ViewDiff
	ViewPicker
	ViewHelp
	ViewBranchWarning
	ViewWorktreeSpinner
	ViewCompletion
	ViewSettings
	ViewQuitConfirm
)

// App is the main Bubble Tea model for the Chief TUI.
type App struct {
	prd           *prd.PRD
	prdPath       string
	prdName       string
	state         AppState
	iteration     int
	startTime     time.Time
	selectedIndex      int
	storiesScrollOffset int
	width              int
	height             int
	err           error

	// Loop manager for parallel PRD execution
	manager *loop.Manager
	maxIter int

	// Activity tracking
	lastActivity string

	// File watching
	watcher         *prd.Watcher
	progressWatcher *prd.ProgressWatcher
	progress        map[string][]prd.ProgressEntry

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

	// Diff viewer
	diffViewer *DiffViewer

	// Help overlay
	helpOverlay      *HelpOverlay
	previousViewMode ViewMode // View to return to when closing help

	// Branch warning dialog
	branchWarning      *BranchWarning
	pendingStartPRD    string // PRD name waiting to start after branch decision
	pendingWorktreePath string // Absolute worktree path for pending PRD

	// Worktree setup spinner
	worktreeSpinner *WorktreeSpinner

	// Completion screen
	completionScreen *CompletionScreen

	// Story timing tracking
	storyTimings     []StoryTiming
	currentStoryID   string
	currentStoryStart time.Time

	// Settings overlay
	settingsOverlay *SettingsOverlay

	// Quit confirmation dialog
	quitConfirm *QuitConfirmation

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

	// Prune stale worktrees on startup (clean git's internal tracking)
	if git.IsGitRepo(baseDir) {
		_ = git.PruneWorktrees(baseDir)
	}

	// Create progress watcher and load initial progress
	progressWatcher, _ := prd.NewProgressWatcher(prdPath)
	progress, _ := prd.ParseProgress(prd.ProgressPath(prdPath))

	// Create loop manager for parallel PRD execution
	manager := loop.NewManager(maxIter)
	manager.SetBaseDir(baseDir)
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
		watcher:         watcher,
		progressWatcher: progressWatcher,
		progress:        progress,
		viewMode:        ViewDashboard,
		logViewer:     NewLogViewer(),
		diffViewer:    NewDiffViewer(baseDir),
		tabBar:        tabBar,
		picker:        picker,
		baseDir:       baseDir,
		config:        cfg,
		helpOverlay:      NewHelpOverlay(),
		branchWarning:    NewBranchWarning(),
		worktreeSpinner:  NewWorktreeSpinner(),
		completionScreen: NewCompletionScreen(),
		settingsOverlay:  NewSettingsOverlay(),
		quitConfirm:     NewQuitConfirmation(),
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

	// Start the progress watcher
	if a.progressWatcher != nil {
		_ = a.progressWatcher.Start()
	}

	return tea.Batch(
		tea.EnterAltScreen,
		a.listenForPRDChanges(),
		a.listenForManagerEvents(),
		a.listenForProgressChanges(),
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
		// Log viewer size is set authoritatively in renderLogView (with correct -4 width).
		// Only update height here for scroll calculations; width will match on next render.
		a.logViewer.SetSize(a.width-4, a.height-headerHeight-footerHeight-2)
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

	case mergeResultMsg:
		return a.handleMergeResult(msg)

	case cleanResultMsg:
		return a.handleCleanResult(msg)

	case autoActionResultMsg:
		return a.handleAutoActionResult(msg)

	case backgroundAutoActionResultMsg:
		return a.handleBackgroundAutoAction(msg)

	case completionSpinnerTickMsg:
		if a.viewMode == ViewCompletion && a.completionScreen.IsAutoActionRunning() {
			a.completionScreen.Tick()
			return a, tickCompletionSpinner()
		}
		return a, nil

	case confettiTickMsg:
		if a.viewMode == ViewCompletion && a.completionScreen.HasConfetti() {
			a.completionScreen.TickConfetti()
			return a, tickConfetti()
		}
		return a, nil

	case worktreeStepResultMsg:
		return a.handleWorktreeStepResult(msg)

	case elapsedTickMsg:
		if a.state == StateRunning {
			return a, tickElapsed()
		}
		return a, nil

	case worktreeSpinnerTickMsg:
		if a.viewMode == ViewWorktreeSpinner {
			a.worktreeSpinner.Tick()
			return a, tickWorktreeSpinner()
		}
		return a, nil

	case settingsGHCheckResultMsg:
		return a.handleSettingsGHCheck(msg)

	case ProgressUpdateMsg:
		a.progress = msg.Entries
		return a, a.listenForProgressChanges()

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

		// Handle settings overlay (can be opened/closed from any view)
		if msg.String() == "," {
			if a.viewMode == ViewSettings {
				// Close settings
				a.viewMode = a.previousViewMode
				return a, nil
			}
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog || a.viewMode == ViewPicker || a.viewMode == ViewCompletion {
				a.previousViewMode = a.viewMode
				a.settingsOverlay.SetSize(a.width, a.height)
				a.settingsOverlay.LoadFromConfig(a.config)
				a.viewMode = ViewSettings
				return a, nil
			}
		}

		// Handle help view (only Esc closes it besides ?)
		if a.viewMode == ViewHelp {
			if msg.String() == "esc" {
				a.viewMode = a.previousViewMode
			}
			// Ignore other keys in help view
			return a, nil
		}

		// Handle settings view
		if a.viewMode == ViewSettings {
			return a.handleSettingsKeys(msg)
		}

		// Handle picker view separately (it has its own input mode)
		if a.viewMode == ViewPicker {
			return a.handlePickerKeys(msg)
		}

		// Handle branch warning view
		if a.viewMode == ViewBranchWarning {
			return a.handleBranchWarningKeys(msg)
		}

		// Handle worktree spinner view - only Esc is active
		if a.viewMode == ViewWorktreeSpinner {
			return a.handleWorktreeSpinnerKeys(msg)
		}

		// Handle completion screen view
		if a.viewMode == ViewCompletion {
			return a.handleCompletionKeys(msg)
		}

		// Handle quit confirmation dialog
		if a.viewMode == ViewQuitConfirm {
			return a.handleQuitConfirmKeys(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return a.tryQuit()

		// View switching
		case "t":
			if a.viewMode == ViewDashboard || a.viewMode == ViewDiff {
				a.viewMode = ViewLog
				// SetSize is handled by renderLogView with correct dimensions
			} else {
				a.viewMode = ViewDashboard
			}
			return a, nil

		// Diff view
		case "d":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog {
				// Use the current PRD's worktree directory if available, otherwise base dir
				diffDir := a.baseDir
				if instance := a.manager.GetInstance(a.prdName); instance != nil && instance.WorktreeDir != "" {
					diffDir = instance.WorktreeDir
				}
				a.diffViewer.SetBaseDir(diffDir)
				a.diffViewer.SetSize(a.width-4, a.height-headerHeight-footerHeight-2)
				// Load diff for the selected story's commit
				if story := a.GetSelectedStory(); story != nil {
					a.diffViewer.LoadForStory(story.ID, story.Title)
				} else {
					a.diffViewer.Load()
				}
				a.viewMode = ViewDiff
			} else if a.viewMode == ViewDiff {
				a.viewMode = ViewDashboard
			}
			return a, nil

		// New PRD (opens picker in input mode)
		case "n":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog || a.viewMode == ViewDiff {
				a.picker.Refresh()
				a.picker.SetSize(a.width, a.height)
				a.picker.StartInputMode()
				a.viewMode = ViewPicker
			}
			return a, nil

		// List PRDs (opens picker in selection mode)
		case "l":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog || a.viewMode == ViewDiff {
				a.picker.Refresh()
				a.picker.SetSize(a.width, a.height)
				a.viewMode = ViewPicker
			}
			return a, nil

		// Edit current PRD
		case "e":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog || a.viewMode == ViewDiff {
				a.stopAllLoops()
				a.stopWatcher()
				return a, func() tea.Msg {
					return LaunchEditMsg{Name: a.prdName}
				}
			}
			return a, nil

		// Number keys 1-9 to switch PRDs
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog || a.viewMode == ViewDiff {
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
			} else if a.viewMode == ViewDiff {
				a.diffViewer.ScrollUp()
			} else {
				if a.selectedIndex > 0 {
					a.selectedIndex--
					if a.selectedIndex < a.storiesScrollOffset {
						a.storiesScrollOffset = a.selectedIndex
					}
				}
			}
		case "down", "j":
			if a.viewMode == ViewLog {
				a.logViewer.ScrollDown()
			} else if a.viewMode == ViewDiff {
				a.diffViewer.ScrollDown()
			} else {
				if a.selectedIndex < len(a.prd.UserStories)-1 {
					a.selectedIndex++
					a.adjustStoriesScroll()
				}
			}

		// Log/diff scrolling
		case "ctrl+d", "pgdown":
			if a.viewMode == ViewLog {
				a.logViewer.PageDown()
			} else if a.viewMode == ViewDiff {
				a.diffViewer.PageDown()
			}
		case "ctrl+u", "pgup":
			if a.viewMode == ViewLog {
				a.logViewer.PageUp()
			} else if a.viewMode == ViewDiff {
				a.diffViewer.PageUp()
			}
		case "g":
			if a.viewMode == ViewLog {
				a.logViewer.ScrollToTop()
			} else if a.viewMode == ViewDiff {
				a.diffViewer.ScrollToTop()
			}
		case "G":
			if a.viewMode == ViewLog {
				a.logViewer.ScrollToBottom()
			} else if a.viewMode == ViewDiff {
				a.diffViewer.ScrollToBottom()
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

	if !isProtected && !anotherRunningInSameDir {
		// No conflicts: skip the dialog entirely and start the loop directly
		return a.doStartLoop(prdName, prdDir)
	}

	var dialogCtx DialogContext
	if isProtected {
		dialogCtx = DialogProtectedBranch
	} else {
		dialogCtx = DialogAnotherPRDRunning
	}

	// Show the dialog only for protected branch or another PRD running
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
		// Reset story timing state
		a.storyTimings = nil
		a.currentStoryID = ""
		a.currentStoryStart = time.Time{}
		return a, tickElapsed()
	}

	a.lastActivity = "Started loop for: " + prdName
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

// tryQuit attempts to quit the app. If any loop is running, it shows the quit
// confirmation dialog instead of quitting immediately.
func (a App) tryQuit() (tea.Model, tea.Cmd) {
	if a.manager != nil && a.manager.IsAnyRunning() {
		a.previousViewMode = a.viewMode
		a.viewMode = ViewQuitConfirm
		a.quitConfirm.Reset()
		a.quitConfirm.SetSize(a.width, a.height)
		return a, nil
	}
	a.stopAllLoops()
	a.stopWatcher()
	return a, tea.Quit
}

// handleQuitConfirmKeys handles keyboard input for the quit confirmation dialog.
func (a App) handleQuitConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.viewMode = a.previousViewMode
		return a, nil
	case "up", "k":
		a.quitConfirm.MoveUp()
		return a, nil
	case "down", "j":
		a.quitConfirm.MoveDown()
		return a, nil
	case "enter":
		if a.quitConfirm.GetSelected() == QuitOptionQuit {
			a.stopAllLoops()
			a.stopWatcher()
			return a, tea.Quit
		}
		// Cancel
		a.viewMode = a.previousViewMode
		return a, nil
	}
	return a, nil
}

// renderQuitConfirmView renders the quit confirmation dialog.
func (a *App) renderQuitConfirmView() string {
	a.quitConfirm.SetSize(a.width, a.height)
	return a.quitConfirm.Render()
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

	var autoActionCmd tea.Cmd

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
			// Finalize previous story timing
			a.finalizeStoryTiming()
			// Start tracking the new story
			a.currentStoryID = event.StoryID
			a.currentStoryStart = time.Now()
		}
	case loop.EventComplete:
		if isCurrentPRD {
			a.state = StateComplete
			a.lastActivity = "All stories complete!"
			// Finalize the last story's timing
			a.finalizeStoryTiming()
			autoActionCmd = a.showCompletionScreen(prdName)
		} else {
			// For background PRDs, trigger auto-push/PR without showing completion screen
			autoActionCmd = a.runBackgroundAutoActions(prdName)
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
	case loop.EventWatchdogTimeout:
		if isCurrentPRD {
			a.lastActivity = event.Text
		}
	}

	// Reload PRD from disk only on meaningful state changes (not every event)
	if isCurrentPRD {
		switch event.Type {
		case loop.EventStoryStarted, loop.EventComplete, loop.EventError, loop.EventMaxIterationsReached:
			if p, err := prd.LoadPRD(a.prdPath); err == nil {
				a.prd = p
			}
		}

		// Mark the story as in-progress in the PRD and auto-select it
		if event.Type == loop.EventStoryStarted && event.StoryID != "" {
			a.markStoryInProgress(event.StoryID)
			a.selectStoryByID(event.StoryID)
		}

		// Clear in-progress when the PRD completes or the loop stops
		if event.Type == loop.EventComplete || event.Type == loop.EventError || event.Type == loop.EventMaxIterationsReached {
			a.clearInProgress()
		}
	}

	// Refresh tab bar to show updated state
	if a.tabBar != nil {
		a.tabBar.Refresh()
	}

	// Continue listening for manager events, plus any auto-action commands
	if autoActionCmd != nil {
		return a, tea.Batch(a.listenForManagerEvents(), autoActionCmd)
	}
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
	case ViewDiff:
		return a.renderDiffView()
	case ViewPicker:
		return a.renderPickerView()
	case ViewHelp:
		return a.renderHelpView()
	case ViewBranchWarning:
		return a.renderBranchWarningView()
	case ViewWorktreeSpinner:
		return a.renderWorktreeSpinnerView()
	case ViewCompletion:
		return a.renderCompletionView()
	case ViewSettings:
		return a.renderSettingsView()
	case ViewQuitConfirm:
		return a.renderQuitConfirmView()
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
			branchName := a.branchWarning.GetSuggestedBranch()
			worktreePath := git.WorktreePathForPRD(a.baseDir, prdName)
			relWorktreePath := fmt.Sprintf(".chief/worktrees/%s/", prdName)

			// Detect default branch for display
			defaultBranch := "main"
			if db, err := git.GetDefaultBranch(a.baseDir); err == nil {
				defaultBranch = db
			}

			// Configure and show the spinner
			a.worktreeSpinner.Configure(prdName, branchName, defaultBranch, relWorktreePath, a.config.Worktree.Setup)
			a.worktreeSpinner.SetSize(a.width, a.height)
			a.pendingStartPRD = prdName
			a.pendingWorktreePath = worktreePath
			a.viewMode = ViewWorktreeSpinner

			// Start the first async step (create worktree which includes branch creation)
			return a, tea.Batch(
				tickWorktreeSpinner(),
				a.runWorktreeStep(SpinnerStepCreateBranch, a.baseDir, worktreePath, branchName),
			)

		case BranchOptionCreateBranch:
			// Create the branch with (possibly edited) name
			branchName := a.branchWarning.GetSuggestedBranch()
			if err := git.CreateBranch(a.baseDir, branchName); err != nil {
				a.lastActivity = "Error creating branch: " + err.Error()
				return a, nil
			}
			// Track the branch on the manager instance
			if instance := a.manager.GetInstance(prdName); instance != nil {
				a.manager.UpdateWorktreeInfo(prdName, "", branchName)
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

// renderWorktreeSpinnerView renders the worktree setup spinner.
func (a *App) renderWorktreeSpinnerView() string {
	a.worktreeSpinner.SetSize(a.width, a.height)
	return a.worktreeSpinner.Render()
}

// handleWorktreeSpinnerKeys handles keyboard input for the worktree spinner.
func (a App) handleWorktreeSpinnerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel setup and clean up
		a.worktreeSpinner.Cancel()
		a.cleanupWorktreeSetup()
		a.viewMode = ViewDashboard
		a.lastActivity = "Worktree setup cancelled"
		a.pendingStartPRD = ""
		a.pendingWorktreePath = ""
		return a, nil
	}
	// Ignore all other keys during spinner
	return a, nil
}

// cleanupWorktreeSetup cleans up a partially created worktree and branch.
func (a *App) cleanupWorktreeSetup() {
	if a.pendingWorktreePath != "" {
		// Try to remove the worktree if it was created
		if git.IsWorktree(a.pendingWorktreePath) {
			_ = git.RemoveWorktree(a.baseDir, a.pendingWorktreePath)
		}
	}
}

// finalizeStoryTiming records the duration of the currently tracked story.
func (a *App) finalizeStoryTiming() {
	if a.currentStoryID == "" {
		return
	}
	duration := time.Since(a.currentStoryStart)
	title := a.currentStoryID
	// Look up the story title from the PRD
	for _, story := range a.prd.UserStories {
		if story.ID == a.currentStoryID {
			title = story.Title
			break
		}
	}
	a.storyTimings = append(a.storyTimings, StoryTiming{
		StoryID:  a.currentStoryID,
		Title:    title,
		Duration: duration,
	})
	a.currentStoryID = ""
	a.currentStoryStart = time.Time{}
}

// showCompletionScreen configures and shows the completion screen for a PRD.
// Returns a tea.Cmd if auto-actions need to be started, nil otherwise.
func (a *App) showCompletionScreen(prdName string) tea.Cmd {
	// Count completed stories
	completed := 0
	total := len(a.prd.UserStories)
	for _, story := range a.prd.UserStories {
		if story.Passes {
			completed++
		}
	}

	// Get branch from manager
	branch := ""
	if instance := a.manager.GetInstance(prdName); instance != nil {
		branch = instance.Branch
	}

	// Count commits on the branch
	commitCount := 0
	if branch != "" {
		commitCount = git.CommitCount(a.baseDir, branch)
	}

	// Check if auto-actions are configured
	hasAutoActions := a.config != nil && (a.config.OnComplete.Push || a.config.OnComplete.CreatePR)

	totalDuration := a.GetElapsedTime()
	a.completionScreen.Configure(prdName, completed, total, branch, commitCount, hasAutoActions, totalDuration, a.storyTimings)
	a.completionScreen.SetSize(a.width, a.height)
	a.viewMode = ViewCompletion

	// Always start confetti tick
	cmds := []tea.Cmd{tickConfetti()}

	// Trigger auto-push if configured and branch is set
	if a.config != nil && a.config.OnComplete.Push && branch != "" {
		a.completionScreen.SetPushInProgress()
		cmds = append(cmds, tickCompletionSpinner(), a.runAutoPush())
	}

	// If only PR is configured (no push), we can't create a PR without pushing first
	// So PR-only without push is a no-op (push is required for PR)
	return tea.Batch(cmds...)
}

// backgroundAutoActionResultMsg is sent when a background PRD auto-action completes.
type backgroundAutoActionResultMsg struct {
	prdName string
	action  string // "push" or "pr"
	err     error
}

// runBackgroundAutoActions triggers auto-push/PR for a background PRD that just completed.
func (a *App) runBackgroundAutoActions(prdName string) tea.Cmd {
	if a.config == nil || !a.config.OnComplete.Push {
		return nil
	}

	instance := a.manager.GetInstance(prdName)
	if instance == nil || instance.Branch == "" {
		return nil
	}

	branch := instance.Branch
	dir := a.baseDir
	if instance.WorktreeDir != "" {
		dir = instance.WorktreeDir
	}

	return func() tea.Msg {
		if err := git.PushBranch(dir, branch); err != nil {
			return backgroundAutoActionResultMsg{prdName: prdName, action: "push", err: err}
		}
		return backgroundAutoActionResultMsg{prdName: prdName, action: "push"}
	}
}

// handleAutoActionResult handles the result of an auto-action (push or PR creation).
func (a App) handleAutoActionResult(msg autoActionResultMsg) (tea.Model, tea.Cmd) {
	switch msg.action {
	case "push":
		if msg.err != nil {
			a.completionScreen.SetPushError(msg.err.Error())
			return a, nil
		}
		a.completionScreen.SetPushSuccess()

		// If PR creation is configured, start it now
		if a.config != nil && a.config.OnComplete.CreatePR && a.completionScreen.HasBranch() {
			a.completionScreen.SetPRInProgress()
			return a, tea.Batch(
				tickCompletionSpinner(),
				a.runAutoCreatePR(),
			)
		}
		return a, nil

	case "pr":
		if msg.err != nil {
			a.completionScreen.SetPRError(msg.err.Error())
			return a, nil
		}
		a.completionScreen.SetPRSuccess(msg.prURL, msg.prTitle)
		return a, nil
	}
	return a, nil
}

// handleBackgroundAutoAction handles auto-action results for background PRDs.
func (a App) handleBackgroundAutoAction(msg backgroundAutoActionResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Log error but don't block - background action failed silently
		return a, nil
	}

	if msg.action == "push" && a.config != nil && a.config.OnComplete.CreatePR {
		// Chain PR creation after successful push
		instance := a.manager.GetInstance(msg.prdName)
		if instance != nil && instance.Branch != "" {
			prdName := msg.prdName
			branch := instance.Branch
			dir := a.baseDir
			prdPath := filepath.Join(a.baseDir, ".chief", "prds", prdName, "prd.json")
			return a, func() tea.Msg {
				p, err := prd.LoadPRD(prdPath)
				if err != nil {
					return backgroundAutoActionResultMsg{prdName: prdName, action: "pr", err: err}
				}
				title := git.PRTitleFromPRD(prdName, p)
				body := git.PRBodyFromPRD(p)
				_, err = git.CreatePR(dir, branch, title, body)
				return backgroundAutoActionResultMsg{prdName: prdName, action: "pr", err: err}
			}
		}
	}

	return a, nil
}

// runAutoPush returns a tea.Cmd that pushes the branch in the background.
func (a *App) runAutoPush() tea.Cmd {
	branch := a.completionScreen.Branch()
	// Use worktree dir if available, otherwise base dir
	dir := a.baseDir
	if instance := a.manager.GetInstance(a.completionScreen.PRDName()); instance != nil && instance.WorktreeDir != "" {
		dir = instance.WorktreeDir
	}
	return func() tea.Msg {
		err := git.PushBranch(dir, branch)
		return autoActionResultMsg{action: "push", err: err}
	}
}

// runAutoCreatePR returns a tea.Cmd that creates a PR in the background.
func (a *App) runAutoCreatePR() tea.Cmd {
	prdName := a.completionScreen.PRDName()
	branch := a.completionScreen.Branch()
	dir := a.baseDir

	// Load the PRD to generate PR content
	prdPath := filepath.Join(a.baseDir, ".chief", "prds", prdName, "prd.json")
	return func() tea.Msg {
		p, err := prd.LoadPRD(prdPath)
		if err != nil {
			return autoActionResultMsg{action: "pr", err: fmt.Errorf("failed to load PRD: %s", err.Error())}
		}
		title := git.PRTitleFromPRD(prdName, p)
		body := git.PRBodyFromPRD(p)
		url, err := git.CreatePR(dir, branch, title, body)
		if err != nil {
			return autoActionResultMsg{action: "pr", err: err}
		}
		return autoActionResultMsg{action: "pr", prURL: url, prTitle: title}
	}
}

// renderCompletionView renders the completion screen.
func (a *App) renderCompletionView() string {
	a.completionScreen.SetSize(a.width, a.height)
	return a.completionScreen.Render()
}

// renderSettingsView renders the settings overlay.
func (a *App) renderSettingsView() string {
	a.settingsOverlay.SetSize(a.width, a.height)
	return a.settingsOverlay.Render()
}

// handleSettingsKeys handles keyboard input for the settings overlay.
func (a App) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dismiss GH error on any key
	if a.settingsOverlay.HasGHError() {
		a.settingsOverlay.DismissGHError()
		return a, nil
	}

	// Handle inline text editing
	if a.settingsOverlay.IsEditing() {
		switch msg.String() {
		case "enter":
			a.settingsOverlay.ConfirmEdit()
			a.settingsOverlay.ApplyToConfig(a.config)
			_ = config.Save(a.baseDir, a.config)
			return a, nil
		case "esc":
			a.settingsOverlay.CancelEdit()
			return a, nil
		case "backspace":
			a.settingsOverlay.DeleteEditChar()
			return a, nil
		default:
			if len(msg.String()) == 1 {
				a.settingsOverlay.AddEditChar(rune(msg.String()[0]))
			}
			return a, nil
		}
	}

	switch msg.String() {
	case "esc":
		a.viewMode = a.previousViewMode
		return a, nil
	case "q", "ctrl+c":
		return a.tryQuit()
	case "up", "k":
		a.settingsOverlay.MoveUp()
		return a, nil
	case "down", "j":
		a.settingsOverlay.MoveDown()
		return a, nil
	case "enter":
		item := a.settingsOverlay.GetSelectedItem()
		if item == nil {
			return a, nil
		}
		switch item.Type {
		case SettingsItemBool:
			key, newVal := a.settingsOverlay.ToggleBool()
			if key == "onComplete.createPR" && newVal {
				// Validate GH CLI asynchronously
				return a, func() tea.Msg {
					installed, authenticated, err := git.CheckGHCLI()
					return settingsGHCheckResultMsg{installed: installed, authenticated: authenticated, err: err}
				}
			}
			a.settingsOverlay.ApplyToConfig(a.config)
			_ = config.Save(a.baseDir, a.config)
			return a, nil
		case SettingsItemString:
			a.settingsOverlay.StartEditing()
			return a, nil
		}
	}

	return a, nil
}

// handleSettingsGHCheck handles the GH CLI check result from settings.
func (a App) handleSettingsGHCheck(msg settingsGHCheckResultMsg) (tea.Model, tea.Cmd) {
	if a.viewMode != ViewSettings {
		return a, nil
	}

	if msg.err != nil || !msg.installed || !msg.authenticated {
		// Validation failed - revert toggle and show error
		a.settingsOverlay.RevertToggle()
		errMsg := "GitHub CLI (gh) is not installed"
		if msg.installed && !msg.authenticated {
			errMsg = "GitHub CLI (gh) is not authenticated. Run: gh auth login"
		}
		if msg.err != nil {
			errMsg = msg.err.Error()
		}
		a.settingsOverlay.SetGHError(errMsg)
		return a, nil
	}

	// Validation passed - save the config
	a.settingsOverlay.ApplyToConfig(a.config)
	_ = config.Save(a.baseDir, a.config)
	return a, nil
}

// handleCompletionKeys handles keyboard input for the completion screen.
func (a App) handleCompletionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return a.tryQuit()

	case "l":
		// Switch to the picker
		a.picker.Refresh()
		a.picker.SetSize(a.width, a.height)
		a.viewMode = ViewPicker
		return a, nil

	case "m":
		// Merge the completed PRD's branch
		if a.completionScreen.HasBranch() {
			branch := a.completionScreen.Branch()
			baseDir := a.baseDir
			a.viewMode = ViewDashboard
			return a, func() tea.Msg {
				conflicts, err := git.MergeBranch(baseDir, branch)
				if err != nil {
					return mergeResultMsg{branch: branch, conflicts: conflicts, err: err}
				}
				output := parseMergeSuccessMessage(baseDir, branch)
				return mergeResultMsg{branch: branch, output: output}
			}
		}
		return a, nil

	case "c":
		// Clean the PRD's worktree - switch to picker with clean dialog
		if a.completionScreen.HasBranch() {
			prdName := a.completionScreen.PRDName()
			a.picker.Refresh()
			a.picker.SetSize(a.width, a.height)
			// Select the completed PRD in the picker
			for i, entry := range a.picker.entries {
				if entry.Name == prdName {
					a.picker.selectedIndex = i
					break
				}
			}
			if a.picker.CanClean() {
				a.picker.StartCleanConfirmation()
			}
			a.viewMode = ViewPicker
		}
		return a, nil

	case "esc":
		a.viewMode = ViewDashboard
		return a, nil
	}

	return a, nil
}

// tickCompletionSpinner returns a tea.Cmd that ticks the completion screen spinner.
func tickCompletionSpinner() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return completionSpinnerTickMsg{}
	})
}

// tickConfetti returns a tea.Cmd that ticks the confetti animation.
func tickConfetti() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return confettiTickMsg{}
	})
}

// tickWorktreeSpinner returns a tea.Cmd that ticks the spinner animation.
func tickWorktreeSpinner() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return worktreeSpinnerTickMsg{}
	})
}

// tickElapsed returns a tea.Cmd that ticks every second for the elapsed time display.
func tickElapsed() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return elapsedTickMsg{}
	})
}

// runWorktreeStep runs a worktree setup step asynchronously.
func (a *App) runWorktreeStep(step WorktreeSpinnerStep, baseDir, worktreePath, branchName string) tea.Cmd {
	switch step {
	case SpinnerStepCreateBranch:
		return func() tea.Msg {
			// CreateWorktree handles both branch creation and worktree addition
			if err := git.CreateWorktree(baseDir, worktreePath, branchName); err != nil {
				return worktreeStepResultMsg{step: SpinnerStepCreateBranch, err: err}
			}
			return worktreeStepResultMsg{step: SpinnerStepCreateBranch}
		}

	case SpinnerStepRunSetup:
		setupCmd := a.config.Worktree.Setup
		return func() tea.Msg {
			cmd := exec.Command("sh", "-c", setupCmd)
			cmd.Dir = worktreePath
			if out, err := cmd.CombinedOutput(); err != nil {
				return worktreeStepResultMsg{
					step: SpinnerStepRunSetup,
					err:  fmt.Errorf("%s\n%s", err.Error(), strings.TrimSpace(string(out))),
				}
			}
			return worktreeStepResultMsg{step: SpinnerStepRunSetup}
		}
	}
	return nil
}

// handleWorktreeStepResult handles the result of a worktree setup step.
func (a App) handleWorktreeStepResult(msg worktreeStepResultMsg) (tea.Model, tea.Cmd) {
	// Ignore results if we've already cancelled or left the spinner view
	if a.viewMode != ViewWorktreeSpinner || a.worktreeSpinner.IsCancelled() {
		return a, nil
	}

	if msg.err != nil {
		a.worktreeSpinner.SetError(msg.err.Error())
		return a, nil
	}

	switch msg.step {
	case SpinnerStepCreateBranch:
		// Branch creation completed - advance through both branch and worktree steps
		// (CreateWorktree does both in one call)
		a.worktreeSpinner.AdvanceStep() // Complete "Creating branch"
		a.worktreeSpinner.AdvanceStep() // Complete "Creating worktree"

		// Check if we need to run setup
		if a.worktreeSpinner.HasSetupCommand() {
			return a, a.runWorktreeStep(SpinnerStepRunSetup, a.baseDir, a.pendingWorktreePath, "")
		}

		// No setup - we're done, transition to loop
		return a.finishWorktreeSetup()

	case SpinnerStepRunSetup:
		a.worktreeSpinner.AdvanceStep() // Complete "Running setup"
		return a.finishWorktreeSetup()
	}

	return a, nil
}

// finishWorktreeSetup completes the worktree setup and starts the loop.
func (a App) finishWorktreeSetup() (tea.Model, tea.Cmd) {
	prdName := a.pendingStartPRD
	worktreePath := a.pendingWorktreePath
	branchName := a.worktreeSpinner.branchName
	prdDir := filepath.Join(a.baseDir, ".chief", "prds", prdName)

	// Register or update with worktree info
	prdPath := filepath.Join(prdDir, "prd.json")
	if instance := a.manager.GetInstance(prdName); instance == nil {
		a.manager.RegisterWithWorktree(prdName, prdPath, worktreePath, branchName)
	} else {
		a.manager.UpdateWorktreeInfo(prdName, worktreePath, branchName)
	}

	a.lastActivity = fmt.Sprintf("Created worktree at %s on branch %s", worktreePath, branchName)
	a.viewMode = ViewDashboard
	a.pendingStartPRD = ""
	a.pendingWorktreePath = ""

	return a.doStartLoop(prdName, prdDir)
}

// handleMergeResult handles the result of an async merge operation.
func (a App) handleMergeResult(msg mergeResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.picker.SetMergeResult(&MergeResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to merge %s into current branch", msg.branch),
			Conflicts: msg.conflicts,
			Branch:    msg.branch,
		})
	} else {
		a.picker.SetMergeResult(&MergeResult{
			Success: true,
			Message: msg.output,
			Branch:  msg.branch,
		})
		a.lastActivity = fmt.Sprintf("Merged %s", msg.branch)
	}
	// Switch to picker to show the merge result if not already there
	if a.viewMode != ViewPicker {
		a.picker.Refresh()
		a.picker.SetSize(a.width, a.height)
		a.viewMode = ViewPicker
	}
	return a, nil
}

// handleCleanConfirmationKeys handles keyboard input for the clean confirmation dialog.
func (a App) handleCleanConfirmationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.picker.CancelCleanConfirmation()
		return a, nil
	case "up", "k":
		a.picker.CleanConfirmMoveUp()
		return a, nil
	case "down", "j":
		a.picker.CleanConfirmMoveDown()
		return a, nil
	case "enter":
		cc := a.picker.GetCleanConfirmation()
		if cc == nil {
			return a, nil
		}

		option := a.picker.GetCleanOption()
		if option == CleanOptionCancel {
			a.picker.CancelCleanConfirmation()
			return a, nil
		}

		prdName := cc.EntryName
		branch := cc.Branch
		clearBranch := option == CleanOptionRemoveAll
		baseDir := a.baseDir
		worktreePath := git.WorktreePathForPRD(baseDir, prdName)

		return a, func() tea.Msg {
			// Remove the worktree
			if err := git.RemoveWorktree(baseDir, worktreePath); err != nil {
				return cleanResultMsg{
					prdName: prdName,
					success: false,
					message: fmt.Sprintf("Failed to remove worktree: %s", err.Error()),
				}
			}

			// Delete branch if requested
			if clearBranch && branch != "" {
				if err := git.DeleteBranch(baseDir, branch); err != nil {
					return cleanResultMsg{
						prdName:     prdName,
						success:     true,
						message:     fmt.Sprintf("Removed worktree but failed to delete branch: %s", err.Error()),
						clearBranch: false,
					}
				}
			}

			msg := fmt.Sprintf("Removed worktree for %s", prdName)
			if clearBranch && branch != "" {
				msg = fmt.Sprintf("Removed worktree and deleted branch %s", branch)
			}
			return cleanResultMsg{
				prdName:     prdName,
				success:     true,
				message:     msg,
				clearBranch: clearBranch,
			}
		}
	}

	return a, nil
}

// handleCleanResult handles the result of an async clean operation.
func (a App) handleCleanResult(msg cleanResultMsg) (tea.Model, tea.Cmd) {
	a.picker.CancelCleanConfirmation()
	a.picker.SetCleanResult(&CleanResult{
		Success: msg.success,
		Message: msg.message,
	})

	if msg.success {
		// Clear worktree info from manager
		if a.manager != nil {
			a.manager.ClearWorktreeInfo(msg.prdName, msg.clearBranch)
		}
		a.picker.Refresh()
		a.lastActivity = fmt.Sprintf("Cleaned worktree for %s", msg.prdName)
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

	// Dismiss clean result on any key
	if a.picker.HasCleanResult() {
		a.picker.ClearCleanResult()
		a.picker.Refresh()
		return a, nil
	}

	// Handle clean confirmation dialog
	if a.picker.HasCleanConfirmation() {
		return a.handleCleanConfirmationKeys(msg)
	}

	// Dismiss merge result on any key
	if a.picker.HasMergeResult() {
		a.picker.ClearMergeResult()
		a.picker.Refresh()
		return a, nil
	}

	// Normal picker mode
	switch msg.String() {
	case "esc", "l":
		a.viewMode = ViewDashboard
		return a, nil
	case "q", "ctrl+c":
		return a.tryQuit()
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

	case "m":
		// Merge completed PRD's branch
		if a.picker.CanMerge() {
			entry := a.picker.GetSelectedEntry()
			branch := entry.Branch
			baseDir := a.baseDir
			return a, func() tea.Msg {
				conflicts, err := git.MergeBranch(baseDir, branch)
				if err != nil {
					return mergeResultMsg{branch: branch, conflicts: conflicts, err: err}
				}
				// Build success message with merge details
				output := parseMergeSuccessMessage(baseDir, branch)
				return mergeResultMsg{branch: branch, output: output}
			}
		}
		return a, nil

	case "c":
		// Clean worktree for non-running PRD
		if a.picker.CanClean() {
			a.picker.StartCleanConfirmation()
		}
		return a, nil
	}

	return a, nil
}

// parseMergeSuccessMessage constructs a success message after a merge.
func parseMergeSuccessMessage(repoDir, branch string) string {
	// Try to get the default branch for display
	defaultBranch := "current branch"
	if db, err := git.GetDefaultBranch(repoDir); err == nil {
		defaultBranch = db
	}
	return fmt.Sprintf("Merged %s into %s", branch, defaultBranch)
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

	// Create new progress watcher and load initial progress
	newProgressWatcher, err := prd.NewProgressWatcher(prdPath)
	if err == nil {
		a.progressWatcher = newProgressWatcher
		_ = a.progressWatcher.Start()
	}
	a.progress, _ = prd.ParseProgress(prd.ProgressPath(prdPath))

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

	// Only recalculate max iterations if no loop is currently running for this PRD
	if instance := a.manager.GetInstance(name); instance == nil || instance.State != loop.LoopStateRunning {
		remaining := 0
		for _, story := range newPRD.UserStories {
			if !story.Passes {
				remaining++
			}
		}
		a.maxIter = remaining + 5
		if a.maxIter < 5 {
			a.maxIter = 5
		}
	}

	// Update app state
	a.prd = newPRD
	a.prdPath = prdPath
	a.prdName = name
	a.selectedIndex = 0
	a.storiesScrollOffset = 0
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

	// Clear log viewer and story timing (each PRD has its own log/timing)
	a.logViewer.Clear()
	a.storyTimings = nil
	a.currentStoryID = ""
	a.currentStoryStart = time.Time{}

	// Return with new watcher listeners (and elapsed tick if running)
	cmds := []tea.Cmd{a.listenForPRDChanges(), a.listenForProgressChanges()}
	if appState == StateRunning {
		cmds = append(cmds, tickElapsed())
	}
	return a, tea.Batch(cmds...)
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

// storiesListHeight calculates how many story lines fit in the panel.
// Must match the calculation in renderStoriesPanel.
func (a *App) storiesListHeight() int {
	fh := footerHeight
	if a.height < 12 {
		fh = 0
	}
	contentHeight := a.height - a.effectiveHeaderHeight() - fh - 2
	if a.isNarrowMode() {
		storiesHeight := max((contentHeight*40)/100, 5)
		return storiesHeight - 5
	}
	return contentHeight - 5
}

// adjustStoriesScroll ensures the selected index is visible in the scroll window.
func (a *App) adjustStoriesScroll() {
	listHeight := a.storiesListHeight()
	if listHeight <= 0 {
		return
	}
	if a.selectedIndex < a.storiesScrollOffset {
		a.storiesScrollOffset = a.selectedIndex
	}
	if a.selectedIndex >= a.storiesScrollOffset+listHeight {
		a.storiesScrollOffset = a.selectedIndex - listHeight + 1
	}
	// Clamp
	maxOffset := len(a.prd.UserStories) - listHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if a.storiesScrollOffset > maxOffset {
		a.storiesScrollOffset = maxOffset
	}
	if a.storiesScrollOffset < 0 {
		a.storiesScrollOffset = 0
	}
}

// markStoryInProgress clears any existing in-progress flags and marks the
// given story as in-progress, then saves the PRD to disk.
func (a *App) markStoryInProgress(storyID string) {
	for i := range a.prd.UserStories {
		a.prd.UserStories[i].InProgress = a.prd.UserStories[i].ID == storyID
	}
	_ = a.prd.Save(a.prdPath)
}

// clearInProgress clears all in-progress flags and saves the PRD to disk.
func (a *App) clearInProgress() {
	dirty := false
	for i := range a.prd.UserStories {
		if a.prd.UserStories[i].InProgress {
			a.prd.UserStories[i].InProgress = false
			dirty = true
		}
	}
	if dirty {
		_ = a.prd.Save(a.prdPath)
	}
}

// selectStoryByID sets the selected index to the story with the given ID.
func (a *App) selectStoryByID(storyID string) {
	for i, story := range a.prd.UserStories {
		if story.ID == storyID {
			a.selectedIndex = i
			a.adjustStoriesScroll()
			return
		}
	}
}

// selectInProgressStory sets the selected index to the first in-progress story.
func (a *App) selectInProgressStory() {
	for i, story := range a.prd.UserStories {
		if story.InProgress {
			a.selectedIndex = i
			a.adjustStoriesScroll()
			return
		}
	}
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

// listenForProgressChanges listens for progress.md file changes and returns them as messages.
func (a *App) listenForProgressChanges() tea.Cmd {
	if a.progressWatcher == nil {
		return nil
	}
	return func() tea.Msg {
		entries, ok := <-a.progressWatcher.Events()
		if !ok {
			return nil
		}
		return ProgressUpdateMsg{Entries: entries}
	}
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

		// Auto-select the in-progress story so the user sees its details
		a.selectInProgressStory()
		a.adjustStoriesScroll()
	}

	// Continue listening for changes
	return a, a.listenForPRDChanges()
}

// stopWatcher stops the file watchers.
func (a *App) stopWatcher() {
	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.progressWatcher != nil {
		a.progressWatcher.Stop()
	}
}
