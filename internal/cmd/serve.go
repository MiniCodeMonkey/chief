package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/minicodemonkey/chief/internal/engine"
	"github.com/minicodemonkey/chief/internal/uplink"
	"github.com/minicodemonkey/chief/internal/update"
	"github.com/minicodemonkey/chief/internal/workspace"
	"github.com/minicodemonkey/chief/internal/ws"
)

const (
	// defaultServerURL is the default HTTP base URL for the chief server.
	defaultServerURL = "https://uplink.chiefloop.com"
)

// ServeOptions contains configuration for the serve command.
type ServeOptions struct {
	Workspace   string          // Path to workspace directory
	DeviceName  string          // Override device name (default: from credentials)
	LogFile     string          // Path to log file (default: stdout)
	BaseURL     string          // Override base URL (for testing)
	ServerURL   string          // Override server URL for uplink (for testing/dev)
	Version     string          // Chief version string
	ReleasesURL string          // Override GitHub releases URL (for testing)
	Ctx         context.Context // Optional context for cancellation (for testing)

}

// RunServe starts the headless serve daemon.
func RunServe(opts ServeOptions) error {
	// Validate workspace directory exists
	if opts.Workspace == "" {
		opts.Workspace = "."
	}
	absWorkspace, err := filepath.Abs(opts.Workspace)
	if err != nil {
		return fmt.Errorf("resolving workspace path: %w", err)
	}
	opts.Workspace = absWorkspace
	info, err := os.Stat(opts.Workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workspace directory does not exist: %s", opts.Workspace)
		}
		return fmt.Errorf("checking workspace directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace path is not a directory: %s", opts.Workspace)
	}

	// Set up logging
	var logFile *os.File
	if opts.LogFile != "" {
		f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		logFile = f
		defer func() {
			logFile.Sync()
			logFile.Close()
		}()
		log.SetOutput(f)
	}

	// Check for credentials
	creds, err := auth.LoadCredentials()
	if err != nil {
		if errors.Is(err, auth.ErrNotLoggedIn) {
			return fmt.Errorf("Not logged in. Run 'chief login' first.")
		}
		return fmt.Errorf("loading credentials: %w", err)
	}

	// Refresh token if near-expiry
	if creds.IsNearExpiry(5 * time.Minute) {
		log.Println("Access token near expiry, refreshing...")
		creds, err = auth.RefreshToken(opts.BaseURL)
		if err != nil {
			return fmt.Errorf("refreshing token: %w", err)
		}
		log.Println("Token refreshed successfully")
	}

	// Determine device name
	deviceName := opts.DeviceName
	if deviceName == "" {
		deviceName = creds.DeviceName
	}

	// Determine server URL (precedence: ServerURL flag > env > default)
	serverURL := opts.ServerURL
	if serverURL == "" {
		serverURL = os.Getenv("CHIEF_SERVER_URL")
	}
	if serverURL == "" {
		serverURL = defaultServerURL
	}

	log.Printf("Starting chief serve (workspace: %s, device: %s)", opts.Workspace, deviceName)
	log.Printf("Connecting to %s", serverURL)

	// Set up context with cancellation for clean shutdown
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Determine version string
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	// Start workspace scanner (before connect so initial scan is ready)
	scanner := workspace.New(opts.Workspace, nil) // sender set after connect
	scanner.ScanAndUpdate()

	// Create engine for Ralph loop runs (default 5 max iterations)
	eng := engine.New(5)

	// Create rate limiter for incoming messages
	rateLimiter := ws.NewRateLimiter()

	// Create the uplink HTTP client
	httpClient, err := uplink.New(serverURL, creds.AccessToken,
		uplink.WithDeviceName(deviceName),
		uplink.WithChiefVersion(version),
	)
	if err != nil {
		return fmt.Errorf("creating uplink client: %w", err)
	}

	// Create the uplink with reconnect handler that re-sends state
	var sender messageSender
	var sessions *sessionManager
	var runs *runManager
	var ul *uplink.Uplink

	ul = uplink.NewUplink(httpClient,
		uplink.WithOnReconnect(func() {
			log.Println("Uplink reconnected, re-sending state snapshot")
			rateLimiter.Reset()
			sendStateSnapshot(sender, scanner, sessions, runs)
		}),
		uplink.WithOnAuthFailure(func() error {
			log.Println("Auth failed during reconnection, refreshing token...")
			newCreds, err := auth.RefreshToken(opts.BaseURL)
			if err != nil {
				return fmt.Errorf("token refresh failed: %w", err)
			}
			ul.SetAccessToken(newCreds.AccessToken)
			log.Println("Token refreshed successfully during reconnection")
			return nil
		}),
	)

	// Create the sender adapter that wraps the uplink
	sender = newUplinkSender(ul)

	// Set scanner's sender now that it exists
	scanner.SetSender(sender)

	// Create session manager for Claude PRD sessions
	sessions = newSessionManager(sender)

	// Create run manager for Ralph loop runs
	runs = newRunManager(eng, sender)

	// Start engine event monitor for quota detection
	runs.startEventMonitor(ctx)

	// Connect to server (HTTP connect + Pusher subscribe + batcher start)
	if err := ul.Connect(ctx); err != nil {
		if errors.Is(err, uplink.ErrAuthFailed) {
			return fmt.Errorf("Device deauthorized. Run 'chief login' to re-authenticate.")
		}
		if errors.Is(err, uplink.ErrDeviceRevoked) {
			return fmt.Errorf("Device deauthorized. Run 'chief login' to re-authenticate.")
		}
		return fmt.Errorf("connecting to server: %w", err)
	}
	log.Println("Connected to server")

	// Send initial state snapshot after successful connect
	sendStateSnapshot(sender, scanner, sessions, runs)

	// Start periodic scanning loop
	go scanner.Run(ctx)
	log.Println("Workspace scanner started")

	// Start file watcher
	watcher, err := workspace.NewWatcher(opts.Workspace, scanner, sender)
	if err != nil {
		log.Printf("Warning: could not start file watcher: %v", err)
	} else {
		go watcher.Run(ctx)
		log.Println("File watcher started")
	}

	// Start periodic version check (every 24 hours)
	go runVersionChecker(ctx, sender, opts.Version, opts.ReleasesURL)

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	log.Println("Serve is running. Press Ctrl+C to stop.")

	// Main event loop — commands arrive from Pusher via uplink.Receive()
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down...")
			return serveShutdown(ul, watcher, sessions, runs, eng)

		case sig := <-sigCh:
			log.Printf("Received signal %s, shutting down...", sig)
			return serveShutdown(ul, watcher, sessions, runs, eng)

		case raw, ok := <-ul.Receive():
			if !ok {
				// Channel closed, connection lost permanently
				log.Println("Uplink connection closed permanently")
				return serveShutdown(ul, watcher, sessions, runs, eng)
			}

			// Parse the raw JSON into a ws.Message for dispatch
			var msg ws.Message
			if err := json.Unmarshal(raw, &msg); err != nil {
				log.Printf("Ignoring unparseable command: %v", err)
				continue
			}
			msg.Raw = raw

			// Extract payload wrapper if present.
			// The CommandRelayController sends {"type": "...", "payload": {...}}
			// but handlers expect fields at the top level of msg.Raw.
			var env struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload,omitempty"`
			}
			if err := json.Unmarshal(raw, &env); err == nil && len(env.Payload) > 0 {
				msg.Raw = env.Payload
			}

			// Check rate limit before processing
			if result := rateLimiter.Allow(msg.Type); !result.Allowed {
				log.Printf("Rate limited message type=%s, retry after %s", msg.Type, ws.FormatRetryAfter(result.RetryAfter))
				sendError(sender, ws.ErrCodeRateLimited,
					fmt.Sprintf("Rate limited. Try again in %s.", ws.FormatRetryAfter(result.RetryAfter)),
					msg.ID)
				continue
			}

			if shouldExit := handleMessage(sender, scanner, watcher, sessions, runs, msg, version, opts.ReleasesURL); shouldExit {
				log.Println("Update installed, exiting for restart...")
				serveShutdown(ul, watcher, sessions, runs, eng)
				return nil
			}
		}
	}
}

// sendStateSnapshot sends a full state snapshot via the uplink.
func sendStateSnapshot(sender messageSender, scanner *workspace.Scanner, sessions *sessionManager, runs *runManager) {
	projects := scanner.Projects()
	envelope := ws.NewMessage(ws.TypeStateSnapshot)

	var activeSessions []ws.SessionState
	if sessions != nil {
		activeSessions = sessions.activeSessions()
	}
	if activeSessions == nil {
		activeSessions = []ws.SessionState{}
	}

	activeRuns := []ws.RunState{}
	if runs != nil {
		if r := runs.activeRuns(); r != nil {
			activeRuns = r
		}
	}

	snapshot := ws.StateSnapshotMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Projects:  projects,
		Runs:      activeRuns,
		Sessions:  activeSessions,
	}
	if err := sender.Send(snapshot); err != nil {
		log.Printf("Error sending state_snapshot: %v", err)
	} else {
		log.Printf("Sent state_snapshot with %d projects", len(projects))
	}
}

// sendError sends an error message.
func sendError(sender messageSender, code, message, requestID string) {
	envelope := ws.NewMessage(ws.TypeError)
	errMsg := ws.ErrorMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}
	if err := sender.Send(errMsg); err != nil {
		log.Printf("Error sending error message: %v", err)
	}
}

// handleMessage routes incoming commands.
// Returns true if the serve loop should exit (e.g., after a successful remote update).
func handleMessage(sender messageSender, scanner *workspace.Scanner, watcher *workspace.Watcher, sessions *sessionManager, runs *runManager, msg ws.Message, version, releasesURL string) bool {
	log.Printf("Received command type=%s id=%s", msg.Type, msg.ID)

	switch msg.Type {
	case ws.TypePing:
		pong := ws.NewMessage(ws.TypePong)
		if err := sender.Send(pong); err != nil {
			log.Printf("Error sending pong: %v", err)
		}

	case ws.TypeListProjects:
		handleListProjects(sender, scanner)

	case ws.TypeGetProject:
		handleGetProject(sender, scanner, watcher, msg)

	case ws.TypeGetPRD:
		handleGetPRD(sender, scanner, msg)

	case ws.TypeGetPRDs:
		handleGetPRDs(sender, scanner, msg)

	case ws.TypeNewPRD:
		handleNewPRD(sender, scanner, sessions, msg)

	case ws.TypeRefinePRD:
		handleRefinePRD(sender, scanner, sessions, msg)

	case ws.TypePRDMessage:
		handlePRDMessage(sender, sessions, msg)

	case ws.TypeClosePRDSession:
		handleClosePRDSession(sender, sessions, msg)

	case ws.TypeStartRun:
		handleStartRun(sender, scanner, runs, watcher, msg)

	case ws.TypePauseRun:
		handlePauseRun(sender, runs, msg)

	case ws.TypeResumeRun:
		handleResumeRun(sender, runs, msg)

	case ws.TypeStopRun:
		handleStopRun(sender, runs, msg)

	case ws.TypeGetDiff:
		handleGetDiff(sender, scanner, msg)

	case ws.TypeGetDiffs:
		handleGetDiffs(sender, scanner, msg)

	case ws.TypeGetLogs:
		handleGetLogs(sender, scanner, msg)

	case ws.TypeGetSettings:
		handleGetSettings(sender, scanner, msg)

	case ws.TypeUpdateSettings:
		handleUpdateSettings(sender, scanner, msg)

	case ws.TypeCloneRepo:
		handleCloneRepo(sender, scanner, msg)

	case ws.TypeCreateProject:
		handleCreateProject(sender, scanner, msg)

	case ws.TypeTriggerUpdate:
		return handleTriggerUpdate(sender, msg, version, releasesURL)

	default:
		log.Printf("Received message type: %s", msg.Type)
	}
	return false
}

// handleListProjects handles a list_projects request.
func handleListProjects(sender messageSender, scanner *workspace.Scanner) {
	projects := scanner.Projects()
	envelope := ws.NewMessage(ws.TypeProjectList)
	plMsg := ws.ProjectListMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Projects:  projects,
	}
	if err := sender.Send(plMsg); err != nil {
		log.Printf("Error sending project_list: %v", err)
	}
}

// handleGetProject handles a get_project request.
func handleGetProject(sender messageSender, scanner *workspace.Scanner, watcher *workspace.Watcher, msg ws.Message) {
	var req ws.GetProjectMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_project message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	// Activate file watching for the requested project
	if watcher != nil {
		watcher.Activate(req.Project)
	}

	envelope := ws.NewMessage(ws.TypeProjectState)
	psMsg := ws.ProjectStateMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Project:   project,
	}
	if err := sender.Send(psMsg); err != nil {
		log.Printf("Error sending project_state: %v", err)
	}
}

// handleGetPRD handles a get_prd request.
func handleGetPRD(sender messageSender, scanner *workspace.Scanner, msg ws.Message) {
	var req ws.GetPRDMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_prd message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	// Read PRD markdown content
	prdDir := filepath.Join(project.Path, ".chief", "prds", req.PRDID)
	prdMD := filepath.Join(prdDir, "prd.md")
	prdJSON := filepath.Join(prdDir, "prd.json")

	// Check that the PRD directory exists
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		sendError(sender, ws.ErrCodePRDNotFound,
			fmt.Sprintf("PRD %q not found in project %q", req.PRDID, req.Project), msg.ID)
		return
	}

	// Read markdown content (optional — may not exist yet)
	var content string
	if data, err := os.ReadFile(prdMD); err == nil {
		content = string(data)
	}

	// Read prd.json state
	var state interface{}
	if data, err := os.ReadFile(prdJSON); err == nil {
		var parsed interface{}
		if json.Unmarshal(data, &parsed) == nil {
			state = parsed
		}
	}

	envelope := ws.NewMessage(ws.TypePRDContent)
	prdMsg := ws.PRDContentMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Project:   req.Project,
		PRDID:     req.PRDID,
		Content:   content,
		State:     state,
	}
	if err := sender.Send(prdMsg); err != nil {
		log.Printf("Error sending prd_content: %v", err)
	}
}

// runVersionChecker periodically checks for updates and sends update_available.
func runVersionChecker(ctx context.Context, sender messageSender, version, releasesURL string) {
	// Check immediately on startup
	checkAndNotify(sender, version, releasesURL)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkAndNotify(sender, version, releasesURL)
		}
	}
}

// checkAndNotify performs a version check and sends update_available if needed.
func checkAndNotify(sender messageSender, version, releasesURL string) {
	result, err := update.CheckForUpdate(version, update.Options{
		ReleasesURL: releasesURL,
	})
	if err != nil {
		log.Printf("Version check failed: %v", err)
		return
	}
	if result.UpdateAvailable {
		log.Printf("Update available: v%s (current: v%s)", result.LatestVersion, result.CurrentVersion)
		envelope := ws.NewMessage(ws.TypeUpdateAvailable)
		msg := ws.UpdateAvailableMessage{
			Type:           envelope.Type,
			ID:             envelope.ID,
			Timestamp:      envelope.Timestamp,
			CurrentVersion: result.CurrentVersion,
			LatestVersion:  result.LatestVersion,
		}
		if err := sender.Send(msg); err != nil {
			log.Printf("Error sending update_available: %v", err)
		}
	}
}

// uplinkCloser is an interface for closing the uplink connection.
type uplinkCloser interface {
	Close() error
	CloseWithTimeout(timeout time.Duration) error
}

// shutdownTimeout is the maximum time allowed for the entire shutdown sequence.
// This covers: process killing, batcher flush, Pusher close, and HTTP disconnect.
const shutdownTimeout = 10 * time.Second

// processKillTimeout is the maximum time to wait for Claude processes to exit gracefully.
const processKillTimeout = 5 * time.Second

// serveShutdown performs clean shutdown of the serve command.
// It kills all child Claude processes, marks interrupted stories, closes the uplink,
// and flushes log files. Processes are force-killed after 5 seconds if they haven't
// exited gracefully. The entire shutdown completes within 10 seconds.
func serveShutdown(closer uplinkCloser, watcher *workspace.Watcher, sessions *sessionManager, runs *runManager, eng *engine.Engine) error {
	log.Println("Shutting down...")

	// Enforce an overall shutdown deadline.
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		doShutdown(closer, watcher, sessions, runs, eng)
	}()

	select {
	case <-shutdownDone:
		// Normal shutdown completed within the timeout.
	case <-time.After(shutdownTimeout):
		log.Printf("Shutdown timed out after %s — forcing exit", shutdownTimeout)
	}

	log.Println("Goodbye.")
	return nil
}

// doShutdown performs the actual shutdown sequence.
func doShutdown(closer uplinkCloser, watcher *workspace.Watcher, sessions *sessionManager, runs *runManager, eng *engine.Engine) {
	// Count processes before shutdown.
	processCount := 0
	if sessions != nil {
		processCount += sessions.sessionCount()
	}
	if runs != nil {
		processCount += runs.activeRunCount()
	}

	// Mark any in-progress stories as interrupted in prd.json.
	if runs != nil {
		runs.markInterruptedStories()
	}

	// Use a channel to track when graceful process shutdown completes.
	done := make(chan struct{})
	go func() {
		// Stop all active Ralph loop runs.
		if runs != nil {
			runs.stopAll()
		}

		// Kill all active Claude sessions.
		if sessions != nil {
			sessions.killAll()
		}

		// Shut down the engine (stops event forwarding goroutine).
		if eng != nil {
			eng.Shutdown()
		}

		close(done)
	}()

	// Wait for graceful shutdown or force-kill after 5 seconds.
	select {
	case <-done:
		// Graceful shutdown completed.
	case <-time.After(processKillTimeout):
		log.Println("Force-killing hung processes after 5 second timeout")
	}

	if processCount > 0 {
		log.Printf("Killed %d processes", processCount)
	}

	// Close file watcher.
	if watcher != nil {
		if err := watcher.Close(); err != nil {
			log.Printf("Error closing file watcher: %v", err)
		}
	}

	// Close the uplink with a timeout to prevent hanging on unreachable servers.
	// The batcher flush + Pusher close + HTTP disconnect must complete within this window.
	if err := closer.CloseWithTimeout(5 * time.Second); err != nil {
		log.Printf("Error closing connection: %v", err)
	}
}
