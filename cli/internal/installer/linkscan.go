package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
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

// LinkScanError is a provider install directory that exists but that a link
// scan could not read, so links in it went unseen.
type LinkScanError struct {
	Provider string
	Dir      string
	Err      error
}

func (e LinkScanError) Error() string { return fmt.Sprintf("reading %s: %v", e.Dir, e.Err) }
func (e LinkScanError) Unwrap() error { return e.Err }

// ScanProviderLinks walks each provider's install directory for every content
// type and returns the symlinks whose target resolves into any of roots.
// Symlinks pointing elsewhere (user-owned) and non-symlink entries are ignored.
// A directory it cannot read is skipped; ScanProviderLinksChecked reports it.
func ScanProviderLinks(providers []provider.Provider, home string, roots []string) []ScannedLink {
	links, _ := ScanProviderLinksChecked(providers, home, roots)
	return links
}

// ScanProviderLinksChecked is ScanProviderLinks that also returns each
// install directory it could not read. A caller about to delete what the
// links point at needs to know a link may have gone unseen.
func ScanProviderLinksChecked(providers []provider.Provider, home string, roots []string) ([]ScannedLink, []LinkScanError) {
	var links []ScannedLink
	var errs []LinkScanError
	seen := make(map[string]bool)

	for _, prov := range providers {
		if prov.InstallDir == nil {
			continue
		}
		for _, ct := range catalog.AllContentTypes() {
			dirs := []string{prov.InstallDir(home, ct)}
			if prov.LegacyInstallDir != nil {
				dirs = append(dirs, prov.LegacyInstallDir(home, ct))
			}
			for _, dir := range dirs {
				found, err := scanLinkDir(prov, ct, dir, roots, seen)
				links = append(links, found...)
				if err != nil {
					errs = append(errs, LinkScanError{Provider: prov.Slug, Dir: dir, Err: err})
				}
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
	return links, errs
}

// scanLinkDir returns the symlinks in dir whose target resolves into roots,
// skipping paths already in seen and recording the ones it returns. A dir
// that does not exist holds no links; any other read failure is returned.
func scanLinkDir(prov provider.Provider, ct catalog.ContentType, dir string, roots []string, seen map[string]bool) ([]ScannedLink, error) {
	if dir == "" || dir == provider.JSONMergeSentinel || dir == provider.ProjectScopeSentinel {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var links []ScannedLink
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
	return links, nil
}

// SourcePathFor returns the filesystem source path used when installing item.
func SourcePathFor(item catalog.ContentItem) string {
	if item.Type == catalog.Agents {
		if p := converter.ResolveContentFile(item); p != "" {
			return p
		}
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
		// Agents install as rendered copies, never as links to the canonical
		// file, so a broken agent link is pruned rather than relinked.
		if item, ok := findLinkFixMatch(link, libraryItems); ok && item.Type != catalog.Agents {
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
