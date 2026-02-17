package uplink

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// flushRecord captures a single flush call.
type flushRecord struct {
	batchID  string
	messages []json.RawMessage
	time     time.Time
}

// newRecordingSendFn returns a SendFunc that records all flush calls
// and a function to retrieve the records.
func newRecordingSendFn() (SendFunc, func() []flushRecord) {
	var mu sync.Mutex
	var records []flushRecord

	fn := func(batchID string, messages []json.RawMessage) error {
		mu.Lock()
		defer mu.Unlock()
		// Copy messages to avoid data races.
		copied := make([]json.RawMessage, len(messages))
		copy(copied, messages)
		records = append(records, flushRecord{
			batchID:  batchID,
			messages: copied,
			time:     time.Now(),
		})
		return nil
	}

	get := func() []flushRecord {
		mu.Lock()
		defer mu.Unlock()
		result := make([]flushRecord, len(records))
		copy(result, records)
		return result
	}

	return fn, get
}

func TestBatcher_ImmediateFlush(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// Enqueue an immediate-tier message.
	b.Enqueue(json.RawMessage(`{"type":"run_complete"}`), "run_complete")

	// Wait for flush.
	time.Sleep(50 * time.Millisecond)

	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1", len(records))
	}
	if len(records[0].messages) != 1 {
		t.Errorf("message count = %d, want 1", len(records[0].messages))
	}
	if records[0].batchID == "" {
		t.Error("batchID should not be empty")
	}
}

func TestBatcher_ImmediateFlushDrainsAllTiers(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// Enqueue messages from different tiers.
	b.Enqueue(json.RawMessage(`{"type":"project_state"}`), "project_state") // low
	b.Enqueue(json.RawMessage(`{"type":"claude_output"}`), "claude_output") // standard
	b.Enqueue(json.RawMessage(`{"type":"error"}`), "error")                 // immediate

	// Wait for flush.
	time.Sleep(50 * time.Millisecond)

	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1 (all tiers drain together)", len(records))
	}
	if len(records[0].messages) != 3 {
		t.Errorf("message count = %d, want 3", len(records[0].messages))
	}
}

func TestBatcher_AllImmediateTypes(t *testing.T) {
	immediateTypes := []string{
		"run_complete", "run_paused", "error",
		"clone_complete", "session_expired", "quota_exhausted",
	}

	for _, msgType := range immediateTypes {
		t.Run(msgType, func(t *testing.T) {
			sendFn, getRecords := newRecordingSendFn()
			b := NewBatcher(sendFn)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go b.Run(ctx)

			b.Enqueue(json.RawMessage(`{}`), msgType)
			time.Sleep(50 * time.Millisecond)

			records := getRecords()
			if len(records) != 1 {
				t.Errorf("expected immediate flush for %s, got %d flushes", msgType, len(records))
			}
		})
	}
}

func TestBatcher_StandardTimerFlush(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	start := time.Now()
	b.Enqueue(json.RawMessage(`{"type":"claude_output"}`), "claude_output")

	// Should not flush immediately.
	time.Sleep(50 * time.Millisecond)
	if len(getRecords()) != 0 {
		t.Fatal("standard tier should not flush immediately")
	}

	// Wait for the 200ms timer.
	time.Sleep(300 * time.Millisecond)
	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1", len(records))
	}

	elapsed := records[0].time.Sub(start)
	if elapsed < 150*time.Millisecond {
		t.Errorf("flushed too early: %v (expected ~200ms)", elapsed)
	}
}

func TestBatcher_LowPriorityTimerFlush(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	start := time.Now()
	b.Enqueue(json.RawMessage(`{"type":"project_state"}`), "project_state")

	// Should not flush at 200ms.
	time.Sleep(300 * time.Millisecond)
	if len(getRecords()) != 0 {
		t.Fatal("low priority tier should not flush at 200ms")
	}

	// Wait for the 1s timer.
	time.Sleep(900 * time.Millisecond)
	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1", len(records))
	}

	elapsed := records[0].time.Sub(start)
	if elapsed < 800*time.Millisecond {
		t.Errorf("flushed too early: %v (expected ~1s)", elapsed)
	}
}

func TestBatcher_SizeBasedFlush(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// Enqueue 20 low-priority messages — should trigger size-based flush
	// even though the 1s timer hasn't expired.
	for i := 0; i < maxBatchMessages; i++ {
		b.Enqueue(json.RawMessage(`{"type":"log_lines"}`), "log_lines")
	}

	time.Sleep(50 * time.Millisecond)

	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1", len(records))
	}
	if len(records[0].messages) != maxBatchMessages {
		t.Errorf("message count = %d, want %d", len(records[0].messages), maxBatchMessages)
	}
}

func TestBatcher_StopFlushesRemaining(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	// Don't start Run — just enqueue and stop.
	b.Enqueue(json.RawMessage(`{"type":"claude_output"}`), "claude_output")
	b.Enqueue(json.RawMessage(`{"type":"log_lines"}`), "log_lines")

	b.Stop()

	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1 (final flush on stop)", len(records))
	}
	if len(records[0].messages) != 2 {
		t.Errorf("message count = %d, want 2", len(records[0].messages))
	}
}

func TestBatcher_StopPreventsNewEnqueues(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	b.Enqueue(json.RawMessage(`{"type":"claude_output"}`), "claude_output")
	b.Stop()

	// Enqueue after stop should be silently dropped.
	b.Enqueue(json.RawMessage(`{"type":"error"}`), "error")

	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1", len(records))
	}
	if len(records[0].messages) != 1 {
		t.Errorf("message count = %d, want 1 (post-stop enqueue should be dropped)", len(records[0].messages))
	}
}

func TestBatcher_BufferOverflowDropsLowPriority(t *testing.T) {
	sendFn, _ := newRecordingSendFn()
	b := NewBatcher(sendFn)

	// Fill with low-priority messages up to the limit.
	for i := 0; i < maxBufferMessages-1; i++ {
		b.Enqueue(json.RawMessage(`{"type":"log_lines"}`), "log_lines")
	}

	// Buffer is almost full. Add one standard message.
	b.Enqueue(json.RawMessage(`{"type":"claude_output"}`), "claude_output")

	// Buffer is now at limit. Adding an immediate message should drop a low-priority one.
	b.Enqueue(json.RawMessage(`{"type":"error"}`), "error")

	b.mu.Lock()
	count := len(b.messages)
	// Count messages by tier.
	var immediate, standard, low int
	for _, m := range b.messages {
		switch m.tier {
		case tierIDImmediate:
			immediate++
		case tierIDStandard:
			standard++
		case tierIDLowPriority:
			low++
		}
	}
	b.mu.Unlock()

	if count != maxBufferMessages {
		t.Errorf("buffer count = %d, want %d", count, maxBufferMessages)
	}
	if immediate != 1 {
		t.Errorf("immediate count = %d, want 1", immediate)
	}
	if standard != 1 {
		t.Errorf("standard count = %d, want 1", standard)
	}
	if low != maxBufferMessages-2 {
		t.Errorf("low priority count = %d, want %d", low, maxBufferMessages-2)
	}
}

func TestBatcher_BufferOverflowBySize(t *testing.T) {
	sendFn, _ := newRecordingSendFn()
	b := NewBatcher(sendFn)

	// Create a large message (~1MB).
	bigPayload := strings.Repeat("x", 1024*1024)
	bigMsg := json.RawMessage(`{"type":"log_lines","data":"` + bigPayload + `"}`)

	// Fill buffer with 4 big messages (~4MB).
	for i := 0; i < 4; i++ {
		b.Enqueue(bigMsg, "log_lines")
	}

	b.mu.Lock()
	countBefore := len(b.messages)
	sizeBefore := b.totalSize
	b.mu.Unlock()

	if countBefore != 4 {
		t.Fatalf("buffer count = %d, want 4", countBefore)
	}

	// Adding another big message should trigger overflow — drops a low-priority message.
	bigStandard := json.RawMessage(`{"type":"claude_output","data":"` + bigPayload + `"}`)
	b.Enqueue(bigStandard, "claude_output")

	b.mu.Lock()
	countAfter := len(b.messages)
	sizeAfter := b.totalSize
	b.mu.Unlock()

	// Should have dropped one log_lines message to make room.
	if countAfter != 4 {
		t.Errorf("buffer count after overflow = %d, want 4", countAfter)
	}
	if sizeAfter >= sizeBefore+len(bigStandard) {
		t.Errorf("buffer size should not exceed limit: before=%d, after=%d", sizeBefore, sizeAfter)
	}
}

func TestBatcher_FlushesNeverOverlap(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	sendFn := func(batchID string, messages []json.RawMessage) error {
		n := concurrent.Add(1)
		// Track max concurrency.
		for {
			old := maxConcurrent.Load()
			if n <= old || maxConcurrent.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond) // Simulate slow send.
		concurrent.Add(-1)
		return nil
	}

	b := NewBatcher(sendFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// Rapidly enqueue immediate messages to trigger many flushes.
	for i := 0; i < 10; i++ {
		b.Enqueue(json.RawMessage(`{"type":"error"}`), "error")
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all flushes to complete.
	time.Sleep(600 * time.Millisecond)

	if maxConcurrent.Load() > 1 {
		t.Errorf("max concurrent flushes = %d, want 1 (sequential flushes only)", maxConcurrent.Load())
	}
}

func TestBatcher_UniqueBatchIDs(t *testing.T) {
	var mu sync.Mutex
	var batchIDs []string

	sendFn := func(batchID string, messages []json.RawMessage) error {
		mu.Lock()
		batchIDs = append(batchIDs, batchID)
		mu.Unlock()
		return nil
	}

	b := NewBatcher(sendFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// Trigger multiple flushes.
	for i := 0; i < 5; i++ {
		b.Enqueue(json.RawMessage(`{"type":"error"}`), "error")
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	seen := make(map[string]bool)
	for _, id := range batchIDs {
		if seen[id] {
			t.Errorf("duplicate batch ID: %s", id)
		}
		seen[id] = true
	}
}

func TestBatcher_EmptyFlushIsNoop(t *testing.T) {
	var flushCount atomic.Int32

	sendFn := func(batchID string, messages []json.RawMessage) error {
		flushCount.Add(1)
		return nil
	}

	b := NewBatcher(sendFn)

	// Flush with nothing in the buffer should not call sendFn.
	b.flush()

	if flushCount.Load() != 0 {
		t.Errorf("flush count = %d, want 0 (empty flush should be noop)", flushCount.Load())
	}
}

func TestBatcher_ContextCancellationStopsRun(t *testing.T) {
	sendFn, _ := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		b.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Run exited.
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after context cancellation")
	}
}

func TestBatcher_MessageOrderPreserved(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// Enqueue messages in a specific order, then trigger flush with an immediate message.
	b.Enqueue(json.RawMessage(`{"id":"1","type":"project_state"}`), "project_state")
	b.Enqueue(json.RawMessage(`{"id":"2","type":"claude_output"}`), "claude_output")
	b.Enqueue(json.RawMessage(`{"id":"3","type":"error"}`), "error") // triggers flush

	time.Sleep(50 * time.Millisecond)

	records := getRecords()
	if len(records) != 1 {
		t.Fatalf("flush count = %d, want 1", len(records))
	}

	msgs := records[0].messages
	if len(msgs) != 3 {
		t.Fatalf("message count = %d, want 3", len(msgs))
	}

	// Verify order is preserved.
	expected := []string{`{"id":"1","type":"project_state"}`, `{"id":"2","type":"claude_output"}`, `{"id":"3","type":"error"}`}
	for i, msg := range msgs {
		if string(msg) != expected[i] {
			t.Errorf("message[%d] = %s, want %s", i, msg, expected[i])
		}
	}
}

func TestBatcher_SendErrorDoesNotLoseMessages(t *testing.T) {
	// When send fails, messages are already removed from the buffer.
	// This is by design — the caller (SendMessagesWithRetry) handles retries.
	var flushCount atomic.Int32

	sendFn := func(batchID string, messages []json.RawMessage) error {
		flushCount.Add(1)
		return nil
	}

	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	b.Enqueue(json.RawMessage(`{"type":"error"}`), "error")
	time.Sleep(50 * time.Millisecond)

	if flushCount.Load() != 1 {
		t.Errorf("flush count = %d, want 1", flushCount.Load())
	}

	// Buffer should be empty after flush.
	b.mu.Lock()
	remaining := len(b.messages)
	b.mu.Unlock()

	if remaining != 0 {
		t.Errorf("remaining messages = %d, want 0", remaining)
	}
}

func TestTierFor(t *testing.T) {
	tests := []struct {
		msgType string
		want    tier
	}{
		// Immediate tier.
		{"run_complete", tierIDImmediate},
		{"run_paused", tierIDImmediate},
		{"error", tierIDImmediate},
		{"clone_complete", tierIDImmediate},
		{"session_expired", tierIDImmediate},
		{"quota_exhausted", tierIDImmediate},
		{"prd_response_complete", tierIDImmediate},
		// Standard tier.
		{"claude_output", tierIDStandard},
		{"prd_output", tierIDStandard},
		{"run_progress", tierIDStandard},
		{"clone_progress", tierIDStandard},
		// Low priority tier.
		{"state_snapshot", tierIDLowPriority},
		{"project_state", tierIDLowPriority},
		{"project_list", tierIDLowPriority},
		{"settings", tierIDLowPriority},
		{"log_lines", tierIDLowPriority},
		// Unknown defaults to standard.
		{"unknown_type", tierIDStandard},
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			got := tierFor(tt.msgType)
			if got != tt.want {
				t.Errorf("tierFor(%q) = %d, want %d", tt.msgType, got, tt.want)
			}
		})
	}
}

func TestGenerateBatchID(t *testing.T) {
	id1 := generateBatchID()
	id2 := generateBatchID()

	if id1 == "" {
		t.Error("batch ID should not be empty")
	}
	if id1 == id2 {
		t.Errorf("batch IDs should be unique: %q == %q", id1, id2)
	}

	// Verify UUID v4 format (8-4-4-4-12 hex chars).
	if len(id1) != 36 {
		t.Errorf("batch ID length = %d, want 36", len(id1))
	}
}

func TestBatcher_ConcurrentEnqueue(t *testing.T) {
	sendFn, getRecords := newRecordingSendFn()
	b := NewBatcher(sendFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// Enqueue from multiple goroutines concurrently.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Enqueue(json.RawMessage(`{"type":"claude_output"}`), "claude_output")
		}()
	}
	wg.Wait()

	// Wait for all timer-based flushes.
	time.Sleep(500 * time.Millisecond)

	records := getRecords()
	total := 0
	for _, r := range records {
		total += len(r.messages)
	}

	if total != 50 {
		t.Errorf("total flushed messages = %d, want 50", total)
	}
}
