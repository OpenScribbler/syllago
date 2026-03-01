// cli/internal/tui/filebrowser.go
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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

	// File preview state (inline viewer, same pattern as detail fileViewerModel)
	previewContent string
	previewName    string // filename shown in title
	previewPath    string // absolute path of previewed file
	previewOffset  int
	previewing     bool
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

// skipDirs lists directory names that are never useful as import sources.
var skipDirs = map[string]bool{
	"node_modules":  true,
	".git":          true,
	"__pycache__":   true,
	".venv":         true,
	"venv":          true,
	"target":        true, // Rust
	"vendor":        true, // Go
	".tox":          true,
	".mypy_cache":   true,
	".pytest_cache": true,
	"dist":          true,
	"build":         true,
}

const maxBrowserEntries = 500

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

	if len(osEntries) > maxBrowserEntries {
		fb.errMsg = fmt.Sprintf("Directory has %d entries (showing first %d). Navigate into subdirectories for better performance.", len(osEntries), maxBrowserEntries)
		osEntries = osEntries[:maxBrowserEntries]
	}

	var dirs, files []fileBrowserEntry

	for _, e := range osEntries {
		// Skip . and .. (handled separately) but show other dotfiles/dotdirs
		// since config dirs like .claude/, .config/ are common import sources
		if e.Name() == "." || e.Name() == ".." {
			continue
		}
		if e.IsDir() && skipDirs[e.Name()] {
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

const maxPreviewBytes = 100 * 1024 // 100KB cap for file previews

func (fb fileBrowserModel) Update(msg tea.Msg) (fileBrowserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Preview mode: handle scroll and exit before normal browser keys
		if fb.previewing {
			switch {
			case key.Matches(msg, keys.Back):
				fb.previewing = false
				fb.previewContent = ""
				fb.previewOffset = 0
			case key.Matches(msg, keys.Up):
				if fb.previewOffset > 0 {
					fb.previewOffset--
				}
			case key.Matches(msg, keys.Down):
				fb.previewOffset++
			case key.Matches(msg, keys.PageUp):
				pageSize := fb.height - 7
				if pageSize < 1 {
					pageSize = 10
				}
				fb.previewOffset -= pageSize
				if fb.previewOffset < 0 {
					fb.previewOffset = 0
				}
			case key.Matches(msg, keys.PageDown):
				pageSize := fb.height - 7
				if pageSize < 1 {
					pageSize = 10
				}
				fb.previewOffset += pageSize
			case key.Matches(msg, keys.Space):
				// Toggle selection on the previewed file
				if fb.selected[fb.previewPath] {
					delete(fb.selected, fb.previewPath)
				} else {
					fb.selected[fb.previewPath] = true
				}
			}
			return fb, nil // swallow all other keys while previewing
		}

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
				// Preview file content
				data, err := os.ReadFile(entry.path)
				if err != nil {
					fb.errMsg = "Cannot read file: " + err.Error()
					return fb, nil
				}
				content := string(data)
				if len(data) > maxPreviewBytes {
					content = string(data[:maxPreviewBytes]) + "\n\n(truncated at 100KB)"
				}
				fb.previewContent = content
				fb.previewName = entry.name
				fb.previewPath = entry.path
				fb.previewOffset = 0
				fb.previewing = true
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

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			if fb.previewing {
				if fb.previewOffset > 0 {
					fb.previewOffset--
				}
			} else {
				if fb.cursor > 0 {
					fb.cursor--
				}
				fb.adjustScroll()
			}
		}
		if msg.Button == tea.MouseButtonWheelDown {
			if fb.previewing {
				fb.previewOffset++
			} else {
				if fb.cursor < len(fb.entries)-1 {
					fb.cursor++
				}
				fb.adjustScroll()
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
	if fb.previewing {
		return fb.viewPreview()
	}

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
		s += helpStyle.Render(fmt.Sprintf("  (%d more above)", fb.offset)) + "\n"
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
			sel = installedStyle.Render("✓")
			if i != fb.cursor {
				style = installedStyle
			}
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
				tag = installedStyle.Render("[✓] " + entry.detection)
			default:
				tag = countStyle.Render("(" + entry.detection + ")")
			}
			line += " " + tag
		}

		s += line + "\n"
	}

	if end < len(fb.entries) {
		s += helpStyle.Render(fmt.Sprintf("  (%d more below)", len(fb.entries)-end)) + "\n"
	}

	// Footer
	s += "\n"
	selCount := len(fb.selected)
	if selCount > 0 {
		s += helpStyle.Render(fmt.Sprintf("  %d selected", selCount)) + "\n"
	}
	s += helpStyle.Render("up/down navigate • enter open/preview • space select • a select all • d done • esc back")

	return s
}

// viewPreview renders the file content preview with line numbers and scroll.
func (fb fileBrowserModel) viewPreview() string {
	s := labelStyle.Render(fb.previewName) + "\n\n"

	lines := strings.Split(fb.previewContent, "\n")

	// Visible area: height minus title(1) + blank(1) + scroll indicators(2) + footer(3)
	visibleHeight := fb.height - 7
	if visibleHeight < 5 {
		visibleHeight = len(lines)
	}

	// Clamp scroll offset
	maxOffset := len(lines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := fb.previewOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	if offset > 0 {
		s += helpStyle.Render(fmt.Sprintf("(%d lines above)", offset)) + "\n"
	}

	for i := offset; i < end; i++ {
		lineNum := helpStyle.Render(fmt.Sprintf("%4d ", i+1))
		s += lineNum + valueStyle.Render(StripControlChars(lines[i])) + "\n"
	}

	if end < len(lines) {
		s += helpStyle.Render(fmt.Sprintf("(%d lines below)", len(lines)-end)) + "\n"
	}

	// Selection status and help
	s += "\n"
	if fb.selected[fb.previewPath] {
		s += installedStyle.Render("  ✓ selected") + "\n"
	}
	s += helpStyle.Render("esc back • up/down scroll • space select")

	return s
}
