package loop

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// createTestPRDWithName creates a minimal test PRD file with a given name and returns its path.
func createTestPRDWithName(t *testing.T, dir, name string) string {
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

func TestNewManager(t *testing.T) {
	m := NewManager(10)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.maxIter != 10 {
		t.Errorf("expected maxIter 10, got %d", m.maxIter)
	}
	if m.instances == nil {
		t.Error("expected non-nil instances map")
	}
}

func TestManagerRegister(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDWithName(t, tmpDir, "test-prd")

	m := NewManager(10)

	// Register a new PRD
	err := m.Register("test-prd", prdPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it was registered
	instance := m.GetInstance("test-prd")
	if instance == nil {
		t.Fatal("expected instance to be registered")
	}
	if instance.Name != "test-prd" {
		t.Errorf("expected name 'test-prd', got '%s'", instance.Name)
	}
	if instance.State != LoopStateReady {
		t.Errorf("expected state Ready, got %v", instance.State)
	}

	// Try to register again - should fail
	err = m.Register("test-prd", prdPath)
	if err == nil {
		t.Error("expected error when registering duplicate PRD")
	}
}

func TestManagerUnregister(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDWithName(t, tmpDir, "test-prd")

	m := NewManager(10)
	m.Register("test-prd", prdPath)

	// Unregister
	err := m.Unregister("test-prd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it was removed
	instance := m.GetInstance("test-prd")
	if instance != nil {
		t.Error("expected instance to be removed")
	}

	// Try to unregister non-existent - should error
	err = m.Unregister("non-existent")
	if err == nil {
		t.Error("expected error when unregistering non-existent PRD")
	}
}

func TestManagerGetState(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDWithName(t, tmpDir, "test-prd")

	m := NewManager(10)
	m.Register("test-prd", prdPath)

	state, iteration, err := m.GetState("test-prd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != LoopStateReady {
		t.Errorf("expected Ready state, got %v", state)
	}
	if iteration != 0 {
		t.Errorf("expected iteration 0, got %d", iteration)
	}

	// Non-existent PRD
	_, _, err = m.GetState("non-existent")
	if err == nil {
		t.Error("expected error for non-existent PRD")
	}
}

func TestManagerGetAllInstances(t *testing.T) {
	tmpDir := t.TempDir()
	prd1Path := createTestPRDWithName(t, tmpDir, "prd1")
	prd2Path := createTestPRDWithName(t, tmpDir, "prd2")
	prd3Path := createTestPRDWithName(t, tmpDir, "prd3")

	m := NewManager(10)
	m.Register("prd1", prd1Path)
	m.Register("prd2", prd2Path)
	m.Register("prd3", prd3Path)

	instances := m.GetAllInstances()
	if len(instances) != 3 {
		t.Errorf("expected 3 instances, got %d", len(instances))
	}

	// Check all names are present
	names := make(map[string]bool)
	for _, inst := range instances {
		names[inst.Name] = true
	}
	for _, name := range []string{"prd1", "prd2", "prd3"} {
		if !names[name] {
			t.Errorf("expected %s in instances", name)
		}
	}
}

func TestManagerGetRunningPRDs(t *testing.T) {
	m := NewManager(10)

	// Initially no running PRDs
	running := m.GetRunningPRDs()
	if len(running) != 0 {
		t.Errorf("expected 0 running PRDs, got %d", len(running))
	}
}

func TestManagerGetRunningCount(t *testing.T) {
	m := NewManager(10)

	count := m.GetRunningCount()
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestManagerIsAnyRunning(t *testing.T) {
	m := NewManager(10)

	if m.IsAnyRunning() {
		t.Error("expected no running loops")
	}
}

func TestManagerPauseNonRunning(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDWithName(t, tmpDir, "test-prd")

	m := NewManager(10)
	m.Register("test-prd", prdPath)

	// Pause a non-running PRD should error
	err := m.Pause("test-prd")
	if err == nil {
		t.Error("expected error when pausing non-running PRD")
	}
}

func TestManagerStopNonRunning(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDWithName(t, tmpDir, "test-prd")

	m := NewManager(10)
	m.Register("test-prd", prdPath)

	// Stop a non-running PRD should not error (idempotent)
	err := m.Stop("test-prd")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestManagerStartNonExistent(t *testing.T) {
	m := NewManager(10)

	err := m.Start("non-existent")
	if err == nil {
		t.Error("expected error when starting non-existent PRD")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDWithName(t, tmpDir, "test-prd")

	m := NewManager(10)
	m.Register("test-prd", prdPath)

	// Test concurrent access to manager methods
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.GetInstance("test-prd")
			_ = m.GetAllInstances()
			_ = m.GetRunningPRDs()
			_ = m.GetRunningCount()
			_, _, _ = m.GetState("test-prd")
		}()
	}
	wg.Wait()
}

func TestLoopStateString(t *testing.T) {
	tests := []struct {
		state    LoopState
		expected string
	}{
		{LoopStateReady, "Ready"},
		{LoopStateRunning, "Running"},
		{LoopStatePaused, "Paused"},
		{LoopStateStopped, "Stopped"},
		{LoopStateComplete, "Complete"},
		{LoopStateError, "Error"},
		{LoopState(99), "Unknown"},
	}

	for _, test := range tests {
		if got := test.state.String(); got != test.expected {
			t.Errorf("expected %s, got %s", test.expected, got)
		}
	}
}

func TestManagerSetCompletionCallback(t *testing.T) {
	m := NewManager(10)

	called := false
	var calledWith string
	m.SetCompletionCallback(func(prdName string) {
		called = true
		calledWith = prdName
	})

	// Verify callback is stored
	m.mu.RLock()
	if m.onComplete == nil {
		t.Error("expected callback to be set")
	}
	m.mu.RUnlock()

	// Manually call it to verify it works
	m.onComplete("test-prd")
	if !called {
		t.Error("callback was not called")
	}
	if calledWith != "test-prd" {
		t.Errorf("expected 'test-prd', got '%s'", calledWith)
	}
}

func TestManagerStopAll(t *testing.T) {
	tmpDir := t.TempDir()
	prd1Path := createTestPRDWithName(t, tmpDir, "prd1")
	prd2Path := createTestPRDWithName(t, tmpDir, "prd2")

	m := NewManager(10)
	m.Register("prd1", prd1Path)
	m.Register("prd2", prd2Path)

	// StopAll should work even when nothing is running
	done := make(chan struct{})
	go func() {
		m.StopAll()
		close(done)
	}()

	select {
	case <-done:
		// Good, StopAll completed
	case <-time.After(time.Second):
		t.Error("StopAll did not complete in time")
	}
}

func TestManagerSetMaxIterations(t *testing.T) {
	m := NewManager(10)

	if m.MaxIterations() != 10 {
		t.Errorf("expected initial maxIter 10, got %d", m.MaxIterations())
	}

	m.SetMaxIterations(20)

	if m.MaxIterations() != 20 {
		t.Errorf("expected maxIter 20, got %d", m.MaxIterations())
	}
}

func TestManagerRetryConfig(t *testing.T) {
	m := NewManager(10)

	// Check default retry config
	if !m.retryConfig.Enabled {
		t.Error("expected default retry to be enabled")
	}

	// Disable retry
	m.DisableRetry()
	if m.retryConfig.Enabled {
		t.Error("expected retry to be disabled")
	}

	// Set custom retry config
	m.SetRetryConfig(RetryConfig{
		MaxRetries: 5,
		Enabled:    true,
	})

	if m.retryConfig.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", m.retryConfig.MaxRetries)
	}
}
