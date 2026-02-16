package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/minicodemonkey/chief/embed"
	"github.com/minicodemonkey/chief/internal/prd"
	"github.com/minicodemonkey/chief/internal/ws"
)

// Default session timeout configuration.
const (
	defaultSessionTimeout = 30 * time.Minute
)

// Default warning thresholds (minutes of inactivity at which to warn).
var defaultWarningThresholds = []int{20, 25, 29}

// claudeSession tracks a single Claude PRD session process.
type claudeSession struct {
	sessionID   string
	project     string
	projectPath string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	done        chan struct{} // closed when the process exits
	lastActive  time.Time    // last time a prd_message was received
	activeMu    sync.Mutex   // protects lastActive
}

// resetActivity updates the last active time for this session.
func (s *claudeSession) resetActivity() {
	s.activeMu.Lock()
	s.lastActive = time.Now()
	s.activeMu.Unlock()
}

// inactiveDuration returns how long the session has been inactive.
func (s *claudeSession) inactiveDuration() time.Duration {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	return time.Since(s.lastActive)
}

// sessionManager manages Claude PRD sessions spawned via WebSocket.
type sessionManager struct {
	mu                sync.RWMutex
	sessions          map[string]*claudeSession
	sender            messageSender
	timeout           time.Duration // session inactivity timeout
	warningThresholds []int         // minutes of inactivity at which to send warnings
	checkInterval     time.Duration // how often to check for timeouts (configurable for tests)
	stopTimeout       chan struct{} // closed to stop the timeout checker
}

// newSessionManager creates a new session manager.
func newSessionManager(sender messageSender) *sessionManager {
	sm := &sessionManager{
		sessions:          make(map[string]*claudeSession),
		sender:            sender,
		timeout:           defaultSessionTimeout,
		warningThresholds: defaultWarningThresholds,
		checkInterval:     30 * time.Second,
		stopTimeout:       make(chan struct{}),
	}
	go sm.runTimeoutChecker(sm.stopTimeout)
	return sm
}

// sessionCount returns the number of active sessions.
func (sm *sessionManager) sessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// getSession returns a session by ID, or nil if not found.
func (sm *sessionManager) getSession(sessionID string) *claudeSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[sessionID]
}

// activeSessions returns a list of active session states for state snapshots.
func (sm *sessionManager) activeSessions() []ws.SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var sessions []ws.SessionState
	for _, s := range sm.sessions {
		sessions = append(sessions, ws.SessionState{
			SessionID: s.sessionID,
			Project:   s.project,
		})
	}
	return sessions
}

// newPRD spawns a new Claude PRD session.
func (sm *sessionManager) newPRD(projectPath, projectName, sessionID, initialMessage string) error {
	sm.mu.Lock()
	if _, exists := sm.sessions[sessionID]; exists {
		sm.mu.Unlock()
		return fmt.Errorf("session %s already exists", sessionID)
	}
	sm.mu.Unlock()

	// Ensure .chief/prds directory structure exists
	prdsDir := filepath.Join(projectPath, ".chief", "prds")
	if err := os.MkdirAll(prdsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create prds directory: %w", err)
	}

	// Build prompt from init_prompt.txt template
	// Use a temp PRD dir name based on session ID — Claude will create the actual
	// directory when it writes prd.md (the init prompt instructs it to).
	// We pass the prds base dir so the prompt has the right context.
	prompt := embed.GetInitPrompt(prdsDir, initialMessage)

	// Spawn claude with the prompt as argument (interactive mode)
	cmd := exec.Command("claude", prompt)
	cmd.Dir = projectPath

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return fmt.Errorf("failed to start Claude: %w", err)
	}

	sess := &claudeSession{
		sessionID:   sessionID,
		project:     projectName,
		projectPath: projectPath,
		cmd:         cmd,
		stdin:       stdinPipe,
		done:        make(chan struct{}),
		lastActive:  time.Now(),
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = sess
	sm.mu.Unlock()

	// Stream stdout in a goroutine
	go sm.streamOutput(sessionID, stdoutPipe)

	// Stream stderr in a goroutine (merged into same claude_output)
	go sm.streamOutput(sessionID, stderrPipe)

	// Wait for process to exit
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("Claude session %s exited with error: %v", sessionID, err)
		} else {
			log.Printf("Claude session %s exited normally", sessionID)
		}

		// Send final done message
		envelope := ws.NewMessage(ws.TypeClaudeOutput)
		doneMsg := ws.ClaudeOutputMessage{
			Type:      envelope.Type,
			ID:        envelope.ID,
			Timestamp: envelope.Timestamp,
			SessionID: sessionID,
			Project:   projectName,
			Data:      "",
			Done:      true,
		}
		if sendErr := sm.sender.Send(doneMsg); sendErr != nil {
			log.Printf("Error sending claude_output done: %v", sendErr)
		}

		// Auto-convert prd.md to prd.json if prd.md was created
		sm.autoConvert(projectPath)

		close(sess.done)

		sm.mu.Lock()
		delete(sm.sessions, sessionID)
		sm.mu.Unlock()
	}()

	return nil
}

// streamOutput reads from an io.Reader and sends each chunk as a claude_output message.
func (sm *sessionManager) streamOutput(sessionID string, r io.Reader) {
	sm.mu.RLock()
	sess := sm.sessions[sessionID]
	sm.mu.RUnlock()
	if sess == nil {
		return
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		envelope := ws.NewMessage(ws.TypeClaudeOutput)
		msg := ws.ClaudeOutputMessage{
			Type:      envelope.Type,
			ID:        envelope.ID,
			Timestamp: envelope.Timestamp,
			SessionID: sessionID,
			Project:   sess.project,
			Data:      line + "\n",
			Done:      false,
		}
		if err := sm.sender.Send(msg); err != nil {
			log.Printf("Error sending claude_output: %v", err)
			return
		}
	}
}

// sendMessage writes a user message to an active session's stdin.
func (sm *sessionManager) sendMessage(sessionID, content string) error {
	sess := sm.getSession(sessionID)
	if sess == nil {
		return fmt.Errorf("session not found")
	}

	// Reset the inactivity timer
	sess.resetActivity()

	// Write the message followed by a newline to the Claude process stdin
	_, err := fmt.Fprintf(sess.stdin, "%s\n", content)
	if err != nil {
		return fmt.Errorf("failed to write to Claude stdin: %w", err)
	}
	return nil
}

// closeSession closes a PRD session. If save is true, waits for Claude to finish.
// If save is false, kills immediately.
func (sm *sessionManager) closeSession(sessionID string, save bool) error {
	sess := sm.getSession(sessionID)
	if sess == nil {
		return fmt.Errorf("session not found")
	}

	if save {
		// Close stdin to signal EOF to Claude, then wait for it to finish
		sess.stdin.Close()
		<-sess.done
	} else {
		// Kill immediately
		if sess.cmd.Process != nil {
			sess.cmd.Process.Kill()
		}
		<-sess.done
	}

	return nil
}

// killAll kills all active sessions (used during shutdown).
func (sm *sessionManager) killAll() {
	// Stop the timeout checker
	select {
	case <-sm.stopTimeout:
		// Already closed
	default:
		close(sm.stopTimeout)
	}

	sm.mu.RLock()
	sessions := make([]*claudeSession, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	sm.mu.RUnlock()

	for _, s := range sessions {
		if s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
	}

	// Wait for all to finish
	for _, s := range sessions {
		<-s.done
	}
}

// runTimeoutChecker periodically checks all sessions for inactivity and sends
// warnings at the configured thresholds. When the timeout is reached, the session
// is expired: state is saved to disk, the process is killed, and session_expired is sent.
func (sm *sessionManager) runTimeoutChecker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(sm.checkInterval)
	defer ticker.Stop()

	// Track which warnings have been sent for each session to avoid duplicates.
	// Key: sessionID, Value: set of warning minutes already sent.
	sentWarnings := make(map[string]map[int]bool)

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			sm.mu.RLock()
			sessions := make([]*claudeSession, 0, len(sm.sessions))
			for _, s := range sm.sessions {
				sessions = append(sessions, s)
			}
			sm.mu.RUnlock()

			for _, sess := range sessions {
				inactive := sess.inactiveDuration()
				inactiveMinutes := int(inactive.Minutes())

				// Check if session should be expired
				if inactive >= sm.timeout {
					log.Printf("Session %s timed out after %v of inactivity", sess.sessionID, sm.timeout)
					sm.expireSession(sess)
					delete(sentWarnings, sess.sessionID)
					continue
				}

				// Check warning thresholds
				if _, ok := sentWarnings[sess.sessionID]; !ok {
					sentWarnings[sess.sessionID] = make(map[int]bool)
				}

				for _, threshold := range sm.warningThresholds {
					if inactiveMinutes >= threshold && !sentWarnings[sess.sessionID][threshold] {
						timeoutMinutes := int(sm.timeout.Minutes())
						remaining := timeoutMinutes - threshold
						log.Printf("Session %s: sending timeout warning (%d minutes remaining)", sess.sessionID, remaining)
						sm.sendTimeoutWarning(sess.sessionID, remaining)
						sentWarnings[sess.sessionID][threshold] = true
					}
				}
			}

			// Clean up sentWarnings for sessions that no longer exist
			sm.mu.RLock()
			for sid := range sentWarnings {
				if _, exists := sm.sessions[sid]; !exists {
					delete(sentWarnings, sid)
				}
			}
			sm.mu.RUnlock()
		}
	}
}

// sendTimeoutWarning sends a session_timeout_warning message over WebSocket.
func (sm *sessionManager) sendTimeoutWarning(sessionID string, minutesRemaining int) {
	envelope := ws.NewMessage(ws.TypeSessionTimeoutWarning)
	msg := ws.SessionTimeoutWarningMessage{
		Type:             envelope.Type,
		ID:               envelope.ID,
		Timestamp:        envelope.Timestamp,
		SessionID:        sessionID,
		MinutesRemaining: minutesRemaining,
	}
	if err := sm.sender.Send(msg); err != nil {
		log.Printf("Error sending session_timeout_warning: %v", err)
	}
}

// expireSession saves whatever PRD state exists, kills the Claude process,
// and sends a session_expired message.
func (sm *sessionManager) expireSession(sess *claudeSession) {
	// Close stdin to let Claude finish writing, then kill after a brief grace period
	sess.stdin.Close()

	// Give Claude 2 seconds to finish writing
	select {
	case <-sess.done:
		// Process exited cleanly
	case <-time.After(2 * time.Second):
		// Force kill
		if sess.cmd.Process != nil {
			sess.cmd.Process.Kill()
		}
		<-sess.done
	}

	// Send session_expired message
	envelope := ws.NewMessage(ws.TypeSessionExpired)
	expiredMsg := ws.SessionExpiredMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		SessionID: sess.sessionID,
	}
	if err := sm.sender.Send(expiredMsg); err != nil {
		log.Printf("Error sending session_expired: %v", err)
	}

	log.Printf("Session %s expired and cleaned up", sess.sessionID)
}

// autoConvert scans for any prd.md files that need conversion and converts them.
func (sm *sessionManager) autoConvert(projectPath string) {
	prdsDir := filepath.Join(projectPath, ".chief", "prds")
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		prdDir := filepath.Join(prdsDir, entry.Name())
		needs, err := prd.NeedsConversion(prdDir)
		if err != nil {
			log.Printf("Error checking conversion for %s: %v", prdDir, err)
			continue
		}
		if needs {
			log.Printf("Auto-converting PRD in %s", prdDir)
			if err := prd.Convert(prd.ConvertOptions{PRDDir: prdDir}); err != nil {
				log.Printf("Auto-conversion failed for %s: %v", prdDir, err)
			} else {
				log.Printf("Auto-conversion succeeded for %s", prdDir)
			}
		}
	}
}

// handleNewPRD handles a new_prd WebSocket message.
func handleNewPRD(sender messageSender, scanner projectFinder, sessions *sessionManager, msg ws.Message) {
	var req ws.NewPRDMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing new_prd message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	if err := sessions.newPRD(project.Path, req.Project, req.SessionID, req.InitialMessage); err != nil {
		sendError(sender, ws.ErrCodeClaudeError,
			fmt.Sprintf("Failed to start Claude session: %v", err), msg.ID)
		return
	}

	log.Printf("Started Claude PRD session %s for project %s", req.SessionID, req.Project)
}

// handlePRDMessage handles a prd_message WebSocket message.
func handlePRDMessage(sender messageSender, sessions *sessionManager, msg ws.Message) {
	var req ws.PRDMessageMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing prd_message: %v", err)
		return
	}

	if err := sessions.sendMessage(req.SessionID, req.Content); err != nil {
		sendError(sender, ws.ErrCodeSessionNotFound,
			fmt.Sprintf("Session %q not found", req.SessionID), msg.ID)
		return
	}
}

// handleClosePRDSession handles a close_prd_session WebSocket message.
func handleClosePRDSession(sender messageSender, sessions *sessionManager, msg ws.Message) {
	var req ws.ClosePRDSessionMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing close_prd_session: %v", err)
		return
	}

	if err := sessions.closeSession(req.SessionID, req.Save); err != nil {
		sendError(sender, ws.ErrCodeSessionNotFound,
			fmt.Sprintf("Session %q not found", req.SessionID), msg.ID)
		return
	}

	log.Printf("Closed Claude PRD session %s (save=%v)", req.SessionID, req.Save)
}

// projectFinder is an interface for finding projects (for testability).
type projectFinder interface {
	FindProject(name string) (ws.ProjectSummary, bool)
}
