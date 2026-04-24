package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

func newTestSettingsModel() settingsModel {
	return newSettingsModel(&config.Config{}, []string{}, "0.9.0", 80, 24)
}

func TestSettingsModel_DefaultPanelIsConfig(t *testing.T) {
	m := newTestSettingsModel()
	if m.panel != settingsPanelConfig {
		t.Errorf("default panel = %v, want settingsPanelConfig", m.panel)
	}
}

func TestSettingsModel_TabCyclesPanels(t *testing.T) {
	m := newTestSettingsModel()

	// Config → Telemetry
	updated, _ := m.Update(keyPress(tea.KeyTab))
	m = updated.(settingsModel)
	if m.panel != settingsPanelTelemetry {
		t.Errorf("after Tab, panel = %v, want settingsPanelTelemetry", m.panel)
	}

	// Telemetry → About
	updated, _ = m.Update(keyPress(tea.KeyTab))
	m = updated.(settingsModel)
	if m.panel != settingsPanelAbout {
		t.Errorf("after Tab×2, panel = %v, want settingsPanelAbout", m.panel)
	}

	// About → Config (wrap)
	updated, _ = m.Update(keyPress(tea.KeyTab))
	m = updated.(settingsModel)
	if m.panel != settingsPanelConfig {
		t.Errorf("after Tab×3, panel = %v, want settingsPanelConfig (wrap)", m.panel)
	}
}

func TestSettingsModel_ShiftTabCyclesBackward(t *testing.T) {
	m := newTestSettingsModel()
	// Config → About (backward wrap)
	updated, _ := m.Update(keyPress(tea.KeyShiftTab))
	m = updated.(settingsModel)
	if m.panel != settingsPanelAbout {
		t.Errorf("Shift+Tab from Config should wrap to About, got %v", m.panel)
	}
}

func TestSettingsModel_TKeyTogglesOnTelemetryPanel(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelTelemetry
	m.telemetryEnabled = true

	updated, cmd := m.Update(keyRune('t'))
	m = updated.(settingsModel)

	if m.telemetryEnabled {
		t.Error("telemetryEnabled should be false after toggle")
	}
	if cmd == nil {
		t.Error("toggle should emit a save cmd, got nil")
	}
}

func TestSettingsModel_TKeyIgnoredOnNonTelemetryPanel(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelConfig
	m.telemetryEnabled = true

	updated, cmd := m.Update(keyRune('t'))
	m = updated.(settingsModel)

	if !m.telemetryEnabled {
		t.Error("telemetryEnabled should not change when panel is Config")
	}
	if cmd != nil {
		t.Error("should not emit cmd when t is pressed on non-telemetry panel")
	}
}

func TestSettingsModel_RKeyResetsIDOnTelemetryPanel(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelTelemetry

	_, cmd := m.Update(keyRune('r'))
	if cmd == nil {
		t.Error("'r' on telemetry panel should emit a reset cmd, got nil")
	}
}

func TestSettingsModel_RKeyIgnoredOnConfigPanel(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelConfig

	_, cmd := m.Update(keyRune('r'))
	if cmd != nil {
		t.Error("'r' on config panel should not emit cmd")
	}
}

func TestSettingsModel_UKeyEmitsUpdateCheckOnAboutPanel(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelAbout

	_, cmd := m.Update(keyRune('u'))
	if cmd == nil {
		t.Error("'u' on about panel should emit an update-check cmd, got nil")
	}
}

func TestSettingsModel_UpdateCheckedMsgPopulatesLatestVersion(t *testing.T) {
	m := newTestSettingsModel()
	m.checkingUpdate = true

	updated, _ := m.Update(settingsUpdateCheckedMsg{latestVersion: "1.0.0", isNewer: true})
	m = updated.(settingsModel)

	if m.checkingUpdate {
		t.Error("checkingUpdate should be false after result")
	}
	if m.latestVersion != "1.0.0" {
		t.Errorf("latestVersion = %q, want 1.0.0", m.latestVersion)
	}
	if !m.updateAvail {
		t.Error("updateAvail should be true when isNewer=true")
	}
}

func TestSettingsModel_TelemetryStatusMsgPopulates(t *testing.T) {
	m := newTestSettingsModel()
	updated, _ := m.Update(settingsTelemetryStatusMsg{enabled: false, anonID: "syl_abc123"})
	m = updated.(settingsModel)

	if m.telemetryEnabled {
		t.Error("telemetryEnabled should be false")
	}
	if m.telemetryAnonID != "syl_abc123" {
		t.Errorf("anonID = %q, want syl_abc123", m.telemetryAnonID)
	}
}

func TestSettingsModel_ViewRendersWithoutPanic(t *testing.T) {
	m := newTestSettingsModel()
	m.telemetryEnabled = true
	m.telemetryAnonID = "syl_test123"
	m.latestVersion = "1.0.0"
	// Should not panic at any panel
	for _, panel := range []settingsPanel{settingsPanelConfig, settingsPanelTelemetry, settingsPanelAbout} {
		m.panel = panel
		_ = m.View()
	}
}

func TestSettingsModel_SetSize(t *testing.T) {
	m := newTestSettingsModel()
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("SetSize(120, 40) → width=%d height=%d", m.width, m.height)
	}
}

func TestSettingsModel_Init_ReturnsTelemetryLoadCmd(t *testing.T) {
	t.Parallel()
	m := newTestSettingsModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init")
	}
	msg := cmd()
	if _, ok := msg.(settingsTelemetryStatusMsg); !ok {
		t.Errorf("expected settingsTelemetryStatusMsg, got %T", msg)
	}
}

func TestSettingsModel_MouseClickTelemetryTab(t *testing.T) {
	m := newTestSettingsModel()
	scanZones(m.View())
	z := zone.Get("cfg-settings-tab-1")
	if z.IsZero() {
		t.Fatal("zone cfg-settings-tab-1 not registered")
	}
	updated, _ := m.Update(mouseClick(z.StartX+1, z.StartY))
	m = updated.(settingsModel)
	if m.panel != settingsPanelTelemetry {
		t.Errorf("click on telemetry tab → panel=%v, want settingsPanelTelemetry", m.panel)
	}
}

func TestSettingsModel_MouseClickAboutTab(t *testing.T) {
	m := newTestSettingsModel()
	scanZones(m.View())
	z := zone.Get("cfg-settings-tab-2")
	if z.IsZero() {
		t.Fatal("zone cfg-settings-tab-2 not registered")
	}
	updated, _ := m.Update(mouseClick(z.StartX+1, z.StartY))
	m = updated.(settingsModel)
	if m.panel != settingsPanelAbout {
		t.Errorf("click on about tab → panel=%v, want settingsPanelAbout", m.panel)
	}
}

func TestSettingsModel_MouseClickTelemetryToggle(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelTelemetry
	m.telemetryEnabled = true
	scanZones(m.View())
	z := zone.Get("cfg-settings-telemetry-toggle")
	if z.IsZero() {
		t.Fatal("zone cfg-settings-telemetry-toggle not registered")
	}
	updated, cmd := m.Update(mouseClick(z.StartX+1, z.StartY))
	m = updated.(settingsModel)
	if m.telemetryEnabled {
		t.Error("telemetryEnabled should flip to false")
	}
	if cmd == nil {
		t.Error("expected save cmd from toggle click")
	}
}

func TestSettingsModel_MouseClickTelemetryReset(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelTelemetry
	scanZones(m.View())
	z := zone.Get("cfg-settings-telemetry-reset")
	if z.IsZero() {
		t.Fatal("zone cfg-settings-telemetry-reset not registered")
	}
	_, cmd := m.Update(mouseClick(z.StartX+1, z.StartY))
	if cmd == nil {
		t.Error("expected reset cmd from reset-id click")
	}
}

func TestSettingsModel_MouseClickUpdateCheck(t *testing.T) {
	m := newTestSettingsModel()
	m.panel = settingsPanelAbout
	scanZones(m.View())
	z := zone.Get("cfg-settings-update-check")
	if z.IsZero() {
		t.Fatal("zone cfg-settings-update-check not registered")
	}
	updated, cmd := m.Update(mouseClick(z.StartX+1, z.StartY))
	m = updated.(settingsModel)
	if !m.checkingUpdate {
		t.Error("checkingUpdate should be true after click")
	}
	if cmd == nil {
		t.Error("expected update-check cmd")
	}
}

func TestSettingsModel_MouseNonLeftIgnored(t *testing.T) {
	m := newTestSettingsModel()
	scanZones(m.View())
	nonLeft := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	updated, cmd := m.Update(nonLeft)
	m = updated.(settingsModel)
	if m.panel != settingsPanelConfig {
		t.Error("non-left click should not change panel")
	}
	if cmd != nil {
		t.Error("non-left click should not emit cmd")
	}
}

func TestSettingsModel_MouseClickOutsideZonesIsNoop(t *testing.T) {
	m := newTestSettingsModel()
	scanZones(m.View())
	updated, cmd := m.Update(mouseClick(999, 999))
	m = updated.(settingsModel)
	if m.panel != settingsPanelConfig {
		t.Error("click outside zones should not change panel")
	}
	if cmd != nil {
		t.Error("click outside zones should not emit cmd")
	}
}

func TestSettingsModel_SaveTelemetryCmd_ReturnsStatusMsg(t *testing.T) {
	t.Parallel()
	m := newTestSettingsModel()
	cmd := m.saveTelemetryCmd(true)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(settingsTelemetryStatusMsg); !ok {
		t.Errorf("expected settingsTelemetryStatusMsg, got %T", msg)
	}
}

func TestSettingsModel_ResetAnonIDCmd_ReturnsStatusMsg(t *testing.T) {
	t.Parallel()
	m := newTestSettingsModel()
	cmd := m.resetAnonIDCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(settingsTelemetryStatusMsg); !ok {
		t.Errorf("expected settingsTelemetryStatusMsg, got %T", msg)
	}
}

func TestSettingsModel_CheckUpdateCmd_ReturnsUpdateMsg(t *testing.T) {
	t.Parallel()
	m := newTestSettingsModel()
	cmd := m.checkUpdateCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(settingsUpdateCheckedMsg); !ok {
		t.Errorf("expected settingsUpdateCheckedMsg, got %T", msg)
	}
}
