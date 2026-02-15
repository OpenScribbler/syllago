// cli/internal/tui/filebrowser_test.go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
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
	if !strings.Contains(view, "📁") {
		t.Fatal("view should contain directory icon 📁")
	}
	if !strings.Contains(view, "my-skill") {
		t.Fatal("view should show 'my-skill' directory")
	}
}
