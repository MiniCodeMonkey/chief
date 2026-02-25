package loop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/minicodemonkey/chief/embed"
	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/prd"
)

// LoopState represents the state of a loop instance.
type LoopState int

const (
	LoopStateReady LoopState = iota
	LoopStateRunning
	LoopStatePaused
	LoopStateStopped
	LoopStateComplete
	LoopStateError
)

func (s LoopState) String() string {
	switch s {
	case LoopStateReady:
		return "Ready"
	case LoopStateRunning:
		return "Running"
	case LoopStatePaused:
		return "Paused"
	case LoopStateStopped:
		return "Stopped"
	case LoopStateComplete:
		return "Complete"
	case LoopStateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// LoopInstance represents a single loop with its metadata.
type LoopInstance struct {
	Name        string
	PRDPath     string
	WorktreeDir string // Working directory for this PRD (empty = project root)
	Branch      string // Git branch for this PRD (empty = current branch)
	Loop        *Loop
	State       LoopState
	Iteration   int
	StartTime   time.Time
	Error       error
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
}

// ManagerEvent represents an event from any managed loop.
type ManagerEvent struct {
	PRDName   string
	Event     Event
	Completed bool // True if this PRD just completed all stories
}

// Manager manages multiple Loop instances for parallel PRD execution.
type Manager struct {
	instances     map[string]*LoopInstance
	events        chan ManagerEvent
	maxIter       int
	retryConfig  RetryConfig
	provider      Provider
	baseDir       string                               // Project root directory (for CLAUDE.md etc.)
	config        *config.Config                      // Project config for post-completion actions
	mu            sync.RWMutex
	wg            sync.WaitGroup
	onComplete    func(prdName string)                 // Callback when a PRD completes
	onPostComplete func(prdName, branch, workDir string) // Callback for post-completion actions (push, PR)
}

// NewManager creates a new loop manager.
func NewManager(maxIter int, provider Provider) *Manager {
	return &Manager{
		instances:   make(map[string]*LoopInstance),
		events:      make(chan ManagerEvent, 100),
		maxIter:     maxIter,
		retryConfig: DefaultRetryConfig(),
		provider:    provider,
	}
}

// SetRetryConfig sets the retry configuration for new loops.
func (m *Manager) SetRetryConfig(config RetryConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryConfig = config
}

// DisableRetry disables automatic retry for new loops.
func (m *Manager) DisableRetry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryConfig.Enabled = false
}

// SetCompletionCallback sets a callback that is called when any PRD completes.
func (m *Manager) SetCompletionCallback(fn func(prdName string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onComplete = fn
}

// SetPostCompleteCallback sets a callback for post-completion actions (push, PR creation).
// The callback receives the PRD name, branch name, and working directory.
func (m *Manager) SetPostCompleteCallback(fn func(prdName, branch, workDir string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPostComplete = fn
}

// SetBaseDir sets the project root directory so Claude runs from there and picks up CLAUDE.md.
func (m *Manager) SetBaseDir(baseDir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseDir = baseDir
}

// SetConfig sets the project config for post-completion actions.
func (m *Manager) SetConfig(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// Config returns the current project config.
func (m *Manager) Config() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Events returns the channel for receiving events from all loops.
func (m *Manager) Events() <-chan ManagerEvent {
	return m.events
}

// Register registers a PRD with the manager (does not start it).
func (m *Manager) Register(name, prdPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already registered
	if _, exists := m.instances[name]; exists {
		return fmt.Errorf("PRD %s is already registered", name)
	}

	m.instances[name] = &LoopInstance{
		Name:    name,
		PRDPath: prdPath,
		State:   LoopStateReady,
	}

	return nil
}

// RegisterWithWorktree registers a PRD with worktree metadata (does not start it).
func (m *Manager) RegisterWithWorktree(name, prdPath, worktreeDir, branch string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already registered
	if _, exists := m.instances[name]; exists {
		return fmt.Errorf("PRD %s is already registered", name)
	}

	m.instances[name] = &LoopInstance{
		Name:        name,
		PRDPath:     prdPath,
		WorktreeDir: worktreeDir,
		Branch:      branch,
		State:       LoopStateReady,
	}

	return nil
}

// Unregister removes a PRD from the manager (stops it first if running).
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	instance, exists := m.instances[name]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("PRD %s not found", name)
	}

	// Stop if running
	if instance.State == LoopStateRunning {
		m.Stop(name)
	}

	m.mu.Lock()
	delete(m.instances, name)
	m.mu.Unlock()

	return nil
}

// Start starts the loop for a specific PRD.
func (m *Manager) Start(name string) error {
	m.mu.Lock()
	instance, exists := m.instances[name]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("PRD %s not found", name)
	}

	instance.mu.Lock()
	if instance.State == LoopStateRunning {
		instance.mu.Unlock()
		return fmt.Errorf("PRD %s is already running", name)
	}

	// Create a new loop instance, using worktree-aware constructor if WorktreeDir is set.
	// When no worktree is configured, run from the project root (baseDir) so that
	// CLAUDE.md and other project-level files are visible to Claude.
	prompt := embed.GetPrompt(instance.PRDPath, prd.ProgressPath(instance.PRDPath))
	workDir := instance.WorktreeDir
	if workDir == "" {
		m.mu.RLock()
		workDir = m.baseDir
		m.mu.RUnlock()
	}
	instance.Loop = NewLoopWithWorkDir(instance.PRDPath, workDir, prompt, m.maxIter, m.provider)
	m.mu.RLock()
	instance.Loop.SetRetryConfig(m.retryConfig)
	m.mu.RUnlock()
	instance.ctx, instance.cancel = context.WithCancel(context.Background())
	instance.State = LoopStateRunning
	instance.StartTime = time.Now()
	instance.Error = nil
	instance.mu.Unlock()

	// Start the loop in a goroutine
	m.wg.Add(1)
	go m.runLoop(instance)

	return nil
}

// runLoop runs a loop instance and forwards events.
func (m *Manager) runLoop(instance *LoopInstance) {
	defer m.wg.Done()

	// Start event forwarding goroutine
	done := make(chan struct{})
	go func() {
		for {
			select {
			case event, ok := <-instance.Loop.Events():
				if !ok {
					close(done)
					return
				}

				instance.mu.Lock()
				instance.Iteration = event.Iteration
				instance.mu.Unlock()

				// Check if this is a completion event
				completed := event.Type == EventComplete

				// Forward event to manager channel
				m.events <- ManagerEvent{
					PRDName:   instance.Name,
					Event:     event,
					Completed: completed,
				}

				// If completed, trigger callbacks
				if completed {
					m.mu.RLock()
					callback := m.onComplete
					postCallback := m.onPostComplete
					m.mu.RUnlock()
					if callback != nil {
						callback(instance.Name)
					}
					if postCallback != nil {
						instance.mu.Lock()
						branch := instance.Branch
						workDir := instance.WorktreeDir
						instance.mu.Unlock()
						postCallback(instance.Name, branch, workDir)
					}
				}
			case <-instance.ctx.Done():
				close(done)
				return
			}
		}
	}()

	// Run the loop
	err := instance.Loop.Run(instance.ctx)

	// Update state based on result
	instance.mu.Lock()
	if err != nil && err != context.Canceled {
		instance.State = LoopStateError
		instance.Error = err
	} else if instance.Loop.IsPaused() {
		instance.State = LoopStatePaused
	} else if instance.Loop.IsStopped() {
		instance.State = LoopStateStopped
	} else {
		// Check if PRD is complete
		p, loadErr := prd.LoadPRD(instance.PRDPath)
		if loadErr == nil && p.AllComplete() {
			instance.State = LoopStateComplete
		} else if instance.State == LoopStateRunning {
			// Loop ended but not explicitly stopped/paused/completed
			instance.State = LoopStatePaused
		}
	}
	instance.mu.Unlock()

	<-done
}

// Pause pauses the loop for a specific PRD (stops after current iteration).
func (m *Manager) Pause(name string) error {
	m.mu.RLock()
	instance, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("PRD %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.State != LoopStateRunning {
		return fmt.Errorf("PRD %s is not running", name)
	}

	if instance.Loop != nil {
		instance.Loop.Pause()
	}

	return nil
}

// Stop stops the loop for a specific PRD immediately.
func (m *Manager) Stop(name string) error {
	m.mu.RLock()
	instance, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("PRD %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.State != LoopStateRunning && instance.State != LoopStatePaused {
		return nil // Already stopped
	}

	if instance.Loop != nil {
		instance.Loop.Stop()
	}
	if instance.cancel != nil {
		instance.cancel()
	}

	instance.State = LoopStateStopped

	return nil
}

// UpdateWorktreeInfo updates the worktree directory and branch for an existing PRD instance.
func (m *Manager) UpdateWorktreeInfo(name, worktreeDir, branch string) error {
	m.mu.RLock()
	instance, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("PRD %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	instance.WorktreeDir = worktreeDir
	instance.Branch = branch

	return nil
}

// ClearWorktreeInfo clears the worktree directory and optionally the branch for a PRD instance.
func (m *Manager) ClearWorktreeInfo(name string, clearBranch bool) error {
	m.mu.RLock()
	instance, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("PRD %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	instance.WorktreeDir = ""
	if clearBranch {
		instance.Branch = ""
	}

	return nil
}

// GetState returns the state of a specific PRD loop.
func (m *Manager) GetState(name string) (LoopState, int, error) {
	m.mu.RLock()
	instance, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return LoopStateReady, 0, fmt.Errorf("PRD %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	return instance.State, instance.Iteration, instance.Error
}

// GetInstance returns a copy of the loop instance data for a specific PRD.
func (m *Manager) GetInstance(name string) *LoopInstance {
	m.mu.RLock()
	instance, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	// Return a copy to avoid race conditions
	return &LoopInstance{
		Name:        instance.Name,
		PRDPath:     instance.PRDPath,
		WorktreeDir: instance.WorktreeDir,
		Branch:      instance.Branch,
		State:       instance.State,
		Iteration:   instance.Iteration,
		StartTime:   instance.StartTime,
		Error:       instance.Error,
	}
}

// GetAllInstances returns a snapshot of all loop instances.
func (m *Manager) GetAllInstances() []*LoopInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LoopInstance, 0, len(m.instances))
	for _, instance := range m.instances {
		instance.mu.Lock()
		copy := &LoopInstance{
			Name:        instance.Name,
			PRDPath:     instance.PRDPath,
			WorktreeDir: instance.WorktreeDir,
			Branch:      instance.Branch,
			State:       instance.State,
			Iteration:   instance.Iteration,
			StartTime:   instance.StartTime,
			Error:       instance.Error,
		}
		instance.mu.Unlock()
		result = append(result, copy)
	}

	return result
}

// GetRunningPRDs returns the names of all currently running PRDs.
func (m *Manager) GetRunningPRDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0)
	for name, instance := range m.instances {
		instance.mu.Lock()
		if instance.State == LoopStateRunning {
			result = append(result, name)
		}
		instance.mu.Unlock()
	}

	return result
}

// GetRunningCount returns the number of currently running loops.
func (m *Manager) GetRunningCount() int {
	return len(m.GetRunningPRDs())
}

// StopAll stops all running loops.
func (m *Manager) StopAll() {
	m.mu.RLock()
	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		m.Stop(name)
	}

	// Wait for all loops to finish
	m.wg.Wait()
}

// IsAnyRunning returns true if any loop is currently running.
func (m *Manager) IsAnyRunning() bool {
	return m.GetRunningCount() > 0
}

// SetMaxIterations updates the default max iterations for new loops.
func (m *Manager) SetMaxIterations(maxIter int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxIter = maxIter
}

// MaxIterations returns the current default max iterations.
func (m *Manager) MaxIterations() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxIter
}

// SetMaxIterationsForInstance updates max iterations for a specific running loop.
func (m *Manager) SetMaxIterationsForInstance(name string, maxIter int) error {
	m.mu.RLock()
	instance, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("PRD %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.Loop != nil {
		instance.Loop.SetMaxIterations(maxIter)
	}

	return nil
}
