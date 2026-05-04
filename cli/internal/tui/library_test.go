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

// --- Slice 1: Registry Clone items visible in Library tab ---

// testCatalogUnifiedList returns a catalog with one item from each category:
// Library item, Project Content, and Registry Clone (not yet Added).
func testCatalogUnifiedList(t *testing.T) *catalog.Catalog {
	t.Helper()
	return &catalog.Catalog{
		Items: []catalog.ContentItem{
			{
				Name:    "local-skill",
				Type:    catalog.Skills,
				Source:  "library",
				Library: true,
				Files:   []string{"SKILL.md"},
			},
			{
				Name:    "shared-rule",
				Type:    catalog.Rules,
				Source:  "project",
				Library: false,
				// Registry empty — Project Content
				Files: []string{"rule.md"},
			},
			{
				Name:     "registry-skill",
				Type:     catalog.Skills,
				Source:   "test-registry",
				Registry: "test-registry",
				Library:  false,
				Files:    []string{"SKILL.md"},
			},
		},
	}
}

// testAppWithUnifiedLibraryCatalog creates an App using the unified list
// catalog at the given dimensions, landed on the Library tab (default).
func testAppWithUnifiedLibraryCatalog(t *testing.T, w, h int) App {
	t.Helper()
	app := NewApp(testCatalogUnifiedList(t), testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(App)
}

// TestLibraryTable_RegistryCloneItemRenderedMuted verifies that a Registry
// Clone item (Registry != "" && Library == false) renders with a
// [not in library] chip in the Library table.
func TestLibraryTable_RegistryCloneItemRenderedMuted(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:     "guide-skill",
		Type:     catalog.Skills,
		Source:   "test-registry",
		Registry: "test-registry",
		Library:  false,
		Files:    []string{"SKILL.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 10)
	view := ansi.Strip(l.View())
	if !strings.Contains(view, "[not in library]") {
		t.Errorf("expected Registry Clone row to contain [not in library], view:\n%s", view)
	}
}

// TestLibraryTable_LibraryItemRenderedNormal verifies that a Library item
// (Library == true) does NOT render with a [not in library] chip.
func TestLibraryTable_LibraryItemRenderedNormal(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:    "local-skill",
		Type:    catalog.Skills,
		Source:  "library",
		Library: true,
		Files:   []string{"SKILL.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 10)
	view := ansi.Strip(l.View())
	if strings.Contains(view, "[not in library]") {
		t.Errorf("Library item must not render [not in library], view:\n%s", view)
	}
}

// TestLibraryTable_ProjectContentRenderedNormal verifies that a Project
// Content item (Library == false, Registry == "") does NOT render with a
// [not in library] chip.
func TestLibraryTable_ProjectContentRenderedNormal(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:    "shared-rule",
		Type:    catalog.Rules,
		Source:  "project",
		Library: false,
		// Registry intentionally empty — discriminates Project Content
		Files: []string{"rule.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 10)
	view := ansi.Strip(l.View())
	if strings.Contains(view, "[not in library]") {
		t.Errorf("Project Content item must not render [not in library], view:\n%s", view)
	}
}

// TestGoldenLibrary_UnifiedList verifies that the Library tab renders all three
// item categories — Library item, Project Content, and Registry Clone — in a
// single unified list. The Registry Clone row must contain [not in library].
func TestGoldenLibrary_UnifiedList(t *testing.T) {
	app := testAppWithUnifiedLibraryCatalog(t, 80, 30)
	view := snapshotApp(t, app)
	// Verify [not in library] appears somewhere in the unified list output.
	if !strings.Contains(view, "[not in library]") {
		t.Errorf("unified list golden must contain [not in library] for registry clone item, view:\n%s", view)
	}
	requireGolden(t, "library-unified-list-80x30", view)
}

// ── Slice 2: filter chips ──────────────────────────────────────────────────

// TestLibraryFilter_AllShowsEverything verifies the default (All) filter shows all items.
func TestLibraryFilter_AllShowsEverything(t *testing.T) {
	t.Parallel()
	l := newLibraryModel(testCatalogUnifiedList(t).Items, nil, "")
	l.SetSize(80, 20)
	if l.table.Len() != 3 {
		t.Errorf("filterAll: want 3 items, got %d", l.table.Len())
	}
}

// TestLibraryFilter_NotInLibraryNarrows verifies filterNotInLibrary shows only Registry Clone items.
func TestLibraryFilter_NotInLibraryNarrows(t *testing.T) {
	t.Parallel()
	l := newLibraryModel(testCatalogUnifiedList(t).Items, nil, "")
	l.SetSize(80, 20)
	l.setFilter(filterNotInLibrary)
	if l.table.Len() != 1 {
		t.Errorf("filterNotInLibrary: want 1 item, got %d", l.table.Len())
	}
	sel := l.table.Selected()
	if sel == nil || sel.Registry == "" || sel.Library {
		t.Errorf("filterNotInLibrary: selected item must be Registry Clone, got %v", sel)
	}
}

// TestLibraryFilter_InLibraryNarrows verifies filterInLibrary shows only Library items.
func TestLibraryFilter_InLibraryNarrows(t *testing.T) {
	t.Parallel()
	l := newLibraryModel(testCatalogUnifiedList(t).Items, nil, "")
	l.SetSize(80, 20)
	l.setFilter(filterInLibrary)
	if l.table.Len() != 1 {
		t.Errorf("filterInLibrary: want 1 item, got %d", l.table.Len())
	}
	sel := l.table.Selected()
	if sel == nil || !sel.Library {
		t.Errorf("filterInLibrary: selected must be Library item, got %v", sel)
	}
}

// TestLibraryFilter_ProjectNarrows verifies filterProject shows only Project Content items.
func TestLibraryFilter_ProjectNarrows(t *testing.T) {
	t.Parallel()
	l := newLibraryModel(testCatalogUnifiedList(t).Items, nil, "")
	l.SetSize(80, 20)
	l.setFilter(filterProject)
	if l.table.Len() != 1 {
		t.Errorf("filterProject: want 1 item, got %d", l.table.Len())
	}
	sel := l.table.Selected()
	if sel == nil || sel.Library || sel.Registry != "" {
		t.Errorf("filterProject: selected must be Project Content, got %v", sel)
	}
}

// TestLibraryFilter_ChipKeyActivates verifies pressing 'f' cycles the active filter chip.
func TestLibraryFilter_ChipKeyActivates(t *testing.T) {
	t.Parallel()
	l := newLibraryModel(testCatalogUnifiedList(t).Items, nil, "")
	l.SetSize(80, 20)
	// Default filter is filterAll. First 'f' press → filterInLibrary.
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if l.filter != filterInLibrary {
		t.Errorf("after one 'f': want filterInLibrary (%d), got %d", filterInLibrary, l.filter)
	}
	// Second press → filterNotInLibrary.
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if l.filter != filterNotInLibrary {
		t.Errorf("after two 'f': want filterNotInLibrary (%d), got %d", filterNotInLibrary, l.filter)
	}
}

// TestLibraryFilter_MouseChipClick verifies clicking the "Not in Library" chip zone
// activates filterNotInLibrary. Must not use t.Parallel — uses bubblezone singleton.
func TestLibraryFilter_MouseChipClick(t *testing.T) {
	l := newLibraryModel(testCatalogUnifiedList(t).Items, nil, "")
	l.SetSize(80, 20)
	scanZones(l.View())
	z := zone.Get("lib-filter-not-in-library")
	if z.IsZero() {
		t.Skip("zone lib-filter-not-in-library not registered")
	}
	l, _ = l.updateMouse(mouseClick(z.StartX, z.StartY))
	if l.filter != filterNotInLibrary {
		t.Errorf("after chip click: want filterNotInLibrary (%d), got %d", filterNotInLibrary, l.filter)
	}
}

// TestGoldenLibrary_FilterChipsRendered verifies the filter chip row is visible
// in the Library browse view and the golden output matches the baseline.
func TestGoldenLibrary_FilterChipsRendered(t *testing.T) {
	app := testAppWithUnifiedLibraryCatalog(t, 80, 30)
	view := snapshotApp(t, app)
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "All") {
		t.Errorf("filter chips must include 'All' chip, view:\n%s", stripped)
	}
	requireGolden(t, "library-filter-chips-80x30", view)
}

// ── Slice 3: metapanel Add/Add+Install buttons ─────────────────────────────

// TestMetapanel_AddButtonsForRegistryClone verifies that a Registry Clone item
// (Registry != "" && !Library) shows [a] Add and [i] Add + Install in the metapanel,
// and does NOT show [e] Edit.
func TestMetapanel_AddButtonsForRegistryClone(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:     "registry-skill",
		Type:     catalog.Skills,
		Registry: "test-registry",
		Library:  false,
		Files:    []string{"SKILL.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 20)
	view := ansi.Strip(l.View())
	if !strings.Contains(view, "[a] Add") {
		t.Errorf("Registry Clone metapanel must contain [a] Add, view:\n%s", view)
	}
	if !strings.Contains(view, "[i] Add + Install") {
		t.Errorf("Registry Clone metapanel must contain [i] Add + Install, view:\n%s", view)
	}
	if strings.Contains(view, "[e] Edit") {
		t.Errorf("Registry Clone metapanel must NOT contain [e] Edit, view:\n%s", view)
	}
}

// TestMetapanel_EditButtonForLibraryItem verifies that a Library item (Library == true)
// shows [e] Edit and does NOT show add buttons.
func TestMetapanel_EditButtonForLibraryItem(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:    "local-skill",
		Type:    catalog.Skills,
		Library: true,
		Files:   []string{"SKILL.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 20)
	view := ansi.Strip(l.View())
	if !strings.Contains(view, "[e] Edit") {
		t.Errorf("Library item metapanel must contain [e] Edit, view:\n%s", view)
	}
	if strings.Contains(view, "[a] Add") {
		t.Errorf("Library item metapanel must NOT contain [a] Add, view:\n%s", view)
	}
}

// TestMetapanel_NoAddButtonsForProjectContent verifies that Project Content
// (Registry == "" && !Library) shows neither [a] Add nor [i] Add + Install.
func TestMetapanel_NoAddButtonsForProjectContent(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:    "shared-rule",
		Type:    catalog.Rules,
		Library: false,
		// Registry intentionally empty — Project Content
		Files: []string{"rule.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 20)
	view := ansi.Strip(l.View())
	if strings.Contains(view, "[a] Add") {
		t.Errorf("Project Content metapanel must NOT contain [a] Add, view:\n%s", view)
	}
	if strings.Contains(view, "[i] Add + Install") {
		t.Errorf("Project Content metapanel must NOT contain [i] Add + Install, view:\n%s", view)
	}
}

// TestLibraryAddMsg_EmittedOnKeyA verifies that pressing 'a' on a Registry
// Clone item emits libraryAddMsg (not the add wizard).
func TestLibraryAddMsg_EmittedOnKeyA(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:     "registry-skill",
		Type:     catalog.Skills,
		Registry: "test-registry",
		Library:  false,
		Files:    []string{"SKILL.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 20)
	_, cmd := l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("pressing 'a' on Registry Clone must emit a cmd")
	}
	if _, ok := cmd().(libraryAddMsg); !ok {
		t.Errorf("pressing 'a' on Registry Clone must emit libraryAddMsg, got %T", cmd())
	}
}

// TestLibraryAddInstallMsg_EmittedOnKeyI verifies that pressing 'i' on a
// Registry Clone item emits libraryAddInstallMsg.
func TestLibraryAddInstallMsg_EmittedOnKeyI(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:     "registry-skill",
		Type:     catalog.Skills,
		Registry: "test-registry",
		Library:  false,
		Files:    []string{"SKILL.md"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, nil, "")
	l.SetSize(80, 20)
	_, cmd := l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd == nil {
		t.Fatal("pressing 'i' on Registry Clone must emit a cmd")
	}
	if _, ok := cmd().(libraryAddInstallMsg); !ok {
		t.Errorf("pressing 'i' on Registry Clone must emit libraryAddInstallMsg, got %T", cmd())
	}
}
