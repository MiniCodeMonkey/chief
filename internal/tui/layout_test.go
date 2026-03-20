package tui

import (
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/engine"
)

func TestIsNarrowMode(t *testing.T) {
	tests := []struct {
		width    int
		expected bool
		desc     string
	}{
		{79, true, "79 columns should be narrow mode"},
		{80, true, "80 columns (minWidth) should be narrow mode"},
		{99, true, "99 columns should be narrow mode"},
		{100, false, "100 columns (threshold) should NOT be narrow mode"},
		{120, false, "120 columns should NOT be narrow mode"},
		{200, false, "200 columns should NOT be narrow mode"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			app := &App{width: tt.width}
			if got := app.isNarrowMode(); got != tt.expected {
				t.Errorf("isNarrowMode() at width %d = %v, want %v", tt.width, got, tt.expected)
			}
		})
	}
}

func TestNarrowWidthThreshold(t *testing.T) {
	// Verify the threshold constant is set correctly
	if narrowWidthThreshold != 100 {
		t.Errorf("narrowWidthThreshold = %d, want 100", narrowWidthThreshold)
	}
}

func TestMinWidth(t *testing.T) {
	// Verify minimum supported width
	if minWidth != 80 {
		t.Errorf("minWidth = %d, want 80", minWidth)
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	tests := []struct {
		text     string
		maxLen   int
		expected string
		desc     string
	}{
		{"Hello World", 20, "Hello World", "text shorter than maxLen"},
		{"Hello World", 11, "Hello World", "text exactly maxLen"},
		{"Hello World", 10, "Hello W...", "text needs truncation"},
		{"Hello World", 5, "He...", "aggressive truncation"},
		{"Hi", 5, "Hi", "short text stays unchanged"},
		{"Hello", 3, "Hel", "maxLen <= 3 does not add ellipsis"},
		{"Hello", 2, "He", "maxLen = 2"},
		{"Hello", 1, "H", "maxLen = 1"},
		{"Hello", 0, "", "maxLen = 0 returns empty (no space for characters)"},
		{"", 10, "", "empty string"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := truncateWithEllipsis(tt.text, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateWithEllipsis(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestStackedLayoutHeightCalculations(t *testing.T) {
	// Test that stacked layout properly divides height
	tests := []struct {
		totalHeight    int
		expectedStory  int // minimum expected story panel height
		expectedDetail int // minimum expected detail panel height
		desc           string
	}{
		{20, 5, 5, "small terminal (20 lines)"},
		{30, 5, 10, "medium terminal (30 lines)"},
		{50, 10, 20, "large terminal (50 lines)"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Simulate the stacked layout calculation
			contentHeight := tt.totalHeight - headerHeight - footerHeight - 2
			storiesHeight := max((contentHeight*40)/100, 5)
			detailsHeight := contentHeight - storiesHeight - 1

			if storiesHeight < tt.expectedStory {
				t.Errorf("storiesHeight = %d, want at least %d", storiesHeight, tt.expectedStory)
			}
			if detailsHeight < tt.expectedDetail && contentHeight > 15 {
				// Only check details minimum for terminals with enough space
				t.Errorf("detailsHeight = %d, want at least %d", detailsHeight, tt.expectedDetail)
			}
		})
	}
}

func TestLayoutConstants(t *testing.T) {
	// Verify layout constants are reasonable
	if storiesPanelPct+detailsPanelPct != 100 {
		t.Errorf("Panel percentages should sum to 100, got %d", storiesPanelPct+detailsPanelPct)
	}

	if storiesPanelPct < 20 || storiesPanelPct > 50 {
		t.Errorf("storiesPanelPct = %d, should be between 20-50%%", storiesPanelPct)
	}

	if headerHeight < 2 || headerHeight > 5 {
		t.Errorf("headerHeight = %d, should be between 2-5", headerHeight)
	}

	if footerHeight < 2 || footerHeight > 5 {
		t.Errorf("footerHeight = %d, should be between 2-5", footerHeight)
	}
}

func TestWideLayoutPanelWidths(t *testing.T) {
	tests := []struct {
		terminalWidth int
		desc          string
	}{
		{100, "at threshold (100)"},
		{120, "standard terminal (120)"},
		{160, "wide terminal (160)"},
		{200, "extra wide terminal (200)"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			storiesWidth := (tt.terminalWidth * storiesPanelPct / 100) - 2
			detailsWidth := tt.terminalWidth - storiesWidth - 4

			// Both panels should have positive width
			if storiesWidth <= 0 {
				t.Errorf("storiesWidth = %d at width %d, should be positive", storiesWidth, tt.terminalWidth)
			}
			if detailsWidth <= 0 {
				t.Errorf("detailsWidth = %d at width %d, should be positive", detailsWidth, tt.terminalWidth)
			}

			// Combined width should not exceed terminal width
			if storiesWidth+detailsWidth+4 > tt.terminalWidth {
				t.Errorf("combined panel widths exceed terminal width: %d + %d + 4 > %d",
					storiesWidth, detailsWidth, tt.terminalWidth)
			}
		})
	}
}

func TestNarrowLayoutPanelWidths(t *testing.T) {
	tests := []struct {
		terminalWidth int
		desc          string
	}{
		{80, "minimum (80)"},
		{85, "narrow (85)"},
		{95, "near threshold (95)"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			panelWidth := tt.terminalWidth - 2 // As used in renderStackedDashboard

			// Panel should have positive width
			if panelWidth <= 0 {
				t.Errorf("panelWidth = %d at width %d, should be positive", panelWidth, tt.terminalWidth)
			}

			// Panel width should not exceed terminal width
			if panelWidth >= tt.terminalWidth {
				t.Errorf("panelWidth %d should be less than terminal width %d", panelWidth, tt.terminalWidth)
			}
		})
	}
}

func TestMinMaxHelpers(t *testing.T) {
	// Test the min/max helper functions
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should return 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should return 5")
	}
	if min(5, 5) != 5 {
		t.Error("min(5, 5) should return 5")
	}

	if max(5, 10) != 10 {
		t.Error("max(5, 10) should return 10")
	}
	if max(10, 5) != 10 {
		t.Error("max(10, 5) should return 10")
	}
	if max(5, 5) != 5 {
		t.Error("max(5, 5) should return 5")
	}
}

// newTestEngine creates a test engine with the given PRD registered.
func newTestEngine(name, prdPath string) *engine.Engine {
	eng := engine.New(10)
	if prdPath != "" {
		eng.Register(name, prdPath)
	}
	return eng
}

// newTestEngineWithWorktree creates a test engine with a worktree-registered PRD.
func newTestEngineWithWorktree(name, prdPath, worktreeDir, branch string) *engine.Engine {
	eng := engine.New(10)
	eng.RegisterWithWorktree(name, prdPath, worktreeDir, branch)
	return eng
}

func TestGetWorktreeInfo_NoBranch(t *testing.T) {
	// No engine - should return empty
	app := &App{prdName: "auth"}
	branch, dir := app.getWorktreeInfo()
	if branch != "" || dir != "" {
		t.Errorf("expected empty worktree info without engine, got branch=%q dir=%q", branch, dir)
	}
}

func TestGetWorktreeInfo_WithBranch(t *testing.T) {
	eng := newTestEngineWithWorktree("auth", "/tmp/prd.json", "/tmp/.chief/worktrees/auth", "chief/auth")
	defer eng.Shutdown()

	app := &App{prdName: "auth", eng: eng}
	branch, dir := app.getWorktreeInfo()
	if branch != "chief/auth" {
		t.Errorf("branch = %q, want %q", branch, "chief/auth")
	}
	if dir != ".chief/worktrees/auth/" {
		t.Errorf("dir = %q, want %q", dir, ".chief/worktrees/auth/")
	}
}

func TestGetWorktreeInfo_WithBranchNoWorktree(t *testing.T) {
	// Branch set but no worktree dir (branch-only mode)
	eng := newTestEngineWithWorktree("auth", "/tmp/prd.json", "", "chief/auth")
	defer eng.Shutdown()

	app := &App{prdName: "auth", eng: eng}
	branch, dir := app.getWorktreeInfo()
	if branch != "chief/auth" {
		t.Errorf("branch = %q, want %q", branch, "chief/auth")
	}
	if dir != "./ (current directory)" {
		t.Errorf("dir = %q, want %q", dir, "./ (current directory)")
	}
}

func TestGetWorktreeInfo_RegisteredNoBranch(t *testing.T) {
	// Registered without worktree - should return empty (backward compatible)
	eng := newTestEngine("auth", "/tmp/prd.json")
	defer eng.Shutdown()

	app := &App{prdName: "auth", eng: eng}
	branch, dir := app.getWorktreeInfo()
	if branch != "" || dir != "" {
		t.Errorf("expected empty worktree info for no-branch PRD, got branch=%q dir=%q", branch, dir)
	}
}

func TestHasWorktreeInfo(t *testing.T) {
	// No engine
	app := &App{prdName: "auth"}
	if app.hasWorktreeInfo() {
		t.Error("expected hasWorktreeInfo=false without engine")
	}

	// With branch
	eng := newTestEngineWithWorktree("auth", "/tmp/prd.json", "/tmp/.chief/worktrees/auth", "chief/auth")
	defer eng.Shutdown()

	app.eng = eng
	if !app.hasWorktreeInfo() {
		t.Error("expected hasWorktreeInfo=true with branch set")
	}
}

func TestEffectiveHeaderHeight_NoBranch(t *testing.T) {
	app := &App{prdName: "auth"}
	if got := app.effectiveHeaderHeight(); got != headerHeight {
		t.Errorf("effectiveHeaderHeight() = %d, want %d (no branch)", got, headerHeight)
	}
}

func TestEffectiveHeaderHeight_WithBranch(t *testing.T) {
	eng := newTestEngineWithWorktree("auth", "/tmp/prd.json", "/tmp/.chief/worktrees/auth", "chief/auth")
	defer eng.Shutdown()

	app := &App{prdName: "auth", eng: eng}
	if got := app.effectiveHeaderHeight(); got != headerHeight+1 {
		t.Errorf("effectiveHeaderHeight() = %d, want %d (with branch)", got, headerHeight+1)
	}
}

func TestRenderWorktreeInfoLine_NoBranch(t *testing.T) {
	app := &App{prdName: "auth"}
	if got := app.renderWorktreeInfoLine(); got != "" {
		t.Errorf("renderWorktreeInfoLine() should be empty for no-branch, got %q", got)
	}
}

func TestRenderWorktreeInfoLine_WithBranch(t *testing.T) {
	eng := newTestEngineWithWorktree("auth", "/tmp/prd.json", "/tmp/.chief/worktrees/auth", "chief/auth")
	defer eng.Shutdown()

	app := &App{prdName: "auth", eng: eng}
	got := app.renderWorktreeInfoLine()
	if got == "" {
		t.Error("renderWorktreeInfoLine() should not be empty with branch set")
	}
	if !strings.Contains(got, "branch:") {
		t.Errorf("renderWorktreeInfoLine() should contain 'branch:', got %q", got)
	}
	if !strings.Contains(got, "chief/auth") {
		t.Errorf("renderWorktreeInfoLine() should contain branch name 'chief/auth', got %q", got)
	}
	if !strings.Contains(got, "dir:") {
		t.Errorf("renderWorktreeInfoLine() should contain 'dir:', got %q", got)
	}
	if !strings.Contains(got, ".chief/worktrees/auth/") {
		t.Errorf("renderWorktreeInfoLine() should contain worktree path, got %q", got)
	}
}

func TestRenderWorktreeInfoLine_BranchNoWorktree(t *testing.T) {
	eng := newTestEngineWithWorktree("auth", "/tmp/prd.json", "", "chief/auth")
	defer eng.Shutdown()

	app := &App{prdName: "auth", eng: eng}
	got := app.renderWorktreeInfoLine()
	if !strings.Contains(got, "current directory") {
		t.Errorf("renderWorktreeInfoLine() should contain 'current directory' for branch-only mode, got %q", got)
	}
}
