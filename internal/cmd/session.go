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

	"github.com/creack/pty"
	"github.com/minicodemonkey/chief/embed"
	"github.com/minicodemonkey/chief/internal/loop"
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

	// Spawn claude in print mode.
	// --output-format stream-json streams JSONL events as they are generated.
	// We use a PTY for stdin so Claude treats input as a real terminal — without
	// a PTY, Claude detects a non-TTY stdin and buffers all output until exit.
	cmd := exec.Command(claudeBinary(), "-p", "--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose", prompt)
	cmd.Dir = projectPath
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	// Create a PTY pair: ptm (master, used by chief) and pts (slave, used by Claude).
	ptm, pts, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to open PTY: %w", err)
	}
	cmd.Stdin = pts // Claude reads from the slave PTY (looks like a real terminal)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		ptm.Close()
		pts.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		ptm.Close()
		pts.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	log.Printf("[debug] newPRD: launching %s -p --dangerously-skip-permissions --output-format stream-json (PTY stdin) <prompt len=%d> in dir=%s", claudeBinary(), len(prompt), projectPath)
	if err := cmd.Start(); err != nil {
		ptm.Close()
		pts.Close()
		return fmt.Errorf("failed to start Claude: %w", err)
	}
	// Close the slave in the parent after the child has inherited it.
	pts.Close()
	log.Printf("[debug] newPRD: Claude process started pid=%d session=%s", cmd.Process.Pid, sessionID)

	// Drain PTY master reads (echo of what we write) to prevent buffer backpressure.
	go drainPTY(ptm)

	sess := &claudeSession{
		sessionID:   sessionID,
		project:     projectName,
		projectPath: projectPath,
		cmd:         cmd,
		stdin:       ptm, // Write user messages to PTY master
		done:        make(chan struct{}),
		lastActive:  time.Now(),
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = sess
	sm.mu.Unlock()

	// Stream stdout in a goroutine
	go sm.streamOutput(sessionID, stdoutPipe)

	// Stream stderr in a goroutine
	go sm.streamOutput(sessionID, stderrPipe)

	// Send the user's initial message via the PTY master.
	go func() {
		time.Sleep(100 * time.Millisecond)
		msg := initialMessage
		if msg == "" {
			msg = "Please start."
		}
		log.Printf("[debug] newPRD: writing PTY message (%d bytes) for session %s", len(msg), sessionID)
		n, err := fmt.Fprintf(ptm, "%s\n", msg)
		log.Printf("[debug] newPRD: PTY write complete n=%d err=%v for session %s", n, err, sessionID)
	}()

	// Watchdog: log process state every 10s until it exits.
	go func() {
		for i := 1; i <= 6; i++ {
			time.Sleep(10 * time.Second)
			select {
			case <-sess.done:
				return
			default:
			}
			if sess.cmd.ProcessState != nil {
				log.Printf("[debug] watchdog session=%s tick=%d: process already exited state=%v", sessionID, i, sess.cmd.ProcessState)
				return
			}
			log.Printf("[debug] watchdog session=%s tick=%d: process pid=%d still running", sessionID, i, sess.cmd.Process.Pid)
		}
	}()

	// Wait for process to exit
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("Claude session %s exited with error: %v (pid=%d)", sessionID, err, cmd.Process.Pid)
		} else {
			log.Printf("Claude session %s exited normally (pid=%d)", sessionID, cmd.Process.Pid)
		}

		// Send prd_response_complete to signal the PRD session is done
		log.Printf("[debug] newPRD: sending prd_response_complete for session %s", sessionID)
		completeMsg := ws.PRDResponseCompleteMessage{
			Type: ws.TypePRDResponseComplete,
			Payload: ws.PRDResponseCompletePayload{
				SessionID: sessionID,
				Project:   projectName,
			},
		}
		if sendErr := sm.sender.Send(completeMsg); sendErr != nil {
			log.Printf("Error sending prd_response_complete: %v", sendErr)
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

// refinePRD spawns a Claude PRD session to edit an existing PRD.
func (sm *sessionManager) refinePRD(projectPath, projectName, sessionID, prdID, message string) error {
	sm.mu.Lock()
	if _, exists := sm.sessions[sessionID]; exists {
		sm.mu.Unlock()
		return fmt.Errorf("session %s already exists", sessionID)
	}
	sm.mu.Unlock()

	// Verify the PRD directory exists
	prdDir := filepath.Join(projectPath, ".chief", "prds", prdID)
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		return fmt.Errorf("PRD %q not found in project", prdID)
	}

	// Build prompt from edit_prompt.txt template
	prompt := embed.GetEditPrompt(prdDir)

	// Spawn claude in print mode with PTY stdin so Claude treats input as a terminal.
	// --output-format stream-json streams JSONL events as they are generated.
	cmd := exec.Command(claudeBinary(), "-p", "--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose", prompt)
	cmd.Dir = projectPath
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	ptm, pts, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to open PTY: %w", err)
	}
	cmd.Stdin = pts

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		ptm.Close()
		pts.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		ptm.Close()
		pts.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		ptm.Close()
		pts.Close()
		return fmt.Errorf("failed to start Claude: %w", err)
	}
	pts.Close()

	go drainPTY(ptm)

	sess := &claudeSession{
		sessionID:   sessionID,
		project:     projectName,
		projectPath: projectPath,
		cmd:         cmd,
		stdin:       ptm,
		done:        make(chan struct{}),
		lastActive:  time.Now(),
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = sess
	sm.mu.Unlock()

	// Stream stdout in a goroutine
	go sm.streamOutput(sessionID, stdoutPipe)

	// Stream stderr in a goroutine
	go sm.streamOutput(sessionID, stderrPipe)

	// Send the user's message via the PTY master
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintf(ptm, "%s\n", message)
	}()

	// Wait for process to exit
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("Claude session %s exited with error: %v", sessionID, err)
		} else {
			log.Printf("Claude session %s exited normally", sessionID)
		}

		// Send prd_response_complete to signal the PRD session is done
		completeMsg := ws.PRDResponseCompleteMessage{
			Type: ws.TypePRDResponseComplete,
			Payload: ws.PRDResponseCompletePayload{
				SessionID: sessionID,
				Project:   projectName,
			},
		}
		if sendErr := sm.sender.Send(completeMsg); sendErr != nil {
			log.Printf("Error sending prd_response_complete: %v", sendErr)
		}

		// Auto-convert prd.md to prd.json if prd.md was updated
		sm.autoConvert(projectPath)

		close(sess.done)

		sm.mu.Lock()
		delete(sm.sessions, sessionID)
		sm.mu.Unlock()
	}()

	return nil
}

// streamOutput reads stream-json JSONL from r, extracts assistant text events,
// and forwards them as prd_output messages. Non-text events (tool calls, etc.) are logged.
// Stderr lines are forwarded as-is since they don't contain JSONL.
func (sm *sessionManager) streamOutput(sessionID string, r io.Reader) {
	sm.mu.RLock()
	sess := sm.sessions[sessionID]
	sm.mu.RUnlock()
	if sess == nil {
		log.Printf("[debug] streamOutput: session %s not found", sessionID)
		return
	}

	log.Printf("[debug] streamOutput: started for session %s", sessionID)
	lineCount := 0
	textCount := 0

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		if lineCount <= 5 {
			preview := line
			if len(preview) > 120 {
				preview = preview[:120] + "..."
			}
			log.Printf("[debug] streamOutput session=%s line=%d: %q", sessionID, lineCount, preview)
		}

		// Try to parse as stream-json and extract assistant text.
		event := loop.ParseLine(line)
		if event != nil && event.Type == loop.EventAssistantText && event.Text != "" {
			textCount++
			outMsg := ws.PRDOutputMessage{
				Type: ws.TypePRDOutput,
				Payload: ws.PRDOutputPayload{
					Content:   event.Text,
					SessionID: sessionID,
					Project:   sess.project,
				},
			}
			if sendErr := sm.sender.Send(outMsg); sendErr != nil {
				log.Printf("[debug] streamOutput: send error for session %s: %v", sessionID, sendErr)
				return
			}
		} else if event != nil {
			log.Printf("[debug] streamOutput session=%s non-text event: %s", sessionID, event.Type.String())
		} else if line != "" {
			// ParseLine returned nil — this is either a stream-json event we
			// don't handle (hook_started, hook_response, thinking blocks, result,
			// rate_limit_event, etc.) or a non-JSON stderr line. Only forward
			// non-JSON lines; silently skip unhandled JSON events.
			var raw json.RawMessage
			if json.Unmarshal([]byte(line), &raw) == nil {
				// Valid JSON event we don't need — skip silently.
				continue
			}
			// Non-JSON line (stderr output) — forward as-is.
			outMsg := ws.PRDOutputMessage{
				Type: ws.TypePRDOutput,
				Payload: ws.PRDOutputPayload{
					Content:   line + "\n",
					SessionID: sessionID,
					Project:   sess.project,
				},
			}
			if sendErr := sm.sender.Send(outMsg); sendErr != nil {
				log.Printf("[debug] streamOutput: send error for session %s: %v", sessionID, sendErr)
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[debug] streamOutput: scanner error for session %s after %d lines: %v", sessionID, lineCount, err)
	} else {
		log.Printf("[debug] streamOutput: EOF for session %s after %d lines (%d text events)", sessionID, lineCount, textCount)
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

	if err := sessions.newPRD(project.Path, req.Project, req.SessionID, req.Message); err != nil {
		sendError(sender, ws.ErrCodeClaudeError,
			fmt.Sprintf("Failed to start Claude session: %v", err), msg.ID)
		return
	}

	log.Printf("Started Claude PRD session %s for project %s", req.SessionID, req.Project)
}

// handleRefinePRD handles a refine_prd WebSocket message.
func handleRefinePRD(sender messageSender, scanner projectFinder, sessions *sessionManager, msg ws.Message) {
	var req ws.RefinePRDMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing refine_prd message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	if err := sessions.refinePRD(project.Path, req.Project, req.SessionID, req.PRDID, req.Message); err != nil {
		sendError(sender, ws.ErrCodeClaudeError,
			fmt.Sprintf("Failed to start Claude session: %v", err), msg.ID)
		return
	}

	log.Printf("Started Claude PRD refine session %s for project %s (prd: %s)", req.SessionID, req.Project, req.PRDID)
}

// handlePRDMessage handles a prd_message WebSocket message.
func handlePRDMessage(sender messageSender, sessions *sessionManager, msg ws.Message) {
	var req ws.PRDMessageMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing prd_message: %v", err)
		return
	}

	if err := sessions.sendMessage(req.SessionID, req.Message); err != nil {
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

// drainPTY reads and discards data from a PTY master file descriptor.
// This prevents the PTY echo buffer from filling up when we write user messages.
// Runs until the PTY is closed (typically when the child process exits).
func drainPTY(f *os.File) {
	buf := make([]byte, 256)
	for {
		_, err := f.Read(buf)
		if err != nil {
			return
		}
	}
}

// claudeBinary returns the path to the claude CLI binary.
// It checks the CHIEF_CLAUDE_BINARY environment variable first, falling back to "claude".
func claudeBinary() string {
	if bin := os.Getenv("CHIEF_CLAUDE_BINARY"); bin != "" {
		return bin
	}
	return "claude"
}

// filterEnv returns a copy of env with the named variables removed.
func filterEnv(env []string, keys ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, key := range keys {
			if len(e) > len(key) && e[:len(key)+1] == key+"=" {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// projectFinder is an interface for finding projects (for testability).
type projectFinder interface {
	FindProject(name string) (ws.ProjectSummary, bool)
}
