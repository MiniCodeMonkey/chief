package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
)

func TestRunLoginSuccess(t *testing.T) {
	// Create a test server that simulates the device flow
	requestCalled := false
	verifyCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/device/request":
			requestCalled = true
			json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "test-device-code",
				UserCode:   "ABCD-1234",
				VerifyURL:  "https://example.com/verify",
			})
		case "/api/auth/device/verify":
			verifyCalls++
			// Return success immediately
			json.NewEncoder(w).Encode(auth.TokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
				DeviceID:     "dev-123",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Use a temp home dir so we don't pollute the real one
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	err := RunLogin(LoginOptions{URL: server.URL, PollInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("RunLogin returned error: %v", err)
	}

	if !requestCalled {
		t.Error("expected device/request to be called")
	}
	if verifyCalls != 1 {
		t.Errorf("expected 1 verify call, got %d", verifyCalls)
	}

	// Verify credentials were saved
	creds, err := auth.LoadCredentialsFrom(tmpHome)
	if err != nil {
		t.Fatalf("failed to load saved credentials: %v", err)
	}
	if creds.AccessToken != "test-access-token" {
		t.Errorf("expected access token 'test-access-token', got %q", creds.AccessToken)
	}
	if creds.DeviceID != "dev-123" {
		t.Errorf("expected device ID 'dev-123', got %q", creds.DeviceID)
	}
	if creds.UplinkURL != server.URL {
		t.Errorf("expected uplink URL %q, got %q", server.URL, creds.UplinkURL)
	}

	// Verify file permissions
	credPath := filepath.Join(tmpHome, ".chief", "credentials.yaml")
	info, err := os.Stat(credPath)
	if err != nil {
		t.Fatalf("cannot stat credentials file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestRunLoginDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/device/request":
			json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "test-device-code",
				UserCode:   "ABCD-1234",
				VerifyURL:  "https://example.com/verify",
			})
		case "/api/auth/device/verify":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	err := RunLogin(LoginOptions{URL: server.URL, PollInterval: 10 * time.Millisecond})
	if err == nil {
		t.Fatal("expected error for denied authorization")
	}
	if err.Error() != "authorization denied" {
		t.Errorf("expected 'authorization denied', got %q", err.Error())
	}
}

func TestRunLoginRequestFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := RunLogin(LoginOptions{URL: server.URL, PollInterval: 10 * time.Millisecond})
	if err == nil {
		t.Fatal("expected error for request failure")
	}
}
