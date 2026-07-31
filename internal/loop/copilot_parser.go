package loop

import (
	"encoding/json"
	"strings"
)

type copilotEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type copilotAssistantMessage struct {
	Content string `json:"content"`
}

type copilotToolStart struct {
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
}

type copilotToolComplete struct {
	ToolName string `json:"toolName"`
	Success  bool   `json:"success"`
	Result   struct {
		Content         string `json:"content"`
		DetailedContent string `json:"detailedContent"`
	} `json:"result"`
}

// ParseLineCopilot parses a single line of GitHub Copilot CLI JSONL output.
func ParseLineCopilot(line string) *Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var event copilotEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil
	}

	switch event.Type {
	case "assistant.turn_start":
		return &Event{Type: EventIterationStart}
	case "assistant.message":
		var message copilotAssistantMessage
		if json.Unmarshal(event.Data, &message) != nil || message.Content == "" {
			return nil
		}
		if strings.Contains(message.Content, "<chief-complete/>") {
			return &Event{Type: EventComplete, Text: message.Content}
		}
		if strings.Contains(message.Content, "<chief-done/>") {
			return &Event{Type: EventStoryDone, Text: message.Content}
		}
		return &Event{Type: EventAssistantText, Text: message.Content}
	case "tool.execution_start":
		var tool copilotToolStart
		if json.Unmarshal(event.Data, &tool) != nil {
			return nil
		}
		return &Event{Type: EventToolStart, Tool: tool.ToolName, ToolInput: tool.Arguments}
	case "tool.execution_complete":
		var tool copilotToolComplete
		if json.Unmarshal(event.Data, &tool) != nil {
			return nil
		}
		result := tool.Result.Content
		if result == "" {
			result = tool.Result.DetailedContent
		}
		return &Event{Type: EventToolResult, Tool: tool.ToolName, Text: result}
	default:
		return nil
	}
}
