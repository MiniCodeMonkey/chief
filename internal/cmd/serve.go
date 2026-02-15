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
	"github.com/minicodemonkey/chief/internal/update"
	"github.com/minicodemonkey/chief/internal/workspace"
	"github.com/minicodemonkey/chief/internal/ws"
)

// ServeOptions contains configuration for the serve command.
type ServeOptions struct {
	Workspace   string          // Path to workspace directory
	DeviceName  string          // Override device name (default: from credentials)
	LogFile     string          // Path to log file (default: stdout)
	BaseURL     string          // Override base URL (for testing)
	WSURL       string          // Override WebSocket URL (for testing/dev)
	Version     string          // Chief version string
	ReleasesURL string          // Override GitHub releases URL (for testing)
	Ctx         context.Context // Optional context for cancellation (for testing)
}

// RunServe starts the headless serve daemon.
func RunServe(opts ServeOptions) error {
	// Validate workspace directory exists
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
	if opts.LogFile != "" {
		f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer f.Close()
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

	// Determine WebSocket URL
	wsURL := opts.WSURL
	if wsURL == "" {
		wsURL = ws.DefaultURL
	}

	log.Printf("Starting chief serve (workspace: %s, device: %s)", opts.Workspace, deviceName)
	log.Printf("Connecting to %s", wsURL)

	// Set up context with cancellation for clean shutdown
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start workspace scanner (before WebSocket connect so initial scan is ready)
	scanner := workspace.New(opts.Workspace, nil) // client set after connect
	scanner.ScanAndUpdate()

	// Create engine for Ralph loop runs (default 5 max iterations)
	eng := engine.New(5)
	defer eng.Shutdown()

	// Create WebSocket client with reconnect handler that re-sends state
	var client *ws.Client
	var sessions *sessionManager
	var runs *runManager
	client = ws.New(wsURL, ws.WithOnReconnect(func() {
		log.Println("WebSocket reconnected, re-sending state snapshot")
		sendStateSnapshot(client, scanner, sessions, runs)
	}))

	// Set scanner's client now that it exists
	scanner.SetClient(client)

	// Create session manager for Claude PRD sessions
	sessions = newSessionManager(client)

	// Create run manager for Ralph loop runs
	runs = newRunManager(eng, client)

	// Start engine event monitor for quota detection
	runs.startEventMonitor(ctx)

	// Connect to WebSocket server
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connecting to WebSocket server: %w", err)
	}
	log.Println("WebSocket connected")

	// Perform protocol handshake
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	if err := client.Handshake(creds.AccessToken, version, deviceName); err != nil {
		// Cancel context first to stop reconnection attempts before closing
		cancel()
		client.Close()
		var incompErr *ws.ErrIncompatible
		if errors.As(err, &incompErr) {
			return fmt.Errorf("incompatible version: %s", incompErr.Message)
		}
		if errors.Is(err, ws.ErrAuthFailed) {
			return fmt.Errorf("Device deauthorized. Run 'chief login' to re-authenticate.")
		}
		return fmt.Errorf("handshake failed: %w", err)
	}
	log.Println("Handshake complete")

	// Send initial state snapshot after successful handshake
	sendStateSnapshot(client, scanner, sessions, runs)

	// Start periodic scanning loop
	go scanner.Run(ctx)
	log.Println("Workspace scanner started")

	// Start file watcher
	watcher, err := workspace.NewWatcher(opts.Workspace, scanner, client)
	if err != nil {
		log.Printf("Warning: could not start file watcher: %v", err)
	} else {
		go watcher.Run(ctx)
		log.Println("File watcher started")
	}

	// Start periodic version check (every 24 hours)
	go runVersionChecker(ctx, client, opts.Version, opts.ReleasesURL)

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	log.Println("Serve is running. Press Ctrl+C to stop.")

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down...")
			return serveShutdown(client, watcher, sessions, runs)

		case sig := <-sigCh:
			log.Printf("Received signal %s, shutting down...", sig)
			return serveShutdown(client, watcher, sessions, runs)

		case msg, ok := <-client.Receive():
			if !ok {
				// Channel closed, connection lost permanently
				log.Println("WebSocket connection closed permanently")
				return serveShutdown(client, watcher, sessions, runs)
			}
			handleMessage(client, scanner, watcher, sessions, runs, msg)
		}
	}
}

// sendStateSnapshot sends a full state snapshot over WebSocket.
func sendStateSnapshot(client *ws.Client, scanner *workspace.Scanner, sessions *sessionManager, runs *runManager) {
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
	if err := client.Send(snapshot); err != nil {
		log.Printf("Error sending state_snapshot: %v", err)
	} else {
		log.Printf("Sent state_snapshot with %d projects", len(projects))
	}
}

// sendError sends an error message over WebSocket.
func sendError(client *ws.Client, code, message, requestID string) {
	envelope := ws.NewMessage(ws.TypeError)
	errMsg := ws.ErrorMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}
	if err := client.Send(errMsg); err != nil {
		log.Printf("Error sending error message: %v", err)
	}
}

// handleMessage routes incoming WebSocket messages.
func handleMessage(client *ws.Client, scanner *workspace.Scanner, watcher *workspace.Watcher, sessions *sessionManager, runs *runManager, msg ws.Message) {
	switch msg.Type {
	case ws.TypePing:
		pong := ws.NewMessage(ws.TypePong)
		if err := client.Send(pong); err != nil {
			log.Printf("Error sending pong: %v", err)
		}

	case ws.TypeListProjects:
		handleListProjects(client, scanner)

	case ws.TypeGetProject:
		handleGetProject(client, scanner, watcher, msg)

	case ws.TypeGetPRD:
		handleGetPRD(client, scanner, msg)

	case ws.TypeNewPRD:
		handleNewPRD(client, scanner, sessions, msg)

	case ws.TypePRDMessage:
		handlePRDMessage(client, sessions, msg)

	case ws.TypeClosePRDSession:
		handleClosePRDSession(client, sessions, msg)

	case ws.TypeStartRun:
		handleStartRun(client, scanner, runs, watcher, msg)

	case ws.TypePauseRun:
		handlePauseRun(client, runs, msg)

	case ws.TypeResumeRun:
		handleResumeRun(client, runs, msg)

	case ws.TypeStopRun:
		handleStopRun(client, runs, msg)

	case ws.TypeGetDiff:
		handleGetDiff(client, scanner, msg)

	case ws.TypeGetLogs:
		handleGetLogs(client, scanner, msg)

	case ws.TypeGetSettings:
		handleGetSettings(client, scanner, msg)

	case ws.TypeUpdateSettings:
		handleUpdateSettings(client, scanner, msg)

	case ws.TypeCloneRepo:
		handleCloneRepo(client, scanner, msg)

	case ws.TypeCreateProject:
		handleCreateProject(client, scanner, msg)

	default:
		log.Printf("Received message type: %s", msg.Type)
	}
}

// handleListProjects handles a list_projects request.
func handleListProjects(client *ws.Client, scanner *workspace.Scanner) {
	projects := scanner.Projects()
	envelope := ws.NewMessage(ws.TypeProjectList)
	plMsg := ws.ProjectListMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Projects:  projects,
	}
	if err := client.Send(plMsg); err != nil {
		log.Printf("Error sending project_list: %v", err)
	}
}

// handleGetProject handles a get_project request.
func handleGetProject(client *ws.Client, scanner *workspace.Scanner, watcher *workspace.Watcher, msg ws.Message) {
	var req ws.GetProjectMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_project message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(client, ws.ErrCodeProjectNotFound,
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
	if err := client.Send(psMsg); err != nil {
		log.Printf("Error sending project_state: %v", err)
	}
}

// handleGetPRD handles a get_prd request.
func handleGetPRD(client *ws.Client, scanner *workspace.Scanner, msg ws.Message) {
	var req ws.GetPRDMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_prd message: %v", err)
		return
	}

	project, found := scanner.FindProject(req.Project)
	if !found {
		sendError(client, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	// Read PRD markdown content
	prdDir := filepath.Join(project.Path, ".chief", "prds", req.PRDID)
	prdMD := filepath.Join(prdDir, "prd.md")
	prdJSON := filepath.Join(prdDir, "prd.json")

	// Check that the PRD directory exists
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		sendError(client, ws.ErrCodePRDNotFound,
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
	if err := client.Send(prdMsg); err != nil {
		log.Printf("Error sending prd_content: %v", err)
	}
}

// runVersionChecker periodically checks for updates and sends update_available over WebSocket.
func runVersionChecker(ctx context.Context, client *ws.Client, version, releasesURL string) {
	// Check immediately on startup
	checkAndNotify(client, version, releasesURL)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkAndNotify(client, version, releasesURL)
		}
	}
}

// checkAndNotify performs a version check and sends update_available if needed.
func checkAndNotify(client *ws.Client, version, releasesURL string) {
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
		if err := client.Send(msg); err != nil {
			log.Printf("Error sending update_available: %v", err)
		}
	}
}

// serveShutdown performs clean shutdown of the serve command.
func serveShutdown(client *ws.Client, watcher *workspace.Watcher, sessions *sessionManager, runs *runManager) error {
	log.Println("Shutting down...")

	// Stop all active Ralph loop runs
	if runs != nil {
		runs.stopAll()
	}

	// Kill all active Claude sessions
	if sessions != nil {
		sessions.killAll()
	}

	// Close file watcher
	if watcher != nil {
		if err := watcher.Close(); err != nil {
			log.Printf("Error closing file watcher: %v", err)
		}
	}

	// Close WebSocket connection
	if err := client.Close(); err != nil {
		log.Printf("Error closing WebSocket: %v", err)
	}

	log.Println("Goodbye.")
	return nil
}
