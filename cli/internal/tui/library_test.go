package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

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

func TestLibrary_DrillIn_UnstagedRegistryItem_ShowsExplanation(t *testing.T) {
	t.Parallel()
	// MOAT-materialized items have empty Path (cache holds only manifest +
	// signature.bundle, not the content tree). Without the unstaged-item
	// branch in loadSelectedFile the preview pane silently rendered nothing,
	// which read as a bug.
	item := catalog.ContentItem{
		Name:   "syllago-guide",
		Type:   catalog.Skills,
		Source: "OpenScribbler/syllago-meta-registry",
		// Path and Files intentionally empty — that's the materialization tell.
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(120, 30)
	l.drillIn(&l.table.items[0])

	if l.preview.fileName != "(not staged)" {
		t.Errorf("expected preview filename '(not staged)' for unstaged registry item, got %q", l.preview.fileName)
	}
	body := strings.Join(l.preview.lines, "\n")
	if !strings.Contains(body, "OpenScribbler/syllago-meta-registry") {
		t.Errorf("expected preview body to name the source registry, got: %q", body)
	}
	if !strings.Contains(body, "[i]") {
		t.Errorf("expected preview body to point user at [i] install, got: %q", body)
	}
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

// --- Browse-mode mouse tests: meta bar buttons emit routed messages ---
// These are sequential because bubblezone uses a global singleton.

func TestLibrary_UpdateMouse_BrowseMetaInstallEmitsMsg(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	// Render to register zones
	scanZones(l.View())
	z := zone.Get("meta-install")
	if z.IsZero() {
		t.Skip("zone meta-install not registered")
	}
	_, cmd := l.updateMouse(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("click on meta-install should emit cmd")
	}
	if _, ok := cmd().(libraryInstallMsg); !ok {
		t.Errorf("expected libraryInstallMsg, got %T", cmd())
	}
}

func TestLibrary_UpdateMouse_BrowseMetaEditEmitsMsg(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	scanZones(l.View())
	z := zone.Get("meta-edit")
	if z.IsZero() {
		t.Skip("zone meta-edit not registered")
	}
	_, cmd := l.updateMouse(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("click on meta-edit should emit cmd")
	}
	if _, ok := cmd().(libraryEditMsg); !ok {
		t.Errorf("expected libraryEditMsg, got %T", cmd())
	}
}

func TestLibrary_UpdateMouse_BrowseMetaRemoveEmitsMsg(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 120, 30)
	scanZones(l.View())
	z := zone.Get("meta-remove")
	if z.IsZero() {
		t.Skip("zone meta-remove not registered")
	}
	_, cmd := l.updateMouse(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("click on meta-remove should emit cmd")
	}
	if _, ok := cmd().(libraryRemoveMsg); !ok {
		t.Errorf("expected libraryRemoveMsg, got %T", cmd())
	}
}

func TestLibrary_UpdateMouse_BrowseColumnHeaderSortsTable(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 140, 30)
	scanZones(l.View())
	z := zone.Get("col-type")
	if z.IsZero() {
		t.Skip("zone col-type not registered")
	}
	before := l.table.sortCol
	updated, _ := l.updateMouse(mouseClick(z.StartX, z.StartY))
	l = updated
	if l.table.sortCol == before && before != sortByType {
		t.Errorf("click on col-type should set sortCol to sortByType, got %v", l.table.sortCol)
	}
}

func TestLibrary_UpdateMouse_BrowseRowClickMovesCursor(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 140, 30)
	scanZones(l.View())
	z := zone.Get("tbl-1")
	if z.IsZero() {
		t.Skip("zone tbl-1 not registered")
	}
	l.table.cursor = 0
	updated, _ := l.updateMouse(mouseClick(z.StartX, z.StartY))
	l = updated
	if l.table.cursor != 1 {
		t.Errorf("click on tbl-1 should move cursor to 1, got %d", l.table.cursor)
	}
}

func TestLibrary_UpdateMouse_BrowseRowDoubleClickDrillsIn(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 140, 30)
	scanZones(l.View())
	z := zone.Get("tbl-0")
	if z.IsZero() {
		t.Skip("zone tbl-0 not registered")
	}
	// Cursor already at 0 — clicking again should drill in.
	l.table.cursor = 0
	updated, cmd := l.updateMouse(mouseClick(z.StartX, z.StartY))
	l = updated
	if l.mode != libraryDetail {
		t.Errorf("double-click should switch to libraryDetail, got mode=%v", l.mode)
	}
	if cmd == nil {
		t.Fatal("double-click should emit libraryDrillMsg")
	}
	if _, ok := cmd().(libraryDrillMsg); !ok {
		t.Errorf("expected libraryDrillMsg, got %T", cmd())
	}
}

func TestLibrary_UpdateMouse_DetailFileTreeNodeClickLoadsFile(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 140, 30)
	l.drillIn(&l.table.items[0])
	scanZones(l.View())
	// Click a non-first tree node — ftnode-0 is already selected, go for ftnode-1.
	z := zone.Get("ftnode-1")
	if z.IsZero() {
		t.Skip("zone ftnode-1 not registered")
	}
	l.tree.cursor = 0
	updated, _ := l.updateMouse(mouseClick(z.StartX, z.StartY))
	l = updated
	if l.tree.cursor != 1 {
		t.Errorf("click on ftnode-1 should move tree cursor to 1, got %d", l.tree.cursor)
	}
}

func TestLibrary_UpdateMouse_DetailTreePaneFocusesItems(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 140, 30)
	l.drillIn(&l.table.items[0])
	l.setDetailFocus(panePreview)
	scanZones(l.View())
	z := zone.Get("lib-tree")
	if z.IsZero() {
		t.Skip("zone lib-tree not registered")
	}
	// Click away from any file-tree node by targeting the bottom-right of the pane.
	// Use end-of-zone coords so we miss individual ftnode entries.
	updated, _ := l.updateMouse(mouseClick(z.EndX, z.EndY))
	l = updated
	if l.focus != paneItems {
		t.Errorf("click on lib-tree pane should focus items, got %v", l.focus)
	}
}

func TestLibrary_UpdateMouse_DetailPreviewPaneFocusesPreview(t *testing.T) {
	l, _ := newLibraryWithDiskItems(t, 140, 30)
	l.drillIn(&l.table.items[0])
	l.setDetailFocus(paneItems)
	scanZones(l.View())
	z := zone.Get("lib-preview")
	if z.IsZero() {
		t.Skip("zone lib-preview not registered")
	}
	updated, _ := l.updateMouse(mouseClick(z.EndX, z.EndY))
	l = updated
	if l.focus != panePreview {
		t.Errorf("click on lib-preview pane should focus preview, got %v", l.focus)
	}
}
