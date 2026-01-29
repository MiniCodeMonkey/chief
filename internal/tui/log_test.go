package tui

import "testing"

func TestGetToolIcon(t *testing.T) {
	tests := []struct {
		toolName string
		expected string
	}{
		{"Read", "📖"},
		{"Edit", "✏️"},
		{"Write", "📝"},
		{"Bash", "🔨"},
		{"Glob", "🔍"},
		{"Grep", "🔎"},
		{"Task", "🤖"},
		{"WebFetch", "🌐"},
		{"WebSearch", "🌐"},
		{"Unknown", "⚙️"},
		{"", "⚙️"},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := getToolIcon(tt.toolName)
			if result != tt.expected {
				t.Errorf("getToolIcon(%q) = %q, want %q", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestGetToolArgument(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]interface{}
		expected string
	}{
		{
			name:     "Read with file_path",
			toolName: "Read",
			input:    map[string]interface{}{"file_path": "/path/to/file.go"},
			expected: "/path/to/file.go",
		},
		{
			name:     "Edit with file_path",
			toolName: "Edit",
			input:    map[string]interface{}{"file_path": "/test.go", "old_string": "foo"},
			expected: "/test.go",
		},
		{
			name:     "Bash with command",
			toolName: "Bash",
			input:    map[string]interface{}{"command": "go test ./..."},
			expected: "go test ./...",
		},
		{
			name:     "Bash with long command",
			toolName: "Bash",
			input:    map[string]interface{}{"command": "very long command that exceeds sixty characters and should be truncated"},
			expected: "very long command that exceeds sixty characters and shoul...",
		},
		{
			name:     "Glob with pattern",
			toolName: "Glob",
			input:    map[string]interface{}{"pattern": "**/*.go"},
			expected: "**/*.go",
		},
		{
			name:     "Grep with pattern",
			toolName: "Grep",
			input:    map[string]interface{}{"pattern": "func Test"},
			expected: "func Test",
		},
		{
			name:     "WebFetch with url",
			toolName: "WebFetch",
			input:    map[string]interface{}{"url": "https://example.com"},
			expected: "https://example.com",
		},
		{
			name:     "WebSearch with query",
			toolName: "WebSearch",
			input:    map[string]interface{}{"query": "golang testing"},
			expected: "golang testing",
		},
		{
			name:     "Task with description",
			toolName: "Task",
			input:    map[string]interface{}{"description": "run tests"},
			expected: "run tests",
		},
		{
			name:     "nil input",
			toolName: "Read",
			input:    nil,
			expected: "",
		},
		{
			name:     "missing key",
			toolName: "Read",
			input:    map[string]interface{}{"other": "value"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getToolArgument(tt.toolName, tt.input)
			if result != tt.expected {
				t.Errorf("getToolArgument(%q, %v) = %q, want %q", tt.toolName, tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewLogViewer(t *testing.T) {
	lv := NewLogViewer()
	if lv == nil {
		t.Fatal("NewLogViewer returned nil")
	}
	if !lv.autoScroll {
		t.Error("Expected autoScroll to be true by default")
	}
	if len(lv.entries) != 0 {
		t.Error("Expected entries to be empty")
	}
}

func TestLogViewer_Clear(t *testing.T) {
	lv := NewLogViewer()
	lv.entries = []LogEntry{{Text: "test"}}
	lv.scrollPos = 5
	lv.autoScroll = false

	lv.Clear()

	if len(lv.entries) != 0 {
		t.Error("Expected entries to be empty after Clear")
	}
	if lv.scrollPos != 0 {
		t.Error("Expected scrollPos to be 0 after Clear")
	}
	if !lv.autoScroll {
		t.Error("Expected autoScroll to be true after Clear")
	}
}

func TestLogViewer_SetSize(t *testing.T) {
	lv := NewLogViewer()
	lv.SetSize(100, 50)

	if lv.width != 100 {
		t.Errorf("Expected width 100, got %d", lv.width)
	}
	if lv.height != 50 {
		t.Errorf("Expected height 50, got %d", lv.height)
	}
}

func TestLogViewer_IsAutoScrolling(t *testing.T) {
	lv := NewLogViewer()
	if !lv.IsAutoScrolling() {
		t.Error("Expected IsAutoScrolling to be true by default")
	}

	lv.ScrollUp()
	// autoScroll should still be true if scrollPos is at 0
	if !lv.IsAutoScrolling() {
		t.Error("Expected IsAutoScrolling to remain true when at top")
	}
}
