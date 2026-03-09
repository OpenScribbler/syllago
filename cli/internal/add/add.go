// Package add provides the core logic for "syllago add": discovery from a
// provider with library-status annotation, and writing items to the library.
// Both the CLI command and the TUI use these functions directly.
package add

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// ItemStatus describes the relationship between a discovered provider item
// and the global library.
type ItemStatus int

const (
	// StatusNew means the item is not present in the library.
	StatusNew ItemStatus = iota
	// StatusInLibrary means the item exists and its source hash matches.
	// For hooks, it means the directory exists (no hash comparison).
	StatusInLibrary
	// StatusOutdated means the item exists but the provider's source has changed.
	// Hooks never have this status.
	StatusOutdated
)

// String returns the human-readable label used in discovery output.
func (s ItemStatus) String() string {
	switch s {
	case StatusInLibrary:
		return "in library"
	case StatusOutdated:
		return "in library, outdated"
	default:
		return "new"
	}
}

// DiscoveryItem is a discovered provider file annotated with its library status.
type DiscoveryItem struct {
	Name   string
	Type   catalog.ContentType
	Path   string // absolute path to source file
	Status ItemStatus
	Scope  string // "project" or "global" (set by caller)
}

// AddStatus tracks the outcome of a single write operation.
type AddStatus int

const (
	AddStatusAdded    AddStatus = iota // item was written (new)
	AddStatusUpdated                   // item was overwritten (--force on outdated)
	AddStatusUpToDate                  // hash matched, skipped
	AddStatusSkipped                   // source changed but --force not set
	AddStatusError                     // write failed
)

// AddResult holds the per-item result from AddItems.
type AddResult struct {
	Name   string
	Type   catalog.ContentType
	Status AddStatus
	Error  error
}

// AddOptions controls the behavior of AddItems.
type AddOptions struct {
	Force    bool
	DryRun   bool
	Provider string // provider slug, used for directory layout
}

// LibraryIndex is a pre-built map from "type/provider/name" (or "type/name"
// for universal types) to the loaded metadata for that item. A nil value
// means the directory exists but has no .syllago.yaml.
type LibraryIndex map[string]*metadata.Meta

// BuildLibraryIndex scans globalDir once and returns a map keyed by
// "type/provider/name" for provider-specific types, or "type/name" for
// universal types. Items without a .syllago.yaml get a nil value entry
// (they exist but have no metadata).
func BuildLibraryIndex(globalDir string) (LibraryIndex, error) {
	idx := make(LibraryIndex)

	for _, ct := range catalog.AllContentTypes() {
		typeDir := filepath.Join(globalDir, string(ct))
		entries, err := os.ReadDir(typeDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", typeDir, err)
		}

		if ct.IsUniversal() {
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				itemDir := filepath.Join(typeDir, e.Name())
				// Load returns nil, nil when .syllago.yaml is absent — store nil entry.
				meta, err := metadata.Load(itemDir)
				if err != nil {
					return nil, fmt.Errorf("loading metadata for %s/%s: %w", ct, e.Name(), err)
				}
				key := string(ct) + "/" + e.Name()
				idx[key] = meta
			}
		} else {
			// Provider-specific: typeDir/<provider>/<name>/
			for _, provEntry := range entries {
				if !provEntry.IsDir() || strings.HasPrefix(provEntry.Name(), ".") {
					continue
				}
				provDir := filepath.Join(typeDir, provEntry.Name())
				items, err := os.ReadDir(provDir)
				if err != nil {
					return nil, fmt.Errorf("reading %s: %w", provDir, err)
				}
				for _, itemEntry := range items {
					if !itemEntry.IsDir() || strings.HasPrefix(itemEntry.Name(), ".") {
						continue
					}
					itemDir := filepath.Join(provDir, itemEntry.Name())
					// Load returns nil, nil when .syllago.yaml is absent — store nil entry.
					meta, err := metadata.Load(itemDir)
					if err != nil {
						return nil, fmt.Errorf("loading metadata for %s/%s/%s: %w", ct, provEntry.Name(), itemEntry.Name(), err)
					}
					key := string(ct) + "/" + provEntry.Name() + "/" + itemEntry.Name()
					idx[key] = meta
				}
			}
		}
	}

	return idx, nil
}

// libraryKey returns the index key for a given type, provider slug, and item name.
func libraryKey(ct catalog.ContentType, provSlug, name string) string {
	if ct.IsUniversal() {
		return string(ct) + "/" + name
	}
	return string(ct) + "/" + provSlug + "/" + name
}

// itemName derives an item name from a file path (filename without extension).
func itemName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// sourceHash computes "sha256:<hex>" for the given raw content bytes.
func sourceHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}

// Canonicalizer converts provider-specific content to canonical form.
// The add package defines this interface so callers can supply a converter
// adapter without the add package depending on the converter package.
// If Canonicalize returns a nil Content slice, the raw bytes are used as-is.
type Canonicalizer interface {
	Canonicalize(raw []byte, sourceProvider string) (content []byte, filename string, err error)
}

// AddItems writes the given items to globalDir according to opts.
// Items with StatusInLibrary are skipped (returns AddStatusUpToDate) unless opts.Force.
// Items with StatusOutdated are skipped (returns AddStatusSkipped) unless opts.Force.
// canon may be nil; if nil, raw source bytes are written without conversion.
// ver is the syllago version string written into .syllago.yaml (e.g. "syllago v0.1.0").
// Returns one AddResult per item.
func AddItems(items []DiscoveryItem, opts AddOptions, globalDir string, canon Canonicalizer, ver string) []AddResult {
	results := make([]AddResult, 0, len(items))
	for _, item := range items {
		results = append(results, writeItem(item, opts, globalDir, canon, ver))
	}
	return results
}

func writeItem(item DiscoveryItem, opts AddOptions, globalDir string, canon Canonicalizer, ver string) AddResult {
	r := AddResult{Name: item.Name, Type: item.Type}

	// Decide whether to write based on status and --force.
	switch item.Status {
	case StatusInLibrary:
		if !opts.Force {
			r.Status = AddStatusUpToDate
			return r
		}
	case StatusOutdated:
		if !opts.Force {
			r.Status = AddStatusSkipped
			return r
		}
	}

	// Determine destination directory.
	var destDir string
	if item.Type.IsUniversal() {
		destDir = filepath.Join(globalDir, string(item.Type), item.Name)
	} else {
		destDir = filepath.Join(globalDir, string(item.Type), opts.Provider, item.Name)
	}

	if opts.DryRun {
		if item.Status == StatusNew {
			r.Status = AddStatusAdded
		} else {
			r.Status = AddStatusUpdated
		}
		return r
	}

	// Read source.
	raw, err := os.ReadFile(item.Path)
	if err != nil {
		r.Status = AddStatusError
		r.Error = fmt.Errorf("reading %s: %w", item.Path, err)
		return r
	}

	hash := sourceHash(raw)

	// Canonicalize if a converter is provided.
	content := raw
	ext := filepath.Ext(item.Path)
	if canon != nil {
		canonical, outFilename, canonErr := canon.Canonicalize(raw, opts.Provider)
		if canonErr == nil && canonical != nil {
			content = canonical
			if outFilename != "" {
				ext = filepath.Ext(outFilename)
			}
		}
		// On canonicalize error or nil result, fall through and write raw content.
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		r.Status = AddStatusError
		r.Error = fmt.Errorf("creating %s: %w", destDir, err)
		return r
	}

	// Write canonical content file.
	contentFile := contentFilename(item.Type, item.Name, ext)
	if err := os.WriteFile(filepath.Join(destDir, contentFile), content, 0644); err != nil {
		r.Status = AddStatusError
		r.Error = fmt.Errorf("writing content: %w", err)
		return r
	}

	// Preserve original in .source/ if source format differs from canonical (.md).
	hasSource := false
	sourceExt := filepath.Ext(item.Path)
	if sourceExt != "" && sourceExt != ".md" {
		sourceDir := filepath.Join(destDir, ".source")
		if mkErr := os.MkdirAll(sourceDir, 0755); mkErr == nil {
			origDest := filepath.Join(sourceDir, filepath.Base(item.Path))
			// Non-fatal: best-effort preservation of source file.
			if writeErr := os.WriteFile(origDest, raw, 0644); writeErr == nil {
				hasSource = true
			}
		}
	}

	// Write metadata with source hash.
	if ver == "" {
		ver = "syllago"
	}
	now := timeNow()
	sourceFormatExt := strings.TrimPrefix(filepath.Ext(item.Path), ".")
	meta := &metadata.Meta{
		ID:             metadata.NewID(),
		Name:           item.Name,
		Type:           string(item.Type),
		SourceProvider: opts.Provider,
		SourceFormat:   sourceFormatExt,
		SourceType:     "provider",
		SourceHash:     hash,
		HasSource:      hasSource,
		AddedAt:        &now,
		AddedBy:        ver,
	}
	// Non-fatal: metadata write failure does not fail the add operation.
	_ = metadata.Save(destDir, meta)

	if item.Status == StatusNew {
		r.Status = AddStatusAdded
	} else {
		r.Status = AddStatusUpdated
	}
	return r
}

// contentFilename returns the canonical content filename for a content type.
func contentFilename(ct catalog.ContentType, name, ext string) string {
	if ext == "" {
		ext = ".md"
	}
	switch ct {
	case catalog.Rules:
		return "rule" + ext
	case catalog.Hooks:
		return "hook.json"
	case catalog.Commands:
		return "command" + ext
	case catalog.Skills:
		return "SKILL.md"
	case catalog.Agents:
		return "agent.md"
	case catalog.MCP:
		return "mcp.json"
	default:
		return name + ext
	}
}

// timeNow is a var so tests can override it for deterministic timestamps.
var timeNow = func() time.Time { return time.Now().UTC() }

// DiscoverFromProvider discovers all content from prov, annotates each item
// with its library status against globalDir, and returns the list.
// Hooks and MCP are skipped (they use JSON merge, not file-based discovery).
func DiscoverFromProvider(prov provider.Provider, projectRoot string, resolver *config.PathResolver, globalDir string) ([]DiscoveryItem, error) {
	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		return nil, fmt.Errorf("building library index: %w", err)
	}

	var items []DiscoveryItem
	seen := make(map[string]bool) // "type/name" dedup

	for _, ct := range catalog.AllContentTypes() {
		if prov.SupportsType != nil && !prov.SupportsType(ct) {
			continue
		}
		if prov.DiscoveryPaths == nil {
			continue
		}
		// Hooks and MCP use JSON merge — handled separately in the CLI.
		if ct == catalog.Hooks || ct == catalog.MCP {
			continue
		}

		var paths []string
		if resolver != nil {
			paths = resolver.DiscoveryPaths(prov, ct, projectRoot)
		} else {
			paths = prov.DiscoveryPaths(projectRoot, ct)
		}

		for _, p := range paths {
			for _, raw := range discoverItemsAtPath(p) {
				dedupKey := string(ct) + "/" + raw.name
				if seen[dedupKey] {
					continue
				}
				seen[dedupKey] = true

				status := computeItemStatus(raw.path, ct, prov.Slug, raw.name, idx)
				items = append(items, DiscoveryItem{
					Name:   raw.name,
					Type:   ct,
					Path:   raw.path,
					Status: status,
				})
			}
		}
	}

	return items, nil
}

// rawDiscoveredItem is an intermediate result from scanning a discovery path.
type rawDiscoveredItem struct {
	name string // logical item name (directory name or filename sans extension)
	path string // path to the main content file
}

// discoverItemsAtPath scans a discovery path and returns logical items.
// If the path is a file, it returns one item. If a directory, each entry
// becomes an item: subdirectories use the directory name as item name and
// locate the main content file inside; regular files use filename sans extension.
// Symlinks are followed to determine whether targets are files or directories.
func discoverItemsAtPath(path string) []rawDiscoveredItem {
	info, err := os.Stat(path) // follows symlinks
	if err != nil {
		return nil
	}

	// Discovery path is a single file (e.g., CLAUDE.md).
	if !info.IsDir() {
		return []rawDiscoveredItem{{
			name: itemName(path),
			path: path,
		}}
	}

	// Discovery path is a directory — each entry is an item.
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var items []rawDiscoveredItem
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(path, e.Name())

		// Use os.Stat (not e.IsDir) to follow symlinks.
		fi, err := os.Stat(full)
		if err != nil {
			continue
		}

		if fi.IsDir() {
			// Directory-based item (e.g., skills/my-skill/).
			contentFile := findContentFile(full)
			if contentFile == "" {
				continue
			}
			items = append(items, rawDiscoveredItem{
				name: e.Name(),
				path: contentFile,
			})
		} else {
			// Single-file item (e.g., rules/my-rule.md).
			items = append(items, rawDiscoveredItem{
				name: itemName(full),
				path: full,
			})
		}
	}

	return items
}

// findContentFile returns the path to the first non-hidden file in a directory.
// This is the file that represents the item's content for hashing and adding.
func findContentFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		return filepath.Join(dir, e.Name())
	}
	return ""
}

// computeItemStatus determines the library status for a discovered item.
func computeItemStatus(filePath string, ct catalog.ContentType, provSlug, name string, idx LibraryIndex) ItemStatus {
	key := libraryKey(ct, provSlug, name)
	existing, inLib := idx[key]

	if !inLib {
		return StatusNew
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		// Can't read source — treat as new so the error surfaces on add.
		return StatusNew
	}

	if existing != nil && existing.SourceHash == sourceHash(raw) {
		return StatusInLibrary
	}

	return StatusOutdated
}
