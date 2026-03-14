package loop

import (
	"encoding/json"
	"fmt"
	"strings"
)

// cursorEvent represents the top-level structure of Cursor CLI stream-json NDJSON.
type cursorEvent struct {
	Type     string          `json:"type"`
	Subtype  string          `json:"subtype,omitempty"`
	Message  json.RawMessage `json:"message,omitempty"`
	ToolCall json.RawMessage `json:"tool_call,omitempty"`
}

// cursorAssistantMessage is the message body for type "assistant".
type cursorAssistantMessage struct {
	Role    string               `json:"role"`
	Content []cursorContentBlock `json:"content"`
}

// cursorContentBlock is a content block in an assistant message.
type cursorContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// cursorToolCall is the tool_call object.
type cursorToolCall struct {
	ReadToolCall      *cursorReadToolCall      `json:"readToolCall,omitempty"`
	WriteToolCall     *cursorWriteToolCall     `json:"writeToolCall,omitempty"`
	EditToolCall      *cursorEditToolCall      `json:"editToolCall,omitempty"`
	ShellToolCall     *cursorShellToolCall     `json:"shellToolCall,omitempty"`
	GrepToolCall      *cursorGrepToolCall      `json:"grepToolCall,omitempty"`
	GlobToolCall      *cursorGlobToolCall      `json:"globToolCall,omitempty"`
	LsToolCall        *cursorLsToolCall        `json:"lsToolCall,omitempty"`
	DeleteToolCall    *cursorDeleteToolCall    `json:"deleteToolCall,omitempty"`
	WebFetchToolCall  *cursorWebFetchToolCall  `json:"webFetchToolCall,omitempty"`
	WebSearchToolCall *cursorWebSearchToolCall `json:"webSearchToolCall,omitempty"`
	Function          *cursorFunctionCall      `json:"function,omitempty"`
}

// cursorWebFetchToolCall holds web fetch args and optional result.
type cursorWebFetchToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct {
			URL      string `json:"url"`
			Markdown string `json:"markdown"`
		} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorWebSearchToolCall holds web search args and optional result.
type cursorWebSearchToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct {
			References []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				Chunk string `json:"chunk"`
			} `json:"references,omitempty"`
		} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorEditToolCall holds edit/strreplace args and optional result.
type cursorEditToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct{} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorShellToolCall holds shell command args and optional result.
type cursorShellToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct {
			ExitCode *int   `json:"exitCode,omitempty"`
			Output   string `json:"output,omitempty"`
		} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorGrepToolCall holds grep args and optional result.
type cursorGrepToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct {
			WorkspaceResults map[string]struct {
				Content struct {
					TotalMatchedLines int `json:"totalMatchedLines"`
				} `json:"content"`
			} `json:"workspaceResults,omitempty"`
		} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorGlobToolCall holds glob args and optional result.
type cursorGlobToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct {
			TotalFiles int `json:"totalFiles"`
		} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorLsToolCall holds list-directory args and optional result.
type cursorLsToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct{} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorDeleteToolCall holds delete-file args and optional result.
type cursorDeleteToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct{} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorReadToolCall holds read file args and optional result.
type cursorReadToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct {
			Content string `json:"content"`
		} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorWriteToolCall holds write file args and optional result.
type cursorWriteToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result *struct {
		Success *struct {
			Path         string `json:"path"`
			LinesCreated int    `json:"linesCreated"`
			FileSize     int    `json:"fileSize"`
		} `json:"success,omitempty"`
	} `json:"result,omitempty"`
}

// cursorFunctionCall holds generic function name, arguments, and optional result.
type cursorFunctionCall struct {
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

// ParseLineCursor parses a single line of Cursor CLI stream-json NDJSON and returns an Event.
// If the line cannot be parsed or is not relevant, it returns nil.
func ParseLineCursor(line string) *Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var ev cursorEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return nil
	}

	switch ev.Type {
	case "system":
		if ev.Subtype == "init" {
			return &Event{Type: EventIterationStart}
		}
		return nil

	case "assistant":
		return parseCursorAssistantMessage(ev.Message)

	case "tool_call":
		return parseCursorToolCall(ev.Subtype, ev.ToolCall)

	case "user", "result":
		return nil

	default:
		return nil
	}
}

func parseCursorAssistantMessage(raw json.RawMessage) *Event {
	if raw == nil {
		return nil
	}
	var msg cursorAssistantMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	for _, block := range msg.Content {
		if block.Type != "text" {
			continue
		}
		text := block.Text
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
	}
	return nil
}

func parseCursorToolCall(subtype string, raw json.RawMessage) *Event {
	if raw == nil {
		return nil
	}
	var tc cursorToolCall
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil
	}
	toolName, toolInput := cursorToolCallNameAndInput(&tc)
	switch subtype {
	case "started":
		return &Event{Type: EventToolStart, Tool: toolName, ToolInput: toolInput}
	case "completed":
		text := cursorToolCallResultSummary(&tc)
		return &Event{Type: EventToolResult, Tool: toolName, Text: text}
	}
	return nil
}

// cursorToolCallNameAndInput returns display name (PascalCase for TUI icons) and optional ToolInput for the log.
func cursorToolCallNameAndInput(tc *cursorToolCall) (name string, input map[string]interface{}) {
	if tc.ReadToolCall != nil {
		input = make(map[string]interface{})
		if path, ok := tc.ReadToolCall.Args["path"].(string); ok {
			input["file_path"] = path
		}
		return "Read", input
	}
	if tc.WriteToolCall != nil {
		input = make(map[string]interface{})
		if path, ok := tc.WriteToolCall.Args["path"].(string); ok {
			input["file_path"] = path
		}
		return "Write", input
	}
	if tc.EditToolCall != nil {
		input = make(map[string]interface{})
		if path, ok := tc.EditToolCall.Args["path"].(string); ok {
			input["file_path"] = path
		}
		return "Edit", input
	}
	if tc.ShellToolCall != nil {
		input = make(map[string]interface{})
		if cmd, ok := tc.ShellToolCall.Args["command"].(string); ok {
			input["command"] = cmd
		}
		return "Bash", input
	}
	if tc.GrepToolCall != nil {
		input = make(map[string]interface{})
		if pattern, ok := tc.GrepToolCall.Args["pattern"].(string); ok {
			input["pattern"] = pattern
		}
		if path, ok := tc.GrepToolCall.Args["path"].(string); ok {
			input["path"] = path
		}
		return "Grep", input
	}
	if tc.GlobToolCall != nil {
		input = make(map[string]interface{})
		if pattern, ok := tc.GlobToolCall.Args["globPattern"].(string); ok {
			input["pattern"] = pattern
		}
		if dir, ok := tc.GlobToolCall.Args["targetDirectory"].(string); ok {
			input["path"] = dir
		}
		return "Glob", input
	}
	if tc.LsToolCall != nil {
		input = make(map[string]interface{})
		if path, ok := tc.LsToolCall.Args["path"].(string); ok {
			input["path"] = path
		}
		return "List", input
	}
	if tc.DeleteToolCall != nil {
		input = make(map[string]interface{})
		if path, ok := tc.DeleteToolCall.Args["path"].(string); ok {
			input["file_path"] = path
		}
		return "Delete", input
	}
	if tc.WebFetchToolCall != nil {
		input = make(map[string]interface{})
		if url, ok := tc.WebFetchToolCall.Args["url"].(string); ok {
			input["url"] = url
		}
		return "WebFetch", input
	}
	if tc.WebSearchToolCall != nil {
		input = make(map[string]interface{})
		if term, ok := tc.WebSearchToolCall.Args["searchTerm"].(string); ok {
			input["query"] = term
		}
		return "WebSearch", input
	}
	if tc.Function != nil && tc.Function.Name != "" {
		// TUI knows "Bash" for command execution; Cursor may use different names
		name = tc.Function.Name
		if name == "run_terminal_cmd" || name == "run_command" {
			name = "Bash"
		}
		if tc.Function.Arguments != "" {
			input = map[string]interface{}{"arguments": tc.Function.Arguments}
			// Try to extract command for Bash display
			var argsMap map[string]interface{}
			if json.Unmarshal([]byte(tc.Function.Arguments), &argsMap) == nil {
				if cmd, ok := argsMap["command"].(string); ok {
					input["command"] = cmd
				}
			}
		}
		return name, input
	}
	return "tool", nil
}

func cursorToolCallResultSummary(tc *cursorToolCall) string {
	if tc.ReadToolCall != nil && tc.ReadToolCall.Result != nil && tc.ReadToolCall.Result.Success != nil {
		return tc.ReadToolCall.Result.Success.Content
	}
	if tc.WriteToolCall != nil && tc.WriteToolCall.Result != nil && tc.WriteToolCall.Result.Success != nil {
		s := tc.WriteToolCall.Result.Success
		if s.Path != "" {
			return s.Path
		}
		return "(written)"
	}
	if tc.EditToolCall != nil && tc.EditToolCall.Result != nil && tc.EditToolCall.Result.Success != nil {
		return "(edited)"
	}
	if tc.ShellToolCall != nil && tc.ShellToolCall.Result != nil && tc.ShellToolCall.Result.Success != nil {
		s := tc.ShellToolCall.Result.Success
		if s.Output != "" {
			return strings.TrimSpace(s.Output)
		}
		if s.ExitCode != nil {
			return fmt.Sprintf("(exit %d)", *s.ExitCode)
		}
		return "(executed)"
	}
	if tc.GrepToolCall != nil && tc.GrepToolCall.Result != nil && tc.GrepToolCall.Result.Success != nil {
		for _, v := range tc.GrepToolCall.Result.Success.WorkspaceResults {
			return fmt.Sprintf("%d matches", v.Content.TotalMatchedLines)
		}
		return "(matches)"
	}
	if tc.GlobToolCall != nil && tc.GlobToolCall.Result != nil && tc.GlobToolCall.Result.Success != nil {
		n := tc.GlobToolCall.Result.Success.TotalFiles
		return fmt.Sprintf("%d files", n)
	}
	if tc.LsToolCall != nil && tc.LsToolCall.Result != nil && tc.LsToolCall.Result.Success != nil {
		return "(listed)"
	}
	if tc.DeleteToolCall != nil && tc.DeleteToolCall.Result != nil && tc.DeleteToolCall.Result.Success != nil {
		return "(deleted)"
	}
	if tc.WebFetchToolCall != nil && tc.WebFetchToolCall.Result != nil && tc.WebFetchToolCall.Result.Success != nil {
		s := tc.WebFetchToolCall.Result.Success
		if s.Markdown != "" {
			return strings.TrimSpace(s.Markdown)
		}
		return "(fetched)"
	}
	if tc.WebSearchToolCall != nil && tc.WebSearchToolCall.Result != nil && tc.WebSearchToolCall.Result.Success != nil {
		refs := tc.WebSearchToolCall.Result.Success.References
		if len(refs) == 0 {
			return "(no results)"
		}
		if len(refs) == 1 && refs[0].Chunk != "" {
			return strings.TrimSpace(refs[0].Chunk)
		}
		return fmt.Sprintf("%d reference(s)", len(refs))
	}
	if tc.Function != nil && len(tc.Function.Result) > 0 {
		s := extractFunctionResultText(tc.Function.Result)
		if s != "" {
			return s
		}
	}
	if tc.Function != nil {
		return "(executed)"
	}
	return ""
}

// extractFunctionResultText tries to get a short result string from Cursor function result JSON.
func extractFunctionResultText(raw json.RawMessage) string {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	for _, key := range []string{"output", "content", "result", "stdout", "text"} {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}
	if success, ok := m["success"].(map[string]interface{}); ok {
		for _, key := range []string{"output", "content", "result", "stdout"} {
			if v, ok := success[key].(string); ok && v != "" {
				return v
			}
		}
	}
	return ""
}
