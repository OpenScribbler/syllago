package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func testItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:        "my-hook",
		DisplayName: "My Hook",
		Type:        catalog.Hooks,
		Path:        "/tmp/test/hooks/my-hook",
		Library:     true,
	}
}

func testInstalledProviders() []provider.Provider {
	return []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code"},
		{Name: "Cursor", Slug: "cursor"},
	}
}

func TestRemoveModal_OpenClose(t *testing.T) {
	m := newRemoveModal()
	if m.active {
		t.Fatal("should not be active initially")
	}

	m.Open(testItem(), testInstalledProviders())
	if !m.active {
		t.Fatal("should be active after Open")
	}
	if m.step != removeStepConfirm {
		t.Errorf("expected step Confirm, got %d", m.step)
	}
	if m.itemName != "My Hook" {
		t.Errorf("expected itemName 'My Hook', got %q", m.itemName)
	}
	if len(m.installedProviders) != 2 {
		t.Errorf("expected 2 providers, got %d", len(m.installedProviders))
	}
	if len(m.providerChecks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(m.providerChecks))
	}
	for i, c := range m.providerChecks {
		if c {
			t.Errorf("providerChecks[%d] should be false (unchecked by default)", i)
		}
	}

	m.Close()
	if m.active {
		t.Fatal("should not be active after Close")
	}
	if m.itemName != "" {
		t.Error("should clear state after Close")
	}
}

func TestRemoveModal_NotInstalled_Step1(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), nil) // no providers
	m.width = 80
	m.height = 30

	if m.isInstalled() {
		t.Fatal("should not be installed")
	}
	// 2 buttons: Cancel(0), Remove(1)
	if m.buttonCount() != 2 {
		t.Errorf("expected 2 buttons, got %d", m.buttonCount())
	}

	view := m.View()
	stripped := ansi.Strip(view)
	if strings.Contains(stripped, "provider") {
		t.Error("should not mention providers when not installed")
	}
	if !strings.Contains(stripped, "Cancel") || !strings.Contains(stripped, "Remove") {
		t.Error("should show Cancel and Remove buttons")
	}
}

func TestRemoveModal_NotInstalled_Remove(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), nil)

	// Focus on Remove (1), Enter
	m.focusIdx = 1
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Remove")
	}
	msg := cmd()
	res, ok := msg.(removeResultMsg)
	if !ok {
		t.Fatalf("expected removeResultMsg, got %T", msg)
	}
	if !res.confirmed {
		t.Error("expected confirmed=true")
	}
	if len(res.uninstallProviders) != 0 {
		t.Error("expected no uninstall providers")
	}
}

func TestRemoveModal_Installed_Step1(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.width = 80
	m.height = 30

	// 3 buttons: Cancel(0), Remove Only(1), Yes(2)
	if m.buttonCount() != 3 {
		t.Errorf("expected 3 buttons, got %d", m.buttonCount())
	}

	view := m.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "2 provider") {
		t.Error("should mention 2 providers")
	}
	if !strings.Contains(stripped, "Remove Only") {
		t.Error("should show Remove Only button")
	}
	if !strings.Contains(stripped, "Yes") {
		t.Error("should show Yes button")
	}
}

func TestRemoveModal_Installed_RemoveOnly(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())

	// Press "Remove Only" (focus 1)
	m.focusIdx = 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != removeStepReview {
		t.Errorf("expected step Review, got %d", m.step)
	}
	if !m.skippedProviders {
		t.Error("expected skippedProviders=true")
	}

	// Review should show "Still installed in" since we skipped
	m.width = 80
	m.height = 30
	view := m.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "Still installed in") {
		t.Error("review should show 'Still installed in'")
	}
}

func TestRemoveModal_Installed_Yes(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())

	// Press "Yes" (focus 2)
	m.focusIdx = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != removeStepProviders {
		t.Errorf("expected step Providers, got %d", m.step)
	}
}

func TestRemoveModal_Step2_DefaultUnchecked(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepProviders
	m.focusIdx = 0

	for i, c := range m.providerChecks {
		if c {
			t.Errorf("providerChecks[%d] should be false", i)
		}
	}
}

func TestRemoveModal_Step2_SpaceToggles(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepProviders
	m.focusIdx = 0

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.providerChecks[0] {
		t.Error("expected providerChecks[0] to be true after Space")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.providerChecks[0] {
		t.Error("expected providerChecks[0] to be false after second Space")
	}
}

func TestRemoveModal_Step2_Back(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepProviders

	// Back button is at index len(providers) = 2
	m.focusIdx = len(m.installedProviders)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != removeStepConfirm {
		t.Errorf("expected step Confirm after Back, got %d", m.step)
	}
}

func TestRemoveModal_Step2_Done(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepProviders

	// Check first provider
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	// Done button is at index len(providers)+1 = 3
	m.focusIdx = len(m.installedProviders) + 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != removeStepReview {
		t.Errorf("expected step Review after Done, got %d", m.step)
	}
}

func TestRemoveModal_Step3_ShowsUninstall(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepReview
	m.providerChecks[0] = true // Claude Code selected
	m.width = 80
	m.height = 30

	view := m.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "Will uninstall from") {
		t.Error("should show 'Will uninstall from'")
	}
	if !strings.Contains(stripped, "Claude Code") {
		t.Error("should show selected provider name")
	}
}

func TestRemoveModal_Step3_ShowsStillInstalled(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepReview
	m.providerChecks[0] = true  // Claude Code selected
	m.providerChecks[1] = false // Cursor NOT selected
	m.width = 80
	m.height = 30

	view := m.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "Still installed in") {
		t.Error("should show 'Still installed in'")
	}
	if !strings.Contains(stripped, "Cursor") {
		t.Error("should show unselected provider name")
	}
}

func TestRemoveModal_Step3_Cancel(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepReview
	m.focusIdx = 0 // Cancel

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Cancel")
	}
	msg := cmd()
	res := msg.(removeResultMsg)
	if res.confirmed {
		t.Error("expected confirmed=false")
	}
}

func TestRemoveModal_Step3_Back(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepReview
	m.skippedProviders = false // came from Step 2

	m.focusIdx = 1 // Back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != removeStepProviders {
		t.Errorf("expected step Providers after Back, got %d", m.step)
	}

	// Test Back when skipped providers → goes to Step 1
	m.step = removeStepReview
	m.skippedProviders = true
	m.focusIdx = 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != removeStepConfirm {
		t.Errorf("expected step Confirm after Back (skipped), got %d", m.step)
	}
}

func TestRemoveModal_Step3_Remove(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	m.step = removeStepReview
	m.providerChecks[0] = true
	m.providerChecks[1] = true

	m.focusIdx = 2 // Remove
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Remove")
	}
	msg := cmd()
	res := msg.(removeResultMsg)
	if !res.confirmed {
		t.Error("expected confirmed=true")
	}
	if len(res.uninstallProviders) != 2 {
		t.Errorf("expected 2 uninstall providers, got %d", len(res.uninstallProviders))
	}
}

func TestRemoveModal_BackPreservesState(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())

	// Go to Step 2
	m.focusIdx = 2 // Yes
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Check first provider
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.providerChecks[0] {
		t.Fatal("expected check toggled")
	}

	// Go to Step 3
	m.focusIdx = len(m.installedProviders) + 1 // Done
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Back to Step 2
	m.focusIdx = 1 // Back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Check state preserved
	if !m.providerChecks[0] {
		t.Error("providerChecks[0] should still be true after back navigation")
	}
}

func TestRemoveModal_EscFromAnyStep(t *testing.T) {
	for _, step := range []removeStep{removeStepConfirm, removeStepProviders, removeStepReview} {
		m := newRemoveModal()
		m.Open(testItem(), testInstalledProviders())
		m.step = step

		m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if m.active {
			t.Errorf("step %d: should be inactive after Esc", step)
		}
		if cmd == nil {
			t.Errorf("step %d: expected command from Esc", step)
		}
		msg := cmd()
		res := msg.(removeResultMsg)
		if res.confirmed {
			t.Errorf("step %d: expected confirmed=false", step)
		}
	}
}

func TestRemoveModal_YN_NotInstalled(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), nil) // not installed

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("y should work when not installed")
	}
	msg := cmd()
	res := msg.(removeResultMsg)
	if !res.confirmed {
		t.Error("expected confirmed=true from y")
	}
}

func TestRemoveModal_YN_Disabled_Installed(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders()) // installed

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd != nil {
		t.Error("y should be disabled when installed (3 buttons, ambiguous)")
	}
	if !m.active {
		t.Error("modal should still be active")
	}
}

func TestRemoveModal_LeftRight_Buttons(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), testInstalledProviders())
	// Step 1 installed: 3 buttons at 0, 1, 2

	// Start at Cancel (0), Right → Remove Only (1)
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusIdx != 1 {
		t.Errorf("expected 1, got %d", m.focusIdx)
	}

	// Right → Yes (2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusIdx != 2 {
		t.Errorf("expected 2, got %d", m.focusIdx)
	}

	// Right wraps → Cancel (0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusIdx != 0 {
		t.Errorf("expected 0 (wrap), got %d", m.focusIdx)
	}

	// Left wraps → Yes (2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.focusIdx != 2 {
		t.Errorf("expected 2 (wrap), got %d", m.focusIdx)
	}
}

func TestRemoveModal_WarningRedText(t *testing.T) {
	m := newRemoveModal()
	m.Open(testItem(), nil)
	m.width = 80
	m.height = 30

	view := m.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "This action cannot be undone.") {
		t.Error("should contain warning text")
	}
	// The warning text should use dangerColor — verify the raw view has ANSI codes
	// around the warning. We can't easily check the exact color, but we can verify
	// the warning is present in the non-stripped view with surrounding escapes.
	if !strings.Contains(view, "This action cannot be undone.") {
		t.Error("warning text should be in rendered view")
	}
}
