package loop

import "testing"

func TestParseLineOpenCode_text(t *testing.T) {
	line := `{"type":"text","part":{"type":"text","text":"Working on it."}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventAssistantText {
		t.Errorf("expected EventAssistantText, got %v", ev.Type)
	}
	if ev.Text != "Working on it." {
		t.Errorf("unexpected Text: %q", ev.Text)
	}
}

func TestParseLineOpenCode_textComplete(t *testing.T) {
	line := `{"type":"text","part":{"type":"text","text":"Done <chief-complete/>"}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventComplete {
		t.Errorf("expected EventComplete, got %v", ev.Type)
	}
}

func TestParseLineOpenCode_textStoryMarker(t *testing.T) {
	line := `{"type":"text","part":{"type":"text","text":"Now <ralph-status>US-007</ralph-status>"}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventStoryStarted {
		t.Errorf("expected EventStoryStarted, got %v", ev.Type)
	}
	if ev.StoryID != "US-007" {
		t.Errorf("expected StoryID US-007, got %q", ev.StoryID)
	}
}

func TestParseLineOpenCode_toolUseCompleted(t *testing.T) {
	line := `{"type":"tool_use","part":{"type":"tool","tool":"bash","state":{"status":"completed","output":"ok"}}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev.Type)
	}
	if ev.Tool != "bash" {
		t.Errorf("expected Tool bash, got %q", ev.Tool)
	}
	if ev.Text != "ok" {
		t.Errorf("expected Text ok, got %q", ev.Text)
	}
}

func TestParseLineOpenCode_toolUseNoOutput(t *testing.T) {
	line := `{"type":"tool_use","part":{"type":"tool","tool":"read","state":{"status":"running"}}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "read" {
		t.Errorf("expected Tool read, got %q", ev.Tool)
	}
}

func TestParseLineOpenCode_error(t *testing.T) {
	line := `{"type":"error","message":"boom"}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventError {
		t.Errorf("expected EventError, got %v", ev.Type)
	}
	if ev.Err == nil || ev.Err.Error() != "boom" {
		t.Errorf("unexpected Err: %v", ev.Err)
	}
}

func TestParseLineOpenCode_irrelevantOrInvalid(t *testing.T) {
	tests := []string{
		"",
		"  ",
		"not json",
		`{"type":"step_start","part":{"type":"step-start"}}`,
		`{"type":"step_finish","part":{"type":"step-finish","reason":"stop"}}`,
	}
	for _, line := range tests {
		if ev := ParseLineOpenCode(line); ev != nil {
			t.Errorf("ParseLineOpenCode(%q) expected nil, got %v", line, ev)
		}
	}
}
