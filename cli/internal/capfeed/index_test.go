package capfeed

import (
	"strconv"
	"testing"
	"time"
)

// validIndexJSON mirrors the live feed shape at
// https://openscribbler.github.io/capmon/v1/index.json: files is a map of
// path → {sha256}, per-provider entries live in a separate providers array,
// and max_staleness_hours is published by the feed itself.
const validIndexJSON = `{
  "cadence": "daily",
  "data_revision": "ea39f43f88e5eb951ada3bc791bfa070112a41e7d5578ab1e62e1c7ecd84ed05",
  "files": {
    "by-content-type/skills.json": {"sha256": "668db67b236c9de9673ae0f1bf884160d7c2ac426efef9eeb62657cd3594efb4"},
    "spec/field-semantics.md": {"sha256": "2f10a4853cc7c0171359d321a502760fc29a42e543f254983506004b649512bb"}
  },
  "generated_at": "2026-07-12T20:41:41Z",
  "max_staleness_hours": 48,
  "providers": [
    {"path": "capabilities/amp.json", "sha256": "cb6f53a8cc7d1e3c8a2f15e33aa57f74c50304b3e4c0e1ad83b0983b57ff6b4f", "slug": "amp", "status": "tracked"}
  ]
}`

func TestParseIndex_TolerantUnknownFields(t *testing.T) {
	// Same shape as validIndexJSON plus unknown keys at every level and an
	// unrecognized file path — all must decode cleanly and be retained.
	body := `{
  "cadence": "daily",
  "data_revision": "rev-42",
  "future_top_level_key": {"nested": true},
  "files": {
    "by-content-type/skills.json": {"sha256": "aaa1", "future_file_key": 7},
    "extras/new-thing.json": {"sha256": "bbb2"}
  },
  "generated_at": "2026-07-12T20:41:41Z",
  "max_staleness_hours": 48,
  "providers": [
    {"path": "capabilities/amp.json", "sha256": "ccc3", "slug": "amp", "status": "some-future-status", "future_provider_key": []}
  ]
}`
	idx, err := ParseIndex([]byte(body))
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if idx.DataRevision != "rev-42" {
		t.Errorf("DataRevision = %q; want rev-42", idx.DataRevision)
	}
	want := time.Date(2026, 7, 12, 20, 41, 41, 0, time.UTC)
	if !idx.GeneratedAt.Equal(want) {
		t.Errorf("GeneratedAt = %v; want %v", idx.GeneratedAt, want)
	}
	if idx.MaxStalenessHours != 48 {
		t.Errorf("MaxStalenessHours = %d; want 48", idx.MaxStalenessHours)
	}

	// The flattened file list carries every attested path: both files-map
	// entries (including the unrecognized one) and the provider path.
	got := map[string]string{}
	for _, f := range idx.Files {
		got[f.Path] = f.SHA256
	}
	wantFiles := map[string]string{
		"by-content-type/skills.json": "aaa1",
		"extras/new-thing.json":       "bbb2",
		"capabilities/amp.json":       "ccc3",
	}
	for path, sha := range wantFiles {
		if got[path] != sha {
			t.Errorf("Files[%q] sha256 = %q; want %q", path, got[path], sha)
		}
	}
	if len(idx.Files) != len(wantFiles) {
		t.Errorf("len(Files) = %d; want %d (%v)", len(idx.Files), len(wantFiles), got)
	}

	// Provider entries keep their slugs for changed-provider diffing; an
	// unknown status string is an open enum, not an error.
	if len(idx.Providers) != 1 {
		t.Fatalf("len(Providers) = %d; want 1", len(idx.Providers))
	}
	p := idx.Providers[0]
	if p.Slug != "amp" || p.Path != "capabilities/amp.json" || p.SHA256 != "ccc3" || p.Status != "some-future-status" {
		t.Errorf("Providers[0] = %+v; want slug=amp path=capabilities/amp.json sha256=ccc3 status=some-future-status", p)
	}
}

func TestParseIndex_LiveShape(t *testing.T) {
	idx, err := ParseIndex([]byte(validIndexJSON))
	if err != nil {
		t.Fatalf("ParseIndex on live-shaped fixture: %v", err)
	}
	if idx.DataRevision != "ea39f43f88e5eb951ada3bc791bfa070112a41e7d5578ab1e62e1c7ecd84ed05" {
		t.Errorf("DataRevision = %q; want the fixture revision", idx.DataRevision)
	}
	if len(idx.Files) != 3 {
		t.Errorf("len(Files) = %d; want 3 (2 files-map entries + 1 provider)", len(idx.Files))
	}
}

// TestParseIndex_RejectsUnsafePaths is the regression test for the path
// containment gate: a provenance-verified index must still not be able to
// direct writes outside docs/provider-capabilities/ or over files the
// mirror does not own.
func TestParseIndex_RejectsUnsafePaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "parent traversal", path: "../escape.json"},
		{name: "embedded traversal", path: "capabilities/../../escape.json"},
		{name: "absolute path", path: "/etc/passwd"},
		{name: "backslash separator", path: `capabilities\amp.json`},
		{name: "current-dir prefix", path: "./capabilities/amp.json"},
		{name: "double slash", path: "capabilities//amp.json"},
		{name: "empty path", path: ""},
		{name: "provenance marker collision", path: "provenance.json"},
		{name: "keep-list README collision", path: "README.md"},
		{name: "keep-list compatibility-matrix collision", path: "compatibility-matrix.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Once as a files-map entry, once as a provider path: both
			// routes feed the mirror and both must reject.
			asFile := `{"data_revision": "rev", "generated_at": "2026-07-12T20:41:41Z",
				"files": {` + strconv.Quote(tt.path) + `: {"sha256": "aa"}}}`
			if idx, err := ParseIndex([]byte(asFile)); err == nil {
				t.Errorf("ParseIndex accepted files-map path %q: %+v", tt.path, idx)
			}
			asProvider := `{"data_revision": "rev", "generated_at": "2026-07-12T20:41:41Z",
				"providers": [{"path": ` + strconv.Quote(tt.path) + `, "sha256": "aa", "slug": "x"}]}`
			if idx, err := ParseIndex([]byte(asProvider)); err == nil {
				t.Errorf("ParseIndex accepted provider path %q: %+v", tt.path, idx)
			}
		})
	}
}

func TestParseIndex_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing data_revision",
			body: `{"files": {"a.json": {"sha256": "aa"}}, "generated_at": "2026-07-12T20:41:41Z"}`,
		},
		{
			name: "missing generated_at",
			body: `{"data_revision": "rev", "files": {"a.json": {"sha256": "aa"}}}`,
		},
		{
			name: "no files and no providers",
			body: `{"data_revision": "rev", "generated_at": "2026-07-12T20:41:41Z"}`,
		},
		{
			name: "file entry missing sha256",
			body: `{"data_revision": "rev", "generated_at": "2026-07-12T20:41:41Z", "files": {"a.json": {}}}`,
		},
		{
			name: "unparseable generated_at",
			body: `{"data_revision": "rev", "generated_at": "not-a-time", "files": {"a.json": {"sha256": "aa"}}}`,
		},
		{
			name: "not JSON",
			body: `<html>404</html>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := ParseIndex([]byte(tt.body))
			if err == nil {
				t.Fatalf("ParseIndex succeeded with %+v; want error", idx)
			}
		})
	}
}
