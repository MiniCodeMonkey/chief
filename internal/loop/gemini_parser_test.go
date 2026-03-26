package loop

import (
	"testing"
)

func TestParseLineGemini_init(t *testing.T) {
	line := `{"type":"init","timestamp":"2025-01-01T00:00:00.000Z","session_id":"abc123","model":"gemini-2.5-pro"}`
	e := ParseLineGemini(line)
	if e == nil {
		t.Fatal("ParseLineGemini(init) returned nil")
	}
	if e.Type != EventIterationStart {
		t.Errorf("ParseLineGemini(init) Type = %v, want EventIterationStart", e.Type)
	}
}

func TestParseLineGemini_assistantMessage(t *testing.T) {
	line := `{"type":"message","timestamp":"2025-01-01T00:00:00.000Z","role":"assistant","content":"Hello, world!","delta":true}`
	e := ParseLineGemini(line)
	if e == nil {
		t.Fatal("ParseLineGemini(assistant message) returned nil")
	}
	if e.Type != EventAssistantText {
		t.Errorf("ParseLineGemini(assistant message) Type = %v, want EventAssistantText", e.Type)
	}
	if e.Text != "Hello, world!" {
		t.Errorf("ParseLineGemini(assistant message) Text = %q, want %q", e.Text, "Hello, world!")
	}
}

func TestParseLineGemini_userMessage(t *testing.T) {
	// User messages should be ignored.
	line := `{"type":"message","timestamp":"2025-01-01T00:00:00.000Z","role":"user","content":"do something","delta":false}`
	e := ParseLineGemini(line)
	if e != nil {
		t.Errorf("ParseLineGemini(user message) expected nil, got %v", e)
	}
}

func TestParseLineGemini_emptyContent(t *testing.T) {
	// Assistant message with empty content should be ignored.
	line := `{"type":"message","timestamp":"2025-01-01T00:00:00.000Z","role":"assistant","content":"","delta":true}`
	e := ParseLineGemini(line)
	if e != nil {
		t.Errorf("ParseLineGemini(empty content) expected nil, got %v", e)
	}
}

func TestParseLineGemini_storyDone(t *testing.T) {
	line := `{"type":"message","timestamp":"2025-01-01T00:00:00.000Z","role":"assistant","content":"All done! <chief-done/>","delta":true}`
	e := ParseLineGemini(line)
	if e == nil {
		t.Fatal("ParseLineGemini(chief-done) returned nil")
	}
	if e.Type != EventStoryDone {
		t.Errorf("ParseLineGemini(chief-done) Type = %v, want EventStoryDone", e.Type)
	}
	if e.Text != "All done! <chief-done/>" {
		t.Errorf("ParseLineGemini(chief-done) Text = %q, want %q", e.Text, "All done! <chief-done/>")
	}
}

func TestParseLineGemini_toolUse(t *testing.T) {
	line := `{"type":"tool_use","timestamp":"2025-01-01T00:00:00.000Z","tool_name":"read_file","tool_id":"call_1","parameters":{"path":"/foo/bar.go"}}`
	e := ParseLineGemini(line)
	if e == nil {
		t.Fatal("ParseLineGemini(tool_use) returned nil")
	}
	if e.Type != EventToolStart {
		t.Errorf("ParseLineGemini(tool_use) Type = %v, want EventToolStart", e.Type)
	}
	if e.Tool != "read_file" {
		t.Errorf("ParseLineGemini(tool_use) Tool = %q, want read_file", e.Tool)
	}
	if e.ToolInput == nil {
		t.Fatal("ParseLineGemini(tool_use) ToolInput is nil")
	}
	if v, ok := e.ToolInput["path"]; !ok || v != "/foo/bar.go" {
		t.Errorf("ParseLineGemini(tool_use) ToolInput[path] = %v, want /foo/bar.go", e.ToolInput["path"])
	}
}

func TestParseLineGemini_toolResult(t *testing.T) {
	line := `{"type":"tool_result","timestamp":"2025-01-01T00:00:00.000Z","tool_id":"call_1","status":"success","output":"file contents here"}`
	e := ParseLineGemini(line)
	if e == nil {
		t.Fatal("ParseLineGemini(tool_result) returned nil")
	}
	if e.Type != EventToolResult {
		t.Errorf("ParseLineGemini(tool_result) Type = %v, want EventToolResult", e.Type)
	}
	if e.Text != "file contents here" {
		t.Errorf("ParseLineGemini(tool_result) Text = %q, want %q", e.Text, "file contents here")
	}
}

func TestParseLineGemini_result(t *testing.T) {
	// Final result event should return nil (not actionable in the loop).
	line := `{"type":"result","timestamp":"2025-01-01T00:00:00.000Z","status":"success","stats":{}}`
	e := ParseLineGemini(line)
	if e != nil {
		t.Errorf("ParseLineGemini(result) expected nil, got %v", e)
	}
}

func TestParseLineGemini_error(t *testing.T) {
	// Error event should return nil (handled externally via stderr/exit code).
	line := `{"type":"error","timestamp":"2025-01-01T00:00:00.000Z","severity":"error","message":"something went wrong"}`
	e := ParseLineGemini(line)
	if e != nil {
		t.Errorf("ParseLineGemini(error) expected nil, got %v", e)
	}
}

func TestParseLineGemini_unknown(t *testing.T) {
	line := `{"type":"unknown_event","data":"foo"}`
	e := ParseLineGemini(line)
	if e != nil {
		t.Errorf("ParseLineGemini(unknown) expected nil, got %v", e)
	}
}

func TestParseLineGemini_emptyLine(t *testing.T) {
	e := ParseLineGemini("")
	if e != nil {
		t.Errorf("ParseLineGemini('') expected nil, got %v", e)
	}
	e = ParseLineGemini("   ")
	if e != nil {
		t.Errorf("ParseLineGemini('   ') expected nil, got %v", e)
	}
}

func TestParseLineGemini_invalidJSON(t *testing.T) {
	e := ParseLineGemini("not json at all")
	if e != nil {
		t.Errorf("ParseLineGemini(invalid json) expected nil, got %v", e)
	}
}

func TestParseLineGemini_toolUseNoParams(t *testing.T) {
	// Tool use event with no parameters should still parse.
	line := `{"type":"tool_use","timestamp":"2025-01-01T00:00:00.000Z","tool_name":"list_files","tool_id":"call_2","parameters":{}}`
	e := ParseLineGemini(line)
	if e == nil {
		t.Fatal("ParseLineGemini(tool_use no params) returned nil")
	}
	if e.Type != EventToolStart {
		t.Errorf("ParseLineGemini(tool_use no params) Type = %v, want EventToolStart", e.Type)
	}
	if e.Tool != "list_files" {
		t.Errorf("ParseLineGemini(tool_use no params) Tool = %q, want list_files", e.Tool)
	}
}
