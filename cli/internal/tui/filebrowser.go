// cli/internal/tui/filebrowser.go
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

// fileBrowserEntry represents a single entry (file or directory) in the browser.
type fileBrowserEntry struct {
	name      string
	path      string // absolute path
	isDir     bool
	detection string // content detection result, e.g. "Skill", ".md"
}

// fileBrowserModel provides filesystem navigation with multi-select and inline content detection.
type fileBrowserModel struct {
	currentDir     string
	entries        []fileBrowserEntry
	cursor         int
	offset         int // scroll offset
	selected       map[string]bool
	detectionCache map[string]string // path → detection result
	contentType    catalog.ContentType
	width, height  int
	errMsg         string // error message for permission denied, etc.
}

// fileBrowserDoneMsg is sent when the user confirms their selection.
type fileBrowserDoneMsg struct {
	paths []string
}

// newFileBrowser creates a file browser starting at the given directory.
func newFileBrowser(dir string, ct catalog.ContentType) fileBrowserModel {
	fb := fileBrowserModel{
		currentDir:     dir,
		selected:       make(map[string]bool),
		detectionCache: make(map[string]string),
		contentType:    ct,
	}
	fb.loadDir(dir)
	return fb
}

// loadDir reads the given directory and populates entries with detection results.
func (fb *fileBrowserModel) loadDir(dir string) {
	fb.currentDir = dir
	fb.cursor = 0
	fb.offset = 0
	fb.errMsg = ""

	osEntries, err := os.ReadDir(dir)
	if err != nil {
		fb.entries = nil
		fb.errMsg = "Cannot read directory: " + err.Error()
		return
	}

	var dirs, files []fileBrowserEntry

	for _, e := range osEntries {
		// Skip . and .. (handled separately) but show other dotfiles/dotdirs
		// since config dirs like .claude/, .config/ are common import sources
		if e.Name() == "." || e.Name() == ".." {
			continue
		}
		absPath := filepath.Join(dir, e.Name())
		entry := fileBrowserEntry{
			name:  e.Name(),
			path:  absPath,
			isDir: e.IsDir(),
		}
		// Run detection (check cache first)
		if det, ok := fb.detectionCache[absPath]; ok {
			entry.detection = det
		} else {
			det, _ := catalog.DetectContent(absPath)
			fb.detectionCache[absPath] = det
			entry.detection = det
		}

		if e.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	// Build final list: .. first, then dirs, then files
	fb.entries = nil
	if dir != "/" {
		fb.entries = append(fb.entries, fileBrowserEntry{
			name:  "..",
			path:  filepath.Dir(dir),
			isDir: true,
		})
	}
	fb.entries = append(fb.entries, dirs...)
	fb.entries = append(fb.entries, files...)
}

// SelectedPaths returns the list of selected absolute paths.
func (fb fileBrowserModel) SelectedPaths() []string {
	var paths []string
	for p := range fb.selected {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// SelectionCount returns the number of selected items.
func (fb fileBrowserModel) SelectionCount() int {
	return len(fb.selected)
}

// fbKeys holds file-browser-specific key bindings that don't belong in
// the global keyMap (they only apply inside the file browser).
var fbKeys = struct {
	SelectAll key.Binding
	Done      key.Binding
}{
	SelectAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
	Done:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
}

func (fb fileBrowserModel) Update(msg tea.Msg) (fileBrowserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if fb.cursor > 0 {
				fb.cursor--
			}
			fb.adjustScroll()

		case key.Matches(msg, keys.Down):
			if fb.cursor < len(fb.entries)-1 {
				fb.cursor++
			}
			fb.adjustScroll()

		case msg.Type == tea.KeyBackspace:
			parent := filepath.Dir(fb.currentDir)
			if parent != fb.currentDir {
				fb.loadDir(parent)
			}

		case key.Matches(msg, keys.Space):
			if fb.cursor >= 0 && fb.cursor < len(fb.entries) {
				entry := fb.entries[fb.cursor]
				if entry.name != ".." {
					if fb.selected[entry.path] {
						delete(fb.selected, entry.path)
					} else {
						fb.selected[entry.path] = true
					}
				}
			}

		case key.Matches(msg, keys.Enter):
			if len(fb.entries) == 0 {
				return fb, nil
			}
			entry := fb.entries[fb.cursor]
			if entry.isDir {
				fb.loadDir(entry.path)
			} else {
				// Toggle selection for files
				if fb.selected[entry.path] {
					delete(fb.selected, entry.path)
				} else {
					fb.selected[entry.path] = true
				}
			}

		case key.Matches(msg, fbKeys.SelectAll):
			// Select all non-".." entries
			for _, entry := range fb.entries {
				if entry.name != ".." {
					fb.selected[entry.path] = true
				}
			}

		case key.Matches(msg, fbKeys.Done):
			// Confirm selection
			if len(fb.selected) > 0 {
				return fb, func() tea.Msg {
					return fileBrowserDoneMsg{paths: fb.SelectedPaths()}
				}
			}
		}
	}
	return fb, nil
}

// adjustScroll keeps the cursor visible within the viewport.
func (fb *fileBrowserModel) adjustScroll() {
	visibleRows := fb.visibleRows()
	if visibleRows <= 0 {
		return
	}
	if fb.cursor < fb.offset {
		fb.offset = fb.cursor
	}
	if fb.cursor >= fb.offset+visibleRows {
		fb.offset = fb.cursor - visibleRows + 1
	}
}

func (fb fileBrowserModel) visibleRows() int {
	// Reserve lines for: header, path, blank, footer, blank
	rows := fb.height - 6
	if rows < 1 {
		rows = len(fb.entries)
	}
	return rows
}

func (fb fileBrowserModel) View() string {
	s := helpStyle.Render(fb.currentDir) + "\n\n"

	if fb.errMsg != "" {
		s += errorMsgStyle.Render(fb.errMsg) + "\n"
		return s
	}

	if len(fb.entries) == 0 {
		s += helpStyle.Render("  (empty directory)") + "\n"
		return s
	}

	visibleRows := fb.visibleRows()
	end := fb.offset + visibleRows
	if end > len(fb.entries) {
		end = len(fb.entries)
	}

	if fb.offset > 0 {
		s += helpStyle.Render("  (more items above)") + "\n"
	}

	for i := fb.offset; i < end; i++ {
		entry := fb.entries[i]
		prefix := "   "
		style := itemStyle
		if i == fb.cursor {
			prefix = " > "
			style = selectedItemStyle
		}

		// Selection indicator
		sel := " "
		if fb.selected[entry.path] {
			sel = "x"
		}

		// Directory indicator: append "/" to dir names
		name := entry.name
		if entry.isDir && entry.name != ".." {
			name += "/"
		}
		if entry.name == ".." {
			sel = " " // can't select ..
		}

		line := prefix + sel + " " + style.Render(StripControlChars(name))

		// Detection tag
		if entry.detection != "" && entry.name != ".." {
			tag := ""
			switch entry.detection {
			case "Skill", "Agent", "Prompt", "App":
				tag = installedStyle.Render("[x] " + entry.detection)
			default:
				tag = countStyle.Render("(" + entry.detection + ")")
			}
			line += " " + tag
		}

		s += line + "\n"
	}

	if end < len(fb.entries) {
		s += helpStyle.Render("  (more items below)") + "\n"
	}

	// Footer
	s += "\n"
	selCount := len(fb.selected)
	if selCount > 0 {
		s += helpStyle.Render(fmt.Sprintf("  %d selected", selCount)) + "\n"
	}
	s += helpStyle.Render("up/down navigate • enter open dir • space select • a select all • d done • esc parent dir")

	return s
}
