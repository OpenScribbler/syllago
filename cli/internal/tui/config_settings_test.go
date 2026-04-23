package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
