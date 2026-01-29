package loop

import (
	"encoding/json"
	"strings"
)

// EventType represents the type of event parsed from Claude's stream-json output.
type EventType int

const (
	// EventUnknown represents an unrecognized event type.
	EventUnknown EventType = iota
	// EventIterationStart is emitted at the start of a Claude iteration (system init).
	EventIterationStart
	// EventAssistantText is emitted when Claude outputs text.
	EventAssistantText
	// EventToolStart is emitted when Claude invokes a tool.
	EventToolStart
	// EventToolResult is emitted when a tool returns a result.
	EventToolResult
	// EventStoryStarted is emitted when Claude indicates a story is being worked on.
	EventStoryStarted
	// EventStoryCompleted is emitted when Claude completes a story.
	EventStoryCompleted
	// EventComplete is emitted when <chief-complete/> is detected.
	EventComplete
	// EventMaxIterationsReached is emitted when max iterations are reached.
	EventMaxIterationsReached
	// EventError is emitted when an error occurs.
	EventError
	// EventRetrying is emitted when retrying after a crash.
	EventRetrying
)

// String returns the string representation of an EventType.
func (e EventType) String() string {
	switch e {
	case EventIterationStart:
		return "IterationStart"
	case EventAssistantText:
		return "AssistantText"
	case EventToolStart:
		return "ToolStart"
	case EventToolResult:
		return "ToolResult"
	case EventStoryStarted:
		return "StoryStarted"
	case EventStoryCompleted:
		return "StoryCompleted"
	case EventComplete:
		return "Complete"
	case EventMaxIterationsReached:
		return "MaxIterationsReached"
	case EventError:
		return "Error"
	case EventRetrying:
		return "Retrying"
	default:
		return "Unknown"
	}
}

// Event represents a parsed event from Claude's stream-json output.
type Event struct {
	Type       EventType
	Iteration  int
	Text       string
	Tool       string
	ToolInput  map[string]interface{}
	StoryID    string
	Err        error
	RetryCount int // Current retry attempt (1-based)
	RetryMax   int // Maximum retries allowed
}

// streamMessage represents the top-level structure of a stream-json line.
type streamMessage struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

// assistantMessage represents the structure of an assistant message.
type assistantMessage struct {
	Content []contentBlock `json:"content"`
}

// contentBlock represents a block of content in an assistant message.
type contentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// userMessage represents a tool result message.
type userMessage struct {
	Content []toolResultBlock `json:"content"`
}

// toolResultBlock represents a tool result in a user message.
type toolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

// ParseLine parses a single line of stream-json output and returns an Event.
// If the line cannot be parsed or is not relevant, it returns nil.
func ParseLine(line string) *Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var msg streamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil
	}

	switch msg.Type {
	case "system":
		if msg.Subtype == "init" {
			return &Event{Type: EventIterationStart}
		}
		return nil

	case "assistant":
		return parseAssistantMessage(msg.Message)

	case "user":
		return parseUserMessage(msg.Message)

	case "result":
		// Result messages indicate the end of an iteration
		// We don't emit a specific event for this, but could in the future
		return nil

	default:
		return nil
	}
}

// parseAssistantMessage parses an assistant message and returns appropriate events.
func parseAssistantMessage(raw json.RawMessage) *Event {
	if raw == nil {
		return nil
	}

	var msg assistantMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}

	// Process content blocks - return the first meaningful event
	// In practice, we might want to return multiple events, but for simplicity
	// we return the first one found
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			text := block.Text
			// Check for <chief-complete/> tag
			if strings.Contains(text, "<chief-complete/>") {
				return &Event{
					Type: EventComplete,
					Text: text,
				}
			}
			// Check for story markers using ralph-status tags
			if storyID := extractStoryID(text, "<ralph-status>", "</ralph-status>"); storyID != "" {
				return &Event{
					Type:    EventStoryStarted,
					Text:    text,
					StoryID: storyID,
				}
			}
			return &Event{
				Type: EventAssistantText,
				Text: text,
			}

		case "tool_use":
			return &Event{
				Type:      EventToolStart,
				Tool:      block.Name,
				ToolInput: block.Input,
			}
		}
	}

	return nil
}

// parseUserMessage parses a user message (typically tool results).
func parseUserMessage(raw json.RawMessage) *Event {
	if raw == nil {
		return nil
	}

	var msg userMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}

	for _, block := range msg.Content {
		if block.Type == "tool_result" {
			return &Event{
				Type: EventToolResult,
				Text: block.Content,
			}
		}
	}

	return nil
}

// extractStoryID extracts a story ID from text between start and end tags.
func extractStoryID(text, startTag, endTag string) string {
	startIdx := strings.Index(text, startTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startTag)

	endIdx := strings.Index(text[startIdx:], endTag)
	if endIdx == -1 {
		return ""
	}

	return strings.TrimSpace(text[startIdx : startIdx+endIdx])
}
