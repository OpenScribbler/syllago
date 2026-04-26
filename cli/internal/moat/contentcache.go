package moat

// Sync-time content cache (bead syllago-i352v).
//
// MOAT manifests publish content_hash + source_uri pairs but no content
// bytes — install fetches at use time. Without a local content cache the
// TUI library preview can only render a placeholder for unstaged registry
// items, even though we have everything we need to clone, verify, and
// stage the bytes for read-only browsing.
//
// This file populates that cache during sync (the network-touching path)
// so refresh (the disk-only rescan path) can render content directly.
// The cache lives alongside the per-registry manifest cache:
//
//	<cacheDir>/moat/registries/<name>/items/<categoryDir>/<entry.Name>/
//
// Lifecycle:
//   - Sync writes (registryops.SyncOne after WriteManifestCache) — failures
//     are non-fatal warnings, so a sync that produced fresh trust state
//     succeeds even if a single source repo is unreachable.
//   - Refresh reads (materializeMOATItems consults the cache to populate
//     ContentItem.Path + Files when present, falling back to the previous
//     empty-Path stub).
//   - Remove drops the items/ subtree (registryops.RemoveOne) without
//     touching manifest.json or signature.bundle.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

const contentCacheItemsDir = "items"

// CacheReport summarizes a WriteContentCache run. Cached counts entries
// successfully copied + verified into the cache. Warnings collects per-entry
// failure messages — surfaced upstream as catalog warnings so users can see
// which items could not be cached without the whole sync failing.
type CacheReport struct {
	Cached   int
	Warnings []string
}

// ContentCachePathFor returns the absolute on-disk path for a single
// content-cache item, with the same registry-name validation + traversal
// guard as manifestCachePathsFor. Returns an error for an empty cacheDir,
// an invalid registry name, or any path that escapes cacheDir.
//
// categoryDir and entryName are the per-item path segments under
// <cacheDir>/moat/registries/<name>/items/, taken from
// CategoryDirForMOATType(entry.Type) and entry.Name respectively.
func ContentCachePathFor(cacheDir, registryName, categoryDir, entryName string) (string, error) {
	if cacheDir == "" {
		return "", errors.New("ContentCachePathFor: cacheDir is empty")
	}
	if !catalog.IsValidRegistryName(registryName) {
		return "", fmt.Errorf("ContentCachePathFor: registry name %q is not valid", registryName)
	}
	if categoryDir == "" {
		return "", errors.New("ContentCachePathFor: categoryDir is empty")
	}
	if entryName == "" {
		return "", errors.New("ContentCachePathFor: entryName is empty")
	}
	if strings.ContainsAny(categoryDir, "/\\") || categoryDir == ".." {
		return "", fmt.Errorf("ContentCachePathFor: invalid categoryDir %q", categoryDir)
	}
	if strings.ContainsAny(entryName, "/\\") || entryName == ".." {
		return "", fmt.Errorf("ContentCachePathFor: invalid entryName %q", entryName)
	}

	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return "", fmt.Errorf("resolve cacheDir: %w", err)
	}
	regRoot := filepath.Join(absCache, manifestCacheDirName, manifestCacheSubDir, registryName)
	target := filepath.Join(regRoot, contentCacheItemsDir, categoryDir, entryName)

	rel, err := filepath.Rel(absCache, target)
	if err != nil {
		return "", fmt.Errorf("compute rel path: %w", err)
	}
	if strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("path escapes cache directory: %s", target)
	}
	return target, nil
}

// WriteContentCache materializes every supported manifest entry into the
// per-registry content cache by cloning each unique source_uri once and
// copying the verified <category>/<name>/ subtree into the cache.
//
// Failure mode is degraded — a per-entry error (clone failure, missing
// subdir, hash mismatch, copy error) appends a warning to CacheReport but
// does not abort. The caller (registryops.SyncOne) treats CacheReport
// warnings as non-fatal so a sync that updated trust state still completes
// even if some content cannot be cached.
//
// cloneFn is required (no package-level fallback) so each call site picks
// its own substitution policy: production passes moat.CloneRepoFn, tests
// pass a fixture-copying closure.
func WriteContentCache(ctx context.Context, cacheDir, registryName string, m *Manifest, cloneFn CloneRepoFunc) (CacheReport, error) {
	var report CacheReport
	if cacheDir == "" {
		return report, errors.New("WriteContentCache: cacheDir is empty")
	}
	if !catalog.IsValidRegistryName(registryName) {
		return report, fmt.Errorf("WriteContentCache: registry name %q is not valid", registryName)
	}
	if m == nil {
		return report, errors.New("WriteContentCache: manifest is nil")
	}
	if cloneFn == nil {
		return report, errors.New("WriteContentCache: cloneFn is nil")
	}

	// Group entries by source_uri so a multi-item repo clones once. Entries
	// whose type is not MOAT-recognized are skipped silently — the spec
	// mandates ignore-unknown for forward compatibility, and we don't want
	// to drown users in warnings for content that was never cacheable.
	type entryRef struct {
		entry       *ContentEntry
		categoryDir string
	}
	bySource := make(map[string][]entryRef)
	var sourceOrder []string
	for i := range m.Content {
		entry := &m.Content[i]
		categoryDir, ok := CategoryDirForMOATType(entry.Type)
		if !ok {
			continue
		}
		if err := ValidateSourceURI(entry.SourceURI); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("MOAT cache: invalid source_uri for %s/%s: %v", registryName, entry.Name, err))
			continue
		}
		if _, seen := bySource[entry.SourceURI]; !seen {
			sourceOrder = append(sourceOrder, entry.SourceURI)
		}
		bySource[entry.SourceURI] = append(bySource[entry.SourceURI], entryRef{entry: entry, categoryDir: categoryDir})
	}

	// Resolve the items root once. mkdir is deferred until the first
	// successful copy so a manifest with zero supported entries does not
	// leave an empty items/ directory behind.
	itemsRoot, err := contentCacheItemsRoot(cacheDir, registryName)
	if err != nil {
		return report, err
	}

	for _, sourceURI := range sourceOrder {
		refs := bySource[sourceURI]

		scratchDir, err := os.MkdirTemp("", "syllago-moat-cache-*")
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("MOAT cache: cannot create scratch dir for %s: %v", sourceURI, err))
			continue
		}
		// MkdirTemp creates the dir; cloneRepoShallow expects an absent
		// path. Remove the empty dir before cloning, and clean up after.
		_ = os.Remove(scratchDir)

		if err := cloneFn(ctx, sourceURI, scratchDir); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("MOAT cache: clone failed for %s: %v", sourceURI, err))
			_ = os.RemoveAll(scratchDir)
			continue
		}

		for _, ref := range refs {
			itemSrc := filepath.Join(scratchDir, ref.categoryDir, ref.entry.Name)
			info, statErr := os.Stat(itemSrc)
			if statErr != nil || !info.IsDir() {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("MOAT cache: item not found in source repo for %s/%s (expected %s/%s)",
						registryName, ref.entry.Name, ref.categoryDir, ref.entry.Name))
				continue
			}

			actualHash, hashErr := ContentHash(itemSrc)
			if hashErr != nil {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("MOAT cache: hash failed for %s/%s: %v", registryName, ref.entry.Name, hashErr))
				continue
			}
			if !strings.EqualFold(actualHash, ref.entry.ContentHash) {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("MOAT cache: content_hash mismatch for %s/%s (got %s, want %s)",
						registryName, ref.entry.Name, actualHash, ref.entry.ContentHash))
				continue
			}

			target := filepath.Join(itemsRoot, ref.categoryDir, ref.entry.Name)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("MOAT cache: mkdir parent failed for %s/%s: %v", registryName, ref.entry.Name, err))
				continue
			}
			if err := CopyTree(itemSrc, target); err != nil {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("MOAT cache: copy failed for %s/%s: %v", registryName, ref.entry.Name, err))
				continue
			}
			report.Cached++
		}

		_ = os.RemoveAll(scratchDir)
	}

	return report, nil
}

// RemoveContentCache deletes the per-registry items/ subtree. Idempotent —
// a missing directory is not an error so callers can call this on every
// `registry remove` without first checking whether content was ever cached.
//
// Leaves manifest.json + signature.bundle in place so callers may invoke
// RemoveContentCache and RemoveManifestCache independently.
func RemoveContentCache(cacheDir, registryName string) error {
	if cacheDir == "" {
		return errors.New("RemoveContentCache: cacheDir is empty")
	}
	if !catalog.IsValidRegistryName(registryName) {
		return fmt.Errorf("RemoveContentCache: registry name %q is not valid", registryName)
	}
	itemsRoot, err := contentCacheItemsRoot(cacheDir, registryName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(itemsRoot); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat items dir: %w", err)
	}
	if err := os.RemoveAll(itemsRoot); err != nil {
		return fmt.Errorf("remove items dir: %w", err)
	}
	return nil
}

// contentCacheItemsRoot returns the per-registry items root with the same
// traversal-guard as ContentCachePathFor. Internal helper — exported callers
// reach individual paths through ContentCachePathFor.
func contentCacheItemsRoot(cacheDir, registryName string) (string, error) {
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return "", fmt.Errorf("resolve cacheDir: %w", err)
	}
	regRoot := filepath.Join(absCache, manifestCacheDirName, manifestCacheSubDir, registryName)
	itemsRoot := filepath.Join(regRoot, contentCacheItemsDir)
	rel, err := filepath.Rel(absCache, itemsRoot)
	if err != nil {
		return "", fmt.Errorf("compute rel path: %w", err)
	}
	if strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("path escapes cache directory: %s", itemsRoot)
	}
	return itemsRoot, nil
}
