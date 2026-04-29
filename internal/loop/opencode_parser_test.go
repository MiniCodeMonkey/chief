package loop

import (
	"testing"
)

func TestParseLineOpenCode_stepStart(t *testing.T) {
	line := `{"type":"step_start","timestamp":1767036059338,"sessionID":"ses_494719016ffe85dkDMj0FPRbHK","part":{"id":"prt_b6b8e7ec7001qAZUB7eTENxPpI","sessionID":"ses_494719016ffe85dkDMj0FPRbHK","messageID":"msg_b6b8e702b0012XuEC4bGe0XhKa","type":"step-start","snapshot":"71db24a798b347669c0ebadb2dfad238f991753d"}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventIterationStart {
		t.Errorf("expected EventIterationStart, got %v", ev.Type)
	}
}

func TestParseLineOpenCode_toolUseCompleted(t *testing.T) {
	line := `{"type":"tool_use","timestamp":1767036061199,"sessionID":"ses_494719016ffe85dkDMj0FPRbHK","part":{"id":"prt_b6b8e85bb001CzBoN2dDlEZJnP","sessionID":"ses_494719016ffe85dkDMj0FPRbHK","messageID":"msg_b6b8e702b0012XuEC4bGe0XhKa","type":"tool","callID":"r9bQWsNLvOrJGIOz","tool":"bash","state":{"status":"completed","input":{"command":"echo hello","description":"Print hello to stdout"},"output":"hello\n","title":"Print hello to stdout","metadata":{"output":"hello\n","exit":0,"description":"Print hello to stdout"},"time":{"start":1767036061123,"end":1767036061173}}}}`
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
	if ev.Text != "hello\n" {
		t.Errorf("expected Text hello\\n, got %q", ev.Text)
	}
}

func TestParseLineOpenCode_toolUseStarting(t *testing.T) {
	line := `{"type":"tool_use","timestamp":1767036061100,"sessionID":"ses_test","part":{"id":"prt_1","type":"tool","tool":"bash","callID":"abc123","state":{"status":"pending","input":{"command":"ls"}}}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventToolStart {
		t.Errorf("expected EventToolStart, got %v", ev.Type)
	}
	if ev.Tool != "bash" {
		t.Errorf("expected Tool bash, got %q", ev.Tool)
	}
}

func TestParseLineOpenCode_toolUseNoState(t *testing.T) {
	line := `{"type":"tool_use","timestamp":1767036061100,"sessionID":"ses_test","part":{"id":"prt_1","type":"tool","tool":"read"}}`
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

func TestParseLineOpenCode_text(t *testing.T) {
	line := `{"type":"text","timestamp":1767036064268,"sessionID":"ses_494719016ffe85dkDMj0FPRbHK","part":{"id":"prt_b6b8e8ff2002mxSx9LtvAlf8Ng","sessionID":"ses_494719016ffe85dkDMj0FPRbHK","messageID":"msg_b6b8e8627001yM4qKJCXdC7W1L","type":"text","text":"hello\n","time":{"start":1767036064265,"end":1767036064265}}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventAssistantText {
		t.Errorf("expected EventAssistantText, got %v", ev.Type)
	}
	if ev.Text != "hello\n" {
		t.Errorf("expected Text hello\\n, got %q", ev.Text)
	}
}

func TestParseLineOpenCode_stepFinishStop(t *testing.T) {
	line := `{"type":"step_finish","timestamp":1767036064273,"sessionID":"ses_494719016ffe85dkDMj0FPRbHK","part":{"id":"prt_b6b8e9209001ojZ4ECN1geZISm","sessionID":"ses_494719016ffe85dkDMj0FPRbHK","messageID":"msg_b6b8e8627001yM4qKJCXdC7W1L","type":"step-finish","reason":"stop","snapshot":"09dd05d11a4ac013136c1df10932efc0ad9116e8","cost":0.001,"tokens":{"input":671,"output":8,"reasoning":0,"cache":{"read":21415,"write":0}}}}`
	ev := ParseLineOpenCode(line)
	if ev != nil {
		t.Errorf("expected nil (step_finish does not signal PRD completion; <chief-done/> + buildPrompt drive completion), got %v", ev)
	}
}

func TestParseLineOpenCode_stepFinishToolCalls(t *testing.T) {
	line := `{"type":"step_finish","timestamp":1767036061205,"sessionID":"ses_494719016ffe85dkDMj0FPRbHK","part":{"id":"prt_b6b8e85fb001L4I3WHMqH6EQNI","sessionID":"ses_494719016ffe85dkDMj0FPRbHK","messageID":"msg_b6b8e702b0012XuEC4bGe0XhKa","type":"step-finish","reason":"tool-calls","snapshot":"ee3406d50c7d9048674bbb1a3e325d82513b74ed","cost":0,"tokens":{"input":21772,"output":110,"reasoning":0,"cache":{"read":0,"write":0}}}}`
	ev := ParseLineOpenCode(line)
	if ev != nil {
		t.Errorf("expected nil (ignore step_finish with reason=tool-calls), got %v", ev)
	}
}

func TestParseLineOpenCode_error(t *testing.T) {
	line := `{"type":"error","timestamp":1767036065000,"sessionID":"ses_494719016ffe85dkDMj0FPRbHK","error":{"name":"APIError","data":{"message":"Rate limit exceeded","statusCode":429,"isRetryable":true}}}`
	ev := ParseLineOpenCode(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventError {
		t.Errorf("expected EventError, got %v", ev.Type)
	}
	if ev.Err == nil {
		t.Fatal("expected Err set")
	}
	if ev.Err.Error() != "Rate limit exceeded" {
		t.Errorf("unexpected Err: %v", ev.Err)
	}
}

func TestParseLineOpenCode_emptyOrInvalid_returnsNil(t *testing.T) {
	tests := []string{"", "   ", "not json", "{}", `{"type":"unknown"}`}
	for _, line := range tests {
		ev := ParseLineOpenCode(line)
		if ev != nil {
			t.Errorf("ParseLineOpenCode(%q) expected nil, got %v", line, ev)
		}
	}
}
