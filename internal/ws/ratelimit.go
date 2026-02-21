package ws

import (
	"fmt"
	"sync"
	"time"
)

// Rate limiter constants (hardcoded for V1).
const (
	// Global token bucket: 30 burst, 10/second sustained.
	globalBurst    = 30
	globalRate     = 10.0 // tokens per second
	globalInterval = time.Second / 10

	// Expensive operations: 2 per minute.
	expensiveLimit    = 2
	expensiveWindow   = time.Minute
	expensiveInterval = expensiveWindow / 2 // 30 seconds between allowed ops
)

// expensiveTypes are message types with stricter per-type rate limits.
var expensiveTypes = map[string]bool{
	TypeCloneRepo: true,
	TypeStartRun:  true,
	TypeNewPRD:    true,
}

// exemptTypes are message types exempt from rate limiting.
var exemptTypes = map[string]bool{
	TypePing: true,
}

// IsExpensiveType returns true if the message type has a stricter per-type rate limit.
func IsExpensiveType(msgType string) bool {
	return expensiveTypes[msgType]
}

// IsExemptType returns true if the message type is exempt from rate limiting.
func IsExemptType(msgType string) bool {
	return exemptTypes[msgType]
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	lastTime time.Time
}

func newTokenBucket(capacity float64, rate float64) *tokenBucket {
	return &tokenBucket{
		tokens:   capacity,
		capacity: capacity,
		rate:     rate,
		lastTime: time.Now(),
	}
}

// allow checks if a token is available and consumes one if so.
// Returns true if allowed, false if rate limited.
func (tb *tokenBucket) allow(now time.Time) bool {
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastTime = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// retryAfter returns the duration until the next token is available.
func (tb *tokenBucket) retryAfter() time.Duration {
	if tb.tokens >= 1 {
		return 0
	}
	needed := 1.0 - tb.tokens
	return time.Duration(needed / tb.rate * float64(time.Second))
}

// expensiveTracker tracks per-type rate limiting for expensive operations.
type expensiveTracker struct {
	timestamps []time.Time
	limit      int
	window     time.Duration
}

func newExpensiveTracker(limit int, window time.Duration) *expensiveTracker {
	return &expensiveTracker{
		limit:  limit,
		window: window,
	}
}

// allow checks if the operation is allowed within the rate limit window.
func (et *expensiveTracker) allow(now time.Time) bool {
	// Remove expired timestamps
	cutoff := now.Add(-et.window)
	valid := et.timestamps[:0]
	for _, ts := range et.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	et.timestamps = valid

	if len(et.timestamps) >= et.limit {
		return false
	}
	et.timestamps = append(et.timestamps, now)
	return true
}

// retryAfter returns the duration until the next operation would be allowed.
func (et *expensiveTracker) retryAfter(now time.Time) time.Duration {
	if len(et.timestamps) < et.limit {
		return 0
	}
	oldest := et.timestamps[0]
	return oldest.Add(et.window).Sub(now)
}

// RateLimiter provides rate limiting for incoming WebSocket messages.
type RateLimiter struct {
	mu       sync.Mutex
	global   *tokenBucket
	expensive map[string]*expensiveTracker
}

// NewRateLimiter creates a new rate limiter with default settings.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		global:    newTokenBucket(globalBurst, globalRate),
		expensive: make(map[string]*expensiveTracker),
	}
}

// RateLimitResult contains the result of a rate limit check.
type RateLimitResult struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Allow checks if a message of the given type should be allowed.
// Returns RateLimitResult indicating whether the message is allowed and retry-after hint.
func (rl *RateLimiter) Allow(msgType string) RateLimitResult {
	// Exempt types bypass all rate limiting
	if IsExemptType(msgType) {
		return RateLimitResult{Allowed: true}
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Check expensive operation limit first
	if IsExpensiveType(msgType) {
		tracker, ok := rl.expensive[msgType]
		if !ok {
			tracker = newExpensiveTracker(expensiveLimit, expensiveWindow)
			rl.expensive[msgType] = tracker
		}
		if !tracker.allow(now) {
			retryAfter := tracker.retryAfter(now)
			return RateLimitResult{
				Allowed:    false,
				RetryAfter: retryAfter,
			}
		}
	}

	// Check global rate limit
	if !rl.global.allow(now) {
		retryAfter := rl.global.retryAfter()
		return RateLimitResult{
			Allowed:    false,
			RetryAfter: retryAfter,
		}
	}

	return RateLimitResult{Allowed: true}
}

// Reset clears all rate limiter state (called on reconnection).
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.global = newTokenBucket(globalBurst, globalRate)
	rl.expensive = make(map[string]*expensiveTracker)
}

// FormatRetryAfter returns a human-readable retry-after string.
func FormatRetryAfter(d time.Duration) string {
	secs := int(d.Seconds()) + 1 // round up
	if secs <= 0 {
		secs = 1
	}
	return fmt.Sprintf("%ds", secs)
}
