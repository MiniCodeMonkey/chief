package loop

import (
	"strings"
	"testing"
)

func TestParseLineCursor_systemInit(t *testing.T) {
	line := `{"type":"system","subtype":"init","apiKeySource":"login","cwd":"/Users/user/project","session_id":"c6b62c6f-7ead-4fd6-9922-e952131177ff","model":"Claude 4 Sonnet","permissionMode":"default"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventIterationStart {
		t.Errorf("expected EventIterationStart, got %v", ev.Type)
	}
}

func TestParseLineCursor_assistantText(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll read the README.md file"}]},"session_id":"c6b62c6f-7ead-4fd6-9922-e952131177ff"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventAssistantText {
		t.Errorf("expected EventAssistantText, got %v", ev.Type)
	}
	if ev.Text != "I'll read the README.md file" {
		t.Errorf("expected Text, got %q", ev.Text)
	}
}

func TestParseLineCursor_chiefComplete(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done. <chief-complete/>"}]},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventComplete {
		t.Errorf("expected EventComplete, got %v", ev.Type)
	}
}

func TestParseLineCursor_ralphStatus(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Working on story. <ralph-status>story-1</ralph-status>"}]},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventStoryStarted {
		t.Errorf("expected EventStoryStarted, got %v", ev.Type)
	}
	if ev.StoryID != "story-1" {
		t.Errorf("expected StoryID story-1, got %q", ev.StoryID)
	}
}

func TestParseLineCursor_toolCallStartedRead(t *testing.T) {
	line := `{"type":"tool_call","subtype":"started","call_id":"toolu_abc","tool_call":{"readToolCall":{"args":{"path":"README.md"}}},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "Read" {
		t.Errorf("expected Tool Read, got %q", ev.Tool)
	}
	if ev.ToolInput == nil || ev.ToolInput["file_path"] != "README.md" {
		t.Errorf("expected ToolInput file_path=README.md, got %v", ev.ToolInput)
	}
}

func TestParseLineCursor_toolCallStartedWrite(t *testing.T) {
	line := `{"type":"tool_call","subtype":"started","call_id":"toolu_xyz","tool_call":{"writeToolCall":{"args":{"path":"summary.txt","fileText":"content"}}},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "Write" {
		t.Errorf("expected Tool Write, got %q", ev.Tool)
	}
	if ev.ToolInput == nil || ev.ToolInput["file_path"] != "summary.txt" {
		t.Errorf("expected ToolInput file_path=summary.txt, got %v", ev.ToolInput)
	}
}

func TestParseLineCursor_toolCallCompletedRead(t *testing.T) {
	line := `{"type":"tool_call","subtype":"completed","call_id":"toolu_abc","tool_call":{"readToolCall":{"args":{"path":"README.md"},"result":{"success":{"content":"# Project\n\nContent here.","isEmpty":false,"exceededLimit":false,"totalLines":10,"totalChars":100}}}},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev.Type)
	}
	if ev.Tool != "Read" {
		t.Errorf("expected Tool Read, got %q", ev.Tool)
	}
	if ev.Text != "# Project\n\nContent here." {
		t.Errorf("expected Text content, got %q", ev.Text)
	}
}

func TestParseLineCursor_toolCallCompletedWrite(t *testing.T) {
	line := `{"type":"tool_call","subtype":"completed","call_id":"toolu_xyz","tool_call":{"writeToolCall":{"args":{"path":"summary.txt"},"result":{"success":{"path":"/Users/user/project/summary.txt","linesCreated":19,"fileSize":942}}}},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev.Type)
	}
	if ev.Tool != "Write" {
		t.Errorf("expected Tool Write, got %q", ev.Tool)
	}
	if ev.Text != "/Users/user/project/summary.txt" {
		t.Errorf("expected Text path, got %q", ev.Text)
	}
}

func TestParseLineCursor_toolCallFunctionWithResult(t *testing.T) {
	line := `{"type":"tool_call","subtype":"completed","call_id":"toolu_fn","tool_call":{"function":{"name":"run_terminal_cmd","arguments":"{}","result":{"success":{"output":"hello world"}}}},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev.Type)
	}
	if ev.Text != "hello world" {
		t.Errorf("expected Text from result.success.output, got %q", ev.Text)
	}
}

func TestParseLineCursor_toolCallFunctionNoResult(t *testing.T) {
	line := `{"type":"tool_call","subtype":"completed","call_id":"toolu_fn2","tool_call":{"function":{"name":"some_tool","arguments":"{}"}},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev.Type)
	}
	if ev.Text != "(executed)" {
		t.Errorf("expected Text (executed) when no result, got %q", ev.Text)
	}
}

func TestParseLineCursor_userAndResult_ignored(t *testing.T) {
	userLine := `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"prompt"}]},"session_id":"x"}`
	if ev := ParseLineCursor(userLine); ev != nil {
		t.Errorf("user event expected nil, got %v", ev)
	}
	resultLine := `{"type":"result","subtype":"success","duration_ms":1234,"result":"done","session_id":"x"}`
	if ev := ParseLineCursor(resultLine); ev != nil {
		t.Errorf("result event expected nil, got %v", ev)
	}
}

func TestParseLineCursor_toolCallEditStartedAndCompleted(t *testing.T) {
	started := `{"type":"tool_call","subtype":"started","call_id":"toolu_edit","tool_call":{"editToolCall":{"args":{"path":"index.html"}}},"session_id":"x"}`
	ev := ParseLineCursor(started)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "Edit" {
		t.Errorf("expected Tool Edit, got %q", ev.Tool)
	}
	if ev.ToolInput == nil || ev.ToolInput["file_path"] != "index.html" {
		t.Errorf("expected ToolInput file_path=index.html, got %v", ev.ToolInput)
	}

	completed := `{"type":"tool_call","subtype":"completed","call_id":"toolu_edit","tool_call":{"editToolCall":{"args":{"path":"index.html"},"result":{"success":{}}}},"session_id":"x"}`
	ev2 := ParseLineCursor(completed)
	if ev2 == nil {
		t.Fatal("expected event, got nil")
	}
	if ev2.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev2.Type)
	}
	if ev2.Tool != "Edit" {
		t.Errorf("expected Tool Edit, got %q", ev2.Tool)
	}
	if ev2.Text != "(edited)" {
		t.Errorf("expected Text (edited), got %q", ev2.Text)
	}
}

func TestParseLineCursor_toolCallShellStartedAndCompleted(t *testing.T) {
	started := `{"type":"tool_call","subtype":"started","call_id":"toolu_shell","tool_call":{"shellToolCall":{"args":{"command":"echo hello"}}},"session_id":"x"}`
	ev := ParseLineCursor(started)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "Bash" {
		t.Errorf("expected Tool Bash, got %q", ev.Tool)
	}
	if ev.ToolInput == nil || ev.ToolInput["command"] != "echo hello" {
		t.Errorf("expected ToolInput command=echo hello, got %v", ev.ToolInput)
	}

	completed := `{"type":"tool_call","subtype":"completed","call_id":"toolu_shell","tool_call":{"shellToolCall":{"args":{"command":"echo hello"},"result":{"success":{"exitCode":0,"output":"hello\n"}}}},"session_id":"x"}`
	ev2 := ParseLineCursor(completed)
	if ev2 == nil {
		t.Fatal("expected event, got nil")
	}
	if ev2.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev2.Type)
	}
	if ev2.Tool != "Bash" {
		t.Errorf("expected Tool Bash, got %q", ev2.Tool)
	}
	if ev2.Text != "hello" {
		t.Errorf("expected Text hello, got %q", ev2.Text)
	}
}

func TestParseLineCursor_toolCallWebSearchStartedAndCompleted(t *testing.T) {
	started := `{"type":"tool_call","subtype":"started","call_id":"toolu_01SFCs5FKmApRiaNPKy3BBqi","tool_call":{"webSearchToolCall":{"args":{"searchTerm":"place.horse API random horse image","toolCallId":"toolu_01SFCs5FKmApRiaNPKy3BBqi"}}},"session_id":"x"}`
	ev := ParseLineCursor(started)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "WebSearch" {
		t.Errorf("expected Tool WebSearch, got %q", ev.Tool)
	}
	if ev.ToolInput == nil || ev.ToolInput["query"] != "place.horse API random horse image" {
		t.Errorf("expected ToolInput query=place.horse API random horse image, got %v", ev.ToolInput)
	}

	completed := `{"type":"tool_call","subtype":"completed","call_id":"toolu_01SFCs5FKmApRiaNPKy3BBqi","tool_call":{"webSearchToolCall":{"args":{"searchTerm":"place.horse API random horse image","toolCallId":"toolu_01SFCs5FKmApRiaNPKy3BBqi"},"result":{"success":{"references":[{"title":"Web search results for query: place.horse API random horse image","url":"","chunk":"Links:\n1. [API docs](https://theponyapi.com/docs)\n2. [Lorem Picsum](http://picsum.photos/)\n\nBased on your query about a place.horse API..."}]}}}},"session_id":"x"}`
	ev2 := ParseLineCursor(completed)
	if ev2 == nil {
		t.Fatal("expected event, got nil")
	}
	if ev2.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev2.Type)
	}
	if ev2.Tool != "WebSearch" {
		t.Errorf("expected Tool WebSearch, got %q", ev2.Tool)
	}
	// Single reference with chunk: summary is the chunk text
	if ev2.Text == "" {
		t.Errorf("expected non-empty Text (chunk), got %q", ev2.Text)
	}
	if !strings.Contains(ev2.Text, "Based on your query") {
		t.Errorf("expected Text to contain chunk content, got %q", ev2.Text)
	}
}

func TestParseLineCursor_toolCallWebSearchCompletedMultipleRefs(t *testing.T) {
	line := `{"type":"tool_call","subtype":"completed","call_id":"toolu_x","tool_call":{"webSearchToolCall":{"result":{"success":{"references":[{"title":"A","url":"","chunk":""},{"title":"B","url":"","chunk":""}]}}}},"session_id":"x"}`
	ev := ParseLineCursor(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev.Type)
	}
	if ev.Tool != "WebSearch" {
		t.Errorf("expected Tool WebSearch, got %q", ev.Tool)
	}
	if ev.Text != "2 reference(s)" {
		t.Errorf("expected Text 2 reference(s), got %q", ev.Text)
	}
}

func TestParseLineCursor_toolCallWebFetchStartedAndCompleted(t *testing.T) {
	started := `{"type":"tool_call","subtype":"started","call_id":"toolu_01SALput2Tb7iCNqx4jfy8v2","tool_call":{"webFetchToolCall":{"args":{"url":"https://github.com/treboryx/animalsAPI","toolCallId":"toolu_01SALput2Tb7iCNqx4jfy8v2"}}},"session_id":"x"}`
	ev := ParseLineCursor(started)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "WebFetch" {
		t.Errorf("expected Tool WebFetch, got %q", ev.Tool)
	}
	if ev.ToolInput == nil || ev.ToolInput["url"] != "https://github.com/treboryx/animalsAPI" {
		t.Errorf("expected ToolInput url=https://github.com/treboryx/animalsAPI, got %v", ev.ToolInput)
	}

	completed := `{"type":"tool_call","subtype":"completed","call_id":"toolu_01SALput2Tb7iCNqx4jfy8v2","tool_call":{"webFetchToolCall":{"args":{"url":"https://github.com/treboryx/animalsAPI","toolCallId":"toolu_01SALput2Tb7iCNqx4jfy8v2"},"result":{"success":{"url":"https://github.com/treboryx/animalsAPI","markdown":"# treboryx/animalsAPI\n\nAll-in-one API for random animal images."}}}},"session_id":"x"}`
	ev2 := ParseLineCursor(completed)
	if ev2 == nil {
		t.Fatal("expected event, got nil")
	}
	if ev2.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev2.Type)
	}
	if ev2.Tool != "WebFetch" {
		t.Errorf("expected Tool WebFetch, got %q", ev2.Tool)
	}
	if ev2.Text == "" {
		t.Errorf("expected non-empty Text (markdown), got %q", ev2.Text)
	}
	if !strings.Contains(ev2.Text, "treboryx/animalsAPI") {
		t.Errorf("expected Text to contain markdown content, got %q", ev2.Text)
	}
}

func TestParseLineCursor_emptyOrInvalid_returnsNil(t *testing.T) {
	tests := []string{"", "   ", "not json", "{}", `{"type":"unknown"}`}
	for _, line := range tests {
		ev := ParseLineCursor(line)
		if ev != nil {
			t.Errorf("ParseLineCursor(%q) expected nil, got %v", line, ev)
		}
	}
}
