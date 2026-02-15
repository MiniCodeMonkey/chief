package auth

import (
	"errors"
	"os"
	"path/filepath"
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
