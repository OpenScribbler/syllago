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
	"github.com/OpenScribbler/syllago/cli/internal/registry"
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
	Name            string
	Type            catalog.ContentType
	Path            string // absolute path to primary content file
	SourceDir       string // absolute path to the item's root directory (empty for single-file items)
	Status          ItemStatus
	Scope           string // "project" or "global" (set by caller)
	Confidence      float64
	DetectionSource string
	DetectionMethod string

	// DisplayName, when non-empty, overrides Name in the written .syllago.yaml
	// metadata (but never the on-disk directory layout, which is keyed by Name
	// for identity). Used by the TUI review-step rename and the CLI --name flag.
	DisplayName string
	// Description, when non-empty, is written to .syllago.yaml metadata.
	Description string
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
	Force            bool
	DryRun           bool
	Provider         string // provider slug, used for directory layout
	SourceRegistry   string // registry name for taint propagation (e.g., "acme/internal-rules")
	SourceVisibility string // visibility at import time: "public", "private", "unknown"
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

	// Copy supporting files (subdirectories, non-primary files) for directory-based items.
	if item.SourceDir != "" {
		if err := copySupportingFiles(item.SourceDir, destDir, filepath.Base(item.Path)); err != nil {
			r.Status = AddStatusError
			r.Error = fmt.Errorf("copying supporting files: %w", err)
			return r
		}
	}

	// Strip scanner-computed fields from any .syllago.yaml copied from source.
	// These fields are set below from the actual discovery source, not from
	// attacker-controlled package metadata.
	if item.SourceDir != "" {
		stripAnalyzerMetadata(destDir)
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
	metaName := item.Name
	if item.DisplayName != "" {
		metaName = item.DisplayName
	}
	meta := &metadata.Meta{
		ID:             metadata.NewID(),
		Name:           metaName,
		Description:    item.Description,
		Type:           string(item.Type),
		SourceProvider: opts.Provider,
		SourceFormat:   sourceFormatExt,
		SourceType:     "provider",
		SourceHash:     hash,
		HasSource:      hasSource,
		AddedAt:        &now,
		AddedBy:        ver,
	}

	// Taint propagation: if registry source is explicitly provided, use it.
	if opts.SourceRegistry != "" {
		meta.SourceRegistry = opts.SourceRegistry
		meta.SourceVisibility = opts.SourceVisibility
		meta.SourceType = "registry"
	} else {
		// Laundering defense: check if this file is a symlink back into the library.
		taintReg, taintVis := traceSymlinkTaint(item.Path, globalDir)
		if taintReg == "" {
			// Fallback: hash-match against private library content.
			taintReg, taintVis = hashMatchTaint(hash, globalDir)
		}
		if taintReg != "" {
			meta.SourceRegistry = taintReg
			meta.SourceVisibility = taintVis
		}
	}

	// Set analyzer fields from actual discovery source (never from source package YAML).
	if item.Confidence > 0 {
		meta.Confidence = item.Confidence
		meta.DetectionSource = item.DetectionSource
		meta.DetectionMethod = item.DetectionMethod
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

// stripAnalyzerMetadata zeros out scanner-computed fields on any .syllago.yaml
// that was copied from a source package. Prevents a malicious package from
// pre-setting confidence/detection_source/detection_method to influence
// tier badges, pre-check behavior, or display.
// Non-fatal: load/save errors are silently ignored (metadata.Save overwrites anyway).
func stripAnalyzerMetadata(destDir string) {
	m, err := metadata.Load(destDir)
	if err != nil || m == nil {
		return
	}
	m.Confidence = 0
	m.DetectionSource = ""
	m.DetectionMethod = ""
	_ = metadata.Save(destDir, m)
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

// copySupportingFiles copies all files from srcDir to destDir, skipping
// the primary content file (already written and possibly canonicalized)
// and hidden entries. Directory structure is preserved. Symlinks are
// skipped — they can point anywhere on disk (including external drives
// or unrelated system paths), so following them would import arbitrary
// content that isn't conceptually part of the item.
func copySupportingFiles(srcDir, destDir, primaryFilename string) error {
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		// Skip the root itself.
		if rel == "." {
			return nil
		}
		// Skip hidden entries (and their subtrees).
		if strings.HasPrefix(filepath.Base(rel), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip symlinks — WalkDir does not follow them, and os.ReadFile would
		// follow the link and either import arbitrary external content or
		// error with "is a directory" when the target is a directory.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		// Skip the primary content file (already handled with canonicalization).
		if rel == primaryFilename {
			return nil
		}
		// Skip metadata file — metadata.Save writes the authoritative version.
		if filepath.Base(rel) == metadata.FileName {
			return nil
		}
		dest := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		return os.WriteFile(dest, data, 0644)
	})
}

// timeNow is a var so tests can override it for deterministic timestamps.
var timeNow = func() time.Time { return time.Now().UTC() }

// traceSymlinkTaint checks if filePath is a symlink (or inside a symlink dir)
// pointing back into the library at globalDir. If so, reads the target item's
// .syllago.yaml and returns its SourceRegistry and SourceVisibility.
func traceSymlinkTaint(filePath, globalDir string) (srcRegistry, srcVisibility string) {
	if globalDir == "" {
		return "", ""
	}

	// Check if the file itself or its parent directory is a symlink
	// into the library. Provider installs typically symlink at the
	// item directory level (e.g., ~/.claude/rules/my-rule -> ~/.syllago/content/rules/my-rule).
	dir := filepath.Dir(filePath)
	target, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", ""
	}

	// Check if the resolved target is inside the library
	absGlobal, err := filepath.Abs(globalDir)
	if err != nil {
		return "", ""
	}
	if !strings.HasPrefix(target, absGlobal+string(filepath.Separator)) && target != absGlobal {
		return "", ""
	}

	// It's a symlink into the library — load the metadata
	meta, err := metadata.Load(target)
	if err != nil || meta == nil {
		return "", ""
	}
	return meta.SourceRegistry, meta.SourceVisibility
}

// hashMatchTaint scans all library items with private taint and checks if
// any of them have the same source hash. Returns the taint if found.
func hashMatchTaint(hash, globalDir string) (srcRegistry, srcVisibility string) {
	if globalDir == "" || hash == "" {
		return "", ""
	}
	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		return "", ""
	}
	for _, meta := range idx {
		if meta == nil {
			continue
		}
		if registry.IsPrivate(meta.SourceVisibility) && meta.SourceRegistry != "" && meta.SourceHash == hash {
			return meta.SourceRegistry, meta.SourceVisibility
		}
	}
	return "", ""
}

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
					Name:      raw.name,
					Type:      ct,
					Path:      raw.path,
					SourceDir: raw.sourceDir,
					Status:    status,
				})
			}
		}
	}

	return items, nil
}

// rawDiscoveredItem is an intermediate result from scanning a discovery path.
type rawDiscoveredItem struct {
	name      string // logical item name (directory name or filename sans extension)
	path      string // path to the main content file
	sourceDir string // item's root directory (empty for single-file items)
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
				name:      e.Name(),
				path:      contentFile,
				sourceDir: full,
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

// DiscoverFromRegistry scans a registry clone directory and returns DiscoveryItems
// annotated with their library status. Items from a manifest-indexed registry
// (registry.yaml with items) are returned as-is from the index; otherwise the
// clone directory is walked for recognized content types.
func DiscoverFromRegistry(regName, cloneDir, globalDir string) ([]DiscoveryItem, error) {
	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		return nil, fmt.Errorf("building library index: %w", err)
	}

	sources := []catalog.RegistrySource{{Name: regName, Path: cloneDir}}
	cat, err := catalog.ScanRegistriesOnly(sources)
	if err != nil {
		return nil, fmt.Errorf("scanning registry %q: %w", regName, err)
	}

	var items []DiscoveryItem
	for _, ci := range cat.Items {
		// ci.Path may be either a directory (directory-walk scan) or a file
		// (index-based scan from registry.yaml items). Normalize to a file path
		// so AddItems can read the primary content.
		primaryFile := ci.Path
		sourceDir := ""

		if fi, statErr := os.Stat(ci.Path); statErr == nil {
			if fi.IsDir() {
				// Directory-walk case: ci.Path is the item directory.
				// Find the primary content file within it.
				primaryFile = findContentFile(ci.Path)
				if primaryFile == "" {
					continue // no readable content file — skip
				}
				sourceDir = ci.Path
			} else {
				// Index-based case: ci.Path is already the primary file.
				parent := filepath.Dir(ci.Path)
				// Only set SourceDir when the item lives in its own subdirectory
				// (i.e., the parent is not the registry root itself).
				if parent != cloneDir {
					sourceDir = parent
				}
			}
		}

		// ci.Provider is set from the manifest (e.g., "content-signal").
		// For universal types (skills, agents, etc.) the provider is ignored
		// in the library key lookup, so it has no effect on the status check.
		status := computeItemStatus(primaryFile, ci.Type, ci.Provider, ci.Name, idx)
		items = append(items, DiscoveryItem{
			Name:      ci.Name,
			Type:      ci.Type,
			Path:      primaryFile,
			SourceDir: sourceDir,
			Status:    status,
		})
	}
	return items, nil
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
