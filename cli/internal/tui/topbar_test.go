package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func init() {
	// Disable color output for deterministic test rendering.
	lipgloss.SetColorProfile(0)
}

func TestNewTopBarModel(t *testing.T) {
	t.Parallel()
	m := newTopBarModel()

	t.Run("content menu items", func(t *testing.T) {
		want := []string{"Skills", "Agents", "MCP Configs", "Rules", "Hooks", "Commands"}
		if len(m.contentMenu.items) != len(want) {
			t.Fatalf("content menu: got %d items, want %d", len(m.contentMenu.items), len(want))
		}
		for i, item := range want {
			if m.contentMenu.items[i] != item {
				t.Errorf("content menu[%d]: got %q, want %q", i, m.contentMenu.items[i], item)
			}
		}
	})

	t.Run("collection menu items", func(t *testing.T) {
		want := []string{"Library", "Registries", "Loadouts"}
		if len(m.collectionMenu.items) != len(want) {
			t.Fatalf("collection menu: got %d items, want %d", len(m.collectionMenu.items), len(want))
		}
		for i, item := range want {
			if m.collectionMenu.items[i] != item {
				t.Errorf("collection menu[%d]: got %q, want %q", i, m.collectionMenu.items[i], item)
			}
		}
	})

	t.Run("config menu items", func(t *testing.T) {
		want := []string{"Settings", "Sandbox"}
		if len(m.configMenu.items) != len(want) {
			t.Fatalf("config menu: got %d items, want %d", len(m.configMenu.items), len(want))
		}
		for i, item := range want {
			if m.configMenu.items[i] != item {
				t.Errorf("config menu[%d]: got %q, want %q", i, m.configMenu.items[i], item)
			}
		}
	})

	t.Run("defaults", func(t *testing.T) {
		if m.activeCategory != dropdownContent {
			t.Errorf("activeCategory: got %d, want %d", m.activeCategory, dropdownContent)
		}
		if m.openMenu != -1 {
			t.Errorf("openMenu: got %d, want -1", m.openMenu)
		}
		if m.menuOpen {
			t.Error("menuOpen should be false initially")
		}
	})
}

func TestTopBarDropdownOpenClose(t *testing.T) {
	t.Parallel()

	t.Run("toggle open", func(t *testing.T) {
		m := newTopBarModel()
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
		if !m.menuOpen {
			t.Error("menu should be open after pressing 1")
		}
		if m.openMenu != dropdownContent {
			t.Errorf("openMenu: got %d, want %d", m.openMenu, dropdownContent)
		}
	})

	t.Run("toggle close", func(t *testing.T) {
		m := newTopBarModel()
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
		if m.menuOpen {
			t.Error("menu should be closed after pressing 1 twice")
		}
	})

	t.Run("esc closes", func(t *testing.T) {
		m := newTopBarModel()
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		if !m.menuOpen {
			t.Fatal("menu should be open")
		}
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if m.menuOpen {
			t.Error("menu should be closed after Esc")
		}
	})

	t.Run("switching menus", func(t *testing.T) {
		m := newTopBarModel()
		// Open content menu
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
		if m.openMenu != dropdownContent {
			t.Fatal("should have content menu open")
		}
		// Switch to collection menu
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		if m.openMenu != dropdownCollection {
			t.Errorf("openMenu: got %d, want %d", m.openMenu, dropdownCollection)
		}
		if m.contentMenu.open {
			t.Error("content menu should be closed when switching")
		}
		if !m.collectionMenu.open {
			t.Error("collection menu should be open")
		}
	})
}

func TestTopBarSelection(t *testing.T) {
	t.Parallel()

	t.Run("select content item", func(t *testing.T) {
		m := newTopBarModel()
		// Open content menu
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
		// Move down to "Agents"
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		// Select
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if m.menuOpen {
			t.Error("menu should close after selection")
		}
		if m.activeCategory != dropdownContent {
			t.Errorf("activeCategory: got %d, want %d", m.activeCategory, dropdownContent)
		}
		if cmd == nil {
			t.Fatal("expected a command from selection")
		}
		msg := cmd()
		sel, ok := msg.(topBarSelectMsg)
		if !ok {
			t.Fatalf("expected topBarSelectMsg, got %T", msg)
		}
		if sel.category != dropdownContent {
			t.Errorf("category: got %d, want %d", sel.category, dropdownContent)
		}
		if sel.item != "Agents" {
			t.Errorf("item: got %q, want %q", sel.item, "Agents")
		}
	})

	t.Run("select collection changes active category", func(t *testing.T) {
		m := newTopBarModel()
		// Initially content is active
		if m.activeCategory != dropdownContent {
			t.Fatal("should start with content active")
		}
		// Open and select from collection
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if m.activeCategory != dropdownCollection {
			t.Errorf("activeCategory should be collection, got %d", m.activeCategory)
		}
	})

	t.Run("config selection does not change active category", func(t *testing.T) {
		m := newTopBarModel()
		m.activeCategory = dropdownContent
		// Open and select from config
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// activeCategory should remain content (config doesn't set it)
		if m.activeCategory != dropdownContent {
			t.Errorf("activeCategory should stay content after config select, got %d", m.activeCategory)
		}
	})
}

func TestTopBarMutualExclusion(t *testing.T) {
	t.Parallel()

	t.Run("content active grays collection", func(t *testing.T) {
		m := newTopBarModel()
		m.activeCategory = dropdownContent
		view := m.View(120)
		// When content is active, collection should show "--"
		if !strings.Contains(view, "--") {
			t.Error("expected '--' for inactive collection dropdown")
		}
		// Content should show "Skills" (default selection)
		if !strings.Contains(view, "Skills") {
			t.Error("expected 'Skills' in active content dropdown")
		}
	})

	t.Run("collection active grays content", func(t *testing.T) {
		m := newTopBarModel()
		m.activeCategory = dropdownCollection
		view := m.View(120)
		// Content should show "--" when collection is active
		if !strings.Contains(view, "Content: --") {
			t.Error("expected 'Content: --' for inactive content dropdown")
		}
		// Collection should show "Library" (default selection)
		if !strings.Contains(view, "Library") {
			t.Error("expected 'Library' in active collection dropdown")
		}
	})
}

func TestTopBarView(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		width int
	}{
		{"width 120", 120},
		{"width 80", 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTopBarModel()
			view := m.View(tt.width)

			// Must contain logo
			if !strings.Contains(view, "SYL") {
				t.Error("expected SYL logo in view")
			}
			// Must contain dropdown indicator
			if !strings.Contains(view, "\u25be") {
				t.Error("expected dropdown indicator (▾) in view")
			}
			// Must contain action buttons
			if !strings.Contains(view, "+ Add") {
				t.Error("expected '+ Add' button in view")
			}
			if !strings.Contains(view, "* New") {
				t.Error("expected '* New' button in view")
			}
			// Must contain Content label
			if !strings.Contains(view, "Content:") {
				t.Error("expected 'Content:' label in view")
			}
			// Must contain Config label
			if !strings.Contains(view, "Config") {
				t.Error("expected 'Config' label in view")
			}
			// View should not be empty
			if len(view) == 0 {
				t.Error("view should not be empty")
			}
		})
	}
}

func TestDropdownMenu(t *testing.T) {
	t.Parallel()

	t.Run("new dropdown", func(t *testing.T) {
		d := newDropdownMenu([]string{"A", "B", "C"})
		if d.cursor != 0 {
			t.Errorf("cursor: got %d, want 0", d.cursor)
		}
		if d.open {
			t.Error("should start closed")
		}
		if d.selected() != "A" {
			t.Errorf("selected: got %q, want %q", d.selected(), "A")
		}
	})

	t.Run("navigation", func(t *testing.T) {
		d := newDropdownMenu([]string{"A", "B", "C"})
		d.moveDown()
		if d.selected() != "B" {
			t.Errorf("after moveDown: got %q, want %q", d.selected(), "B")
		}
		d.moveDown()
		if d.selected() != "C" {
			t.Errorf("after second moveDown: got %q, want %q", d.selected(), "C")
		}
		d.moveDown() // should clamp
		if d.selected() != "C" {
			t.Errorf("after clamp moveDown: got %q, want %q", d.selected(), "C")
		}
		d.moveUp()
		if d.selected() != "B" {
			t.Errorf("after moveUp: got %q, want %q", d.selected(), "B")
		}
		d.moveUp()
		d.moveUp() // should clamp at 0
		if d.selected() != "A" {
			t.Errorf("after clamp moveUp: got %q, want %q", d.selected(), "A")
		}
	})

	t.Run("toggle", func(t *testing.T) {
		d := newDropdownMenu([]string{"X"})
		d.toggle()
		if !d.open {
			t.Error("should be open after toggle")
		}
		d.toggle()
		if d.open {
			t.Error("should be closed after second toggle")
		}
	})

	t.Run("view when closed", func(t *testing.T) {
		d := newDropdownMenu([]string{"A", "B"})
		if d.View() != "" {
			t.Error("View should return empty string when closed")
		}
	})

	t.Run("view when open", func(t *testing.T) {
		d := newDropdownMenu([]string{"A", "B", "C"})
		d.open = true
		d.cursor = 1 // B is selected
		view := d.View()
		if !strings.Contains(view, "> B") {
			t.Error("expected '> B' for selected item")
		}
		if !strings.Contains(view, "  A") {
			t.Error("expected '  A' for unselected item")
		}
		if !strings.Contains(view, "  C") {
			t.Error("expected '  C' for unselected item")
		}
	})

	t.Run("empty dropdown", func(t *testing.T) {
		d := newDropdownMenu([]string{})
		if d.selected() != "" {
			t.Error("selected on empty should return empty string")
		}
		d.open = true
		if d.View() != "" {
			t.Error("View on empty open dropdown should return empty string")
		}
	})
}
