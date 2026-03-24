package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

func TestAppendUniqueTUI(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		wantLen  int
		wantLast string
	}{
		{"empty slice", nil, "a", 1, "a"},
		{"new item", []string{"a", "b"}, "c", 3, "c"},
		{"duplicate", []string{"a", "b"}, "a", 2, "b"},
		{"duplicate last", []string{"a", "b"}, "b", 2, "b"},
		{"empty string item", []string{}, "", 1, ""},
		{"empty string duplicate", []string{""}, "", 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUniqueTUI(tt.slice, tt.item)
			if len(got) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(got), tt.wantLen)
			}
			if len(got) > 0 && got[len(got)-1] != tt.wantLast {
				t.Errorf("last = %q, want %q", got[len(got)-1], tt.wantLast)
			}
		})
	}
}

func TestAppendUniqueIntTUI(t *testing.T) {
	tests := []struct {
		name     string
		slice    []int
		item     int
		wantLen  int
		wantLast int
	}{
		{"empty slice", nil, 1, 1, 1},
		{"new item", []int{1, 2}, 3, 3, 3},
		{"duplicate", []int{1, 2}, 1, 2, 2},
		{"duplicate last", []int{1, 2}, 2, 2, 2},
		{"zero value", []int{}, 0, 1, 0},
		{"zero duplicate", []int{0}, 0, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUniqueIntTUI(tt.slice, tt.item)
			if len(got) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(got), tt.wantLen)
			}
			if len(got) > 0 && got[len(got)-1] != tt.wantLast {
				t.Errorf("last = %d, want %d", got[len(got)-1], tt.wantLast)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// commitEdit tests
// ---------------------------------------------------------------------------

func TestCommitEdit_Domain(t *testing.T) {
	m := &sandboxSettingsModel{
		sb:        config.SandboxConfig{},
		editMode:  1,
		editInput: "example.com",
	}
	m.commitEdit()
	if len(m.sb.AllowedDomains) != 1 || m.sb.AllowedDomains[0] != "example.com" {
		t.Errorf("expected [example.com], got %v", m.sb.AllowedDomains)
	}
}

func TestCommitEdit_Env(t *testing.T) {
	m := &sandboxSettingsModel{
		sb:        config.SandboxConfig{},
		editMode:  2,
		editInput: "MY_VAR",
	}
	m.commitEdit()
	if len(m.sb.AllowedEnv) != 1 || m.sb.AllowedEnv[0] != "MY_VAR" {
		t.Errorf("expected [MY_VAR], got %v", m.sb.AllowedEnv)
	}
}

func TestCommitEdit_Port(t *testing.T) {
	m := &sandboxSettingsModel{
		sb:        config.SandboxConfig{},
		editMode:  3,
		editInput: "8080",
	}
	m.commitEdit()
	if len(m.sb.AllowedPorts) != 1 || m.sb.AllowedPorts[0] != 8080 {
		t.Errorf("expected [8080], got %v", m.sb.AllowedPorts)
	}
}

func TestCommitEdit_EmptyInput(t *testing.T) {
	m := &sandboxSettingsModel{
		sb:        config.SandboxConfig{},
		editMode:  1,
		editInput: "   ",
	}
	m.commitEdit()
	if len(m.sb.AllowedDomains) != 0 {
		t.Error("empty input should not add domain")
	}
}

func TestCommitEdit_InvalidPort(t *testing.T) {
	m := &sandboxSettingsModel{
		sb:        config.SandboxConfig{},
		editMode:  3,
		editInput: "not-a-number",
	}
	m.commitEdit()
	if len(m.sb.AllowedPorts) != 0 {
		t.Error("invalid port should not be added")
	}
}

// ---------------------------------------------------------------------------
// deleteSelected tests
// ---------------------------------------------------------------------------

func TestDeleteSelected_Domain(t *testing.T) {
	m := &sandboxSettingsModel{
		cursor: sandboxRowDomains,
		sb:     config.SandboxConfig{AllowedDomains: []string{"a.com", "b.com"}},
	}
	m.deleteSelected()
	if len(m.sb.AllowedDomains) != 1 || m.sb.AllowedDomains[0] != "a.com" {
		t.Errorf("expected [a.com], got %v", m.sb.AllowedDomains)
	}
}

func TestDeleteSelected_Env(t *testing.T) {
	m := &sandboxSettingsModel{
		cursor: sandboxRowEnv,
		sb:     config.SandboxConfig{AllowedEnv: []string{"VAR1", "VAR2"}},
	}
	m.deleteSelected()
	if len(m.sb.AllowedEnv) != 1 {
		t.Errorf("expected 1 env var, got %d", len(m.sb.AllowedEnv))
	}
}

func TestDeleteSelected_Ports(t *testing.T) {
	m := &sandboxSettingsModel{
		cursor: sandboxRowPorts,
		sb:     config.SandboxConfig{AllowedPorts: []int{80, 443}},
	}
	m.deleteSelected()
	if len(m.sb.AllowedPorts) != 1 || m.sb.AllowedPorts[0] != 80 {
		t.Errorf("expected [80], got %v", m.sb.AllowedPorts)
	}
}

func TestDeleteSelected_EmptySlice(t *testing.T) {
	m := &sandboxSettingsModel{
		cursor: sandboxRowDomains,
		sb:     config.SandboxConfig{},
	}
	m.deleteSelected() // should not panic
	if len(m.sb.AllowedDomains) != 0 {
		t.Error("deleting from empty slice should be a no-op")
	}
}

// ---------------------------------------------------------------------------
// View tests
// ---------------------------------------------------------------------------

func TestSandboxSettingsView_Empty(t *testing.T) {
	m := sandboxSettingsModel{
		sb:     config.SandboxConfig{},
		cursor: 0,
		width:  60,
		height: 24,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "Sandbox") {
		t.Error("view should contain 'Sandbox' breadcrumb")
	}
	if !strings.Contains(got, "Allowed Domains") {
		t.Error("view should contain 'Allowed Domains'")
	}
	if !strings.Contains(got, "(none)") {
		t.Error("view should show '(none)' for empty lists")
	}
}

func TestSandboxSettingsView_WithData(t *testing.T) {
	m := sandboxSettingsModel{
		sb: config.SandboxConfig{
			AllowedDomains: []string{"example.com", "api.test.io"},
			AllowedEnv:     []string{"HOME", "PATH"},
			AllowedPorts:   []int{443, 8080},
		},
		cursor: 1,
		width:  60,
		height: 24,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "example.com") {
		t.Error("view should show domains")
	}
	if !strings.Contains(got, "443, 8080") {
		t.Error("view should show ports")
	}
}

func TestSandboxSettingsView_EditMode(t *testing.T) {
	m := sandboxSettingsModel{
		sb:        config.SandboxConfig{},
		cursor:    0,
		editMode:  1,
		editInput: "test.com",
		width:     60,
		height:    24,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "Add domain") {
		t.Error("view should show 'Add domain' prompt in edit mode")
	}
	if !strings.Contains(got, "test.com") {
		t.Error("view should show edit input text")
	}
}

// ---------------------------------------------------------------------------
// helpText tests
// ---------------------------------------------------------------------------

func TestSandboxSettingsHelpText(t *testing.T) {
	m := sandboxSettingsModel{editMode: 0}
	got := m.helpText()
	if !strings.Contains(got, "navigate") {
		t.Error("normal helpText should contain 'navigate'")
	}

	m.editMode = 1
	got = m.helpText()
	if !strings.Contains(got, "save") {
		t.Error("edit mode helpText should contain 'save'")
	}
}

// ---------------------------------------------------------------------------
// sandboxListOrNone / sandboxPortsOrNone tests
// ---------------------------------------------------------------------------

func TestSandboxListOrNone(t *testing.T) {
	if sandboxListOrNone(nil) != "(none)" {
		t.Error("nil should return '(none)'")
	}
	if sandboxListOrNone([]string{}) != "(none)" {
		t.Error("empty should return '(none)'")
	}
	got := sandboxListOrNone([]string{"a", "b"})
	if got != "a, b" {
		t.Errorf("got %q, want %q", got, "a, b")
	}
}

func TestSandboxPortsOrNone(t *testing.T) {
	if sandboxPortsOrNone(nil) != "(none)" {
		t.Error("nil should return '(none)'")
	}
	got := sandboxPortsOrNone([]int{80, 443})
	if got != "80, 443" {
		t.Errorf("got %q, want %q", got, "80, 443")
	}
}
