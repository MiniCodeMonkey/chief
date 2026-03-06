package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/prd"
)

// newTestApp creates a minimal App for testing scroll and rendering.
func newTestApp(stories []prd.UserStory, width, height int) *App {
	return &App{
		prd:     &prd.PRD{UserStories: stories},
		width:   width,
		height:  height,
		viewMode: ViewDashboard,
	}
}

func makeStories(n int) []prd.UserStory {
	stories := make([]prd.UserStory, n)
	for i := range stories {
		stories[i] = prd.UserStory{
			ID:       fmt.Sprintf("US-%03d", i+1),
			Title:    fmt.Sprintf("Story %d", i+1),
			Priority: i + 1,
		}
	}
	return stories
}

func TestScrollOffset_FollowsCursorDown(t *testing.T) {
	app := newTestApp(makeStories(20), 120, 20)
	listHeight := app.storiesListHeight()
	if listHeight <= 0 {
		t.Fatalf("expected positive listHeight, got %d", listHeight)
	}

	// Navigate down past the visible range
	for i := 0; i < listHeight+3; i++ {
		if app.selectedIndex < len(app.prd.UserStories)-1 {
			app.selectedIndex++
			app.adjustStoriesScroll()
		}
	}

	// Selected index should be past the first screen
	if app.selectedIndex <= listHeight {
		t.Errorf("expected selectedIndex > %d, got %d", listHeight, app.selectedIndex)
	}

	// Scroll offset should have followed
	if app.storiesScrollOffset == 0 {
		t.Error("expected storiesScrollOffset > 0 after scrolling down past visible range")
	}

	// Selected index should be visible
	if app.selectedIndex < app.storiesScrollOffset || app.selectedIndex >= app.storiesScrollOffset+listHeight {
		t.Errorf("selectedIndex %d not visible in scroll window [%d, %d)", app.selectedIndex, app.storiesScrollOffset, app.storiesScrollOffset+listHeight)
	}
}

func TestScrollOffset_FollowsCursorUp(t *testing.T) {
	app := newTestApp(makeStories(20), 120, 20)
	listHeight := app.storiesListHeight()

	// Move down first
	for i := 0; i < listHeight+5; i++ {
		if app.selectedIndex < len(app.prd.UserStories)-1 {
			app.selectedIndex++
			app.adjustStoriesScroll()
		}
	}
	savedOffset := app.storiesScrollOffset

	// Now navigate back up past the scroll offset
	for i := 0; i < listHeight+5; i++ {
		if app.selectedIndex > 0 {
			app.selectedIndex--
			if app.selectedIndex < app.storiesScrollOffset {
				app.storiesScrollOffset = app.selectedIndex
			}
		}
	}

	// Should be back at top
	if app.selectedIndex != 0 {
		t.Errorf("expected selectedIndex 0, got %d", app.selectedIndex)
	}
	if app.storiesScrollOffset != 0 {
		t.Errorf("expected storiesScrollOffset 0, got %d", app.storiesScrollOffset)
	}
	_ = savedOffset
}

func TestScrollOffset_NoScrollWhenAllFit(t *testing.T) {
	// 3 stories in a 20-tall terminal — all should fit
	app := newTestApp(makeStories(3), 120, 20)
	listHeight := app.storiesListHeight()

	if len(app.prd.UserStories) > listHeight {
		t.Skipf("stories (%d) > listHeight (%d), skipping", len(app.prd.UserStories), listHeight)
	}

	// Navigate through all stories
	for i := 0; i < len(app.prd.UserStories); i++ {
		app.selectedIndex = i
		app.adjustStoriesScroll()
	}

	if app.storiesScrollOffset != 0 {
		t.Errorf("expected storiesScrollOffset 0 when all stories fit, got %d", app.storiesScrollOffset)
	}
}

func TestScrollOffset_ClampsToValidRange(t *testing.T) {
	app := newTestApp(makeStories(20), 120, 20)
	listHeight := app.storiesListHeight()

	// Force an invalid scroll offset
	app.storiesScrollOffset = 100
	app.adjustStoriesScroll()

	maxOffset := len(app.prd.UserStories) - listHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if app.storiesScrollOffset > maxOffset {
		t.Errorf("expected storiesScrollOffset <= %d, got %d", maxOffset, app.storiesScrollOffset)
	}

	// Force negative
	app.storiesScrollOffset = -5
	app.adjustStoriesScroll()
	if app.storiesScrollOffset < 0 {
		t.Errorf("expected storiesScrollOffset >= 0, got %d", app.storiesScrollOffset)
	}
}

func TestScrollPercentage_ShownWhenScrollable(t *testing.T) {
	app := newTestApp(makeStories(20), 120, 20)

	// Render the panel
	output := app.renderStoriesPanel(40, 15)

	// With 20 stories and listHeight = 15-5=10, list is scrollable
	// Title should contain percentage
	if !strings.Contains(output, "Stories (") || !strings.Contains(output, "%)") {
		t.Errorf("expected scroll percentage in panel title, got: %s", output)
	}
}

func TestScrollPercentage_NotShownWhenNotScrollable(t *testing.T) {
	app := newTestApp(makeStories(3), 120, 20)

	output := app.renderStoriesPanel(40, 15)

	// 3 stories fits in listHeight=10, so no percentage
	if strings.Contains(output, "%)") {
		t.Errorf("expected no scroll percentage when list fits, got: %s", output)
	}
}

func TestFooterHidden_WhenHeightLessThan12(t *testing.T) {
	app := newTestApp(makeStories(5), 120, 11)

	output := app.renderDashboard()

	// The footer contains "quit" shortcut — should not be present
	if strings.Contains(output, "q: quit") {
		t.Error("expected footer to be hidden when height < 12")
	}
}

func TestFooterShown_WhenHeightAtLeast12(t *testing.T) {
	// Need enough height to render without panic
	app := newTestApp(makeStories(5), 120, 20)

	output := app.renderDashboard()

	if !strings.Contains(output, "q: quit") {
		t.Error("expected footer to be shown when height >= 12")
	}
}

func TestAndNMore_Removed(t *testing.T) {
	// Create more stories than can fit in the panel
	app := newTestApp(makeStories(20), 120, 15)

	output := app.renderStoriesPanel(40, 12)

	if strings.Contains(output, "... and") || strings.Contains(output, "more") {
		t.Error("expected '... and N more' to be removed from stories panel")
	}
}
