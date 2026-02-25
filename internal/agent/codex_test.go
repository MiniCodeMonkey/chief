package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestCodexProvider_Name(t *testing.T) {
	p := NewCodexProvider("")
	if p.Name() != "Codex" {
		t.Errorf("Name() = %q, want Codex", p.Name())
	}
}

func TestCodexProvider_CLIPath(t *testing.T) {
	p := NewCodexProvider("")
	if p.CLIPath() != "codex" {
		t.Errorf("CLIPath() empty arg = %q, want codex", p.CLIPath())
	}
	p2 := NewCodexProvider("/usr/local/bin/codex")
	if p2.CLIPath() != "/usr/local/bin/codex" {
		t.Errorf("CLIPath() custom = %q, want /usr/local/bin/codex", p2.CLIPath())
	}
}

func TestCodexProvider_LogFileName(t *testing.T) {
	p := NewCodexProvider("")
	if p.LogFileName() != "codex.log" {
		t.Errorf("LogFileName() = %q, want codex.log", p.LogFileName())
	}
}

func TestCodexProvider_LoopCommand(t *testing.T) {
	ctx := context.Background()
	p := NewCodexProvider("/bin/codex")
	cmd := p.LoopCommand(ctx, "hello world", "/work/dir")

	if cmd.Path != "/bin/codex" {
		t.Errorf("LoopCommand Path = %q, want /bin/codex", cmd.Path)
	}
	wantArgs := []string{"/bin/codex", "exec", "--json", "--yolo", "-C", "/work/dir", "-"}
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
	// Stdin should contain the prompt
	// We can't easily read cmd.Stdin without running; just check it's non-nil (done above)
}

func TestCodexProvider_ConvertCommand(t *testing.T) {
	p := NewCodexProvider("codex")
	cmd, mode, outPath := p.ConvertCommand("/prd/dir", "convert prompt")
	if mode != loop.OutputFromFile {
		t.Errorf("ConvertCommand mode = %v, want OutputFromFile", mode)
	}
	if outPath == "" {
		t.Error("ConvertCommand outPath should be non-empty temp file")
	}
	if !strings.Contains(cmd.Path, "codex") {
		t.Errorf("ConvertCommand Path = %q", cmd.Path)
	}
	// Should have -o outPath
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

func TestCodexProvider_FixJSONCommand(t *testing.T) {
	p := NewCodexProvider("codex")
	cmd, mode, outPath := p.FixJSONCommand("fix prompt")
	if mode != loop.OutputFromFile {
		t.Errorf("FixJSONCommand mode = %v, want OutputFromFile", mode)
	}
	if outPath == "" {
		t.Error("FixJSONCommand outPath should be non-empty temp file")
	}
	// -o outPath present
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

func TestCodexProvider_InteractiveCommand(t *testing.T) {
	p := NewCodexProvider("codex")
	cmd := p.InteractiveCommand("/work", "my prompt")
	if cmd.Dir != "/work" {
		t.Errorf("InteractiveCommand Dir = %q, want /work", cmd.Dir)
	}
	if len(cmd.Args) < 2 || cmd.Args[0] != "codex" || cmd.Args[1] != "my prompt" {
		t.Errorf("InteractiveCommand Args = %v", cmd.Args)
	}
}

func TestCodexProvider_ParseLine(t *testing.T) {
	p := NewCodexProvider("")
	// thread.started -> EventIterationStart
	e := p.ParseLine(`{"type":"thread.started"}`)
	if e == nil {
		t.Fatal("ParseLine(thread.started) returned nil")
	}
	if e.Type != loop.EventIterationStart {
		t.Errorf("ParseLine(thread.started) Type = %v, want EventIterationStart", e.Type)
	}
}
