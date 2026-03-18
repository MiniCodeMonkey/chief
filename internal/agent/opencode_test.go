package agent

import (
	"context"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestOpenCodeProvider_Name(t *testing.T) {
	p := NewOpenCodeProvider("")
	if p.Name() != "OpenCode" {
		t.Errorf("Name() = %q, want OpenCode", p.Name())
	}
}

func TestOpenCodeProvider_CLIPath(t *testing.T) {
	p := NewOpenCodeProvider("")
	if p.CLIPath() != "opencode" {
		t.Errorf("CLIPath() empty arg = %q, want opencode", p.CLIPath())
	}
	p2 := NewOpenCodeProvider("/usr/local/bin/opencode")
	if p2.CLIPath() != "/usr/local/bin/opencode" {
		t.Errorf("CLIPath() custom = %q, want /usr/local/bin/opencode", p2.CLIPath())
	}
}

func TestOpenCodeProvider_LogFileName(t *testing.T) {
	p := NewOpenCodeProvider("")
	if p.LogFileName() != "opencode.log" {
		t.Errorf("LogFileName() = %q, want opencode.log", p.LogFileName())
	}
}

func TestOpenCodeProvider_LoopCommand(t *testing.T) {
	ctx := context.Background()
	p := NewOpenCodeProvider("/bin/opencode")
	cmd := p.LoopCommand(ctx, "hello world", "/work/dir")

	if cmd.Path != "/bin/opencode" {
		t.Errorf("LoopCommand Path = %q, want /bin/opencode", cmd.Path)
	}
	wantArgs := []string{"/bin/opencode", "run", "--format", "json", "hello world"}
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

func TestOpenCodeProvider_CleanOutput_PlainText(t *testing.T) {
	p := NewOpenCodeProvider("")
	// Non-NDJSON input should be returned as-is
	input := `{"project": "test"}`
	got := p.CleanOutput(input)
	if got != input {
		t.Errorf("CleanOutput(plain) = %q, want %q", got, input)
	}
}

func TestOpenCodeProvider_CleanOutput_NDJSON(t *testing.T) {
	p := NewOpenCodeProvider("")
	input := `{"type":"step_start","timestamp":1234,"sessionID":"ses_1"}
{"type":"text","timestamp":1235,"sessionID":"ses_1","part":{"id":"prt_1","type":"text","text":"hello world"}}
{"type":"step_finish","timestamp":1236,"sessionID":"ses_1","part":{"id":"prt_2","reason":"stop"}}`
	got := p.CleanOutput(input)
	if got != "hello world" {
		t.Errorf("CleanOutput(ndjson) = %q, want %q", got, "hello world")
	}
}

func TestOpenCodeProvider_CleanOutput_LastTextWins(t *testing.T) {
	p := NewOpenCodeProvider("")
	input := `{"type":"text","timestamp":1,"sessionID":"s","part":{"id":"a","type":"text","text":"first"}}
{"type":"text","timestamp":2,"sessionID":"s","part":{"id":"b","type":"text","text":"second"}}`
	got := p.CleanOutput(input)
	if got != "second" {
		t.Errorf("CleanOutput(multi-text) = %q, want %q", got, "second")
	}
}

func TestOpenCodeProvider_CleanOutput_NoTextEvent(t *testing.T) {
	p := NewOpenCodeProvider("")
	input := `{"type":"step_start","timestamp":1,"sessionID":"s"}
{"type":"step_finish","timestamp":2,"sessionID":"s","part":{"id":"a","reason":"stop"}}`
	got := p.CleanOutput(input)
	if got != input {
		t.Errorf("CleanOutput(no-text) should return original, got %q", got)
	}
}

func TestOpenCodeProvider_InteractiveCommand(t *testing.T) {
	p := NewOpenCodeProvider("opencode")
	cmd := p.InteractiveCommand("/work", "my prompt")
	if cmd.Dir != "/work" {
		t.Errorf("InteractiveCommand Dir = %q, want /work", cmd.Dir)
	}
	if len(cmd.Args) != 3 || cmd.Args[0] != "opencode" || cmd.Args[1] != "--prompt" || cmd.Args[2] != "my prompt" {
		t.Errorf("InteractiveCommand Args = %v, want [opencode --prompt 'my prompt']", cmd.Args)
	}
}

func TestOpenCodeProvider_ParseLine(t *testing.T) {
	p := NewOpenCodeProvider("")
	e := p.ParseLine(`{"type":"step_start","timestamp":1234567890,"sessionID":"ses_test123"}`)
	if e == nil {
		t.Fatal("ParseLine(step_start) returned nil")
	}
	if e.Type != loop.EventIterationStart {
		t.Errorf("ParseLine(step_start) Type = %v, want EventIterationStart", e.Type)
	}
}
