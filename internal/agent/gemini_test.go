package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestGeminiProvider_Name(t *testing.T) {
	p := NewGeminiProvider("")
	if p.Name() != "Gemini" {
		t.Errorf("Name() = %q, want Gemini", p.Name())
	}
}

func TestGeminiProvider_CLIPath(t *testing.T) {
	p := NewGeminiProvider("")
	if p.CLIPath() != "gemini" {
		t.Errorf("CLIPath() empty arg = %q, want gemini", p.CLIPath())
	}
	p2 := NewGeminiProvider("/usr/local/bin/gemini")
	if p2.CLIPath() != "/usr/local/bin/gemini" {
		t.Errorf("CLIPath() custom = %q, want /usr/local/bin/gemini", p2.CLIPath())
	}
}

func TestGeminiProvider_LogFileName(t *testing.T) {
	p := NewGeminiProvider("")
	if p.LogFileName() != "gemini.log" {
		t.Errorf("LogFileName() = %q, want gemini.log", p.LogFileName())
	}
}

func TestGeminiProvider_LoopCommand(t *testing.T) {
	ctx := context.Background()
	p := NewGeminiProvider("/bin/gemini")
	cmd := p.LoopCommand(ctx, "hello world", "/work/dir")

	if cmd.Path != "/bin/gemini" {
		t.Errorf("LoopCommand Path = %q, want /bin/gemini", cmd.Path)
	}
	wantArgs := []string{"/bin/gemini", "-p", "hello world", "--output-format", "stream-json", "--yolo"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("LoopCommand Args len = %d, want %d: %v", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i, w := range wantArgs {
		if cmd.Args[i] != w {
			t.Errorf("LoopCommand Args[%d] = %q, want %q", i, cmd.Args[i], w)
		}
	}
	if cmd.Dir != "/work/dir" {
		t.Errorf("LoopCommand Dir = %q, want /work/dir", cmd.Dir)
	}
}

func TestGeminiProvider_InteractiveCommand(t *testing.T) {
	p := NewGeminiProvider("/bin/gemini")
	cmd := p.InteractiveCommand("/work", "my prompt")
	if cmd.Dir != "/work" {
		t.Errorf("InteractiveCommand Dir = %q, want /work", cmd.Dir)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "/bin/gemini" || cmd.Args[1] != "my prompt" {
		t.Errorf("InteractiveCommand Args = %v, want [/bin/gemini my prompt]", cmd.Args)
	}
}

func TestGeminiProvider_ParseLine(t *testing.T) {
	p := NewGeminiProvider("")

	// init event -> EventIterationStart
	e := p.ParseLine(`{"type":"init","timestamp":"2025-01-01T00:00:00.000Z","session_id":"abc","model":"gemini-2.5-pro"}`)
	if e == nil {
		t.Fatal("ParseLine(init) returned nil")
	}
	if e.Type != loop.EventIterationStart {
		t.Errorf("ParseLine(init) Type = %v, want EventIterationStart", e.Type)
	}

	// assistant message -> EventAssistantText
	e = p.ParseLine(`{"type":"message","timestamp":"2025-01-01T00:00:00.000Z","role":"assistant","content":"Hello!","delta":true}`)
	if e == nil {
		t.Fatal("ParseLine(assistant message) returned nil")
	}
	if e.Type != loop.EventAssistantText {
		t.Errorf("ParseLine(assistant message) Type = %v, want EventAssistantText", e.Type)
	}
	if e.Text != "Hello!" {
		t.Errorf("ParseLine(assistant message) Text = %q, want Hello!", e.Text)
	}

	// chief-done tag -> EventStoryDone
	e = p.ParseLine(`{"type":"message","timestamp":"2025-01-01T00:00:00.000Z","role":"assistant","content":"Done <chief-done/>","delta":true}`)
	if e == nil {
		t.Fatal("ParseLine(chief-done) returned nil")
	}
	if e.Type != loop.EventStoryDone {
		t.Errorf("ParseLine(chief-done) Type = %v, want EventStoryDone", e.Type)
	}
}

func TestGeminiProvider_CleanOutput_singleChunk(t *testing.T) {
	p := NewGeminiProvider("")
	input := `{"type":"init","session_id":"s1","model":"gemini-2.5-pro"}
{"type":"message","role":"assistant","content":"Hello from Gemini!","delta":true}
{"type":"result","status":"success","stats":{}}`
	got := p.CleanOutput(input)
	if got != "Hello from Gemini!" {
		t.Errorf("CleanOutput(single chunk) = %q, want %q", got, "Hello from Gemini!")
	}
}

func TestGeminiProvider_CleanOutput_multipleChunks(t *testing.T) {
	p := NewGeminiProvider("")
	input := `{"type":"init","session_id":"s1","model":"gemini-2.5-pro"}
{"type":"message","role":"assistant","content":"Hello ","delta":true}
{"type":"message","role":"assistant","content":"from ","delta":true}
{"type":"message","role":"assistant","content":"Gemini!","delta":true}
{"type":"result","status":"success","stats":{}}`
	got := p.CleanOutput(input)
	want := "Hello from Gemini!"
	if got != want {
		t.Errorf("CleanOutput(multiple chunks) = %q, want %q", got, want)
	}
}

func TestGeminiProvider_CleanOutput_noAssistantMessages(t *testing.T) {
	p := NewGeminiProvider("")
	// When there are no assistant messages, fall back to the raw output.
	input := `{"type":"result","status":"success","stats":{}}`
	got := p.CleanOutput(input)
	if got != input {
		t.Errorf("CleanOutput(no assistant) = %q, want raw output %q", got, input)
	}
}

func TestGeminiProvider_CleanOutput_empty(t *testing.T) {
	p := NewGeminiProvider("")
	if p.CleanOutput("") != "" {
		t.Errorf("CleanOutput('') should return empty string")
	}
	if p.CleanOutput("   ") != "" {
		t.Errorf("CleanOutput('   ') should return empty string")
	}
}

func TestGeminiProvider_CleanOutput_skipsUserMessages(t *testing.T) {
	p := NewGeminiProvider("")
	input := `{"type":"message","role":"user","content":"do something","delta":false}
{"type":"message","role":"assistant","content":"Sure!","delta":true}`
	got := p.CleanOutput(input)
	if got != "Sure!" {
		t.Errorf("CleanOutput(skips user) = %q, want %q", got, "Sure!")
	}
	if strings.Contains(got, "do something") {
		t.Errorf("CleanOutput should not include user messages in output")
	}
}
