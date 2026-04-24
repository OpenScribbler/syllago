package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/doctor"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func newTestSystemModel() systemModel {
	return newSystemModel("", 80, 24)
}

func TestSystemModel_DefaultModeIsDoctor(t *testing.T) {
	m := newTestSystemModel()
	if m.mode != systemModeDoctor {
		t.Errorf("default mode = %v, want systemModeDoctor", m.mode)
	}
}

func TestSystemModel_TabSwitchesToProviders(t *testing.T) {
	m := newTestSystemModel()
	updated, _ := m.Update(keyPress(tea.KeyTab))
	m = updated.(systemModel)
	if m.mode != systemModeProviders {
		t.Errorf("after Tab, mode = %v, want systemModeProviders", m.mode)
	}
}

func TestSystemModel_TabWrapsBackToDoctor(t *testing.T) {
	m := newTestSystemModel()
	// First Tab → providers
	updated, _ := m.Update(keyPress(tea.KeyTab))
	m = updated.(systemModel)
	// Second Tab → back to doctor
	updated, _ = m.Update(keyPress(tea.KeyTab))
	m = updated.(systemModel)
	if m.mode != systemModeDoctor {
		t.Errorf("after two Tabs, mode = %v, want systemModeDoctor", m.mode)
	}
}

func TestSystemModel_RKeyEmitsLoadCmd(t *testing.T) {
	m := newTestSystemModel()
	// Seed some existing checks so we can verify the reload replaces them.
	m.checks = []doctor.CheckResult{{Name: "stale", Status: doctor.CheckOK, Message: "old"}}
	_, cmd := m.Update(keyRune('r'))
	if cmd == nil {
		t.Error("'r' should emit a reload command, got nil")
	}
}

func TestSystemModel_LoadedMsgPopulatesChecks(t *testing.T) {
	m := newTestSystemModel()
	m.loading = true

	checks := []doctor.CheckResult{
		{Name: "library", Status: doctor.CheckOK, Message: "Library: ok"},
		{Name: "config", Status: doctor.CheckWarn, Message: "Config: warn"},
	}
	providers := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
	}

	updated, _ := m.Update(systemLoadedMsg{checks: checks, allProviders: providers})
	m = updated.(systemModel)

	if m.loading {
		t.Error("loading should be false after systemLoadedMsg")
	}
	if len(m.checks) != 2 {
		t.Errorf("checks len = %d, want 2", len(m.checks))
	}
	if len(m.allProviders) != 1 {
		t.Errorf("providers len = %d, want 1", len(m.allProviders))
	}
}

func TestSystemModel_UpDownMovesCursorInProvidersMode(t *testing.T) {
	m := newTestSystemModel()
	m.mode = systemModeProviders
	m.allProviders = []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
		{Name: "Gemini CLI", Slug: "gemini-cli", Detected: true},
		{Name: "Cursor", Slug: "cursor", Detected: false},
	}
	m.selectedProv = 0

	// Down
	updated, _ := m.Update(keyPress(tea.KeyDown))
	m = updated.(systemModel)
	if m.selectedProv != 1 {
		t.Errorf("after Down, selectedProv = %d, want 1", m.selectedProv)
	}

	// j also moves down
	updated, _ = m.Update(keyRune('j'))
	m = updated.(systemModel)
	if m.selectedProv != 2 {
		t.Errorf("after j, selectedProv = %d, want 2", m.selectedProv)
	}

	// Up
	updated, _ = m.Update(keyPress(tea.KeyUp))
	m = updated.(systemModel)
	if m.selectedProv != 1 {
		t.Errorf("after Up, selectedProv = %d, want 1", m.selectedProv)
	}
}

func TestSystemModel_CursorClampsAtBounds(t *testing.T) {
	m := newTestSystemModel()
	m.mode = systemModeProviders
	m.allProviders = []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code"},
	}
	m.selectedProv = 0

	// Up at top — stays at 0
	updated, _ := m.Update(keyPress(tea.KeyUp))
	m = updated.(systemModel)
	if m.selectedProv != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.selectedProv)
	}

	// Down at bottom — stays at 0 (only one provider)
	updated, _ = m.Update(keyPress(tea.KeyDown))
	m = updated.(systemModel)
	if m.selectedProv != 0 {
		t.Errorf("cursor should clamp at last index, got %d", m.selectedProv)
	}
}

func TestSystemModel_ViewRendersWithoutPanic(t *testing.T) {
	m := newTestSystemModel()
	m.checks = []doctor.CheckResult{
		{Name: "library", Status: doctor.CheckOK, Message: "Library: ok (5 items)"},
		{Name: "config", Status: doctor.CheckWarn, Message: "Config: warn", Details: []string{"detail"}},
	}
	// Should not panic
	_ = m.View()
}

func TestSystemModel_SetSize(t *testing.T) {
	m := newTestSystemModel()
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("SetSize(120, 40) → width=%d height=%d", m.width, m.height)
	}
}

func TestSystemModel_Init_ReturnsLoadCmd(t *testing.T) {
	t.Parallel()
	m := newTestSystemModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init")
	}
	// Execute the cmd — it should return a systemLoadedMsg.
	msg := cmd()
	if _, ok := msg.(systemLoadedMsg); !ok {
		t.Errorf("expected systemLoadedMsg, got %T", msg)
	}
}

func TestSystemModel_RenderProviders(t *testing.T) {
	t.Parallel()
	m := newTestSystemModel()
	m.loading = false
	m.mode = systemModeProviders
	m.allProviders = []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
		{Name: "Cursor", Slug: "cursor", Detected: false},
	}
	m.SetSize(80, 20)
	view := m.View()
	if !contains(view, "Claude Code") {
		t.Error("providers view should contain provider name")
	}
	if !contains(view, "detected") {
		t.Error("providers view should show detected status")
	}
	if !contains(view, "not detected") {
		t.Error("providers view should show not detected status")
	}
}

func TestSystemModel_RenderProviders_Loading(t *testing.T) {
	t.Parallel()
	m := newTestSystemModel()
	m.loading = true
	m.mode = systemModeProviders
	m.SetSize(80, 20)
	if !contains(m.View(), "Scanning providers") {
		t.Error("loading view should show Scanning providers message")
	}
}

func TestSystemModel_ShiftTabCyclesBackward(t *testing.T) {
	m := newTestSystemModel()
	// Doctor → Providers (wrap backward)
	updated, _ := m.Update(keyPress(tea.KeyShiftTab))
	m = updated.(systemModel)
	if m.mode != systemModeProviders {
		t.Errorf("Shift+Tab from Doctor should go to Providers (wrap), got %v", m.mode)
	}
	// Providers → Doctor
	updated, _ = m.Update(keyPress(tea.KeyShiftTab))
	m = updated.(systemModel)
	if m.mode != systemModeDoctor {
		t.Errorf("Shift+Tab should go back to Doctor, got %v", m.mode)
	}
}

func TestSystemModel_TabResetsSelectedProv(t *testing.T) {
	m := newTestSystemModel()
	m.selectedProv = 5
	updated, _ := m.Update(keyPress(tea.KeyTab))
	m = updated.(systemModel)
	if m.selectedProv != 0 {
		t.Errorf("Tab should reset selectedProv to 0, got %d", m.selectedProv)
	}
}

func TestSystemModel_RenderDoctor_Loading(t *testing.T) {
	t.Parallel()
	m := newTestSystemModel()
	m.loading = true
	m.mode = systemModeDoctor
	m.SetSize(80, 20)
	if !contains(m.View(), "Running diagnostics") {
		t.Error("loading doctor view should show diagnostics message")
	}
}

func TestSystemModel_RenderDoctor_Empty(t *testing.T) {
	t.Parallel()
	m := newTestSystemModel()
	m.loading = false
	m.checks = nil
	m.mode = systemModeDoctor
	m.SetSize(80, 20)
	if !contains(m.View(), "No checks available") {
		t.Error("empty doctor view should show no-checks message")
	}
}

func TestSystemModel_RenderDoctor_AllStatuses(t *testing.T) {
	t.Parallel()
	m := newTestSystemModel()
	m.loading = false
	m.mode = systemModeDoctor
	m.checks = []doctor.CheckResult{
		{Name: "ok-check", Status: doctor.CheckOK, Message: "all good"},
		{Name: "warn-check", Status: doctor.CheckWarn, Message: "beware", Details: []string{"detail line"}},
		{Name: "err-check", Status: doctor.CheckErr, Message: "broken"},
	}
	m.SetSize(80, 30)
	view := m.View()
	for _, want := range []string{"[ok]", "[warn]", "[err]", "all good", "beware", "broken", "detail line"} {
		if !contains(view, want) {
			t.Errorf("doctor view missing %q in:\n%s", want, view)
		}
	}
}

// --- Mouse tests (sequential — bubblezone is a singleton) ---

func TestSystemModel_MouseClickProvidersTab(t *testing.T) {
	m := newTestSystemModel()
	m.SetSize(80, 20)
	// Render + scan to register zones
	zone.Scan(m.View())
	z := zone.Get("cfg-system-tab-providers")
	if z.IsZero() {
		t.Skip("zone cfg-system-tab-providers not registered")
	}
	updated, _ := m.Update(mouseClick(z.StartX, z.StartY))
	m = updated.(systemModel)
	if m.mode != systemModeProviders {
		t.Errorf("click on providers tab should switch mode, got %v", m.mode)
	}
}

func TestSystemModel_MouseClickDoctorTab(t *testing.T) {
	m := newTestSystemModel()
	m.mode = systemModeProviders
	m.SetSize(80, 20)
	zone.Scan(m.View())
	z := zone.Get("cfg-system-tab-doctor")
	if z.IsZero() {
		t.Skip("zone cfg-system-tab-doctor not registered")
	}
	updated, _ := m.Update(mouseClick(z.StartX, z.StartY))
	m = updated.(systemModel)
	if m.mode != systemModeDoctor {
		t.Errorf("click on doctor tab should switch mode, got %v", m.mode)
	}
}

func TestSystemModel_MouseClickProviderRow(t *testing.T) {
	m := newTestSystemModel()
	m.loading = false
	m.mode = systemModeProviders
	m.allProviders = []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
		{Name: "Cursor", Slug: "cursor", Detected: false},
	}
	m.selectedProv = 0
	m.SetSize(80, 20)
	zone.Scan(m.View())
	z := zone.Get("cfg-system-prov-1")
	if z.IsZero() {
		t.Skip("zone cfg-system-prov-1 not registered")
	}
	updated, _ := m.Update(mouseClick(z.StartX, z.StartY))
	m = updated.(systemModel)
	if m.selectedProv != 1 {
		t.Errorf("click on provider row 1 should select it, got %d", m.selectedProv)
	}
}

func TestSystemModel_MouseClickOutsideZonesIsNoop(t *testing.T) {
	m := newTestSystemModel()
	m.SetSize(80, 20)
	updated, _ := m.Update(mouseClick(500, 500))
	m = updated.(systemModel)
	if m.mode != systemModeDoctor {
		t.Errorf("off-zone click should not change mode, got %v", m.mode)
	}
}

func TestSystemModel_MouseNonLeftClickIgnored(t *testing.T) {
	m := newTestSystemModel()
	// Right-click / mouse motion should be ignored.
	msg := tea.MouseMsg{X: 1, Y: 1, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	updated, _ := m.Update(msg)
	m = updated.(systemModel)
	if m.mode != systemModeDoctor {
		t.Errorf("right-click should not change mode, got %v", m.mode)
	}
}
