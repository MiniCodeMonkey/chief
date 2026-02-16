package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
)

func TestRunLogout_Success(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Save credentials
	creds := &auth.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := auth.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	var revokedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/revoke" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			revokedToken = body["access_token"]
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := RunLogout(LogoutOptions{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("RunLogout failed: %v", err)
	}

	// Verify revocation was called with correct token
	if revokedToken != "test-token" {
		t.Errorf("expected revoked token %q, got %q", "test-token", revokedToken)
	}

	// Verify credentials are deleted
	_, err = auth.LoadCredentials()
	if err != auth.ErrNotLoggedIn {
		t.Fatalf("expected ErrNotLoggedIn after logout, got %v", err)
	}
}

func TestRunLogout_NotLoggedIn(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	err := RunLogout(LogoutOptions{})
	if err != nil {
		t.Fatalf("RunLogout should not error when not logged in: %v", err)
	}
}

func TestRunLogout_RevocationFails_StillDeletesCredentials(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	creds := &auth.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := auth.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	// Server returns error on revocation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := RunLogout(LogoutOptions{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("RunLogout should not error when revocation fails: %v", err)
	}

	// Credentials should still be deleted
	_, err = auth.LoadCredentials()
	if err != auth.ErrNotLoggedIn {
		t.Fatalf("expected ErrNotLoggedIn after logout, got %v", err)
	}
}
