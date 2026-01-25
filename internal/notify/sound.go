//go:build darwin || windows || (linux && cgo)

package notify

import (
	_ "embed"
	"io"
	"log"
	"sync"
	"time"

	"github.com/hajimehoshi/oto/v2"
)

//go:embed complete.wav
var completionSound []byte

// Notifier handles audio notifications.
type Notifier struct {
	context *oto.Context
	mu      sync.Mutex
	enabled bool
}

var (
	globalNotifier *Notifier
	initOnce       sync.Once
	initErr        error
)

// GetNotifier returns the global notifier instance.
// This is a singleton since oto.Context should only be created once.
func GetNotifier() (*Notifier, error) {
	initOnce.Do(func() {
		// oto context: sample rate 22050, mono channel, format (16-bit signed = 2 bytes)
		ctx, ready, err := oto.NewContext(22050, 1, 2)
		if err != nil {
			initErr = err
			return
		}
		<-ready

		globalNotifier = &Notifier{
			context: ctx,
			enabled: true,
		}
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

// PlayCompletion plays the completion sound.
func (n *Notifier) PlayCompletion() {
	n.mu.Lock()
	if !n.enabled || n.context == nil {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	// Play in a goroutine to avoid blocking
	go func() {
		if err := n.playWAV(completionSound); err != nil {
			// Log warning but don't crash
			log.Printf("Warning: failed to play completion sound: %v", err)
		}
	}()
}

// playWAV plays a WAV file from bytes.
func (n *Notifier) playWAV(data []byte) error {
	if len(data) < 44 {
		return nil // Invalid WAV, skip silently
	}

	// Skip WAV header (44 bytes for standard WAV)
	audioData := data[44:]

	player := n.context.NewPlayer(NewWAVReader(audioData))
	defer player.Close()

	player.Play()

	// Wait for playback to complete
	for player.IsPlaying() {
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// WAVReader implements io.Reader for raw PCM data.
type WAVReader struct {
	data   []byte
	offset int
}

// NewWAVReader creates a new WAVReader.
func NewWAVReader(data []byte) *WAVReader {
	return &WAVReader{data: data}
}

// Read implements io.Reader.
func (r *WAVReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}
