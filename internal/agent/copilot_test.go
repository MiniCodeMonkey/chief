package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestCopilotProvider(t *testing.T) {
	p := NewCopilotProvider("")
	if p.Name() != "Copilot" {
		t.Errorf("Name() = %q, want Copilot", p.Name())
	}
	if p.CLIPath() != "copilot" {
		t.Errorf("CLIPath() = %q, want copilot", p.CLIPath())
	}
	if p.LogFileName() != "copilot.log" {
		t.Errorf("LogFileName() = %q, want copilot.log", p.LogFileName())
	}

	custom := NewCopilotProvider("/opt/copilot")
	if custom.CLIPath() != "/opt/copilot" {
		t.Errorf("custom CLIPath() = %q, want /opt/copilot", custom.CLIPath())
	}
}

func TestCopilotProvider_LoopCommand(t *testing.T) {
	p := NewCopilotProvider("/bin/copilot")
	cmd := p.LoopCommand(context.Background(), "build it", "/work")

	wantArgs := []string{
		"/bin/copilot", "-p", "build it",
		"--output-format", "json",
		"--allow-all",
		"--no-ask-user",
		"--no-color",
		"--no-auto-update",
		"--no-remote",
		"--no-remote-export",
	}
	if strings.Join(cmd.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Errorf("LoopCommand Args = %v, want %v", cmd.Args, wantArgs)
	}
	if cmd.Dir != "/work" {
		t.Errorf("LoopCommand Dir = %q, want /work", cmd.Dir)
	}
}

func TestCopilotProvider_InteractiveCommand(t *testing.T) {
	p := NewCopilotProvider("/bin/copilot")
	cmd := p.InteractiveCommand("/work", "review this")
	wantArgs := []string{"/bin/copilot", "-i", "review this"}
	if strings.Join(cmd.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Errorf("InteractiveCommand Args = %v, want %v", cmd.Args, wantArgs)
	}
	if cmd.Dir != "/work" {
		t.Errorf("InteractiveCommand Dir = %q, want /work", cmd.Dir)
	}
}

func TestCopilotProvider_ParseLine(t *testing.T) {
	p := NewCopilotProvider("")
	event := p.ParseLine(`{"type":"assistant.message","data":{"content":"done <chief-done/>"}}`)
	if event == nil || event.Type != loop.EventStoryDone {
		t.Fatalf("ParseLine() = %#v, want EventStoryDone", event)
	}
}

func TestCopilotProvider_CleanOutput(t *testing.T) {
	p := NewCopilotProvider("")
	output := `{"type":"assistant.message","data":{"content":"first"}}
{"type":"tool.execution_start","data":{"toolName":"bash"}}
{"type":"assistant.message","data":{"content":"second"}}`
	if got := p.CleanOutput(output); got != "first\nsecond" {
		t.Errorf("CleanOutput() = %q, want %q", got, "first\nsecond")
	}
	if got := p.CleanOutput("plain text"); got != "plain text" {
		t.Errorf("CleanOutput(plain) = %q, want plain text", got)
	}
}
