package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/minicodemonkey/chief/internal/engine"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
	"github.com/minicodemonkey/chief/internal/ws"
)

// runManager manages Ralph loop runs driven by WebSocket commands.
type runManager struct {
	mu     sync.RWMutex
	eng    *engine.Engine
	sender messageSender
	// tracks which engine registration key maps to which project/prd
	runs    map[string]*runInfo
	loggers map[string]*storyLogger
}

// runInfo tracks metadata about a registered run.
type runInfo struct {
	project   string
	prdID     string
	prdPath   string // absolute path to prd.json
	startTime time.Time
	storyID   string // currently active story ID
}

// runKey returns the engine registration key for a project/PRD combination.
func runKey(project, prdID string) string {
	return project + "/" + prdID
}

// newRunManager creates a new run manager.
func newRunManager(eng *engine.Engine, sender messageSender) *runManager {
	return &runManager{
		eng:     eng,
		sender:  sender,
		runs:    make(map[string]*runInfo),
		loggers: make(map[string]*storyLogger),
	}
}

// startEventMonitor subscribes to engine events and handles progress streaming,
// run completion, Claude output streaming, and quota exhaustion.
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
				rm.handleEvent(event)
			}
		}
	}()
}

// handleEvent routes an engine event to the appropriate handler.
func (rm *runManager) handleEvent(event engine.ManagerEvent) {
	rm.mu.RLock()
	info, exists := rm.runs[event.PRDName]
	rm.mu.RUnlock()

	if !exists {
		// Events from runs we don't track (e.g., TUI-driven runs)
		return
	}

	switch event.Event.Type {
	case loop.EventQuotaExhausted:
		rm.handleQuotaExhausted(event.PRDName)

	case loop.EventIterationStart:
		rm.sendRunProgress(info, "iteration_started", event.Event)

	case loop.EventStoryStarted:
		// Track the current story ID
		rm.mu.Lock()
		info.storyID = event.Event.StoryID
		rm.mu.Unlock()
		rm.sendRunProgress(info, "story_started", event.Event)

	case loop.EventStoryCompleted:
		rm.sendRunProgress(info, "story_completed", event.Event)
		rm.sendStoryDiff(info, event.Event)

	case loop.EventComplete:
		rm.sendRunProgress(info, "complete", event.Event)
		rm.sendRunComplete(info, event.PRDName)

	case loop.EventMaxIterationsReached:
		rm.sendRunProgress(info, "max_iterations_reached", event.Event)
		rm.sendRunComplete(info, event.PRDName)

	case loop.EventRetrying:
		rm.sendRunProgress(info, "retrying", event.Event)

	case loop.EventAssistantText:
		rm.writeStoryLog(event.PRDName, info.storyID, event.Event.Text)
		rm.sendClaudeOutput(info, event.Event.Text, false)

	case loop.EventToolStart:
		text := fmt.Sprintf("[tool_use] %s", event.Event.Tool)
		rm.writeStoryLog(event.PRDName, info.storyID, text)
		rm.sendClaudeOutput(info, text, false)

	case loop.EventToolResult:
		rm.writeStoryLog(event.PRDName, info.storyID, event.Event.Text)
		rm.sendClaudeOutput(info, event.Event.Text, false)

	case loop.EventError:
		errText := ""
		if event.Event.Err != nil {
			errText = event.Event.Err.Error()
		}
		rm.writeStoryLog(event.PRDName, info.storyID, errText)
		rm.sendClaudeOutput(info, errText, true)
	}
}

// sendRunProgress sends a run_progress message over WebSocket.
func (rm *runManager) sendRunProgress(info *runInfo, status string, event loop.Event) {
	if rm.sender == nil {
		return
	}

	rm.mu.RLock()
	storyID := info.storyID
	rm.mu.RUnlock()

	// Use the event's story ID if available, otherwise use tracked story ID
	if event.StoryID != "" {
		storyID = event.StoryID
	}

	envelope := ws.NewMessage(ws.TypeRunProgress)
	msg := ws.RunProgressMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Project:   info.project,
		PRDID:     info.prdID,
		StoryID:   storyID,
		Status:    status,
		Iteration: event.Iteration,
		Attempt:   event.RetryCount,
	}
	if err := rm.sender.Send(msg); err != nil {
		log.Printf("Error sending run_progress: %v", err)
	}
}

// sendRunComplete sends a run_complete message over WebSocket.
func (rm *runManager) sendRunComplete(info *runInfo, prdName string) {
	if rm.sender == nil {
		return
	}

	// Calculate duration
	rm.mu.RLock()
	duration := time.Since(info.startTime)
	rm.mu.RUnlock()

	// Load PRD to get pass/fail counts
	var passCount, failCount, storiesCompleted int
	p, err := prd.LoadPRD(info.prdPath)
	if err == nil {
		for _, s := range p.UserStories {
			if s.Passes {
				passCount++
				storiesCompleted++
			} else {
				failCount++
			}
		}
	}

	envelope := ws.NewMessage(ws.TypeRunComplete)
	msg := ws.RunCompleteMessage{
		Type:             envelope.Type,
		ID:               envelope.ID,
		Timestamp:        envelope.Timestamp,
		Project:          info.project,
		PRDID:            info.prdID,
		StoriesCompleted: storiesCompleted,
		Duration:         duration.Round(time.Second).String(),
		PassCount:        passCount,
		FailCount:        failCount,
	}
	if err := rm.sender.Send(msg); err != nil {
		log.Printf("Error sending run_complete: %v", err)
	}
}

// sendClaudeOutput sends a claude_output message for an active run over WebSocket.
func (rm *runManager) sendClaudeOutput(info *runInfo, data string, done bool) {
	if rm.sender == nil {
		return
	}

	rm.mu.RLock()
	storyID := info.storyID
	rm.mu.RUnlock()

	envelope := ws.NewMessage(ws.TypeClaudeOutput)
	msg := ws.ClaudeOutputMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Project:   info.project,
		PRDID:     info.prdID,
		StoryID:   storyID,
		Data:      data,
		Done:      done,
	}
	if err := rm.sender.Send(msg); err != nil {
		log.Printf("Error sending claude_output: %v", err)
	}
}

// sendStoryDiff sends a proactive diff message when a story completes during a run.
func (rm *runManager) sendStoryDiff(info *runInfo, event loop.Event) {
	if rm.sender == nil {
		return
	}

	storyID := event.StoryID
	if storyID == "" {
		rm.mu.RLock()
		storyID = info.storyID
		rm.mu.RUnlock()
	}
	if storyID == "" {
		return
	}

	// Get the project path from the PRD path
	// prdPath is like /path/to/project/.chief/prds/<id>/prd.json
	projectPath := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(info.prdPath))))

	diffText, files, err := getStoryDiff(projectPath, storyID)
	if err != nil {
		log.Printf("Could not get diff for story %s: %v", storyID, err)
		return
	}

	sendDiffMessage(rm.sender, info.project, info.prdID, storyID, files, diffText)
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

	if rm.sender == nil {
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
	if err := rm.sender.Send(pausedMsg); err != nil {
		log.Printf("Error sending run_paused: %v", err)
	}

	// Send quota_exhausted message listing affected runs
	rm.sendQuotaExhausted(info.project, info.prdID)
}

// sendQuotaExhausted sends a quota_exhausted message over WebSocket.
func (rm *runManager) sendQuotaExhausted(project, prdID string) {
	if rm.sender == nil {
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
	if err := rm.sender.Send(msg); err != nil {
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

	// Create per-story logger (removes previous logs for this PRD)
	sl, err := newStoryLogger(prdPath)
	if err != nil {
		log.Printf("Warning: could not create story logger: %v", err)
	}

	rm.mu.Lock()
	rm.runs[key] = &runInfo{
		project:   project,
		prdID:     prdID,
		prdPath:   prdPath,
		startTime: time.Now(),
	}
	if sl != nil {
		rm.loggers[key] = sl
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

// writeStoryLog writes a line to the per-story log file.
func (rm *runManager) writeStoryLog(runKey, storyID, text string) {
	rm.mu.RLock()
	sl := rm.loggers[runKey]
	rm.mu.RUnlock()

	if sl != nil {
		sl.WriteLog(storyID, text)
	}
}

// cleanup removes tracking for a completed/stopped run.
func (rm *runManager) cleanup(key string) {
	rm.mu.Lock()
	if sl, ok := rm.loggers[key]; ok {
		sl.Close()
		delete(rm.loggers, key)
	}
	delete(rm.runs, key)
	rm.mu.Unlock()
}

// markInterruptedStories marks any in-progress stories as interrupted in prd.json
// so that the next run resumes from where it left off.
func (rm *runManager) markInterruptedStories() {
	rm.mu.RLock()
	runs := make([]*runInfo, 0, len(rm.runs))
	for _, info := range rm.runs {
		runs = append(runs, info)
	}
	rm.mu.RUnlock()

	for _, info := range runs {
		if info.storyID == "" {
			continue
		}
		p, err := prd.LoadPRD(info.prdPath)
		if err != nil {
			log.Printf("Warning: could not load PRD %s to mark interrupted story: %v", info.prdPath, err)
			continue
		}
		for i := range p.UserStories {
			if p.UserStories[i].ID == info.storyID && !p.UserStories[i].Passes {
				p.UserStories[i].InProgress = true
				if err := p.Save(info.prdPath); err != nil {
					log.Printf("Warning: could not save PRD %s: %v", info.prdPath, err)
				}
				break
			}
		}
	}
}

// activeRunCount returns the number of currently tracked runs.
func (rm *runManager) activeRunCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.runs)
}

// stopAll stops all active runs (for shutdown).
func (rm *runManager) stopAll() {
	rm.eng.StopAll()

	// Close all story loggers
	rm.mu.Lock()
	for key, sl := range rm.loggers {
		sl.Close()
		delete(rm.loggers, key)
	}
	rm.mu.Unlock()
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
func handleStartRun(sender messageSender, scanner projectFinder, runs *runManager, watcher activator, msg ws.Message) {
	var req ws.StartRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing start_run message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	if err := runs.startRun(req.Project, req.PRDID, project.Path); err != nil {
		if err.Error() == "RUN_ALREADY_ACTIVE" {
			sendError(sender, ws.ErrCodeRunAlreadyActive,
				fmt.Sprintf("Run already active for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(sender, ws.ErrCodeClaudeError,
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
func handlePauseRun(sender messageSender, runs *runManager, msg ws.Message) {
	var req ws.PauseRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing pause_run message: %v", err)
		return
	}

	if err := runs.pauseRun(req.Project, req.PRDID); err != nil {
		if err.Error() == "RUN_NOT_ACTIVE" {
			sendError(sender, ws.ErrCodeRunNotActive,
				fmt.Sprintf("No active run for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(sender, ws.ErrCodeClaudeError,
				fmt.Sprintf("Failed to pause run: %v", err), msg.ID)
		}
		return
	}

	log.Printf("Paused run for %s/%s", req.Project, req.PRDID)
}

// handleResumeRun handles a resume_run WebSocket message.
func handleResumeRun(sender messageSender, runs *runManager, msg ws.Message) {
	var req ws.ResumeRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing resume_run message: %v", err)
		return
	}

	if err := runs.resumeRun(req.Project, req.PRDID); err != nil {
		if err.Error() == "RUN_NOT_ACTIVE" {
			sendError(sender, ws.ErrCodeRunNotActive,
				fmt.Sprintf("No paused run for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(sender, ws.ErrCodeClaudeError,
				fmt.Sprintf("Failed to resume run: %v", err), msg.ID)
		}
		return
	}

	log.Printf("Resumed run for %s/%s", req.Project, req.PRDID)
}

// handleStopRun handles a stop_run WebSocket message.
func handleStopRun(sender messageSender, runs *runManager, msg ws.Message) {
	var req ws.StopRunMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing stop_run message: %v", err)
		return
	}

	if err := runs.stopRun(req.Project, req.PRDID); err != nil {
		if err.Error() == "RUN_NOT_ACTIVE" {
			sendError(sender, ws.ErrCodeRunNotActive,
				fmt.Sprintf("No active run for %s/%s", req.Project, req.PRDID), msg.ID)
		} else {
			sendError(sender, ws.ErrCodeClaudeError,
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
