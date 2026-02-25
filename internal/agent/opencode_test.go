package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestOpenCodeProvider_Name(t *testing.T) {
	p := NewOpenCodeProvider("", "")
	if p.Name() != "OpenCode" {
		t.Errorf("Name() = %q, want OpenCode", p.Name())
	}
}

func TestOpenCodeProvider_CLIPath(t *testing.T) {
	p := NewOpenCodeProvider("", "")
	if p.CLIPath() != "opencode" {
		t.Errorf("CLIPath() empty arg = %q, want opencode", p.CLIPath())
	}
	p2 := NewOpenCodeProvider("/usr/local/bin/opencode", "")
	if p2.CLIPath() != "/usr/local/bin/opencode" {
		t.Errorf("CLIPath() custom = %q, want /usr/local/bin/opencode", p2.CLIPath())
	}
}

func TestOpenCodeProvider_LogFileName(t *testing.T) {
	p := NewOpenCodeProvider("", "")
	if p.LogFileName() != "opencode.log" {
		t.Errorf("LogFileName() = %q, want opencode.log", p.LogFileName())
	}
}

func TestOpenCodeProvider_LoopCommand(t *testing.T) {
	ctx := context.Background()
	p := NewOpenCodeProvider("/bin/opencode", "")
	cmd := p.LoopCommand(ctx, "hello world", "/work/dir")

	if cmd.Path != "/bin/opencode" {
		t.Errorf("LoopCommand Path = %q, want /bin/opencode", cmd.Path)
	}
	wantArgs := []string{"/bin/opencode", "run", "--format", "json", "--dir", "/work/dir", "hello world"}
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
	if cmd.Stdin != nil {
		t.Error("LoopCommand Stdin should be nil (prompt passed as positional arg)")
	}
}

func TestOpenCodeProvider_ConvertCommand(t *testing.T) {
	p := NewOpenCodeProvider("opencode", "")
	cmd, mode, outPath, err := p.ConvertCommand("/prd/dir", "convert prompt")
	if err != nil {
		t.Fatalf("ConvertCommand unexpected error: %v", err)
	}
	if mode != loop.OutputStdout {
		t.Errorf("ConvertCommand mode = %v, want OutputStdout", mode)
	}
	if outPath != "" {
		t.Errorf("ConvertCommand outPath = %q, want empty", outPath)
	}
	if !strings.Contains(cmd.Path, "opencode") {
		t.Errorf("ConvertCommand Path = %q", cmd.Path)
	}
	wantArgs := []string{"opencode", "run", "--format", "default", "--dir", "/prd/dir", "convert prompt"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("ConvertCommand Args len = %d, want %d: %v", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i, want := range wantArgs {
		if cmd.Args[i] != want {
			t.Errorf("ConvertCommand Args[%d] = %q, want %q", i, cmd.Args[i], want)
		}
	}
	if cmd.Dir != "/prd/dir" {
		t.Errorf("ConvertCommand Dir = %q, want /prd/dir", cmd.Dir)
	}
}

func TestOpenCodeProvider_FixJSONCommand(t *testing.T) {
	p := NewOpenCodeProvider("opencode", "")
	cmd, mode, outPath, err := p.FixJSONCommand("fix prompt")
	if err != nil {
		t.Fatalf("FixJSONCommand unexpected error: %v", err)
	}
	if mode != loop.OutputStdout {
		t.Errorf("FixJSONCommand mode = %v, want OutputStdout", mode)
	}
	if outPath != "" {
		t.Errorf("FixJSONCommand outPath = %q, want empty", outPath)
	}
	wantArgs := []string{"opencode", "run", "--format", "default", "fix prompt"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("FixJSONCommand Args len = %d, want %d: %v", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i, want := range wantArgs {
		if cmd.Args[i] != want {
			t.Errorf("FixJSONCommand Args[%d] = %q, want %q", i, cmd.Args[i], want)
		}
	}
}

func TestOpenCodeProvider_InteractiveCommand(t *testing.T) {
	p := NewOpenCodeProvider("opencode", "")
	cmd := p.InteractiveCommand("/work", "my prompt")
	if cmd.Dir != "/work" {
		t.Errorf("InteractiveCommand Dir = %q, want /work", cmd.Dir)
	}
	wantArgs := []string{"opencode", "--prompt", "my prompt"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("InteractiveCommand Args len = %d, want %d: %v", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i, want := range wantArgs {
		if cmd.Args[i] != want {
			t.Errorf("InteractiveCommand Args[%d] = %q, want %q", i, cmd.Args[i], want)
		}
	}
}

func TestOpenCodeProvider_ParseLine(t *testing.T) {
	p := NewOpenCodeProvider("", "")
	e := p.ParseLine(`{"type":"thread.started"}`)
	if e == nil {
		t.Fatal("ParseLine(thread.started) returned nil")
	}
	if e.Type != loop.EventIterationStart {
		t.Errorf("ParseLine(thread.started) Type = %v, want EventIterationStart", e.Type)
	}
}

func TestOpenCodeProvider_ParseLineOpenCodeFormat(t *testing.T) {
	p := NewOpenCodeProvider("", "")
	e := p.ParseLine(`{"type":"text","part":{"type":"text","text":"hello"}}`)
	if e == nil {
		t.Fatal("ParseLine(text) returned nil")
	}
	if e.Type != loop.EventAssistantText {
		t.Errorf("ParseLine(text) Type = %v, want EventAssistantText", e.Type)
	}
	if e.Text != "hello" {
		t.Errorf("ParseLine(text) Text = %q, want hello", e.Text)
	}
}

func TestOpenCodeProvider_UsesConfiguredModel(t *testing.T) {
	ctx := context.Background()
	p := NewOpenCodeProvider("opencode", "openai/gpt-5")

	loopCmd := p.LoopCommand(ctx, "prompt", "/work")
	wantLoop := []string{"opencode", "--model", "openai/gpt-5", "run", "--format", "json", "--dir", "/work", "prompt"}
	for i, want := range wantLoop {
		if loopCmd.Args[i] != want {
			t.Fatalf("LoopCommand Args[%d] = %q, want %q", i, loopCmd.Args[i], want)
		}
	}

	interactiveCmd := p.InteractiveCommand("/work", "prompt")
	wantInteractive := []string{"opencode", "--model", "openai/gpt-5", "--prompt", "prompt"}
	for i, want := range wantInteractive {
		if interactiveCmd.Args[i] != want {
			t.Fatalf("InteractiveCommand Args[%d] = %q, want %q", i, interactiveCmd.Args[i], want)
		}
	}
}
