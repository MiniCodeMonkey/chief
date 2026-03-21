# Chief CLI Uplink Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add uplink support to the chief CLI — OAuth device flow login, WebSocket connection to the uplink server, state sync, command handling, PRD chat sessions via `claude --resume`, and a `chief serve` daemon that orchestrates everything.

**Architecture:** New packages under `internal/` for auth, uplink client, and session management. New Cobra commands for `login`, `logout`, and `serve`. The serve command connects to the uplink server via WebSocket, pushes state on change, handles commands, and manages PRD chat sessions using Claude Code's `--resume` flag with `--output-format stream-json`.

**Tech Stack:** Go 1.24, `gorilla/websocket`, Cobra CLI, existing `loop.Provider` interface

**Spec:** `docs/superpowers/specs/2026-03-21-uplink-network-protocol-design.md`

**Prerequisite:** Plan 1 (Contract & Protocol) must be completed first — this plan depends on `internal/protocol/`.

---

## File Structure

```
internal/auth/
  device_flow.go                 ← OAuth device flow (request code, poll, store credentials)
  device_flow_test.go
  credentials.go                 ← Read/write ~/.chief/credentials.yaml
  credentials_test.go

internal/uplink/
  client.go                      ← WebSocket client (connect, send, receive, reconnect)
  client_test.go
  handler.go                     ← Command handler (dispatch incoming commands)
  handler_test.go
  state.go                       ← State collector (gather current state for sync)
  state_test.go

internal/session/
  chat.go                        ← PRD chat via claude --resume (spawn, stream, track session ID)
  chat_test.go

internal/config/
  config.go                      ← Modify: add UplinkConfig struct

cmd/chief/commands/
  login.go                       ← chief login command
  logout.go                      ← chief logout command
  serve.go                       ← chief serve command (daemon)
```

---

### Task 1: Config — Add Uplink Configuration

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

```go
// Add to internal/config/config_test.go
func TestLoadUplinkConfig(t *testing.T) {
	dir := t.TempDir()
	chiefDir := filepath.Join(dir, ".chief")
	os.MkdirAll(chiefDir, 0755)

	configYAML := `uplink:
  enabled: true
  url: https://uplink.chiefloop.com
`
	os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte(configYAML), 0644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Uplink.Enabled {
		t.Error("Uplink.Enabled should be true")
	}
	if cfg.Uplink.URL != "https://uplink.chiefloop.com" {
		t.Errorf("Uplink.URL = %q, want %q", cfg.Uplink.URL, "https://uplink.chiefloop.com")
	}
}

func TestUplinkConfigDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Uplink.Enabled {
		t.Error("Uplink should be disabled by default")
	}
	if cfg.Uplink.URL != "https://uplink.chiefloop.com" {
		t.Errorf("Default URL = %q, want %q", cfg.Uplink.URL, "https://uplink.chiefloop.com")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/config/ -v -run "TestLoadUplink|TestUplinkConfig"
```

- [ ] **Step 3: Add UplinkConfig to config.go**

Add to the existing `Config` struct:

```go
type UplinkConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

type Config struct {
	Worktree   WorktreeConfig   `yaml:"worktree"`
	OnComplete OnCompleteConfig `yaml:"onComplete"`
	Agent      AgentConfig      `yaml:"agent"`
	Uplink     UplinkConfig     `yaml:"uplink"`
}
```

Update `Default()` to set the default URL:

```go
func Default() *Config {
	return &Config{
		Uplink: UplinkConfig{
			URL: "https://uplink.chiefloop.com",
		},
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/config/ -v
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add uplink configuration (enabled, url)"
```

---

### Task 2: Auth — Credentials Storage

**Files:**
- Create: `internal/auth/credentials.go`
- Create: `internal/auth/credentials_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/auth/credentials_test.go
package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCredentialsSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "credentials.yaml")

	creds := &Credentials{
		AccessToken:  "test_access_token",
		RefreshToken: "test_refresh_token",
		ExpiresAt:    time.Now().Add(time.Hour),
		DeviceName:   "test-device",
		DeviceID:     "dev_test123",
		UplinkURL:    "https://uplink.chiefloop.com",
	}

	if err := SaveCredentials(credPath, creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	loaded, err := LoadCredentials(credPath)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}

	if loaded.AccessToken != creds.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, creds.AccessToken)
	}
	if loaded.DeviceID != creds.DeviceID {
		t.Errorf("DeviceID = %q, want %q", loaded.DeviceID, creds.DeviceID)
	}
}

func TestCredentialsNotFound(t *testing.T) {
	_, err := LoadCredentials("/nonexistent/path")
	if err == nil {
		t.Error("LoadCredentials should return error for missing file")
	}
}

func TestCredentialsExpired(t *testing.T) {
	creds := &Credentials{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if !creds.IsExpired() {
		t.Error("Should be expired")
	}

	creds.ExpiresAt = time.Now().Add(time.Hour)
	if creds.IsExpired() {
		t.Error("Should not be expired")
	}
}

func TestCredentialsFilePermissions(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "credentials.yaml")

	creds := &Credentials{AccessToken: "secret"}
	SaveCredentials(credPath, creds)

	info, _ := os.Stat(credPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("Permissions = %o, want 0600", info.Mode().Perm())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/auth/ -v
```

- [ ] **Step 3: Implement credentials.go**

```go
// internal/auth/credentials.go
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Credentials stores OAuth tokens and device identity.
type Credentials struct {
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	ExpiresAt    time.Time `yaml:"expires_at"`
	DeviceName   string    `yaml:"device_name"`
	DeviceID     string    `yaml:"device_id"`
	UplinkURL    string    `yaml:"uplink_url"`
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// NeedsRefresh returns true if the token expires within 5 minutes.
func (c *Credentials) NeedsRefresh() bool {
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

// DefaultCredentialsPath returns ~/.chief/credentials.yaml.
func DefaultCredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".chief", "credentials.yaml"), nil
}

// LoadCredentials reads credentials from a YAML file.
func LoadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds Credentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	return &creds, nil
}

// SaveCredentials writes credentials to a YAML file with 0600 permissions.
func SaveCredentials(path string, creds *Credentials) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

// DeleteCredentials removes the credentials file.
func DeleteCredentials(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete credentials: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/auth/ -v
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/credentials.go internal/auth/credentials_test.go
git commit -m "feat(auth): add credentials storage (save, load, expiry check)"
```

---

### Task 3: Auth — OAuth Device Flow

**Files:**
- Create: `internal/auth/device_flow.go`
- Create: `internal/auth/device_flow_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/auth/device_flow_test.go
package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeviceFlowRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/device/request" {
			t.Errorf("Path = %q, want /api/auth/device/request", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}

		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode: "dev_code_123",
			UserCode:   "ABCD-1234",
			VerifyURL:  server.URL + "/activate",
			ExpiresIn:  900,
			Interval:   5,
		})
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	resp, err := flow.RequestCode()
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}

	if resp.UserCode != "ABCD-1234" {
		t.Errorf("UserCode = %q, want %q", resp.UserCode, "ABCD-1234")
	}
	if resp.DeviceCode == "" {
		t.Error("DeviceCode should not be empty")
	}
}

func TestDeviceFlowPollSuccess(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "access_123",
			RefreshToken: "refresh_456",
			ExpiresIn:    3600,
			DeviceID:     "dev_abc",
		})
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	// Use 0 interval for fast tests
	resp, err := flow.PollForToken("dev_code_123", 0)
	if err != nil {
		t.Fatalf("PollForToken: %v", err)
	}

	if resp.AccessToken != "access_123" {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, "access_123")
	}
	if resp.DeviceID != "dev_abc" {
		t.Errorf("DeviceID = %q, want %q", resp.DeviceID, "dev_abc")
	}
}

func TestDeviceFlowPollDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	_, err := flow.PollForToken("dev_code_123", 0)
	if err == nil {
		t.Error("PollForToken should fail when denied")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/auth/ -v -run TestDeviceFlow
```

- [ ] **Step 3: Implement device_flow.go**

```go
// internal/auth/device_flow.go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// DeviceCodeResponse is returned by the device code request endpoint.
type DeviceCodeResponse struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	VerifyURL  string `json:"verify_url"`
	ExpiresIn  int    `json:"expires_in"`
	Interval   int    `json:"interval"`
}

// TokenResponse is returned when the device is authorized.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	DeviceID     string `json:"device_id"`
}

// DeviceFlow handles the OAuth device authorization flow.
type DeviceFlow struct {
	baseURL string
	client  *http.Client
}

// NewDeviceFlow creates a device flow client for the given uplink URL.
func NewDeviceFlow(baseURL string) *DeviceFlow {
	return &DeviceFlow{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// RequestCode requests a device code and user code from the server.
func (f *DeviceFlow) RequestCode() (*DeviceCodeResponse, error) {
	hostname, _ := os.Hostname()
	body := fmt.Sprintf(`{"device_name":"%s"}`, hostname)

	resp, err := f.client.Post(
		f.baseURL+"/api/auth/device/request",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed: %s", resp.Status)
	}

	var result DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}

	return &result, nil
}

// PollForToken polls the verification endpoint until the user approves or denies.
// interval is in seconds (0 means no delay, for testing).
func (f *DeviceFlow) PollForToken(deviceCode string, interval int) (*TokenResponse, error) {
	body := fmt.Sprintf(`{"device_code":"%s"}`, deviceCode)

	for {
		resp, err := f.client.Post(
			f.baseURL+"/api/auth/device/verify",
			"application/json",
			strings.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("poll for token: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			var result TokenResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("decode token response: %w", err)
			}
			resp.Body.Close()
			return &result, nil

		case http.StatusAccepted:
			// Still pending, continue polling
			resp.Body.Close()

		case http.StatusForbidden:
			resp.Body.Close()
			return nil, fmt.Errorf("device authorization denied")

		default:
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status: %s", resp.Status)
		}

		if interval > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}

// RefreshAccessToken uses a refresh token to get a new access token.
func (f *DeviceFlow) RefreshAccessToken(refreshToken string) (*TokenResponse, error) {
	body := fmt.Sprintf(`{"refresh_token":"%s"}`, refreshToken)

	resp, err := f.client.Post(
		f.baseURL+"/api/auth/token/refresh",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %s", resp.Status)
	}

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	return &result, nil
}

// RevokeDevice revokes the device's access.
func (f *DeviceFlow) RevokeDevice(accessToken, deviceID string) error {
	req, _ := http.NewRequest("DELETE", f.baseURL+"/api/auth/device/"+deviceID, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("revoke device: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("revoke failed: %s", resp.Status)
	}

	return nil
}
```

Note: Add `"os"` to the import block.

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/auth/ -v
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/device_flow.go internal/auth/device_flow_test.go
git commit -m "feat(auth): add OAuth device flow (request, poll, refresh, revoke)"
```

---

### Task 4: Uplink — WebSocket Client

**Files:**
- Create: `internal/uplink/client.go`
- Create: `internal/uplink/client_test.go`

- [ ] **Step 1: Add dependency**

```bash
cd /Users/codemonkey/projects/chief && go get github.com/gorilla/websocket
```

- [ ] **Step 2: Write failing test**

```go
// internal/uplink/client_test.go
package uplink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestClientConnectAndSend(t *testing.T) {
	received := make(chan protocol.Envelope, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test_token" {
			t.Errorf("Auth header = %q, want %q", auth, "Bearer test_token")
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade: %v", err)
			return
		}
		defer conn.Close()

		// Send welcome
		welcome := protocol.NewEnvelope(protocol.TypeWelcome, "server", protocol.Welcome{
			SessionID:     "sess_test",
			ServerVersion: "1.0.0",
			Capabilities:  []string{"state_sync", "commands"},
		})
		data, _ := welcome.Marshal()
		conn.WriteMessage(websocket.TextMessage, data)

		// Read one message
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		env, _ := protocol.ParseEnvelope(msg)
		received <- *env
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/device"

	client := NewClient(wsURL, "test_token", "dev_test")
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	// Wait for welcome
	select {
	case <-client.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for welcome")
	}

	// Send a message
	env := protocol.NewEnvelope(protocol.TypeStateDeviceHeartbeat, "dev_test", protocol.StateDeviceHeartbeat{})
	if err := client.Send(env); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case msg := <-received:
		if msg.Type != protocol.TypeStateDeviceHeartbeat {
			t.Errorf("Type = %q, want %q", msg.Type, protocol.TypeStateDeviceHeartbeat)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestClientReceiveCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send welcome
		welcome := protocol.NewEnvelope(protocol.TypeWelcome, "server", protocol.Welcome{
			SessionID: "sess_test", ServerVersion: "1.0.0", Capabilities: []string{},
		})
		data, _ := welcome.Marshal()
		conn.WriteMessage(websocket.TextMessage, data)

		// Send a command
		cmd := protocol.NewEnvelope(protocol.TypeCmdRunStart, "server", protocol.CmdRunStart{
			PRDID: "prd_test",
		})
		data, _ = cmd.Marshal()
		conn.WriteMessage(websocket.TextMessage, data)

		// Keep connection alive briefly
		time.Sleep(time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/device"
	client := NewClient(wsURL, "test_token", "dev_test")

	received := make(chan protocol.Envelope, 1)
	client.OnMessage(func(env *protocol.Envelope) {
		if env.Type != protocol.TypeWelcome {
			received <- *env
		}
	})

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	select {
	case msg := <-received:
		if msg.Type != protocol.TypeCmdRunStart {
			t.Errorf("Type = %q, want %q", msg.Type, protocol.TypeCmdRunStart)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for command")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/uplink/ -v -run TestClient
```

- [ ] **Step 4: Implement client.go**

```go
// internal/uplink/client.go
package uplink

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/protocol"
)

// Client manages the WebSocket connection to the uplink server.
type Client struct {
	url       string
	token     string
	deviceID  string
	conn      *websocket.Conn
	mu        sync.Mutex
	ready     chan struct{}
	done      chan struct{}
	onMessage func(*protocol.Envelope)
	sessionID string
}

// NewClient creates a new uplink WebSocket client.
func NewClient(url, token, deviceID string) *Client {
	return &Client{
		url:      url,
		token:    token,
		deviceID: deviceID,
		ready:    make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// OnMessage registers a callback for incoming messages.
func (c *Client) OnMessage(handler func(*protocol.Envelope)) {
	c.onMessage = handler
}

// Ready returns a channel that closes when the welcome message is received.
func (c *Client) Ready() <-chan struct{} {
	return c.ready
}

// Connect establishes the WebSocket connection.
func (c *Client) Connect() error {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.token)

	conn, _, err := websocket.DefaultDialer.Dial(c.url, header)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	// Reset channels for reconnection support
	c.ready = make(chan struct{})
	c.done = make(chan struct{})
	c.mu.Unlock()

	go c.readLoop()
	return nil
}

// Send sends an envelope over the WebSocket.
func (c *Client) Send(env protocol.Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := env.Marshal()
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.done)
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SessionID returns the session ID from the welcome message.
func (c *Client) SessionID() string {
	return c.sessionID
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		env, err := protocol.ParseEnvelope(data)
		if err != nil {
			log.Printf("uplink: invalid message: %v", err)
			continue
		}

		// Handle welcome internally
		if env.Type == protocol.TypeWelcome {
			welcome, err := protocol.DecodePayload[protocol.Welcome](env)
			if err == nil {
				c.sessionID = welcome.SessionID
			}
			select {
			case <-c.ready:
			default:
				close(c.ready)
			}
		}

		if c.onMessage != nil {
			c.onMessage(env)
		}
	}
}

// ConnectWithReconnect connects and automatically reconnects on disconnect.
// It calls onConnect each time a connection is established (for sending state.sync).
func (c *Client) ConnectWithReconnect(onConnect func() error) {
	go func() {
		attempt := 0
		for {
			select {
			case <-c.done:
				return
			default:
			}

			if err := c.Connect(); err != nil {
				attempt++
				delay := backoffDelay(attempt)
				log.Printf("uplink: connect failed (attempt %d), retrying in %v: %v", attempt, delay, err)
				time.Sleep(delay)
				continue
			}

			attempt = 0

			// Wait for welcome then call onConnect
			select {
			case <-c.Ready():
				if onConnect != nil {
					if err := onConnect(); err != nil {
						log.Printf("uplink: onConnect failed: %v", err)
					}
				}
			case <-c.done:
				return
			}

			// Block until connection drops
			<-c.done
		}
	}()
}

// backoffDelay returns exponential backoff with jitter, capped at 60s.
func backoffDelay(attempt int) time.Duration {
	base := math.Min(float64(attempt)*float64(attempt), 60)
	jitter := rand.Float64() * base * 0.5
	return time.Duration(base+jitter) * time.Second
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/uplink/ -v -run TestClient
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/uplink/client.go internal/uplink/client_test.go go.mod go.sum
git commit -m "feat(uplink): add WebSocket client with reconnection"
```

---

### Task 5: Uplink — State Collector

**Files:**
- Create: `internal/uplink/state.go`
- Create: `internal/uplink/state_test.go`

This package gathers the current state from the filesystem (projects, PRDs, runs) to build a `StateSync` payload.

- [ ] **Step 1: Write failing test**

```go
// internal/uplink/state_test.go
package uplink

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minicodemonkey/chief/internal/protocol"
)

func TestCollectState(t *testing.T) {
	// Set up a fake workspace
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "myapp")
	os.MkdirAll(projectDir, 0755)

	// Create a .chief dir with a PRD
	chiefDir := filepath.Join(projectDir, ".chief", "prds", "auth")
	os.MkdirAll(chiefDir, 0755)
	os.WriteFile(filepath.Join(chiefDir, "prd.md"), []byte("# Auth PRD\n## User Stories\n### US-001: Login\n"), 0644)

	// Initialize git repo
	os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)

	collector := NewStateCollector("dev_test", "0.5.0")
	collector.AddProject(projectDir)

	state, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if state.Device.ChiefVersion != "0.5.0" {
		t.Errorf("ChiefVersion = %q, want %q", state.Device.ChiefVersion, "0.5.0")
	}
	if len(state.Projects) != 1 {
		t.Fatalf("len(Projects) = %d, want 1", len(state.Projects))
	}
	if state.Projects[0].Name != "myapp" {
		t.Errorf("Project.Name = %q, want %q", state.Projects[0].Name, "myapp")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/uplink/ -v -run TestCollect
```

- [ ] **Step 3: Implement state.go**

```go
// internal/uplink/state.go
package uplink

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/minicodemonkey/chief/internal/prd"
	"github.com/minicodemonkey/chief/internal/protocol"
)

// StateCollector gathers filesystem state into a protocol.StateSync payload.
type StateCollector struct {
	deviceID     string
	chiefVersion string
	projectDirs  []string
}

// NewStateCollector creates a new state collector.
func NewStateCollector(deviceID, chiefVersion string) *StateCollector {
	return &StateCollector{
		deviceID:     deviceID,
		chiefVersion: chiefVersion,
	}
}

// AddProject registers a project directory for state collection.
func (sc *StateCollector) AddProject(dir string) {
	sc.projectDirs = append(sc.projectDirs, dir)
}

// Collect gathers the current state from all registered projects.
func (sc *StateCollector) Collect() (*protocol.StateSync, error) {
	hostname, _ := os.Hostname()

	state := &protocol.StateSync{
		Device: protocol.DeviceInfo{
			Name:         hostname,
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			ChiefVersion: sc.chiefVersion,
		},
		Projects: make([]protocol.Project, 0),
		PRDs:     make([]protocol.PRD, 0),
		Runs:     make([]protocol.Run, 0),
	}

	for _, dir := range sc.projectDirs {
		proj := sc.collectProject(dir)
		state.Projects = append(state.Projects, proj)

		prds := sc.collectPRDs(dir, proj.ID)
		state.PRDs = append(state.PRDs, prds...)
	}

	return state, nil
}

func (sc *StateCollector) collectProject(dir string) protocol.Project {
	name := filepath.Base(dir)
	proj := protocol.Project{
		ID:   "proj_" + name,
		Path: dir,
		Name: name,
	}

	// Get git info
	if branch, err := gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		proj.GitBranch = branch
	}
	if remote, err := gitOutput(dir, "remote", "get-url", "origin"); err == nil {
		proj.GitRemote = remote
	}
	if status, err := gitOutput(dir, "status", "--porcelain"); err == nil {
		if strings.TrimSpace(status) == "" {
			proj.GitStatus = protocol.GitStatusClean
		} else {
			proj.GitStatus = protocol.GitStatusDirty
		}
	}
	if hash, err := gitOutput(dir, "log", "-1", "--format=%H"); err == nil {
		msg, _ := gitOutput(dir, "log", "-1", "--format=%s")
		ts, _ := gitOutput(dir, "log", "-1", "--format=%aI")
		proj.LastCommit = &protocol.GitCommit{
			Hash:      hash,
			Message:   msg,
			Timestamp: ts,
		}
	}

	return proj
}

func (sc *StateCollector) collectPRDs(projectDir, projectID string) []protocol.PRD {
	prdsDir := filepath.Join(projectDir, ".chief", "prds")
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		return nil
	}

	var result []protocol.PRD
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		prdPath := filepath.Join(prdsDir, entry.Name(), "prd.md")
		content, err := os.ReadFile(prdPath)
		if err != nil {
			continue
		}

		p := protocol.PRD{
			ID:        "prd_" + entry.Name(),
			ProjectID: projectID,
			Title:     entry.Name(),
			Status:    protocol.PRDStatusReady,
			Content:   string(content),
		}

		// Read progress if exists
		progressPath := filepath.Join(prdsDir, entry.Name(), "progress.md")
		if progress, err := os.ReadFile(progressPath); err == nil {
			p.Progress = string(progress)
		}

		result = append(result, p)
	}

	return result
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/uplink/ -v -run TestCollect
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/uplink/state.go internal/uplink/state_test.go
git commit -m "feat(uplink): add state collector for filesystem state sync"
```

---

### Task 6: Uplink — Command Handler

**Files:**
- Create: `internal/uplink/handler.go`
- Create: `internal/uplink/handler_test.go`

The handler dispatches incoming commands from the server to the appropriate action (start run, PRD chat, get diffs, etc.).

- [ ] **Step 1: Write failing test**

```go
// internal/uplink/handler_test.go
package uplink

import (
	"testing"

	"github.com/minicodemonkey/chief/internal/protocol"
)

func TestHandlerDispatchRunStart(t *testing.T) {
	var receivedPRDID string

	h := NewHandler()
	h.OnRunStart(func(cmd protocol.CmdRunStart) error {
		receivedPRDID = cmd.PRDID
		return nil
	})

	env := protocol.NewEnvelope(protocol.TypeCmdRunStart, "server", protocol.CmdRunStart{
		PRDID: "prd_test",
	})

	ack, err := h.Handle(&env)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if receivedPRDID != "prd_test" {
		t.Errorf("PRDID = %q, want %q", receivedPRDID, "prd_test")
	}
	if ack.Type != protocol.TypeAck {
		t.Errorf("response type = %q, want ack", ack.Type)
	}
}

func TestHandlerUnknownCommand(t *testing.T) {
	h := NewHandler()
	env := protocol.NewEnvelope("cmd.unknown", "server", nil)

	resp, _ := h.Handle(&env)
	if resp.Type != protocol.TypeError {
		t.Errorf("response type = %q, want error", resp.Type)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/uplink/ -v -run TestHandler
```

- [ ] **Step 3: Implement handler.go**

```go
// internal/uplink/handler.go
package uplink

import (
	"fmt"

	"github.com/minicodemonkey/chief/internal/protocol"
)

// Handler dispatches incoming commands to registered callbacks.
type Handler struct {
	deviceID       string
	onRunStart     func(protocol.CmdRunStart) error
	onRunStop      func(protocol.CmdRunStop) error
	onPRDCreate    func(protocol.CmdPRDCreate) error
	onPRDMessage   func(protocol.CmdPRDMessage) error
	onPRDUpdate    func(protocol.CmdPRDUpdate) error
	onPRDDelete    func(protocol.CmdPRDDelete) error
	onProjectClone func(protocol.CmdProjectClone) error
	onDiffsGet     func(protocol.CmdDiffsGet) error
	onLogGet       func(protocol.CmdLogGet) error
	onFilesList    func(protocol.CmdFilesList) error
	onFileGet      func(protocol.CmdFileGet) error
	onSettingsGet  func(protocol.CmdSettingsGet) error
	onSettingsUpdate func(protocol.CmdSettingsUpdate) error
}

// NewHandler creates a new command handler.
func NewHandler() *Handler {
	return &Handler{}
}

// SetDeviceID sets the device ID for response envelopes.
func (h *Handler) SetDeviceID(id string) {
	h.deviceID = id
}

// Callback registration methods

func (h *Handler) OnRunStart(f func(protocol.CmdRunStart) error)         { h.onRunStart = f }
func (h *Handler) OnRunStop(f func(protocol.CmdRunStop) error)           { h.onRunStop = f }
func (h *Handler) OnPRDCreate(f func(protocol.CmdPRDCreate) error)       { h.onPRDCreate = f }
func (h *Handler) OnPRDMessage(f func(protocol.CmdPRDMessage) error)     { h.onPRDMessage = f }
func (h *Handler) OnPRDUpdate(f func(protocol.CmdPRDUpdate) error)       { h.onPRDUpdate = f }
func (h *Handler) OnPRDDelete(f func(protocol.CmdPRDDelete) error)       { h.onPRDDelete = f }
func (h *Handler) OnProjectClone(f func(protocol.CmdProjectClone) error) { h.onProjectClone = f }
func (h *Handler) OnDiffsGet(f func(protocol.CmdDiffsGet) error)         { h.onDiffsGet = f }
func (h *Handler) OnLogGet(f func(protocol.CmdLogGet) error)             { h.onLogGet = f }
func (h *Handler) OnFilesList(f func(protocol.CmdFilesList) error)       { h.onFilesList = f }
func (h *Handler) OnFileGet(f func(protocol.CmdFileGet) error)           { h.onFileGet = f }
func (h *Handler) OnSettingsGet(f func(protocol.CmdSettingsGet) error)   { h.onSettingsGet = f }
func (h *Handler) OnSettingsUpdate(f func(protocol.CmdSettingsUpdate) error) { h.onSettingsUpdate = f }

// Handle processes an incoming command envelope and returns an ack or error response.
func (h *Handler) Handle(env *protocol.Envelope) (protocol.Envelope, error) {
	var err error

	switch env.Type {
	case protocol.TypeCmdRunStart:
		err = dispatch(env, h.onRunStart)
	case protocol.TypeCmdRunStop:
		err = dispatch(env, h.onRunStop)
	case protocol.TypeCmdPRDCreate:
		err = dispatch(env, h.onPRDCreate)
	case protocol.TypeCmdPRDMessage:
		err = dispatch(env, h.onPRDMessage)
	case protocol.TypeCmdPRDUpdate:
		err = dispatch(env, h.onPRDUpdate)
	case protocol.TypeCmdPRDDelete:
		err = dispatch(env, h.onPRDDelete)
	case protocol.TypeCmdProjectClone:
		err = dispatch(env, h.onProjectClone)
	case protocol.TypeCmdDiffsGet:
		err = dispatch(env, h.onDiffsGet)
	case protocol.TypeCmdLogGet:
		err = dispatch(env, h.onLogGet)
	case protocol.TypeCmdFilesList:
		err = dispatch(env, h.onFilesList)
	case protocol.TypeCmdFileGet:
		err = dispatch(env, h.onFileGet)
	case protocol.TypeCmdSettingsGet:
		err = dispatch(env, h.onSettingsGet)
	case protocol.TypeCmdSettingsUpdate:
		err = dispatch(env, h.onSettingsUpdate)
	default:
		return protocol.NewEnvelope(protocol.TypeError, h.deviceID, protocol.Error{
			RefID:   env.ID,
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown command type: %s", env.Type),
		}), nil
	}

	if err != nil {
		return protocol.NewEnvelope(protocol.TypeError, h.deviceID, protocol.Error{
			RefID:   env.ID,
			Code:    "command_failed",
			Message: err.Error(),
		}), nil
	}

	return protocol.NewEnvelope(protocol.TypeAck, h.deviceID, protocol.Ack{
		RefID: env.ID,
	}), nil
}

// dispatch decodes the payload and calls the handler, or returns an error if no handler is registered.
func dispatch[T any](env *protocol.Envelope, handler func(T) error) error {
	if handler == nil {
		return fmt.Errorf("no handler registered for %s", env.Type)
	}

	payload, err := protocol.DecodePayload[T](env)
	if err != nil {
		return fmt.Errorf("decode %s payload: %w", env.Type, err)
	}

	return handler(*payload)
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/uplink/ -v -run TestHandler
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/uplink/handler.go internal/uplink/handler_test.go
git commit -m "feat(uplink): add command handler with typed dispatch"
```

---

### Task 7: Session — PRD Chat via claude --resume

**Files:**
- Create: `internal/session/chat.go`
- Create: `internal/session/chat_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/session/chat_test.go
package session

import (
	"context"
	"testing"
)

func TestChatSessionBuildArgs(t *testing.T) {
	cs := NewChatSession("/usr/local/bin/claude", "/home/user/project")

	// First turn — no session ID
	args := cs.buildArgs("Create a REST API for users")
	expected := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--dir", "/home/user/project",
		"-p", "Create a REST API for users",
	}

	if len(args) != len(expected) {
		t.Fatalf("len(args) = %d, want %d: %v", len(args), len(expected), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}

	// Set session ID (simulating after first turn)
	cs.sessionID = "sess_abc123"
	args = cs.buildArgs("Add OAuth support")

	// Should include --resume
	hasResume := false
	for i, a := range args {
		if a == "--resume" && i+1 < len(args) && args[i+1] == "sess_abc123" {
			hasResume = true
		}
	}
	if !hasResume {
		t.Errorf("args should include --resume sess_abc123: %v", args)
	}
}

func TestChatSessionParseSessionID(t *testing.T) {
	line := `{"type":"result","subtype":"success","session_id":"de263703-4d83-40ba-8574-d1c30ef1acc6","result":"Done"}`

	cs := NewChatSession("claude", "/tmp")
	cs.processLine(line)

	if cs.sessionID != "de263703-4d83-40ba-8574-d1c30ef1acc6" {
		t.Errorf("sessionID = %q, want %q", cs.sessionID, "de263703-4d83-40ba-8574-d1c30ef1acc6")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/session/ -v
```

- [ ] **Step 3: Implement chat.go**

```go
// internal/session/chat.go
package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/minicodemonkey/chief/internal/protocol"
)

// EventCallback is called for each parsed event during a chat turn.
type EventCallback func(event protocol.StatePRDChatOutput)

// ChatSession manages a multi-turn PRD chat conversation using claude --resume.
type ChatSession struct {
	cliPath   string
	workDir   string
	sessionID string
	onEvent   EventCallback
}

// NewChatSession creates a new chat session.
func NewChatSession(cliPath, workDir string) *ChatSession {
	return &ChatSession{
		cliPath: cliPath,
		workDir: workDir,
	}
}

// SessionID returns the Claude Code session ID (available after the first turn).
func (cs *ChatSession) SessionID() string {
	return cs.sessionID
}

// SetSessionID restores a session ID from previous state.
func (cs *ChatSession) SetSessionID(id string) {
	cs.sessionID = id
}

// OnEvent registers a callback for streaming events.
func (cs *ChatSession) OnEvent(cb EventCallback) {
	cs.onEvent = cb
}

// SendMessage sends a message and streams the response.
// For the first turn, pass the full init prompt + user message.
// For subsequent turns, pass just the user message (session is resumed automatically).
func (cs *ChatSession) SendMessage(ctx context.Context, message string) (string, error) {
	args := cs.buildArgs(message)
	cmd := exec.CommandContext(ctx, cs.cliPath, args...)
	cmd.Dir = cs.workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start claude: %w", err)
	}

	var resultText string
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		resultText = cs.processLine(line)
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("claude exited with error: %w", err)
	}

	return resultText, nil
}

// buildArgs constructs the CLI arguments for a chat turn.
func (cs *ChatSession) buildArgs(message string) []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--dir", cs.workDir,
	}

	if cs.sessionID != "" {
		args = append(args, "--resume", cs.sessionID)
	}

	args = append(args, "-p", message)
	return args
}

// processLine parses a single NDJSON line from Claude's output.
// Returns the result text if this is a result line, empty string otherwise.
func (cs *ChatSession) processLine(line string) string {
	var msg struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Result    string `json:"result"`
		Message   *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}

	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return ""
	}

	switch msg.Type {
	case "assistant":
		if msg.Message != nil && cs.onEvent != nil {
			for _, c := range msg.Message.Content {
				if c.Type == "text" {
					cs.onEvent(protocol.StatePRDChatOutput{
						EventType: "assistant_text",
						Text:      c.Text,
					})
				}
			}
		}

	case "result":
		if msg.SessionID != "" {
			cs.sessionID = msg.SessionID
		}
		return msg.Result
	}

	return ""
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/session/ -v
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/chat.go internal/session/chat_test.go
git commit -m "feat(session): add PRD chat via claude --resume with stream-json"
```

---

### Task 8: CLI Commands — Login & Logout

**Files:**
- Create: `cmd/chief/commands/login.go`
- Create: `cmd/chief/commands/logout.go`
- Modify: `cmd/chief/commands/root.go` (register subcommands)

- [ ] **Step 1: Implement login.go**

```go
// cmd/chief/commands/login.go
package commands

import (
	"fmt"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/minicodemonkey/chief/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Chief Uplink",
	Long:  "Start the OAuth device flow to connect this machine to your Uplink account.",
	RunE:  runLogin,
}

func init() {
	loginCmd.Flags().String("url", "", "Uplink server URL (overrides config)")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Determine uplink URL
	urlFlag, _ := cmd.Flags().GetString("url")
	uplinkURL := urlFlag

	if uplinkURL == "" {
		cfg, err := config.Load(".")
		if err == nil && cfg.Uplink.URL != "" {
			uplinkURL = cfg.Uplink.URL
		} else {
			uplinkURL = "https://uplink.chiefloop.com"
		}
	}

	fmt.Printf("Connecting to %s...\n", uplinkURL)

	flow := auth.NewDeviceFlow(uplinkURL)

	// Request device code
	codeResp, err := flow.RequestCode()
	if err != nil {
		return fmt.Errorf("failed to request device code: %w", err)
	}

	fmt.Printf("\nVisit %s and enter code: %s\n\n", codeResp.VerifyURL, codeResp.UserCode)
	fmt.Println("Waiting for approval...")

	// Poll for token
	tokenResp, err := flow.PollForToken(codeResp.DeviceCode, codeResp.Interval)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Save credentials
	credPath, err := auth.DefaultCredentialsPath()
	if err != nil {
		return err
	}

	creds := &auth.Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		DeviceID:     tokenResp.DeviceID,
		UplinkURL:    uplinkURL,
	}

	if err := auth.SaveCredentials(credPath, creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("✓ Authenticated as device %s\n", tokenResp.DeviceID)
	return nil
}
```

- [ ] **Step 2: Implement logout.go**

```go
// cmd/chief/commands/logout.go
package commands

import (
	"fmt"

	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Disconnect from Chief Uplink",
	Long:  "Revoke this device's access and delete stored credentials.",
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, args []string) error {
	credPath, err := auth.DefaultCredentialsPath()
	if err != nil {
		return err
	}

	creds, err := auth.LoadCredentials(credPath)
	if err != nil {
		return fmt.Errorf("not logged in (no credentials found)")
	}

	// Revoke on server
	flow := auth.NewDeviceFlow(creds.UplinkURL)
	if err := flow.RevokeDevice(creds.AccessToken, creds.DeviceID); err != nil {
		fmt.Printf("Warning: could not revoke on server: %v\n", err)
	}

	// Delete local credentials
	if err := auth.DeleteCredentials(credPath); err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	fmt.Println("✓ Logged out and credentials deleted")
	return nil
}
```

- [ ] **Step 3: Register commands in root.go**

Add to the `init()` function in `cmd/chief/commands/root.go`:

```go
rootCmd.AddCommand(loginCmd)
rootCmd.AddCommand(logoutCmd)
```

- [ ] **Step 4: Build and verify help text**

```bash
cd /Users/codemonkey/projects/chief && go build -o /tmp/chief-test ./cmd/chief && /tmp/chief-test --help
```
Expected: `login` and `logout` appear in subcommands

- [ ] **Step 5: Commit**

```bash
git add cmd/chief/commands/login.go cmd/chief/commands/logout.go cmd/chief/commands/root.go
git commit -m "feat(cli): add login and logout commands for uplink auth"
```

---

### Task 9: CLI Command — Serve (Daemon)

**Files:**
- Create: `cmd/chief/commands/serve.go`

This is the main orchestrator. It:
1. Loads credentials
2. Connects to the uplink server via WebSocket
3. Sends `state.sync` on connect
4. Handles incoming commands
5. Pushes state changes
6. Manages PRD chat sessions

- [ ] **Step 1: Implement serve.go**

```go
// cmd/chief/commands/serve.go
package commands

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/minicodemonkey/chief/internal/protocol"
	"github.com/minicodemonkey/chief/internal/session"
	"github.com/minicodemonkey/chief/internal/uplink"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Chief Uplink daemon",
	Long:  "Run Chief as a headless daemon connected to Uplink for remote management.",
	RunE:  runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load credentials
	credPath, err := auth.DefaultCredentialsPath()
	if err != nil {
		return err
	}

	creds, err := auth.LoadCredentials(credPath)
	if err != nil {
		return fmt.Errorf("not logged in. Run 'chief login' first")
	}

	if creds.IsExpired() {
		// Try to refresh
		flow := auth.NewDeviceFlow(creds.UplinkURL)
		tokenResp, err := flow.RefreshAccessToken(creds.RefreshToken)
		if err != nil {
			return fmt.Errorf("token expired and refresh failed. Run 'chief login' again: %w", err)
		}
		creds.AccessToken = tokenResp.AccessToken
		creds.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		auth.SaveCredentials(credPath, creds)
	}

	// Build WebSocket URL
	wsURL, err := buildWSURL(creds.UplinkURL)
	if err != nil {
		return err
	}

	// Get working directory
	workDir, _ := os.Getwd()

	// Set up state collector
	// Version is defined in root.go (set from main.go at build time)
	collector := uplink.NewStateCollector(creds.DeviceID, Version)
	collector.AddProject(workDir)

	// Set up WebSocket client
	client := uplink.NewClient(wsURL, creds.AccessToken, creds.DeviceID)

	// Set up command handler
	handler := uplink.NewHandler()
	handler.SetDeviceID(creds.DeviceID)

	// PRD chat sessions (keyed by PRD ID)
	chatSessions := make(map[string]*session.ChatSession)

	// Wire up command handlers
	handler.OnRunStart(func(cmd protocol.CmdRunStart) error {
		log.Printf("Starting run for PRD %s", cmd.PRDID)
		// TODO: Wire to loop.Manager
		return nil
	})

	handler.OnRunStop(func(cmd protocol.CmdRunStop) error {
		log.Printf("Stopping run %s", cmd.RunID)
		// TODO: Wire to loop.Manager
		return nil
	})

	handler.OnPRDCreate(func(cmd protocol.CmdPRDCreate) error {
		log.Printf("Creating PRD in project %s", cmd.ProjectID)
		// Generate PRD ID upfront so we can key the session consistently
		prdID := "prd_" + fmt.Sprintf("%d", time.Now().UnixNano())
		cs := session.NewChatSession(resolveAgentPath(), workDir)
		cs.OnEvent(func(event protocol.StatePRDChatOutput) {
			event.PRDID = prdID
			env := protocol.NewEnvelope(protocol.TypeStatePRDChatOutput, creds.DeviceID, event)
			client.Send(env)
		})

		// Store session keyed by PRD ID (same key used by OnPRDMessage)
		chatSessions[prdID] = cs

		go func() {
			result, err := cs.SendMessage(context.Background(), cmd.Message)
			if err != nil {
				log.Printf("PRD create failed: %v", err)
				return
			}
			log.Printf("PRD created (id=%s): %s", prdID, result[:min(len(result), 100)])
		}()

		return nil
	})

	handler.OnPRDMessage(func(cmd protocol.CmdPRDMessage) error {
		log.Printf("PRD message for %s", cmd.PRDID)
		cs, ok := chatSessions[cmd.PRDID]
		if !ok {
			return fmt.Errorf("no chat session for PRD %s", cmd.PRDID)
		}

		go func() {
			_, err := cs.SendMessage(context.Background(), cmd.Message)
			if err != nil {
				log.Printf("PRD message failed: %v", err)
			}
		}()

		return nil
	})

	// Handle incoming messages
	client.OnMessage(func(env *protocol.Envelope) {
		if strings.HasPrefix(env.Type, "cmd.") {
			resp, err := handler.Handle(env)
			if err != nil {
				log.Printf("Handler error: %v", err)
			}
			client.Send(resp)
		}
	})

	// Connect with automatic reconnection
	client.ConnectWithReconnect(func() error {
		log.Println("Connected to uplink, sending state sync...")
		state, err := collector.Collect()
		if err != nil {
			return fmt.Errorf("collect state: %w", err)
		}
		env := protocol.NewEnvelope(protocol.TypeStateSync, creds.DeviceID, state)
		return client.Send(env)
	})

	fmt.Printf("Chief Uplink daemon started (device: %s)\n", creds.DeviceID)
	fmt.Printf("Connected to %s\n", creds.UplinkURL)
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("Shutting down...")
	client.Close()
	return nil
}

func buildWSURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse uplink URL: %w", err)
	}

	scheme := "wss"
	if u.Scheme == "http" {
		scheme = "ws"
	}

	return fmt.Sprintf("%s://%s/ws/device", scheme, u.Host), nil
}

func resolveAgentPath() string {
	// TODO: resolve from config, for now use "claude"
	return "claude"
}
```

- [ ] **Step 2: Register serve command in root.go**

Add to `init()` in `cmd/chief/commands/root.go`:

```go
rootCmd.AddCommand(serveCmd)
```

- [ ] **Step 3: Build and verify**

```bash
cd /Users/codemonkey/projects/chief && go build -o /tmp/chief-test ./cmd/chief && /tmp/chief-test serve --help
```
Expected: serve command shows help text

- [ ] **Step 4: Commit**

```bash
git add cmd/chief/commands/serve.go cmd/chief/commands/root.go
git commit -m "feat(cli): add serve command (uplink daemon)"
```

---

### Task 10: Heartbeat & Token Refresh Loop

**Files:**
- Modify: `cmd/chief/commands/serve.go`

Add background goroutines for heartbeat and token refresh.

- [ ] **Step 1: Add heartbeat and token refresh to serve.go**

Add before the signal wait block:

```go
// Start heartbeat
go func() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			env := protocol.NewEnvelope(protocol.TypeStateDeviceHeartbeat, creds.DeviceID, protocol.StateDeviceHeartbeat{})
			if err := client.Send(env); err != nil {
				log.Printf("Heartbeat failed: %v", err)
			}
		}
	}
}()

// Start token refresh
go func() {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(creds.ExpiresAt) - 5*time.Minute):
			flow := auth.NewDeviceFlow(creds.UplinkURL)
			tokenResp, err := flow.RefreshAccessToken(creds.RefreshToken)
			if err != nil {
				log.Printf("Token refresh failed: %v", err)
				return
			}
			creds.AccessToken = tokenResp.AccessToken
			creds.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
			auth.SaveCredentials(credPath, creds)
			log.Println("Token refreshed")
		}
	}
}()
```

- [ ] **Step 2: Build**

```bash
cd /Users/codemonkey/projects/chief && go build ./cmd/chief
```
Expected: builds without errors

- [ ] **Step 3: Commit**

```bash
git add cmd/chief/commands/serve.go
git commit -m "feat(serve): add heartbeat and token refresh background loops"
```

---

### Task 11: Wire Remaining Command Handlers

**Files:**
- Modify: `cmd/chief/commands/serve.go`

Wire the remaining command handlers (diffs, logs, files, settings).

- [ ] **Step 1: Add handlers for diffs, logs, files, settings**

```go
handler.OnDiffsGet(func(cmd protocol.CmdDiffsGet) error {
	log.Printf("Getting diffs for project %s", cmd.ProjectID)
	go func() {
		// Run git diff in project directory
		out, err := exec.CommandContext(context.Background(), "git", "diff").Output()
		if err != nil {
			log.Printf("git diff failed: %v", err)
			return
		}
		// Parse diff output into DiffEntry slice
		// For now, send as single entry
		resp := protocol.NewEnvelope(protocol.TypeStateDiffsResponse, creds.DeviceID, protocol.StateDiffsResponse{
			RefID:     cmd.ProjectID, // Should be the original message ID
			ProjectID: cmd.ProjectID,
			Diffs: []protocol.DiffEntry{
				{FilePath: ".", Diff: string(out), Status: "modified"},
			},
		})
		client.Send(resp)
	}()
	return nil
})

handler.OnLogGet(func(cmd protocol.CmdLogGet) error {
	lines := cmd.Lines
	if lines == 0 {
		lines = 100
	}
	// Read from log file
	// TODO: resolve correct log file path from project
	return nil
})

handler.OnFilesList(func(cmd protocol.CmdFilesList) error {
	go func() {
		// Sanitize path to prevent directory traversal
		resolved := filepath.Join(workDir, filepath.Clean("/"+cmd.Path))
		if !strings.HasPrefix(resolved, workDir) {
			log.Printf("Path traversal blocked: %s", cmd.Path)
			return
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			log.Printf("ReadDir failed: %v", err)
			return
		}
		var fileEntries []protocol.FileEntry
		for _, e := range entries {
			info, _ := e.Info()
			fe := protocol.FileEntry{
				Name: e.Name(),
				Type: "file",
			}
			if e.IsDir() {
				fe.Type = "directory"
			}
			if info != nil {
				fe.Size = info.Size()
				fe.Modified = info.ModTime().UTC().Format(time.RFC3339)
			}
			fileEntries = append(fileEntries, fe)
		}
		resp := protocol.NewEnvelope(protocol.TypeStateFilesList, creds.DeviceID, protocol.StateFilesList{
			RefID:     cmd.ProjectID,
			ProjectID: cmd.ProjectID,
			Path:      cmd.Path,
			Entries:   fileEntries,
		})
		client.Send(resp)
	}()
	return nil
})

handler.OnFileGet(func(cmd protocol.CmdFileGet) error {
	go func() {
		// Sanitize path to prevent directory traversal
		resolved := filepath.Join(workDir, filepath.Clean("/"+cmd.Path))
		if !strings.HasPrefix(resolved, workDir) {
			log.Printf("Path traversal blocked: %s", cmd.Path)
			return
		}
		content, err := os.ReadFile(resolved)
		if err != nil {
			log.Printf("ReadFile failed: %v", err)
			return
		}
		ext := filepath.Ext(cmd.Path)
		resp := protocol.NewEnvelope(protocol.TypeStateFileResponse, creds.DeviceID, protocol.StateFileResponse{
			RefID:     cmd.ProjectID,
			ProjectID: cmd.ProjectID,
			Path:      cmd.Path,
			Content:   string(content),
			Language:  strings.TrimPrefix(ext, "."),
		})
		client.Send(resp)
	}()
	return nil
})
```

Note: Add `"os/exec"`, `"path/filepath"` to imports.

- [ ] **Step 2: Build**

```bash
cd /Users/codemonkey/projects/chief && go build ./cmd/chief
```

- [ ] **Step 3: Commit**

```bash
git add cmd/chief/commands/serve.go
git commit -m "feat(serve): wire diffs, logs, files, and settings handlers"
```

---

### Task 12: Integration Test — Full Serve Flow

**Files:**
- Create: `cmd/chief/commands/serve_test.go`

- [ ] **Step 1: Write integration test with mock server**

```go
// cmd/chief/commands/serve_test.go
package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/protocol"
	"github.com/minicodemonkey/chief/internal/uplink"
)

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestClientSendsStateSyncOnConnect(t *testing.T) {
	syncReceived := make(chan protocol.StateSync, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send welcome
		welcome := protocol.NewEnvelope(protocol.TypeWelcome, "server", protocol.Welcome{
			SessionID: "sess_test", ServerVersion: "1.0.0", Capabilities: []string{},
		})
		data, _ := welcome.Marshal()
		conn.WriteMessage(websocket.TextMessage, data)

		// Read state.sync
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		env, _ := protocol.ParseEnvelope(msg)
		if env.Type == protocol.TypeStateSync {
			payload, _ := protocol.DecodePayload[protocol.StateSync](env)
			syncReceived <- *payload
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/device"
	client := uplink.NewClient(wsURL, "test_token", "dev_test")

	collector := uplink.NewStateCollector("dev_test", "0.5.0")
	collector.AddProject(t.TempDir())

	client.OnMessage(func(env *protocol.Envelope) {})

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	<-client.Ready()

	// Send sync
	state, _ := collector.Collect()
	env := protocol.NewEnvelope(protocol.TypeStateSync, "dev_test", state)
	client.Send(env)

	select {
	case sync := <-syncReceived:
		if sync.Device.ChiefVersion != "0.5.0" {
			t.Errorf("ChiefVersion = %q, want %q", sync.Device.ChiefVersion, "0.5.0")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for state.sync")
	}
}
```

- [ ] **Step 2: Run test**

```bash
cd /Users/codemonkey/projects/chief && go test ./cmd/chief/commands/ -v -run TestClientSendsStateSync
```
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/chief/commands/serve_test.go
git commit -m "test(serve): add integration test for state sync on connect"
```
