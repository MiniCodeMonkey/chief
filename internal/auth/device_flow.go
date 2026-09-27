package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeviceCodeResponse holds the response from the device code request endpoint.
type DeviceCodeResponse struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	VerifyURL  string `json:"verify_url"`
}

// TokenResponse holds the response from a successful token exchange.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	DeviceID     int    `json:"device_id"`
}

// DeviceFlow handles the OAuth device authorization flow for CLI authentication.
type DeviceFlow struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewDeviceFlow creates a new DeviceFlow client for the given base URL.
func NewDeviceFlow(baseURL string) *DeviceFlow {
	return &DeviceFlow{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// RequestCode initiates the device flow by requesting a device code from the server.
func (d *DeviceFlow) RequestCode(deviceName string) (*DeviceCodeResponse, error) {
	body := fmt.Sprintf(`{"device_name":%q}`, deviceName)
	req, err := http.NewRequest(http.MethodPost, d.BaseURL+"/api/auth/device/request", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("cannot decode response: %w", err)
	}
	return &result, nil
}

// PollResult indicates the outcome of a poll attempt.
type PollResult int

const (
	PollSuccess PollResult = iota
	PollPending
	PollDenied
)

// PollResponse holds the result of a single poll attempt.
type PollResponse struct {
	Result PollResult
	Token  *TokenResponse
}

// PollForToken checks whether the user has authorized the device.
// Returns PollSuccess with a token on 200, PollPending on 202, PollDenied on 403.
func (d *DeviceFlow) PollForToken(deviceCode string) (*PollResponse, error) {
	body := fmt.Sprintf(`{"device_code":%q}`, deviceCode)
	req, err := http.NewRequest(http.MethodPost, d.BaseURL+"/api/auth/device/verify", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var token TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
			return nil, fmt.Errorf("cannot decode token response: %w", err)
		}
		return &PollResponse{Result: PollSuccess, Token: &token}, nil
	case http.StatusAccepted:
		return &PollResponse{Result: PollPending}, nil
	case http.StatusForbidden:
		return &PollResponse{Result: PollDenied}, nil
	default:
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}

// RefreshAccessToken exchanges a refresh token for a new access token.
func (d *DeviceFlow) RefreshAccessToken(refreshToken string) (*TokenResponse, error) {
	body := fmt.Sprintf(`{"refresh_token":%q}`, refreshToken)
	req, err := http.NewRequest(http.MethodPost, d.BaseURL+"/api/auth/token/refresh", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("cannot decode token response: %w", err)
	}
	return &token, nil
}

// RevokeDevice revokes a device's authorization by its device ID.
func (d *DeviceFlow) RevokeDevice(deviceID, accessToken string) error {
	req, err := http.NewRequest(http.MethodDelete, d.BaseURL+"/api/auth/device/"+deviceID, nil)
	if err != nil {
		return fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("revoke failed with status %d", resp.StatusCode)
	}

	return nil
}
