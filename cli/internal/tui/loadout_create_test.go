package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
	})

	t.Run("skips provider step when pre-filled", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		if m.step != clStepTypes {
			t.Errorf("step = %v, want clStepTypes", m.step)
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
		// Use a catalog with items so selectedTypes can be populated
		catWithItems := &catalog.Catalog{
			Items: []catalog.ContentItem{
				{Name: "r1", Type: catalog.Rules, Provider: "claude-code"},
				{Name: "s1", Type: catalog.Skills},
			},
		}
		m := newCreateLoadoutModal("claude-code", "", providers, catWithItems)
		if m.currentStepNum() != 1 {
			t.Errorf("currentStepNum() = %d, want 1 for types step with prefill", m.currentStepNum())
		}
		// Simulate advancing through types step (builds selectedTypes)
		m.buildTypeItemMaps()
		m.step = clStepItems
		m.typeStepIndex = 0
		if m.currentStepNum() != 2 {
			t.Errorf("currentStepNum() = %d, want 2 for first items step with prefill", m.currentStepNum())
		}
		// With 2 types selected, name step = 1 + 2 + 1 = 4
		m.step = clStepName
		if m.currentStepNum() != 4 {
			t.Errorf("currentStepNum() = %d, want 4 for name step with prefill (2 types)", m.currentStepNum())
		}
		m.step = clStepDest
		if m.currentStepNum() != 5 {
			t.Errorf("currentStepNum() = %d, want 5 for dest step with prefill (2 types)", m.currentStepNum())
		}
		m.step = clStepReview
		if m.currentStepNum() != 6 {
			t.Errorf("currentStepNum() = %d, want 6 for review step with prefill (2 types)", m.currentStepNum())
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

	t.Run("confirmed is false on Esc dismissal", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		// Navigate to name step and enter a name
		m.step = clStepName
		m.nameInput.SetValue("test-loadout")
		// Press Esc to cancel
		m, _ = m.Update(keyEsc)
		if m.confirmed {
			t.Error("confirmed should be false after Esc")
		}
	})

	t.Run("dest Enter advances to review step", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.step = clStepDest
		m, _ = m.Update(keyEnter)
		if m.step != clStepReview {
			t.Errorf("step = %v, want clStepReview", m.step)
		}
		if m.reviewCursor != 1 {
			t.Error("reviewCursor should default to 1 (Create)")
		}
	})

	t.Run("confirmed is true on Create in review step", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.step = clStepReview
		m.reviewCursor = 1 // Create button
		m, _ = m.Update(keyEnter)
		if !m.confirmed {
			t.Error("confirmed should be true after Enter on Create")
		}
		if m.active {
			t.Error("active should be false after confirming")
		}
	})

	t.Run("Back button in review returns to dest", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.step = clStepReview
		m.reviewCursor = 0 // Back button
		m, _ = m.Update(keyEnter)
		if m.step != clStepDest {
			t.Errorf("step = %v, want clStepDest", m.step)
		}
		if m.confirmed {
			t.Error("confirmed should be false after Back")
		}
	})
}

func TestCreateLoadoutTypeSelection(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "rule-a", Type: catalog.Rules, Provider: "claude-code"},
			{Name: "rule-b", Type: catalog.Rules, Provider: "claude-code"},
			{Name: "skill-a", Type: catalog.Skills},
			{Name: "agent-a", Type: catalog.Agents},
		},
	}
	providers := []provider.Provider{
		{
			Name: "Claude Code", Slug: "claude-code", Detected: true,
			SupportsType: func(ct catalog.ContentType) bool { return true },
		},
		{
			Name: "Cursor", Slug: "cursor", Detected: false,
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Rules },
		},
	}

	t.Run("shows only types with available items for provider", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		// Claude Code supports all types, but catalog only has Rules, Skills, Agents
		typeNames := make([]string, len(m.typeEntries))
		for i, te := range m.typeEntries {
			typeNames[i] = string(te.ct)
		}
		if len(m.typeEntries) != 3 {
			t.Errorf("typeEntries = %d (%v), want 3 (Rules, Skills, Agents)", len(m.typeEntries), typeNames)
		}
	})

	t.Run("cursor provider only shows Rules when no other supported types have items", func(t *testing.T) {
		m := newCreateLoadoutModal("cursor", "", providers, cat)
		// Cursor only supports Rules, so only Rules should appear
		if len(m.typeEntries) != 1 {
			t.Errorf("typeEntries = %d, want 1 (Rules only)", len(m.typeEntries))
		}
		if len(m.typeEntries) > 0 && m.typeEntries[0].ct != catalog.Rules {
			t.Errorf("typeEntries[0].ct = %v, want Rules", m.typeEntries[0].ct)
		}
	})

	t.Run("all types default to checked", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		for _, te := range m.typeEntries {
			if !te.checked {
				t.Errorf("type %v should be checked by default", te.ct)
			}
		}
	})

	t.Run("space toggles individual checkbox", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		if !m.typeEntries[0].checked {
			t.Fatal("first type should start checked")
		}
		m, _ = m.Update(keySpace)
		if m.typeEntries[0].checked {
			t.Error("first type should be unchecked after space")
		}
		m, _ = m.Update(keySpace)
		if !m.typeEntries[0].checked {
			t.Error("first type should be re-checked after second space")
		}
	})

	t.Run("a key toggles all on and off", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		// All start checked; pressing 'a' should uncheck all
		m, _ = m.Update(keyRune('a'))
		for _, te := range m.typeEntries {
			if te.checked {
				t.Errorf("type %v should be unchecked after toggle-all", te.ct)
			}
		}
		// Press 'a' again to check all
		m, _ = m.Update(keyRune('a'))
		for _, te := range m.typeEntries {
			if !te.checked {
				t.Errorf("type %v should be checked after second toggle-all", te.ct)
			}
		}
	})

	t.Run("enter with 0 types selected shows validation", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		// Uncheck all
		m, _ = m.Update(keyRune('a'))
		// Try to advance
		m, _ = m.Update(keyEnter)
		if m.step != clStepTypes {
			t.Error("should stay on types step when 0 selected")
		}
		if m.message != "Select at least one content type" {
			t.Errorf("message = %q, want validation message", m.message)
		}
	})

	t.Run("enter advances to items step when types selected", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m, _ = m.Update(keyEnter)
		if m.step != clStepItems {
			t.Errorf("step = %v, want clStepItems", m.step)
		}
	})

	t.Run("type counts reflect catalog items", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		for _, te := range m.typeEntries {
			switch te.ct {
			case catalog.Rules:
				if te.count != 2 {
					t.Errorf("Rules count = %d, want 2", te.count)
				}
			case catalog.Skills:
				if te.count != 1 {
					t.Errorf("Skills count = %d, want 1", te.count)
				}
			case catalog.Agents:
				if te.count != 1 {
					t.Errorf("Agents count = %d, want 1", te.count)
				}
			}
		}
	})
}

// TestCreateLoadoutPerTypeItems tests the per-type item selection step (Task 1.3).
func TestCreateLoadoutPerTypeItems(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	// Helper: create a modal at the items step with all types checked for claude-code.
	setupItems := func(t *testing.T) createLoadoutModal {
		t.Helper()
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		// Advance past types step (all checked by default)
		m, _ = m.Update(keyEnter)
		if m.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", m.step)
		}
		return m
	}

	t.Run("space toggles compatible item selection", func(t *testing.T) {
		m := setupItems(t)
		ct := m.currentType()
		filtered := m.filteredTypeItems()
		if len(filtered) == 0 {
			t.Fatal("no items for first type")
		}
		// Item should start unselected
		firstIdx := filtered[0]
		if m.entries[firstIdx].selected {
			t.Fatal("item should start unselected")
		}
		// Space to select
		m, _ = m.Update(keySpace)
		if !m.entries[firstIdx].selected {
			t.Error("space should select item")
		}
		// Space again to deselect
		m, _ = m.Update(keySpace)
		if m.entries[firstIdx].selected {
			t.Error("second space should deselect item")
		}
		_ = ct // used indirectly via currentType()
	})

	t.Run("t key toggles compatibility filter", func(t *testing.T) {
		// Use cursor provider (Skills + Rules only) so some types are incompatible.
		m := newCreateLoadoutModal("cursor", "", providers, cat)
		// Cursor only has Rules type entry (the only supported type with items)
		m, _ = m.Update(keyEnter) // advance to items
		if m.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", m.step)
		}

		ct := m.currentType()
		compatCount := len(m.typeItemMap[ct])
		allCount := len(m.typeItemMapAll[ct])

		// Default: showAllCompat=false, only compatible items shown
		if m.showAllCompat {
			t.Fatal("showAllCompat should start false")
		}
		if len(m.filteredTypeItems()) != compatCount {
			t.Errorf("default filter should show %d compatible items, got %d", compatCount, len(m.filteredTypeItems()))
		}

		// Press 't' to show all (including incompatible)
		m, _ = m.Update(keyRune('t'))
		if !m.showAllCompat {
			t.Error("t should toggle showAllCompat to true")
		}
		if len(m.filteredTypeItems()) != allCount {
			t.Errorf("after toggle should show %d items (all), got %d", allCount, len(m.filteredTypeItems()))
		}

		// Press 't' again to go back
		m, _ = m.Update(keyRune('t'))
		if m.showAllCompat {
			t.Error("second t should toggle showAllCompat back to false")
		}
	})

	t.Run("a key toggles all compatible items", func(t *testing.T) {
		m := setupItems(t)
		ct := m.currentType()
		compatible := m.typeItemMap[ct]
		if len(compatible) == 0 {
			t.Skip("no compatible items for first type")
		}

		// Press 'a' to select all
		m, _ = m.Update(keyRune('a'))
		for _, idx := range compatible {
			if !m.entries[idx].selected {
				t.Errorf("item %q should be selected after toggle-all", m.entries[idx].item.Name)
			}
		}

		// Press 'a' again to deselect all
		m, _ = m.Update(keyRune('a'))
		for _, idx := range compatible {
			if m.entries[idx].selected {
				t.Errorf("item %q should be deselected after second toggle-all", m.entries[idx].item.Name)
			}
		}
	})

	t.Run("search filter reduces visible items", func(t *testing.T) {
		m := setupItems(t)
		ct := m.currentType()
		fullCount := len(m.filteredTypeItems())
		if fullCount < 2 {
			t.Skip("need at least 2 items to test search")
		}

		// Select first item before searching
		m, _ = m.Update(keySpace)
		firstIdx := m.filteredTypeItems()[0]
		if !m.entries[firstIdx].selected {
			t.Fatal("first item should be selected")
		}

		// Activate search with '/'
		m, _ = m.Update(keyRune('/'))
		if !m.searchInput.Focused() {
			t.Fatal("search should be focused after /")
		}

		// Type a search query matching one specific item
		targetName := m.entries[m.filteredTypeItems()[0]].item.Name
		for _, r := range targetName {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		filtered := m.filteredTypeItems()
		if len(filtered) >= fullCount {
			t.Errorf("search should reduce items, got %d (was %d)", len(filtered), fullCount)
		}

		// Clear search via Esc
		m, _ = m.Update(keyEsc)
		if m.searchInput.Focused() {
			t.Error("search should blur after Esc")
		}
		if m.perTypeSearch[ct] != "" {
			t.Error("search query should be cleared after Esc")
		}

		// Selection should persist after search cleared
		if !m.entries[firstIdx].selected {
			t.Error("selection should persist after clearing search")
		}
	})

	t.Run("esc from first type returns to type selection", func(t *testing.T) {
		m := setupItems(t)
		// Select an item first
		m, _ = m.Update(keySpace)
		firstIdx := m.filteredTypeItems()[0]
		if !m.entries[firstIdx].selected {
			t.Fatal("item should be selected")
		}

		// Esc goes back to types step
		m, _ = m.Update(keyEsc)
		if m.step != clStepTypes {
			t.Errorf("Esc from first type should go to clStepTypes, got %d", m.step)
		}

		// Re-enter items step
		m, _ = m.Update(keyEnter)
		if m.step != clStepItems {
			t.Fatalf("Enter should go to clStepItems, got %d", m.step)
		}

		// Selection should be preserved
		if !m.entries[firstIdx].selected {
			t.Error("item selection should persist after going back and re-entering")
		}
	})

	t.Run("esc preserves cursor position per type", func(t *testing.T) {
		m := setupItems(t)
		ct := m.currentType()

		// Move cursor down
		m, _ = m.Update(keyDown)
		if m.perTypeCursor[ct] != 1 {
			t.Fatalf("cursor should be 1 after down, got %d", m.perTypeCursor[ct])
		}

		// Advance to next type via Enter (if multiple types)
		if len(m.selectedTypes) < 2 {
			t.Skip("need at least 2 types to test cursor preservation across types")
		}
		m, _ = m.Update(keyEnter)
		nextCt := m.currentType()
		if nextCt == ct {
			t.Fatal("should have advanced to next type")
		}

		// Esc back to first type
		m, _ = m.Update(keyEsc)
		if m.currentType() != ct {
			t.Fatal("should have gone back to first type")
		}
		if m.perTypeCursor[ct] != 1 {
			t.Error("cursor position should be preserved after going back")
		}
	})

	t.Run("empty type shows message and Enter advances", func(t *testing.T) {
		// Create a catalog where one type has items only from a specific provider
		// and the wizard provider doesn't match, resulting in 0 compatible items.
		// We'll use a custom minimal catalog for this.
		emptyCat := &catalog.Catalog{
			Items: []catalog.ContentItem{
				{Name: "r1", Type: catalog.Rules, Provider: "claude-code"},
				{Name: "s1", Type: catalog.Skills}, // universal
			},
		}
		m := newCreateLoadoutModal("claude-code", "", providers, emptyCat)
		// Only check Rules (which has 1 item) and uncheck Skills
		for i := range m.typeEntries {
			if m.typeEntries[i].ct == catalog.Skills {
				m.typeEntries[i].checked = false
			}
		}
		m, _ = m.Update(keyEnter) // advance to items
		if m.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", m.step)
		}

		// Should have 1 type (Rules) with items
		filtered := m.filteredTypeItems()
		if len(filtered) == 0 {
			// Enter should advance even with 0 items
			m, _ = m.Update(keyEnter)
			if m.step != clStepName {
				t.Errorf("Enter on empty type should advance to name step, got %d", m.step)
			}
		} else {
			// Enter advances to next step since only 1 type
			m, _ = m.Update(keyEnter)
			if m.step != clStepName {
				t.Errorf("Enter should advance to name step, got %d", m.step)
			}
		}
	})

	t.Run("enter advances through all types then to name", func(t *testing.T) {
		m := setupItems(t)
		nTypes := len(m.selectedTypes)
		if nTypes == 0 {
			t.Fatal("no selected types")
		}

		// Press Enter for each type to advance through all of them
		for i := 0; i < nTypes; i++ {
			if m.step != clStepItems {
				t.Fatalf("should be on items step at type %d, got %d", i, m.step)
			}
			m, _ = m.Update(keyEnter)
		}

		// After all types, should be on name step
		if m.step != clStepName {
			t.Errorf("after all types, expected clStepName, got %d", m.step)
		}
	})

	t.Run("incompatible items cannot be toggled", func(t *testing.T) {
		// Use cursor provider — only supports Skills + Rules
		m := newCreateLoadoutModal("cursor", "", providers, cat)
		m, _ = m.Update(keyEnter) // advance to items

		ct := m.currentType()
		// Show all items (including incompatible)
		m, _ = m.Update(keyRune('t'))

		allItems := m.filteredTypeItems()
		// Find an incompatible item
		for _, idx := range allItems {
			if !m.isItemCompatible(idx) {
				// Navigate cursor to it
				m.perTypeCursor[ct] = 0
				for vi, fi := range allItems {
					if fi == idx {
						m.perTypeCursor[ct] = vi
						break
					}
				}
				// Try to toggle — should not select
				m, _ = m.Update(keySpace)
				if m.entries[idx].selected {
					t.Error("incompatible item should not be selectable via space")
				}
				return
			}
		}
		// If all items are compatible for cursor's type, that's OK — skip
		t.Skip("no incompatible items found for this provider+type combo")
	})
}

// TestCreateLoadoutReviewStep tests the review step (Task 1.4).
func TestCreateLoadoutReviewStep(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("review shows name and provider", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.nameInput.SetValue("my-loadout")
		m.descInput.SetValue("A test loadout")
		m.step = clStepReview
		view := m.View()
		if !strings.Contains(view, "my-loadout") {
			t.Error("review should show loadout name")
		}
		if !strings.Contains(view, "Claude Code") {
			t.Error("review should show provider name")
		}
		if !strings.Contains(view, "A test loadout") {
			t.Error("review should show description")
		}
	})

	t.Run("review groups selected items by type", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.nameInput.SetValue("test")
		// Select some items of different types
		for i := range m.entries {
			if m.entries[i].item.Type == catalog.Skills || m.entries[i].item.Type == catalog.Rules {
				m.entries[i].selected = true
			}
		}
		m.step = clStepReview
		view := m.View()
		if !strings.Contains(view, "Skills") {
			t.Error("review should show Skills group")
		}
		if !strings.Contains(view, "Rules") {
			t.Error("review should show Rules group")
		}
	})

	t.Run("review shows security warning for hooks", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.nameInput.SetValue("test")
		for i := range m.entries {
			if m.entries[i].item.Type == catalog.Hooks {
				m.entries[i].selected = true
			}
		}
		m.step = clStepReview
		view := m.View()
		if !strings.Contains(view, "Security Notice") {
			t.Error("review should show security warning when hooks selected")
		}
	})

	t.Run("review shows security warning for MCP", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.nameInput.SetValue("test")
		for i := range m.entries {
			if m.entries[i].item.Type == catalog.MCP {
				m.entries[i].selected = true
			}
		}
		m.step = clStepReview
		view := m.View()
		if !strings.Contains(view, "Security Notice") {
			t.Error("review should show security warning when MCP selected")
		}
	})

	t.Run("review with 0 items shows warning", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.nameInput.SetValue("empty")
		m.step = clStepReview
		view := m.View()
		if !strings.Contains(view, "No items selected") {
			t.Error("review should show warning when no items selected")
		}
	})

	t.Run("left/right switches buttons", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.step = clStepReview
		m.reviewCursor = 1 // Create
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		if m.reviewCursor != 0 {
			t.Errorf("left should move to Back (0), got %d", m.reviewCursor)
		}
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		if m.reviewCursor != 1 {
			t.Errorf("right should move to Create (1), got %d", m.reviewCursor)
		}
	})

	t.Run("esc from review returns to dest", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.step = clStepReview
		m, _ = m.Update(keyEsc)
		if m.step != clStepDest {
			t.Errorf("Esc should return to clStepDest, got %d", m.step)
		}
		if m.confirmed {
			t.Error("confirmed should be false after Esc")
		}
	})

	t.Run("review omits empty content types", func(t *testing.T) {
		m := newCreateLoadoutModal("claude-code", "", providers, cat)
		m.nameInput.SetValue("test")
		// Select only Skills items
		for i := range m.entries {
			if m.entries[i].item.Type == catalog.Skills {
				m.entries[i].selected = true
			}
		}
		m.step = clStepReview
		view := m.View()
		if !strings.Contains(view, "Skills") {
			t.Error("review should show Skills")
		}
		// Agents, Hooks, MCP etc. should not appear since none selected
		if strings.Contains(view, "Agents (") {
			t.Error("review should not show Agents when none selected")
		}
	})
}

// TestCreateLoadoutRescanFindsNewLoadout verifies that after doCreateLoadout
// writes a loadout to the project destination, the catalog rescan finds it.
// This is the regression test for the creation bug where loadouts didn't appear.
func TestCreateLoadoutRescanFindsNewLoadout(t *testing.T) {
	// Use testCatalog which creates real files on disk with RepoRoot = tmp dir.
	// The scanner will be able to rescan this directory.
	app := testApp(t)
	contentRoot := app.catalog.RepoRoot

	// Count loadouts before creation
	beforeCount := len(app.catalog.ByType(catalog.Loadouts))

	// Set up the modal as if the user completed the wizard
	modal := newCreateLoadoutModal("claude-code", "", app.providers, app.catalog)
	modal.prefilledProvider = "claude-code"
	modal.nameInput.SetValue("new-test-loadout")
	modal.descInput.SetValue("A test loadout")
	modal.step = clStepDest
	modal.destCursor = 0 // Project destination
	modal.confirmed = true

	// Execute doCreateLoadout (the tea.Cmd) synchronously
	cmd := app.doCreateLoadout(modal)
	msg := cmd()

	// Verify no error
	result := msg.(doCreateLoadoutMsg)
	if result.err != nil {
		t.Fatalf("doCreateLoadout failed: %v", result.err)
	}

	// Verify the loadout.yaml was written to contentRoot (not projectRoot)
	loadoutPath := filepath.Join(contentRoot, "loadouts", "claude-code", "new-test-loadout", "loadout.yaml")
	if _, err := os.Stat(loadoutPath); err != nil {
		t.Fatalf("loadout.yaml not found at %s: %v", loadoutPath, err)
	}

	// Simulate the doCreateLoadoutMsg handler: rescan the catalog
	cat, err := catalog.Scan(contentRoot, contentRoot)
	if err != nil {
		t.Fatalf("rescan failed: %v", err)
	}

	afterCount := len(cat.ByType(catalog.Loadouts))
	if afterCount != beforeCount+1 {
		t.Errorf("loadout count after rescan = %d, want %d (before=%d)", afterCount, beforeCount+1, beforeCount)
	}

	// Verify the new loadout is in the catalog
	found := false
	for _, item := range cat.ByType(catalog.Loadouts) {
		if item.Name == "new-test-loadout" {
			found = true
			if item.Provider != "claude-code" {
				t.Errorf("provider = %q, want claude-code", item.Provider)
			}
			break
		}
	}
	if !found {
		t.Error("new-test-loadout not found in catalog after rescan")
	}
}
