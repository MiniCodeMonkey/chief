package loop

import (
	"encoding/json"
	"strings"
)

// geminiStreamEvent is the top-level structure for a Gemini stream-json line.
type geminiStreamEvent struct {
	Type string `json:"type"`
}

// geminiInitEvent represents the "init" event emitted at session start.
type geminiInitEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

// geminiMessageEvent represents a "message" event (user or assistant delta).
type geminiMessageEvent struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content string `json:"content"`
	Delta   bool   `json:"delta"`
}

// geminiToolUseEvent represents a "tool_use" event (tool call request).
type geminiToolUseEvent struct {
	Type       string                 `json:"type"`
	ToolName   string                 `json:"tool_name"`
	ToolID     string                 `json:"tool_id"`
	Parameters map[string]interface{} `json:"parameters"`
}

// geminiToolResultEvent represents a "tool_result" event.
type geminiToolResultEvent struct {
	Type   string `json:"type"`
	ToolID string `json:"tool_id"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

// geminiErrorEvent represents an "error" event.
type geminiErrorEvent struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ParseLineGemini parses a single line of Gemini's stream-json output and
// returns an Event. Returns nil for lines that are not relevant to Chief.
func ParseLineGemini(line string) *Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Peek at the type field first.
	var base geminiStreamEvent
	if err := json.Unmarshal([]byte(line), &base); err != nil {
		return nil
	}

	switch base.Type {
	case "init":
		// Session start maps to EventIterationStart.
		return &Event{Type: EventIterationStart}

	case "message":
		var msg geminiMessageEvent
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil
		}
		if msg.Role != "assistant" || msg.Content == "" {
			return nil
		}
		if strings.Contains(msg.Content, "<chief-done/>") {
			return &Event{Type: EventStoryDone, Text: msg.Content}
		}
		return &Event{Type: EventAssistantText, Text: msg.Content}

	case "tool_use":
		var tu geminiToolUseEvent
		if err := json.Unmarshal([]byte(line), &tu); err != nil {
			return nil
		}
		return &Event{
			Type:      EventToolStart,
			Tool:      tu.ToolName,
			ToolInput: tu.Parameters,
		}

	case "tool_result":
		var tr geminiToolResultEvent
		if err := json.Unmarshal([]byte(line), &tr); err != nil {
			return nil
		}
		return &Event{Type: EventToolResult, Text: tr.Output}

	case "result", "error":
		// Terminal / metadata events — not actionable inside the loop.
		return nil

	default:
		return nil
	}
}
