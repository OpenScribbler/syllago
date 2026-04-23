package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
