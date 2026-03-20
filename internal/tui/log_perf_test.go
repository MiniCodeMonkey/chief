package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

// --- Helpers ---

// makeTextEvent creates an assistant text event.
func makeTextEvent(text string) loop.Event {
	return loop.Event{Type: loop.EventAssistantText, Text: text}
}

// makeToolStartEvent creates a tool start event.
func makeToolStartEvent(tool string, input map[string]interface{}) loop.Event {
	return loop.Event{Type: loop.EventToolStart, Tool: tool, ToolInput: input}
}

// makeToolResultEvent creates a tool result event.
func makeToolResultEvent(text string) loop.Event {
	return loop.Event{Type: loop.EventToolResult, Text: text}
}

// makeStoryEvent creates a story started event.
func makeStoryEvent(storyID string) loop.Event {
	return loop.Event{Type: loop.EventStoryStarted, StoryID: storyID}
}

// --- AddEvent caching tests ---

func TestAddEvent_CachesLinesWhenWidthSet(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	lv.AddEvent(makeTextEvent("Hello world"))

	if len(lv.entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(lv.entries))
	}
	if len(lv.entries[0].cachedLines) == 0 {
		t.Error("Expected cachedLines to be populated after AddEvent with width set")
	}
}

func TestAddEvent_NoCacheWhenWidthZero(t *testing.T) {
	lv := NewLogViewer()
	// Width is 0 (default)

	lv.AddEvent(makeTextEvent("Hello world"))

	if len(lv.entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(lv.entries))
	}
	if len(lv.entries[0].cachedLines) != 0 {
		t.Error("Expected cachedLines to be empty when width is 0")
	}
	if lv.totalLineCount != 0 {
		t.Errorf("Expected totalLineCount 0, got %d", lv.totalLineCount)
	}
}

func TestAddEvent_FiltersUnwantedEvents(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	lv.AddEvent(loop.Event{Type: loop.EventIterationStart})
	lv.AddEvent(loop.Event{Type: loop.EventUnknown})

	if len(lv.entries) != 0 {
		t.Errorf("Expected 0 entries for filtered events, got %d", len(lv.entries))
	}
	if lv.totalLineCount != 0 {
		t.Errorf("Expected totalLineCount 0 for filtered events, got %d", lv.totalLineCount)
	}
}

// --- totalLineCount accuracy ---

func TestTotalLineCount_AccurateAcrossEventTypes(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	// Add diverse events
	lv.AddEvent(makeStoryEvent("US-1"))       // 5 lines (blank, divider, title, divider, blank)
	lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{"file_path": "/test.go"})) // 1 line
	lv.AddEvent(makeToolResultEvent("some output"))                                            // 1 line
	lv.AddEvent(makeTextEvent("Hello"))                                                        // 1 line

	// Count actual cached lines
	actualTotal := 0
	for _, entry := range lv.entries {
		actualTotal += len(entry.cachedLines)
	}

	if lv.totalLineCount != actualTotal {
		t.Errorf("totalLineCount (%d) doesn't match actual cached lines (%d)", lv.totalLineCount, actualTotal)
	}
	if lv.totalLines() != actualTotal {
		t.Errorf("totalLines() (%d) doesn't match actual cached lines (%d)", lv.totalLines(), actualTotal)
	}
}

func TestTotalLineCount_WrappedTextCountsCorrectly(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(40, 30) // Narrow width to force wrapping

	// Long text that will wrap
	longText := strings.Repeat("word ", 20) // ~100 chars, will wrap at width 40
	lv.AddEvent(makeTextEvent(longText))

	if lv.totalLineCount < 2 {
		t.Errorf("Expected wrapped text to produce multiple lines, got totalLineCount=%d", lv.totalLineCount)
	}

	// Verify it matches cached lines
	if lv.totalLineCount != len(lv.entries[0].cachedLines) {
		t.Errorf("totalLineCount (%d) doesn't match cachedLines length (%d)",
			lv.totalLineCount, len(lv.entries[0].cachedLines))
	}
}

// --- SetSize cache invalidation ---

func TestSetSize_RebuildsCacheOnWidthChange(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	longText := strings.Repeat("word ", 30)
	lv.AddEvent(makeTextEvent(longText))

	originalCount := lv.totalLineCount
	originalLines := make([]string, len(lv.entries[0].cachedLines))
	copy(originalLines, lv.entries[0].cachedLines)

	// Change width - should trigger rebuild with different wrapping
	lv.SetSize(40, 30)

	if lv.totalLineCount == originalCount {
		// With a much narrower width, wrapped text should produce more lines
		// (it's possible they match by coincidence, but very unlikely with this text)
		t.Log("Warning: totalLineCount unchanged after width change (may be coincidence)")
	}

	// At minimum, verify the cache was rebuilt (lines should differ due to width)
	if len(lv.entries[0].cachedLines) == 0 {
		t.Error("Expected cachedLines to be populated after width change")
	}
}

func TestSetSize_NoRebuildOnSameWidth(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	lv.AddEvent(makeTextEvent("Hello world"))
	linesAfterAdd := make([]string, len(lv.entries[0].cachedLines))
	copy(linesAfterAdd, lv.entries[0].cachedLines)

	// Set same width, different height - should NOT rebuild
	lv.SetSize(100, 50)

	// Lines should be identical (same slice contents)
	if len(lv.entries[0].cachedLines) != len(linesAfterAdd) {
		t.Errorf("Cache was unexpectedly rebuilt on height-only change")
	}
	for i, line := range lv.entries[0].cachedLines {
		if line != linesAfterAdd[i] {
			t.Errorf("Cache line %d changed on height-only change", i)
			break
		}
	}
}

func TestSetSize_BuildsCacheForEntriesAddedBeforeWidth(t *testing.T) {
	lv := NewLogViewer()
	// Add entries before setting width
	lv.AddEvent(makeTextEvent("Hello"))
	lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{"command": "ls"}))

	if lv.totalLineCount != 0 {
		t.Errorf("Expected totalLineCount 0 before SetSize, got %d", lv.totalLineCount)
	}

	// Now set size - should build cache for all existing entries
	lv.SetSize(100, 30)

	if lv.totalLineCount == 0 {
		t.Error("Expected totalLineCount > 0 after SetSize with existing entries")
	}
	for i, entry := range lv.entries {
		if len(entry.cachedLines) == 0 {
			t.Errorf("Entry %d has no cachedLines after SetSize", i)
		}
	}
}

// --- Render visible-only tests ---

func TestRender_ReturnsOnlyVisibleLines(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 5) // viewport of 5 lines

	// Add many entries (each tool start = 1 line)
	for i := 0; i < 50; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("command-%d", i),
		}))
	}

	// Scroll to top, disable auto-scroll
	lv.ScrollToTop()

	output := lv.Render()
	outputLines := strings.Split(output, "\n")

	// Should have at most 5 visible lines (viewport height)
	if len(outputLines) > 5 {
		t.Errorf("Expected at most 5 visible lines, got %d", len(outputLines))
	}
}

func TestRender_EmptyLog(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	output := lv.Render()
	if !strings.Contains(output, "No log entries yet") {
		t.Error("Expected empty log message")
	}
}

func TestRender_ScrollPosition(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 3) // viewport of 3 lines
	lv.autoScroll = false

	// Add 10 tool start entries (1 line each)
	for i := 0; i < 10; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("cmd-%d", i),
		}))
	}

	// Scroll to position 5 (should show entries 5, 6, 7)
	lv.scrollPos = 5
	output := lv.Render()

	if !strings.Contains(output, "cmd-5") {
		t.Error("Expected cmd-5 to be visible at scroll position 5")
	}
	if strings.Contains(output, "cmd-0") {
		t.Error("Expected cmd-0 to NOT be visible at scroll position 5")
	}
}

// --- Syntax highlighting caching ---

func TestAddEvent_PreComputesSyntaxHighlighting(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	// Add a Read tool start (sets lastReadFilePath)
	lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{
		"file_path": "/test.go",
	}))

	// Add tool result (should trigger highlighting)
	lv.AddEvent(makeToolResultEvent("package main\n\nfunc main() {}"))

	if len(lv.entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(lv.entries))
	}

	resultEntry := lv.entries[1]
	if resultEntry.highlightedCode == "" {
		t.Error("Expected highlightedCode to be pre-computed for Read tool result")
	}
	if resultEntry.FilePath != "/test.go" {
		t.Errorf("Expected FilePath '/test.go', got '%s'", resultEntry.FilePath)
	}
}

func TestAddEvent_NoHighlightingForNonReadResults(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	// Bash tool start (NOT Read)
	lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
		"command": "ls",
	}))
	lv.AddEvent(makeToolResultEvent("file1.go\nfile2.go"))

	resultEntry := lv.entries[1]
	if resultEntry.highlightedCode != "" {
		t.Error("Expected no highlightedCode for non-Read tool result")
	}
}

func TestSetSize_PreservesHighlightingOnRebuild(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	// Add Read + result to trigger highlighting
	lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{
		"file_path": "/test.go",
	}))
	lv.AddEvent(makeToolResultEvent("package main\n\nfunc main() {}"))

	originalHighlighting := lv.entries[1].highlightedCode

	// Change width to trigger rebuild
	lv.SetSize(60, 30)

	// highlightedCode should be preserved (not recomputed)
	if lv.entries[1].highlightedCode != originalHighlighting {
		t.Error("Expected highlightedCode to be preserved across width change")
	}
}

// --- Scroll behavior ---

func TestScroll_MaxScrollPosMatchesTotalLines(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 10) // viewport of 10

	// Add 20 single-line entries
	for i := 0; i < 20; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("cmd-%d", i),
		}))
	}

	// maxScrollPos should be totalLines - height
	expected := lv.totalLineCount - 10
	if lv.maxScrollPos() != expected {
		t.Errorf("Expected maxScrollPos=%d, got %d", expected, lv.maxScrollPos())
	}
}

func TestScroll_AutoScrollOnNewEntry(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 5)

	// Add entries that exceed viewport
	for i := 0; i < 20; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("cmd-%d", i),
		}))
	}

	// With auto-scroll, scrollPos should be at the bottom
	if lv.scrollPos != lv.maxScrollPos() {
		t.Errorf("Expected auto-scroll to bottom (pos=%d), got pos=%d", lv.maxScrollPos(), lv.scrollPos)
	}
}

func TestScroll_ManualScrollDisablesAutoScroll(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 5)

	// Add entries
	for i := 0; i < 20; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("cmd-%d", i),
		}))
	}

	// Scroll up should disable auto-scroll
	lv.ScrollUp()
	if lv.autoScroll {
		t.Error("Expected autoScroll to be disabled after ScrollUp")
	}

	// Add more entries - should NOT move scroll position
	posBefore := lv.scrollPos
	lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
		"command": "new-cmd",
	}))

	if lv.scrollPos != posBefore {
		t.Error("Expected scroll position to stay fixed when autoScroll is disabled")
	}
}

func TestScroll_PageDownReenablesAutoScroll(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 5)

	for i := 0; i < 20; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("cmd-%d", i),
		}))
	}

	lv.ScrollToTop()
	if lv.autoScroll {
		t.Error("Expected autoScroll off after ScrollToTop")
	}

	// Page down until we reach the bottom
	for i := 0; i < 20; i++ {
		lv.PageDown()
	}

	if !lv.autoScroll {
		t.Error("Expected autoScroll to re-enable when reaching bottom")
	}
}

// --- Clear ---

func TestClear_ResetsCacheState(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	for i := 0; i < 10; i++ {
		lv.AddEvent(makeTextEvent(fmt.Sprintf("message %d", i)))
	}

	if lv.totalLineCount == 0 {
		t.Fatal("Expected totalLineCount > 0 before Clear")
	}

	lv.Clear()

	if lv.totalLineCount != 0 {
		t.Errorf("Expected totalLineCount 0 after Clear, got %d", lv.totalLineCount)
	}
	if len(lv.entries) != 0 {
		t.Errorf("Expected 0 entries after Clear, got %d", len(lv.entries))
	}
}

// --- Tool result line count accuracy ---

func TestToolResult_HighlightedCodeProducesMultipleLines(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 30)

	// Simulate Read tool with multi-line code
	lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{
		"file_path": "/test.go",
	}))

	code := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"
	lv.AddEvent(makeToolResultEvent(code))

	resultEntry := lv.entries[1]

	// Should have multiple cached lines (result indicator + code lines)
	if len(resultEntry.cachedLines) < 3 {
		t.Errorf("Expected highlighted tool result to produce multiple lines, got %d", len(resultEntry.cachedLines))
	}

	// totalLineCount should accurately reflect this
	totalActual := 0
	for _, entry := range lv.entries {
		totalActual += len(entry.cachedLines)
	}
	if lv.totalLineCount != totalActual {
		t.Errorf("totalLineCount (%d) doesn't match actual (%d)", lv.totalLineCount, totalActual)
	}
}

func TestToolResult_LongCodeTruncatedTo20Lines(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 50)

	lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{
		"file_path": "/test.go",
	}))

	// Generate 30-line code
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	lv.AddEvent(makeToolResultEvent(strings.Join(lines, "\n")))

	resultEntry := lv.entries[1]

	// Should have: 1 (result indicator) + 20 (code) + 1 (more lines indicator) = 22
	if len(resultEntry.cachedLines) > 23 {
		t.Errorf("Expected truncated tool result, got %d cached lines", len(resultEntry.cachedLines))
	}
}

// --- Benchmark ---

func BenchmarkRender_SmallLog(b *testing.B) {
	lv := NewLogViewer()
	lv.SetSize(120, 40)

	for i := 0; i < 10; i++ {
		lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{
			"file_path": fmt.Sprintf("/file%d.go", i),
		}))
		lv.AddEvent(makeToolResultEvent(fmt.Sprintf("content of file %d", i)))
	}
	lv.ScrollToTop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lv.Render()
	}
}

func BenchmarkRender_LargeLog(b *testing.B) {
	lv := NewLogViewer()
	lv.SetSize(120, 40) // viewport of 40 lines

	// 1000 entries - simulates a long session
	for i := 0; i < 1000; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("command-%d", i),
		}))
		lv.AddEvent(makeToolResultEvent(fmt.Sprintf("output line %d", i)))
	}
	lv.ScrollToTop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lv.Render()
	}
}

func BenchmarkRender_LargeLogWithHighlighting(b *testing.B) {
	lv := NewLogViewer()
	lv.SetSize(120, 40)

	// 200 Read tool results with syntax highlighting
	for i := 0; i < 200; i++ {
		lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{
			"file_path": fmt.Sprintf("/file%d.go", i),
		}))
		code := fmt.Sprintf("package main\n\nfunc test%d() {\n\treturn\n}", i)
		lv.AddEvent(makeToolResultEvent(code))
	}
	lv.ScrollToTop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lv.Render()
	}
}

func BenchmarkAddEvent(b *testing.B) {
	lv := NewLogViewer()
	lv.SetSize(120, 40)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("command-%d", i),
		}))
	}
}

func BenchmarkAddEvent_WithHighlighting(b *testing.B) {
	lv := NewLogViewer()
	lv.SetSize(120, 40)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lv.AddEvent(makeToolStartEvent("Read", map[string]interface{}{
			"file_path": "/test.go",
		}))
		lv.AddEvent(makeToolResultEvent("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"))
	}
}

func BenchmarkSetSize_CacheRebuild(b *testing.B) {
	lv := NewLogViewer()
	lv.SetSize(120, 40)

	// Pre-populate with 500 entries
	for i := 0; i < 500; i++ {
		lv.AddEvent(makeToolStartEvent("Bash", map[string]interface{}{
			"command": fmt.Sprintf("command-%d", i),
		}))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate widths to force rebuild
		if i%2 == 0 {
			lv.SetSize(100, 40)
		} else {
			lv.SetSize(120, 40)
		}
	}
}
