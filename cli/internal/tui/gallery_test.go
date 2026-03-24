package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func testGalleryCards() []cardData {
	return []cardData{
		{
			title: "Python-Web",
			lines: []string{"4 Skills", "2 Rules", "Target: Claude Code"},
			contents: []contentGroup{
				{typeName: "Skills", items: []string{"Refactor-Python", "Py-Doc-Gen", "Django-Patterns", "Test-Generator"}},
				{typeName: "Rules", items: []string{"Strict-Types", "PEP8-Lint"}},
			},
		},
		{
			title: "React-Frontend",
			lines: []string{"6 Skills", "1 Agent", "Target: Cursor"},
			contents: []contentGroup{
				{typeName: "Skills", items: []string{"React-Hooks", "JSX-Patterns", "CSS-Modules", "Storybook", "Testing-Library", "Accessibility"}},
				{typeName: "Agents", items: []string{"UI-Reviewer"}},
			},
		},
		{
			title: "Go-Backend",
			lines: []string{"3 Skills"},
			contents: []contentGroup{
				{typeName: "Skills", items: []string{"Error-Handling", "Concurrency", "Testing"}},
			},
		},
	}
}

func TestGalleryRendersGridAndSidebar(t *testing.T) {
	t.Parallel()

	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 120, 30)
	v := m.View()

	// Card grid should show card titles
	if !strings.Contains(v, "Python-Web") {
		t.Error("view should contain card title 'Python-Web'")
	}

	// Sidebar should show contents of first card
	if !strings.Contains(v, "Contents (6)") {
		t.Error("view should contain sidebar title")
	}

	// Sidebar should show item names from first card
	if !strings.Contains(v, "Refactor-Python") {
		t.Error("sidebar should show first card's items")
	}

	// Separator should be present
	if !strings.Contains(v, "\u2502") {
		t.Error("view should contain vertical separator")
	}
}

func TestGalleryTabSwitchesFocus(t *testing.T) {
	t.Parallel()

	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 120, 30)

	// Initially grid is focused
	if !m.focusGrid {
		t.Fatal("grid should be focused initially")
	}

	// Tab switches to sidebar
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusGrid {
		t.Error("tab should switch focus to sidebar")
	}

	// Tab switches back to grid
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.focusGrid {
		t.Error("second tab should switch focus back to grid")
	}
}

func TestGalleryCardSelectionUpdatesSidebar(t *testing.T) {
	t.Parallel()

	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 120, 30)

	// Initial sidebar shows first card contents
	if !strings.Contains(m.contents.title, "Contents (6)") {
		t.Errorf("initial sidebar title should be 'Contents (6)', got %q", m.contents.title)
	}

	// Move cursor right to second card
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Sidebar should now show second card contents (7 items)
	if !strings.Contains(m.contents.title, "Contents (7)") {
		t.Errorf("sidebar should update to 'Contents (7)' after moving to card 2, got %q", m.contents.title)
	}

	v := m.View()
	if !strings.Contains(v, "React-Hooks") {
		t.Error("sidebar should show second card's items after cursor move")
	}
}

func TestGallerySidebarHiddenAtNarrowWidth(t *testing.T) {
	t.Parallel()

	// Width < 65 should hide the sidebar
	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 60, 30)

	if m.showSidebar() {
		t.Error("sidebar should be hidden at width 60")
	}

	v := m.View()
	// Cards should still render
	if !strings.Contains(v, "Python-Web") {
		t.Error("cards should still render at narrow width")
	}

	// Separator should NOT be present (no sidebar)
	// Count separator occurrences — at narrow width there should be none from gallery
	lines := strings.Split(v, "\n")
	for _, line := range lines {
		// The vertical bar used as separator is the only use of this char in gallery
		// But cards might have their own borders. Just check sidebar content is absent.
		if strings.Contains(line, "Refactor-Python") {
			t.Error("sidebar content should not appear at narrow width")
		}
	}
}

func TestGallerySidebarShownAtWideWidth(t *testing.T) {
	t.Parallel()

	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 120, 30)
	if !m.showSidebar() {
		t.Error("sidebar should be shown at width 120")
	}
}

func TestGalleryTabNoopAtNarrowWidth(t *testing.T) {
	t.Parallel()

	// Tab should be a no-op when sidebar is hidden
	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 60, 30)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if !m.focusGrid {
		t.Error("tab should not switch focus when sidebar is hidden")
	}
}

func TestGalleryEmptyCards(t *testing.T) {
	t.Parallel()

	m := newGalleryModel(nil, catalog.Loadouts, 120, 30)
	v := m.View()

	// Should not panic and should render something
	if v == "" {
		t.Error("empty gallery should still render")
	}

	// Should show empty contents
	if !strings.Contains(m.contents.title, "Contents (0)") {
		t.Errorf("empty gallery sidebar should show 'Contents (0)', got %q", m.contents.title)
	}
}

func TestGallerySplit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		width       int
		wantSidebar bool
		wantGridMin int
		wantSideMin int
	}{
		{"narrow hides sidebar", 60, false, 60, 0},
		{"boundary hides sidebar", 64, false, 64, 0},
		{"boundary shows sidebar", 65, true, 20, 20},
		{"wide", 120, true, 20, 20},
		{"very wide", 200, true, 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gridW, sidebarW := gallerySplit(tt.width)

			if tt.wantSidebar {
				if sidebarW < tt.wantSideMin {
					t.Errorf("sidebarW=%d, want >= %d", sidebarW, tt.wantSideMin)
				}
				if gridW < tt.wantGridMin {
					t.Errorf("gridW=%d, want >= %d", gridW, tt.wantGridMin)
				}
				// Grid + sidebar + separator should equal total width
				total := gridW + sidebarW + 1
				if total != tt.width {
					t.Errorf("gridW(%d) + sidebarW(%d) + 1 = %d, want %d", gridW, sidebarW, total, tt.width)
				}
			} else {
				if sidebarW != 0 {
					t.Errorf("narrow width: sidebarW=%d, want 0", sidebarW)
				}
				if gridW != tt.width {
					t.Errorf("narrow width: gridW=%d, want %d", gridW, tt.width)
				}
			}
		})
	}
}

func TestGalleryContentsTitle(t *testing.T) {
	t.Parallel()

	card := testGalleryCards()[0] // 4 skills + 2 rules = 6
	title := contentsTitle(card)
	if title != "Contents (6)" {
		t.Errorf("contentsTitle = %q, want 'Contents (6)'", title)
	}
}

func TestGallerySidebarScrollWhenFocused(t *testing.T) {
	t.Parallel()

	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 120, 5)

	// Switch focus to sidebar
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusGrid {
		t.Fatal("focus should be on sidebar")
	}

	// Scroll down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.contents.scrollOffset != 1 {
		t.Errorf("sidebar scroll: expected offset=1, got %d", m.contents.scrollOffset)
	}
}

func TestGalleryGridNavigationWhenFocused(t *testing.T) {
	t.Parallel()

	m := newGalleryModel(testGalleryCards(), catalog.Loadouts, 120, 30)

	// Grid is focused by default — right arrow should move cursor
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.cards.cursor != 1 {
		t.Errorf("grid nav: expected cursor=1, got %d", m.cards.cursor)
	}
}
