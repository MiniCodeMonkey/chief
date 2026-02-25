package main

import "testing"

func TestParseNewArgsRejectsUnknownFlag(t *testing.T) {
	_, _, _, err := parseNewArgs([]string{"--agnt", "codex"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if got, want := err.Error(), "unknown flag: --agnt"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestParseEditArgsRejectsUnknownFlag(t *testing.T) {
	_, _, _, err := parseEditArgs([]string{"--agnt", "codex"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if got, want := err.Error(), "unknown flag: --agnt"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestParseNewArgsParsesNameContextAndAgentFlags(t *testing.T) {
	opts, flagAgent, flagPath, err := parseNewArgs([]string{
		"auth",
		"JWT login",
		"--agent=opencode",
		"--agent-path=/usr/local/bin/opencode",
	})
	if err != nil {
		t.Fatalf("parseNewArgs returned error: %v", err)
	}
	if opts.Name != "auth" {
		t.Fatalf("opts.Name = %q, want auth", opts.Name)
	}
	if opts.Context != "JWT login" {
		t.Fatalf("opts.Context = %q, want %q", opts.Context, "JWT login")
	}
	if flagAgent != "opencode" {
		t.Fatalf("flagAgent = %q, want opencode", flagAgent)
	}
	if flagPath != "/usr/local/bin/opencode" {
		t.Fatalf("flagPath = %q, want /usr/local/bin/opencode", flagPath)
	}
}

func TestParseEditArgsParsesFlagsAndName(t *testing.T) {
	opts, flagAgent, flagPath, err := parseEditArgs([]string{
		"auth",
		"--merge",
		"--force",
		"--agent",
		"codex",
		"--agent-path",
		"/usr/local/bin/codex",
	})
	if err != nil {
		t.Fatalf("parseEditArgs returned error: %v", err)
	}
	if opts.Name != "auth" {
		t.Fatalf("opts.Name = %q, want auth", opts.Name)
	}
	if !opts.Merge {
		t.Fatal("opts.Merge = false, want true")
	}
	if !opts.Force {
		t.Fatal("opts.Force = false, want true")
	}
	if flagAgent != "codex" {
		t.Fatalf("flagAgent = %q, want codex", flagAgent)
	}
	if flagPath != "/usr/local/bin/codex" {
		t.Fatalf("flagPath = %q, want /usr/local/bin/codex", flagPath)
	}
}

func TestParseNewArgsRequiresAgentValue(t *testing.T) {
	_, _, _, err := parseNewArgs([]string{"--agent"})
	if err == nil {
		t.Fatal("expected error for missing --agent value")
	}
	if got, want := err.Error(), "--agent requires a value (claude, codex, or opencode)"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestParseEditArgsRequiresAgentPathValue(t *testing.T) {
	_, _, _, err := parseEditArgs([]string{"--agent-path"})
	if err == nil {
		t.Fatal("expected error for missing --agent-path value")
	}
	if got, want := err.Error(), "--agent-path requires a value"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
