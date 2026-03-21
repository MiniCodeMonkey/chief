package commands

import (
	"testing"
)

func TestRootCommandHasExpectedSubcommands(t *testing.T) {
	expected := []string{"new", "edit", "status", "list", "update", "wiggum"}
	commands := rootCmd.Commands()

	found := make(map[string]bool)
	for _, cmd := range commands {
		found[cmd.Name()] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestPersistentFlagsRegistered(t *testing.T) {
	flags := []string{"agent", "agent-path", "verbose"}
	for _, name := range flags {
		if rootCmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("expected persistent flag %q not found", name)
		}
	}
}

func TestLocalFlagsRegistered(t *testing.T) {
	flags := []string{"max-iterations", "no-retry", "merge", "force"}
	for _, name := range flags {
		if rootCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected local flag %q not found", name)
		}
	}
}

func TestMaxIterationsHasShorthand(t *testing.T) {
	f := rootCmd.Flags().Lookup("max-iterations")
	if f == nil {
		t.Fatal("max-iterations flag not found")
	}
	if f.Shorthand != "n" {
		t.Errorf("expected shorthand 'n', got %q", f.Shorthand)
	}
}

func TestEditCommandHasMergeAndForceFlags(t *testing.T) {
	if editCmd.Flags().Lookup("merge") == nil {
		t.Error("edit command missing --merge flag")
	}
	if editCmd.Flags().Lookup("force") == nil {
		t.Error("edit command missing --force flag")
	}
}
