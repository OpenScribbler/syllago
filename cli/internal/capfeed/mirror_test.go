package capfeed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func mirrorIndex() *Index {
	return &Index{
		DataRevision:      "rev-mirror-1",
		GeneratedAt:       time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC),
		MaxStalenessHours: 48,
		Providers: []ProviderEntry{
			{Slug: "amp", Path: "capabilities/amp.json", SHA256: "a1", Status: "tracked"},
			{Slug: "zed", Path: "capabilities/zed.json", SHA256: "z1", Status: "tracked"},
		},
	}
}

func writeFileTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
}

func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	slices.Sort(paths)
	return paths
}

func TestWriteMirror_VerbatimIncludingUnknownFiles(t *testing.T) {
	capDir := t.TempDir()
	idx := mirrorIndex()
	files := map[string][]byte{
		"capabilities/amp.json": []byte(`{"slug":"amp","supported":{"skills":true}}`),
		"capabilities/zed.json": []byte(`{"slug":"zed"}`),
		"extras/new-thing.json": []byte(`{"future":"format"}`), // unrecognized path: mirrored, not rejected
	}

	res, err := WriteMirror(capDir, idx, files)
	if err != nil {
		t.Fatalf("WriteMirror: %v", err)
	}
	for path, want := range files {
		got, err := os.ReadFile(filepath.Join(capDir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("mirrored file %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Errorf("mirrored %s = %q; want verbatim %q", path, got, want)
		}
	}
	if len(res.Written) != len(files)+1 { // +1 for provenance.json
		t.Errorf("Written = %v; want the %d mirrored files plus provenance.json", res.Written, len(files))
	}
}

func TestWriteMirror_SweepRetiresUnmanagedKeepsKeepList(t *testing.T) {
	capDir := t.TempDir()
	writeFileTree(t, capDir, map[string]string{
		"claude-code.yaml":            "stale: yaml",
		"schema.json":                 `{"old": true}`,
		"by-content-type/skills.yaml": "stale: view",
		"README.md":                   "# keep me",
		"compatibility-matrix.md":     "| keep | me |",
	})

	idx := mirrorIndex()
	files := map[string][]byte{
		"capabilities/amp.json": []byte(`{"slug":"amp"}`),
		"capabilities/zed.json": []byte(`{"slug":"zed"}`),
	}
	res, err := WriteMirror(capDir, idx, files)
	if err != nil {
		t.Fatalf("WriteMirror: %v", err)
	}

	got := listTree(t, capDir)
	want := []string{
		"README.md",
		"capabilities/amp.json",
		"capabilities/zed.json",
		"compatibility-matrix.md",
		"provenance.json",
	}
	if !slices.Equal(got, want) {
		t.Errorf("tree after sweep = %v; want %v", got, want)
	}

	for _, retired := range []string{"claude-code.yaml", "schema.json", "by-content-type/skills.yaml"} {
		if !slices.Contains(res.Removed, retired) {
			t.Errorf("Removed = %v; missing retired path %s", res.Removed, retired)
		}
	}
	if slices.Contains(res.Removed, "README.md") || slices.Contains(res.Removed, "compatibility-matrix.md") {
		t.Errorf("Removed = %v; keep-list files must never be swept", res.Removed)
	}
}

func TestWriteMirror_ProvenanceMarkerAndChangedProviders(t *testing.T) {
	capDir := t.TempDir()
	idx := mirrorIndex()

	// First mirror: both providers land; no prior copies exist, so both count
	// as changed.
	first := map[string][]byte{
		"capabilities/amp.json": []byte(`{"slug":"amp","v":1}`),
		"capabilities/zed.json": []byte(`{"slug":"zed","v":1}`),
	}
	res, err := WriteMirror(capDir, idx, first)
	if err != nil {
		t.Fatalf("first WriteMirror: %v", err)
	}
	slices.Sort(res.ChangedProviders)
	if !slices.Equal(res.ChangedProviders, []string{"amp", "zed"}) {
		t.Errorf("first ChangedProviders = %v; want [amp zed]", res.ChangedProviders)
	}

	// Marker records the index identity.
	markerBytes, err := os.ReadFile(filepath.Join(capDir, "provenance.json"))
	if err != nil {
		t.Fatalf("reading provenance.json: %v", err)
	}
	var marker struct {
		DataRevision string    `json:"data_revision"`
		GeneratedAt  time.Time `json:"generated_at"`
	}
	if err := json.Unmarshal(markerBytes, &marker); err != nil {
		t.Fatalf("decoding provenance.json: %v", err)
	}
	if marker.DataRevision != idx.DataRevision {
		t.Errorf("marker data_revision = %q; want %q", marker.DataRevision, idx.DataRevision)
	}
	if !marker.GeneratedAt.Equal(idx.GeneratedAt) {
		t.Errorf("marker generated_at = %v; want %v", marker.GeneratedAt, idx.GeneratedAt)
	}

	// Second mirror: only amp's bytes differ → exactly amp is changed.
	second := map[string][]byte{
		"capabilities/amp.json": []byte(`{"slug":"amp","v":2}`),
		"capabilities/zed.json": []byte(`{"slug":"zed","v":1}`),
	}
	res, err = WriteMirror(capDir, idx, second)
	if err != nil {
		t.Fatalf("second WriteMirror: %v", err)
	}
	if !slices.Equal(res.ChangedProviders, []string{"amp"}) {
		t.Errorf("second ChangedProviders = %v; want [amp]", res.ChangedProviders)
	}
}
