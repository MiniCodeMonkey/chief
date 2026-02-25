package loop

import (
	"encoding/json"
	"errors"
	"strings"
)

type openCodeEvent struct {
	Type    string        `json:"type"`
	Message string        `json:"message,omitempty"`
	Error   *openCodeErr  `json:"error,omitempty"`
	Part    *openCodePart `json:"part,omitempty"`
}

type openCodeErr struct {
	Message string `json:"message"`
}

type openCodePart struct {
	Type  string             `json:"type"`
	Text  string             `json:"text,omitempty"`
	Tool  string             `json:"tool,omitempty"`
	State *openCodeToolState `json:"state,omitempty"`
}

type openCodeToolState struct {
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

// ParseLineOpenCode parses a single line of OpenCode run --format json output.
// If the line cannot be parsed or is not relevant, it returns nil.
func ParseLineOpenCode(line string) *Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var ev openCodeEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return nil
	}

	switch ev.Type {
	case "text":
		if ev.Part == nil || ev.Part.Text == "" {
			return nil
		}
		text := ev.Part.Text
		if strings.Contains(text, "<chief-complete/>") {
			return &Event{Type: EventComplete, Text: text}
		}
		if storyID := extractStoryID(text, "<ralph-status>", "</ralph-status>"); storyID != "" {
			return &Event{
				Type:    EventStoryStarted,
				Text:    text,
				StoryID: storyID,
			}
		}
		return &Event{Type: EventAssistantText, Text: text}

	case "tool_use":
		if ev.Part == nil {
			return nil
		}
		tool := strings.TrimSpace(ev.Part.Tool)
		if tool == "" {
			tool = "tool"
		}
		if ev.Part.State != nil && strings.TrimSpace(ev.Part.State.Output) != "" {
			return &Event{
				Type: EventToolResult,
				Tool: tool,
				Text: ev.Part.State.Output,
			}
		}
		return &Event{
			Type: EventToolStart,
			Tool: tool,
		}

	case "error":
		msg := strings.TrimSpace(ev.Message)
		if msg == "" && ev.Error != nil {
			msg = strings.TrimSpace(ev.Error.Message)
		}
		if msg == "" {
			msg = "unknown error"
		}
		return &Event{Type: EventError, Err: errors.New(msg)}

	default:
		return nil
	}
}
