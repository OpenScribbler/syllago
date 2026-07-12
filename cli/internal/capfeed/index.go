package capfeed

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// DefaultMaxStalenessHours applies when the feed omits max_staleness_hours.
// The published contract's value is 48; the feed is authoritative when it
// says otherwise (heartbeat semantics, capmon ADR 0012).
const DefaultMaxStalenessHours = 48

// Index is the tolerantly-decoded v1/index.json. Unknown fields at every
// level are ignored (default encoding/json semantics — never
// DisallowUnknownFields), enums are open strings, per the Capability Feed's
// field-semantics spec.
type Index struct {
	// DataRevision identifies the published data snapshot; the change-
	// detection key. Required.
	DataRevision string
	// GeneratedAt is the publish timestamp used by the staleness gate.
	// Required.
	GeneratedAt time.Time
	// MaxStalenessHours is the feed's own liveness contract;
	// DefaultMaxStalenessHours when absent.
	MaxStalenessHours int
	// Files is the flattened attested file list: every entry of the index's
	// files map plus every provider's capability file, sorted by path. This
	// is the complete set of (path, sha256) pairs the mirror verifies.
	Files []IndexFile
	// Providers carries the per-provider entries with their slugs, for
	// changed-provider diffing.
	Providers []ProviderEntry
}

// IndexFile is one attested file: a feed-relative path and its sha256.
type IndexFile struct {
	Path   string
	SHA256 string
}

// ProviderEntry is one entry of the index's providers array. Status is an
// open enum (unknown values are preserved, never rejected).
type ProviderEntry struct {
	Slug   string
	Path   string
	SHA256 string
	Status string
}

// rawIndex mirrors the wire shape of v1/index.json.
type rawIndex struct {
	DataRevision      string                  `json:"data_revision"`
	GeneratedAt       string                  `json:"generated_at"`
	MaxStalenessHours *int                    `json:"max_staleness_hours"`
	Files             map[string]rawIndexFile `json:"files"`
	Providers         []rawProvider           `json:"providers"`
}

type rawIndexFile struct {
	SHA256 string `json:"sha256"`
}

type rawProvider struct {
	Slug   string `json:"slug"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Status string `json:"status"`
}

// ParseIndex tolerantly decodes v1/index.json bytes. Missing data_revision,
// generated_at, or an empty attested file list is an error — the tool cannot
// change-detect or verify without them (fail-closed). A file entry without a
// sha256 is likewise an error: an unverifiable file must never be mirrored.
func ParseIndex(body []byte) (*Index, error) {
	var raw rawIndex
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("capfeed index: decode: %w", err)
	}
	if raw.DataRevision == "" {
		return nil, errors.New("capfeed index: missing data_revision")
	}
	if raw.GeneratedAt == "" {
		return nil, errors.New("capfeed index: missing generated_at")
	}
	generatedAt, err := time.Parse(time.RFC3339, raw.GeneratedAt)
	if err != nil {
		return nil, fmt.Errorf("capfeed index: generated_at: %w", err)
	}

	idx := &Index{
		DataRevision:      raw.DataRevision,
		GeneratedAt:       generatedAt,
		MaxStalenessHours: DefaultMaxStalenessHours,
	}
	if raw.MaxStalenessHours != nil {
		idx.MaxStalenessHours = *raw.MaxStalenessHours
	}

	for path, f := range raw.Files {
		if f.SHA256 == "" {
			return nil, fmt.Errorf("capfeed index: file %q has no sha256", path)
		}
		idx.Files = append(idx.Files, IndexFile{Path: path, SHA256: f.SHA256})
	}
	for _, p := range raw.Providers {
		if p.SHA256 == "" {
			return nil, fmt.Errorf("capfeed index: provider %q file %q has no sha256", p.Slug, p.Path)
		}
		idx.Providers = append(idx.Providers, ProviderEntry(p))
		idx.Files = append(idx.Files, IndexFile{Path: p.Path, SHA256: p.SHA256})
	}
	if len(idx.Files) == 0 {
		return nil, errors.New("capfeed index: no attested files (files and providers both empty)")
	}
	sort.Slice(idx.Files, func(i, j int) bool { return idx.Files[i].Path < idx.Files[j].Path })

	return idx, nil
}
