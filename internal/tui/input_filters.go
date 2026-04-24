package tui

import tea "github.com/charmbracelet/bubbletea"

// isTextualKey reports whether a KeyMsg carries rune input that must run
// through a charset filter. bubbles delivers a bare spacebar press as
// KeySpace (not KeyRunes), so both types must be handled — otherwise spaces
// would slip past the filter. Shared by FirstTimeSetup, PRDPicker, and
// BranchWarning so the three widgets can't drift on this corner case.
func isTextualKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace
}

// isPasteLike reports whether a KeyMsg should be treated as a paste for the
// purposes of input normalization. A bracketed-paste event sets Paste=true
// explicitly; terminals that do not advertise bracketed paste still deliver
// the clipboard as a single multi-rune KeyRunes message, which we treat as
// a paste too so behavior stays consistent across terminals. Single-rune
// input (keystrokes) always goes through the drop filter instead — there is
// no "run" of invalid chars to collapse in that case.
func isPasteLike(msg tea.KeyMsg) bool {
	return msg.Paste || len(msg.Runes) > 1
}

// prdNameSeparators are the word-separator runes used by PRD-name editors
// (both FirstTimeSetup's StepPRDName and PRDPicker's new-PRD-name input) for
// Ctrl+Left/Right word jumps. Defined once so the two widgets can't drift.
var prdNameSeparators = []rune{'-', '_'}

// branchNameSeparators are the word-separator runes used by the BranchWarning
// branch-name editor for Ctrl+Left/Right word jumps.
var branchNameSeparators = []rune{'-', '_', '/'}

// isAllowedPRDNameRune reports whether r is in the PRD-name charset
// ([a-zA-Z0-9_-]).
func isAllowedPRDNameRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_'
}

// isAllowedBranchNameRune reports whether r is in the branch-name charset
// ([a-zA-Z0-9_/-]).
func isAllowedBranchNameRune(r rune) bool {
	return isAllowedPRDNameRune(r) || r == '/'
}

// filterPRDNameRunes drops any rune outside the allowed PRD-name character
// set ([a-zA-Z0-9_-]). Returns a new slice so the caller can safely forward
// the filtered KeyMsg to the textinput.
func filterPRDNameRunes(runes []rune) []rune {
	return filterRunes(runes, isAllowedPRDNameRune)
}

// filterBranchNameRunes drops any rune outside the allowed branch-name
// character set ([a-zA-Z0-9_/-]). Returns a new slice so the caller can safely
// forward the filtered KeyMsg to the textinput.
func filterBranchNameRunes(runes []rune) []rune {
	return filterRunes(runes, isAllowedBranchNameRune)
}

func filterRunes(runes []rune, allowed func(rune) bool) []rune {
	filtered := make([]rune, 0, len(runes))
	for _, r := range runes {
		if allowed(r) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// normalizePastedRunes is the paste-time counterpart of filterRunes: instead
// of silently dropping disallowed runes, it replaces any interior run of
// disallowed runes with a single '-' and strips disallowed runes at the start
// and end. Consecutive '-' runes (either already present in the paste or
// introduced by the replacement) collapse to a single '-' so the result never
// contains "--". Normalization is scoped to the pasted content — adjacency
// with runes already in the target field is intentionally not considered.
func normalizePastedRunes(runes []rune, allowed func(rune) bool) []rune {
	start := 0
	for start < len(runes) && !allowed(runes[start]) {
		start++
	}
	end := len(runes)
	for end > start && !allowed(runes[end-1]) {
		end--
	}

	out := make([]rune, 0, end-start)
	pendingDash := false
	for i := start; i < end; i++ {
		r := runes[i]
		if !allowed(r) {
			pendingDash = true
			continue
		}
		if r == '-' {
			pendingDash = false
			if len(out) > 0 && out[len(out)-1] == '-' {
				continue
			}
			out = append(out, '-')
			continue
		}
		if pendingDash {
			if len(out) == 0 || out[len(out)-1] != '-' {
				out = append(out, '-')
			}
			pendingDash = false
		}
		out = append(out, r)
	}
	return out
}

// wordBackward returns the caret position after a word-jump-left from pos,
// treating any rune in seps as a word separator. Mirrors bubbles'
// wordBackward structure (skip separators, then skip non-separators) so
// behavior is predictable next to the built-in key bindings.
func wordBackward(value []rune, pos int, seps []rune) int {
	if pos <= 0 || len(value) == 0 {
		return 0
	}
	i := pos - 1
	for i >= 0 && isSeparator(value[i], seps) {
		i--
	}
	for i >= 0 && !isSeparator(value[i], seps) {
		i--
	}
	return i + 1
}

// wordForward is the forward counterpart of wordBackward.
func wordForward(value []rune, pos int, seps []rune) int {
	n := len(value)
	if pos >= n {
		return n
	}
	i := pos
	for i < n && isSeparator(value[i], seps) {
		i++
	}
	for i < n && !isSeparator(value[i], seps) {
		i++
	}
	return i
}

func isSeparator(r rune, seps []rune) bool {
	for _, s := range seps {
		if r == s {
			return true
		}
	}
	return false
}
