package tui

import (
	"strings"
	"testing"
)

func TestWorktreeSpinnerConfigure(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "")

	if s.prdName != "auth" {
		t.Errorf("expected prdName 'auth', got %q", s.prdName)
	}
	if s.branchName != "chief/auth" {
		t.Errorf("expected branchName 'chief/auth', got %q", s.branchName)
	}
	if s.defaultBranch != "main" {
		t.Errorf("expected defaultBranch 'main', got %q", s.defaultBranch)
	}

	// Without setup command, should have 2 steps
	if len(s.steps) != 2 {
		t.Errorf("expected 2 steps without setup, got %d", len(s.steps))
	}
	if !s.steps[0].active {
		t.Error("expected first step to be active")
	}
}

func TestWorktreeSpinnerConfigureWithSetup(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "npm install")

	// With setup command, should have 3 steps
	if len(s.steps) != 3 {
		t.Errorf("expected 3 steps with setup, got %d", len(s.steps))
	}
	if !s.HasSetupCommand() {
		t.Error("expected HasSetupCommand to be true")
	}
}

func TestWorktreeSpinnerAdvanceStep(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "npm install")

	// Initially at step 0
	if s.GetCurrentStep() != SpinnerStepCreateBranch {
		t.Errorf("expected step CreateBranch, got %d", s.GetCurrentStep())
	}
	if s.IsDone() {
		t.Error("should not be done at start")
	}

	// Advance to worktree step
	s.AdvanceStep()
	if s.GetCurrentStep() != SpinnerStepCreateWorktree {
		t.Errorf("expected step CreateWorktree, got %d", s.GetCurrentStep())
	}
	if !s.steps[0].complete {
		t.Error("first step should be complete")
	}
	if !s.steps[1].active {
		t.Error("second step should be active")
	}

	// Advance to setup step
	s.AdvanceStep()
	if s.GetCurrentStep() != SpinnerStepRunSetup {
		t.Errorf("expected step RunSetup, got %d", s.GetCurrentStep())
	}

	// Advance to done
	s.AdvanceStep()
	if !s.IsDone() {
		t.Error("should be done after all steps")
	}
}

func TestWorktreeSpinnerAdvanceStepSkipsSetup(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "")

	// Advance past branch
	s.AdvanceStep()
	// Advance past worktree - should skip setup since no command
	s.AdvanceStep()

	if !s.IsDone() {
		t.Error("should be done after skipping setup step")
	}
}

func TestWorktreeSpinnerSetError(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "")

	s.SetError("branch already exists")

	if !s.HasError() {
		t.Error("should have error")
	}
	if s.steps[0].errMsg != "branch already exists" {
		t.Errorf("expected error on step 0, got %q", s.steps[0].errMsg)
	}
	if s.steps[0].active {
		t.Error("step with error should not be active")
	}
}

func TestWorktreeSpinnerCancel(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "")

	if s.IsCancelled() {
		t.Error("should not be cancelled initially")
	}

	s.Cancel()
	if !s.IsCancelled() {
		t.Error("should be cancelled after Cancel()")
	}
}

func TestWorktreeSpinnerTick(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "")

	if s.spinnerFrame != 0 {
		t.Errorf("expected initial frame 0, got %d", s.spinnerFrame)
	}

	s.Tick()
	if s.spinnerFrame != 1 {
		t.Errorf("expected frame 1 after tick, got %d", s.spinnerFrame)
	}
}

func TestWorktreeSpinnerRender(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "npm install")
	s.SetSize(80, 24)

	rendered := s.Render()

	// Should contain the title
	if !strings.Contains(rendered, "Setting up worktree") {
		t.Error("rendered output should contain title")
	}

	// Should contain branch name
	if !strings.Contains(rendered, "chief/auth") {
		t.Error("rendered output should contain branch name")
	}

	// Should contain worktree path
	if !strings.Contains(rendered, ".chief/worktrees/auth/") {
		t.Error("rendered output should contain worktree path")
	}

	// Should contain setup command
	if !strings.Contains(rendered, "npm install") {
		t.Error("rendered output should contain setup command")
	}

	// Should contain Esc hint
	if !strings.Contains(rendered, "Esc") {
		t.Error("rendered output should contain Esc hint")
	}
}

func TestWorktreeSpinnerRenderComplete(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "")
	s.SetSize(80, 24)

	// Complete all steps
	s.AdvanceStep() // branch
	s.AdvanceStep() // worktree (skips setup)

	rendered := s.Render()

	// Should show "Starting loop..."
	if !strings.Contains(rendered, "Starting loop...") {
		t.Error("rendered done state should contain 'Starting loop...'")
	}

	// Should show checkmarks
	if !strings.Contains(rendered, "✓") {
		t.Error("rendered done state should contain checkmarks")
	}
}

func TestWorktreeSpinnerRenderError(t *testing.T) {
	s := NewWorktreeSpinner()
	s.Configure("auth", "chief/auth", "main", ".chief/worktrees/auth/", "")
	s.SetSize(80, 24)

	s.SetError("branch already exists")

	rendered := s.Render()

	// Should show error marker
	if !strings.Contains(rendered, "✗") {
		t.Error("rendered error state should contain error marker")
	}

	// Should show error message
	if !strings.Contains(rendered, "branch already exists") {
		t.Error("rendered error state should contain error message")
	}

	// Should show cleanup hint
	if !strings.Contains(rendered, "clean up") {
		t.Error("rendered error state should contain cleanup hint")
	}
}
