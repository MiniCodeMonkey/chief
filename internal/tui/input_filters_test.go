package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestIsPasteLike locks the exact fallback contract: a KeyMsg is treated as
// a paste iff it has Paste=true (bracketed-paste path) OR it carries more
// than one rune in a single KeyRunes event (terminals without bracketed
// paste deliver pasted clipboard content this way). Single-rune keystrokes
// must NOT be treated as pastes — otherwise invalid single-char input would
// be silently stripped instead of dropped, and the three widgets would
// diverge from their typing-path semantics.
func TestIsPasteLike(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
		want bool
	}{
		{"single rune typing", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, false},
		{"single invalid rune typing", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}}, false},
		{"space keystroke", tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}, false},
		{"bracketed paste single rune", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Paste: true}, true},
		{"bracketed paste multi rune", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo bar"), Paste: true}, true},
		{"non-bracketed multi-rune fallback", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo bar")}, true},
		{"two-rune paste fallback", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'b'}}, true},
		{"empty runes", tea.KeyMsg{Type: tea.KeyRunes, Runes: nil}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPasteLike(tc.msg); got != tc.want {
				t.Fatalf("isPasteLike(%+v) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}

func TestNormalizePastedRunes_PRDName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"all valid unchanged", "my-feature_v2", "my-feature_v2"},
		{"interior run collapses to dash", "my feature v2", "my-feature-v2"},
		{"mixed invalid run collapses", "my f@@!v2", "my-f-v2"},
		{"slash collapses for PRD charset", "feat/oops v2", "feat-oops-v2"},
		{"trailing invalid stripped", "foo!", "foo"},
		{"leading invalid stripped", "!foo", "foo"},
		{"both ends stripped", "!!foo!!", "foo"},
		{"double dash collapses", "foo--bar", "foo-bar"},
		{"dash plus invalid collapses", "foo-@-bar", "foo-bar"},
		{"invalid plus dash collapses", "foo@-bar", "foo-bar"},
		{"long invalid run collapses to one dash", "foo@@@@bar", "foo-bar"},
		{"all invalid yields empty", "!@#$", ""},
		{"empty input yields empty", "", ""},
		{"leading dash preserved", "-foo", "-foo"},
		{"trailing dash preserved", "foo-", "foo-"},
		{"stripped invalid then dash then invalid then valid", "!-@foo", "-foo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(normalizePastedRunes([]rune(tc.in), isAllowedPRDNameRune))
			if got != tc.want {
				t.Fatalf("normalizePastedRunes(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizePastedRunes_BranchName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"slash preserved", "feat/oops", "feat/oops"},
		{"slash preserved around invalid run", "feat/oops bad", "feat/oops-bad"},
		{"trailing bang stripped preserves slash", "feat/oops bad!", "feat/oops-bad"},
		{"unicode stripped, interior run collapses", "féature/v2", "f-ature/v2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(normalizePastedRunes([]rune(tc.in), isAllowedBranchNameRune))
			if got != tc.want {
				t.Fatalf("normalizePastedRunes(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
