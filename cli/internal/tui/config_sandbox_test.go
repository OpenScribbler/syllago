package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/sandbox"
)

func newTestSandboxModel() sandboxConfigModel {
	cfg := &config.Config{
		Sandbox: config.SandboxConfig{
			AllowedDomains: []string{"example.com", "api.example.com"},
			AllowedPorts:   []int{8080, 9090},
			AllowedEnv:     []string{"MY_TOKEN"},
		},
	}
	return newSandboxConfigModel(cfg, 80, 24)
}

func TestSandboxConfigModel_DefaultTabIsDomains(t *testing.T) {
	m := newTestSandboxModel()
	if m.tab != sandboxTabDomains {
		t.Errorf("default tab = %v, want sandboxTabDomains", m.tab)
	}
}

func TestSandboxConfigModel_TabCyclesTabs(t *testing.T) {
	m := newTestSandboxModel()

	// Domains → Ports
	updated, _ := m.Update(keyPress(tea.KeyTab))
	m = updated.(sandboxConfigModel)
	if m.tab != sandboxTabPorts {
		t.Errorf("after Tab, tab = %v, want sandboxTabPorts", m.tab)
	}

	// Ports → Env
	updated, _ = m.Update(keyPress(tea.KeyTab))
	m = updated.(sandboxConfigModel)
	if m.tab != sandboxTabEnv {
		t.Errorf("after Tab×2, tab = %v, want sandboxTabEnv", m.tab)
	}

	// Env → Domains (wrap)
	updated, _ = m.Update(keyPress(tea.KeyTab))
	m = updated.(sandboxConfigModel)
	if m.tab != sandboxTabDomains {
		t.Errorf("after Tab×3, tab = %v, want sandboxTabDomains (wrap)", m.tab)
	}
}

func TestSandboxConfigModel_ShiftTabCyclesBackward(t *testing.T) {
	m := newTestSandboxModel()
	// Domains → Env (backward wrap)
	updated, _ := m.Update(keyPress(tea.KeyShiftTab))
	m = updated.(sandboxConfigModel)
	if m.tab != sandboxTabEnv {
		t.Errorf("Shift+Tab from Domains should wrap to Env, got %v", m.tab)
	}
}

func TestSandboxConfigModel_TabResetsCursor(t *testing.T) {
	m := newTestSandboxModel()
	m.cursor = 1

	updated, _ := m.Update(keyPress(tea.KeyTab))
	m = updated.(sandboxConfigModel)
	if m.cursor != 0 {
		t.Errorf("tab switch should reset cursor to 0, got %d", m.cursor)
	}
}

func TestSandboxConfigModel_AKeyOpensAddModal(t *testing.T) {
	m := newTestSandboxModel()
	updated, _ := m.Update(keyRune('a'))
	m = updated.(sandboxConfigModel)
	if !m.showAddModal {
		t.Error("'a' should open add modal")
	}
}

func TestSandboxConfigModel_EscClosesAddModal(t *testing.T) {
	m := newTestSandboxModel()
	m.showAddModal = true

	updated, _ := m.Update(keyPress(tea.KeyEsc))
	m = updated.(sandboxConfigModel)
	if m.showAddModal {
		t.Error("Esc should close add modal")
	}
}

func TestSandboxConfigModel_UpDownMoveCursor(t *testing.T) {
	m := newTestSandboxModel()
	m.cursor = 0

	// Down
	updated, _ := m.Update(keyPress(tea.KeyDown))
	m = updated.(sandboxConfigModel)
	if m.cursor != 1 {
		t.Errorf("after Down, cursor = %d, want 1", m.cursor)
	}

	// j also moves down
	updated, _ = m.Update(keyRune('j'))
	m = updated.(sandboxConfigModel)
	// Domains has 2 items, so clamped at 1
	if m.cursor > 1 {
		t.Errorf("cursor should clamp at last item, got %d", m.cursor)
	}

	// Up
	updated, _ = m.Update(keyPress(tea.KeyUp))
	m = updated.(sandboxConfigModel)
	if m.cursor != 0 {
		t.Errorf("after Up, cursor = %d, want 0", m.cursor)
	}
}

func TestSandboxConfigModel_CursorClampsAtBounds(t *testing.T) {
	m := newTestSandboxModel()
	m.cursor = 0

	// Up at top — stays at 0
	updated, _ := m.Update(keyPress(tea.KeyUp))
	m = updated.(sandboxConfigModel)
	if m.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.cursor)
	}

	m.cursor = 1 // last item in domains (index 1)
	updated, _ = m.Update(keyPress(tea.KeyDown))
	m = updated.(sandboxConfigModel)
	if m.cursor != 1 {
		t.Errorf("cursor should clamp at last index, got %d", m.cursor)
	}
}

func TestSandboxConfigModel_DKeyRemovesDomainEntry(t *testing.T) {
	m := newTestSandboxModel()
	m.tab = sandboxTabDomains
	m.cursor = 0

	_, cmd := m.Update(keyRune('d'))
	if cmd == nil {
		t.Error("'d' on domains should emit a save cmd, got nil")
	}
}

func TestSandboxConfigModel_DKeyRemovesPortEntry(t *testing.T) {
	m := newTestSandboxModel()
	m.tab = sandboxTabPorts
	m.cursor = 0

	_, cmd := m.Update(keyRune('d'))
	if cmd == nil {
		t.Error("'d' on ports should emit a save cmd, got nil")
	}
}

func TestSandboxConfigModel_DKeyNoopOnEmptyList(t *testing.T) {
	m := newTestSandboxModel()
	m.tab = sandboxTabEnv
	m.cfg = &config.Config{} // no env entries

	_, cmd := m.Update(keyRune('d'))
	if cmd != nil {
		t.Error("'d' on empty list should not emit cmd")
	}
}

func TestSandboxConfigModel_CheckLoadedMsgPopulates(t *testing.T) {
	m := newTestSandboxModel()

	result := sandbox.CheckResult{
		BwrapOK:      true,
		BwrapVersion: "bubblewrap 0.8.0",
		SocatOK:      true,
	}
	updated, _ := m.Update(sandboxCheckLoadedMsg{result: result})
	m = updated.(sandboxConfigModel)

	if !m.checkResult.BwrapOK {
		t.Error("BwrapOK should be true after checkLoadedMsg")
	}
	if m.checkResult.BwrapVersion != "bubblewrap 0.8.0" {
		t.Errorf("BwrapVersion = %q, want bubblewrap 0.8.0", m.checkResult.BwrapVersion)
	}
}

func TestSandboxConfigModel_SavedMsgUpdatesCfg(t *testing.T) {
	m := newTestSandboxModel()
	newCfg := &config.Config{
		Sandbox: config.SandboxConfig{
			AllowedDomains: []string{"newdomain.com"},
		},
	}

	updated, _ := m.Update(sandboxSavedMsg{cfg: newCfg})
	m = updated.(sandboxConfigModel)

	if len(m.cfg.Sandbox.AllowedDomains) != 1 || m.cfg.Sandbox.AllowedDomains[0] != "newdomain.com" {
		t.Errorf("cfg should be updated after sandboxSavedMsg, got %v", m.cfg.Sandbox.AllowedDomains)
	}
}

func TestSandboxConfigModel_ViewRendersWithoutPanic(t *testing.T) {
	m := newTestSandboxModel()
	m.checkResult = sandbox.CheckResult{BwrapOK: true, BwrapVersion: "0.8.0", SocatOK: true}
	for _, tab := range []sandboxTab{sandboxTabDomains, sandboxTabPorts, sandboxTabEnv} {
		m.tab = tab
		_ = m.View()
	}
	// Also render with add modal open
	m.showAddModal = true
	_ = m.View()
}

func TestSandboxConfigModel_SetSize(t *testing.T) {
	m := newTestSandboxModel()
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("SetSize(120, 40) → width=%d height=%d", m.width, m.height)
	}
}
