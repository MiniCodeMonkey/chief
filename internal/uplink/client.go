package uplink

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/minicodemonkey/chief/internal/ws"
)

const (
	// maxBackoff is the maximum reconnection delay.
	maxBackoff = 60 * time.Second

	// initialBackoff is the starting reconnection delay.
	initialBackoff = 1 * time.Second

	// httpTimeout is the default HTTP request timeout.
	httpTimeout = 10 * time.Second
)

// WelcomeResponse is the response from POST /api/device/connect.
type WelcomeResponse struct {
	Type            string       `json:"type"`
	ProtocolVersion int          `json:"protocol_version"`
	DeviceID        int          `json:"device_id"`
	SessionID       string       `json:"session_id"`
	Reverb          ReverbConfig `json:"reverb"`
}

// ReverbConfig contains Pusher/Reverb connection details from the connect response.
type ReverbConfig struct {
	Key    string `json:"key"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme"`
}

// connectRequest is the JSON body sent to POST /api/device/connect.
type connectRequest struct {
	ChiefVersion    string `json:"chief_version"`
	DeviceName      string `json:"device_name"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	ProtocolVersion int    `json:"protocol_version"`
}

// errorResponse is a JSON error returned by the server.
type errorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ErrAuthFailed is returned when the server rejects authentication (401).
var ErrAuthFailed = fmt.Errorf("device deauthorized — run 'chief login' to re-authenticate")

// ErrDeviceRevoked is returned when the device is revoked (403).
var ErrDeviceRevoked = fmt.Errorf("device revoked — run 'chief login' to re-authenticate")

// Client is an HTTP client for the uplink device API.
type Client struct {
	baseURL     string
	accessToken string
	mu          sync.RWMutex
	httpClient  *http.Client

	// Device metadata sent on connect.
	chiefVersion string
	deviceName   string
}

// Option configures a Client.
type Option func(*Client)

// WithChiefVersion sets the chief CLI version string.
func WithChiefVersion(v string) Option {
	return func(c *Client) {
		c.chiefVersion = v
	}
}

// WithDeviceName sets the device name.
func WithDeviceName(name string) Option {
	return func(c *Client) {
		c.deviceName = name
	}
}

// WithHTTPClient sets a custom http.Client (useful for testing).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// New creates a new uplink HTTP client.
// The baseURL must use HTTPS unless the host is localhost or 127.0.0.1.
func New(baseURL, accessToken string, opts ...Option) (*Client, error) {
	if err := validateBaseURL(baseURL); err != nil {
		return nil, err
	}

	c := &Client{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: httpTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// validateBaseURL ensures the URL uses HTTPS unless the host is localhost/127.0.0.1.
func validateBaseURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	host := u.Hostname()
	if u.Scheme == "http" && host != "localhost" && host != "127.0.0.1" {
		return fmt.Errorf("base URL must use HTTPS (got %s); HTTP is only allowed for localhost", rawURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base URL must use http or https scheme (got %s)", u.Scheme)
	}

	return nil
}

// SetAccessToken updates the access token in a thread-safe manner.
// This is called after a token refresh.
func (c *Client) SetAccessToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = token
}

// Connect calls POST /api/device/connect to register the device with the server.
// Returns the welcome response containing session ID and Reverb configuration.
func (c *Client) Connect(ctx context.Context) (*WelcomeResponse, error) {
	version := c.chiefVersion
	if version == "" {
		version = "dev"
	}

	body := connectRequest{
		ChiefVersion:    version,
		DeviceName:      c.deviceName,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		ProtocolVersion: ws.ProtocolVersion,
	}

	var welcome WelcomeResponse
	if err := c.doJSON(ctx, "POST", "/api/device/connect", body, &welcome); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	return &welcome, nil
}

// IngestResponse is the response from POST /api/device/messages.
type IngestResponse struct {
	Accepted  int    `json:"accepted"`
	BatchID   string `json:"batch_id"`
	SessionID string `json:"session_id"`
}

// ingestRequest is the JSON body sent to POST /api/device/messages.
type ingestRequest struct {
	BatchID  string            `json:"batch_id"`
	Messages []json.RawMessage `json:"messages"`
}

// SendMessages sends a batch of messages via POST /api/device/messages.
// It does NOT retry on failure — use SendMessagesWithRetry for retry behavior.
func (c *Client) SendMessages(ctx context.Context, batchID string, messages []json.RawMessage) (*IngestResponse, error) {
	body := ingestRequest{
		BatchID:  batchID,
		Messages: messages,
	}

	var resp IngestResponse
	if err := c.doJSON(ctx, "POST", "/api/device/messages", body, &resp); err != nil {
		return nil, fmt.Errorf("send messages: %w", err)
	}

	return &resp, nil
}

// SendMessagesWithRetry sends a message batch with exponential backoff retry on transient failures.
// It does not retry on 401/403 auth errors. Retries use the same batchID for server-side deduplication.
func (c *Client) SendMessagesWithRetry(ctx context.Context, batchID string, messages []json.RawMessage) (*IngestResponse, error) {
	attempt := 0
	for {
		resp, err := c.SendMessages(ctx, batchID, messages)
		if err == nil {
			return resp, nil
		}

		// Don't retry auth errors.
		if isAuthError(err) {
			return nil, err
		}

		attempt++
		delay := backoff(attempt)
		log.Printf("SendMessages failed (attempt %d, batch %s): %v — retrying in %s", attempt, batchID, err, delay.Round(time.Millisecond))

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// Heartbeat calls POST /api/device/heartbeat to tell the server the device is alive.
func (c *Client) Heartbeat(ctx context.Context) error {
	var resp json.RawMessage
	if err := c.doJSON(ctx, "POST", "/api/device/heartbeat", nil, &resp); err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	return nil
}

// Disconnect calls POST /api/device/disconnect to notify the server the device is going offline.
func (c *Client) Disconnect(ctx context.Context) error {
	var resp json.RawMessage
	if err := c.doJSON(ctx, "POST", "/api/device/disconnect", nil, &resp); err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	return nil
}

// doJSON performs an HTTP request with JSON body and parses the JSON response.
// It handles auth headers and classifies HTTP error responses.
func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the full response body (capped to prevent OOM on rogue responses).
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	// Classify HTTP errors.
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthFailed
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrDeviceRevoked
	}
	if resp.StatusCode >= 400 {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return fmt.Errorf("server error %d: %s", resp.StatusCode, errResp.Message)
		}
		return fmt.Errorf("server error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
	}

	return nil
}

// backoff returns a duration for the given attempt using exponential backoff + jitter.
func backoff(attempt int) time.Duration {
	base := float64(initialBackoff) * math.Pow(2, float64(attempt-1))
	if base > float64(maxBackoff) {
		base = float64(maxBackoff)
	}
	// Add jitter: 0.5x to 1.5x
	jitter := 0.5 + rand.Float64()
	return time.Duration(base * jitter)
}

// ConnectWithRetry calls Connect with exponential backoff retry on transient failures.
// It does not retry on 401/403 auth errors.
func (c *Client) ConnectWithRetry(ctx context.Context) (*WelcomeResponse, error) {
	attempt := 0
	for {
		welcome, err := c.Connect(ctx)
		if err == nil {
			return welcome, nil
		}

		// Don't retry auth errors.
		if isAuthError(err) {
			return nil, err
		}

		attempt++
		delay := backoff(attempt)
		log.Printf("Connect failed (attempt %d): %v — retrying in %s", attempt, err, delay.Round(time.Millisecond))

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// isAuthError returns true if the error is a 401 or 403 that should not be retried.
// Uses errors.Is to handle wrapped errors.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrAuthFailed) || errors.Is(err, ErrDeviceRevoked)
}
