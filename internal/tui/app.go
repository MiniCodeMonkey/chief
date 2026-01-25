package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	Event loop.Event
}

// LoopFinishedMsg is sent when the loop finishes (complete, paused, stopped, or error).
type LoopFinishedMsg struct {
	Err error
}

// ViewMode represents which view is currently active.
type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewLog
	ViewPicker
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

	// Loop control
	loop       *loop.Loop
	loopCtx    context.Context
	loopCancel context.CancelFunc
	maxIter    int

	// Activity tracking
	lastActivity string

	// File watching
	watcher *prd.Watcher

	// View mode
	viewMode  ViewMode
	logViewer *LogViewer

	// PRD picker
	picker  *PRDPicker
	baseDir string // Base directory for .chief/prds/
}

// NewApp creates a new App with the given PRD.
func NewApp(prdPath string) (*App, error) {
	return NewAppWithOptions(prdPath, 10) // default max iterations
}

// NewAppWithOptions creates a new App with the given PRD and options.
func NewAppWithOptions(prdPath string, maxIter int) (*App, error) {
	p, err := prd.LoadPRD(prdPath)
	if err != nil {
		return nil, err
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
	// If path contains .chief/prds/, go up to the parent
	baseDir := filepath.Dir(filepath.Dir(filepath.Dir(prdPath)))
	if !strings.Contains(prdPath, ".chief/prds/") {
		// Fallback to current working directory
		baseDir, _ = os.Getwd()
	}

	return &App{
		prd:           p,
		prdPath:       prdPath,
		prdName:       prdName,
		state:         StateReady,
		iteration:     0,
		selectedIndex: 0,
		maxIter:       maxIter,
		watcher:       watcher,
		viewMode:      ViewDashboard,
		logViewer:     NewLogViewer(),
		picker:        NewPRDPicker(baseDir, prdName),
		baseDir:       baseDir,
	}, nil
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
	)
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
		return a.handleLoopEvent(msg.Event)

	case LoopFinishedMsg:
		return a.handleLoopFinished(msg.Err)

	case PRDUpdateMsg:
		return a.handlePRDUpdate(msg)

	case tea.KeyMsg:
		// Handle picker view separately (it has its own input mode)
		if a.viewMode == ViewPicker {
			return a.handlePickerKeys(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			a.stopLoop()
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

		// PRD picker
		case "l":
			if a.viewMode == ViewDashboard || a.viewMode == ViewLog {
				a.picker.Refresh()
				a.picker.SetSize(a.width, a.height)
				a.viewMode = ViewPicker
			}
			return a, nil

		// Loop controls (work in both views)
		case "s":
			if a.state == StateReady || a.state == StatePaused {
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
		}
	}

	return a, nil
}

// startLoop starts the agent loop.
func (a App) startLoop() (tea.Model, tea.Cmd) {
	a.state = StateRunning
	a.startTime = time.Now()
	a.lastActivity = "Starting loop..."

	// Create a new loop instance
	a.loop = loop.NewLoopWithEmbeddedPrompt(a.prdPath, a.maxIter)
	a.loopCtx, a.loopCancel = context.WithCancel(context.Background())

	// Start the loop in a goroutine and listen for events
	return a, tea.Batch(
		a.runLoop(),
		a.listenForEvents(),
	)
}

// runLoop runs the loop in a goroutine and returns a command that signals completion.
func (a *App) runLoop() tea.Cmd {
	return func() tea.Msg {
		err := a.loop.Run(a.loopCtx)
		return LoopFinishedMsg{Err: err}
	}
}

// listenForEvents listens for loop events and returns them as messages.
func (a *App) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-a.loop.Events()
		if !ok {
			return nil
		}
		return LoopEventMsg{Event: event}
	}
}

// pauseLoop sets the pause flag so the loop stops after the current iteration.
func (a App) pauseLoop() (tea.Model, tea.Cmd) {
	if a.loop != nil {
		a.loop.Pause()
	}
	a.lastActivity = "Pausing after current iteration..."
	return a, nil
}

// stopLoop stops the loop immediately.
func (a *App) stopLoop() {
	if a.loop != nil {
		a.loop.Stop()
	}
	if a.loopCancel != nil {
		a.loopCancel()
	}
}

// stopLoopAndUpdate stops the loop and updates the state.
func (a App) stopLoopAndUpdate() (tea.Model, tea.Cmd) {
	a.stopLoop()
	a.state = StateStopped
	a.lastActivity = "Stopped"
	return a, nil
}

// handleLoopEvent handles events from the loop.
func (a App) handleLoopEvent(event loop.Event) (tea.Model, tea.Cmd) {
	a.iteration = event.Iteration

	// Add event to log viewer
	a.logViewer.AddEvent(event)

	switch event.Type {
	case loop.EventIterationStart:
		a.lastActivity = "Starting iteration..."
	case loop.EventAssistantText:
		// Truncate long text for activity display
		text := event.Text
		if len(text) > 100 {
			text = text[:97] + "..."
		}
		a.lastActivity = text
	case loop.EventToolStart:
		a.lastActivity = "Running tool: " + event.Tool
	case loop.EventToolResult:
		a.lastActivity = "Tool completed"
	case loop.EventStoryStarted:
		a.lastActivity = "Working on: " + event.StoryID
	case loop.EventComplete:
		a.state = StateComplete
		a.lastActivity = "All stories complete!"
	case loop.EventMaxIterationsReached:
		a.state = StatePaused
		a.lastActivity = "Max iterations reached"
	case loop.EventError:
		a.state = StateError
		a.err = event.Err
		if event.Err != nil {
			a.lastActivity = "Error: " + event.Err.Error()
		}
	}

	// Reload PRD to reflect any changes made by Claude
	if p, err := prd.LoadPRD(a.prdPath); err == nil {
		a.prd = p
	}

	// Continue listening for events if running
	if a.state == StateRunning {
		return a, a.listenForEvents()
	}

	return a, nil
}

// handleLoopFinished handles when the loop finishes.
func (a App) handleLoopFinished(err error) (tea.Model, tea.Cmd) {
	if err != nil && a.state != StateStopped {
		a.state = StateError
		a.err = err
		a.lastActivity = "Error: " + err.Error()
	} else if a.loop != nil && a.loop.IsPaused() {
		a.state = StatePaused
		a.lastActivity = "Paused"
	}

	// Reload PRD to reflect any changes
	if p, err := prd.LoadPRD(a.prdPath); err == nil {
		a.prd = p
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
	default:
		return a.renderDashboard()
	}
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
				// Create the new PRD directory structure
				newPRDDir := filepath.Join(a.baseDir, ".chief", "prds", name)
				newPRDPath := filepath.Join(newPRDDir, "prd.json")

				// Create directory if it doesn't exist
				if err := os.MkdirAll(newPRDDir, 0755); err == nil {
					// Create a minimal prd.json
					newPRD := &prd.PRD{
						Project:     name,
						Description: "New PRD - edit this description",
						UserStories: []prd.UserStory{},
					}
					if err := newPRD.Save(newPRDPath); err == nil {
						// Switch to the new PRD
						return a.switchToPRD(name, newPRDPath)
					}
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
		a.stopLoop()
		a.stopWatcher()
		return a, tea.Quit
	case "up", "k":
		a.picker.MoveUp()
		return a, nil
	case "down", "j":
		a.picker.MoveDown()
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
	}

	return a, nil
}

// switchToPRD switches to a different PRD.
func (a App) switchToPRD(name, prdPath string) (tea.Model, tea.Cmd) {
	// Stop current loop if running
	a.stopLoop()

	// Stop current watcher
	a.stopWatcher()

	// Load the new PRD
	newPRD, err := prd.LoadPRD(prdPath)
	if err != nil {
		a.lastActivity = "Error loading PRD: " + err.Error()
		a.viewMode = ViewDashboard
		return a, nil
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

	// Update app state
	a.prd = newPRD
	a.prdPath = prdPath
	a.prdName = name
	a.selectedIndex = 0
	a.state = StateReady
	a.iteration = 0
	a.startTime = time.Time{}
	a.lastActivity = "Switched to PRD: " + name
	a.viewMode = ViewDashboard
	a.picker.SetCurrentPRD(name)

	// Clear log viewer
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
