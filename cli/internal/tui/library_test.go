package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// newLibraryWithDiskItems returns a libraryModel whose items reference real
// on-disk files so drillIn + loadSelectedFile populate preview content via
// catalog.ReadFileContent.
func newLibraryWithDiskItems(t *testing.T, w, h int) (libraryModel, string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"alpha-skill", "beta-skill"} {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		for _, f := range []string{"SKILL.md", "notes.md"} {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("# "+name+"/"+f+"\nLine 2\nLine 3\n"), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
		}
	}
	items := []catalog.ContentItem{
		{Name: "alpha-skill", Type: catalog.Skills, Source: "library",
			Files: []string{"SKILL.md", "notes.md"},
			Path:  filepath.Join(root, "skills", "alpha-skill")},
		{Name: "beta-skill", Type: catalog.Skills, Source: "library",
			Files: []string{"SKILL.md", "notes.md"},
			Path:  filepath.Join(root, "skills", "beta-skill")},
	}
	l := newLibraryModel(items, nil, root)
	l.SetSize(w, h)
	return l, root
}

func TestLibrary_SetDetailFocus_TogglesFocusFields(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	l.drillIn(&l.table.items[0])

	l.setDetailFocus(panePreview)
	if l.focus != panePreview || l.tree.focused || !l.preview.focused {
		t.Errorf("setDetailFocus(panePreview): focus=%d tree.focused=%v preview.focused=%v",
			l.focus, l.tree.focused, l.preview.focused)
	}

	l.setDetailFocus(paneItems)
	if l.focus != paneItems || !l.tree.focused || l.preview.focused {
		t.Errorf("setDetailFocus(paneItems): focus=%d tree.focused=%v preview.focused=%v",
			l.focus, l.tree.focused, l.preview.focused)
	}
}

func TestLibrary_UpdateTree_KeysMoveCursor(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	l.drillIn(&l.table.items[0])
	start := l.tree.cursor

	l, _ = l.updateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if l.tree.cursor == start {
		t.Errorf("expected tree cursor to advance on 'j', still %d", l.tree.cursor)
	}
	l, _ = l.updateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if l.tree.cursor != start {
		t.Errorf("expected 'k' to restore cursor to %d, got %d", start, l.tree.cursor)
	}
}

func TestLibrary_UpdateTree_EnterOnDirectoryToggles(t *testing.T) {
	t.Parallel()
	// Add nested files so the tree has a directory to toggle.
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "nested-skill", "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.md"), []byte("child"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	item := catalog.ContentItem{
		Name: "nested-skill", Type: catalog.Skills, Source: "library",
		Files: []string{"sub/child.md"},
		Path:  filepath.Join(root, "skills", "nested-skill"),
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, root)
	l.SetSize(120, 30)
	l.drillIn(&l.table.items[0])

	// Find the directory node
	dirIdx := -1
	for i, n := range l.tree.nodes {
		if n.isDir {
			dirIdx = i
			break
		}
	}
	if dirIdx < 0 {
		t.Fatal("expected a directory node")
	}
	l.tree.cursor = dirIdx
	before := len(l.tree.nodes)
	l, _ = l.updateTree(tea.KeyMsg{Type: tea.KeyEnter})
	if len(l.tree.nodes) == before {
		t.Errorf("expected ToggleDir to change node count, still %d", before)
	}
}

func TestLibrary_UpdatePreviewKeys_ScrollsPreview(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 10)
	l.drillIn(&l.table.items[0])
	// Force a scrollable preview by making many lines.
	long := make([]string, 100)
	for i := range long {
		long[i] = "line " + itoa(i)
	}
	l.preview.lines = long
	l.preview.SetSize(60, 5)
	l.setDetailFocus(panePreview)

	startOffset := l.preview.offset
	l, _ = l.updatePreviewKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if l.preview.offset == startOffset {
		t.Errorf("expected preview offset to advance on j, still %d", l.preview.offset)
	}
	l, _ = l.updatePreviewKeys(tea.KeyMsg{Type: tea.KeyPgDown})
	// After PageDown offset should be larger or clamped at max.
	l, _ = l.updatePreviewKeys(tea.KeyMsg{Type: tea.KeyPgUp})
	l, _ = l.updatePreviewKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if l.preview.offset < 0 {
		t.Errorf("preview offset went negative: %d", l.preview.offset)
	}
}

func TestLibrary_ViewDetail_ContainsFilename(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 140, 30)
	l.drillIn(&l.table.items[0])
	out := ansi.Strip(l.viewDetail())
	if !strings.Contains(out, "SKILL.md") {
		t.Errorf("expected viewDetail to contain SKILL.md, got:\n%s", out)
	}
}

func TestLibrary_RenderPreviewBody_EmptyShowsHint(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	l.drillIn(&l.table.items[0])
	l.preview.lines = nil
	out := ansi.Strip(l.renderPreviewBody(10, 40))
	if !strings.Contains(out, "Select a file to preview") {
		t.Errorf("expected empty-state hint, got:\n%s", out)
	}
}

func TestLibrary_RenderPreviewBody_ShowsAboveBelowIndicators(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	l.drillIn(&l.table.items[0])
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "row-" + itoa(i)
	}
	l.preview.lines = lines
	l.preview.offset = 10 // scroll partway

	out := ansi.Strip(l.renderPreviewBody(10, 40))
	if !strings.Contains(out, "more above") {
		t.Errorf("expected \"more above\" indicator, got:\n%s", out)
	}
	if !strings.Contains(out, "more below") {
		t.Errorf("expected \"more below\" indicator, got:\n%s", out)
	}
}

func TestLibrary_UpdateMouse_WheelScrollsTable(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	start := l.table.cursor
	l, _ = l.updateMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if l.table.cursor == start {
		t.Errorf("expected wheel down to advance cursor, still %d", l.table.cursor)
	}
	l, _ = l.updateMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if l.table.cursor != start {
		t.Errorf("expected wheel up to restore cursor to %d, got %d", start, l.table.cursor)
	}
}

func TestLibrary_UpdateMouse_DetailWheelScrollsPreview(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	l.drillIn(&l.table.items[0])
	l.setDetailFocus(panePreview)
	// Populate preview with enough lines to scroll.
	lines := make([]string, 80)
	for i := range lines {
		lines[i] = "row"
	}
	l.preview.lines = lines
	l.preview.SetSize(40, 10)

	start := l.preview.offset
	l, _ = l.updateMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if l.preview.offset == start {
		t.Errorf("expected wheel down in preview pane to scroll, still %d", l.preview.offset)
	}
}

func TestLibrary_UpdateMouse_DetailWheelScrollsTreeWhenItemsFocus(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	l.drillIn(&l.table.items[0])
	l.setDetailFocus(paneItems)

	start := l.tree.cursor
	l, _ = l.updateMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if l.tree.cursor == start && len(l.tree.nodes) > 1 {
		t.Errorf("expected wheel down in tree pane to scroll tree, still %d", l.tree.cursor)
	}
}

func TestLibrary_UpdateMouse_IgnoresNonLeftNonWheel(t *testing.T) {
	t.Parallel()
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	// MouseActionMotion with left — shouldn't trigger anything.
	startCursor := l.table.cursor
	l, cmd := l.updateMouse(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion})
	if cmd != nil {
		t.Errorf("expected nil cmd on motion mouse, got %v", cmd)
	}
	if l.table.cursor != startCursor {
		t.Errorf("motion mouse changed cursor %d -> %d", startCursor, l.table.cursor)
	}
}
