package agent

import (
	"context"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestCursorProvider_Name(t *testing.T) {
	p := NewCursorProvider("")
	if p.Name() != "Cursor" {
		t.Errorf("Name() = %q, want Cursor", p.Name())
	}
}

func TestCursorProvider_CLIPath(t *testing.T) {
	p := NewCursorProvider("")
	if p.CLIPath() != "agent" {
		t.Errorf("CLIPath() empty arg = %q, want agent", p.CLIPath())
	}
	p2 := NewCursorProvider("/usr/local/bin/agent")
	if p2.CLIPath() != "/usr/local/bin/agent" {
		t.Errorf("CLIPath() custom = %q, want /usr/local/bin/agent", p2.CLIPath())
	}
}

func TestCursorProvider_LogFileName(t *testing.T) {
	p := NewCursorProvider("")
	if p.LogFileName() != "cursor.log" {
		t.Errorf("LogFileName() = %q, want cursor.log", p.LogFileName())
	}
}

func TestCursorProvider_LoopCommand(t *testing.T) {
	ctx := context.Background()
	p := NewCursorProvider("/bin/agent")
	cmd := p.LoopCommand(ctx, "hello world", "/work/dir")

	if cmd.Path != "/bin/agent" {
		t.Errorf("LoopCommand Path = %q, want /bin/agent", cmd.Path)
	}
	wantArgs := []string{"/bin/agent", "-p", "--output-format", "stream-json", "--force", "--workspace", "/work/dir", "--trust"}
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
	if cmd.Stdin == nil {
		t.Error("LoopCommand Stdin must be set (prompt via stdin)")
	}
}

func TestCursorProvider_ConvertCommand(t *testing.T) {
	p := NewCursorProvider("/bin/agent")
	cmd, mode, outPath, err := p.ConvertCommand("/prd/dir", "convert prompt")
	if err != nil {
		t.Fatalf("ConvertCommand unexpected error: %v", err)
	}
	if mode != loop.OutputStdout {
		t.Errorf("ConvertCommand mode = %v, want OutputStdout", mode)
	}
	if outPath != "" {
		t.Errorf("ConvertCommand outPath = %q, want empty string", outPath)
	}
	if cmd.Dir != "/prd/dir" {
		t.Errorf("ConvertCommand Dir = %q, want /prd/dir", cmd.Dir)
	}
	wantArgs := []string{"/bin/agent", "-p", "--output-format", "text", "--workspace", "/prd/dir"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("ConvertCommand Args = %v, want %v", cmd.Args, wantArgs)
	}
	for i, w := range wantArgs {
		if cmd.Args[i] != w {
			t.Errorf("ConvertCommand Args[%d] = %q, want %q", i, cmd.Args[i], w)
		}
	}
	if cmd.Stdin == nil {
		t.Error("ConvertCommand Stdin must be set (prompt via stdin)")
	}
}

func TestCursorProvider_FixJSONCommand(t *testing.T) {
	p := NewCursorProvider("/bin/agent")
	cmd, mode, outPath, err := p.FixJSONCommand("fix prompt")
	if err != nil {
		t.Fatalf("FixJSONCommand unexpected error: %v", err)
	}
	if mode != loop.OutputStdout {
		t.Errorf("FixJSONCommand mode = %v, want OutputStdout", mode)
	}
	if outPath != "" {
		t.Errorf("FixJSONCommand outPath = %q, want empty string", outPath)
	}
	wantArgs := []string{"/bin/agent", "-p", "--output-format", "text"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("FixJSONCommand Args = %v, want %v", cmd.Args, wantArgs)
	}
	for i, w := range wantArgs {
		if cmd.Args[i] != w {
			t.Errorf("FixJSONCommand Args[%d] = %q, want %q", i, cmd.Args[i], w)
		}
	}
	if cmd.Stdin == nil {
		t.Error("FixJSONCommand Stdin must be set (prompt via stdin)")
	}
}

func TestCursorProvider_InteractiveCommand(t *testing.T) {
	p := NewCursorProvider("/bin/agent")
	cmd := p.InteractiveCommand("/work", "my prompt")
	if cmd.Dir != "/work" {
		t.Errorf("InteractiveCommand Dir = %q, want /work", cmd.Dir)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "/bin/agent" || cmd.Args[1] != "my prompt" {
		t.Errorf("InteractiveCommand Args = %v, want [/bin/agent my prompt]", cmd.Args)
	}
}

func TestCursorProvider_ParseLine(t *testing.T) {
	p := NewCursorProvider("")
	line := `{"type":"system","subtype":"init","session_id":"x"}`
	e := p.ParseLine(line)
	if e == nil {
		t.Fatal("ParseLine(system init) returned nil")
	}
	if e.Type != loop.EventIterationStart {
		t.Errorf("ParseLine(system init) Type = %v, want EventIterationStart", e.Type)
	}
}

func TestCursorProvider_CleanOutput(t *testing.T) {
	p := NewCursorProvider("")
	// NDJSON: last result/success
	ndjson := `{"type":"system","subtype":"init"}
{"type":"result","subtype":"success","result":"final answer","session_id":"x"}`
	if got := p.CleanOutput(ndjson); got != "final answer" {
		t.Errorf("CleanOutput(NDJSON) = %q, want final answer", got)
	}
	// Single JSON result
	single := `{"type":"result","subtype":"success","result":"single result","session_id":"x"}`
	if got := p.CleanOutput(single); got != "single result" {
		t.Errorf("CleanOutput(single JSON) = %q, want single result", got)
	}
	// No result: return as-is
	plain := "plain text"
	if got := p.CleanOutput(plain); got != plain {
		t.Errorf("CleanOutput(plain) = %q, want %q", got, plain)
	}
}
