package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
)

// loadoutContentsModel wraps splitViewModel for the loadout Contents tab.
// It resolves manifest item references against the catalog to show
// grouped items with preview of their primary files.
type loadoutContentsModel struct {
	splitView splitViewModel
	// Resolved items indexed by split view position (nil = unresolved)
	resolvedItems []*catalog.ContentItem
}

// resolvedLoadoutItem pairs a manifest reference name with its resolved catalog item.
type resolvedLoadoutItem struct {
	name     string
	ct       catalog.ContentType
	resolved *catalog.ContentItem // nil if not found in catalog
}

// newLoadoutContents creates a content viewer for a loadout manifest.
// It resolves each manifest reference against the catalog and builds
// a split view with type group headers as disabled items.
func newLoadoutContents(manifest *loadout.Manifest, cat *catalog.Catalog) loadoutContentsModel {
	if manifest == nil {
		return loadoutContentsModel{
			splitView: newSplitView(nil, "sv-contents"),
		}
	}

	// Resolve all manifest references against the catalog
	resolved := resolveManifestItems(manifest, cat)

	// Build split view items with type group headers
	var items []splitViewItem
	var resolvedMap []*catalog.ContentItem

	// Group by content type in display order
	type group struct {
		label string
		ct    catalog.ContentType
		refs  []resolvedLoadoutItem
	}

	groups := []group{
		{"Rules", catalog.Rules, nil},
		{"Hooks", catalog.Hooks, nil},
		{"Skills", catalog.Skills, nil},
		{"Agents", catalog.Agents, nil},
		{"MCP Configs", catalog.MCP, nil},
		{"Commands", catalog.Commands, nil},
	}

	// Distribute resolved items into groups
	for _, r := range resolved {
		for i := range groups {
			if groups[i].ct == r.ct {
				groups[i].refs = append(groups[i].refs, r)
				break
			}
		}
	}

	for _, g := range groups {
		if len(g.refs) == 0 {
			continue
		}
		// Type header (disabled/non-selectable)
		items = append(items, splitViewItem{
			Label:    fmt.Sprintf("%s (%d)", g.label, len(g.refs)),
			Disabled: true,
		})
		resolvedMap = append(resolvedMap, nil) // placeholder for header

		for _, r := range g.refs {
			items = append(items, splitViewItem{
				Label: r.name,
			})
			resolvedMap = append(resolvedMap, r.resolved)
		}
	}

	sv := newSplitView(items, "sv-contents")
	sv.listTitle = "Contents"
	m := loadoutContentsModel{
		splitView:     sv,
		resolvedItems: resolvedMap,
	}

	// Set cursor to first selectable item and load preview
	if len(items) > 0 {
		idx := sv.nextSelectableItem(-1, 1)
		m.splitView.cursor = idx
		m.splitView.adjustScroll()
		m.loadPreview(idx)
	}

	return m
}

// loadPreview loads the primary file content for the item at the given index.
func (m *loadoutContentsModel) loadPreview(index int) {
	if index < 0 || index >= len(m.resolvedItems) {
		m.splitView.SetPreview("")
		return
	}

	resolved := m.resolvedItems[index]
	if resolved == nil {
		m.splitView.SetPreview("(not found in catalog)")
		return
	}

	// Find the primary file for this content type
	primaryFile := findPrimaryFile(*resolved)
	if primaryFile == "" {
		m.splitView.SetPreview("(no preview available)")
		return
	}

	absPath := filepath.Join(resolved.Path, primaryFile)
	data, err := os.ReadFile(absPath)
	if err != nil {
		m.splitView.SetPreview(fmt.Sprintf("Cannot read file: %s", err))
		return
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > maxPreviewLines {
		content = strings.Join(lines[:maxPreviewLines], "\n")
		content += fmt.Sprintf("\n\n(%d more lines)", len(lines)-maxPreviewLines)
	}

	m.splitView.SetPreview(content)
}

// findPrimaryFile returns the relative path of the primary file for preview.
// Uses the same logic as primaryFileIndex but returns the path string directly.
func findPrimaryFile(item catalog.ContentItem) string {
	if len(item.Files) == 0 {
		return ""
	}

	for _, f := range item.Files {
		name := strings.ToLower(filepath.Base(f))
		switch item.Type {
		case catalog.Skills:
			if name == "skill.md" {
				return f
			}
		case catalog.Agents:
			if strings.HasSuffix(name, ".md") {
				return f
			}
		case catalog.Rules:
			if strings.HasSuffix(name, ".md") {
				return f
			}
		case catalog.Hooks:
			if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				return f
			}
		case catalog.MCP:
			if strings.HasSuffix(name, ".json") {
				return f
			}
		case catalog.Commands:
			return f
		}
	}

	// Default: first file
	return item.Files[0]
}

// resolveManifestItems resolves all manifest references against the catalog.
func resolveManifestItems(manifest *loadout.Manifest, cat *catalog.Catalog) []resolvedLoadoutItem {
	refs := manifest.RefsByType()

	var resolved []resolvedLoadoutItem
	// Process in display order
	typeOrder := []catalog.ContentType{
		catalog.Rules, catalog.Hooks, catalog.Skills,
		catalog.Agents, catalog.MCP, catalog.Commands,
	}

	for _, ct := range typeOrder {
		itemRefs, ok := refs[ct]
		if !ok {
			continue
		}
		for _, ref := range itemRefs {
			item := findCatalogItem(cat, ref.Name, ct)
			resolved = append(resolved, resolvedLoadoutItem{
				name:     ref.Name,
				ct:       ct,
				resolved: item,
			})
		}
	}

	return resolved
}

// findCatalogItem finds a content item by name and type in the catalog.
func findCatalogItem(cat *catalog.Catalog, name string, ct catalog.ContentType) *catalog.ContentItem {
	if cat == nil {
		return nil
	}
	for i := range cat.Items {
		if cat.Items[i].Name == name && cat.Items[i].Type == ct {
			return &cat.Items[i]
		}
	}
	return nil
}
