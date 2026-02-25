package loop

import (
	"testing"
)

func TestParseLineCodex_threadStarted(t *testing.T) {
	line := `{"type":"thread.started","thread_id":"0199a213-81c0-7800-8aa1-bbab2a035a53"}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventIterationStart {
		t.Errorf("expected EventIterationStart, got %v", ev.Type)
	}
}

func TestParseLineCodex_turnStarted(t *testing.T) {
	line := `{"type":"turn.started"}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventIterationStart {
		t.Errorf("expected EventIterationStart, got %v", ev.Type)
	}
}

func TestParseLineCodex_commandExecutionStarted(t *testing.T) {
	line := `{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"bash -lc ls","aggregated_output":"","exit_code":null,"status":"in_progress"}}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "bash -lc ls" {
		t.Errorf("expected Tool bash -lc ls, got %q", ev.Tool)
	}
}

func TestParseLineCodex_commandExecutionCompleted(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"bash -lc ls","aggregated_output":"docs\nsrc\n","exit_code":0,"status":"completed"}}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", ev.Type)
	}
	if ev.Text != "docs\nsrc\n" {
		t.Errorf("expected Text docs\\nsrc\\n, got %q", ev.Text)
	}
}

func TestParseLineCodex_agentMessageWithComplete(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Done. <chief-complete/>"}}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventComplete {
		t.Errorf("expected EventComplete, got %v", ev.Type)
	}
	if ev.Text != "Done. <chief-complete/>" {
		t.Errorf("unexpected Text: %q", ev.Text)
	}
}

func TestParseLineCodex_agentMessageWithRalphStatus(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Working on <ralph-status>US-056</ralph-status> now."}}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventStoryStarted {
		t.Errorf("expected EventStoryStarted, got %v", ev.Type)
	}
	if ev.StoryID != "US-056" {
		t.Errorf("expected StoryID US-056, got %q", ev.StoryID)
	}
}

func TestParseLineCodex_agentMessagePlain(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Done. I updated the docs."}}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventAssistantText {
		t.Errorf("expected EventAssistantText, got %v", ev.Type)
	}
	if ev.Text != "Done. I updated the docs." {
		t.Errorf("unexpected Text: %q", ev.Text)
	}
}

func TestParseLineCodex_turnFailed(t *testing.T) {
	line := `{"type":"turn.failed","error":{"message":"model response stream ended unexpectedly"}}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventError {
		t.Errorf("expected EventError, got %v", ev.Type)
	}
	if ev.Err == nil {
		t.Fatal("expected Err set")
	}
	if ev.Err.Error() != "model response stream ended unexpectedly" {
		t.Errorf("unexpected Err: %v", ev.Err)
	}
}

func TestParseLineCodex_error(t *testing.T) {
	line := `{"type":"error","message":"stream error: broken pipe"}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventError {
		t.Errorf("expected EventError, got %v", ev.Type)
	}
}

func TestParseLineCodex_turnCompleted_ignored(t *testing.T) {
	line := `{"type":"turn.completed","usage":{"input_tokens":24763,"cached_input_tokens":24448,"output_tokens":122}}`
	ev := ParseLineCodex(line)
	if ev != nil {
		t.Errorf("expected nil (ignore turn.completed), got %v", ev)
	}
}

func TestParseLineCodex_mcpToolCallStarted(t *testing.T) {
	line := `{"type":"item.started","item":{"id":"item_5","type":"mcp_tool_call","server":"docs","tool":"search","arguments":{"q":"exec --json"},"result":null,"error":null,"status":"in_progress"}}`
	ev := ParseLineCodex(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "docs/search" {
		t.Errorf("expected Tool docs/search, got %q", ev.Tool)
	}
}

func TestParseLineCodex_emptyOrInvalid_returnsNil(t *testing.T) {
	tests := []string{"", "   ", "not json", "{}", `{"type":"unknown"}`}
	for _, line := range tests {
		ev := ParseLineCodex(line)
		if ev != nil {
			t.Errorf("ParseLineCodex(%q) expected nil, got %v", line, ev)
		}
	}
}
