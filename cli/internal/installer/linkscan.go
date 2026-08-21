package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// LinkClass classifies a syllago-owned symlink found in a provider install directory.
type LinkClass string

const (
	LinkHealthy LinkClass = "healthy" // target exists
	LinkBroken  LinkClass = "broken"  // target missing
)

// ScannedLink is one symlink in a provider install directory whose target
// resolves into a syllago-owned root.
type ScannedLink struct {
	Provider    string
	ContentType catalog.ContentType
	Path        string
	Target      string
	Class       LinkClass
}

// FixKind is the repair applied to a broken link.
type FixKind string

const (
	FixRelink FixKind = "relink" // library still has the item; point the link at it
	FixPrune  FixKind = "prune"  // no library match; remove the dead link
)

// FixAction is one planned repair for a broken provider link.
type FixAction struct {
	Kind      FixKind
	Link      ScannedLink
	NewSource string
}

// ScanProviderLinks walks each provider's install directory for every content
// type and returns the symlinks whose target resolves into any of roots.
// Symlinks pointing elsewhere (user-owned) and non-symlink entries are ignored.
func ScanProviderLinks(providers []provider.Provider, home string, roots []string) []ScannedLink {
	var links []ScannedLink
	seen := make(map[string]bool)

	for _, prov := range providers {
		if prov.InstallDir == nil {
			continue
		}
		for _, ct := range catalog.AllContentTypes() {
			dir := prov.InstallDir(home, ct)
			if dir == "" || dir == provider.JSONMergeSentinel || dir == provider.ProjectScopeSentinel {
				continue
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.Type()&os.ModeSymlink == 0 {
					continue
				}

				linkPath := filepath.Join(dir, entry.Name())
				if absPath, err := filepath.Abs(linkPath); err == nil {
					linkPath = absPath
				} else {
					linkPath = filepath.Clean(linkPath)
				}
				if seen[linkPath] {
					continue
				}

				target, err := resolveSymlinkTarget(linkPath)
				if err != nil {
					continue
				}
				if !targetWithinAnyRoot(target, roots) {
					continue
				}

				class := LinkHealthy
				if _, err := os.Stat(linkPath); err != nil {
					class = LinkBroken
				}

				links = append(links, ScannedLink{
					Provider:    prov.Slug,
					ContentType: ct,
					Path:        linkPath,
					Target:      target,
					Class:       class,
				})
				seen[linkPath] = true
			}
		}
	}

	sort.Slice(links, func(i, j int) bool {
		if links[i].Provider != links[j].Provider {
			return links[i].Provider < links[j].Provider
		}
		if links[i].ContentType != links[j].ContentType {
			return links[i].ContentType < links[j].ContentType
		}
		return links[i].Path < links[j].Path
	})
	return links
}

// SourcePathFor returns the filesystem source path used when installing item.
func SourcePathFor(item catalog.ContentItem) string {
	if item.Type == catalog.Agents {
		return filepath.Join(item.Path, "AGENT.md")
	}
	return item.Path
}

// PlanLinkFixes maps each broken link to a repair: relink when the library
// (libraryItems) still contains a matching item, prune otherwise.
func PlanLinkFixes(broken []ScannedLink, libraryItems []catalog.ContentItem) []FixAction {
	actions := make([]FixAction, 0, len(broken))
	for _, link := range broken {
		action := FixAction{Kind: FixPrune, Link: link}
		if item, ok := findLinkFixMatch(link, libraryItems); ok {
			sourcePath := SourcePathFor(item)
			if _, err := os.Stat(sourcePath); err == nil {
				action.Kind = FixRelink
				action.NewSource = sourcePath
			}
		}
		actions = append(actions, action)
	}
	return actions
}

// ApplyLinkFixes executes the plan. Returns the actions that failed with errors.
func ApplyLinkFixes(actions []FixAction) []error {
	var errs []error
	for _, action := range actions {
		switch action.Kind {
		case FixRelink:
			if err := CreateSymlink(action.NewSource, action.Link.Path); err != nil {
				errs = append(errs, fmt.Errorf("relink %s -> %s: %w", action.Link.Path, action.NewSource, err))
			}
		case FixPrune:
			if err := os.Remove(action.Link.Path); err != nil {
				errs = append(errs, fmt.Errorf("prune %s: %w", action.Link.Path, err))
			}
		default:
			errs = append(errs, fmt.Errorf("unknown fix kind %q for %s", action.Kind, action.Link.Path))
		}
	}
	return errs
}

func targetWithinAnyRoot(target string, roots []string) bool {
	for _, root := range roots {
		if root == "" {
			continue
		}
		if pathWithinRoot(target, root) {
			return true
		}
	}
	return false
}

func findLinkFixMatch(link ScannedLink, libraryItems []catalog.ContentItem) (catalog.ContentItem, bool) {
	leaf := filepath.Base(link.Path)
	for _, item := range libraryItems {
		if item.Type != link.ContentType {
			continue
		}
		if item.Type == catalog.Agents {
			if item.Name == strings.TrimSuffix(leaf, ".md") {
				return item, true
			}
			continue
		}
		if item.Name == leaf || filepath.Base(item.Path) == leaf {
			return item, true
		}
	}
	return catalog.ContentItem{}, false
}
