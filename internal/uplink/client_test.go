package uplink

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/ws"
)

// testContext returns a context with a 15-second timeout for tests.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// newTestClient creates a Client pointing at a test server with the given token.
func newTestClient(t *testing.T, serverURL, token string, opts ...Option) *Client {
	t.Helper()
	c, err := New(serverURL, token, opts...)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return c
}

func TestNew_ValidHTTPS(t *testing.T) {
	c, err := New("https://example.com", "token123")
	if err != nil {
		t.Fatalf("New() with HTTPS failed: %v", err)
	}
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://example.com")
	}
}

func TestNew_LocalhostHTTP(t *testing.T) {
	_, err := New("http://localhost:8080", "token123")
	if err != nil {
		t.Fatalf("New() with localhost HTTP failed: %v", err)
	}
}

func TestNew_Loopback127HTTP(t *testing.T) {
	_, err := New("http://127.0.0.1:8080", "token123")
	if err != nil {
		t.Fatalf("New() with 127.0.0.1 HTTP failed: %v", err)
	}
}

func TestNew_RejectsNonLocalhostHTTP(t *testing.T) {
	_, err := New("http://example.com", "token123")
	if err == nil {
		t.Fatal("expected error for non-localhost HTTP, got nil")
	}
}

func TestNew_RejectsInvalidScheme(t *testing.T) {
	_, err := New("ftp://example.com", "token123")
	if err == nil {
		t.Fatal("expected error for ftp scheme, got nil")
	}
}

func TestNew_WithOptions(t *testing.T) {
	c, err := New("https://example.com", "token123",
		WithChiefVersion("1.2.3"),
		WithDeviceName("my-device"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if c.chiefVersion != "1.2.3" {
		t.Errorf("chiefVersion = %q, want %q", c.chiefVersion, "1.2.3")
	}
	if c.deviceName != "my-device" {
		t.Errorf("deviceName = %q, want %q", c.deviceName, "my-device")
	}
}

func TestConnect_Success(t *testing.T) {
	var receivedBody connectRequest
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/device/connect" {
			http.NotFound(w, r)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		receivedAuth = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WelcomeResponse{
			Type:            "welcome",
			ProtocolVersion: 1,
			DeviceID:        42,
			SessionID:       "sess-abc-123",
			Reverb: ReverbConfig{
				Key:    "app-key",
				Host:   "reverb.example.com",
				Port:   443,
				Scheme: "https",
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "test-token-abc",
		WithChiefVersion("2.0.0"),
		WithDeviceName("test-device"),
	)

	ctx := testContext(t)
	welcome, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Verify request.
	if receivedAuth != "Bearer test-token-abc" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer test-token-abc")
	}
	if receivedBody.ChiefVersion != "2.0.0" {
		t.Errorf("chief_version = %q, want %q", receivedBody.ChiefVersion, "2.0.0")
	}
	if receivedBody.DeviceName != "test-device" {
		t.Errorf("device_name = %q, want %q", receivedBody.DeviceName, "test-device")
	}
	if receivedBody.OS != runtime.GOOS {
		t.Errorf("os = %q, want %q", receivedBody.OS, runtime.GOOS)
	}
	if receivedBody.Arch != runtime.GOARCH {
		t.Errorf("arch = %q, want %q", receivedBody.Arch, runtime.GOARCH)
	}
	if receivedBody.ProtocolVersion != ws.ProtocolVersion {
		t.Errorf("protocol_version = %d, want %d", receivedBody.ProtocolVersion, ws.ProtocolVersion)
	}

	// Verify response.
	if welcome.Type != "welcome" {
		t.Errorf("Type = %q, want %q", welcome.Type, "welcome")
	}
	if welcome.DeviceID != 42 {
		t.Errorf("DeviceID = %d, want %d", welcome.DeviceID, 42)
	}
	if welcome.SessionID != "sess-abc-123" {
		t.Errorf("SessionID = %q, want %q", welcome.SessionID, "sess-abc-123")
	}
	if welcome.Reverb.Key != "app-key" {
		t.Errorf("Reverb.Key = %q, want %q", welcome.Reverb.Key, "app-key")
	}
	if welcome.Reverb.Host != "reverb.example.com" {
		t.Errorf("Reverb.Host = %q, want %q", welcome.Reverb.Host, "reverb.example.com")
	}
	if welcome.Reverb.Port != 443 {
		t.Errorf("Reverb.Port = %d, want %d", welcome.Reverb.Port, 443)
	}
	if welcome.Reverb.Scheme != "https" {
		t.Errorf("Reverb.Scheme = %q, want %q", welcome.Reverb.Scheme, "https")
	}
}

func TestConnect_DefaultVersion(t *testing.T) {
	var receivedBody connectRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WelcomeResponse{Type: "welcome", DeviceID: 1, SessionID: "s"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "token")
	ctx := testContext(t)
	_, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	if receivedBody.ChiefVersion != "dev" {
		t.Errorf("chief_version = %q, want %q (default)", receivedBody.ChiefVersion, "dev")
	}
}

func TestConnect_AuthFailed401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errorResponse{Error: "invalid_token", Message: "Invalid token"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "bad-token")
	ctx := testContext(t)
	_, err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !errors.Is(err, ErrAuthFailed) {
		t.Errorf("error = %v, want ErrAuthFailed", err)
	}
}

func TestConnect_DeviceRevoked403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errorResponse{Error: "device_revoked", Message: "Device revoked"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "revoked-token")
	ctx := testContext(t)
	_, err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !errors.Is(err, ErrDeviceRevoked) {
		t.Errorf("error = %v, want ErrDeviceRevoked", err)
	}
}

func TestConnect_ServerError5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Message: "something went wrong"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "token")
	ctx := testContext(t)
	_, err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if isAuthError(err) {
		t.Error("5xx error should not be classified as auth error")
	}
}

func TestDisconnect_Success(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "test-token")
	ctx := testContext(t)
	err := client.Disconnect(ctx)
	if err != nil {
		t.Fatalf("Disconnect() failed: %v", err)
	}

	if receivedMethod != "POST" {
		t.Errorf("method = %q, want POST", receivedMethod)
	}
	if receivedPath != "/api/device/disconnect" {
		t.Errorf("path = %q, want /api/device/disconnect", receivedPath)
	}
	if receivedAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer test-token")
	}
}

func TestDisconnect_AuthFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "bad-token")
	ctx := testContext(t)
	err := client.Disconnect(ctx)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestSetAccessToken_ThreadSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return the received token in the response body for verification.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": r.Header.Get("Authorization"),
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "token-v1")

	// Spawn goroutines that concurrently update the token and make requests.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.SetAccessToken("token-v2")
		}()
	}
	wg.Wait()

	// After all updates, the token should be v2.
	client.mu.RLock()
	token := client.accessToken
	client.mu.RUnlock()

	if token != "token-v2" {
		t.Errorf("accessToken = %q, want %q", token, "token-v2")
	}
}

func TestConnect_RequestFormat(t *testing.T) {
	var receivedContentType string
	var receivedAccept string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WelcomeResponse{Type: "welcome", DeviceID: 1, SessionID: "s"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "token")
	ctx := testContext(t)
	client.Connect(ctx)

	if receivedContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", receivedContentType)
	}
	if receivedAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", receivedAccept)
	}
}

func TestConnect_ContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the test is done (context cancelled will abort the request).
		<-blocked
	}))
	defer srv.Close()
	defer close(blocked) // unblock the handler so the server can shut down cleanly

	client := newTestClient(t, srv.URL, "token")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestConnectWithRetry_SuccessOnSecondAttempt(t *testing.T) {
	var attempt atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempt.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WelcomeResponse{
			Type:      "welcome",
			DeviceID:  42,
			SessionID: "sess-123",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "token")
	ctx := testContext(t)
	welcome, err := client.ConnectWithRetry(ctx)
	if err != nil {
		t.Fatalf("ConnectWithRetry() failed: %v", err)
	}
	if welcome.DeviceID != 42 {
		t.Errorf("DeviceID = %d, want 42", welcome.DeviceID)
	}
	if attempt.Load() != 2 {
		t.Errorf("attempts = %d, want 2", attempt.Load())
	}
}

func TestConnectWithRetry_NoRetryOnAuthError(t *testing.T) {
	var attempt atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "bad-token")
	ctx := testContext(t)
	_, err := client.ConnectWithRetry(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempt.Load() != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on auth error)", attempt.Load())
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		minMs   int64
		maxMs   int64
	}{
		{1, 500, 1500},     // 1s * (0.5 to 1.5)
		{2, 1000, 3000},    // 2s * (0.5 to 1.5)
		{3, 2000, 6000},    // 4s * (0.5 to 1.5)
		{4, 4000, 12000},   // 8s * (0.5 to 1.5)
		{10, 30000, 90000}, // capped at 60s * (0.5 to 1.5)
	}

	for _, tt := range tests {
		d := backoff(tt.attempt)
		ms := d.Milliseconds()
		if ms < tt.minMs || ms > tt.maxMs {
			t.Errorf("backoff(%d) = %dms, want [%d, %d]ms", tt.attempt, ms, tt.minMs, tt.maxMs)
		}
	}
}

func TestIsAuthError(t *testing.T) {
	if isAuthError(nil) {
		t.Error("nil should not be auth error")
	}
	if !isAuthError(ErrAuthFailed) {
		t.Error("ErrAuthFailed should be auth error")
	}
	if !isAuthError(ErrDeviceRevoked) {
		t.Error("ErrDeviceRevoked should be auth error")
	}
	if isAuthError(context.Canceled) {
		t.Error("context.Canceled should not be auth error")
	}
}
