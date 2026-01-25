//go:build linux && !cgo

package notify

import (
	"errors"
	"sync"
)

// Notifier handles audio notifications.
// This is a stub implementation for Linux without CGO support.
type Notifier struct {
	mu      sync.Mutex
	enabled bool
}

var (
	globalNotifier *Notifier
	initOnce       sync.Once
	initErr        error
)

// GetNotifier returns the global notifier instance.
// On Linux without CGO, audio is not available.
func GetNotifier() (*Notifier, error) {
	initOnce.Do(func() {
		globalNotifier = &Notifier{
			enabled: false,
		}
		initErr = errors.New("audio not available: built without CGO support (Linux requires CGO for ALSA)")
	})
	return globalNotifier, initErr
}

// SetEnabled enables or disables sound notifications.
func (n *Notifier) SetEnabled(enabled bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = enabled
}

// IsEnabled returns whether sound is enabled.
func (n *Notifier) IsEnabled() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.enabled
}

// PlayCompletion is a no-op on Linux without CGO.
func (n *Notifier) PlayCompletion() {
	// No-op: audio not available without CGO
}
