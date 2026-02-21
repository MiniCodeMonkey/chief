package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const credentialsFile = "credentials.yaml"

const defaultBaseURL = "https://uplink.chiefloop.com"

// ErrNotLoggedIn is returned when no credentials file exists.
var ErrNotLoggedIn = errors.New("not logged in — run 'chief login' first")

// ErrSessionExpired is returned when the refresh token is revoked or expired.
var ErrSessionExpired = errors.New("session expired — run 'chief login' again")

// refreshMu protects concurrent token refresh operations.
var refreshMu sync.Mutex

// Credentials holds authentication token data for uplink.chiefloop.com.
type Credentials struct {
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	ExpiresAt    time.Time `yaml:"expires_at"`
	DeviceName   string    `yaml:"device_name"`
	User         string    `yaml:"user"`
	WSURL        string    `yaml:"ws_url,omitempty"`
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// IsNearExpiry returns true if the access token will expire within the given duration.
func (c *Credentials) IsNearExpiry(d time.Duration) bool {
	return time.Now().Add(d).After(c.ExpiresAt)
}

// credentialsDir returns the path to the ~/.chief directory.
func credentialsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".chief"), nil
}

// credentialsPath returns the full path to ~/.chief/credentials.yaml.
func credentialsPath() (string, error) {
	dir, err := credentialsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credentialsFile), nil
}

// LoadCredentials reads credentials from ~/.chief/credentials.yaml.
// Returns ErrNotLoggedIn when the file does not exist.
func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotLoggedIn
		}
		return nil, fmt.Errorf("reading credentials: %w", err)
	}

	var creds Credentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}

	return &creds, nil
}

// SaveCredentials writes credentials to ~/.chief/credentials.yaml atomically.
// It writes to a temporary file first, then renames it into place.
// The file is created with 0600 permissions (owner read/write only).
func SaveCredentials(creds *Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating credentials directory: %w", err)
	}

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}

	// Write to temp file in the same directory for atomic rename.
	tmp, err := os.CreateTemp(dir, "credentials-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if err := os.Chmod(tmpPath, 0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("setting temp file permissions: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// DeleteCredentials removes the credentials file.
// Returns nil if the file does not exist.
func DeleteCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing credentials: %w", err)
	}

	return nil
}

// refreshResponse is the response from the token refresh endpoint.
type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	WSURL        string `json:"ws_url"`
	Error        string `json:"error"`
}

// RefreshToken refreshes the access token using the refresh token.
// It is thread-safe (mutex-protected for concurrent use by serve).
// baseURL can be empty to use the default (https://uplink.chiefloop.com).
func RefreshToken(baseURL string) (*Credentials, error) {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}

	// If token was already refreshed by another goroutine, return it.
	if !creds.IsNearExpiry(5 * time.Minute) {
		return creds, nil
	}

	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	reqBody, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": creds.RefreshToken,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/oauth/token", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp refreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("parsing refresh response: %w", err)
	}

	if tokenResp.Error != "" || resp.StatusCode != http.StatusOK {
		return nil, ErrSessionExpired
	}

	creds.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		creds.RefreshToken = tokenResp.RefreshToken
	}
	if tokenResp.WSURL != "" {
		creds.WSURL = tokenResp.WSURL
	}
	creds.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	if err := SaveCredentials(creds); err != nil {
		return nil, fmt.Errorf("saving refreshed credentials: %w", err)
	}

	return creds, nil
}

// RevokeDevice calls the revocation endpoint to deauthorize the device server-side.
// baseURL can be empty to use the default (https://uplink.chiefloop.com).
func RevokeDevice(accessToken, baseURL string) error {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	reqBody, _ := json.Marshal(map[string]string{
		"access_token": accessToken,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(baseURL+"/api/oauth/revoke", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("revoking device: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revocation failed: server returned %s", resp.Status)
	}

	return nil
}
