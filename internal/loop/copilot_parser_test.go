package loop

import "testing"

func TestParseLineCopilot(t *testing.T) {
	tests := []struct {
		name string
		line string
		want EventType
		text string
		tool string
	}{
		{
			name: "turn start",
			line: `{"type":"assistant.turn_start","data":{"turnId":"0"}}`,
			want: EventIterationStart,
		},
		{
			name: "assistant text",
			line: `{"type":"assistant.message","data":{"content":"working on it"}}`,
			want: EventAssistantText,
			text: "working on it",
		},
		{
			name: "story done",
			line: `{"type":"assistant.message","data":{"content":"done <chief-done/>"}}`,
			want: EventStoryDone,
			text: "done <chief-done/>",
		},
		{
			name: "complete",
			line: `{"type":"assistant.message","data":{"content":"all done <chief-complete/>"}}`,
			want: EventComplete,
			text: "all done <chief-complete/>",
		},
		{
			name: "tool start",
			line: `{"type":"tool.execution_start","data":{"toolName":"bash","arguments":{"command":"go test ./..."}}}`,
			want: EventToolStart,
			tool: "bash",
		},
		{
			name: "tool complete",
			line: `{"type":"tool.execution_complete","data":{"toolCallId":"toolu_123","success":true,"result":{"content":"ok"}}}`,
			want: EventToolResult,
			text: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := ParseLineCopilot(tt.line)
			if event == nil {
				t.Fatal("ParseLineCopilot() returned nil")
			}
			if event.Type != tt.want || event.Text != tt.text || event.Tool != tt.tool {
				t.Errorf("ParseLineCopilot() = %#v, want type=%v text=%q tool=%q", event, tt.want, tt.text, tt.tool)
			}
			if tt.name == "tool start" && event.ToolInput["command"] != "go test ./..." {
				t.Errorf("ToolInput = %v, want command", event.ToolInput)
			}
		})
	}
}

func TestParseLineCopilot_IgnoresIrrelevantInput(t *testing.T) {
	for _, line := range []string{
		"",
		"not json",
		`{"type":"assistant.message_delta","data":{"deltaContent":"partial"}}`,
		`{"type":"assistant.message","data":{"content":"","toolRequests":[]}}`,
		`{"type":"result","exitCode":0}`,
	} {
		if event := ParseLineCopilot(line); event != nil {
			t.Errorf("ParseLineCopilot(%q) = %#v, want nil", line, event)
		}
	}
}
