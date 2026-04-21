package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// newExplorerWithDiskItems returns an explorerModel whose items reference
// real files on disk under t.TempDir(). Needed because drillIn() + loadSelected-
// File() call catalog.ReadFileContent which opens the file — without real
// paths the preview shows an "Error reading file" line rather than exercising
// the success branch.
func newExplorerWithDiskItems(t *testing.T, w, h int) (explorerModel, string) {
	t.Helper()
	root := t.TempDir()

	// Two items, each with two files, so the tree has branching to exercise.
	for _, item := range []struct {
		name  string
		files []string
	}{
		{"alpha-skill", []string{"SKILL.md", "notes.md"}},
		{"beta-skill", []string{"SKILL.md", "notes.md"}},
	} {
		dir := filepath.Join(root, "skills", item.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		for _, f := range item.files {
			content := "# " + item.name + " / " + f + "\n\nLine 2\nLine 3\n"
			if err := os.WriteFile(filepath.Join(dir, f), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", f, err)
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
	e := newExplorerModel(items, false, nil, root)
	e.SetSize(w, h)
	return e, root
}

// --- Pure unit tests (no mouse, safe for t.Parallel) -------------------------

func TestExplorer_MetaBarTotal_IsLinesPlusSeparator(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	got := e.metaBarTotal()
	want := e.metaBarLines() + 1
	if got != want {
		t.Errorf("metaBarTotal = %d, want metaBarLines(%d)+1 = %d",
			got, e.metaBarLines(), want)
	}
}

func TestExplorer_DetailTreeWidth_Responsive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		width int
		want  int
	}{
		{120, 35}, // wide — fixed 35
		{150, 35}, // wider — still fixed 35
		{100, 30}, // narrow — 30% of width = 30
		{80, 24},  // 80 * 30 / 100 = 24
		{50, 22},  // clamped to min 22 (50*30/100=15 < 22)
	}
	for _, tc := range cases {
		e, _ := newExplorerWithDiskItems(t, tc.width, 30)
		if got := e.detailTreeWidth(); got != tc.want {
			t.Errorf("detailTreeWidth(width=%d) = %d, want %d", tc.width, got, tc.want)
		}
	}
}

func TestExplorer_DrillIn_EntersDetailMode(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 120, 30)
	item := &e.items.items[0]

	e.drillIn(item)

	if e.mode != explorerDetail {
		t.Errorf("mode = %d, want explorerDetail (%d)", e.mode, explorerDetail)
	}
	if e.detailItem != item {
		t.Error("detailItem should equal the argument")
	}
	if len(e.tree.nodes) == 0 {
		t.Error("tree should have nodes after drillIn")
	}
	if !e.tree.focused {
		t.Error("tree should be focused after drillIn")
	}
	if e.preview.focused {
		t.Error("preview should NOT be focused after drillIn")
	}
	if e.focus != paneItems {
		t.Errorf("focus = %d, want paneItems (%d)", e.focus, paneItems)
	}
	// loadSelectedFile should have populated preview.lines with real content.
	if len(e.preview.lines) == 0 {
		t.Error("preview.lines should be populated after drillIn + loadSelectedFile")
	}
}

// TestExplorer_LoadSelectedFile_NilDetailItemIsNoop pins the early-return at
// explorer.go:569 so a refactor that forgets the nil guard panics loudly.
func TestExplorer_LoadSelectedFile_NilDetailItemIsNoop(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.detailItem = nil
	// Must not panic. preview state must remain untouched.
	origLines := e.preview.lines
	origFile := e.preview.fileName
	e.loadSelectedFile()
	if len(e.preview.lines) != len(origLines) {
		t.Errorf("preview.lines should be unchanged with nil detailItem, got len %d", len(e.preview.lines))
	}
	if e.preview.fileName != origFile {
		t.Errorf("preview.fileName should be unchanged with nil detailItem, got %q", e.preview.fileName)
	}
}

// TestExplorer_LoadSelectedFile_FallsBackToPrimary pins the SelectedPath=""
// branch at explorer.go:572-579. When the tree cursor is on a directory (or
// at an empty position), the function falls back to catalog.PrimaryFileName.
func TestExplorer_LoadSelectedFile_FallsBackToPrimary(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	item := &e.items.items[0]
	e.drillIn(item)

	// drillIn already calls loadSelectedFile — preview should have content.
	if len(e.preview.lines) == 0 {
		t.Fatal("expected preview populated by drillIn")
	}
	// Primary file for a skill is SKILL.md.
	if !strings.Contains(e.preview.fileName, "SKILL.md") {
		t.Errorf("preview.fileName should be SKILL.md (primary for skill), got %q", e.preview.fileName)
	}
}

func TestExplorer_SetDetailFocus_SwitchesFocusFlags(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.drillIn(&e.items.items[0])

	// Initially paneItems (tree focused).
	if !e.tree.focused || e.preview.focused {
		t.Fatal("expected tree focused and preview not focused after drillIn")
	}

	e.setDetailFocus(panePreview)
	if e.focus != panePreview {
		t.Errorf("focus = %d, want panePreview", e.focus)
	}
	if e.tree.focused {
		t.Error("tree should NOT be focused when setDetailFocus(panePreview)")
	}
	if !e.preview.focused {
		t.Error("preview SHOULD be focused when setDetailFocus(panePreview)")
	}

	e.setDetailFocus(paneItems)
	if !e.tree.focused || e.preview.focused {
		t.Error("setDetailFocus(paneItems) should re-focus the tree")
	}
}

// --- Key handler tests -------------------------------------------------------

func TestExplorer_UpdateDetailKeys_EscReturnsToBrowse(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.drillIn(&e.items.items[0])

	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if e.mode != explorerBrowse {
		t.Errorf("mode = %d, want explorerBrowse (%d) after Esc", e.mode, explorerBrowse)
	}
	if e.detailItem != nil {
		t.Error("detailItem should be cleared after Esc")
	}
	if cmd == nil {
		t.Fatal("Esc from detail should emit explorerCloseMsg")
	}
	if _, ok := cmd().(explorerCloseMsg); !ok {
		t.Errorf("expected explorerCloseMsg, got %T", cmd())
	}
}

func TestExplorer_UpdateDetailKeys_XAlsoCloses(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.drillIn(&e.items.items[0])

	e, _ = e.Update(keyRune('x'))
	if e.mode != explorerBrowse {
		t.Errorf("mode = %d, want explorerBrowse after x", e.mode)
	}
}

func TestExplorer_UpdateDetailKeys_LeftRightSwitchFocus(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.drillIn(&e.items.items[0])

	// Start at paneItems (tree). Right -> preview.
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRight})
	if e.focus != panePreview {
		t.Errorf("Right: focus = %d, want panePreview", e.focus)
	}

	// Left -> back to items.
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if e.focus != paneItems {
		t.Errorf("Left: focus = %d, want paneItems", e.focus)
	}
}

func TestExplorer_UpdateTreeKeys_NavigatesAndLoadsFiles(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.drillIn(&e.items.items[0])
	startCursor := e.tree.cursor
	startFile := e.preview.fileName

	// Down must advance cursor (if there's somewhere to go).
	if len(e.tree.nodes) > 1 {
		e, _ = e.Update(tea.KeyMsg{Type: tea.KeyDown})
		if e.tree.cursor == startCursor {
			t.Errorf("Down did not move cursor from %d", startCursor)
		}
		// preview should reload to match new cursor (or stay on same file
		// if cursor lands on a directory — either way loadSelectedFile ran).
		if e.preview.fileName == "" && startFile != "" {
			t.Error("Down should have loaded a file (or at least not cleared the preview)")
		}
	}

	// Up should move back.
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyUp})
	if e.tree.cursor != startCursor {
		t.Errorf("Up did not return to start cursor %d, got %d", startCursor, e.tree.cursor)
	}

	// PgDown + PgUp should also be handled (no panic, cursor stays within bounds).
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if e.tree.cursor < 0 || e.tree.cursor >= len(e.tree.nodes) {
		t.Errorf("PgDown produced out-of-range cursor %d (nodes %d)",
			e.tree.cursor, len(e.tree.nodes))
	}
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if e.tree.cursor < 0 || e.tree.cursor >= len(e.tree.nodes) {
		t.Errorf("PgUp produced out-of-range cursor %d", e.tree.cursor)
	}
}

func TestExplorer_UpdateTreeKeys_EnterOnFileLoadsFile(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.drillIn(&e.items.items[0])

	// Find a file node (not dir) and move cursor to it.
	for i, n := range e.tree.nodes {
		if !n.isDir {
			e.tree.cursor = i
			break
		}
	}

	e.preview.lines = nil // clear to observe reload
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(e.preview.lines) == 0 {
		t.Error("Enter on file should reload preview")
	}
}

func TestExplorer_UpdateDetailPreviewKeys_ScrollsPreview(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.drillIn(&e.items.items[0])
	e.setDetailFocus(panePreview)

	// Pre-populate with lots of lines to enable scrolling.
	e.preview.lines = make([]string, 50)
	for i := range e.preview.lines {
		e.preview.lines[i] = "line " + itoa(i)
	}
	e.preview.SetSize(40, 10)

	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyDown})
	if e.preview.offset != 1 {
		t.Errorf("Down from preview focus: offset = %d, want 1", e.preview.offset)
	}

	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyUp})
	if e.preview.offset != 0 {
		t.Errorf("Up: offset = %d, want 0", e.preview.offset)
	}

	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if e.preview.offset == 0 {
		t.Error("PgDown should advance offset from 0")
	}
	prev := e.preview.offset
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if e.preview.offset >= prev {
		t.Errorf("PgUp should reduce offset below %d, got %d", prev, e.preview.offset)
	}
}

func TestExplorer_UpdatePreview_StackedEscReturnsToItems(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 60, 20) // stacked (width < 80)
	e.setFocus(panePreview)
	if !e.stacked {
		t.Fatal("expected stacked at width 60")
	}

	e, _ = e.updatePreview(tea.KeyMsg{Type: tea.KeyEsc})
	if e.focus != paneItems {
		t.Errorf("Esc from preview in stacked mode: focus = %d, want paneItems", e.focus)
	}
}

func TestExplorer_UpdatePreview_ScrollKeys(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.preview.lines = make([]string, 30)
	for i := range e.preview.lines {
		e.preview.lines[i] = "line"
	}
	e.preview.SetSize(40, 10)

	e, _ = e.updatePreview(tea.KeyMsg{Type: tea.KeyDown})
	if e.preview.offset != 1 {
		t.Errorf("Down: offset = %d, want 1", e.preview.offset)
	}
	e, _ = e.updatePreview(tea.KeyMsg{Type: tea.KeyPgDown})
	if e.preview.offset <= 1 {
		t.Error("PgDown should move offset past 1")
	}
	e, _ = e.updatePreview(tea.KeyMsg{Type: tea.KeyPgUp})
	e, _ = e.updatePreview(tea.KeyMsg{Type: tea.KeyUp})
	if e.preview.offset != 0 {
		t.Errorf("PgUp+Up: offset = %d, want 0", e.preview.offset)
	}
}

// --- View tests --------------------------------------------------------------

func TestExplorer_ViewDetail_RendersTreeAndPreview(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 120, 30)
	e.drillIn(&e.items.items[0])

	view := e.View()
	if view == "" {
		t.Fatal("viewDetail should not be empty")
	}
	// File names from the tree should appear.
	if !strings.Contains(view, "SKILL.md") {
		t.Error("detail view should contain SKILL.md from the tree")
	}
	// Close button.
	if !strings.Contains(view, "Close") {
		t.Error("detail view should contain the Close button")
	}
	// File count caption.
	if !strings.Contains(view, "files)") {
		t.Error("detail view should contain '(N files)' caption")
	}
}

// TestExplorer_RenderDetailPreviewBody_EmptyShowsPlaceholder pins the no-lines
// branch at explorer.go:837-844.
func TestExplorer_RenderDetailPreviewBody_EmptyShowsPlaceholder(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.preview.lines = nil

	body := e.renderDetailPreviewBody(10, 40)
	if !strings.Contains(body, "Select a file to preview") {
		t.Errorf("empty body should show placeholder, got: %q", body)
	}
}

// TestExplorer_RenderDetailPreviewBody_ShowsOverflowIndicators pins the
// (N more above) / (N more below) branches at explorer.go:868-870 and 880-882.
func TestExplorer_RenderDetailPreviewBody_ShowsOverflowIndicators(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.preview.lines = make([]string, 40)
	for i := range e.preview.lines {
		e.preview.lines[i] = "content-line-" + itoa(i)
	}
	e.preview.offset = 10 // scrolled past top

	body := e.renderDetailPreviewBody(10, 60)
	if !strings.Contains(body, "more above") {
		t.Errorf("offset=10 should surface 'more above' indicator, got: %q", body)
	}
	if !strings.Contains(body, "more below") {
		t.Errorf("40 lines in viewport of 10 should surface 'more below' indicator, got: %q", body)
	}
}

func TestExplorer_ViewStacked_RendersAtNarrowWidth(t *testing.T) {
	t.Parallel()
	e, _ := newExplorerWithDiskItems(t, 60, 20)
	if !e.stacked {
		t.Fatal("expected stacked=true at width=60")
	}

	view := e.View()
	if view == "" {
		t.Fatal("stacked view should not be empty")
	}
	// Items name should appear (since focus starts at paneItems).
	if !strings.Contains(view, "alpha-skill") {
		t.Error("stacked view in items focus should contain item names")
	}

	// Switch focus to preview — view should swap to preview content.
	e.setFocus(panePreview)
	viewPrev := e.View()
	if viewPrev == "" {
		t.Fatal("stacked preview view should not be empty")
	}
}

// --- Mouse tests (NOT t.Parallel — bubblezone global state) -----------------

// TestExplorer_UpdateBrowseMouse_ItemRowSelects pins the item-{i} click handler
// at explorer.go:367-380. The first click on an unselected row should move
// cursor without drilling in.
func TestExplorer_UpdateBrowseMouse_ItemRowSelects(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	scanZones(e.View())

	z := zone.Get("item-1")
	if z.IsZero() {
		t.Skip("zone item-1 not registered")
	}
	origCursor := e.items.cursor
	e, cmd := e.Update(mouseClick(z.StartX, z.StartY))
	if e.items.cursor == origCursor {
		t.Errorf("click on item-1 should change cursor from %d", origCursor)
	}
	if e.mode == explorerDetail {
		t.Error("first click should not drill in")
	}
	if cmd == nil {
		t.Error("item row click should emit itemSelectedCmd")
	}
}

// TestExplorer_UpdateBrowseMouse_SecondClickDrills pins the same-row second-
// click drill branch at explorer.go:369-374.
func TestExplorer_UpdateBrowseMouse_SecondClickDrills(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	// Move cursor to row 1 before scanning so the second click lands on it.
	e.items.cursor = 1
	scanZones(e.View())

	z := zone.Get("item-1")
	if z.IsZero() {
		t.Skip("zone item-1 not registered")
	}
	e, cmd := e.Update(mouseClick(z.StartX, z.StartY))
	if e.mode != explorerDetail {
		t.Errorf("second click should drill into detail mode, mode = %d", e.mode)
	}
	if cmd == nil {
		t.Fatal("drill should emit explorerDrillMsg")
	}
	if _, ok := cmd().(explorerDrillMsg); !ok {
		t.Errorf("expected explorerDrillMsg, got %T", cmd())
	}
}

// TestExplorer_UpdateBrowseMouse_WheelScrollsItems pins the wheel branch at
// explorer.go:402-406 for the items pane.
func TestExplorer_UpdateBrowseMouse_WheelScrollsItems(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 100, 30)
	e.items.cursor = 0
	scanZones(e.View())

	z := zone.Get("pane-items")
	if z.IsZero() {
		t.Skip("zone pane-items not registered")
	}
	origCursor := e.items.cursor
	e, _ = e.Update(tea.MouseMsg{
		X: z.StartX + 1, Y: z.StartY + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	if e.items.cursor == origCursor {
		t.Errorf("wheel-down over items pane should advance cursor from %d", origCursor)
	}
	// Wheel up should back out.
	e, _ = e.Update(tea.MouseMsg{
		X: z.StartX + 1, Y: z.StartY + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	if e.items.cursor != origCursor {
		t.Errorf("wheel-up should return cursor to %d, got %d", origCursor, e.items.cursor)
	}
}

// TestExplorer_UpdateDetailMouse_CloseButton pins exp-close zone at
// explorer.go:446-452. Clicking the close button must exit detail mode and
// emit explorerCloseMsg.
func TestExplorer_UpdateDetailMouse_CloseButton(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 120, 30)
	e.drillIn(&e.items.items[0])
	scanZones(e.View())

	z := zone.Get("exp-close")
	if z.IsZero() {
		t.Skip("zone exp-close not registered")
	}
	e, cmd := e.Update(mouseClick(z.StartX, z.StartY))
	if e.mode != explorerBrowse {
		t.Errorf("close click: mode = %d, want explorerBrowse", e.mode)
	}
	if cmd == nil {
		t.Fatal("close should emit explorerCloseMsg")
	}
	if _, ok := cmd().(explorerCloseMsg); !ok {
		t.Errorf("expected explorerCloseMsg, got %T", cmd())
	}
}

// TestExplorer_UpdateDetailMouse_TreeNodeClicksMoveCursor pins ftnode-{i}
// zone loop at explorer.go:455-466.
func TestExplorer_UpdateDetailMouse_TreeNodeClicksMoveCursor(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 120, 30)
	e.drillIn(&e.items.items[0])
	scanZones(e.View())

	// Pick a non-dir node to verify preview reload.
	var targetIdx = -1
	for i, n := range e.tree.nodes {
		if !n.isDir && i != e.tree.cursor {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		t.Skip("no alternate file node to click")
	}

	z := zone.Get("ftnode-" + itoa(targetIdx))
	if z.IsZero() {
		t.Skipf("zone ftnode-%d not registered", targetIdx)
	}
	e, _ = e.Update(mouseClick(z.StartX, z.StartY))
	if e.tree.cursor != targetIdx {
		t.Errorf("click on ftnode-%d: cursor = %d, want %d",
			targetIdx, e.tree.cursor, targetIdx)
	}
}

// TestExplorer_UpdateDetailMouse_WheelInPaneItemsMovesTree pins the focus=
// paneItems wheel branch at explorer.go:472-477.
func TestExplorer_UpdateDetailMouse_WheelInPaneItemsMovesTree(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 120, 30)
	e.drillIn(&e.items.items[0])
	// Drop anywhere; wheel branch only checks msg.Action+Button, not zone.
	origTreeCursor := e.tree.cursor
	e, _ = e.Update(tea.MouseMsg{
		X: 1, Y: 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	if len(e.tree.nodes) > 1 && e.tree.cursor == origTreeCursor {
		t.Errorf("wheel-down in detail (items focus) should move tree cursor from %d", origTreeCursor)
	}
}

// TestExplorer_UpdateDetailMouse_WheelInPanePreviewScrollsPreview pins the
// else-branch at explorer.go:483-484 — wheel when preview is focused scrolls
// the preview instead of the tree.
func TestExplorer_UpdateDetailMouse_WheelInPanePreviewScrollsPreview(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 120, 30)
	e.drillIn(&e.items.items[0])
	e.setDetailFocus(panePreview)
	// Pad preview so scrolling is meaningful.
	e.preview.lines = make([]string, 50)
	for i := range e.preview.lines {
		e.preview.lines[i] = "L"
	}
	e.preview.SetSize(40, 10)

	origOffset := e.preview.offset
	e, _ = e.Update(tea.MouseMsg{
		X: 1, Y: 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	if e.preview.offset == origOffset {
		t.Errorf("wheel-down with preview focused should advance offset from %d", origOffset)
	}
}

// TestExplorer_UpdateMouse_RoutesByMode pins the top-level Update router at
// explorer.go:180-181 and updateMouse at explorer.go:342-347. Same MouseMsg
// must take different branches depending on e.mode.
func TestExplorer_UpdateMouse_RoutesByMode(t *testing.T) {
	e, _ := newExplorerWithDiskItems(t, 120, 30)
	// Browse mode: wheel over items pane should change items cursor.
	scanZones(e.View())
	if z := zone.Get("pane-items"); !z.IsZero() {
		e, _ = e.Update(tea.MouseMsg{
			X: z.StartX + 1, Y: z.StartY + 1,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		})
		if e.items.cursor == 0 && len(e.items.items) > 1 {
			t.Error("browse-mode wheel-down should advance items cursor")
		}
	}

	// Detail mode: same wheel should move tree cursor instead.
	e.drillIn(&e.items.items[0])
	origTreeCursor := e.tree.cursor
	e, _ = e.Update(tea.MouseMsg{
		X: 1, Y: 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	if len(e.tree.nodes) > 1 && e.tree.cursor == origTreeCursor {
		t.Error("detail-mode wheel-down should move tree cursor (not items)")
	}
}
