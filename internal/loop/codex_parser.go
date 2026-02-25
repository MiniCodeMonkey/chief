package loop

import (
	"encoding/json"
	"errors"
	"strings"
)

// codexEvent represents the top-level structure of a Codex exec --json JSONL line.
type codexEvent struct {
	Type    string     `json:"type"`
	Item    *codexItem `json:"item,omitempty"`
	Message string     `json:"message,omitempty"` // top-level for type "error"
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// codexItem represents an item in item.started / item.completed / item.updated events.
type codexItem struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Text             string `json:"text,omitempty"`
	Command          string `json:"command,omitempty"`
	AggregatedOutput string `json:"aggregated_output,omitempty"`
	ExitCode         *int   `json:"exit_code,omitempty"`
	Status           string `json:"status,omitempty"`
	Server           string `json:"server,omitempty"`
	Tool             string `json:"tool,omitempty"`
}

// ParseLineCodex parses a single line of Codex exec --json JSONL output and returns an Event.
// If the line cannot be parsed or is not relevant, it returns nil.
func ParseLineCodex(line string) *Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var ev codexEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return nil
	}

	switch ev.Type {
	case "thread.started", "turn.started":
		return &Event{Type: EventIterationStart}

	case "turn.failed":
		msg := ""
		if ev.Error != nil {
			msg = ev.Error.Message
		}
		return &Event{Type: EventError, Err: errors.New(msg)}

	case "error":
		msg := ev.Message
		if msg == "" && ev.Error != nil {
			msg = ev.Error.Message
		}
		if msg == "" {
			msg = "unknown error"
		}
		return &Event{Type: EventError, Err: errors.New(msg)}

	case "item.started":
		if ev.Item == nil {
			return nil
		}
		switch ev.Item.Type {
		case "command_execution":
			return &Event{
				Type: EventToolStart,
				Tool: ev.Item.Command,
			}
		case "mcp_tool_call":
			toolName := ev.Item.Tool
			if ev.Item.Server != "" {
				toolName = ev.Item.Server + "/" + ev.Item.Tool
			}
			return &Event{
				Type: EventToolStart,
				Tool: toolName,
			}
		}
		return nil

	case "item.completed":
		if ev.Item == nil {
			return nil
		}
		switch ev.Item.Type {
		case "command_execution":
			return &Event{
				Type: EventToolResult,
				Text: ev.Item.AggregatedOutput,
			}
		case "mcp_tool_call":
			return &Event{
				Type: EventToolResult,
				Text: ev.Item.AggregatedOutput,
			}
		case "agent_message":
			text := ev.Item.Text
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
		case "file_change":
			return &Event{
				Type: EventToolResult,
				Tool: "file_change",
				Text: ev.Item.AggregatedOutput,
			}
		}
		return nil

	case "turn.completed":
		// Usage info only, no event
		return nil

	default:
		return nil
	}
}
