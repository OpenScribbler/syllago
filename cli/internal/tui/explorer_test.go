package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func explorerTestItems() []catalog.ContentItem {
	return []catalog.ContentItem{
		{Name: "alpha-skill", Type: catalog.Skills, Source: "team-repo", Files: []string{"SKILL.md"}},
		{Name: "beta-skill", Type: catalog.Skills, Source: "team-repo", Files: []string{"SKILL.md"}},
		{Name: "gamma-skill", Type: catalog.Skills, Source: "library", Files: []string{"SKILL.md"}},
	}
}

func TestExplorerModel_RendersBothPanes(t *testing.T) {
	t.Parallel()

	items := explorerTestItems()
	m := newExplorerModel(items, catalog.Skills, 80, 20)

	view := m.View()

	// Left pane should contain item names.
	if !strings.Contains(view, "alpha-skill") {
		t.Error("expected left pane to contain 'alpha-skill'")
	}
	if !strings.Contains(view, "beta-skill") {
		t.Error("expected left pane to contain 'beta-skill'")
	}

	// Should contain the vertical border separator.
	if !strings.Contains(view, "│") {
		t.Error("expected vertical border separator")
	}

	// Right pane should contain preview title.
	if !strings.Contains(view, "Preview:") {
		t.Error("expected right pane to contain 'Preview:'")
	}
}

func TestExplorerModel_FocusSwitching(t *testing.T) {
	t.Parallel()

	items := explorerTestItems()
	m := newExplorerModel(items, catalog.Skills, 80, 20)

	// Initially focused on items (left).
	if m.focusRight {
		t.Error("expected initial focus on items (left pane)")
	}

	// Press right to focus content zone.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !m.focusRight {
		t.Error("expected focus to move to content zone after pressing right")
	}

	// Press left to focus items again.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.focusRight {
		t.Error("expected focus to return to items after pressing left")
	}
}

func TestExplorerModel_ContentTypeUseSplit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ct        catalog.ContentType
		wantSplit bool
	}{
		{catalog.Skills, true},
		{catalog.Hooks, true},
		{catalog.Agents, false},
		{catalog.Rules, false},
		{catalog.MCP, false},
		{catalog.Commands, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.ct), func(t *testing.T) {
			t.Parallel()
			m := newExplorerModel(nil, tt.ct, 80, 20)
			if m.useSplit != tt.wantSplit {
				t.Errorf("content type %s: useSplit = %v, want %v", tt.ct, m.useSplit, tt.wantSplit)
			}
		})
	}
}

func TestExplorerModel_WidthAllocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		totalWidth int
		wantRatio  string // "25%" or "30%"
	}{
		{"wide terminal", 120, "25%"},
		{"narrow terminal", 60, "30%"},
		{"exact threshold", 100, "25%"},
		{"below threshold", 99, "30%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newExplorerModel(nil, catalog.Skills, tt.totalWidth, 20)
			itemsW, contentW := m.splitWidths()

			// Items width should be at least 20.
			if itemsW < 20 && tt.totalWidth > 21 {
				t.Errorf("items width %d < minimum 20", itemsW)
			}

			// Total should account for the 1-char border.
			if itemsW+contentW+1 != tt.totalWidth {
				t.Errorf("width split: items(%d) + border(1) + content(%d) = %d, want %d",
					itemsW, contentW, itemsW+contentW+1, tt.totalWidth)
			}

			// Check approximate ratio.
			available := float64(tt.totalWidth - 1)
			ratio := float64(itemsW) / available
			if tt.wantRatio == "25%" && (ratio < 0.24 || ratio > 0.26) {
				t.Errorf("expected ~25%% ratio, got %.1f%%", ratio*100)
			}
			if tt.wantRatio == "30%" && (ratio < 0.29 || ratio > 0.35) {
				t.Errorf("expected ~30%% ratio, got %.1f%%", ratio*100)
			}
		})
	}
}

func TestExplorerModel_EmptyItems(t *testing.T) {
	t.Parallel()

	m := newExplorerModel(nil, catalog.Rules, 80, 20)

	view := m.View()

	// Should still render something (items header at minimum).
	if view == "" {
		t.Error("expected non-empty view even with no items")
	}

	// Should show the content type in the title.
	if !strings.Contains(view, "Rules") {
		t.Error("expected content type label in empty view")
	}

	// Should show count of 0.
	if !strings.Contains(view, "(0)") {
		t.Error("expected (0) count in empty view")
	}
}

func TestExplorerModel_ItemNavigationUpdatesPreview(t *testing.T) {
	t.Parallel()

	items := explorerTestItems()
	m := newExplorerModel(items, catalog.Skills, 80, 20)

	// Navigate down — this should emit an itemSelectedMsg.
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if cmd == nil {
		t.Fatal("expected a command from moving cursor down")
	}

	// Execute the command to get the message.
	msg := cmd()
	selMsg, ok := msg.(itemSelectedMsg)
	if !ok {
		t.Fatalf("expected itemSelectedMsg, got %T", msg)
	}

	if selMsg.item.Name != "beta-skill" {
		t.Errorf("expected selected item 'beta-skill', got %q", selMsg.item.Name)
	}

	// Process the selection message — should update preview.
	m, _ = m.Update(selMsg)
	// The preview filename should now reflect beta-skill's file.
	if m.preview.filename != "SKILL.md" {
		t.Errorf("expected preview filename 'SKILL.md', got %q", m.preview.filename)
	}
}

func TestExplorerModel_ContentFocusForwardsKeys(t *testing.T) {
	t.Parallel()

	items := explorerTestItems()
	m := newExplorerModel(items, catalog.Skills, 80, 20)

	// Focus the content zone.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !m.focusRight {
		t.Fatal("expected focus on content zone")
	}

	// Pressing down should scroll the preview, not move the items cursor.
	prevCursor := m.items.cursor
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if m.items.cursor != prevCursor {
		t.Error("items cursor should not move when content zone is focused")
	}
}

func TestExplorerModel_Resize(t *testing.T) {
	t.Parallel()

	items := explorerTestItems()
	m := newExplorerModel(items, catalog.Skills, 80, 20)

	m.resize(120, 40)

	if m.width != 120 || m.height != 40 {
		t.Errorf("expected dimensions 120x40, got %dx%d", m.width, m.height)
	}

	itemsW, contentW := m.splitWidths()
	if m.items.width != itemsW {
		t.Errorf("items model width = %d, want %d", m.items.width, itemsW)
	}
	if m.preview.width != contentW {
		t.Errorf("preview model width = %d, want %d", m.preview.width, contentW)
	}
}

func TestExplorerModel_VeryNarrowTerminal(t *testing.T) {
	t.Parallel()

	m := newExplorerModel(nil, catalog.Rules, 15, 10)

	view := m.View()
	// Should not panic and should render something.
	if view == "" {
		t.Error("expected non-empty view at narrow width")
	}
}
