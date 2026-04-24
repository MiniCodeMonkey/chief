package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/minicodemonkey/chief/internal/loop"
)

// newPickerInputMode returns a *PRDPicker in input mode with the textinput
// pre-populated to value and the cursor at end. Mirrors newPRDNameSetup from
// first_time_setup_test.go.
func newPickerInputMode(t *testing.T, value string) *PRDPicker {
	t.Helper()
	p := NewPRDPicker(t.TempDir(), "", nil)
	p.StartInputMode()
	p.ti.SetValue(value)
	p.ti.CursorEnd()
	return p
}

// sendPickerKey dispatches msg through PRDPicker.UpdateInput — the new
// dispatch path introduced in US-007 — returning the picker for chaining.
func sendPickerKey(t *testing.T, p *PRDPicker, msg tea.KeyMsg) *PRDPicker {
	t.Helper()
	p.UpdateInput(msg)
	return p
}

func TestPickerInput_LeftArrowMovesCaretLeft(t *testing.T) {
	p := newPickerInputMode(t, "main") // pos=4
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyLeft})
	if got, want := p.ti.Position(), 3; got != want {
		t.Fatalf("after left: got pos %d, want %d", got, want)
	}
	if got, want := p.ti.Value(), "main"; got != want {
		t.Fatalf("value should be unchanged: got %q, want %q", got, want)
	}
}

func TestPickerInput_RightArrowMovesCaretRight(t *testing.T) {
	p := newPickerInputMode(t, "main")
	p.ti.SetCursor(0)
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyRight})
	if got, want := p.ti.Position(), 1; got != want {
		t.Fatalf("after right: got pos %d, want %d", got, want)
	}
}

func TestPickerInput_HomeJumpsToStart(t *testing.T) {
	p := newPickerInputMode(t, "main")
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyHome})
	if got, want := p.ti.Position(), 0; got != want {
		t.Fatalf("after home: got pos %d, want %d", got, want)
	}
}

func TestPickerInput_EndJumpsToEnd(t *testing.T) {
	p := newPickerInputMode(t, "main")
	p.ti.SetCursor(0)
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyEnd})
	if got, want := p.ti.Position(), 4; got != want {
		t.Fatalf("after end: got pos %d, want %d", got, want)
	}
}

// TestPickerInput_CtrlLeftStopsAtHyphen confirms the shared word-jump helper
// treats `-` as a separator — stopping Ctrl+Left just past the hyphen so
// inserting 'X' at the new caret yields "foo-Xbar".
func TestPickerInput_CtrlLeftStopsAtHyphen(t *testing.T) {
	p := newPickerInputMode(t, "foo-bar") // pos=7
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyCtrlLeft})
	if got, want := p.ti.Position(), 4; got != want {
		t.Fatalf("ctrl+left on 'foo-bar': got pos %d, want %d", got, want)
	}
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if got, want := p.ti.Value(), "foo-Xbar"; got != want {
		t.Fatalf("ctrl+left + 'X' on 'foo-bar': got %q, want %q", got, want)
	}
}

// TestPickerInput_CtrlLeftStopsAtUnderscore confirms `_` is also a separator
// for the PRD-name charset.
func TestPickerInput_CtrlLeftStopsAtUnderscore(t *testing.T) {
	p := newPickerInputMode(t, "foo_bar") // pos=7
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyCtrlLeft})
	if got, want := p.ti.Position(), 4; got != want {
		t.Fatalf("ctrl+left on 'foo_bar': got pos %d, want %d", got, want)
	}
}

func TestPickerInput_CtrlRightJumpsToNextSeparator(t *testing.T) {
	p := newPickerInputMode(t, "foo-bar")
	p.ti.SetCursor(0)
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyCtrlRight})
	if got, want := p.ti.Position(), 3; got != want {
		t.Fatalf("ctrl+right on 'foo-bar' from pos 0: got pos %d, want %d", got, want)
	}
}

func TestPickerInput_InsertAtCaret(t *testing.T) {
	p := newPickerInputMode(t, "main") // value=main, pos=4
	p.ti.SetCursor(2)                  // "ma|in"
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if got, want := p.ti.Value(), "maXin"; got != want {
		t.Fatalf("insert at caret: got %q, want %q", got, want)
	}
	if got, want := p.ti.Position(), 3; got != want {
		t.Fatalf("cursor should advance past inserted rune: got pos %d, want %d", got, want)
	}
}

func TestPickerInput_BackspaceAtCaret(t *testing.T) {
	p := newPickerInputMode(t, "main")
	p.ti.SetCursor(2) // "ma|in" — backspace deletes 'a'
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyBackspace})
	if got, want := p.ti.Value(), "min"; got != want {
		t.Fatalf("backspace at caret: got %q, want %q", got, want)
	}
	if got, want := p.ti.Position(), 1; got != want {
		t.Fatalf("cursor should move left after backspace: got pos %d, want %d", got, want)
	}
}

func TestPickerInput_InvalidAsciiSilentlyDropped(t *testing.T) {
	p := newPickerInputMode(t, "main")
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	if got, want := p.ti.Value(), "main"; got != want {
		t.Fatalf("invalid ASCII: got %q, want %q", got, want)
	}
}

// TestPickerInput_InvalidMultiByteRunesSilentlyDropped: é, 中, 🦄 must all be
// filtered by the ASCII-only PRD-name charset.
func TestPickerInput_InvalidMultiByteRunesSilentlyDropped(t *testing.T) {
	for _, r := range []rune{'é', '中', '🦄'} {
		p := newPickerInputMode(t, "main")
		sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if got, want := p.ti.Value(), "main"; got != want {
			t.Errorf("multi-byte rune %q: got %q, want %q", r, got, want)
		}
	}
}

// TestPickerInput_SpaceKeyIsFiltered confirms a real spacebar press (which
// arrives with Type=KeySpace, not KeyRunes) is dropped before reaching the
// textinput. Mirrors TestPRDName_SpaceKeyIsFiltered — the subtle US-003 bug
// that must be tested explicitly on every widget.
func TestPickerInput_SpaceKeyIsFiltered(t *testing.T) {
	p := newPickerInputMode(t, "main")
	p.ti.SetCursor(2)
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if got, want := p.ti.Value(), "main"; got != want {
		t.Fatalf("space key should be filtered: got %q, want %q", got, want)
	}
	if got, want := p.ti.Position(), 2; got != want {
		t.Fatalf("filtered key should not advance cursor: got pos %d, want %d", got, want)
	}
}

// TestPickerInput_PasteFiltersInvalidChars: paste "my feature/v2!" →
// "my-feature-v2" (interior invalid runs collapsed to '-', trailing '!'
// stripped).
func TestPickerInput_PasteFiltersInvalidChars(t *testing.T) {
	p := newPickerInputMode(t, "")
	sendPickerKey(t, p, pasteMsg("my feature/v2!"))
	if got, want := p.ti.Value(), "my-feature-v2"; got != want {
		t.Fatalf("paste filtered: got %q, want %q", got, want)
	}
}

// TestPickerInput_PasteTripleMaxLengthTruncates: paste 3*maxPRDNameLength
// valid characters, value must be truncated to exactly maxPRDNameLength.
// References the constant so tuning the cap later doesn't break this test.
func TestPickerInput_PasteTripleMaxLengthTruncates(t *testing.T) {
	p := newPickerInputMode(t, "")
	sendPickerKey(t, p, pasteMsg(strings.Repeat("a", maxPRDNameLength*3)))
	if got := len(p.ti.Value()); got != maxPRDNameLength {
		t.Fatalf("paste length: got %d, want %d", got, maxPRDNameLength)
	}
}

// TestPickerInput_PasteCollapsesInteriorRunAndStripsEnds exercises the full
// paste normalization rule for the picker's new-PRD-name input.
func TestPickerInput_PasteCollapsesInteriorRunAndStripsEnds(t *testing.T) {
	p := newPickerInputMode(t, "")
	sendPickerKey(t, p, pasteMsg("!!foo---@@bar!!"))
	if got, want := p.ti.Value(), "foo-bar"; got != want {
		t.Fatalf("normalized paste: got %q, want %q", got, want)
	}
}

// TestPickerInput_PasteAllInvalidIsNoOp verifies that an all-invalid paste
// normalizes to an empty slice and leaves the value unchanged.
func TestPickerInput_PasteAllInvalidIsNoOp(t *testing.T) {
	p := newPickerInputMode(t, "main")
	sendPickerKey(t, p, pasteMsg("! @ # $"))
	if got, want := p.ti.Value(), "main"; got != want {
		t.Fatalf("all-invalid paste should not change value: got %q, want %q", got, want)
	}
}

// TestPickerInput_PasteWithoutBracketedFlagAlsoNormalized mirrors the
// FirstTimeSetup coverage at the picker widget: a multi-rune KeyRunes event
// without Paste=true (terminals without bracketed paste) is treated as a
// paste and normalized — runs of invalid chars collapse to '-', trailing
// invalid chars are stripped.
func TestPickerInput_PasteWithoutBracketedFlagAlsoNormalized(t *testing.T) {
	p := newPickerInputMode(t, "")
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab/cd!")})
	if got, want := p.ti.Value(), "ab-cd"; got != want {
		t.Fatalf("non-bracketed multi-rune paste: got %q, want %q", got, want)
	}
}

// TestPickerInput_TypingAtMaxLengthIsSilentNoOp: once at max length, typing
// any further allowed character is silently dropped (value unchanged, cursor
// unchanged).
func TestPickerInput_TypingAtMaxLengthIsSilentNoOp(t *testing.T) {
	full := strings.Repeat("a", maxPRDNameLength)
	p := newPickerInputMode(t, full)
	if got := len(p.ti.Value()); got != maxPRDNameLength {
		t.Fatalf("precondition: value should be at max length, got %d", got)
	}
	sendPickerKey(t, p, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if got, want := p.ti.Value(), full; got != want {
		t.Fatalf("typing at max length should not change value: got %q, want %q", got, want)
	}
	if got, want := p.ti.Position(), maxPRDNameLength; got != want {
		t.Fatalf("cursor should not advance past max length: got pos %d, want %d", got, want)
	}
}

// TestPickerInput_StartInputModeReturnsBlinkCmd mirrors US-006's
// TestUS006_GitignoreToPRDNameBlinkCmd: StartInputMode() must return a non-nil
// tea.Cmd that yields the textinput.Blink message type — otherwise the caret
// never blinks (FR-10 regression).
func TestPickerInput_StartInputModeReturnsBlinkCmd(t *testing.T) {
	p := NewPRDPicker(t.TempDir(), "", nil)
	cmd := p.StartInputMode()
	if cmd == nil {
		t.Fatal("StartInputMode should return a non-nil tea.Cmd")
	}
	msg := cmd()
	wantType := reflect.TypeOf(textinput.Blink())
	if gotType := reflect.TypeOf(msg); gotType != wantType {
		t.Fatalf("cmd should produce %v, got %v", wantType, gotType)
	}
}

// TestPickerInput_CancelInputModeBlursTextinput: after cancel the textinput
// must be blurred so the caret stops blinking.
func TestPickerInput_CancelInputModeBlursTextinput(t *testing.T) {
	p := NewPRDPicker(t.TempDir(), "", nil)
	p.StartInputMode()
	if !p.ti.Focused() {
		t.Fatal("precondition: ti should be focused after StartInputMode")
	}
	p.CancelInputMode()
	if p.ti.Focused() {
		t.Fatal("CancelInputMode should leave the textinput blurred")
	}
}

// TestPickerInput_TextinputWidthMatchesModalContent (AC6): ti.Width tracks
// pickerInputWidth(terminalWidth) from construction and across SetSize.
func TestPickerInput_TextinputWidthMatchesModalContent(t *testing.T) {
	p := NewPRDPicker(t.TempDir(), "", nil)
	if got, want := p.ti.Width, pickerInputWidth(0); got != want {
		t.Fatalf("initial ti.Width: got %d, want %d", got, want)
	}
	p.SetSize(120, 40)
	if got, want := p.ti.Width, pickerInputWidth(120); got != want {
		t.Fatalf("ti.Width after SetSize: got %d, want %d", got, want)
	}
}

// TestPickerInput_EmptyAndPopulatedFieldHaveSameRenderedWidth (AC6): the
// input-mode modal renders to the same max line width whether the textinput
// is empty or populated. Locks in the regression where a custom renderer
// would jitter the modal width as characters were typed.
func TestPickerInput_EmptyAndPopulatedFieldHaveSameRenderedWidth(t *testing.T) {
	empty := NewPRDPicker(t.TempDir(), "", nil)
	empty.SetSize(100, 40)
	empty.StartInputMode()
	empty.ti.SetValue("")
	emptyView := empty.Render()

	populated := NewPRDPicker(t.TempDir(), "", nil)
	populated.SetSize(100, 40)
	populated.StartInputMode()
	populated.ti.SetValue("main")
	populatedView := populated.Render()

	emptyMax := maxLineWidth(emptyView)
	populatedMax := maxLineWidth(populatedView)
	if emptyMax != populatedMax {
		t.Fatalf("rendered max width should match: empty=%d populated=%d", emptyMax, populatedMax)
	}
}

// TestPickerInput_CtrlCQuitsFromInputMode locks in the ctrl+c dispatch at
// app.go:1831: while the picker is in input mode, ctrl+c must quit the app
// (matching FirstTimeSetup.handlePRDNameKeys), not slip through to
// UpdateInput where textinput.Update would silently swallow it.
func TestPickerInput_CtrlCQuitsFromInputMode(t *testing.T) {
	picker := NewPRDPicker(t.TempDir(), "", nil)
	picker.StartInputMode()
	picker.ti.SetValue("main")
	app := App{picker: picker, viewMode: ViewPicker}

	_, cmd := app.handlePickerKeys(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c in picker input mode should return a non-nil cmd (tea.Quit)")
	}
	if got, want := reflect.TypeOf(cmd()), reflect.TypeOf(tea.Quit()); got != want {
		t.Fatalf("ctrl+c cmd type: got %v, want %v", got, want)
	}
	if got, want := picker.ti.Value(), "main"; got != want {
		t.Fatalf("ctrl+c must not mutate the textinput value: got %q, want %q", got, want)
	}
}

// managerWithRunningPRD returns a *loop.Manager with one instance whose State
// is forced to LoopStateRunning. Used to exercise the tryQuit branch that
// routes through the quit-confirmation dialog.
func managerWithRunningPRD(t *testing.T, name string) *loop.Manager {
	t.Helper()
	m := loop.NewManager(10, nil)
	if err := m.Register(name, "/tmp/"+name); err != nil {
		t.Fatalf("Register: %v", err)
	}
	// GetInstance returns a copy, so mutating its fields would not affect the
	// manager's internal instance. Use the dedicated test-oriented setter.
	if !m.SetInstanceState(name, loop.LoopStateRunning) {
		t.Fatalf("SetInstanceState: instance %q not found after Register", name)
	}
	return m
}

// TestPickerInput_CtrlCOpensQuitConfirmWhenLoopRunning covers the other tryQuit
// branch: when a loop is running, Ctrl+C in picker input mode must open the
// quit-confirmation dialog (not quit immediately). The in-progress input
// value must survive the detour — canceling the confirmation with Esc
// returns the user to the picker with the textinput still populated.
func TestPickerInput_CtrlCOpensQuitConfirmWhenLoopRunning(t *testing.T) {
	picker := NewPRDPicker(t.TempDir(), "", nil)
	picker.StartInputMode()
	picker.ti.SetValue("main")
	app := App{
		picker:      picker,
		manager:     managerWithRunningPRD(t, "current"),
		quitConfirm: NewQuitConfirmation(),
		viewMode:    ViewPicker,
	}

	model, cmd := app.handlePickerKeys(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatal("ctrl+c must not return tea.Quit while a loop is running")
	}
	after := model.(App)
	if after.viewMode != ViewQuitConfirm {
		t.Fatalf("viewMode after ctrl+c: got %v, want ViewQuitConfirm", after.viewMode)
	}
	if !picker.IsInputMode() {
		t.Fatal("picker must remain in input mode across the quit-confirm detour")
	}
	if got, want := picker.ti.Value(), "main"; got != want {
		t.Fatalf("picker value after ctrl+c: got %q, want %q", got, want)
	}

	// Cancel the quit-confirmation with Esc and verify the picker state survives.
	model, _ = after.handleQuitConfirmKeys(tea.KeyMsg{Type: tea.KeyEsc})
	back := model.(App)
	if back.viewMode != ViewPicker {
		t.Fatalf("viewMode after cancel: got %v, want ViewPicker", back.viewMode)
	}
	if !picker.IsInputMode() {
		t.Fatal("picker must still be in input mode after cancel")
	}
	if got, want := picker.ti.Value(), "main"; got != want {
		t.Fatalf("picker value after cancel: got %q, want %q", got, want)
	}
}
