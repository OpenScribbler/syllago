package tui

import (
	"testing"
)

func TestFileTree_SelectPath_MovesCursor(t *testing.T) {
	t.Parallel()
	m := newFileTreeModel([]string{
		"README.md",
		"docs/intro.md",
		"docs/api.md",
		"pkg/main.go",
	})
	m.SetSize(40, 10)

	m.SelectPath("docs/api.md")
	if m.SelectedPath() != "docs/api.md" {
		t.Errorf("expected selected path docs/api.md, got %q", m.SelectedPath())
	}

	// Missing path is a no-op.
	prev := m.cursor
	m.SelectPath("nonexistent/file.go")
	if m.cursor != prev {
		t.Errorf("SelectPath missing should be no-op, cursor moved %d -> %d", prev, m.cursor)
	}
}

func TestFileTree_SelectPath_DirectorySkippedByPath(t *testing.T) {
	t.Parallel()
	m := newFileTreeModel([]string{
		"a/one.md",
		"a/two.md",
	})
	m.SetSize(40, 10)
	// Selecting a directory path still moves the cursor there, but SelectedPath
	// returns "" because SelectedPath only returns files.
	m.SelectPath("a")
	if sp := m.SelectedPath(); sp != "" {
		t.Errorf("SelectedPath on directory should return \"\", got %q", sp)
	}
}

func TestFileTree_SelectPath_ScrollsOffsetIntoView(t *testing.T) {
	t.Parallel()
	files := make([]string, 30)
	for i := range files {
		files[i] = "f" + itoa(i) + ".txt"
	}
	m := newFileTreeModel(files)
	m.SetSize(40, 5) // small viewport forces offset math

	m.SelectPath("f29.txt")
	if m.cursor < m.offset || m.cursor >= m.offset+m.height {
		t.Errorf("cursor %d not within viewport [%d, %d)", m.cursor, m.offset, m.offset+m.height)
	}
}

func TestFileTree_ToggleDir_CollapsesAndExpands(t *testing.T) {
	t.Parallel()
	m := newFileTreeModel([]string{
		"docs/intro.md",
		"docs/api.md",
		"README.md",
	})
	m.SetSize(40, 10)

	initialCount := len(m.nodes)
	// Find the "docs" directory node and move the cursor there.
	dirIdx := -1
	for i, n := range m.nodes {
		if n.isDir && n.path == "docs" {
			dirIdx = i
			break
		}
	}
	if dirIdx < 0 {
		t.Fatal("expected docs directory node")
	}
	m.cursor = dirIdx
	m.ToggleDir()
	collapsedCount := len(m.nodes)
	if collapsedCount >= initialCount {
		t.Errorf("expected fewer visible nodes after collapsing docs, got %d -> %d", initialCount, collapsedCount)
	}

	// Expand again.
	m.ToggleDir()
	if len(m.nodes) != initialCount {
		t.Errorf("expected %d nodes after re-expanding, got %d", initialCount, len(m.nodes))
	}
}

func TestFileTree_ToggleDir_OnFileIsNoop(t *testing.T) {
	t.Parallel()
	m := newFileTreeModel([]string{
		"README.md",
		"LICENSE",
	})
	m.SetSize(40, 10)

	// Cursor=0 after construction; with only files, that's a file node.
	before := len(m.nodes)
	m.cursor = 0
	m.ToggleDir()
	if len(m.nodes) != before {
		t.Errorf("ToggleDir on a file changed node count %d -> %d", before, len(m.nodes))
	}
}

func TestFileTree_ToggleDir_OutOfBoundsCursor(t *testing.T) {
	t.Parallel()
	m := newFileTreeModel([]string{"README.md"})
	m.SetSize(40, 10)
	m.cursor = 99
	// Should not panic.
	m.ToggleDir()
	if m.cursor != 99 {
		t.Errorf("out-of-bounds ToggleDir mutated cursor: got %d", m.cursor)
	}
}
