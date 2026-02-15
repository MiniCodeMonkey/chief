package ws

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"runtime"
	"time"
)

const (
	// ProtocolVersion is the current protocol version.
	ProtocolVersion = 1

	// handshakeTimeout is how long to wait for a welcome/incompatible response.
	handshakeTimeout = 10 * time.Second
)

// HelloMessage is sent by chief immediately after WebSocket connection.
type HelloMessage struct {
	Type            string `json:"type"`
	ID              string `json:"id"`
	Timestamp       string `json:"timestamp"`
	ProtocolVersion int    `json:"protocol_version"`
	ChiefVersion    string `json:"chief_version"`
	DeviceName      string `json:"device_name"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	AccessToken     string `json:"access_token"`
}

// WelcomeMessage is sent by the server when the handshake succeeds.
type WelcomeMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// IncompatibleMessage is sent by the server when versions are incompatible.
type IncompatibleMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Message   string `json:"message"`
}

// AuthFailedMessage is sent by the server when authentication fails.
type AuthFailedMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Message   string `json:"message,omitempty"`
}

// HandshakeResult indicates the outcome of the protocol handshake.
type HandshakeResult int

const (
	// HandshakeOK means the handshake succeeded.
	HandshakeOK HandshakeResult = iota
	// HandshakeIncompatible means the server reported version incompatibility.
	HandshakeIncompatible
	// HandshakeAuthFailed means the server rejected authentication.
	HandshakeAuthFailed
	// HandshakeTimeout means the server didn't respond within the timeout.
	HandshakeTimeout
)

// ErrIncompatible is returned when the server reports version incompatibility.
type ErrIncompatible struct {
	Message string
}

func (e *ErrIncompatible) Error() string {
	return fmt.Sprintf("incompatible version: %s", e.Message)
}

// ErrAuthFailed is returned when the server rejects authentication.
var ErrAuthFailed = fmt.Errorf("device deauthorized — run 'chief login' to re-authenticate")

// ErrHandshakeTimeout is returned when the server doesn't respond to the hello.
var ErrHandshakeTimeout = fmt.Errorf("handshake timeout: no response within %s", handshakeTimeout)

// Handshake performs the protocol handshake after connecting.
// It sends a hello message and waits for a welcome, incompatible, or auth_failed response.
// Returns nil on success, or an error describing the failure.
func (c *Client) Handshake(accessToken, chiefVersion, deviceName string) error {
	hello := HelloMessage{
		Type:            "hello",
		ID:              newUUID(),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ProtocolVersion: ProtocolVersion,
		ChiefVersion:    chiefVersion,
		DeviceName:      deviceName,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		AccessToken:     accessToken,
	}

	if err := c.Send(hello); err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}

	// Wait for a response with a timeout.
	timer := time.NewTimer(handshakeTimeout)
	defer timer.Stop()

	select {
	case msg, ok := <-c.Receive():
		if !ok {
			return fmt.Errorf("connection closed during handshake")
		}

		switch msg.Type {
		case "welcome":
			return nil
		case "incompatible":
			var inc IncompatibleMessage
			if err := json.Unmarshal(msg.Raw, &inc); err != nil {
				return fmt.Errorf("parsing incompatible message: %w", err)
			}
			return &ErrIncompatible{Message: inc.Message}
		case "auth_failed":
			return ErrAuthFailed
		default:
			return fmt.Errorf("unexpected handshake response type: %s", msg.Type)
		}

	case <-timer.C:
		return ErrHandshakeTimeout
	}
}

// newUUID generates a random UUID v4 string.
func newUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	// Set version 4 bits.
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant bits.
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// NewMessage creates a new message envelope with type, UUID, and ISO8601 timestamp.
func NewMessage(msgType string) Message {
	return Message{
		Type:      msgType,
		ID:        newUUID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
