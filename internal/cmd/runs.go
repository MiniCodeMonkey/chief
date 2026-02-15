package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/minicodemonkey/chief/internal/engine"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/ws"
)

// runManager manages Ralph loop runs driven by WebSocket commands.
type runManager struct {
	mu     sync.RWMutex
	eng    *engine.Engine
	client *ws.Client
	// tracks which engine registration key maps to which project/prd
	runs map[string]*runInfo
}

// runInfo tracks metadata about a registered run.
type runInfo struct {
	project string
	prdID   string
	prdPath string // absolute path to prd.json
}

// runKey returns the engine registration key for a project/PRD combination.
func runKey(project, prdID string) string {
	return project + "/" + prdID
}

// newRunManager creates a new run manager.
func newRunManager(eng *engine.Engine, client *ws.Client) *runManager {
	return &runManager{
		eng:    eng,
		client: client,
		runs:   make(map[string]*runInfo),
	}
}

// startEventMonitor subscribes to engine events and handles quota exhaustion.
// It runs until the context is cancelled.
func (rm *runManager) startEventMonitor(ctx context.Context) {
	eventCh, unsub := rm.eng.Subscribe()
	go func() {
		defer unsub()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				if event.Event.Type == loop.EventQuotaExhausted {
					rm.handleQuotaExhausted(event.PRDName)
				}
			}
		}
	}()
}

// handleQuotaExhausted handles a quota exhaustion event for a specific run.
func (rm *runManager) handleQuotaExhausted(prdName string) {
	rm.mu.RLock()
	info, exists := rm.runs[prdName]
	rm.mu.RUnlock()

	if !exists {
		log.Printf("Quota exhausted for unknown run key: %s", prdName)
		return
	}

	log.Printf("Quota exhausted for %s/%s, auto-pausing", info.project, info.prdID)

	if rm.client == nil {
		return
	}

	// Send run_paused with reason quota_exhausted
	envelope := ws.NewMessage(ws.TypeRunPaused)
	pausedMsg := ws.RunPausedMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Project:   info.project,
		PRDID:     info.prdID,
		Reason:    "quota_exhausted",
	}
	if err := rm.client.Send(pausedMsg); err != nil {
		log.Printf("Error sending run_paused: %v", err)
	}

	// Send quota_exhausted message listing affected runs
	rm.sendQuotaExhausted(info.project, info.prdID)
}

// sendQuotaExhausted sends a quota_exhausted message over WebSocket.
func (rm *runManager) sendQuotaExhausted(project, prdID string) {
	if rm.client == nil {
		return
	}
	envelope := ws.NewMessage(ws.TypeQuotaExhausted)
	msg := ws.QuotaExhaustedMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Runs:      []string{runKey(project, prdID)},
		Sessions:  []string{},
	}
	if err := rm.client.Send(msg); err != nil {
		log.Printf("Error sending quota_exhausted: %v", err)
	}
}

// activeRuns returns the list of active runs for state snapshots.
func (rm *runManager) activeRuns() []ws.RunState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var runs []ws.RunState
	for key := range rm.runs {
		info := rm.runs[key]
		instance := rm.eng.GetInstance(key)
		if instance == nil {
			continue
		}

		status := loopStateToString(instance.State)
		runs = append(runs, ws.RunState{
			Project:   info.project,
			PRDID:     info.prdID,
			Status:    status,
			Iteration: instance.Iteration,
		})
	}
	return runs
}

// startRun starts a Ralph loop for a project/PRD.
func (rm *runManager) startRun(project, prdID, projectPath string) error {
	key := runKey(project, prdID)

	// Check if already running
	if instance := rm.eng.GetInstance(key); instance != nil {
		if instance.State == loop.LoopStateRunning {
			return fmt.Errorf("RUN_ALREADY_ACTIVE")
		}
	}

	prdPath := filepath.Join(projectPath, ".chief", "prds", prdID, "prd.json")

	// Register if not already registered
	if instance := rm.eng.GetInstance(key); instance == nil {
		if err := rm.eng.Register(key, prdPath); err != nil {
			return fmt.Errorf("failed to register PRD: %w", err)
		}
	}

	rm.mu.Lock()
	rm.runs[key] = &runInfo{
		project: project,
		prdID:   prdID,
		prdPath: prdPath,
	}
	rm.mu.Unlock()

	if err := rm.eng.Start(key); err != nil {
		return fmt.Errorf("failed to start run: %w", err)
	}

	return nil
}

// pauseRun pauses a running loop.
func (rm *runManager) pauseRun(project, prdID string) error {
	key := runKey(project, prdID)

	instance := rm.eng.GetInstance(key)
	if instance == nil || instance.State != loop.LoopStateRunning {
		return fmt.Errorf("RUN_NOT_ACTIVE")
	}

	if err := rm.eng.Pause(key); err != nil {
		return fmt.Errorf("failed to pause run: %w", err)
	}

	return nil
}

// resumeRun resumes a paused loop by starting it again.
func (rm *runManager) resumeRun(project, prdID string) error {
	key := runKey(project, prdID)

	instance := rm.eng.GetInstance(key)
	if instance == nil || instance.State != loop.LoopStatePaused {
		return fmt.Errorf("RUN_NOT_ACTIVE")
	}

	// Start creates a fresh Loop that picks up from the next unfinished story
	if err := rm.eng.Start(key); err != nil {
		return fmt.Errorf("failed to resume run: %w", err)
	}

	return nil
}

// stopRun stops a running or paused loop immediately.
func (rm *runManager) stopRun(project, prdID string) error {
	key := runKey(project, prdID)

	instance := rm.eng.GetInstance(key)
	if instance == nil || (instance.State != loop.LoopStateRunning && instance.State != loop.LoopStatePaused) {
		return fmt.Errorf("RUN_NOT_ACTIVE")
	}

	if err := rm.eng.Stop(key); err != nil {
		return fmt.Errorf("failed to stop run: %w", err)
	}

	return nil
}

// cleanup removes tracking for a completed/stopped run.
func (rm *runManager) cleanup(key string) {
	rm.mu.Lock()
	delete(rm.runs, key)
	rm.mu.Unlock()
}

// stopAll stops all active runs (for shutdown).
func (rm *runManager) stopAll() {
	rm.eng.StopAll()
}

// loopStateToString converts a LoopState to a string for WebSocket messages.
func loopStateToString(state loop.LoopState) string {
	switch state {
	case loop.LoopStateReady:
		return "ready"
	case loop.LoopStateRunning:
		return "running"
	case loop.LoopStatePaused:
		return "paused"
	case loop.LoopStateStopped:
		return "stopped"
	case loop.LoopStateComplete:
		return "complete"
	case loop.LoopStateError:
		return "error"
	default:
		return "unknown"
	}
}

// handleStartRun handles a start_run WebSocket message.
func handleStartRun(client *ws.Client, scanner projectFinder, runs *runManager, watcher activator, msg ws.Message) {
	var req ws.StartRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing start_run message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(client, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	if err := runs.startRun(req.Project, req.PRDID, project.Path); err != nil {
		if err.Error() == "RUN_ALREADY_ACTIVE" {
			sendError(client, ws.ErrCodeRunAlreadyActive,
				fmt.Sprintf("Run already active for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(client, ws.ErrCodeClaudeError,
				fmt.Sprintf("Failed to start run: %v", err), msg.ID)
		}
		return
	}

	// Activate file watching for the project
	if watcher != nil {
		watcher.Activate(req.Project)
	}

	log.Printf("Started run for %s/%s", req.Project, req.PRDID)
}

// handlePauseRun handles a pause_run WebSocket message.
func handlePauseRun(client *ws.Client, runs *runManager, msg ws.Message) {
	var req ws.PauseRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing pause_run message: %v", err)
		return
	}

	if err := runs.pauseRun(req.Project, req.PRDID); err != nil {
		if err.Error() == "RUN_NOT_ACTIVE" {
			sendError(client, ws.ErrCodeRunNotActive,
				fmt.Sprintf("No active run for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(client, ws.ErrCodeClaudeError,
				fmt.Sprintf("Failed to pause run: %v", err), msg.ID)
		}
		return
	}

	log.Printf("Paused run for %s/%s", req.Project, req.PRDID)
}

// handleResumeRun handles a resume_run WebSocket message.
func handleResumeRun(client *ws.Client, runs *runManager, msg ws.Message) {
	var req ws.ResumeRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing resume_run message: %v", err)
		return
	}

	if err := runs.resumeRun(req.Project, req.PRDID); err != nil {
		if err.Error() == "RUN_NOT_ACTIVE" {
			sendError(client, ws.ErrCodeRunNotActive,
				fmt.Sprintf("No paused run for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(client, ws.ErrCodeClaudeError,
				fmt.Sprintf("Failed to resume run: %v", err), msg.ID)
		}
		return
	}

	log.Printf("Resumed run for %s/%s", req.Project, req.PRDID)
}

// handleStopRun handles a stop_run WebSocket message.
func handleStopRun(client *ws.Client, runs *runManager, msg ws.Message) {
	var req ws.StopRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing stop_run message: %v", err)
		return
	}

	if err := runs.stopRun(req.Project, req.PRDID); err != nil {
		if err.Error() == "RUN_NOT_ACTIVE" {
			sendError(client, ws.ErrCodeRunNotActive,
				fmt.Sprintf("No active run for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(client, ws.ErrCodeClaudeError,
				fmt.Sprintf("Failed to stop run: %v", err), msg.ID)
		}
		return
	}

	log.Printf("Stopped run for %s/%s", req.Project, req.PRDID)
}

// activator is an interface for activating file watching (for testability).
type activator interface {
	Activate(name string)
}
