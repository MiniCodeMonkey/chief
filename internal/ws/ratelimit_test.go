package ws

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsNormalMessages(t *testing.T) {
	rl := NewRateLimiter()

	// Should allow a burst of messages up to the burst limit
	for i := 0; i < globalBurst; i++ {
		result := rl.Allow(TypeGetProject)
		if !result.Allowed {
			t.Fatalf("expected message %d to be allowed within burst", i+1)
		}
	}
}

func TestRateLimiter_BlocksAfterBurstExhausted(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust the burst
	for i := 0; i < globalBurst; i++ {
		rl.Allow(TypeGetProject)
	}

	// Next message should be blocked
	result := rl.Allow(TypeGetProject)
	if result.Allowed {
		t.Fatal("expected message to be blocked after burst exhausted")
	}
	if result.RetryAfter <= 0 {
		t.Error("expected positive retry-after duration")
	}
}

func TestRateLimiter_RefillsTokensOverTime(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust the burst
	for i := 0; i < globalBurst; i++ {
		rl.Allow(TypeGetProject)
	}

	// Manually advance the token bucket's last time to simulate time passing
	rl.mu.Lock()
	rl.global.lastTime = time.Now().Add(-200 * time.Millisecond) // should refill ~2 tokens
	rl.mu.Unlock()

	result := rl.Allow(TypeGetProject)
	if !result.Allowed {
		t.Fatal("expected message to be allowed after token refill")
	}
}

func TestRateLimiter_PingExempt(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust the burst
	for i := 0; i < globalBurst+10; i++ {
		rl.Allow(TypeGetProject)
	}

	// Ping should still be allowed
	result := rl.Allow(TypePing)
	if !result.Allowed {
		t.Fatal("expected ping to be exempt from rate limiting")
	}
}

func TestRateLimiter_ExpensiveOperationsLimited(t *testing.T) {
	rl := NewRateLimiter()

	// First two clone_repo should be allowed
	result1 := rl.Allow(TypeCloneRepo)
	result2 := rl.Allow(TypeCloneRepo)
	if !result1.Allowed || !result2.Allowed {
		t.Fatal("expected first two expensive operations to be allowed")
	}

	// Third should be blocked
	result3 := rl.Allow(TypeCloneRepo)
	if result3.Allowed {
		t.Fatal("expected third expensive operation to be blocked")
	}
	if result3.RetryAfter <= 0 {
		t.Error("expected positive retry-after for expensive operation")
	}
}

func TestRateLimiter_ExpensiveTypesIndependent(t *testing.T) {
	rl := NewRateLimiter()

	// Use up clone_repo limit
	rl.Allow(TypeCloneRepo)
	rl.Allow(TypeCloneRepo)

	// start_run should still be allowed (independent tracker)
	result := rl.Allow(TypeStartRun)
	if !result.Allowed {
		t.Fatal("expected start_run to be allowed independently of clone_repo")
	}

	// new_prd should also be allowed
	result = rl.Allow(TypeNewPRD)
	if !result.Allowed {
		t.Fatal("expected new_prd to be allowed independently")
	}
}

func TestRateLimiter_ExpensiveWindowExpiry(t *testing.T) {
	rl := NewRateLimiter()

	// Use up the limit
	rl.Allow(TypeStartRun)
	rl.Allow(TypeStartRun)

	// Should be blocked
	result := rl.Allow(TypeStartRun)
	if result.Allowed {
		t.Fatal("expected to be blocked")
	}

	// Simulate time passing beyond the window
	rl.mu.Lock()
	tracker := rl.expensive[TypeStartRun]
	for i := range tracker.timestamps {
		tracker.timestamps[i] = time.Now().Add(-expensiveWindow - time.Second)
	}
	rl.mu.Unlock()

	// Should be allowed again
	result = rl.Allow(TypeStartRun)
	if !result.Allowed {
		t.Fatal("expected to be allowed after window expires")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust burst and expensive limits
	for i := 0; i < globalBurst+5; i++ {
		rl.Allow(TypeGetProject)
	}
	rl.Allow(TypeCloneRepo)
	rl.Allow(TypeCloneRepo)

	// Verify blocked
	result := rl.Allow(TypeGetProject)
	if result.Allowed {
		t.Fatal("expected blocked before reset")
	}
	result = rl.Allow(TypeCloneRepo)
	if result.Allowed {
		t.Fatal("expected expensive blocked before reset")
	}

	// Reset
	rl.Reset()

	// Should be allowed again
	result = rl.Allow(TypeGetProject)
	if !result.Allowed {
		t.Fatal("expected allowed after reset")
	}
	result = rl.Allow(TypeCloneRepo)
	if !result.Allowed {
		t.Fatal("expected expensive allowed after reset")
	}
}

func TestRateLimiter_ExpensiveAlsoConsumesGlobal(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust global bucket
	for i := 0; i < globalBurst; i++ {
		rl.Allow(TypeGetProject)
	}

	// Expensive operation should be blocked by global limit even though
	// the expensive tracker would allow it
	result := rl.Allow(TypeCloneRepo)
	if result.Allowed {
		t.Fatal("expected expensive operation to be blocked by global limit")
	}
}

func TestFormatRetryAfter(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{100 * time.Millisecond, "1s"},
		{500 * time.Millisecond, "1s"},
		{1500 * time.Millisecond, "2s"},
		{30 * time.Second, "31s"},
		{0, "1s"},
	}

	for _, tt := range tests {
		got := FormatRetryAfter(tt.d)
		if got != tt.want {
			t.Errorf("FormatRetryAfter(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestIsExpensiveType(t *testing.T) {
	if !IsExpensiveType(TypeCloneRepo) {
		t.Error("expected clone_repo to be expensive")
	}
	if !IsExpensiveType(TypeStartRun) {
		t.Error("expected start_run to be expensive")
	}
	if !IsExpensiveType(TypeNewPRD) {
		t.Error("expected new_prd to be expensive")
	}
	if IsExpensiveType(TypeGetProject) {
		t.Error("expected get_project to NOT be expensive")
	}
	if IsExpensiveType(TypePing) {
		t.Error("expected ping to NOT be expensive")
	}
}

func TestIsExemptType(t *testing.T) {
	if !IsExemptType(TypePing) {
		t.Error("expected ping to be exempt")
	}
	if IsExemptType(TypeGetProject) {
		t.Error("expected get_project to NOT be exempt")
	}
}

func TestTokenBucket_RetryAfter(t *testing.T) {
	tb := newTokenBucket(1, 10) // 1 burst, 10/sec

	// Use the token
	now := time.Now()
	tb.allow(now)

	// Should need ~100ms for next token
	retryAfter := tb.retryAfter()
	if retryAfter <= 0 {
		t.Error("expected positive retry-after")
	}
	if retryAfter > 200*time.Millisecond {
		t.Errorf("retry-after too large: %v", retryAfter)
	}
}

func TestExpensiveTracker_RetryAfter(t *testing.T) {
	et := newExpensiveTracker(2, time.Minute)

	now := time.Now()
	et.allow(now)
	et.allow(now.Add(time.Second))

	// Should be blocked with retry-after pointing to when oldest expires
	retryAfter := et.retryAfter(now.Add(2 * time.Second))
	if retryAfter <= 0 {
		t.Error("expected positive retry-after")
	}
	// Should be about 58 seconds (60 - 2 seconds elapsed)
	if retryAfter < 55*time.Second || retryAfter > 61*time.Second {
		t.Errorf("unexpected retry-after: %v", retryAfter)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				rl.Allow(TypeGetProject)
				rl.Allow(TypeCloneRepo)
				rl.Allow(TypePing)
			}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Just ensure no panics or races
	rl.Reset()
}
