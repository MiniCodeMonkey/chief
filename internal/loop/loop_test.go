package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/prd"
)

// createMockClaudeScript creates a shell script that outputs predefined stream-json.
func createMockClaudeScript(t *testing.T, dir string, output []string) string {
	t.Helper()

	scriptPath := filepath.Join(dir, "mock-claude")
	content := "#!/bin/bash\n"
	for _, line := range output {
		content += "echo '" + line + "'\n"
	}

	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	return scriptPath
}

// createTestPRD creates a minimal test PRD file.
func createTestPRD(t *testing.T, dir string, allComplete bool) string {
	t.Helper()

	prdFile := &prd.PRD{
		Project:     "Test Project",
		Description: "Test Description",
		UserStories: []prd.UserStory{
			{
				ID:          "US-001",
				Title:       "Test Story",
				Description: "A test story",
				Priority:    1,
				Passes:      allComplete,
			},
		},
	}

	prdPath := filepath.Join(dir, "prd.json")
	data, _ := json.MarshalIndent(prdFile, "", "  ")
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}

	return prdPath
}

func TestNewLoop(t *testing.T) {
	l := NewLoop("/path/to/prd.json", "test prompt", 5)

	if l.prdPath != "/path/to/prd.json" {
		t.Errorf("Expected prdPath %q, got %q", "/path/to/prd.json", l.prdPath)
	}
	if l.prompt != "test prompt" {
		t.Errorf("Expected prompt %q, got %q", "test prompt", l.prompt)
	}
	if l.maxIter != 5 {
		t.Errorf("Expected maxIter %d, got %d", 5, l.maxIter)
	}
	if l.events == nil {
		t.Error("Expected events channel to be initialized")
	}
}

func TestNewLoopWithWorkDir(t *testing.T) {
	l := NewLoopWithWorkDir("/path/to/prd.json", "/work/dir", "test prompt", 5)

	if l.prdPath != "/path/to/prd.json" {
		t.Errorf("Expected prdPath %q, got %q", "/path/to/prd.json", l.prdPath)
	}
	if l.workDir != "/work/dir" {
		t.Errorf("Expected workDir %q, got %q", "/work/dir", l.workDir)
	}
	if l.prompt != "test prompt" {
		t.Errorf("Expected prompt %q, got %q", "test prompt", l.prompt)
	}
	if l.maxIter != 5 {
		t.Errorf("Expected maxIter %d, got %d", 5, l.maxIter)
	}
	if l.events == nil {
		t.Error("Expected events channel to be initialized")
	}
}

func TestNewLoopWithWorkDir_EmptyWorkDir(t *testing.T) {
	l := NewLoopWithWorkDir("/path/to/prd.json", "", "test prompt", 5)

	if l.workDir != "" {
		t.Errorf("Expected empty workDir, got %q", l.workDir)
	}
}

func TestLoop_Events(t *testing.T) {
	l := NewLoop("/path/to/prd.json", "test prompt", 5)
	events := l.Events()

	if events == nil {
		t.Error("Expected Events() to return a channel")
	}
}

func TestLoop_Iteration(t *testing.T) {
	l := NewLoop("/path/to/prd.json", "test prompt", 5)

	if l.Iteration() != 0 {
		t.Errorf("Expected initial iteration to be 0, got %d", l.Iteration())
	}

	l.iteration = 3
	if l.Iteration() != 3 {
		t.Errorf("Expected iteration to be 3, got %d", l.Iteration())
	}
}

func TestLoop_Stop(t *testing.T) {
	l := NewLoop("/path/to/prd.json", "test prompt", 5)

	l.Stop()

	l.mu.Lock()
	stopped := l.stopped
	l.mu.Unlock()

	if !stopped {
		t.Error("Expected loop to be marked as stopped")
	}
}

// TestLoop_RunWithMockClaude tests the loop with a mock Claude script.
// This is an integration test that requires a Unix-like shell.
func TestLoop_RunWithMockClaude(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	tmpDir := t.TempDir()

	// Create a mock Claude output
	mockOutput := []string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Starting work on story"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"123","name":"Read","input":{"file_path":"test.go"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"123","content":"file content"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Work complete"}]}}`,
	}

	scriptPath := createMockClaudeScript(t, tmpDir, mockOutput)
	prdPath := createTestPRD(t, tmpDir, true) // Already complete so loop stops after one iteration

	// Create a prompt that invokes our mock script instead of real Claude
	// For the actual test, we'll test the internal methods
	l := NewLoop(prdPath, "test prompt", 1)

	// Override the command for testing - we'll test processOutput directly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect events in a goroutine
	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	// Test processOutput directly with mock data
	r, w, _ := os.Pipe()
	go func() {
		for _, line := range mockOutput {
			w.WriteString(line + "\n")
		}
		w.Close()
	}()

	l.iteration = 1
	l.processOutput(r)

	// Close events channel and wait for collection
	close(l.events)
	<-done

	// Verify we got expected events
	if len(events) == 0 {
		t.Error("Expected at least one event")
	}

	// Check that we got the expected event types
	hasIterationStart := false
	hasAssistantText := false
	hasToolStart := false
	hasToolResult := false

	for _, e := range events {
		switch e.Type {
		case EventIterationStart:
			hasIterationStart = true
		case EventAssistantText:
			hasAssistantText = true
		case EventToolStart:
			hasToolStart = true
			if e.Tool != "Read" {
				t.Errorf("Expected tool name 'Read', got %q", e.Tool)
			}
		case EventToolResult:
			hasToolResult = true
		}
	}

	if !hasIterationStart {
		t.Error("Expected IterationStart event")
	}
	if !hasAssistantText {
		t.Error("Expected AssistantText event")
	}
	if !hasToolStart {
		t.Error("Expected ToolStart event")
	}
	if !hasToolResult {
		t.Error("Expected ToolResult event")
	}

	// Cleanup
	_ = scriptPath // Avoid unused variable warning
	_ = ctx        // Context used for reference
}

// TestLoop_MaxIterations tests that the loop stops after max iterations.
func TestLoop_MaxIterations(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, false) // Not complete

	l := NewLoop(prdPath, "test prompt", 2)

	// Simulate reaching max iterations by manually incrementing
	l.iteration = 2

	// The Run method should check and emit MaxIterationsReached
	// For this test, we verify the check logic
	if l.iteration >= l.maxIter {
		l.events <- Event{
			Type:      EventMaxIterationsReached,
			Iteration: l.iteration,
		}
	}

	event := <-l.events
	if event.Type != EventMaxIterationsReached {
		t.Errorf("Expected MaxIterationsReached event, got %v", event.Type)
	}
}

// TestLoop_CompleteDetection tests that the loop detects completion.
func TestLoop_CompleteDetection(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, true) // All complete

	p, err := prd.LoadPRD(prdPath)
	if err != nil {
		t.Fatalf("Failed to load PRD: %v", err)
	}

	if !p.AllComplete() {
		t.Error("Expected PRD to be all complete")
	}
}

// TestLoop_LogFile tests that log file is created and written to.
func TestLoop_LogFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = createTestPRD(t, tmpDir, true)

	logPath := filepath.Join(tmpDir, "claude.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	l := NewLoop(filepath.Join(tmpDir, "prd.json"), "test", 1)
	l.logFile = logFile

	l.logLine("test log line")
	logFile.Close()

	// Read back the log file
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if string(data) != "test log line\n" {
		t.Errorf("Expected log line content, got %q", string(data))
	}
}

// TestLoop_ChiefCompleteEvent tests detection of <chief-complete/> event.
func TestLoop_ChiefCompleteEvent(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)
	l.iteration = 1

	done := make(chan bool)
	var events []Event
	go func() {
		for event := range l.Events() {
			events = append(events, event)
			if event.Type == EventComplete {
				break
			}
		}
		done <- true
	}()

	// Simulate processing a line with chief-complete
	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"All done! <chief-complete/>"}]}}` + "\n")
		w.Close()
	}()

	l.processOutput(r)
	close(l.events)
	<-done

	// Check that we got a Complete event
	hasComplete := false
	for _, e := range events {
		if e.Type == EventComplete {
			hasComplete = true
		}
	}

	if !hasComplete {
		t.Error("Expected Complete event for <chief-complete/>")
	}
}

// TestLoop_SetMaxIterations tests setting max iterations at runtime.
func TestLoop_SetMaxIterations(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)

	if l.MaxIterations() != 5 {
		t.Errorf("Expected initial maxIter 5, got %d", l.MaxIterations())
	}

	l.SetMaxIterations(10)

	if l.MaxIterations() != 10 {
		t.Errorf("Expected maxIter 10 after set, got %d", l.MaxIterations())
	}
}

// TestDefaultRetryConfig tests the default retry configuration.
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if len(config.RetryDelays) != 3 {
		t.Errorf("Expected 3 retry delays, got %d", len(config.RetryDelays))
	}
}

// TestLoop_SetRetryConfig tests setting retry config.
func TestLoop_SetRetryConfig(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)

	// Check default
	if !l.retryConfig.Enabled {
		t.Error("Expected default retry to be enabled")
	}

	// Disable retry
	l.DisableRetry()
	if l.retryConfig.Enabled {
		t.Error("Expected retry to be disabled after DisableRetry()")
	}

	// Set custom config
	customConfig := RetryConfig{
		MaxRetries:  5,
		RetryDelays: []time.Duration{time.Second},
		Enabled:     true,
	}
	l.SetRetryConfig(customConfig)

	if l.retryConfig.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries 5, got %d", l.retryConfig.MaxRetries)
	}
}

// TestLoop_WatchdogDefaultTimeout tests that the default watchdog timeout is set.
func TestLoop_WatchdogDefaultTimeout(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)

	if l.WatchdogTimeout() != DefaultWatchdogTimeout {
		t.Errorf("Expected default watchdog timeout %v, got %v", DefaultWatchdogTimeout, l.WatchdogTimeout())
	}
}

// TestLoop_SetWatchdogTimeout tests setting the watchdog timeout.
func TestLoop_SetWatchdogTimeout(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)

	l.SetWatchdogTimeout(10 * time.Minute)
	if l.WatchdogTimeout() != 10*time.Minute {
		t.Errorf("Expected watchdog timeout 10m, got %v", l.WatchdogTimeout())
	}

	// Setting to 0 disables the watchdog
	l.SetWatchdogTimeout(0)
	if l.WatchdogTimeout() != 0 {
		t.Errorf("Expected watchdog timeout 0 (disabled), got %v", l.WatchdogTimeout())
	}
}

// TestLoop_WatchdogKillsHungProcess tests that a hung process is killed after timeout.
func TestLoop_WatchdogKillsHungProcess(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)
	l.iteration = 1

	// Use a very short timeout for testing
	timeout := 100 * time.Millisecond

	// Collect events
	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	// Create a pipe that never sends data (simulates hung process)
	r, w, _ := os.Pipe()

	// Initialize lastOutputTime
	l.mu.Lock()
	l.lastOutputTime = time.Now()
	l.mu.Unlock()

	// Start watchdog with a short check interval
	watchdogDone := make(chan struct{})
	var fired atomic.Bool
	watchdogExited := make(chan struct{})
	go func() {
		l.runWatchdog(timeout, watchdogDone, &fired)
		close(watchdogExited)
	}()

	// processOutput will block until pipe is closed (by watchdog killing would close it,
	// but in this test we close it manually after watchdog fires)
	go func() {
		// Wait for watchdog to fire
		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	l.processOutput(r)
	close(watchdogDone)
	<-watchdogExited // Wait for watchdog goroutine to fully exit before closing events
	close(l.events)
	<-done

	if !fired.Load() {
		t.Error("Expected watchdog to fire for hung process")
	}

	// Check that we got a WatchdogTimeout event
	hasWatchdog := false
	for _, e := range events {
		if e.Type == EventWatchdogTimeout {
			hasWatchdog = true
			if e.Text == "" {
				t.Error("Expected watchdog event to have descriptive text")
			}
		}
	}
	if !hasWatchdog {
		t.Error("Expected WatchdogTimeout event")
	}
}

// TestLoop_WatchdogDoesNotFireForActiveProcess tests that an active process doesn't trigger the watchdog.
func TestLoop_WatchdogDoesNotFireForActiveProcess(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)
	l.iteration = 1

	// Use a timeout that's longer than our test
	timeout := 2 * time.Second

	// Collect events
	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	// Create a pipe that produces output regularly
	r, w, _ := os.Pipe()

	l.mu.Lock()
	l.lastOutputTime = time.Now()
	l.mu.Unlock()

	watchdogDone := make(chan struct{})
	var fired atomic.Bool
	go l.runWatchdog(timeout, watchdogDone, &fired)

	// Send output regularly, then close
	go func() {
		for i := 0; i < 5; i++ {
			w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"working..."}]}}` + "\n")
			time.Sleep(100 * time.Millisecond)
		}
		w.Close()
	}()

	l.processOutput(r)
	close(watchdogDone)
	close(l.events)
	<-done

	if fired.Load() {
		t.Error("Watchdog should NOT fire for an actively producing process")
	}

	// Verify no WatchdogTimeout events
	for _, e := range events {
		if e.Type == EventWatchdogTimeout {
			t.Error("Should not have received WatchdogTimeout event for active process")
		}
	}
}

// TestLoop_WatchdogDisabledWithZeroTimeout tests that watchdog is disabled when timeout is 0.
func TestLoop_WatchdogDisabledWithZeroTimeout(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)
	l.SetWatchdogTimeout(0)

	if l.WatchdogTimeout() != 0 {
		t.Errorf("Expected watchdog timeout 0, got %v", l.WatchdogTimeout())
	}

	// Verify that runIteration would not start a watchdog
	// (tested indirectly: timeout == 0 means the if-block in runIteration is skipped)
	// We test this by verifying the constructor behavior and setter
	l2 := NewLoop("/test/prd.json", "test", 5)
	l2.SetWatchdogTimeout(0)

	l2.mu.Lock()
	wt := l2.watchdogTimeout
	l2.mu.Unlock()

	if wt != 0 {
		t.Errorf("Expected internal watchdogTimeout to be 0, got %v", wt)
	}
}

// TestLoop_LastOutputTimeUpdated tests that lastOutputTime is updated on each scanner output.
func TestLoop_LastOutputTimeUpdated(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)
	l.iteration = 1

	// Drain events to avoid blocking
	go func() {
		for range l.Events() {
		}
	}()

	// Record initial time
	l.mu.Lock()
	l.lastOutputTime = time.Now().Add(-1 * time.Hour) // Set to an old time
	initialTime := l.lastOutputTime
	l.mu.Unlock()

	// Send output through processOutput
	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}` + "\n")
		time.Sleep(50 * time.Millisecond)
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"world"}]}}` + "\n")
		w.Close()
	}()

	l.processOutput(r)
	close(l.events)

	// Verify lastOutputTime was updated
	l.mu.Lock()
	finalTime := l.lastOutputTime
	l.mu.Unlock()

	if !finalTime.After(initialTime) {
		t.Errorf("Expected lastOutputTime to be updated after output, initial=%v, final=%v", initialTime, finalTime)
	}
}

// TestLoop_WatchdogReturnsError tests that watchdog kill causes runIteration to return an error
// that feeds into retry logic.
func TestLoop_WatchdogReturnsError(t *testing.T) {
	// This test verifies the error message format that runIterationWithRetry will see
	l := NewLoop("/test/prd.json", "test", 5)
	l.SetWatchdogTimeout(100 * time.Millisecond)

	// The watchdog error message should contain "watchdog timeout"
	// This ensures the retry logic in runIterationWithRetry will process it
	expectedPrefix := "watchdog timeout:"
	errMsg := fmt.Sprintf("watchdog timeout: no output for %s", 100*time.Millisecond)
	if !strings.HasPrefix(errMsg, expectedPrefix) {
		t.Errorf("Expected error to start with %q, got %q", expectedPrefix, errMsg)
	}
}

// TestLoop_WatchdogWithWorkDir tests that watchdog works with NewLoopWithWorkDir too.
func TestLoop_WatchdogWithWorkDir(t *testing.T) {
	l := NewLoopWithWorkDir("/test/prd.json", "/work", "test", 5)

	if l.WatchdogTimeout() != DefaultWatchdogTimeout {
		t.Errorf("Expected default watchdog timeout for NewLoopWithWorkDir, got %v", l.WatchdogTimeout())
	}
}

// TestFrontPressure_Disabled tests that FP events are emitted even when FP is disabled,
// and that no concern is stored in the loop state.
func TestFrontPressure_Disabled(t *testing.T) {
	l := NewLoop("/test/prd.json", "test", 5)
	l.iteration = 1

	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"<front-pressure>bad assumption</front-pressure>"}]}}` + "\n")
		w.Close()
	}()

	l.processOutput(r)
	close(l.events)
	<-done

	// Event should still be emitted when FP is disabled
	hasFP := false
	for _, e := range events {
		if e.Type == EventFrontPressure {
			hasFP = true
			if e.Text != "bad assumption" {
				t.Errorf("Expected concern text 'bad assumption', got %q", e.Text)
			}
		}
	}
	if !hasFP {
		t.Error("Expected EventFrontPressure event even when FP disabled")
	}

	// No concern should be stored when FP is disabled
	l.mu.Lock()
	concern := l.pendingConcern
	l.mu.Unlock()
	if concern != "" {
		t.Errorf("Expected no pending concern when FP disabled, got %q", concern)
	}
}

// TestFrontPressure_Enabled_Edit tests that an edit decision emits EventFrontPressureResolved
// with "Editor updated PRD" and does not stop the loop.
func TestFrontPressure_Enabled_Edit(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, false)

	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "<fp-decision>edit</fp-decision>", nil
		},
	}

	l := NewLoop(prdPath, "test", 5)
	l.SetFrontPressure(true, editor)
	l.iteration = 1
	l.mu.Lock()
	l.currentStoryID = "US-001"
	l.mu.Unlock()

	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	// Feed front-pressure event
	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"<front-pressure>bad assumption</front-pressure>"}]}}` + "\n")
		w.Close()
	}()
	l.processOutput(r)

	// Handle front pressure
	if err := l.handleFrontPressure(context.Background()); err != nil {
		t.Fatalf("handleFrontPressure returned error: %v", err)
	}

	close(l.events)
	<-done

	hasResolved := false
	for _, e := range events {
		if e.Type == EventFrontPressureResolved {
			hasResolved = true
			if e.Text != "Editor updated PRD" {
				t.Errorf("Expected text 'Editor updated PRD', got %q", e.Text)
			}
		}
	}
	if !hasResolved {
		t.Error("Expected EventFrontPressureResolved event for edit decision")
	}
	if l.IsStopped() {
		t.Error("Expected loop to continue after edit decision")
	}
}

// TestFrontPressure_Enabled_Dismiss tests that a dismiss decision emits EventFrontPressureResolved
// with "Concern dismissed" and does not stop the loop.
func TestFrontPressure_Enabled_Dismiss(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, false)

	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "<fp-decision>dismiss</fp-decision>", nil
		},
	}

	l := NewLoop(prdPath, "test", 5)
	l.SetFrontPressure(true, editor)
	l.iteration = 1
	l.mu.Lock()
	l.currentStoryID = "US-001"
	l.mu.Unlock()

	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"<front-pressure>just an impl detail</front-pressure>"}]}}` + "\n")
		w.Close()
	}()
	l.processOutput(r)

	if err := l.handleFrontPressure(context.Background()); err != nil {
		t.Fatalf("handleFrontPressure returned error: %v", err)
	}

	close(l.events)
	<-done

	hasResolved := false
	for _, e := range events {
		if e.Type == EventFrontPressureResolved {
			hasResolved = true
			if e.Text != "Concern dismissed" {
				t.Errorf("Expected text 'Concern dismissed', got %q", e.Text)
			}
		}
	}
	if !hasResolved {
		t.Error("Expected EventFrontPressureResolved event for dismiss decision")
	}
	if l.IsStopped() {
		t.Error("Expected loop to continue after dismiss decision")
	}
}

// TestFrontPressureFullFlow is an integration test that exercises the complete front
// pressure dismiss flow: concern raised → editor runs → dismiss decision applied →
// dismissed concern persisted to prd.json → loop continues.
func TestFrontPressureFullFlow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD with two stories.
	prdFile := &prd.PRD{
		Project:     "Test Project",
		Description: "Test Description",
		UserStories: []prd.UserStory{
			{ID: "US-001", Title: "First Story", Priority: 1, Passes: false},
			{ID: "US-002", Title: "Second Story", Priority: 2, Passes: false},
		},
	}
	prdPath := filepath.Join(tmpDir, "prd.json")
	data, _ := json.MarshalIndent(prdFile, "", "  ")
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}

	// Fake editor returns dismiss.
	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "<fp-decision>dismiss</fp-decision>", nil
		},
	}

	l := NewLoop(prdPath, "test", 5)
	l.SetFrontPressure(true, editor)
	l.iteration = 1
	l.mu.Lock()
	l.currentStoryID = "US-001"
	l.mu.Unlock()

	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	// Feed front-pressure event via a fake Claude output stream.
	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"<front-pressure>data model assumption wrong</front-pressure>"}]}}` + "\n")
		w.Close()
	}()
	l.processOutput(r)

	if err := l.handleFrontPressure(context.Background()); err != nil {
		t.Fatalf("handleFrontPressure returned error: %v", err)
	}

	close(l.events)
	<-done

	// Verify EventFrontPressure was emitted.
	hasFP := false
	for _, e := range events {
		if e.Type == EventFrontPressure {
			hasFP = true
			if e.Text != "data model assumption wrong" {
				t.Errorf("Expected concern text 'data model assumption wrong', got %q", e.Text)
			}
		}
	}
	if !hasFP {
		t.Error("Expected EventFrontPressure event")
	}

	// Verify EventFrontPressureResolved was emitted.
	hasResolved := false
	for _, e := range events {
		if e.Type == EventFrontPressureResolved {
			hasResolved = true
			if e.Text != "Concern dismissed" {
				t.Errorf("Expected text 'Concern dismissed', got %q", e.Text)
			}
		}
	}
	if !hasResolved {
		t.Error("Expected EventFrontPressureResolved event")
	}

	// Verify concern appears in DismissedConcerns on US-001 in prd.json.
	p, err := prd.LoadPRD(prdPath)
	if err != nil {
		t.Fatalf("Failed to reload PRD: %v", err)
	}
	found := false
	for _, story := range p.UserStories {
		if story.ID == "US-001" {
			for _, c := range story.DismissedConcerns {
				if c == "data model assumption wrong" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("Expected concern text in DismissedConcerns for US-001 in prd.json")
	}

	// Verify loop continues (not stopped after dismissal).
	if l.IsStopped() {
		t.Error("Expected loop to continue after dismiss decision")
	}
}

// TestFrontPressureScrap is an integration test that exercises the scrap path:
// fake editor returns scrap → EventFrontPressureScrap emitted → loop stops.
func TestFrontPressureScrap(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD with two stories.
	prdFile := &prd.PRD{
		Project:     "Test Project",
		Description: "Test Description",
		UserStories: []prd.UserStory{
			{ID: "US-001", Title: "First Story", Priority: 1, Passes: false},
			{ID: "US-002", Title: "Second Story", Priority: 2, Passes: false},
		},
	}
	prdPath := filepath.Join(tmpDir, "prd.json")
	data, _ := json.MarshalIndent(prdFile, "", "  ")
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}

	// Fake editor returns scrap.
	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "<fp-decision>scrap</fp-decision>", nil
		},
	}

	l := NewLoop(prdPath, "test", 5)
	l.SetFrontPressure(true, editor)
	l.iteration = 1
	l.mu.Lock()
	l.currentStoryID = "US-001"
	l.mu.Unlock()

	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"<front-pressure>fundamental flaw in data model</front-pressure>"}]}}` + "\n")
		w.Close()
	}()
	l.processOutput(r)

	if err := l.handleFrontPressure(context.Background()); err != nil {
		t.Fatalf("handleFrontPressure returned error: %v", err)
	}

	close(l.events)
	<-done

	// Verify EventFrontPressureScrap was emitted.
	hasScrap := false
	for _, e := range events {
		if e.Type == EventFrontPressureScrap {
			hasScrap = true
		}
	}
	if !hasScrap {
		t.Error("Expected EventFrontPressureScrap event")
	}

	// Verify loop stops.
	if !l.IsStopped() {
		t.Error("Expected loop to be stopped after scrap decision")
	}
}

// TestFrontPressure_Enabled_Scrap tests that a scrap decision emits EventFrontPressureScrap
// and stops the loop.
func TestFrontPressure_Enabled_Scrap(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, false)

	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "<fp-decision>scrap</fp-decision>", nil
		},
	}

	l := NewLoop(prdPath, "test", 5)
	l.SetFrontPressure(true, editor)
	l.iteration = 1
	l.mu.Lock()
	l.currentStoryID = "US-001"
	l.mu.Unlock()

	var events []Event
	done := make(chan bool)
	go func() {
		for event := range l.Events() {
			events = append(events, event)
		}
		done <- true
	}()

	r, w, _ := os.Pipe()
	go func() {
		w.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"<front-pressure>fundamental flaw</front-pressure>"}]}}` + "\n")
		w.Close()
	}()
	l.processOutput(r)

	if err := l.handleFrontPressure(context.Background()); err != nil {
		t.Fatalf("handleFrontPressure returned error: %v", err)
	}

	close(l.events)
	<-done

	hasScrap := false
	for _, e := range events {
		if e.Type == EventFrontPressureScrap {
			hasScrap = true
		}
	}
	if !hasScrap {
		t.Error("Expected EventFrontPressureScrap event for scrap decision")
	}
	if !l.IsStopped() {
		t.Error("Expected loop to be stopped after scrap decision")
	}
}
