package capfeed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// MarkerFileName is the provenance marker committed alongside the mirror,
// recording which verified feed snapshot the directory reflects. It doubles
// as the change-detection comparison point (data_revision).
const MarkerFileName = "provenance.json"

// mirrorKeepList names the hand-maintained files that survive the sweep.
// Everything else in the directory is owned by the mirror.
var mirrorKeepList = []string{"README.md", "compatibility-matrix.md"}

// MirrorResult reports what a WriteMirror call did.
type MirrorResult struct {
	// ChangedProviders are the slugs whose capability document bytes differ
	// from the prior on-disk copy (or had none), sorted. Feeds the rolling
	// PR body.
	ChangedProviders []string
	// Written lists the mirrored feed-relative paths plus the marker.
	Written []string
	// Removed lists pre-existing files retired by the sweep.
	Removed []string
}

// marker is the wire shape of provenance.json.
type marker struct {
	DataRevision string    `json:"data_revision"`
	GeneratedAt  time.Time `json:"generated_at"`
}

// WriteMirror makes capDir an authoritative verbatim mirror of the verified
// feed files: after a successful call the subtree equals files ∪ marker ∪
// keep-list. Pre-existing files outside that set are swept away — this sweep
// is the mechanism by which the first rolling PR retires the legacy YAML
// inventory. Callers pass only hash-verified bytes (FetchFeedFiles).
func WriteMirror(capDir string, idx *Index, files map[string][]byte) (*MirrorResult, error) {
	res := &MirrorResult{}

	// Changed-provider diff against the prior on-disk copies, captured
	// before anything is overwritten.
	for _, p := range idx.Providers {
		newBytes, ok := files[p.Path]
		if !ok {
			continue
		}
		prior, err := os.ReadFile(filepath.Join(capDir, filepath.FromSlash(p.Path)))
		if err != nil || !bytes.Equal(prior, newBytes) {
			res.ChangedProviders = append(res.ChangedProviders, p.Slug)
		}
	}
	sort.Strings(res.ChangedProviders)

	// Enumerate pre-existing files for the sweep before new writes land.
	existing := map[string]bool{}
	err := filepath.Walk(capDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			rel, relErr := filepath.Rel(capDir, p)
			if relErr != nil {
				return relErr
			}
			existing[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("capfeed mirror: scanning %s: %w", capDir, err)
	}

	// Write the mirrored files verbatim.
	for path, data := range files {
		full := filepath.Join(capDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, fmt.Errorf("capfeed mirror: %s: %w", path, err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return nil, fmt.Errorf("capfeed mirror: %s: %w", path, err)
		}
		res.Written = append(res.Written, path)
	}

	// Write the provenance marker.
	m, err := json.MarshalIndent(marker{DataRevision: idx.DataRevision, GeneratedAt: idx.GeneratedAt}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("capfeed mirror: encoding marker: %w", err)
	}
	if err := os.WriteFile(filepath.Join(capDir, MarkerFileName), append(m, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("capfeed mirror: writing marker: %w", err)
	}
	res.Written = append(res.Written, MarkerFileName)

	// Sweep: retire every pre-existing file that is neither mirrored, the
	// marker, nor on the keep-list.
	keep := map[string]bool{MarkerFileName: true}
	for _, k := range mirrorKeepList {
		keep[k] = true
	}
	for rel := range existing {
		if keep[rel] {
			continue
		}
		if _, mirrored := files[rel]; mirrored {
			continue
		}
		if err := os.Remove(filepath.Join(capDir, filepath.FromSlash(rel))); err != nil {
			return nil, fmt.Errorf("capfeed mirror: retiring %s: %w", rel, err)
		}
		res.Removed = append(res.Removed, rel)
	}
	sort.Strings(res.Removed)
	sort.Strings(res.Written)

	// Tidy directories the sweep emptied.
	if err := removeEmptyDirs(capDir); err != nil {
		return nil, fmt.Errorf("capfeed mirror: pruning empty dirs: %w", err)
	}

	return res, nil
}

// removeEmptyDirs prunes empty subdirectories under root (root itself stays).
func removeEmptyDirs(root string) error {
	var dirs []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && p != root {
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Deepest first so nested empties collapse.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(d); err != nil {
				return err
			}
		}
	}
	return nil
}
