package loop

import (
	"testing"
)

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventUnknown, "Unknown"},
		{EventIterationStart, "IterationStart"},
		{EventAssistantText, "AssistantText"},
		{EventToolStart, "ToolStart"},
		{EventToolResult, "ToolResult"},
		{EventStoryDone, "StoryDone"},
		{EventComplete, "Complete"},
		{EventMaxIterationsReached, "MaxIterationsReached"},
		{EventError, "Error"},
		{EventRetrying, "Retrying"},
		{EventWatchdogTimeout, "WatchdogTimeout"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.expected {
				t.Errorf("EventType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseLineEmpty(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"\t",
		"\n",
	}

	for _, line := range tests {
		if got := ParseLine(line); got != nil {
			t.Errorf("ParseLine(%q) = %v, want nil", line, got)
		}
	}
}

func TestParseLineInvalidJSON(t *testing.T) {
	tests := []string{
		"not json",
		"{invalid}",
		"[1, 2, 3]",
	}

	for _, line := range tests {
		if got := ParseLine(line); got != nil {
			t.Errorf("ParseLine(%q) = %v, want nil", line, got)
		}
	}
}

func TestParseLineSystemInit(t *testing.T) {
	line := `{"type":"system","subtype":"init","cwd":"/test","session_id":"abc123","tools":["Read","Write"]}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventIterationStart {
		t.Errorf("event.Type = %v, want EventIterationStart", event.Type)
	}
}

func TestParseLineSystemOtherSubtype(t *testing.T) {
	line := `{"type":"system","subtype":"other"}`

	event := ParseLine(line)
	if event != nil {
		t.Errorf("ParseLine returned %v, want nil for non-init system message", event)
	}
}

func TestParseLineAssistantText(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello, I will help you."}]}}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventAssistantText {
		t.Errorf("event.Type = %v, want EventAssistantText", event.Type)
	}
	if event.Text != "Hello, I will help you." {
		t.Errorf("event.Text = %q, want %q", event.Text, "Hello, I will help you.")
	}
}

func TestParseLineChiefDone(t *testing.T) {
	tests := []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"All criteria pass! <chief-done/>"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"<chief-done/>"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Done\n<chief-done/>\nGoodbye"}]}}`,
	}

	for _, line := range tests {
		event := ParseLine(line)
		if event == nil {
			t.Fatalf("ParseLine(%q) returned nil, want event", line)
		}
		if event.Type != EventStoryDone {
			t.Errorf("ParseLine(%q): event.Type = %v, want EventStoryDone", line, event.Type)
		}
	}
}

func TestParseLineToolUse(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_123","name":"Read","input":{"file_path":"/test/file.go"}}]}}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventToolStart {
		t.Errorf("event.Type = %v, want EventToolStart", event.Type)
	}
	if event.Tool != "Read" {
		t.Errorf("event.Tool = %q, want %q", event.Tool, "Read")
	}
	if event.ToolInput == nil {
		t.Fatal("event.ToolInput is nil")
	}
	if filePath, ok := event.ToolInput["file_path"].(string); !ok || filePath != "/test/file.go" {
		t.Errorf("event.ToolInput[\"file_path\"] = %v, want %q", event.ToolInput["file_path"], "/test/file.go")
	}
}

func TestParseLineToolResult(t *testing.T) {
	line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_123","content":"File contents here"}]}}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventToolResult {
		t.Errorf("event.Type = %v, want EventToolResult", event.Type)
	}
	if event.Text != "File contents here" {
		t.Errorf("event.Text = %q, want %q", event.Text, "File contents here")
	}
}

func TestParseLineResultMessage(t *testing.T) {
	line := `{"type":"result","subtype":"success","is_error":false,"result":"Done"}`

	event := ParseLine(line)
	// Result messages are not emitted as events
	if event != nil {
		t.Errorf("ParseLine returned %v, want nil for result message", event)
	}
}

func TestParseLineUnknownType(t *testing.T) {
	line := `{"type":"unknown_type"}`

	event := ParseLine(line)
	if event != nil {
		t.Errorf("ParseLine returned %v, want nil for unknown type", event)
	}
}

func TestParseLineEmptyContent(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[]}}`

	event := ParseLine(line)
	if event != nil {
		t.Errorf("ParseLine returned %v, want nil for empty content", event)
	}
}

func TestParseLineRealWorldSystemInit(t *testing.T) {
	line := `{"type":"system","subtype":"init","cwd":"/Users/codemonkey/projects/chief","session_id":"7cdf33f6-72ec-4c0e-94fb-e2b637e109da","tools":["Task","Bash","Read"],"model":"claude-opus-4-5-20251101","permissionMode":"default"}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventIterationStart {
		t.Errorf("event.Type = %v, want EventIterationStart", event.Type)
	}
}

func TestParseLineRealWorldToolUse(t *testing.T) {
	line := `{"type":"assistant","message":{"model":"claude-opus-4-5-20251101","id":"msg_01XPBzHqPaCuQMUFm77eHe4D","type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_01MiR5Ps9inHigemS2gxEz5R","name":"Read","input":{"file_path":"/Users/codemonkey/projects/chief/go.mod"}}]}}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventToolStart {
		t.Errorf("event.Type = %v, want EventToolStart", event.Type)
	}
	if event.Tool != "Read" {
		t.Errorf("event.Tool = %q, want %q", event.Tool, "Read")
	}
}

func TestParseLineMultipleContentBlocks(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"First"},{"type":"tool_use","name":"Read","input":{}}]}}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventAssistantText {
		t.Errorf("event.Type = %v, want EventAssistantText", event.Type)
	}
	if event.Text != "First" {
		t.Errorf("event.Text = %q, want %q", event.Text, "First")
	}
}

func TestParseLineToolUseFirst(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/test"}},{"type":"text","text":"Second"}]}}`

	event := ParseLine(line)
	if event == nil {
		t.Fatal("ParseLine returned nil, want event")
	}
	if event.Type != EventToolStart {
		t.Errorf("event.Type = %v, want EventToolStart", event.Type)
	}
	if event.Tool != "Write" {
		t.Errorf("event.Tool = %q, want %q", event.Tool, "Write")
	}
}
