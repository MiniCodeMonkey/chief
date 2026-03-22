package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/protocol"
	"github.com/minicodemonkey/chief/internal/session"
	"github.com/minicodemonkey/chief/internal/uplink"
)

// ServeOptions configures the serve command.
type ServeOptions struct {
	WorkspaceDir  string        // defaults to current directory
	HomeDir       string        // for testing — empty uses real home dir
	Version       string        // chief CLI version
	Provider      loop.Provider // agent provider for running loops (nil = no run support)
	MaxIterations int           // max iterations per run (0 = default 50)
}

// RunServe starts the persistent daemon that discovers projects, connects to
// Uplink, and handles remote commands. It blocks until SIGINT/SIGTERM.
func RunServe(opts ServeOptions) error {
	if opts.WorkspaceDir == "" {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		opts.WorkspaceDir = dir
	}

	// Load credentials.
	creds, err := loadCredentials(opts.HomeDir)
	if err != nil {
		return err
	}

	// Discover projects.
	collector := uplink.NewStateCollector(opts.WorkspaceDir, opts.Version)
	syncPayload, err := collector.Collect()
	if err != nil {
		return fmt.Errorf("collect state: %w", err)
	}

	fmt.Printf("chief serve starting...\n")
	fmt.Printf("  Workspace: %s\n", opts.WorkspaceDir)
	fmt.Printf("  Projects discovered: %d\n", len(syncPayload.Projects))
	for _, p := range syncPayload.Projects {
		fmt.Printf("    - %s (%s)\n", p.Name, p.Path)
	}

	// Build project lookup: project_id -> path.
	projectPaths := make(map[string]string)
	for _, p := range syncPayload.Projects {
		projectPaths[p.ID] = p.Path
	}

	// Build PRD lookup: prd_id -> prd path, and prd_id -> project_id.
	prdPaths := make(map[string]string)    // prd_id -> filesystem path to prd.md
	prdProjects := make(map[string]string) // prd_id -> project_id
	for _, p := range syncPayload.PRDs {
		projectDir, ok := projectPaths[p.ProjectID]
		if !ok {
			continue
		}
		prdPath := filepath.Join(projectDir, ".chief", "prds", p.Title, "prd.md")
		prdPaths[p.ID] = prdPath
		prdProjects[p.ID] = p.ProjectID
	}

	// Set up loop manager for running PRDs.
	maxIter := opts.MaxIterations
	if maxIter <= 0 {
		maxIter = 50
	}
	manager := loop.NewManager(maxIter, opts.Provider)
	manager.SetBaseDir(opts.WorkspaceDir)

	// Run tracking: runID -> prdID, so cmd.run.stop can find the right loop.
	var runMu sync.Mutex
	activeRuns := make(map[string]string) // run_id -> prd_id

	// Build WebSocket URL from credentials.
	wsURL := buildWSURL(creds.UplinkURL)

	// Create WebSocket client.
	client := uplink.NewClient(wsURL, creds.AccessToken)

	// Chat session management.
	var chatMu sync.Mutex
	chatSessions := make(map[string]*session.ChatSession) // prd_id -> session

	// Set up command handler.
	handler := uplink.NewHandler(creds.DeviceID)

	// PRD chat: create
	handler.OnPRDCreate(func(cmd protocol.CmdPRDCreate) error {
		projectDir, ok := projectPaths[cmd.ProjectID]
		if !ok {
			return fmt.Errorf("unknown project: %s", cmd.ProjectID)
		}

		chatMu.Lock()
		cs := session.NewChatSession(projectDir, "")
		cs.OnEvent(func(event session.ChatEvent) {
			sendChatOutput(client, creds.DeviceID, cmd.ProjectID, event)
		})
		chatSessions[cmd.ProjectID+":"+cmd.Title] = cs
		chatMu.Unlock()

		return cs.SendMessage(context.Background(), cmd.Content)
	})

	// PRD chat: message
	handler.OnPRDMessage(func(cmd protocol.CmdPRDMessage) error {
		chatMu.Lock()
		var found *session.ChatSession
		for _, cs := range chatSessions {
			if cs.SessionID() != "" {
				found = cs
				break
			}
		}
		chatMu.Unlock()

		if found == nil {
			return fmt.Errorf("no active chat session for PRD: %s", cmd.PRDID)
		}

		return found.SendMessage(context.Background(), cmd.Message)
	})

	// File browsing: files-list
	handler.OnFilesList(func(cmd protocol.CmdFilesList) error {
		projectDir, ok := projectPaths[cmd.ProjectID]
		if !ok {
			return fmt.Errorf("unknown project: %s", cmd.ProjectID)
		}

		targetPath := projectDir
		if cmd.Path != "" {
			targetPath = filepath.Join(projectDir, cmd.Path)
		}

		// Directory traversal protection.
		resolved, err := filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}
		absWorkspace, _ := filepath.Abs(opts.WorkspaceDir)
		if !strings.HasPrefix(resolved, absWorkspace) {
			return fmt.Errorf("path escapes workspace")
		}

		entries, err := os.ReadDir(resolved)
		if err != nil {
			return fmt.Errorf("read directory: %w", err)
		}

		var files []protocol.FileEntry
		for _, e := range entries {
			fe := protocol.FileEntry{
				Name:  e.Name(),
				IsDir: e.IsDir(),
			}
			if !e.IsDir() {
				if info, err := e.Info(); err == nil {
					size := int(info.Size())
					fe.Size = &size
				}
			}
			files = append(files, fe)
		}

		relPath := ""
		if cmd.Path != "" {
			relPath = cmd.Path
		}

		resp := protocol.NewEnvelope(protocol.TypeFilesList, creds.DeviceID)
		payload, _ := json.Marshal(protocol.StateFilesList{
			ProjectID: cmd.ProjectID,
			Path:      relPath,
			Files:     files,
		})
		resp.Payload = payload
		return client.Send(resp)
	})

	// File browsing: file-get
	handler.OnFileGet(func(cmd protocol.CmdFileGet) error {
		projectDir, ok := projectPaths[cmd.ProjectID]
		if !ok {
			return fmt.Errorf("unknown project: %s", cmd.ProjectID)
		}

		targetPath := filepath.Join(projectDir, cmd.Path)

		// Directory traversal protection.
		resolved, err := filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}
		absWorkspace, _ := filepath.Abs(opts.WorkspaceDir)
		if !strings.HasPrefix(resolved, absWorkspace) {
			return fmt.Errorf("path escapes workspace")
		}

		content, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		resp := protocol.NewEnvelope(protocol.TypeFileResponse, creds.DeviceID)
		payload, _ := json.Marshal(protocol.StateFileResponse{
			ProjectID: cmd.ProjectID,
			Path:      cmd.Path,
			Content:   string(content),
			Encoding:  "utf-8",
		})
		resp.Payload = payload
		return client.Send(resp)
	})

	// Diffs: diffs-get
	handler.OnDiffsGet(func(cmd protocol.CmdDiffsGet) error {
		projectDir, ok := projectPaths[cmd.ProjectID]
		if !ok {
			return fmt.Errorf("unknown project: %s", cmd.ProjectID)
		}

		diffs, err := collectDiffs(projectDir)
		if err != nil {
			return err
		}

		resp := protocol.NewEnvelope(protocol.TypeDiffsResponse, creds.DeviceID)
		payload, _ := json.Marshal(protocol.StateDiffsResponse{
			ProjectID: cmd.ProjectID,
			Diffs:     diffs,
		})
		resp.Payload = payload
		return client.Send(resp)
	})

	// Run management: start
	handler.OnRunStart(func(cmd protocol.CmdRunStart) error {
		if opts.Provider == nil {
			return fmt.Errorf("no agent provider configured")
		}

		prdPath, ok := prdPaths[cmd.PRDID]
		if !ok {
			return fmt.Errorf("unknown PRD: %s", cmd.PRDID)
		}

		// Register and start the loop.
		_ = manager.Register(cmd.PRDID, prdPath)
		if err := manager.Start(cmd.PRDID); err != nil {
			return fmt.Errorf("start run: %w", err)
		}

		// Track run_id -> prd_id mapping.
		runMu.Lock()
		activeRuns[cmd.RunID] = cmd.PRDID
		runMu.Unlock()

		// Send state.run-started envelope.
		now := time.Now().UTC().Format(time.RFC3339)
		env := protocol.NewEnvelope(protocol.TypeRunStarted, creds.DeviceID)
		payload, _ := json.Marshal(protocol.StateRunStarted{
			Run: protocol.Run{
				ID:        cmd.RunID,
				PRDID:     cmd.PRDID,
				Status:    "running",
				StartedAt: now,
			},
		})
		env.Payload = payload
		return client.Send(env)
	})

	// Run management: stop
	handler.OnRunStop(func(cmd protocol.CmdRunStop) error {
		runMu.Lock()
		prdID, ok := activeRuns[cmd.RunID]
		if ok {
			delete(activeRuns, cmd.RunID)
		}
		runMu.Unlock()

		if !ok {
			return fmt.Errorf("unknown run: %s", cmd.RunID)
		}

		if err := manager.Stop(prdID); err != nil {
			return fmt.Errorf("stop run: %w", err)
		}

		// Unregister so the PRD can be started again later.
		_ = manager.Unregister(prdID)

		// Send state.run-stopped envelope.
		env := protocol.NewEnvelope(protocol.TypeRunStopped, creds.DeviceID)
		payload, _ := json.Marshal(protocol.StateRunStopped{
			RunID:  cmd.RunID,
			PRDID:  prdID,
			Reason: "stopped by user",
		})
		env.Payload = payload
		return client.Send(env)
	})

	// Wire up message handling.
	client.OnMessage(func(env protocol.Envelope) {
		// Skip control messages.
		if env.Type == protocol.TypeWelcome || env.Type == protocol.TypeAck || env.Type == protocol.TypeError {
			return
		}

		resp := handler.Handle(env)
		if err := client.Send(resp); err != nil {
			fmt.Printf("  Error sending response: %v\n", err)
		}
	})

	// Set up signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start background loops.
	startTime := time.Now()
	go heartbeatLoopWithRunCount(ctx, client, creds.DeviceID, startTime, 30*time.Second, manager)
	go refreshLoop(ctx, opts.HomeDir, client, creds)

	// Forward manager events as state envelopes.
	go forwardManagerEvents(ctx, manager, client, creds.DeviceID, &runMu, activeRuns)

	// Connect with auto-reconnect.
	onConnect := func() {
		// Re-collect and send state.sync on each connect.
		sync, err := collector.Collect()
		if err != nil {
			fmt.Printf("  Error collecting state: %v\n", err)
			return
		}

		env := protocol.NewEnvelope(protocol.TypeSync, creds.DeviceID)
		payload, _ := json.Marshal(sync)
		env.Payload = payload

		if err := client.Send(env); err != nil {
			fmt.Printf("  Error sending state.sync: %v\n", err)
			return
		}

		fmt.Printf("  Connected to Uplink (session: %s)\n", client.SessionID())
	}

	// Run WebSocket in background.
	connErrCh := make(chan error, 1)
	go func() {
		connErrCh <- client.ConnectWithReconnect(onConnect)
	}()

	// Wait for signal or connection failure.
	select {
	case sig := <-sigCh:
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
	case err := <-connErrCh:
		if err != nil {
			return fmt.Errorf("connection error: %w", err)
		}
	}

	cancel()

	// Stop all running loops before closing.
	manager.StopAll()

	if err := client.Close(); err != nil {
		fmt.Printf("  Error closing connection: %v\n", err)
	}

	fmt.Println("chief serve stopped.")
	return nil
}

// loadCredentials loads credentials from the home directory.
func loadCredentials(homeDir string) (*auth.Credentials, error) {
	var creds *auth.Credentials
	var err error

	if homeDir != "" {
		creds, err = auth.LoadCredentialsFrom(homeDir)
	} else {
		creds, err = auth.LoadCredentials()
	}

	if err != nil {
		return nil, fmt.Errorf("not logged in — run `chief login` first: %w", err)
	}

	if creds.IsExpired() {
		// Try to refresh before failing.
		refreshed, refreshErr := tryRefreshToken(homeDir, creds)
		if refreshErr != nil {
			return nil, fmt.Errorf("token expired and refresh failed: %w", refreshErr)
		}
		creds = refreshed
	}

	return creds, nil
}

// tryRefreshToken attempts to refresh an expired access token.
func tryRefreshToken(homeDir string, creds *auth.Credentials) (*auth.Credentials, error) {
	flow := auth.NewDeviceFlow(creds.UplinkURL)
	token, err := flow.RefreshAccessToken(creds.RefreshToken)
	if err != nil {
		return nil, err
	}

	creds.AccessToken = token.AccessToken
	creds.RefreshToken = token.RefreshToken
	creds.Expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	if homeDir != "" {
		err = auth.SaveCredentialsTo(homeDir, creds)
	} else {
		err = auth.SaveCredentials(creds)
	}
	if err != nil {
		return nil, fmt.Errorf("save refreshed credentials: %w", err)
	}

	return creds, nil
}

// refreshLoop periodically checks and refreshes the access token before expiry.
func refreshLoop(ctx context.Context, homeDir string, client *uplink.Client, creds *auth.Credentials) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if creds.NeedsRefresh() {
				refreshed, err := tryRefreshToken(homeDir, creds)
				if err != nil {
					fmt.Printf("  Warning: token refresh failed: %v\n", err)
					continue
				}
				// Update in-memory credentials.
				*creds = *refreshed
				fmt.Println("  Token refreshed successfully.")
			}
		}
	}
}

// runCounter provides the active run count for heartbeats.
type runCounter interface {
	GetRunningCount() int
}

// heartbeatLoop sends a device heartbeat every 30 seconds until the context is cancelled.
func heartbeatLoop(ctx context.Context, client *uplink.Client, deviceID string, startTime time.Time) {
	heartbeatLoopWithRunCount(ctx, client, deviceID, startTime, 30*time.Second, nil)
}

// heartbeatLoopWithInterval is the internal implementation with a configurable interval for testing.
func heartbeatLoopWithInterval(ctx context.Context, client *uplink.Client, deviceID string, startTime time.Time, interval time.Duration) {
	heartbeatLoopWithRunCount(ctx, client, deviceID, startTime, interval, nil)
}

// heartbeatLoopWithRunCount sends heartbeats with the actual active run count from the manager.
func heartbeatLoopWithRunCount(ctx context.Context, client *uplink.Client, deviceID string, startTime time.Time, interval time.Duration, counter runCounter) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			activeRuns := 0
			if counter != nil {
				activeRuns = counter.GetRunningCount()
			}
			uptime := int(time.Since(startTime).Seconds())
			env := protocol.NewEnvelope(protocol.TypeDeviceHeartbeat, deviceID)
			payload, _ := json.Marshal(protocol.StateDeviceHeartbeat{
				UptimeSeconds: uptime,
				ActiveRuns:    activeRuns,
			})
			env.Payload = payload

			if err := client.Send(env); err != nil {
				fmt.Printf("  Warning: heartbeat send failed: %v\n", err)
			}
		}
	}
}

// forwardManagerEvents consumes loop.Manager events and sends corresponding
// state envelopes (run-progress, run-output, run-completed) to the server.
func forwardManagerEvents(ctx context.Context, manager *loop.Manager, client *uplink.Client, deviceID string, runMu *sync.Mutex, activeRuns map[string]string) {
	for {
		select {
		case <-ctx.Done():
			return
		case mEvent, ok := <-manager.Events():
			if !ok {
				return
			}
			prdID := mEvent.PRDName // We use prdID as the manager key.

			// Look up the run_id for this prdID.
			runMu.Lock()
			var runID string
			for rid, pid := range activeRuns {
				if pid == prdID {
					runID = rid
					break
				}
			}
			runMu.Unlock()

			if runID == "" {
				continue // No active run tracking for this PRD.
			}

			switch mEvent.Event.Type {
			case loop.EventIterationStart:
				env := protocol.NewEnvelope(protocol.TypeRunProgress, deviceID)
				payload, _ := json.Marshal(protocol.StateRunProgress{
					RunID:   runID,
					PRDID:   prdID,
					Message: fmt.Sprintf("Iteration %d started (story: %s)", mEvent.Event.Iteration, mEvent.Event.StoryID),
				})
				env.Payload = payload
				_ = client.Send(env)

			case loop.EventAssistantText:
				env := protocol.NewEnvelope(protocol.TypeRunOutput, deviceID)
				payload, _ := json.Marshal(protocol.StateRunOutput{
					RunID:  runID,
					Stream: "stdout",
					Data:   mEvent.Event.Text,
				})
				env.Payload = payload
				_ = client.Send(env)

			case loop.EventToolStart:
				env := protocol.NewEnvelope(protocol.TypeRunOutput, deviceID)
				payload, _ := json.Marshal(protocol.StateRunOutput{
					RunID:  runID,
					Stream: "tool",
					Data:   fmt.Sprintf("Tool: %s", mEvent.Event.Tool),
				})
				env.Payload = payload
				_ = client.Send(env)

			case loop.EventToolResult:
				env := protocol.NewEnvelope(protocol.TypeRunOutput, deviceID)
				payload, _ := json.Marshal(protocol.StateRunOutput{
					RunID:  runID,
					Stream: "tool_result",
					Data:   mEvent.Event.Text,
				})
				env.Payload = payload
				_ = client.Send(env)

			case loop.EventStoryDone:
				env := protocol.NewEnvelope(protocol.TypeRunProgress, deviceID)
				payload, _ := json.Marshal(protocol.StateRunProgress{
					RunID:   runID,
					PRDID:   prdID,
					Message: fmt.Sprintf("Story %s completed", mEvent.Event.StoryID),
				})
				env.Payload = payload
				_ = client.Send(env)

			case loop.EventComplete:
				now := time.Now().UTC().Format(time.RFC3339)
				env := protocol.NewEnvelope(protocol.TypeRunCompleted, deviceID)
				payload, _ := json.Marshal(protocol.StateRunCompleted{
					RunID:      runID,
					PRDID:      prdID,
					Status:     "completed",
					Result:     "success",
					FinishedAt: now,
				})
				env.Payload = payload
				_ = client.Send(env)

				// Clean up run tracking.
				runMu.Lock()
				delete(activeRuns, runID)
				runMu.Unlock()
				_ = manager.Unregister(prdID)

			case loop.EventError:
				now := time.Now().UTC().Format(time.RFC3339)
				errMsg := ""
				if mEvent.Event.Err != nil {
					errMsg = mEvent.Event.Err.Error()
				}
				env := protocol.NewEnvelope(protocol.TypeRunCompleted, deviceID)
				payload, _ := json.Marshal(protocol.StateRunCompleted{
					RunID:      runID,
					PRDID:      prdID,
					Status:     "completed",
					Result:     "error",
					Error:      errMsg,
					FinishedAt: now,
				})
				env.Payload = payload
				_ = client.Send(env)

				// Clean up run tracking.
				runMu.Lock()
				delete(activeRuns, runID)
				runMu.Unlock()
				_ = manager.Unregister(prdID)

			case loop.EventMaxIterationsReached:
				now := time.Now().UTC().Format(time.RFC3339)
				env := protocol.NewEnvelope(protocol.TypeRunCompleted, deviceID)
				payload, _ := json.Marshal(protocol.StateRunCompleted{
					RunID:      runID,
					PRDID:      prdID,
					Status:     "completed",
					Result:     "error",
					Error:      "max iterations reached",
					FinishedAt: now,
				})
				env.Payload = payload
				_ = client.Send(env)

				runMu.Lock()
				delete(activeRuns, runID)
				runMu.Unlock()
				_ = manager.Unregister(prdID)

			case loop.EventRetrying:
				env := protocol.NewEnvelope(protocol.TypeRunProgress, deviceID)
				payload, _ := json.Marshal(protocol.StateRunProgress{
					RunID:   runID,
					PRDID:   prdID,
					Message: mEvent.Event.Text,
				})
				env.Payload = payload
				_ = client.Send(env)

			case loop.EventWatchdogTimeout:
				env := protocol.NewEnvelope(protocol.TypeRunProgress, deviceID)
				payload, _ := json.Marshal(protocol.StateRunProgress{
					RunID:   runID,
					PRDID:   prdID,
					Message: mEvent.Event.Text,
				})
				env.Payload = payload
				_ = client.Send(env)
			}
		}
	}
}

// buildWSURL converts an HTTP(S) URL to a WebSocket URL.
func buildWSURL(httpURL string) string {
	wsURL := strings.TrimRight(httpURL, "/")
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	return wsURL + "/ws"
}

// collectDiffs runs git diff in the project directory and parses the output.
func collectDiffs(projectDir string) ([]protocol.DiffEntry, error) {
	cmd := exec.Command("git", "diff", "--name-status")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	var diffs []protocol.DiffEntry
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		status := "modified"
		switch parts[0] {
		case "A":
			status = "added"
		case "D":
			status = "deleted"
		case "M":
			status = "modified"
		default:
			if strings.HasPrefix(parts[0], "R") {
				status = "renamed"
			}
		}

		diff := protocol.DiffEntry{
			Path:   parts[len(parts)-1],
			Status: status,
		}

		// Get the actual patch for this file.
		patchCmd := exec.Command("git", "diff", "--", diff.Path)
		patchCmd.Dir = projectDir
		if patchOut, err := patchCmd.Output(); err == nil {
			diff.Patch = string(patchOut)
		}

		diffs = append(diffs, diff)
	}

	return diffs, nil
}

// sendChatOutput sends a PRD chat output envelope.
func sendChatOutput(client *uplink.Client, deviceID, prdID string, event session.ChatEvent) {
	env := protocol.NewEnvelope(protocol.TypePRDChatOutput, deviceID)
	payload, _ := json.Marshal(protocol.StatePRDChatOutput{
		PRDID:   prdID,
		Role:    "assistant",
		Content: event.Text,
	})
	env.Payload = payload

	if err := client.Send(env); err != nil {
		fmt.Printf("  Error sending chat output: %v\n", err)
	}
}
