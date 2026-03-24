// cli/internal/tui/filebrowser_test.go
package tui_v1

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestFileBrowserNavigation(t *testing.T) {
	// Set up a test directory structure
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "alpha"), 0755)
	os.MkdirAll(filepath.Join(tmp, "beta"), 0755)
	os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0644)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 80
	fb.height = 30

	// Should have 4 entries: .., alpha/, beta/, file.txt
	if len(fb.entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(fb.entries))
	}
	if fb.entries[0].name != ".." {
		t.Fatalf("first entry should be '..', got %q", fb.entries[0].name)
	}
	if !fb.entries[1].isDir || fb.entries[1].name != "alpha" {
		t.Fatalf("second entry should be dir 'alpha', got %q (isDir=%v)", fb.entries[1].name, fb.entries[1].isDir)
	}

	// Navigate down to alpha, then Enter to go into it
	down := tea.KeyMsg{Type: tea.KeyDown}
	fb, _ = fb.Update(down)
	if fb.cursor != 1 {
		t.Fatalf("after Down, expected cursor 1, got %d", fb.cursor)
	}

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	fb, _ = fb.Update(enter)
	if fb.currentDir != filepath.Join(tmp, "alpha") {
		t.Fatalf("expected to be in alpha dir, got %q", fb.currentDir)
	}

	// Backspace goes back up
	backspace := tea.KeyMsg{Type: tea.KeyBackspace}
	fb, _ = fb.Update(backspace)
	if fb.currentDir != tmp {
		t.Fatalf("after backspace, expected %q, got %q", tmp, fb.currentDir)
	}
}

func TestFileBrowserView(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "my-skill"), 0755)
	os.WriteFile(filepath.Join(tmp, "my-skill", "SKILL.md"), []byte("---\nname: test\n---\n"), 0644)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 80
	fb.height = 30

	view := fb.View()
	if !strings.Contains(view, "my-skill") {
		t.Fatal("view should show 'my-skill' directory")
	}
}

func TestFileBrowserDKeyConfirms(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0644)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 80
	fb.height = 30

	// Select file.txt (Down past "..", then Space to select)
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyDown})
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeySpace})

	if fb.SelectionCount() != 1 {
		t.Fatalf("expected 1 selected, got %d", fb.SelectionCount())
	}

	// 'd' confirms
	_, cmd := fb.Update(keyRune('d'))
	if cmd == nil {
		t.Fatal("expected fileBrowserDoneMsg command from 'd'")
	}
}

func TestFileBrowserCKeyDoesNotConfirm(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0644)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 80
	fb.height = 30

	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyDown})
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeySpace})

	// 'c' should NOT confirm anymore
	_, cmd := fb.Update(keyRune('c'))
	if cmd != nil {
		t.Fatal("'c' should NOT trigger confirm in file browser")
	}
}

func TestFileBrowserSelectAllUsesKeyMatches(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "one.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(tmp, "two.txt"), []byte("2"), 0644)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 80
	fb.height = 30

	// 'a' selects all non-".." entries
	fb, _ = fb.Update(keyRune('a'))
	if fb.SelectionCount() != 2 {
		t.Fatalf("expected 2 selected after 'a', got %d", fb.SelectionCount())
	}
}

func TestFileBrowserHelpTextShowsDone(t *testing.T) {
	// File browser help text is now shown in the global footer via importModel.helpText().
	// Verify the import model (stepBrowse) returns the expected help text.
	m := importModel{step: stepBrowse}
	helpText := m.helpText()
	if !strings.Contains(helpText, "d done") {
		t.Fatalf("help text should show 'd done', got: %q", helpText)
	}
	if strings.Contains(helpText, "c confirm") {
		t.Fatal("help text should no longer show 'c confirm'")
	}
}

func TestFileBrowserSkipsNodeModules(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "node_modules"), 0755)
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)

	fb := newFileBrowser(tmp, catalog.Skills)

	for _, entry := range fb.entries {
		if entry.name == "node_modules" {
			t.Fatal("node_modules should be skipped in file browser")
		}
		if entry.name == ".git" {
			t.Fatal(".git should be skipped in file browser")
		}
	}
	// src should still appear
	found := false
	for _, entry := range fb.entries {
		if entry.name == "src" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'src' to still appear")
	}
}

func TestFileBrowserScrollShowsCounts(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 50; i++ {
		os.MkdirAll(filepath.Join(tmp, fmt.Sprintf("dir-%02d", i)), 0755)
	}

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 80
	fb.height = 10 // small height forces scrolling

	view := fb.View()
	assertContains(t, view, "more below")
	// Should show a number
	assertNotContains(t, view, "(more items below)")
}

func TestFileBrowserNoEmoji(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0644)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 80
	fb.height = 30

	view := fb.View()

	assertNotContains(t, view, "📁")
	assertNotContains(t, view, "📄")
	assertNotContains(t, view, "📂")

	// Directories should have "/" suffix
	assertContains(t, view, "subdir/")
}

// ---------------------------------------------------------------------------
// SelectedPaths tests
// ---------------------------------------------------------------------------

func TestSelectedPaths_Empty(t *testing.T) {
	fb := fileBrowserModel{selected: make(map[string]bool)}
	paths := fb.SelectedPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestSelectedPaths_Sorted(t *testing.T) {
	fb := fileBrowserModel{
		selected: map[string]bool{
			"/z/file.txt": true,
			"/a/file.txt": true,
			"/m/file.txt": true,
		},
	}
	paths := fb.SelectedPaths()
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
	if paths[0] != "/a/file.txt" {
		t.Errorf("first path should be /a/file.txt, got %s", paths[0])
	}
	if paths[2] != "/z/file.txt" {
		t.Errorf("last path should be /z/file.txt, got %s", paths[2])
	}
}

// ---------------------------------------------------------------------------
// viewPreview tests
// ---------------------------------------------------------------------------

func TestViewPreview_BasicContent(t *testing.T) {
	fb := fileBrowserModel{
		previewName:    "test.go",
		previewPath:    "/tmp/test.go",
		previewContent: "package main\n\nfunc main() {\n}\n",
		selected:       make(map[string]bool),
		width:          60,
		height:         30,
	}
	got := stripANSI(fb.viewPreview())
	if !strings.Contains(got, "test.go") {
		t.Error("should contain filename")
	}
	if !strings.Contains(got, "package main") {
		t.Error("should contain file content")
	}
}

func TestViewPreview_SelectedFile(t *testing.T) {
	fb := fileBrowserModel{
		previewName:    "test.go",
		previewPath:    "/tmp/test.go",
		previewContent: "content",
		selected:       map[string]bool{"/tmp/test.go": true},
		width:          60,
		height:         30,
	}
	got := stripANSI(fb.viewPreview())
	if !strings.Contains(got, "selected") {
		t.Error("should show 'selected' for selected file")
	}
}

func TestViewPreview_ScrollIndicators(t *testing.T) {
	lines := ""
	for i := 0; i < 50; i++ {
		lines += fmt.Sprintf("line %d\n", i)
	}
	fb := fileBrowserModel{
		previewName:    "big.txt",
		previewPath:    "/tmp/big.txt",
		previewContent: lines,
		previewOffset:  5,
		selected:       make(map[string]bool),
		width:          60,
		height:         20,
	}
	got := stripANSI(fb.viewPreview())
	if !strings.Contains(got, "above") {
		t.Error("should show scroll-up indicator")
	}
	if !strings.Contains(got, "below") {
		t.Error("should show scroll-down indicator")
	}
}
