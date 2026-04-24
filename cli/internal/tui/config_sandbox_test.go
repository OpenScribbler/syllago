package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

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

func TestAppendUniqueSandbox(t *testing.T) {
	t.Parallel()
	// New item appended
	got := appendUniqueSandbox([]string{"a", "b"}, "c")
	if len(got) != 3 || got[2] != "c" {
		t.Errorf("expected [a b c], got %v", got)
	}
	// Duplicate not appended
	got = appendUniqueSandbox([]string{"a", "b"}, "a")
	if len(got) != 2 {
		t.Errorf("expected unchanged slice, got %v", got)
	}
	// Empty slice
	got = appendUniqueSandbox(nil, "x")
	if len(got) != 1 || got[0] != "x" {
		t.Errorf("expected [x], got %v", got)
	}
}

func TestAppendUniquePortSandbox(t *testing.T) {
	t.Parallel()
	got := appendUniquePortSandbox([]int{80, 443}, 8080)
	if len(got) != 3 || got[2] != 8080 {
		t.Errorf("expected [80 443 8080], got %v", got)
	}
	// Duplicate not appended
	got = appendUniquePortSandbox([]int{80, 443}, 80)
	if len(got) != 2 {
		t.Errorf("expected unchanged slice, got %v", got)
	}
}

func TestSandboxConfigModel_Init_ReturnsCmd(t *testing.T) {
	t.Parallel()
	m := newTestSandboxModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init")
	}
	msg := cmd()
	if _, ok := msg.(sandboxCheckLoadedMsg); !ok {
		t.Errorf("expected sandboxCheckLoadedMsg, got %T", msg)
	}
}

func TestSandboxConfigModel_AddEntryCmd_Domains(t *testing.T) {
	t.Parallel()
	m := newTestSandboxModel()
	m.tab = sandboxTabDomains
	cmd := m.addEntryCmd("new.example.com")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd().(sandboxSavedMsg)
	found := false
	for _, d := range msg.cfg.Sandbox.AllowedDomains {
		if d == "new.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected new domain in AllowedDomains")
	}
}

func TestSandboxConfigModel_AddEntryCmd_Ports(t *testing.T) {
	t.Parallel()
	m := newTestSandboxModel()
	m.tab = sandboxTabPorts
	cmd := m.addEntryCmd("3000")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd().(sandboxSavedMsg)
	found := false
	for _, p := range msg.cfg.Sandbox.AllowedPorts {
		if p == 3000 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected port 3000 in AllowedPorts")
	}
}

func TestSandboxConfigModel_AddEntryCmd_Env(t *testing.T) {
	t.Parallel()
	m := newTestSandboxModel()
	m.tab = sandboxTabEnv
	cmd := m.addEntryCmd("NEW_VAR")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd().(sandboxSavedMsg)
	found := false
	for _, e := range msg.cfg.Sandbox.AllowedEnv {
		if e == "NEW_VAR" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected NEW_VAR in AllowedEnv")
	}
}

// --- Mouse tests (sequential — bubblezone singleton) ---

func TestSandboxConfigModel_MouseTabSwitch(t *testing.T) {
	m := newTestSandboxModel()
	m.SetSize(80, 24)
	scanZones(m.View())
	z := zone.Get(fmt.Sprintf("cfg-sandbox-tab-%d", int(sandboxTabPorts)))
	if z.IsZero() {
		t.Skip("tab zone not registered")
	}
	updated, _ := m.Update(mouseClick(z.StartX, z.StartY))
	m = updated.(sandboxConfigModel)
	if m.tab != sandboxTabPorts {
		t.Errorf("click on ports tab should switch, got %v", m.tab)
	}
}

func TestSandboxConfigModel_MouseItemClickMovesCursor(t *testing.T) {
	m := newTestSandboxModel()
	m.tab = sandboxTabDomains
	m.SetSize(80, 24)
	scanZones(m.View())
	z := zone.Get("cfg-sandbox-item-1")
	if z.IsZero() {
		t.Skip("item zone not registered")
	}
	m.cursor = 0
	updated, _ := m.Update(mouseClick(z.StartX, z.StartY))
	m = updated.(sandboxConfigModel)
	if m.cursor != 1 {
		t.Errorf("click on item 1 should move cursor, got %d", m.cursor)
	}
}

func TestSandboxConfigModel_MouseAddButtonOpensModal(t *testing.T) {
	m := newTestSandboxModel()
	m.SetSize(80, 24)
	scanZones(m.View())
	z := zone.Get("cfg-sandbox-btn-add")
	if z.IsZero() {
		t.Skip("add button zone not registered")
	}
	updated, _ := m.Update(mouseClick(z.StartX, z.StartY))
	m = updated.(sandboxConfigModel)
	if !m.showAddModal {
		t.Error("add button click should open modal")
	}
}

func TestSandboxConfigModel_MouseRemoveButtonEmitsCmd(t *testing.T) {
	m := newTestSandboxModel()
	m.tab = sandboxTabDomains
	m.SetSize(80, 24)
	scanZones(m.View())
	z := zone.Get("cfg-sandbox-btn-remove")
	if z.IsZero() {
		t.Skip("remove button zone not registered")
	}
	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Error("remove button click on non-empty list should emit cmd")
	}
}

func TestSandboxConfigModel_MouseNonLeftIgnored(t *testing.T) {
	m := newTestSandboxModel()
	msg := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	updated, cmd := m.Update(msg)
	m = updated.(sandboxConfigModel)
	if cmd != nil {
		t.Error("right-click should not emit cmd")
	}
	if m.tab != sandboxTabDomains {
		t.Error("right-click should not change tab")
	}
}

// --- Modal input flow tests ---

func TestSandboxConfigModel_ModalBackspaceShrinksInput(t *testing.T) {
	m := newTestSandboxModel()
	m.showAddModal = true
	m.inputValue = "abc"
	updated, _ := m.updateModal(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(sandboxConfigModel)
	if m.inputValue != "ab" {
		t.Errorf("backspace should shrink input, got %q", m.inputValue)
	}
}

func TestSandboxConfigModel_ModalBackspaceOnEmptyIsNoop(t *testing.T) {
	m := newTestSandboxModel()
	m.showAddModal = true
	m.inputValue = ""
	updated, _ := m.updateModal(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(sandboxConfigModel)
	if m.inputValue != "" {
		t.Errorf("backspace on empty should be noop, got %q", m.inputValue)
	}
}

func TestSandboxConfigModel_ModalTypeAppendsInput(t *testing.T) {
	m := newTestSandboxModel()
	m.showAddModal = true
	updated, _ := m.updateModal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(sandboxConfigModel)
	if m.inputValue != "x" {
		t.Errorf("typing should append, got %q", m.inputValue)
	}
}

func TestSandboxConfigModel_ModalEscClosesModal(t *testing.T) {
	m := newTestSandboxModel()
	m.showAddModal = true
	m.inputValue = "pending"
	updated, _ := m.updateModal(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(sandboxConfigModel)
	if m.showAddModal {
		t.Error("Esc should close modal")
	}
	if m.inputValue != "" {
		t.Errorf("Esc should clear input, got %q", m.inputValue)
	}
}

func TestSandboxConfigModel_ModalEnterSubmitsAndClosesModal(t *testing.T) {
	m := newTestSandboxModel()
	m.tab = sandboxTabDomains
	m.showAddModal = true
	m.inputValue = "new.com"
	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(sandboxConfigModel)
	if m.showAddModal {
		t.Error("Enter should close modal")
	}
	if cmd == nil {
		t.Error("Enter should emit save cmd")
	}
}

func TestSandboxConfigModel_ModalEnterEmptyIsNoop(t *testing.T) {
	m := newTestSandboxModel()
	m.showAddModal = true
	m.inputValue = ""
	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(sandboxConfigModel)
	if !m.showAddModal {
		t.Error("Enter with empty input should leave modal open")
	}
	if cmd != nil {
		t.Error("Enter with empty input should not emit cmd")
	}
}

func TestSandboxConfigModel_DeleteSelected_Ports(t *testing.T) {
	t.Parallel()
	m := newTestSandboxModel()
	m.tab = sandboxTabPorts
	m.cursor = 0
	cmd := m.deleteSelected()
	if cmd == nil {
		t.Fatal("deleteSelected on non-empty ports should emit cmd")
	}
	msg := cmd().(sandboxSavedMsg)
	if len(msg.cfg.Sandbox.AllowedPorts) != 1 || msg.cfg.Sandbox.AllowedPorts[0] != 9090 {
		t.Errorf("expected AllowedPorts=[9090], got %v", msg.cfg.Sandbox.AllowedPorts)
	}
}

func TestSandboxConfigModel_DeleteSelected_Env(t *testing.T) {
	t.Parallel()
	m := newTestSandboxModel()
	m.tab = sandboxTabEnv
	m.cursor = 0
	cmd := m.deleteSelected()
	if cmd == nil {
		t.Fatal("deleteSelected on non-empty env should emit cmd")
	}
	msg := cmd().(sandboxSavedMsg)
	if len(msg.cfg.Sandbox.AllowedEnv) != 0 {
		t.Errorf("expected empty AllowedEnv, got %v", msg.cfg.Sandbox.AllowedEnv)
	}
}

func TestSandboxConfigModel_AddEntryCmd_InvalidPortIgnored(t *testing.T) {
	t.Parallel()
	m := newTestSandboxModel()
	m.tab = sandboxTabPorts
	initialLen := len(m.cfg.Sandbox.AllowedPorts)
	cmd := m.addEntryCmd("not-a-port")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd().(sandboxSavedMsg)
	if len(msg.cfg.Sandbox.AllowedPorts) != initialLen {
		t.Errorf("expected port list unchanged, got %v", msg.cfg.Sandbox.AllowedPorts)
	}
}
