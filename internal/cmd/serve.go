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
	"github.com/minicodemonkey/chief/internal/workspace"
	"github.com/minicodemonkey/chief/internal/ws"
)

// ServeOptions contains configuration for the serve command.
type ServeOptions struct {
	Workspace  string          // Path to workspace directory
	DeviceName string          // Override device name (default: from credentials)
	LogFile    string          // Path to log file (default: stdout)
	BaseURL    string          // Override base URL (for testing)
	WSURL      string          // Override WebSocket URL (for testing/dev)
	Version    string          // Chief version string
	Ctx        context.Context // Optional context for cancellation (for testing)
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

	// Create WebSocket client with reconnect handler that re-sends state
	var client *ws.Client
	client = ws.New(wsURL, ws.WithOnReconnect(func() {
		log.Println("WebSocket reconnected, re-sending state snapshot")
		sendStateSnapshot(client, scanner)
	}))

	// Set scanner's client now that it exists
	scanner.SetClient(client)

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
	sendStateSnapshot(client, scanner)

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
			return serveShutdown(client, watcher)

		case sig := <-sigCh:
			log.Printf("Received signal %s, shutting down...", sig)
			return serveShutdown(client, watcher)

		case msg, ok := <-client.Receive():
			if !ok {
				// Channel closed, connection lost permanently
				log.Println("WebSocket connection closed permanently")
				return serveShutdown(client, watcher)
			}
			handleMessage(client, scanner, watcher, msg)
		}
	}
}

// sendStateSnapshot sends a full state snapshot over WebSocket.
func sendStateSnapshot(client *ws.Client, scanner *workspace.Scanner) {
	projects := scanner.Projects()
	envelope := ws.NewMessage(ws.TypeStateSnapshot)
	snapshot := ws.StateSnapshotMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Projects:  projects,
		Runs:      []ws.RunState{},
		Sessions:  []ws.SessionState{},
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
func handleMessage(client *ws.Client, scanner *workspace.Scanner, watcher *workspace.Watcher, msg ws.Message) {
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

// serveShutdown performs clean shutdown of the serve command.
func serveShutdown(client *ws.Client, watcher *workspace.Watcher) error {
	log.Println("Shutting down...")

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
