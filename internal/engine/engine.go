// Package engine provides a shared orchestration layer on top of loop.Manager
// that both the TUI and the serve command (WebSocket handler) can consume.
// It supports multiple concurrent event consumers via fan-out subscription.
package engine

import (
	"sync"

	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
)

// Engine wraps loop.Manager to provide a shared interface for driving Ralph loops
// and Claude sessions. It fans out events to multiple consumers.
type Engine struct {
	manager *loop.Manager

	// Fan-out event distribution
	subscribers map[int]chan ManagerEvent
	nextID      int
	subMu       sync.RWMutex

	// Forwarding goroutine lifecycle
	stopForward chan struct{}
	forwarding  bool
	forwardMu   sync.Mutex
}

// ManagerEvent wraps a loop.ManagerEvent for engine consumers.
// It mirrors loop.ManagerEvent to avoid exposing the loop package directly.
type ManagerEvent = loop.ManagerEvent

// New creates a new Engine with the given max iterations.
func New(maxIter int) *Engine {
	e := &Engine{
		manager:     loop.NewManager(maxIter),
		subscribers: make(map[int]chan ManagerEvent),
		stopForward: make(chan struct{}),
	}
	e.startForwarding()
	return e
}

// startForwarding starts the goroutine that reads from the manager's event
// channel and fans out to all subscribers.
func (e *Engine) startForwarding() {
	e.forwardMu.Lock()
	defer e.forwardMu.Unlock()

	if e.forwarding {
		return
	}
	e.forwarding = true

	go func() {
		for {
			select {
			case event, ok := <-e.manager.Events():
				if !ok {
					return
				}
				e.subMu.RLock()
				for _, ch := range e.subscribers {
					// Non-blocking send: drop events for slow consumers
					select {
					case ch <- event:
					default:
					}
				}
				e.subMu.RUnlock()

			case <-e.stopForward:
				return
			}
		}
	}()
}

// Subscribe creates a new event subscription and returns a channel and an
// unsubscribe function. The channel is buffered (100 events). The caller must
// call the returned function when done to avoid resource leaks.
func (e *Engine) Subscribe() (<-chan ManagerEvent, func()) {
	ch := make(chan ManagerEvent, 100)

	e.subMu.Lock()
	id := e.nextID
	e.nextID++
	e.subscribers[id] = ch
	e.subMu.Unlock()

	unsub := func() {
		e.subMu.Lock()
		delete(e.subscribers, id)
		e.subMu.Unlock()
	}

	return ch, unsub
}

// Manager returns the underlying loop.Manager for direct access when needed.
// This is useful for operations like Register, UpdateWorktreeInfo, etc.
// that don't need to be abstracted by the engine.
func (e *Engine) Manager() *loop.Manager {
	return e.manager
}

// --- Delegated Manager methods ---

// Register registers a PRD with the engine (does not start it).
func (e *Engine) Register(name, prdPath string) error {
	return e.manager.Register(name, prdPath)
}

// RegisterWithWorktree registers a PRD with worktree metadata.
func (e *Engine) RegisterWithWorktree(name, prdPath, worktreeDir, branch string) error {
	return e.manager.RegisterWithWorktree(name, prdPath, worktreeDir, branch)
}

// Unregister removes a PRD from the engine.
func (e *Engine) Unregister(name string) error {
	return e.manager.Unregister(name)
}

// Start starts the loop for a specific PRD.
func (e *Engine) Start(name string) error {
	return e.manager.Start(name)
}

// Pause pauses the loop for a specific PRD.
func (e *Engine) Pause(name string) error {
	return e.manager.Pause(name)
}

// Stop stops the loop for a specific PRD immediately.
func (e *Engine) Stop(name string) error {
	return e.manager.Stop(name)
}

// StopAll stops all running loops and waits for completion.
func (e *Engine) StopAll() {
	e.manager.StopAll()
}

// GetState returns the state of a specific PRD loop.
func (e *Engine) GetState(name string) (loop.LoopState, int, error) {
	return e.manager.GetState(name)
}

// GetInstance returns a copy of the loop instance for a specific PRD.
func (e *Engine) GetInstance(name string) *loop.LoopInstance {
	return e.manager.GetInstance(name)
}

// GetAllInstances returns a snapshot of all loop instances.
func (e *Engine) GetAllInstances() []*loop.LoopInstance {
	return e.manager.GetAllInstances()
}

// GetRunningPRDs returns the names of all currently running PRDs.
func (e *Engine) GetRunningPRDs() []string {
	return e.manager.GetRunningPRDs()
}

// GetRunningCount returns the number of currently running loops.
func (e *Engine) GetRunningCount() int {
	return e.manager.GetRunningCount()
}

// IsAnyRunning returns true if any loop is currently running.
func (e *Engine) IsAnyRunning() bool {
	return e.manager.IsAnyRunning()
}

// SetMaxIterations updates the default max iterations for new loops.
func (e *Engine) SetMaxIterations(maxIter int) {
	e.manager.SetMaxIterations(maxIter)
}

// MaxIterations returns the current default max iterations.
func (e *Engine) MaxIterations() int {
	return e.manager.MaxIterations()
}

// SetMaxIterationsForInstance updates max iterations for a running loop.
func (e *Engine) SetMaxIterationsForInstance(name string, maxIter int) error {
	return e.manager.SetMaxIterationsForInstance(name, maxIter)
}

// SetRetryConfig sets the retry configuration for new loops.
func (e *Engine) SetRetryConfig(cfg loop.RetryConfig) {
	e.manager.SetRetryConfig(cfg)
}

// DisableRetry disables automatic retry for new loops.
func (e *Engine) DisableRetry() {
	e.manager.DisableRetry()
}

// SetCompletionCallback sets a callback for when any PRD completes.
func (e *Engine) SetCompletionCallback(fn func(prdName string)) {
	e.manager.SetCompletionCallback(fn)
}

// SetPostCompleteCallback sets a callback for post-completion actions.
func (e *Engine) SetPostCompleteCallback(fn func(prdName, branch, workDir string)) {
	e.manager.SetPostCompleteCallback(fn)
}

// SetConfig sets the project config.
func (e *Engine) SetConfig(cfg *config.Config) {
	e.manager.SetConfig(cfg)
}

// Config returns the current project config.
func (e *Engine) Config() *config.Config {
	return e.manager.Config()
}

// UpdateWorktreeInfo updates the worktree directory and branch for a PRD.
func (e *Engine) UpdateWorktreeInfo(name, worktreeDir, branch string) error {
	return e.manager.UpdateWorktreeInfo(name, worktreeDir, branch)
}

// ClearWorktreeInfo clears the worktree directory and optionally branch.
func (e *Engine) ClearWorktreeInfo(name string, clearBranch bool) error {
	return e.manager.ClearWorktreeInfo(name, clearBranch)
}

// --- Project state queries ---

// LoadPRD loads and returns a PRD from the given path.
func (e *Engine) LoadPRD(prdPath string) (*prd.PRD, error) {
	return prd.LoadPRD(prdPath)
}

// Shutdown stops all loops and the event forwarding goroutine.
func (e *Engine) Shutdown() {
	e.manager.StopAll()

	e.forwardMu.Lock()
	defer e.forwardMu.Unlock()

	if e.forwarding {
		close(e.stopForward)
		e.forwarding = false
	}
}
