package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// Tests for the first-run telemetry consent modal. The modal is the only
// place a TUI user sees telemetry information before any data could be sent,
// so its content, focus model, and dismissal paths are all part of the
// privacy contract — not just visual polish.
//
// The tests below exercise the modal model directly (not through App) so a
// regression in App routing doesn't mask a regression in the modal itself.
// Mouse tests must run sequentially because bubblezone uses a singleton
// scanner — see .claude/rules/tui-testing.md.

// --- Lifecycle ---

func TestConsentModal_OpenSetsActiveAndSafeFocus(t *testing.T) {
	t.Parallel()
	m := newTelemetryConsentModal()
	if m.Active() {
		t.Fatal("modal must start inactive")
	}
	m.Open()
	if !m.Active() {
		t.Fatal("Open must activate the modal")
	}
	if m.focusIdx != 0 {
		t.Errorf("focusIdx after Open = %d, want 0 (the safe 'No' button)", m.focusIdx)
	}
}

func TestConsentModal_CloseDeactivates(t *testing.T) {
	t.Parallel()
	m := newTelemetryConsentModal()
	m.Open()
	m.Close()
	if m.Active() {
		t.Error("Close must deactivate the modal")
	}
}

func TestConsentModal_UpdateIgnoredWhenInactive(t *testing.T) {
	t.Parallel()
	m := newTelemetryConsentModal()
	// Modal not opened. All input must be ignored — no command, no state change.
	got, cmd := m.Update(keyRune('y'))
	if cmd != nil {
		t.Errorf("inactive modal must not return a command, got %T", cmd())
	}
	if got.Active() {
		t.Error("inactive modal must remain inactive after Update")
	}
}

func TestConsentModal_ViewEmptyWhenInactive(t *testing.T) {
	t.Parallel()
	m := newTelemetryConsentModal()
	if got := m.View(); got != "" {
		t.Errorf("inactive View must return empty string, got %q", got)
	}
}

// --- Keyboard ---

// TestConsentModal_KeyAnswers covers the keyboard answer paths. Each row
// pins a specific key behavior in the privacy contract: y/Y opts in, n/N
// opts out, Esc opts out (the default-safe dismissal), Enter respects the
// current focus. Tab/arrow keys move focus without recording an answer.
func TestConsentModal_KeyAnswers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		startFocus     int
		key            tea.KeyMsg
		wantClosed     bool
		wantCmd        bool // expect a tea.Cmd that emits telemetryConsentDoneMsg
		wantEnabled    bool
		wantFocusAfter int // only checked when wantClosed=false
	}{
		{name: "y_lowercase_opts_in", startFocus: 0, key: keyRune('y'), wantClosed: true, wantCmd: true, wantEnabled: true},
		{name: "Y_uppercase_opts_in", startFocus: 0, key: keyRune('Y'), wantClosed: true, wantCmd: true, wantEnabled: true},
		{name: "n_lowercase_opts_out", startFocus: 0, key: keyRune('n'), wantClosed: true, wantCmd: true, wantEnabled: false},
		{name: "N_uppercase_opts_out", startFocus: 0, key: keyRune('N'), wantClosed: true, wantCmd: true, wantEnabled: false},
		{name: "esc_opts_out", startFocus: 1, key: keyPress(tea.KeyEsc), wantClosed: true, wantCmd: true, wantEnabled: false},
		{name: "enter_on_no_focus_opts_out", startFocus: 0, key: keyPress(tea.KeyEnter), wantClosed: true, wantCmd: true, wantEnabled: false},
		{name: "enter_on_yes_focus_opts_in", startFocus: 1, key: keyPress(tea.KeyEnter), wantClosed: true, wantCmd: true, wantEnabled: true},
		// Focus movement keys must NOT emit a done message.
		{name: "tab_moves_focus_to_yes", startFocus: 0, key: keyTab, wantClosed: false, wantFocusAfter: 1},
		{name: "shift_tab_moves_focus_to_no", startFocus: 1, key: keyShiftTab, wantClosed: false, wantFocusAfter: 0},
		{name: "right_arrow_moves_focus_to_yes", startFocus: 0, key: keyPress(tea.KeyRight), wantClosed: false, wantFocusAfter: 1},
		{name: "left_arrow_moves_focus_to_no", startFocus: 1, key: keyPress(tea.KeyLeft), wantClosed: false, wantFocusAfter: 0},
		{name: "h_vim_moves_focus_to_no", startFocus: 1, key: keyRune('h'), wantClosed: false, wantFocusAfter: 0},
		{name: "l_vim_moves_focus_to_yes", startFocus: 0, key: keyRune('l'), wantClosed: false, wantFocusAfter: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newTelemetryConsentModal()
			m.Open()
			m.focusIdx = tt.startFocus

			got, cmd := m.Update(tt.key)

			if tt.wantClosed {
				if got.Active() {
					t.Errorf("modal must be inactive after a final answer key")
				}
				if !tt.wantCmd {
					if cmd != nil {
						t.Errorf("expected nil command, got non-nil")
					}
					return
				}
				if cmd == nil {
					t.Fatal("expected a command emitting telemetryConsentDoneMsg, got nil")
				}
				msg, ok := cmd().(telemetryConsentDoneMsg)
				if !ok {
					t.Fatalf("command emitted %T, want telemetryConsentDoneMsg", cmd())
				}
				if msg.Enabled != tt.wantEnabled {
					t.Errorf("Enabled=%v, want %v", msg.Enabled, tt.wantEnabled)
				}
				return
			}

			if !got.Active() {
				t.Error("focus-movement key must NOT close the modal")
			}
			if cmd != nil {
				t.Errorf("focus-movement key must not emit a command, got %T", cmd())
			}
			if got.focusIdx != tt.wantFocusAfter {
				t.Errorf("focusIdx=%d, want %d", got.focusIdx, tt.wantFocusAfter)
			}
		})
	}
}

// TestConsentModal_UnboundKeyDoesNothing pins that random keystrokes don't
// accidentally dismiss the modal or change focus. Without this, a stray
// keypress while the user reads the disclosure could record a decision they
// didn't intend.
func TestConsentModal_UnboundKeyDoesNothing(t *testing.T) {
	t.Parallel()
	m := newTelemetryConsentModal()
	m.Open()
	m.focusIdx = 0

	got, cmd := m.Update(keyRune('z'))
	if cmd != nil {
		t.Errorf("unbound key emitted a command: %T", cmd())
	}
	if !got.Active() {
		t.Error("unbound key must not close the modal")
	}
	if got.focusIdx != 0 {
		t.Errorf("unbound key changed focus from 0 to %d", got.focusIdx)
	}
}

// --- View content ---

// TestConsentModal_ViewContainsRequiredDisclosure pins every load-bearing
// piece of text in the modal: the appeal paragraph, the OFF-by-default
// emphasis, both enumerated lists in full, both URLs, and the action hint.
// If any of these go missing the user's consent is no longer fully informed
// — this test catches that before the modal regresses.
func TestConsentModal_ViewContainsRequiredDisclosure(t *testing.T) {
	t.Parallel()
	m := newTelemetryConsentModal()
	m.Open()
	m.SetSize(120, 40)

	view := m.View()

	// Direct substring checks for content that fits on a single line.
	mustContain := []string{
		"syllago needs your help",
		"OFF by default",
		"What gets collected (only if you opt in):",
		"Never collected:",
		telemetry.DocsURL,
		"syllago telemetry on",
		"No, thanks",
		"Yes, share usage data",
	}
	for _, want := range mustContain {
		assertContains(t, view, want)
	}

	// CodeURL exceeds the modal's 70-char inner width, so it wraps onto two
	// lines with leading whitespace. Normalize the view (strip borders, fold
	// whitespace) before checking for the URL — the text must be present, even
	// if rendering breaks it across lines.
	flat := flattenView(view)
	if !strings.Contains(flat, telemetry.CodeURL) {
		t.Errorf("normalized view missing CodeURL %q", telemetry.CodeURL)
	}

	// Every collected/never item must appear verbatim. Wrapping never
	// applies inside the bullet list (each item is one line by design),
	// so this is a strict substring match.
	for _, item := range telemetry.CollectedItems() {
		assertContains(t, view, item)
	}
	for _, item := range telemetry.NeverItems() {
		assertContains(t, view, item)
	}
}

// TestConsentModal_ViewIncludesAppealParagraph pins that the maintainer
// appeal is rendered (possibly wrapped). We collapse all whitespace and
// check for a distinctive substring so wrapping mode doesn't cause flakes.
func TestConsentModal_ViewIncludesAppealParagraph(t *testing.T) {
	t.Parallel()
	m := newTelemetryConsentModal()
	m.Open()
	m.SetSize(120, 40)

	stripped := normalizeWhitespace(m.View())
	// Pull a high-signal phrase out of the middle of MaintainerAppeal so
	// the test doesn't break every time the appeal is reworded — but does
	// break if the paragraph drops out entirely.
	if !strings.Contains(stripped, "one person") {
		t.Errorf("modal view missing maintainer appeal text; got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "OFF by default") {
		t.Errorf("modal view missing OFF-by-default emphasis")
	}
}

// normalizeWhitespace collapses runs of whitespace (including line breaks
// from word-wrap) so substring checks survive layout changes. ANSI is
// stripped via assertContains in callers; here we just normalize.
func normalizeWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// --- Mouse ---

// TestConsentModal_MouseClickNo pins zone "consent-no". The "No, thanks"
// button is the safe default — if its click handler regresses, users who
// reach for the mouse instead of the keyboard would be unable to opt out
// without dismissing via Esc.
func TestConsentModal_MouseClickNo(t *testing.T) {
	// Sequential — bubblezone v1.0.0 uses a singleton scanner; concurrent
	// Scan calls clobber the global zone map. See .claude/rules/tui-testing.md.
	m := newTelemetryConsentModal()
	m.Open()
	m.SetSize(120, 40)

	scanZones(m.View())
	z := zone.Get("consent-no")
	if z.IsZero() {
		t.Skip("zone consent-no not registered (bubblezone rendering issue)")
	}

	got, cmd := m.Update(mouseClick(z.StartX, z.StartY))

	if got.Active() {
		t.Error("modal must close after No click")
	}
	if cmd == nil {
		t.Fatal("No click must emit a command")
	}
	msg, ok := cmd().(telemetryConsentDoneMsg)
	if !ok {
		t.Fatalf("command emitted %T, want telemetryConsentDoneMsg", cmd())
	}
	if msg.Enabled {
		t.Error("No click must emit Enabled=false")
	}
}

// TestConsentModal_MouseClickYes pins zone "consent-yes". The opt-in path
// must require a deliberate click — never a click anywhere on the modal —
// so this test is the lower bound on what counts as consent.
func TestConsentModal_MouseClickYes(t *testing.T) {
	m := newTelemetryConsentModal()
	m.Open()
	m.SetSize(120, 40)

	scanZones(m.View())
	z := zone.Get("consent-yes")
	if z.IsZero() {
		t.Skip("zone consent-yes not registered (bubblezone rendering issue)")
	}

	got, cmd := m.Update(mouseClick(z.StartX, z.StartY))

	if got.Active() {
		t.Error("modal must close after Yes click")
	}
	if cmd == nil {
		t.Fatal("Yes click must emit a command")
	}
	msg, ok := cmd().(telemetryConsentDoneMsg)
	if !ok {
		t.Fatalf("command emitted %T, want telemetryConsentDoneMsg", cmd())
	}
	if !msg.Enabled {
		t.Error("Yes click must emit Enabled=true")
	}
}

// TestConsentModal_MouseClickElsewhereDoesNothing pins that a click on the
// modal body (not on a button) does not record a decision. A naive
// implementation that closes on any click inside the modal-zone would let
// users opt in by accident.
func TestConsentModal_MouseClickElsewhereDoesNothing(t *testing.T) {
	m := newTelemetryConsentModal()
	m.Open()
	m.SetSize(120, 40)

	scanZones(m.View())
	// Click at (0,0) — outside any button zone but still within the modal.
	got, cmd := m.Update(mouseClick(0, 0))

	if !got.Active() {
		t.Error("body click must not close the modal")
	}
	if cmd != nil {
		t.Errorf("body click must not emit a command, got %T", cmd())
	}
}

// TestConsentModal_MouseRightClickIgnored pins that the modal only reacts
// to left-button presses. A right-click context-menu attempt must not
// trigger an answer.
func TestConsentModal_MouseRightClickIgnored(t *testing.T) {
	m := newTelemetryConsentModal()
	m.Open()
	m.SetSize(120, 40)

	scanZones(m.View())
	z := zone.Get("consent-yes")
	if z.IsZero() {
		t.Skip("zone consent-yes not registered")
	}

	rightClick := tea.MouseMsg{X: z.StartX, Y: z.StartY, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	got, cmd := m.Update(rightClick)

	if !got.Active() {
		t.Error("right-click must not close the modal")
	}
	if cmd != nil {
		t.Errorf("right-click must not emit a command, got %T", cmd())
	}
}

// --- App-level integration ---

// TestApp_ConsentDoneMsg_PersistsAndToasts pins the App's reaction to
// telemetryConsentDoneMsg: it must call telemetry.RecordConsent with the
// user's choice and surface a toast confirming the result. Without this,
// even a working modal would drop the user's answer on the floor.
func TestApp_ConsentDoneMsg_PersistsAndToasts(t *testing.T) {
	// Sequential because telemetry.UserHomeDirFn is a global override.
	tmp := t.TempDir()
	orig := telemetry.UserHomeDirFn
	telemetry.UserHomeDirFn = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { telemetry.UserHomeDirFn = orig })

	app := testApp(t)

	// Yes branch.
	m, _ := app.Update(telemetryConsentDoneMsg{Enabled: true})
	a := m.(App)
	if !a.toast.visible {
		t.Fatal("Yes path must surface a toast")
	}
	cur := a.toast.Current()
	if cur == nil {
		t.Fatal("Yes path must produce a current toast")
	}
	if !strings.Contains(cur.message, "Telemetry enabled") {
		t.Errorf("Yes toast text=%q, want substring 'Telemetry enabled'", cur.message)
	}
	cfg := telemetry.Status()
	if !cfg.Enabled || !cfg.ConsentRecorded {
		t.Errorf("after Yes msg: Enabled=%v ConsentRecorded=%v, want both true", cfg.Enabled, cfg.ConsentRecorded)
	}

	// No branch — fresh App so the queued Yes toast doesn't shadow the
	// new one (toastModel.Current() returns the head of the FIFO queue).
	app2 := testApp(t)
	m2, _ := app2.Update(telemetryConsentDoneMsg{Enabled: false})
	a2 := m2.(App)
	cur2 := a2.toast.Current()
	if cur2 == nil {
		t.Fatal("No path must produce a current toast")
	}
	if !strings.Contains(cur2.message, "stays off") {
		t.Errorf("No toast text=%q, want substring 'stays off'", cur2.message)
	}
	cfg = telemetry.Status()
	if cfg.Enabled {
		t.Error("after No msg: Enabled must be false")
	}
	if !cfg.ConsentRecorded {
		t.Error("after No msg: ConsentRecorded must remain true")
	}
}

// flattenView returns the visible text of a rendered modal view collapsed to
// a single line: ANSI/box-drawing borders stripped and runs of whitespace
// folded to a single space. Used when assertions span content that the modal
// wraps across multiple lines (e.g., a URL longer than innerW).
func flattenView(view string) string {
	cleaned := view
	for _, r := range []string{"│", "╭", "╮", "╰", "╯", "─"} {
		cleaned = strings.ReplaceAll(cleaned, r, " ")
	}
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	return strings.Join(strings.Fields(cleaned), "")
}
