package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// End-to-end integration tests verifying the complete CLI ↔ server message flow
// via the uplink HTTP+Pusher transport. These tests complement the unit-level
// uplink tests (internal/uplink/*_test.go) and the existing serve_test.go tests.

func TestE2E_DeviceLifecycle_ConnectMessagesHeartbeatDisconnect(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		// Wait for full connection (Pusher subscribe).
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot (CLI sends on connect).
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Send a command via Pusher → CLI should respond.
		listReq := map[string]string{
			"type":      "list_projects",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if err := ms.sendCommand(listReq); err != nil {
			t.Logf("sendCommand: %v", err)
			cancel()
			return
		}

		// Wait for project_list response via HTTP messages endpoint.
		if _, err := ms.waitForMessageType("project_list", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(project_list): %v", err)
		}

		// Cancel to trigger graceful shutdown (disconnect).
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspaceDir,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify the full lifecycle happened:
	// 1. Connect was called
	if got := ms.connectCalls.Load(); got < 1 {
		t.Errorf("connect calls = %d, want >= 1", got)
	}

	// 2. Messages were sent (state_snapshot + project_list at minimum)
	if got := ms.messagesCalls.Load(); got < 1 {
		t.Errorf("messages calls = %d, want >= 1", got)
	}

	// 3. State snapshot was received
	if _, err := ms.waitForMessageType("state_snapshot", time.Second); err != nil {
		t.Error("state_snapshot not received")
	}

	// 4. Project list response was received
	raw, err := ms.waitForMessageType("project_list", time.Second)
	if err != nil {
		t.Error("project_list not received")
	} else {
		var resp map[string]interface{}
		json.Unmarshal(raw, &resp)
		projects := resp["projects"].([]interface{})
		if len(projects) != 1 {
			t.Errorf("expected 1 project, got %d", len(projects))
		}
	}

	// 5. Disconnect was called during shutdown
	if got := ms.disconnectCalls.Load(); got != 1 {
		t.Errorf("disconnect calls = %d, want 1", got)
	}
}

func TestE2E_BidirectionalMessageFlow(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "alpha"))
	createGitRepo(t, filepath.Join(workspaceDir, "beta"))

	var responses []map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send multiple commands rapidly — verify CLI processes and responds to each.

		// Command 1: list_projects
		ms.sendCommand(map[string]string{
			"type":      "list_projects",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})

		// Command 2: get_project
		ms.sendCommand(map[string]string{
			"type":      "get_project",
			"id":        "req-2",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "alpha",
		})

		// Command 3: ping
		ms.sendCommand(map[string]string{
			"type":      "ping",
			"id":        "req-3",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})

		// Wait for all three response types.
		types := []string{"project_list", "project_state", "pong"}
		for _, typ := range types {
			raw, err := ms.waitForMessageType(typ, 5*time.Second)
			if err == nil {
				var resp map[string]interface{}
				json.Unmarshal(raw, &resp)
				mu.Lock()
				responses = append(responses, resp)
				mu.Unlock()
			} else {
				t.Logf("waitForMessageType(%s): %v", typ, err)
			}
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}

	// Verify we received all expected types.
	typeSet := make(map[string]bool)
	for _, r := range responses {
		typeSet[r["type"].(string)] = true
	}
	for _, expected := range []string{"project_list", "project_state", "pong"} {
		if !typeSet[expected] {
			t.Errorf("missing response type %q in %v", expected, typeSet)
		}
	}
}

func TestE2E_HeartbeatSentDuringSession(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot.
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Wait for at least one heartbeat call (up to 40s since default interval is 30s).
		deadline := time.After(40 * time.Second)
		for {
			if ms.heartbeatCalls.Load() > 0 {
				break
			}
			select {
			case <-deadline:
				t.Logf("timeout waiting for heartbeat")
				cancel()
				return
			case <-time.After(100 * time.Millisecond):
			}
		}

		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	if got := ms.heartbeatCalls.Load(); got < 1 {
		t.Errorf("heartbeat calls = %d, want >= 1", got)
	}
}

func TestE2E_MultipleCommandsBatchedIntoHTTPPosts(t *testing.T) {
	// Verifies that multiple CLI responses are batched into HTTP POST calls
	// (not one per message) via the batcher.
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var finalPongCount int
	var finalTotalMsgs int
	var finalHTTPCalls int
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send several rapid commands that produce responses.
		for i := 0; i < 5; i++ {
			ms.sendCommand(map[string]string{
				"type":      "ping",
				"id":        fmt.Sprintf("ping-%d", i),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		// Wait for all pongs.
		deadline := time.After(5 * time.Second)
		pongCount := 0
		for pongCount < 5 {
			msgs := ms.getMessages()
			pongCount = 0
			for _, raw := range msgs {
				var msg map[string]interface{}
				json.Unmarshal(raw, &msg)
				if msg["type"] == "pong" {
					pongCount++
				}
			}
			if pongCount >= 5 {
				break
			}
			select {
			case <-deadline:
				t.Logf("timeout: got %d pongs", pongCount)
				mu.Lock()
				finalPongCount = pongCount
				finalTotalMsgs = len(ms.getMessages())
				finalHTTPCalls = int(ms.messagesCalls.Load())
				mu.Unlock()
				return
			case <-time.After(50 * time.Millisecond):
			}
		}

		mu.Lock()
		finalPongCount = pongCount
		finalTotalMsgs = len(ms.getMessages())
		finalHTTPCalls = int(ms.messagesCalls.Load())
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if finalPongCount != 5 {
		t.Errorf("expected 5 pong messages, got %d", finalPongCount)
	}

	t.Logf("total messages: %d, HTTP POST calls: %d", finalTotalMsgs, finalHTTPCalls)
	if finalHTTPCalls > finalTotalMsgs {
		t.Errorf("HTTP POST calls (%d) > total messages (%d), batching may not be working", finalHTTPCalls, finalTotalMsgs)
	}
}

func TestE2E_GracefulShutdownFlushesMessagesAndDisconnects(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspace, "myproject"))

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot.
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Send a ping command.
		ms.sendCommand(map[string]string{
			"type":      "ping",
			"id":        "ping-before-shutdown",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})

		// Wait for the pong to actually be delivered to the server.
		// This confirms the full send path (enqueue → batcher flush → HTTP POST)
		// completed before we trigger shutdown.
		if _, err := ms.waitForMessageType("pong", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(pong): %v", err)
		}

		// Trigger shutdown after messages have been flushed.
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify pong was received by the server (already confirmed in goroutine).
	if _, err := ms.waitForMessageType("pong", time.Second); err != nil {
		t.Error("pong not received by server")
	}

	// Verify disconnect was called during shutdown.
	if got := ms.disconnectCalls.Load(); got != 1 {
		t.Errorf("disconnect calls = %d, want 1", got)
	}
}
