package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestCreateLoadoutModalSteps(t *testing.T) {
	cat := &catalog.Catalog{}
	providers := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
	}

	t.Run("starts at provider step when no prefill", func(t *testing.T) {
		m := newCreateLoadoutModal("", "", providers, cat)
		if m.step != clStepProvider {
			t.Errorf("step = %v, want clStepProvider", m.step)
		}
		if m.totalSteps != 4 {
			t.Errorf("totalSteps = %d, want 4", m.totalSteps)
		}
	})

	t.Run("skips provider step when pre-filled", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		if m.step != clStepItems {
			t.Errorf("step = %v, want clStepItems", m.step)
		}
		if m.totalSteps != 3 {
			t.Errorf("totalSteps = %d, want 3", m.totalSteps)
		}
	})

	t.Run("dest constraint: registry disabled when items span registries", func(t *testing.T) {
		m := newCreateLoadoutModal("", "reg-a", providers, cat)
		m.entries = []loadoutItemEntry{
			{item: catalog.ContentItem{Name: "a", Registry: "reg-a"}, selected: true},
			{item: catalog.ContentItem{Name: "b", Registry: "reg-b"}, selected: true},
		}
		// scopeRegistry adds a third dest option; verify it exists
		if len(m.destOptions) < 3 {
			t.Fatal("expected 3 dest options with scopeRegistry set")
		}
		m.updateDestConstraints()
		if !m.destDisabled[2] {
			t.Error("registry dest should be disabled when items span registries")
		}
	})

	t.Run("dest constraint: registry disabled when items span providers", func(t *testing.T) {
		m := newCreateLoadoutModal("", "reg-a", providers, cat)
		m.entries = []loadoutItemEntry{
			{item: catalog.ContentItem{Name: "a", Registry: "reg-a", Provider: "claude-code"}, selected: true},
			{item: catalog.ContentItem{Name: "b", Registry: "reg-a", Provider: "cursor"}, selected: true},
		}
		m.updateDestConstraints()
		if !m.destDisabled[2] {
			t.Error("registry dest should be disabled when items span providers")
		}
	})

	t.Run("dest constraint: registry enabled when items are same registry", func(t *testing.T) {
		m := newCreateLoadoutModal("", "reg-a", providers, cat)
		m.entries = []loadoutItemEntry{
			{item: catalog.ContentItem{Name: "a", Registry: "reg-a", Provider: "claude-code"}, selected: true},
			{item: catalog.ContentItem{Name: "b", Registry: "reg-a", Provider: "claude-code"}, selected: true},
		}
		m.updateDestConstraints()
		if m.destDisabled[2] {
			t.Error("registry dest should be enabled when items are same registry+provider")
		}
	})

	t.Run("currentStepNum reflects prefilled offset", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		if m.currentStepNum() != 1 {
			t.Errorf("currentStepNum() = %d, want 1 for items step with prefill", m.currentStepNum())
		}
		m.step = clStepName
		if m.currentStepNum() != 2 {
			t.Errorf("currentStepNum() = %d, want 2 for name step with prefill", m.currentStepNum())
		}
		m.step = clStepDest
		if m.currentStepNum() != 3 {
			t.Errorf("currentStepNum() = %d, want 3 for dest step with prefill", m.currentStepNum())
		}
	})

	t.Run("filteredEntries returns all when no search", func(t *testing.T) {
		m := newCreateLoadoutModal("", "", providers, cat)
		m.entries = []loadoutItemEntry{
			{item: catalog.ContentItem{Name: "rule-a"}},
			{item: catalog.ContentItem{Name: "rule-b"}},
		}
		if len(m.filteredEntries()) != 2 {
			t.Errorf("filteredEntries() = %d, want 2", len(m.filteredEntries()))
		}
	})

	t.Run("filteredEntries filters by search", func(t *testing.T) {
		m := newCreateLoadoutModal("", "", providers, cat)
		m.entries = []loadoutItemEntry{
			{item: catalog.ContentItem{Name: "rule-alpha"}},
			{item: catalog.ContentItem{Name: "rule-beta"}},
		}
		m.searchInput.SetValue("alpha")
		filtered := m.filteredEntries()
		if len(filtered) != 1 {
			t.Errorf("filteredEntries() = %d, want 1", len(filtered))
		}
		if filtered[0].item.Name != "rule-alpha" {
			t.Errorf("filtered[0].Name = %q, want rule-alpha", filtered[0].item.Name)
		}
	})

	t.Run("selectedItems returns only selected", func(t *testing.T) {
		m := newCreateLoadoutModal("", "", providers, cat)
		m.entries = []loadoutItemEntry{
			{item: catalog.ContentItem{Name: "a"}, selected: true},
			{item: catalog.ContentItem{Name: "b"}, selected: false},
			{item: catalog.ContentItem{Name: "c"}, selected: true},
		}
		sel := m.selectedItems()
		if len(sel) != 2 {
			t.Errorf("selectedItems() = %d, want 2", len(sel))
		}
	})
}
