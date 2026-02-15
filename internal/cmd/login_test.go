package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/minicodemonkey/chief/internal/auth"
)

func setTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
}

func TestRunLogin_Success(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	var pollCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			json.NewEncoder(w).Encode(deviceCodeResponse{
				DeviceCode: "test-device-code",
				UserCode:   "ABCD-1234",
			})
		case "/oauth/device/token":
			count := pollCount.Add(1)
			if count < 2 {
				// First poll: authorization pending
				json.NewEncoder(w).Encode(tokenResponse{
					Error: "authorization_pending",
				})
				return
			}
			// Second poll: success
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
				User:         "testuser@example.com",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Override stdin to avoid blocking on "already logged in" prompt
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Close()

	err := RunLogin(LoginOptions{
		DeviceName: "test-device",
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatalf("RunLogin failed: %v", err)
	}

	// Verify credentials were saved
	creds, err := auth.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials after login failed: %v", err)
	}
	if creds.AccessToken != "test-access-token" {
		t.Errorf("expected access_token %q, got %q", "test-access-token", creds.AccessToken)
	}
	if creds.RefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh_token %q, got %q", "test-refresh-token", creds.RefreshToken)
	}
	if creds.User != "testuser@example.com" {
		t.Errorf("expected user %q, got %q", "testuser@example.com", creds.User)
	}
	if creds.DeviceName != "test-device" {
		t.Errorf("expected device_name %q, got %q", "test-device", creds.DeviceName)
	}
}

func TestRunLogin_DeviceCodeError(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := RunLogin(LoginOptions{
		DeviceName: "test-device",
		BaseURL:    server.URL,
	})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestRunLogin_AuthorizationDenied(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			json.NewEncoder(w).Encode(deviceCodeResponse{
				DeviceCode: "test-device-code",
				UserCode:   "ABCD-1234",
			})
		case "/oauth/device/token":
			json.NewEncoder(w).Encode(tokenResponse{
				Error: "access_denied",
			})
		}
	}))
	defer server.Close()

	err := RunLogin(LoginOptions{
		DeviceName: "test-device",
		BaseURL:    server.URL,
	})
	if err == nil {
		t.Fatal("expected error for denied authorization")
	}
}

func TestRunLogin_DefaultDeviceName(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	var receivedDeviceName string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			receivedDeviceName = body["device_name"]
			json.NewEncoder(w).Encode(deviceCodeResponse{
				DeviceCode: "test-device-code",
				UserCode:   "TEST-CODE",
			})
		case "/oauth/device/token":
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  "token",
				RefreshToken: "refresh",
				ExpiresIn:    3600,
				User:         "user",
			})
		}
	}))
	defer server.Close()

	err := RunLogin(LoginOptions{
		BaseURL: server.URL,
		// DeviceName left empty — should default to hostname
	})
	if err != nil {
		t.Fatalf("RunLogin failed: %v", err)
	}

	hostname, _ := os.Hostname()
	if receivedDeviceName != hostname {
		t.Errorf("expected device name %q (hostname), got %q", hostname, receivedDeviceName)
	}
}

func TestRunLogin_AlreadyLoggedIn_Decline(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Save existing credentials
	existing := &auth.Credentials{
		AccessToken: "existing-token",
		User:        "existing-user",
		DeviceName:  "existing-device",
	}
	if err := auth.SaveCredentials(existing); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	// Pipe "n\n" to stdin to decline
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	r, w, _ := os.Pipe()
	w.Write([]byte("n\n"))
	w.Close()
	os.Stdin = r

	// Server should not be called at all when declining
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when login is declined")
	}))
	defer server.Close()

	err := RunLogin(LoginOptions{
		DeviceName: "new-device",
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatalf("RunLogin should not error when declining: %v", err)
	}

	// Credentials should remain unchanged
	creds, err := auth.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if creds.AccessToken != "existing-token" {
		t.Errorf("credentials should not have changed, got access_token %q", creds.AccessToken)
	}
}

func TestOpenBrowser(t *testing.T) {
	// Just verifying it doesn't panic — browser open is best-effort
	openBrowser("https://example.com")
}
