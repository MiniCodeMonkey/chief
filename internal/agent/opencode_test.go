package agent

import (
	"context"
	"strings"
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
	wantArgs := []string{"/bin/opencode", "exec", "--json", "--yolo", "-C", "/work/dir", "-"}
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
		t.Error("LoopCommand Stdin must be set (prompt on stdin)")
	}
}

func TestOpenCodeProvider_ConvertCommand(t *testing.T) {
	p := NewOpenCodeProvider("opencode")
	cmd, mode, outPath, err := p.ConvertCommand("/prd/dir", "convert prompt")
	if err != nil {
		t.Fatalf("ConvertCommand unexpected error: %v", err)
	}
	if mode != loop.OutputFromFile {
		t.Errorf("ConvertCommand mode = %v, want OutputFromFile", mode)
	}
	if outPath == "" {
		t.Error("ConvertCommand outPath should be non-empty temp file")
	}
	if !strings.Contains(cmd.Path, "opencode") {
		t.Errorf("ConvertCommand Path = %q", cmd.Path)
	}
	foundO := false
	for i, a := range cmd.Args {
		if a == "-o" && i+1 < len(cmd.Args) && cmd.Args[i+1] == outPath {
			foundO = true
			break
		}
	}
	if !foundO {
		t.Errorf("ConvertCommand should have -o %q in args: %v", outPath, cmd.Args)
	}
	if cmd.Dir != "/prd/dir" {
		t.Errorf("ConvertCommand Dir = %q, want /prd/dir", cmd.Dir)
	}
}

func TestOpenCodeProvider_FixJSONCommand(t *testing.T) {
	p := NewOpenCodeProvider("opencode")
	cmd, mode, outPath, err := p.FixJSONCommand("fix prompt")
	if err != nil {
		t.Fatalf("FixJSONCommand unexpected error: %v", err)
	}
	if mode != loop.OutputFromFile {
		t.Errorf("FixJSONCommand mode = %v, want OutputFromFile", mode)
	}
	if outPath == "" {
		t.Error("FixJSONCommand outPath should be non-empty temp file")
	}
	foundO := false
	for i, a := range cmd.Args {
		if a == "-o" && i+1 < len(cmd.Args) && cmd.Args[i+1] == outPath {
			foundO = true
			break
		}
	}
	if !foundO {
		t.Errorf("FixJSONCommand should have -o %q in args: %v", outPath, cmd.Args)
	}
}

func TestOpenCodeProvider_InteractiveCommand(t *testing.T) {
	p := NewOpenCodeProvider("opencode")
	cmd := p.InteractiveCommand("/work", "my prompt")
	if cmd.Dir != "/work" {
		t.Errorf("InteractiveCommand Dir = %q, want /work", cmd.Dir)
	}
	if len(cmd.Args) < 2 || cmd.Args[0] != "opencode" || cmd.Args[1] != "my prompt" {
		t.Errorf("InteractiveCommand Args = %v", cmd.Args)
	}
}

func TestOpenCodeProvider_ParseLine(t *testing.T) {
	p := NewOpenCodeProvider("")
	e := p.ParseLine(`{"type":"thread.started"}`)
	if e == nil {
		t.Fatal("ParseLine(thread.started) returned nil")
	}
	if e.Type != loop.EventIterationStart {
		t.Errorf("ParseLine(thread.started) Type = %v, want EventIterationStart", e.Type)
	}
}
