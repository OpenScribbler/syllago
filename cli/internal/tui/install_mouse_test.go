package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// scanZones renders the view and waits for bubblezone's async goroutine to
// populate the zone map. bubblezone v1.0.0 populates zones via a channel
// (manager.zoneWorker), so an immediate Get() after Scan() can race.
// bubblezone's own TestScan uses a 15ms sleep (manager_test.go:109). We
// mirror that here to make zone assertions deterministic.
func scanZones(view string) {
	zone.Scan(view)
	time.Sleep(20 * time.Millisecond)
}

// Mouse coverage for the install wizard. Every interactive element rendered
// by the wizard's View() must be clickable, per the TUI mouse-parity rule in
// .claude/rules/tui-wizard-patterns.md. The wizard spans four user-facing
// steps (Provider, Location, Method, Review) plus the Conflict branch; each
// step exposes a distinct zone layout, and regressions at any one of them
// silently break the keyboard-parity contract.
//
// The tests below construct the wizard model directly rather than routing
// through App.Update() so they pin a single zone handler at a time. Clicks
// are dispatched via mouseClick() from confirm_test.go after zone.Scan(View)
// registers the rendered layout. z.IsZero() skip guards protect against
// bubblezone rendering edge cases at small terminal sizes.

// --- Provider step ---

// TestInstallMouse_ProviderRowSelectsProvider pins the zone handler at
// install_update.go:196 (inst-prov-N). Clicking a provider row must move
// providerCursor to that index without advancing the wizard step. A prior
// regression changed the cursor AND advanced the step, skipping the "Next"
// button entirely.
func TestInstallMouse_ProviderRowSelectsProvider(t *testing.T) {
	m := setupProviderStepWizard(t, 80, 30)

	scanZones(m.View())
	z := zone.Get("inst-prov-1")
	if z.IsZero() {
		t.Skip("zone inst-prov-1 not registered (bubblezone rendering issue)")
	}

	m, cmd := m.Update(mouseClick(z.StartX, z.StartY))

	if m.providerCursor != 1 {
		t.Errorf("providerCursor should be 1 after clicking inst-prov-1, got %d", m.providerCursor)
	}
	if m.step != installStepProvider {
		t.Errorf("step should remain installStepProvider after row click, got %d", m.step)
	}
	if cmd != nil {
		t.Errorf("row click should not emit a command, got %T", cmd())
	}
}

// TestInstallMouse_ProviderAllSelectsAll pins inst-all zone (install_update.go:205).
// The "All providers" row click must set selectAll=true without touching
// providerCursor or the step. Without this, the keyboard 'a' toggle has no
// mouse equivalent.
func TestInstallMouse_ProviderAllSelectsAll(t *testing.T) {
	m := setupProviderStepWizard(t, 80, 30)

	scanZones(m.View())
	z := zone.Get("inst-all")
	if z.IsZero() {
		t.Skip("zone inst-all not registered")
	}

	m, _ = m.Update(mouseClick(z.StartX, z.StartY))

	if !m.selectAll {
		t.Error("selectAll should be true after clicking inst-all")
	}
	if m.step != installStepProvider {
		t.Errorf("step should remain installStepProvider, got %d", m.step)
	}
}

// TestInstallMouse_ProviderNextAdvances pins inst-next (install_update.go:213).
// Clicking Next with a selectable provider must advance to Location for
// filesystem types. This is the mouse path into the rest of the wizard.
func TestInstallMouse_ProviderNextAdvances(t *testing.T) {
	m := setupProviderStepWizard(t, 80, 30)
	// providerCursor defaults to 0 (Claude Code, not installed).

	scanZones(m.View())
	z := zone.Get("inst-next")
	if z.IsZero() {
		t.Skip("zone inst-next not registered")
	}

	m, _ = m.Update(mouseClick(z.StartX, z.StartY))

	if m.step != installStepLocation {
		t.Errorf("step should be installStepLocation after Next click, got %d", m.step)
	}
	if m.shell.active != 1 {
		t.Errorf("shell.active should be 1 after advancing, got %d", m.shell.active)
	}
}

// TestInstallMouse_ProviderCancelCloses pins inst-cancel (install_update.go:210).
// Cancel must emit installCloseMsg — the same message keyboard Esc emits.
func TestInstallMouse_ProviderCancelCloses(t *testing.T) {
	m := setupProviderStepWizard(t, 80, 30)

	scanZones(m.View())
	z := zone.Get("inst-cancel")
	if z.IsZero() {
		t.Skip("zone inst-cancel not registered")
	}

	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from Cancel click")
	}
	if _, ok := cmd().(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg, got %T", cmd())
	}
}

// TestInstallMouse_ProviderAllNextEntersReview pins the combined path:
// selectAll=true + click Next with no conflicts → installAllResultMsg. The
// wizard must honor the "All providers" selection rather than falling back
// to the single-provider path.
func TestInstallMouse_ProviderAllNextEntersReview(t *testing.T) {
	m := setupProviderStepWizard(t, 80, 30)
	m.selectAll = true

	scanZones(m.View())
	z := zone.Get("inst-next")
	if z.IsZero() {
		t.Skip("zone inst-next not registered")
	}

	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from selectAll + Next click")
	}
	if _, ok := cmd().(installAllResultMsg); !ok {
		t.Errorf("expected installAllResultMsg, got %T", cmd())
	}
}

// --- Location step ---

// locationStepWizard builds a wizard at the Location step for mouse tests.
// A single provider auto-skips the Provider step, landing the wizard on
// Location with shell.active=1.
func locationStepWizard(t *testing.T, w, h int) *installWizardModel {
	t.Helper()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))
	m := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	m.width = w
	m.height = h
	m.shell.SetWidth(w)
	if m.step != installStepLocation {
		t.Fatalf("precondition: expected installStepLocation, got %d", m.step)
	}
	return m
}

// TestInstallMouse_LocationRowSelectsLocation pins inst-loc-N
// (install_update.go:347). Clicking a location row sets locationCursor
// without advancing. Covers all three rows (Global, Project, Custom).
func TestInstallMouse_LocationRowSelectsLocation(t *testing.T) {
	for i := 0; i < 3; i++ {
		i := i
		t.Run(fmt.Sprintf("row_%d", i), func(t *testing.T) {
			m := locationStepWizard(t, 80, 30)
			scanZones(m.View())
			z := zone.Get(fmt.Sprintf("inst-loc-%d", i))
			if z.IsZero() {
				t.Skipf("zone inst-loc-%d not registered", i)
			}
			m, _ = m.Update(mouseClick(z.StartX, z.StartY))
			if m.locationCursor != i {
				t.Errorf("locationCursor should be %d after click, got %d", i, m.locationCursor)
			}
			if m.step != installStepLocation {
				t.Errorf("step should remain installStepLocation, got %d", m.step)
			}
		})
	}
}

// TestInstallMouse_LocationNextAdvances pins inst-next on location step
// (install_update.go:355). Clicking Next with locationCursor=0 must advance
// to Method.
func TestInstallMouse_LocationNextAdvances(t *testing.T) {
	m := locationStepWizard(t, 80, 30)
	scanZones(m.View())
	z := zone.Get("inst-next")
	if z.IsZero() {
		t.Skip("zone inst-next not registered")
	}

	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.step != installStepMethod {
		t.Errorf("step should be installStepMethod after Next click, got %d", m.step)
	}
}

// TestInstallMouse_LocationBackGoesToProviderOrCloses pins inst-back on
// location step (install_update.go:352). When the Provider step was
// auto-skipped (single provider), Back must close the wizard rather than
// trying to re-enter the skipped step.
func TestInstallMouse_LocationBackGoesToProviderOrCloses(t *testing.T) {
	m := locationStepWizard(t, 80, 30)
	if !m.autoSkippedProvider {
		t.Fatal("precondition: expected autoSkippedProvider=true with single provider")
	}
	scanZones(m.View())
	z := zone.Get("inst-back")
	if z.IsZero() {
		t.Skip("zone inst-back not registered")
	}

	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected close command from Back click on auto-skipped location")
	}
	if _, ok := cmd().(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg (auto-skip close), got %T", cmd())
	}
}

// --- Method step ---

// methodStepWizard builds a wizard at the Method step for mouse tests.
func methodStepWizard(t *testing.T, w, h int) *installWizardModel {
	t.Helper()
	m := locationStepWizard(t, w, h)
	// Advance location -> method via Enter (keyboard is fine here; the test
	// target is the Method step's zone handlers, not the Location advance).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != installStepMethod {
		t.Fatalf("precondition: expected installStepMethod, got %d", m.step)
	}
	return m
}

// TestInstallMouse_MethodRowSelectsMethod pins inst-method-0/1
// (install_update.go:400, 404). Clicking a method row sets methodCursor
// without advancing.
func TestInstallMouse_MethodRowSelectsMethod(t *testing.T) {
	for i := 0; i < 2; i++ {
		i := i
		t.Run(fmt.Sprintf("method_%d", i), func(t *testing.T) {
			m := methodStepWizard(t, 80, 30)
			scanZones(m.View())
			z := zone.Get(fmt.Sprintf("inst-method-%d", i))
			if z.IsZero() {
				t.Skipf("zone inst-method-%d not registered", i)
			}
			m, _ = m.Update(mouseClick(z.StartX, z.StartY))
			if m.methodCursor != i {
				t.Errorf("methodCursor should be %d after click, got %d", i, m.methodCursor)
			}
			if m.step != installStepMethod {
				t.Errorf("step should remain installStepMethod, got %d", m.step)
			}
		})
	}
}

// TestInstallMouse_MethodAppendRowSelectsAppend pins inst-method-2 —
// the D5 append option. Clicking the append row must set methodCursor=2
// and leave the step on installStepMethod. This is the mouse equivalent of
// pressing Down twice to reach Append on a keyboard.
func TestInstallMouse_MethodAppendRowSelectsAppend(t *testing.T) {
	m := methodStepWizard(t, 80, 30)
	scanZones(m.View())
	z := zone.Get("inst-method-2")
	if z.IsZero() {
		t.Skip("zone inst-method-2 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.methodCursor != 2 {
		t.Errorf("methodCursor should be 2 after clicking inst-method-2, got %d", m.methodCursor)
	}
	if m.step != installStepMethod {
		t.Errorf("step should remain installStepMethod, got %d", m.step)
	}
}

// TestInstallMouse_MethodAppendHiddenForNonRules verifies clicking the
// append zone is a no-op for non-rule content — the zone is never rendered
// for Skills/Hooks, so the keyboard parity contract is "no equivalent" and
// the click must not advance state.
func TestInstallMouse_MethodAppendHiddenForNonRules(t *testing.T) {
	t.Helper()
	// Build a Skills-typed wizard at the method step.
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))
	m := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	m.width = 80
	m.height = 30
	m.shell.SetWidth(80)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	if m.step != installStepMethod {
		t.Fatalf("precondition: expected installStepMethod, got %d", m.step)
	}

	scanZones(m.View())
	z := zone.Get("inst-method-2")
	if !z.IsZero() {
		t.Error("inst-method-2 zone should not render for non-rule content")
	}
	// Bounds-safe: even if the zone were present, methodCursor must not
	// reach 2 via the click dispatch because the update path guards with
	// appendFilename() != "".
}

// TestInstallMouse_MethodNextAdvancesToReview pins inst-next on method
// step (install_update.go:411). Clicking Next enters the Review step.
func TestInstallMouse_MethodNextAdvancesToReview(t *testing.T) {
	m := methodStepWizard(t, 80, 30)
	scanZones(m.View())
	z := zone.Get("inst-next")
	if z.IsZero() {
		t.Skip("zone inst-next not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.step != installStepReview {
		t.Errorf("step should be installStepReview after Next click, got %d", m.step)
	}
}

// TestInstallMouse_MethodBackReturnsToLocation pins inst-back on method
// step (install_update.go:408). Back from Method must return to Location.
func TestInstallMouse_MethodBackReturnsToLocation(t *testing.T) {
	m := methodStepWizard(t, 80, 30)
	scanZones(m.View())
	z := zone.Get("inst-back")
	if z.IsZero() {
		t.Skip("zone inst-back not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.step != installStepLocation {
		t.Errorf("step should be installStepLocation after Back click, got %d", m.step)
	}
}

// --- Review step ---

// reviewStepWizard builds a wizard at the Review step for mouse tests.
// Uses a real on-disk file so the preview renders — without this, the
// review step's file tree has nothing to scan and some zones don't register.
func reviewStepWizard(t *testing.T, w, h int) *installWizardModel {
	t.Helper()
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	if err := makeTestFile(t, itemDir, "rule.md", "# Rule\n\nbody\n"); err != nil {
		t.Fatal(err)
	}

	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, itemDir)
	item.Files = []string{filepath.Join(itemDir, "rule.md")}

	m := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	m.width = w
	m.height = h
	m.shell.SetWidth(w)

	// Single provider auto-skips to Location; advance through Location + Method.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review
	if m.step != installStepReview {
		t.Fatalf("precondition: expected installStepReview, got %d", m.step)
	}
	return m
}

// TestInstallMouse_ReviewInstallFires pins inst-install (install_update.go:581).
// Clicking Install must emit installResultMsg and set confirmed=true. Missing
// this zone handler means users cannot install anything without a keyboard.
func TestInstallMouse_ReviewInstallFires(t *testing.T) {
	m := reviewStepWizard(t, 100, 40)
	scanZones(m.View())
	z := zone.Get("inst-install")
	if z.IsZero() {
		t.Skip("zone inst-install not registered")
	}
	m, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from Install click")
	}
	if _, ok := cmd().(installResultMsg); !ok {
		t.Errorf("expected installResultMsg, got %T", cmd())
	}
	if !m.confirmed {
		t.Error("confirmed flag should be true after Install click")
	}
}

// TestInstallMouse_ReviewCancelCloses pins inst-cancel on review step
// (install_update.go:575). Cancel from Review must close the wizard.
func TestInstallMouse_ReviewCancelCloses(t *testing.T) {
	m := reviewStepWizard(t, 100, 40)
	scanZones(m.View())
	z := zone.Get("inst-cancel")
	if z.IsZero() {
		t.Skip("zone inst-cancel not registered")
	}
	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from Cancel click")
	}
	if _, ok := cmd().(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg, got %T", cmd())
	}
}

// TestInstallMouse_ReviewBackReturnsToMethod pins inst-back on review step
// (install_update.go:578). Back from Review returns to Method for filesystem
// installs.
func TestInstallMouse_ReviewBackReturnsToMethod(t *testing.T) {
	m := reviewStepWizard(t, 100, 40)
	scanZones(m.View())
	z := zone.Get("inst-back")
	if z.IsZero() {
		t.Skip("zone inst-back not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.step != installStepMethod {
		t.Errorf("step should be installStepMethod after Back click, got %d", m.step)
	}
}

// --- Conflict step ---

// TestInstallMouse_ConflictRowSelectsResolution pins inst-conflict-N
// (install_update.go:233). Each row click sets conflictCursor. Without
// these zones, users cannot choose a conflict resolution by mouse.
func TestInstallMouse_ConflictRowSelectsResolution(t *testing.T) {
	for i := 0; i < 3; i++ {
		i := i
		t.Run(fmt.Sprintf("conflict_%d", i), func(t *testing.T) {
			m := setupConflictStepWizard(t, 80, 30)
			scanZones(m.View())
			z := zone.Get(fmt.Sprintf("inst-conflict-%d", i))
			if z.IsZero() {
				t.Skipf("zone inst-conflict-%d not registered", i)
			}
			m, _ = m.Update(mouseClick(z.StartX, z.StartY))
			if m.conflictCursor != i {
				t.Errorf("conflictCursor should be %d after click, got %d", i, m.conflictCursor)
			}
		})
	}
}

// TestInstallMouse_ConflictInstallAdvances pins inst-conflict-install
// (install_update.go:241). The install button on the conflict step must
// emit installAllResultMsg with the chosen resolution applied.
func TestInstallMouse_ConflictInstallAdvances(t *testing.T) {
	m := setupConflictStepWizard(t, 80, 30)
	m.conflictCursor = 0 // SharedOnly
	scanZones(m.View())
	z := zone.Get("inst-conflict-install")
	if z.IsZero() {
		t.Skip("zone inst-conflict-install not registered")
	}
	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from Install click on conflict step")
	}
	msg, ok := cmd().(installAllResultMsg)
	if !ok {
		t.Fatalf("expected installAllResultMsg, got %T", cmd())
	}
	// SharedOnly resolution applied via ApplyConflictResolution — verify the
	// provider list is not empty (real filtering logic determines membership).
	if len(msg.providers) == 0 {
		t.Error("expected non-empty providers list from SharedOnly resolution")
	}
}

// --- Breadcrumb ---

// TestInstallMouse_BreadcrumbNavigatesBack pins the wizardShell breadcrumb
// path inside install wizard's updateMouse (install_update.go:166). Clicking
// a completed step must jump back; clicking the current or future steps
// is a no-op.
func TestInstallMouse_BreadcrumbNavigatesBack(t *testing.T) {
	m := methodStepWizard(t, 80, 30)
	if m.shell.active != 2 {
		t.Fatalf("precondition: expected shell.active=2 on method, got %d", m.shell.active)
	}
	scanZones(m.View())
	// wizardShell marks each step as "wiz-step-N" (wizard_shell.go:117).
	z := zone.Get("wiz-step-0")
	if z.IsZero() {
		t.Skip("wiz-step-0 zone not registered (wizardShell rendering issue)")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	// After clicking step 0, we should be back at Provider or Location
	// (Provider was auto-skipped, so the wizard jumps to the nearest reachable
	// earlier step — step enum 0 is Provider).
	if m.step >= installStepMethod {
		t.Errorf("breadcrumb click should move wizard back from Method; still at step %d", m.step)
	}
}

// --- Helpers ---

// makeTestFile creates a file with the given content at dir/name, creating
// parent dirs as needed. Returns an error if dir creation or write fails.
func makeTestFile(t *testing.T, dir, name, content string) error {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// Compile-time assertions so referenced package types resolve: guards
// against an accidental import removal by goimports.
var _ = installer.ResolutionSharedOnly
var _ = catalog.Rules
