package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// newBranchEditMode returns a *BranchWarning in edit mode with the textinput
// pre-populated to value (cursor at end). Mirrors newPickerInputMode /
// newPRDNameSetup so all three widgets' tests share the same fixture style.
func newBranchEditMode(t *testing.T, value string) *BranchWarning {
	t.Helper()
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)
	bw.Reset()
	bw.StartEditMode()
	bw.ti.SetValue(value)
	bw.ti.CursorEnd()
	return bw
}

// sendBranchKey dispatches msg through BranchWarning.UpdateInput — the
// dispatch path introduced in US-008 — returning bw for chaining.
func sendBranchKey(t *testing.T, bw *BranchWarning, msg tea.KeyMsg) *BranchWarning {
	t.Helper()
	bw.UpdateInput(msg)
	return bw
}

func TestBranchInput_LeftArrowMovesCaretLeft(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth") // pos=10
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyLeft})
	if got, want := bw.ti.Position(), 9; got != want {
		t.Fatalf("after left: got pos %d, want %d", got, want)
	}
	if got, want := bw.GetSuggestedBranch(), "chief/auth"; got != want {
		t.Fatalf("value should be unchanged: got %q, want %q", got, want)
	}
}

func TestBranchInput_RightArrowMovesCaretRight(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth")
	bw.ti.SetCursor(0)
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyRight})
	if got, want := bw.ti.Position(), 1; got != want {
		t.Fatalf("after right: got pos %d, want %d", got, want)
	}
}

func TestBranchInput_HomeJumpsToStart(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth")
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyHome})
	if got, want := bw.ti.Position(), 0; got != want {
		t.Fatalf("after home: got pos %d, want %d", got, want)
	}
}

func TestBranchInput_EndJumpsToEnd(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth")
	bw.ti.SetCursor(0)
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyEnd})
	if got, want := bw.ti.Position(), len("chief/auth"); got != want {
		t.Fatalf("after end: got pos %d, want %d", got, want)
	}
}

// TestBranchInput_CtrlLeftStopsAtHyphenInSlashPath confirms the branch-name
// separator set includes `/` alongside `-`/`_`. On "chief/auth-system" with
// caret at end, Ctrl+Left lands just after the last `-` (pos 11, start of
// "system"). Locks in the US-008 decision to use branchNameSeparators for
// git-ref-style paths.
func TestBranchInput_CtrlLeftStopsAtHyphenInSlashPath(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth-system") // pos=17
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyCtrlLeft})
	if got, want := bw.ti.Position(), 11; got != want {
		t.Fatalf("ctrl+left on 'chief/auth-system': got pos %d, want %d", got, want)
	}
}

// TestBranchInput_CtrlLeftStopsAtSlash confirms `/` by itself is treated as
// a word separator.
func TestBranchInput_CtrlLeftStopsAtSlash(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth") // pos=10, "chief/auth"
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyCtrlLeft})
	if got, want := bw.ti.Position(), 6; got != want {
		t.Fatalf("ctrl+left on 'chief/auth': got pos %d, want %d (just after '/')", got, want)
	}
}

func TestBranchInput_CtrlRightJumpsToNextSeparator(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth")
	bw.ti.SetCursor(0)
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyCtrlRight})
	if got, want := bw.ti.Position(), 5; got != want {
		t.Fatalf("ctrl+right on 'chief/auth' from pos 0: got pos %d, want %d", got, want)
	}
}

func TestBranchInput_InsertAtCaret(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth") // pos=10
	bw.ti.SetCursor(6)                       // "chief/|auth"
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if got, want := bw.GetSuggestedBranch(), "chief/Xauth"; got != want {
		t.Fatalf("insert at caret: got %q, want %q", got, want)
	}
	if got, want := bw.ti.Position(), 7; got != want {
		t.Fatalf("cursor should advance: got pos %d, want %d", got, want)
	}
}

func TestBranchInput_BackspaceAtCaret(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth") // pos=10
	bw.ti.SetCursor(6)                       // "chief/|auth" — backspace deletes '/'
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyBackspace})
	if got, want := bw.GetSuggestedBranch(), "chiefauth"; got != want {
		t.Fatalf("backspace at caret: got %q, want %q", got, want)
	}
	if got, want := bw.ti.Position(), 5; got != want {
		t.Fatalf("cursor should move left: got pos %d, want %d", got, want)
	}
}

func TestBranchInput_InvalidAsciiSilentlyDropped(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth")
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	if got, want := bw.GetSuggestedBranch(), "chief/auth"; got != want {
		t.Fatalf("invalid ASCII: got %q, want %q", got, want)
	}
}

// TestBranchInput_InvalidMultiByteRunesSilentlyDropped: é, 中, 🦄 must all be
// filtered by the ASCII-only branch-name charset.
func TestBranchInput_InvalidMultiByteRunesSilentlyDropped(t *testing.T) {
	for _, r := range []rune{'é', '中', '🦄'} {
		bw := newBranchEditMode(t, "chief/auth")
		sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if got, want := bw.GetSuggestedBranch(), "chief/auth"; got != want {
			t.Errorf("multi-byte rune %q: got %q, want %q", r, got, want)
		}
	}
}

// TestBranchInput_SpaceKeyIsFiltered confirms a real spacebar press (which
// arrives with Type=KeySpace, not KeyRunes) is dropped. Same subtle US-003
// bug as the picker/first-time-setup — must be tested explicitly here too.
func TestBranchInput_SpaceKeyIsFiltered(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth")
	pos := bw.ti.Position()
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if got, want := bw.GetSuggestedBranch(), "chief/auth"; got != want {
		t.Fatalf("space key should be filtered: got %q, want %q", got, want)
	}
	if got, want := bw.ti.Position(), pos; got != want {
		t.Fatalf("filtered key should not advance cursor: got pos %d, want %d", got, want)
	}
}

// TestBranchInput_PasteKeepsSlash: branch-name charset includes `/`, so a
// paste like "feat/oops bad!" yields "feat/oops-bad" — the interior space
// collapses to '-' while '/' is preserved (charset difference from the
// PRD-name filter), and the trailing '!' is stripped.
func TestBranchInput_PasteKeepsSlash(t *testing.T) {
	bw := newBranchEditMode(t, "")
	sendBranchKey(t, bw, pasteMsg("feat/oops bad!"))
	if got, want := bw.GetSuggestedBranch(), "feat/oops-bad"; got != want {
		t.Fatalf("paste 'feat/oops bad!': got %q, want %q", got, want)
	}
}

// TestBranchInput_PasteTripleMaxLengthTruncates: paste 3*maxBranchNameLength
// valid characters, value must be truncated to exactly maxBranchNameLength.
// References the constant so tuning the cap later doesn't break this test.
func TestBranchInput_PasteTripleMaxLengthTruncates(t *testing.T) {
	bw := newBranchEditMode(t, "")
	sendBranchKey(t, bw, pasteMsg(strings.Repeat("a", maxBranchNameLength*3)))
	if got := len(bw.GetSuggestedBranch()); got != maxBranchNameLength {
		t.Fatalf("paste length: got %d, want %d", got, maxBranchNameLength)
	}
}

// TestBranchInput_PasteCollapsesInteriorRunAndStripsEnds exercises the full
// paste normalization rule for the branch-name input: leading/trailing
// invalid runes are stripped, interior runs collapse to '-', and consecutive
// '-' collapse. '/' stays since it is in the branch-name charset.
func TestBranchInput_PasteCollapsesInteriorRunAndStripsEnds(t *testing.T) {
	bw := newBranchEditMode(t, "")
	sendBranchKey(t, bw, pasteMsg("!!feat/oops---@@bar!!"))
	if got, want := bw.GetSuggestedBranch(), "feat/oops-bar"; got != want {
		t.Fatalf("normalized paste: got %q, want %q", got, want)
	}
}

// TestBranchInput_PasteAllInvalidIsNoOp verifies that an all-invalid paste
// normalizes to empty and leaves the value unchanged.
func TestBranchInput_PasteAllInvalidIsNoOp(t *testing.T) {
	bw := newBranchEditMode(t, "feat/existing")
	sendBranchKey(t, bw, pasteMsg("! @ # $"))
	if got, want := bw.GetSuggestedBranch(), "feat/existing"; got != want {
		t.Fatalf("all-invalid paste should not change value: got %q, want %q", got, want)
	}
}

// TestBranchInput_PasteWithoutBracketedFlagAlsoNormalized mirrors the
// picker/FirstTimeSetup coverage at the branch-warning widget: a multi-rune
// KeyRunes event without Paste=true is treated as a paste and normalized.
// '/' is in the branch-name charset so it is preserved; the interior space
// collapses to '-' and the trailing '!' is stripped.
func TestBranchInput_PasteWithoutBracketedFlagAlsoNormalized(t *testing.T) {
	bw := newBranchEditMode(t, "")
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("feat/oops bad!")})
	if got, want := bw.GetSuggestedBranch(), "feat/oops-bad"; got != want {
		t.Fatalf("non-bracketed multi-rune paste: got %q, want %q", got, want)
	}
}

// TestBranchInput_TypingAtMaxLengthIsSilentNoOp: once at max length, typing
// any further allowed character is silently dropped.
func TestBranchInput_TypingAtMaxLengthIsSilentNoOp(t *testing.T) {
	full := strings.Repeat("a", maxBranchNameLength)
	bw := newBranchEditMode(t, full)
	if got := len(bw.GetSuggestedBranch()); got != maxBranchNameLength {
		t.Fatalf("precondition: value should be at max length, got %d", got)
	}
	sendBranchKey(t, bw, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if got, want := bw.GetSuggestedBranch(), full; got != want {
		t.Fatalf("typing at max length should not change value: got %q, want %q", got, want)
	}
	if got, want := bw.ti.Position(), maxBranchNameLength; got != want {
		t.Fatalf("cursor should not advance: got pos %d, want %d", got, want)
	}
}

// TestBranchInput_EditValuePreservedAcrossEscapeAndRetoggle: editing the
// branch, pressing esc (CancelEditMode), then re-entering edit mode
// (StartEditMode) must preserve the edited value. Reset() is the only method
// that should reseed the value.
func TestBranchInput_EditValuePreservedAcrossEscapeAndRetoggle(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)
	bw.Reset()

	if got, want := bw.GetSuggestedBranch(), "chief/auth"; got != want {
		t.Fatalf("initial value: got %q, want %q", got, want)
	}

	bw.StartEditMode()
	bw.UpdateInput(tea.KeyMsg{Type: tea.KeyBackspace})
	bw.UpdateInput(tea.KeyMsg{Type: tea.KeyBackspace})
	if got, want := bw.GetSuggestedBranch(), "chief/au"; got != want {
		t.Fatalf("after 2 backspaces: got %q, want %q", got, want)
	}

	bw.CancelEditMode()
	if bw.IsEditMode() {
		t.Fatal("CancelEditMode should exit edit mode")
	}
	if got, want := bw.GetSuggestedBranch(), "chief/au"; got != want {
		t.Fatalf("value after CancelEditMode: got %q, want %q", got, want)
	}

	cmd := bw.StartEditMode()
	if cmd == nil {
		t.Fatal("StartEditMode re-entry should return a non-nil blink cmd")
	}
	if !bw.IsEditMode() {
		t.Fatal("StartEditMode should enter edit mode")
	}
	if got, want := bw.GetSuggestedBranch(), "chief/au"; got != want {
		t.Fatalf("value after re-StartEditMode: got %q, want %q", got, want)
	}
}

// TestBranchInput_StartEditModeReturnsBlinkCmd mirrors
// TestPickerInput_StartInputModeReturnsBlinkCmd: StartEditMode() must return
// a non-nil tea.Cmd that yields the textinput.Blink message type.
func TestBranchInput_StartEditModeReturnsBlinkCmd(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)
	bw.Reset()
	cmd := bw.StartEditMode()
	if cmd == nil {
		t.Fatal("StartEditMode should return a non-nil tea.Cmd")
	}
	msg := cmd()
	wantType := reflect.TypeOf(textinput.Blink())
	if gotType := reflect.TypeOf(msg); gotType != wantType {
		t.Fatalf("cmd should produce %v, got %v", wantType, gotType)
	}
}

// TestBranchInput_CancelEditModeBlursTextinput: after cancel the textinput
// must be blurred so the caret stops blinking.
func TestBranchInput_CancelEditModeBlursTextinput(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)
	bw.Reset()
	bw.StartEditMode()
	if !bw.ti.Focused() {
		t.Fatal("precondition: ti should be focused after StartEditMode")
	}
	bw.CancelEditMode()
	if bw.ti.Focused() {
		t.Fatal("CancelEditMode should leave the textinput blurred")
	}
}

// TestBranchInput_TextinputWidthMatchesModalContent (AC6): ti.Width tracks
// branchWarningInputWidth(terminalWidth) from construction and across SetSize.
func TestBranchInput_TextinputWidthMatchesModalContent(t *testing.T) {
	bw := NewBranchWarning()
	if got, want := bw.ti.Width, branchWarningInputWidth(0); got != want {
		t.Fatalf("initial ti.Width: got %d, want %d", got, want)
	}
	bw.SetSize(120, 40)
	if got, want := bw.ti.Width, branchWarningInputWidth(120); got != want {
		t.Fatalf("ti.Width after SetSize: got %d, want %d", got, want)
	}
}

// TestBranchInput_EmptyAndPopulatedFieldHaveSameRenderedWidth (AC6): the
// edit-mode modal renders to the same max line width whether the textinput
// is empty or populated. Locks in the regression where a non-width-pinned
// textinput would grow the modal as characters were typed.
func TestBranchInput_EmptyAndPopulatedFieldHaveSameRenderedWidth(t *testing.T) {
	empty := NewBranchWarning()
	empty.SetSize(100, 40)
	empty.SetContext("main", "auth", ".chief/worktrees/auth/")
	empty.SetDialogContext(DialogProtectedBranch)
	empty.Reset()
	empty.StartEditMode()
	empty.ti.SetValue("")
	emptyView := empty.Render()

	populated := NewBranchWarning()
	populated.SetSize(100, 40)
	populated.SetContext("main", "auth", ".chief/worktrees/auth/")
	populated.SetDialogContext(DialogProtectedBranch)
	populated.Reset()
	populated.StartEditMode()
	populated.ti.SetValue("chief/auth-system")
	populatedView := populated.Render()

	emptyMax := maxLineWidth(emptyView)
	populatedMax := maxLineWidth(populatedView)
	if emptyMax != populatedMax {
		t.Fatalf("rendered max width should match: empty=%d populated=%d", emptyMax, populatedMax)
	}
}

// TestBranchInput_CtrlCQuitsFromEditMode locks in the ctrl+c dispatch at
// app.go:1095: while the branch-warning modal is in edit mode, ctrl+c must
// quit the app (matching FirstTimeSetup.handlePRDNameKeys), not slip through
// to UpdateInput where textinput.Update would silently swallow it.
func TestBranchInput_CtrlCQuitsFromEditMode(t *testing.T) {
	bw := newBranchEditMode(t, "chief/auth")
	app := App{branchWarning: bw, viewMode: ViewBranchWarning}

	_, cmd := app.handleBranchWarningKeys(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c in branch-warning edit mode should return a non-nil cmd (tea.Quit)")
	}
	if got, want := reflect.TypeOf(cmd()), reflect.TypeOf(tea.Quit()); got != want {
		t.Fatalf("ctrl+c cmd type: got %v, want %v", got, want)
	}
	if got, want := bw.ti.Value(), "chief/auth"; got != want {
		t.Fatalf("ctrl+c must not mutate the textinput value: got %q, want %q", got, want)
	}
}

// TestBranchInput_CtrlCOpensQuitConfirmWhenLoopRunning mirrors the picker
// counterpart: when a loop is running, Ctrl+C in branch-warning edit mode
// must open the quit-confirmation dialog (not quit immediately). Canceling
// with Esc must return the user to the branch-warning modal with edit mode
// still active and the (possibly edited) branch name preserved.
func TestBranchInput_CtrlCOpensQuitConfirmWhenLoopRunning(t *testing.T) {
	bw := newBranchEditMode(t, "chief/my-edit")
	app := App{
		branchWarning: bw,
		manager:       managerWithRunningPRD(t, "current"),
		quitConfirm:   NewQuitConfirmation(),
		viewMode:      ViewBranchWarning,
	}

	model, cmd := app.handleBranchWarningKeys(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatal("ctrl+c must not return tea.Quit while a loop is running")
	}
	after := model.(App)
	if after.viewMode != ViewQuitConfirm {
		t.Fatalf("viewMode after ctrl+c: got %v, want ViewQuitConfirm", after.viewMode)
	}
	if !bw.IsEditMode() {
		t.Fatal("branch-warning must remain in edit mode across the quit-confirm detour")
	}
	if got, want := bw.ti.Value(), "chief/my-edit"; got != want {
		t.Fatalf("branch value after ctrl+c: got %q, want %q", got, want)
	}

	model, _ = after.handleQuitConfirmKeys(tea.KeyMsg{Type: tea.KeyEsc})
	back := model.(App)
	if back.viewMode != ViewBranchWarning {
		t.Fatalf("viewMode after cancel: got %v, want ViewBranchWarning", back.viewMode)
	}
	if !bw.IsEditMode() {
		t.Fatal("branch-warning must still be in edit mode after cancel")
	}
	if got, want := bw.ti.Value(), "chief/my-edit"; got != want {
		t.Fatalf("branch value after cancel: got %q, want %q", got, want)
	}
}
