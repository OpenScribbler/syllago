package tui_v1

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// fileViewerModel groups the file viewer state for the Files tab.
// It wraps splitViewModel and handles file-specific logic (loading content, primary file).
type fileViewerModel struct {
	splitView splitViewModel
	itemPath  string // absolute path to the content item directory
}

const maxPreviewLines = 200

// buildFileTree converts a flat list of relative file paths into splitViewItems
// with directory grouping and indentation.
func buildFileTree(files []string) []splitViewItem {
	if len(files) == 0 {
		return nil
	}

	// Sort files for consistent ordering
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	// Build tree structure: track directories we've already emitted
	emittedDirs := make(map[string]bool)
	var items []splitViewItem

	for _, f := range sorted {
		parts := strings.Split(filepath.ToSlash(f), "/")
		// Emit parent directories as disabled entries
		for i := 0; i < len(parts)-1; i++ {
			dirPath := strings.Join(parts[:i+1], "/")
			if !emittedDirs[dirPath] {
				emittedDirs[dirPath] = true
				items = append(items, splitViewItem{
					Label:    parts[i],
					IsDir:    true,
					Indent:   i,
					Disabled: true, // directories are visual-only
				})
			}
		}
		// Emit the file
		indent := len(parts) - 1
		items = append(items, splitViewItem{
			Label:  parts[len(parts)-1],
			Path:   f, // relative path
			Indent: indent,
		})
	}

	return items
}

// primaryFileIndex returns the index of the primary file for the given content type.
// Returns 0 if no match found (first file is the default).
func primaryFileIndex(items []splitViewItem, ct catalog.ContentType) int {
	for i, item := range items {
		if item.Disabled || item.IsDir {
			continue
		}
		name := strings.ToLower(item.Label)
		switch ct {
		case catalog.Skills:
			if name == "skill.md" {
				return i
			}
		case catalog.Agents:
			if strings.HasSuffix(name, ".md") {
				return i
			}
		case catalog.Rules:
			if strings.HasSuffix(name, ".md") {
				return i
			}
		case catalog.Hooks:
			if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				return i
			}
		case catalog.MCP:
			if strings.HasSuffix(name, ".json") {
				return i
			}
		case catalog.Commands:
			// First non-disabled file
			return i
		case catalog.Loadouts:
			if name == "loadout.yaml" || name == "loadout.yml" {
				return i
			}
		}
	}
	// Default: first non-disabled item
	for i, item := range items {
		if !item.Disabled {
			return i
		}
	}
	return 0
}

// newFileViewer creates a file viewer for the given content item.
func newFileViewer(item catalog.ContentItem) fileViewerModel {
	items := buildFileTree(item.Files)
	sv := newSplitView(items, "sv-files")

	fv := fileViewerModel{
		splitView: sv,
		itemPath:  item.Path,
	}

	// Set cursor to primary file
	if len(items) > 0 {
		idx := primaryFileIndex(items, item.Type)
		fv.splitView.cursor = idx
		fv.splitView.adjustScroll()
		// Load initial preview
		fv.loadPreview(idx)
	}

	return fv
}

// loadPreview loads file content for the item at the given index.
func (fv *fileViewerModel) loadPreview(index int) {
	if index < 0 || index >= len(fv.splitView.items) {
		fv.splitView.SetPreview("")
		return
	}
	item := fv.splitView.items[index]
	if item.Disabled || item.IsDir || item.Path == "" {
		fv.splitView.SetPreview("")
		return
	}

	absPath := filepath.Join(fv.itemPath, item.Path)
	data, err := os.ReadFile(absPath)
	if err != nil {
		fv.splitView.SetPreview(fmt.Sprintf("Cannot read file: %s", err))
		return
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > maxPreviewLines {
		content = strings.Join(lines[:maxPreviewLines], "\n")
		content += fmt.Sprintf("\n\n(%d more lines)", len(lines)-maxPreviewLines)
	}

	fv.splitView.SetPreview(content)
}
