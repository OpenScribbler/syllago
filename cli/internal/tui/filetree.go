package tui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// fileTreeModel renders a navigable file tree for a content item's files.
type fileTreeModel struct {
	nodes    []fileNode // flat list of visible nodes
	allNodes []fileNode // all nodes (including collapsed children)
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
}

// fileNode represents a file or directory in the tree.
type fileNode struct {
	name     string // display name (just the filename, not full path)
	path     string // relative path from item root
	isDir    bool
	expanded bool // only for directories
	depth    int  // indentation level
}

func newFileTreeModel(files []string) fileTreeModel {
	m := fileTreeModel{}
	m.allNodes = buildTree(files)
	m.rebuildVisible()
	return m
}

// SetSize updates dimensions.
func (m *fileTreeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SelectedPath returns the relative path of the selected file, or "" if a directory.
func (m fileTreeModel) SelectedPath() string {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return ""
	}
	n := m.nodes[m.cursor]
	if n.isDir {
		return ""
	}
	return n.path
}

// CursorUp moves cursor up.
func (m *fileTreeModel) CursorUp() {
	if len(m.nodes) == 0 {
		return
	}
	m.cursor--
	if m.cursor < 0 {
		m.cursor = len(m.nodes) - 1
		m.offset = max(0, len(m.nodes)-m.height)
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
}

// CursorDown moves cursor down.
func (m *fileTreeModel) CursorDown() {
	if len(m.nodes) == 0 {
		return
	}
	m.cursor++
	if m.cursor >= len(m.nodes) {
		m.cursor = 0
		m.offset = 0
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
}

// ToggleDir expands/collapses the directory at the cursor.
func (m *fileTreeModel) ToggleDir() {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return
	}
	n := &m.nodes[m.cursor]
	if !n.isDir {
		return
	}
	// Find the corresponding node in allNodes and toggle it
	for i := range m.allNodes {
		if m.allNodes[i].path == n.path && m.allNodes[i].isDir {
			m.allNodes[i].expanded = !m.allNodes[i].expanded
			break
		}
	}
	m.rebuildVisible()
	// Clamp cursor
	if m.cursor >= len(m.nodes) {
		m.cursor = max(0, len(m.nodes)-1)
	}
}

// rebuildVisible rebuilds the visible node list from allNodes, respecting collapsed dirs.
func (m *fileTreeModel) rebuildVisible() {
	m.nodes = make([]fileNode, 0, len(m.allNodes))
	skipDepth := -1

	for _, n := range m.allNodes {
		// Skip children of collapsed directories
		if skipDepth >= 0 && n.depth > skipDepth {
			continue
		}
		skipDepth = -1

		m.nodes = append(m.nodes, n)

		// If this is a collapsed directory, skip its children
		if n.isDir && !n.expanded {
			skipDepth = n.depth
		}
	}
}

// View renders the file tree.
func (m fileTreeModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if len(m.nodes) == 0 {
		return mutedStyle.Render("(no files)")
	}

	lines := make([]string, 0, m.height)
	visibleCount := min(m.height, len(m.nodes))
	lastVisible := min(m.offset+visibleCount, len(m.nodes))

	for i := m.offset; i < lastVisible; i++ {
		lines = append(lines, m.renderNode(i))
	}

	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}

	return strings.Join(lines, "\n")
}

// renderNode renders a single tree node.
func (m fileTreeModel) renderNode(index int) string {
	n := m.nodes[index]
	isCursor := index == m.cursor

	indent := strings.Repeat("  ", n.depth)
	icon := "  "
	if n.isDir {
		if n.expanded {
			icon = "▾ "
		} else {
			icon = "▸ "
		}
	}

	text := indent + icon + n.name
	maxW := m.width
	if len(text) > maxW {
		text = text[:maxW]
	}

	var row string
	if isCursor && m.focused {
		row = selectedRowStyle.Width(m.width).Render(text)
	} else if isCursor {
		row = boldStyle.Width(m.width).Render(text)
	} else if n.isDir {
		row = lipgloss.NewStyle().Width(m.width).Foreground(primaryColor).Render(text)
	} else {
		row = lipgloss.NewStyle().Width(m.width).Render(text)
	}
	return zone.Mark("ftnode-"+itoa(index), row)
}

// buildTree converts a flat list of relative file paths into a tree structure.
func buildTree(files []string) []fileNode {
	if len(files) == 0 {
		return nil
	}

	sort.Strings(files)

	// Collect unique directories and files
	type entry struct {
		path  string
		isDir bool
		depth int
	}
	seen := make(map[string]bool)
	var entries []entry

	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")

		// Add directory entries for each intermediate path
		for i := 0; i < len(parts)-1; i++ {
			dirPath := strings.Join(parts[:i+1], "/")
			if !seen[dirPath] {
				seen[dirPath] = true
				entries = append(entries, entry{dirPath, true, i})
			}
		}

		// Add the file itself
		entries = append(entries, entry{f, false, len(parts) - 1})
	}

	// Convert to nodes — directories expanded by default
	nodes := make([]fileNode, len(entries))
	for i, e := range entries {
		name := filepath.Base(e.path)
		if e.isDir {
			name += "/"
		}
		nodes[i] = fileNode{
			name:     name,
			path:     e.path,
			isDir:    e.isDir,
			expanded: true,
			depth:    e.depth,
		}
	}
	return nodes
}
