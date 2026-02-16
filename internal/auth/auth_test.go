package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// setTestHome overrides HOME so credentials are read/written inside t.TempDir().
// It returns a cleanup function that restores the original HOME.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	orig := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	t.Cleanup(func() {
		os.Setenv("HOME", orig)
	})
}

func TestLoadCredentials_NotLoggedIn(t *testing.T) {
	setTestHome(t, t.TempDir())

	_, err := LoadCredentials()
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

func TestSaveAndLoadCredentials(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	expires := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	creds := &Credentials{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    expires,
		DeviceName:   "my-laptop",
		User:         "user@example.com",
	}

	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}

	if loaded.AccessToken != "access-abc" {
		t.Errorf("expected access_token %q, got %q", "access-abc", loaded.AccessToken)
	}
	if loaded.RefreshToken != "refresh-xyz" {
		t.Errorf("expected refresh_token %q, got %q", "refresh-xyz", loaded.RefreshToken)
	}
	if !loaded.ExpiresAt.Equal(expires) {
		t.Errorf("expected expires_at %v, got %v", expires, loaded.ExpiresAt)
	}
	if loaded.DeviceName != "my-laptop" {
		t.Errorf("expected device_name %q, got %q", "my-laptop", loaded.DeviceName)
	}
	if loaded.User != "user@example.com" {
		t.Errorf("expected user %q, got %q", "user@example.com", loaded.User)
	}
}

func TestSaveCredentials_FilePermissions(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	creds := &Credentials{
		AccessToken: "token",
	}

	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	path := filepath.Join(home, ".chief", "credentials.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}

func TestSaveCredentials_Atomic(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Save initial credentials.
	initial := &Credentials{
		AccessToken: "first",
		User:        "user1",
	}
	if err := SaveCredentials(initial); err != nil {
		t.Fatalf("SaveCredentials (initial) failed: %v", err)
	}

	// Save updated credentials (should atomically replace).
	updated := &Credentials{
		AccessToken: "second",
		User:        "user2",
	}
	if err := SaveCredentials(updated); err != nil {
		t.Fatalf("SaveCredentials (updated) failed: %v", err)
	}

	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if loaded.AccessToken != "second" {
		t.Errorf("expected access_token %q, got %q", "second", loaded.AccessToken)
	}
	if loaded.User != "user2" {
		t.Errorf("expected user %q, got %q", "user2", loaded.User)
	}
}

func TestDeleteCredentials(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	creds := &Credentials{AccessToken: "to-delete"}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	if err := DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials failed: %v", err)
	}

	_, err := LoadCredentials()
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn after delete, got %v", err)
	}
}

func TestDeleteCredentials_NonExistent(t *testing.T) {
	setTestHome(t, t.TempDir())

	// Deleting when file doesn't exist should not error.
	if err := DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials on non-existent file failed: %v", err)
	}
}

func TestSaveLoadDeleteCycle(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// 1. Not logged in initially.
	_, err := LoadCredentials()
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn initially, got %v", err)
	}

	// 2. Save credentials.
	creds := &Credentials{
		AccessToken:  "cycle-token",
		RefreshToken: "cycle-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		DeviceName:   "test-device",
		User:         "cycle-user",
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	// 3. Load and verify.
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if loaded.AccessToken != "cycle-token" {
		t.Errorf("expected access_token %q, got %q", "cycle-token", loaded.AccessToken)
	}

	// 4. Delete.
	if err := DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials failed: %v", err)
	}

	// 5. Not logged in again.
	_, err = LoadCredentials()
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn after delete, got %v", err)
	}
}

func TestIsExpired(t *testing.T) {
	// Expired token.
	expired := &Credentials{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if !expired.IsExpired() {
		t.Error("expected token to be expired")
	}

	// Valid token.
	valid := &Credentials{
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if valid.IsExpired() {
		t.Error("expected token to not be expired")
	}
}

func TestIsNearExpiry(t *testing.T) {
	// Token expires in 3 minutes — should be near expiry within 5 minutes.
	creds := &Credentials{
		ExpiresAt: time.Now().Add(3 * time.Minute),
	}

	if !creds.IsNearExpiry(5 * time.Minute) {
		t.Error("expected token to be near expiry within 5 minutes")
	}

	if creds.IsNearExpiry(1 * time.Minute) {
		t.Error("expected token to NOT be near expiry within 1 minute")
	}
}

func TestIsNearExpiry_AlreadyExpired(t *testing.T) {
	creds := &Credentials{
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	if !creds.IsNearExpiry(5 * time.Minute) {
		t.Error("expected already-expired token to be near expiry")
	}
}

func TestSaveCredentials_CreatesDirectory(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	chiefDir := filepath.Join(home, ".chief")
	if _, err := os.Stat(chiefDir); !os.IsNotExist(err) {
		t.Fatal("expected .chief directory to not exist initially")
	}

	creds := &Credentials{AccessToken: "create-dir"}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	info, err := os.Stat(chiefDir)
	if err != nil {
		t.Fatalf("expected .chief directory to exist after save, got: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected .chief to be a directory")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Save credentials that are near expiry (within 5 minutes)
	creds := &Credentials{
		AccessToken:  "old-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    time.Now().Add(2 * time.Minute), // near expiry
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["refresh_token"] != "test-refresh-token" {
				t.Errorf("expected refresh_token %q, got %q", "test-refresh-token", body["refresh_token"])
			}
			json.NewEncoder(w).Encode(refreshResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
				ExpiresIn:    3600,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	refreshed, err := RefreshToken(server.URL)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if refreshed.AccessToken != "new-access-token" {
		t.Errorf("expected access_token %q, got %q", "new-access-token", refreshed.AccessToken)
	}
	if refreshed.RefreshToken != "new-refresh-token" {
		t.Errorf("expected refresh_token %q, got %q", "new-refresh-token", refreshed.RefreshToken)
	}

	// Verify credentials were persisted
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if loaded.AccessToken != "new-access-token" {
		t.Errorf("expected persisted access_token %q, got %q", "new-access-token", loaded.AccessToken)
	}
}

func TestRefreshToken_NotNearExpiry(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Save credentials that are NOT near expiry
	creds := &Credentials{
		AccessToken:  "valid-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	// Server should not be called since token is still valid
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when token is not near expiry")
	}))
	defer server.Close()

	refreshed, err := RefreshToken(server.URL)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if refreshed.AccessToken != "valid-token" {
		t.Errorf("expected access_token %q, got %q", "valid-token", refreshed.AccessToken)
	}
}

func TestRefreshToken_SessionExpired(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	creds := &Credentials{
		AccessToken:  "old-token",
		RefreshToken: "revoked-refresh-token",
		ExpiresAt:    time.Now().Add(2 * time.Minute),
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			json.NewEncoder(w).Encode(refreshResponse{
				Error: "invalid_grant",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := RefreshToken(server.URL)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestRefreshToken_NotLoggedIn(t *testing.T) {
	setTestHome(t, t.TempDir())

	_, err := RefreshToken("")
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

func TestRefreshToken_ThreadSafe(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	creds := &Credentials{
		AccessToken:  "old-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(2 * time.Minute),
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	var callCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			mu.Lock()
			callCount++
			mu.Unlock()
			json.NewEncoder(w).Encode(refreshResponse{
				AccessToken:  "new-token",
				RefreshToken: "new-refresh",
				ExpiresIn:    3600,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Run multiple concurrent refreshes
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := RefreshToken(server.URL)
			if err != nil {
				t.Errorf("RefreshToken failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// Only one actual refresh should have hit the server
	// (the first one refreshes, subsequent ones see it's no longer near expiry)
	mu.Lock()
	count := callCount
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected exactly 1 server call, got %d", count)
	}
}

func TestRevokeDevice_Success(t *testing.T) {
	var receivedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/revoke" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			receivedToken = body["access_token"]
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := RevokeDevice("my-token", server.URL)
	if err != nil {
		t.Fatalf("RevokeDevice failed: %v", err)
	}
	if receivedToken != "my-token" {
		t.Errorf("expected token %q, got %q", "my-token", receivedToken)
	}
}

func TestRevokeDevice_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := RevokeDevice("my-token", server.URL)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}
