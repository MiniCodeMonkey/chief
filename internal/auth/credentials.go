package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	credentialsFile = "credentials.yaml"
	chiefDir        = ".chief"
	refreshBuffer   = 5 * time.Minute
)

// Credentials holds OAuth tokens and device info for authenticating with the uplink server.
type Credentials struct {
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	Expiry       time.Time `yaml:"expiry"`
	DeviceName   string    `yaml:"device_name"`
	DeviceID     string    `yaml:"device_id"`
	UplinkURL    string    `yaml:"uplink_url"`
	WebSocketURL string    `yaml:"websocket_url,omitempty"`
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	return time.Now().After(c.Expiry)
}

// NeedsRefresh returns true if the access token will expire within 5 minutes.
func (c *Credentials) NeedsRefresh() bool {
	return time.Now().After(c.Expiry.Add(-refreshBuffer))
}

// credentialsPath returns the full path to the credentials file.
// If homeDir is empty, it uses the user's home directory.
func credentialsPath(homeDir string) (string, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
	}
	return filepath.Join(homeDir, chiefDir, credentialsFile), nil
}

// SaveCredentials writes credentials to ~/.chief/credentials.yaml with 0600 permissions.
func SaveCredentials(creds *Credentials) error {
	return SaveCredentialsTo("", creds)
}

// SaveCredentialsTo writes credentials to the specified home directory.
func SaveCredentialsTo(homeDir string, creds *Credentials) error {
	path, err := credentialsPath(homeDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create credentials directory: %w", err)
	}

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("cannot marshal credentials: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("cannot write credentials file: %w", err)
	}

	return nil
}

// LoadCredentials reads credentials from ~/.chief/credentials.yaml.
// Returns an error if the file does not exist.
func LoadCredentials() (*Credentials, error) {
	return LoadCredentialsFrom("")
}

// LoadCredentialsFrom reads credentials from the specified home directory.
func LoadCredentialsFrom(homeDir string) (*Credentials, error) {
	path, err := credentialsPath(homeDir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read credentials file: %w", err)
	}

	var creds Credentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("cannot parse credentials file: %w", err)
	}

	return &creds, nil
}

// DeleteCredentials removes the credentials file.
func DeleteCredentials() error {
	return DeleteCredentialsFrom("")
}

// DeleteCredentialsFrom removes the credentials file from the specified home directory.
func DeleteCredentialsFrom(homeDir string) error {
	path, err := credentialsPath(homeDir)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot delete credentials file: %w", err)
	}

	return nil
}
