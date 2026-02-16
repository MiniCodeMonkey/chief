package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/update"
	"github.com/minicodemonkey/chief/internal/ws"
)

func TestHandleTriggerUpdate_AlreadyLatest(t *testing.T) {
	// Mock GitHub releases API — same version
	release := update.Release{TagName: "v1.0.0"}
	releaseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer releaseSrv.Close()

	// Set up a WebSocket server to capture messages
	var receivedMsg map[string]interface{}
	var mu sync.Mutex

	upgrader := websocket.Upgrader{}
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &receivedMsg)
			mu.Unlock()
		}
	}))
	defer wsSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http")
	client := ws.New(wsURL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	msg := ws.Message{
		Type: ws.TypeTriggerUpdate,
		ID:   "req-1",
	}

	shouldExit := handleTriggerUpdate(client, msg, "1.0.0", releaseSrv.URL)

	// Give time for the message to be sent
	time.Sleep(100 * time.Millisecond)

	if shouldExit {
		t.Error("should not exit when already on latest version")
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedMsg == nil {
		t.Fatal("expected update_available message to be sent")
	}
	if receivedMsg["type"] != "update_available" {
		t.Errorf("expected type 'update_available', got %v", receivedMsg["type"])
	}
	if receivedMsg["current_version"] != "1.0.0" {
		t.Errorf("expected current_version '1.0.0', got %v", receivedMsg["current_version"])
	}
	if receivedMsg["latest_version"] != "1.0.0" {
		t.Errorf("expected latest_version '1.0.0', got %v", receivedMsg["latest_version"])
	}
}

func TestHandleTriggerUpdate_APIError(t *testing.T) {
	// Mock GitHub releases API — error
	releaseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer releaseSrv.Close()

	// Set up a WebSocket server to capture error message
	var receivedMsg map[string]interface{}
	var mu sync.Mutex

	upgrader := websocket.Upgrader{}
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &receivedMsg)
			mu.Unlock()
		}
	}))
	defer wsSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http")
	client := ws.New(wsURL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	msg := ws.Message{
		Type: ws.TypeTriggerUpdate,
		ID:   "req-1",
	}

	shouldExit := handleTriggerUpdate(client, msg, "1.0.0", releaseSrv.URL)

	time.Sleep(100 * time.Millisecond)

	if shouldExit {
		t.Error("should not exit on API error")
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedMsg == nil {
		t.Fatal("expected error message to be sent")
	}
	if receivedMsg["type"] != "error" {
		t.Errorf("expected type 'error', got %v", receivedMsg["type"])
	}
	if receivedMsg["code"] != "UPDATE_FAILED" {
		t.Errorf("expected code 'UPDATE_FAILED', got %v", receivedMsg["code"])
	}
	if receivedMsg["request_id"] != "req-1" {
		t.Errorf("expected request_id 'req-1', got %v", receivedMsg["request_id"])
	}
}

func TestRunServe_TriggerUpdateAlreadyLatest(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock releases API — same version
	release := update.Release{TagName: "v1.0.0"}
	releaseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer releaseSrv.Close()

	var responseReceived map[string]interface{}
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for initial state_snapshot
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Send trigger_update command via Pusher
		triggerReq := map[string]string{
			"type":      "trigger_update",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		ms.sendCommand(triggerReq)

		// Wait for update_available response
		raw, err := ms.waitForMessageType("update_available", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &responseReceived)
			mu.Unlock()
		}

		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace:   workspaceDir,
		ServerURL:   ms.httpSrv.URL,
		Version:     "1.0.0",
		ReleasesURL: releaseSrv.URL,
		Ctx:         ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if responseReceived == nil {
		t.Fatal("expected response message")
	}
	if responseReceived["type"] != "update_available" {
		t.Errorf("expected type 'update_available', got %v", responseReceived["type"])
	}
	if responseReceived["current_version"] != "1.0.0" {
		t.Errorf("expected current_version '1.0.0', got %v", responseReceived["current_version"])
	}
}

func TestRunServe_TriggerUpdateAPIError(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock releases API — error
	releaseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer releaseSrv.Close()

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for initial state_snapshot
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Send trigger_update command via Pusher
		triggerReq := map[string]string{
			"type":      "trigger_update",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		ms.sendCommand(triggerReq)

		// Wait for error response
		raw, err := ms.waitForMessageType("error", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &errorReceived)
			mu.Unlock()
		}

		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace:   workspaceDir,
		ServerURL:   ms.httpSrv.URL,
		Version:     "1.0.0",
		ReleasesURL: releaseSrv.URL,
		Ctx:         ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if errorReceived == nil {
		t.Fatal("expected error message")
	}
	if errorReceived["type"] != "error" {
		t.Errorf("expected type 'error', got %v", errorReceived["type"])
	}
	if errorReceived["code"] != "UPDATE_FAILED" {
		t.Errorf("expected code 'UPDATE_FAILED', got %v", errorReceived["code"])
	}
}
