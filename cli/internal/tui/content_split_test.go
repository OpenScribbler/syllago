package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestSplitModel_RendersFileTreeAndPreview(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"SKILL.md", "helpers.md", "utils/tool.md"},
		"SKILL.md",
		"---\ntitle: Alpha Skill\n---",
	)
	m.width = 80
	m.height = 10

	view := m.View()

	// Left pane should show file tree title and files
	if !strings.Contains(view, "Files") {
		t.Error("expected 'Files' title in left pane")
	}
	if !strings.Contains(view, "SKILL.md") {
		t.Error("expected SKILL.md in file tree")
	}
	if !strings.Contains(view, "helpers.md") {
		t.Error("expected helpers.md in file tree")
	}

	// Right pane should show preview title and content
	if !strings.Contains(view, "Preview: SKILL.md") {
		t.Error("expected preview title")
	}
	if !strings.Contains(view, "Alpha Skill") {
		t.Error("expected preview content")
	}

	// Vertical separator should appear
	if !strings.Contains(view, "\u2502") {
		t.Error("expected vertical separator between panes")
	}
}

func TestSplitModel_FocusSwitching(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"file1.md", "file2.md"},
		"file1.md",
		"content",
	)
	m.width = 80
	m.height = 10

	// Starts with left focus
	if !m.focusLeft {
		t.Error("expected initial focus on left pane")
	}

	// Press Right to focus preview
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusLeft {
		t.Error("expected focus to shift to right pane after Right key")
	}

	// Press Left to focus file tree
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if !m.focusLeft {
		t.Error("expected focus to shift to left pane after Left key")
	}

	// Also test h/l keys
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.focusLeft {
		t.Error("expected focus to shift to right pane after 'l' key")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if !m.focusLeft {
		t.Error("expected focus to shift to left pane after 'h' key")
	}
}

func TestSplitModel_FileSelection(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"file1.md", "file2.md", "file3.md"},
		"file1.md",
		"content of file1",
	)
	m.width = 80
	m.height = 10

	// Initial cursor at 0
	if m.fileCursor != 0 {
		t.Errorf("expected initial fileCursor=0, got %d", m.fileCursor)
	}

	// Move down
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.fileCursor != 1 {
		t.Errorf("expected fileCursor=1 after Down, got %d", m.fileCursor)
	}
	// Should produce fileSelectedMsg
	if cmd == nil {
		t.Fatal("expected cmd after file selection")
	}
	msg := cmd()
	fsm, ok := msg.(fileSelectedMsg)
	if !ok {
		t.Fatalf("expected fileSelectedMsg, got %T", msg)
	}
	if fsm.filename != "file2.md" {
		t.Errorf("expected filename=file2.md, got %s", fsm.filename)
	}

	// Move up
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.fileCursor != 0 {
		t.Errorf("expected fileCursor=0 after Up, got %d", m.fileCursor)
	}
	msg = cmd()
	fsm = msg.(fileSelectedMsg)
	if fsm.filename != "file1.md" {
		t.Errorf("expected filename=file1.md, got %s", fsm.filename)
	}

	// Can't go above 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.fileCursor != 0 {
		t.Errorf("expected fileCursor=0 when already at top, got %d", m.fileCursor)
	}
}

func TestSplitModel_TabTogglesCompat(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"hook.yaml"},
		"hook.yaml",
		"event: before_tool",
	)
	m.compatData = []compatEntry{
		{provider: "Claude Code", status: "ok", note: "native"},
		{provider: "Gemini CLI", status: "partial", note: "partial"},
		{provider: "Cursor", status: "none", note: ""},
	}
	m.width = 80
	m.height = 12

	// Initially showing files
	if m.showCompat {
		t.Error("expected showCompat=false initially")
	}
	view := m.View()
	if !strings.Contains(view, "Files") {
		t.Error("expected 'Files' title initially")
	}

	// Tab to switch to compat
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.showCompat {
		t.Error("expected showCompat=true after Tab")
	}
	view = m.View()
	if !strings.Contains(view, "Compat") {
		t.Error("expected 'Compat' title after Tab")
	}

	// Tab again to switch back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.showCompat {
		t.Error("expected showCompat=false after second Tab")
	}
}

func TestSplitModel_CompatView(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"hook.yaml"},
		"hook.yaml",
		"event: before_tool",
	)
	m.compatData = []compatEntry{
		{provider: "Claude Code", status: "ok", note: "native"},
		{provider: "Gemini CLI", status: "partial", note: "partial"},
		{provider: "Cursor", status: "none", note: ""},
	}
	m.showCompat = true
	m.width = 80
	m.height = 15

	view := m.View()

	// Should show compat title
	if !strings.Contains(view, "Compat") {
		t.Error("expected 'Compat' title in compat view")
	}

	// Should show provider names
	if !strings.Contains(view, "Claude Code") {
		t.Error("expected 'Claude Code' in compat view")
	}
	if !strings.Contains(view, "Gemini CLI") {
		t.Error("expected 'Gemini CLI' in compat view")
	}
	if !strings.Contains(view, "Cursor") {
		t.Error("expected 'Cursor' in compat view")
	}

	// Should show status icons
	if !strings.Contains(view, "[ok]") {
		t.Error("expected [ok] status icon")
	}
	if !strings.Contains(view, "[~~]") {
		t.Error("expected [~~] status icon")
	}
	if !strings.Contains(view, "[--]") {
		t.Error("expected [--] status icon")
	}
}

func TestSplitModel_TabNoOpWithoutCompatData(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"file.md"},
		"file.md",
		"content",
	)
	m.width = 80
	m.height = 10

	// Tab should not toggle when no compat data
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.showCompat {
		t.Error("Tab should not toggle showCompat when compatData is empty")
	}
}

func TestSplitModel_EmptyFileList(t *testing.T) {
	t.Parallel()
	m := newSplitModel(nil, "", "")
	m.width = 80
	m.height = 10

	// Should not panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view even with empty file list")
	}
	if !strings.Contains(view, "Files") {
		t.Error("expected 'Files' title even with empty file list")
	}
	if !strings.Contains(view, "(no files)") {
		t.Error("expected '(no files)' placeholder")
	}
}

func TestSplitModel_ZeroSize(t *testing.T) {
	t.Parallel()
	m := newSplitModel([]string{"f.md"}, "f.md", "content")
	m.width = 0
	m.height = 0

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view for zero size, got %q", view)
	}
}

func TestSplitModel_PreviewScrollWhenFocusedRight(t *testing.T) {
	t.Parallel()
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line content"
	}
	m := newSplitModel(
		[]string{"big.txt"},
		"big.txt",
		strings.Join(lines, "\n"),
	)
	m.width = 80
	m.height = 10

	// Focus right pane
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Scroll down in preview
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.preview.scrollOffset != 1 {
		t.Errorf("expected preview scrollOffset=1, got %d", m.preview.scrollOffset)
	}

	// Scroll up in preview
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.preview.scrollOffset != 0 {
		t.Errorf("expected preview scrollOffset=0, got %d", m.preview.scrollOffset)
	}
}

func TestSplitModel_FileIndentation(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"top.md", "utils/helper.md", "utils/deep/nested.md"},
		"top.md",
		"content",
	)
	m.width = 80
	m.height = 10

	view := m.View()

	// Files with "/" should appear (indentation is visual, hard to test precisely,
	// but we can verify they appear in output)
	if !strings.Contains(view, "top.md") {
		t.Error("expected top.md in output")
	}
	if !strings.Contains(view, "utils/helper.md") {
		t.Error("expected utils/helper.md in output")
	}
	if !strings.Contains(view, "utils/deep/nested.md") {
		t.Error("expected utils/deep/nested.md in output")
	}
}

func TestSplitModel_WidthConstraints(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"SKILL.md"},
		"SKILL.md",
		"short content",
	)
	m.width = 60
	m.height = 8

	view := m.View()
	for i, line := range strings.Split(view, "\n") {
		w := lipgloss.Width(line)
		if w > m.width {
			t.Errorf("line %d exceeds width %d: got %d: %q", i, m.width, w, line)
		}
	}
}

func TestSplitModel_CursorRendering(t *testing.T) {
	t.Parallel()
	m := newSplitModel(
		[]string{"a.md", "b.md"},
		"a.md",
		"content",
	)
	m.width = 60
	m.height = 8

	view := m.View()
	// Selected file should have "> " prefix
	if !strings.Contains(view, "> ") {
		t.Error("expected '> ' cursor prefix for selected file")
	}
}
