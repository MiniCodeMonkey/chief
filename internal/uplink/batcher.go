package uplink

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Flush tier durations.
const (
	// tierImmediate flushes immediately (0ms delay).
	tierImmediate = 0 * time.Millisecond

	// tierStandard flushes after 200ms.
	tierStandard = 200 * time.Millisecond

	// tierLowPriority flushes after 1s.
	tierLowPriority = 1 * time.Second

	// maxBatchMessages is the maximum number of messages before a forced flush.
	maxBatchMessages = 20

	// maxBufferMessages is the maximum total messages in the buffer before dropping.
	maxBufferMessages = 1000

	// maxBufferBytes is the maximum total payload size (5MB) before dropping.
	maxBufferBytes = 5 * 1024 * 1024
)

// tier identifies a flush priority tier.
type tier int

const (
	tierIDImmediate tier = iota
	tierIDStandard
	tierIDLowPriority
)

// tierFor returns the tier for a given message type.
func tierFor(msgType string) tier {
	switch msgType {
	case "run_complete", "run_paused", "error", "clone_complete", "session_expired", "quota_exhausted":
		return tierIDImmediate
	case "claude_output", "prd_output", "run_progress", "clone_progress":
		return tierIDStandard
	case "state_snapshot", "project_state", "project_list", "settings", "log_lines":
		return tierIDLowPriority
	default:
		// Unknown types go to standard tier.
		return tierIDStandard
	}
}

// tierDelay returns the flush delay for a tier.
func tierDelay(t tier) time.Duration {
	switch t {
	case tierIDImmediate:
		return tierImmediate
	case tierIDStandard:
		return tierStandard
	case tierIDLowPriority:
		return tierLowPriority
	default:
		return tierStandard
	}
}

// bufferedMessage is a message waiting to be flushed.
type bufferedMessage struct {
	data json.RawMessage
	tier tier
	size int
}

// SendFunc is the function called on flush to send a batch of messages.
// batchID is a unique UUID for idempotency. The function should retry internally if needed.
type SendFunc func(batchID string, messages []json.RawMessage) error

// Batcher collects outgoing messages and flushes them in batches.
// Messages are assigned to priority tiers that control flush timing.
// Flushes are sequential — the next flush waits for the current one to complete.
type Batcher struct {
	sendFn SendFunc

	mu          sync.Mutex
	messages    []bufferedMessage
	totalSize   int
	flushNotify chan struct{} // signals the run loop that a flush may be needed
	stopped     bool

	// Timer management for tier-based flushing.
	standardTimer   *time.Timer
	lowPriorityTimer *time.Timer
	standardActive   bool
	lowPriorityActive bool
}

// NewBatcher creates a Batcher that calls sendFn on each flush.
func NewBatcher(sendFn SendFunc) *Batcher {
	return &Batcher{
		sendFn:      sendFn,
		flushNotify: make(chan struct{}, 1),
	}
}

// Enqueue adds a message to the appropriate tier buffer.
// It is safe to call from multiple goroutines.
func (b *Batcher) Enqueue(msg json.RawMessage, msgType string) {
	t := tierFor(msgType)
	size := len(msg)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}

	// Check buffer limits and drop low-priority messages if full.
	for b.totalSize+size > maxBufferBytes || len(b.messages)+1 > maxBufferMessages {
		if !b.dropLowestPriority() {
			// Nothing left to drop — reject this message too.
			log.Printf("batcher: buffer full, dropping %s message (%d bytes)", msgType, size)
			return
		}
	}

	b.messages = append(b.messages, bufferedMessage{data: msg, tier: t, size: size})
	b.totalSize += size

	// Determine if we need to flush now.
	shouldFlushNow := t == tierIDImmediate || len(b.messages) >= maxBatchMessages

	if shouldFlushNow {
		b.notifyFlush()
		return
	}

	// Start tier timers if not already running.
	if t == tierIDStandard && !b.standardActive {
		b.standardActive = true
		if b.standardTimer == nil {
			b.standardTimer = time.AfterFunc(tierStandard, func() {
				b.mu.Lock()
				b.standardActive = false
				b.mu.Unlock()
				b.notifyFlush()
			})
		} else {
			b.standardTimer.Reset(tierStandard)
		}
	}
	if t == tierIDLowPriority && !b.lowPriorityActive {
		b.lowPriorityActive = true
		if b.lowPriorityTimer == nil {
			b.lowPriorityTimer = time.AfterFunc(tierLowPriority, func() {
				b.mu.Lock()
				b.lowPriorityActive = false
				b.mu.Unlock()
				b.notifyFlush()
			})
		} else {
			b.lowPriorityTimer.Reset(tierLowPriority)
		}
	}
}

// notifyFlush signals the run loop that a flush should happen.
// Must not be called with b.mu held if blocking is possible, but the channel is buffered.
func (b *Batcher) notifyFlush() {
	select {
	case b.flushNotify <- struct{}{}:
	default:
		// Already notified.
	}
}

// dropLowestPriority removes the last low-priority message from the buffer.
// Returns false if there are no low-priority messages to drop.
// Caller must hold b.mu.
func (b *Batcher) dropLowestPriority() bool {
	// Search from the end for the lowest-priority message.
	// Priority order for dropping: low priority first, then standard.
	for priority := tierIDLowPriority; priority >= tierIDStandard; priority-- {
		for i := len(b.messages) - 1; i >= 0; i-- {
			if b.messages[i].tier == priority {
				b.totalSize -= b.messages[i].size
				log.Printf("batcher: buffer overflow, dropping message at index %d (tier %d, %d bytes)", i, priority, b.messages[i].size)
				b.messages = append(b.messages[:i], b.messages[i+1:]...)
				return true
			}
		}
	}
	return false
}

// Run starts the background flush loop. It blocks until ctx is done.
// Flushes are sequential — only one flush runs at a time.
func (b *Batcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.flushNotify:
			b.flush()
		}
	}
}

// Stop performs a final flush of all remaining messages, then marks the batcher as stopped.
func (b *Batcher) Stop() {
	b.mu.Lock()
	b.stopped = true
	// Stop timers.
	if b.standardTimer != nil {
		b.standardTimer.Stop()
	}
	if b.lowPriorityTimer != nil {
		b.lowPriorityTimer.Stop()
	}
	b.mu.Unlock()

	// Final flush.
	b.flush()
}

// flush collects all pending messages and sends them as a single batch.
// It is always called from a single goroutine (the Run loop or Stop), so flushes never overlap.
func (b *Batcher) flush() {
	b.mu.Lock()
	if len(b.messages) == 0 {
		b.mu.Unlock()
		return
	}

	// Collect all messages.
	msgs := make([]json.RawMessage, len(b.messages))
	for i, m := range b.messages {
		msgs[i] = m.data
	}
	b.messages = b.messages[:0]
	b.totalSize = 0

	// Reset timer state since we're flushing everything.
	b.standardActive = false
	b.lowPriorityActive = false
	b.mu.Unlock()

	batchID := generateBatchID()
	if err := b.sendFn(batchID, msgs); err != nil {
		log.Printf("batcher: flush failed (batch %s, %d messages): %v", batchID, len(msgs), err)
	}
}

// generateBatchID returns a new UUID v4 string for batch idempotency.
func generateBatchID() string {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (extremely unlikely).
		return fmt.Sprintf("batch-%d", time.Now().UnixNano())
	}
	// Set version (4) and variant (RFC 4122).
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
