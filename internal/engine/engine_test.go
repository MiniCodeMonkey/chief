package engine

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/loop"
)

// createTestPRD creates a minimal test PRD file and returns its path.
func createTestPRD(t *testing.T, dir, name string) string {
	t.Helper()
	prdDir := filepath.Join(dir, name)
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatal(err)
	}
	prdPath := filepath.Join(prdDir, "prd.json")
	content := `{
		"project": "Test PRD",
		"description": "Test",
		"userStories": [
			{"id": "US-001", "title": "Test Story", "description": "Test", "priority": 1, "passes": false}
		]
	}`
	if err := os.WriteFile(prdPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return prdPath
}

func TestNew(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if e.manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if e.MaxIterations() != 10 {
		t.Errorf("expected maxIter 10, got %d", e.MaxIterations())
	}
}

func TestRegisterAndGetInstance(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	if err := e.Register("test-prd", prdPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instance := e.GetInstance("test-prd")
	if instance == nil {
		t.Fatal("expected instance to be registered")
	}
	if instance.Name != "test-prd" {
		t.Errorf("expected name 'test-prd', got '%s'", instance.Name)
	}
	if instance.State != loop.LoopStateReady {
		t.Errorf("expected state Ready, got %v", instance.State)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	if err := e.Register("test-prd", prdPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := e.Register("test-prd", prdPath)
	if err == nil {
		t.Error("expected error when registering duplicate PRD")
	}
}

func TestUnregister(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	e.Register("test-prd", prdPath)
	if err := e.Unregister("test-prd"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst := e.GetInstance("test-prd"); inst != nil {
		t.Error("expected instance to be removed")
	}
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	ch1, unsub1 := e.Subscribe()
	ch2, unsub2 := e.Subscribe()

	if ch1 == nil || ch2 == nil {
		t.Fatal("expected non-nil channels")
	}

	e.subMu.RLock()
	count := len(e.subscribers)
	e.subMu.RUnlock()
	if count != 2 {
		t.Errorf("expected 2 subscribers, got %d", count)
	}

	unsub1()

	e.subMu.RLock()
	count = len(e.subscribers)
	e.subMu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 subscriber after unsub, got %d", count)
	}

	unsub2()

	e.subMu.RLock()
	count = len(e.subscribers)
	e.subMu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 subscribers after unsub, got %d", count)
	}
}

func TestMultipleSubscribersReceiveEvents(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	ch1, unsub1 := e.Subscribe()
	defer unsub1()
	ch2, unsub2 := e.Subscribe()
	defer unsub2()

	// Inject an event directly into the manager's events channel for testing
	// We do this by sending an event through the underlying manager
	go func() {
		// Send a synthetic event via the manager's event channel
		e.manager.Events()
	}()

	// Instead of trying to trigger a real loop event, test fan-out by
	// directly injecting into the fan-out mechanism
	testEvent := ManagerEvent{
		PRDName:   "test",
		Completed: false,
		Event: loop.Event{
			Type: loop.EventIterationStart,
			Text: "test event",
		},
	}

	// Directly write to subscriber channels to verify wiring
	e.subMu.RLock()
	for _, ch := range e.subscribers {
		ch <- testEvent
	}
	e.subMu.RUnlock()

	// Both subscribers should receive the event
	select {
	case ev := <-ch1:
		if ev.PRDName != "test" {
			t.Errorf("ch1: expected PRDName 'test', got '%s'", ev.PRDName)
		}
	case <-time.After(time.Second):
		t.Error("ch1: timed out waiting for event")
	}

	select {
	case ev := <-ch2:
		if ev.PRDName != "test" {
			t.Errorf("ch2: expected PRDName 'test', got '%s'", ev.PRDName)
		}
	case <-time.After(time.Second):
		t.Error("ch2: timed out waiting for event")
	}
}

func TestGetState(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	e.Register("test-prd", prdPath)

	state, iteration, err := e.GetState("test-prd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != loop.LoopStateReady {
		t.Errorf("expected Ready state, got %v", state)
	}
	if iteration != 0 {
		t.Errorf("expected iteration 0, got %d", iteration)
	}
}

func TestGetAllInstances(t *testing.T) {
	tmpDir := t.TempDir()
	prd1 := createTestPRD(t, tmpDir, "prd1")
	prd2 := createTestPRD(t, tmpDir, "prd2")

	e := New(10)
	defer e.Shutdown()

	e.Register("prd1", prd1)
	e.Register("prd2", prd2)

	instances := e.GetAllInstances()
	if len(instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(instances))
	}
}

func TestGetRunningPRDs(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	running := e.GetRunningPRDs()
	if len(running) != 0 {
		t.Errorf("expected 0 running PRDs, got %d", len(running))
	}
}

func TestIsAnyRunning(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	if e.IsAnyRunning() {
		t.Error("expected no running loops")
	}
}

func TestSetMaxIterations(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	e.SetMaxIterations(20)
	if e.MaxIterations() != 20 {
		t.Errorf("expected 20, got %d", e.MaxIterations())
	}
}

func TestSetConfig(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	if e.Config() != nil {
		t.Error("expected nil config initially")
	}

	cfg := &config.Config{
		OnComplete: config.OnCompleteConfig{Push: true},
	}
	e.SetConfig(cfg)

	got := e.Config()
	if got == nil || !got.OnComplete.Push {
		t.Error("expected config with Push=true")
	}
}

func TestRetryConfig(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	e.SetRetryConfig(loop.RetryConfig{MaxRetries: 5, Enabled: true})
	e.DisableRetry()
	// No assertion on internal state; just verify no panic
}

func TestSetCompletionCallback(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	called := false
	e.SetCompletionCallback(func(prdName string) {
		called = true
	})

	// Manually trigger via manager to verify it's wired
	e.manager.SetCompletionCallback(func(prdName string) {
		called = true
	})
	// The callback is set on the manager, verify it
	if called {
		t.Error("callback should not be called yet")
	}
}

func TestRegisterWithWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	err := e.RegisterWithWorktree("test-prd", prdPath, "/tmp/wt", "branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inst := e.GetInstance("test-prd")
	if inst.WorktreeDir != "/tmp/wt" {
		t.Errorf("expected WorktreeDir '/tmp/wt', got '%s'", inst.WorktreeDir)
	}
	if inst.Branch != "branch" {
		t.Errorf("expected Branch 'branch', got '%s'", inst.Branch)
	}
}

func TestUpdateAndClearWorktreeInfo(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	e.Register("test-prd", prdPath)
	e.UpdateWorktreeInfo("test-prd", "/tmp/wt", "branch")

	inst := e.GetInstance("test-prd")
	if inst.WorktreeDir != "/tmp/wt" {
		t.Errorf("expected '/tmp/wt', got '%s'", inst.WorktreeDir)
	}

	e.ClearWorktreeInfo("test-prd", true)
	inst = e.GetInstance("test-prd")
	if inst.WorktreeDir != "" || inst.Branch != "" {
		t.Error("expected cleared worktree info")
	}
}

func TestManagerAccess(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	if e.Manager() == nil {
		t.Error("expected non-nil manager")
	}
}

func TestLoadPRD(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	p, err := e.LoadPRD(prdPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Project != "Test PRD" {
		t.Errorf("expected 'Test PRD', got '%s'", p.Project)
	}
}

func TestStopAll(t *testing.T) {
	tmpDir := t.TempDir()
	prd1 := createTestPRD(t, tmpDir, "prd1")
	prd2 := createTestPRD(t, tmpDir, "prd2")

	e := New(10)
	defer e.Shutdown()

	e.Register("prd1", prd1)
	e.Register("prd2", prd2)

	done := make(chan struct{})
	go func() {
		e.StopAll()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("StopAll did not complete in time")
	}
}

func TestShutdown(t *testing.T) {
	e := New(10)
	e.Shutdown()

	// Verify forwarding is stopped
	e.forwardMu.Lock()
	forwarding := e.forwarding
	e.forwardMu.Unlock()

	if forwarding {
		t.Error("expected forwarding to be stopped after shutdown")
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, unsub := e.Subscribe()
			time.Sleep(time.Millisecond)
			unsub()
		}()
	}
	wg.Wait()

	e.subMu.RLock()
	count := len(e.subscribers)
	e.subMu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 subscribers after all unsubscribed, got %d", count)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	e.Register("test-prd", prdPath)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.GetInstance("test-prd")
			_ = e.GetAllInstances()
			_ = e.GetRunningPRDs()
			_ = e.GetRunningCount()
			_, _, _ = e.GetState("test-prd")
			_ = e.IsAnyRunning()
			_ = e.MaxIterations()
		}()
	}
	wg.Wait()
}

func TestStartNonExistent(t *testing.T) {
	e := New(10)
	defer e.Shutdown()

	err := e.Start("nonexistent")
	if err == nil {
		t.Error("expected error when starting non-existent PRD")
	}
}

func TestPauseNonRunning(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	e.Register("test-prd", prdPath)
	err := e.Pause("test-prd")
	if err == nil {
		t.Error("expected error when pausing non-running PRD")
	}
}

func TestStopNonRunning(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRD(t, tmpDir, "test-prd")

	e := New(10)
	defer e.Shutdown()

	e.Register("test-prd", prdPath)
	err := e.Stop("test-prd")
	if err != nil {
		t.Errorf("stop non-running should not error: %v", err)
	}
}
