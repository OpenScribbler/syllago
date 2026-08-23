package regdiff

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

func TestMOATDiff_ChangeKinds(t *testing.T) {
	t.Parallel()
	oldManifest := testManifest(t, "2026-04-09T00:00:00Z", []moat.ContentEntry{
		{Type: "skill", Name: "alpha", ContentHash: testHash("1")},
		{Type: "skill", Name: "beta", ContentHash: testHash("2")},
		{Type: "rules", Name: "gamma", ContentHash: testHash("3")},
		{Type: "agent", Name: "same", ContentHash: testHash("4")},
	})
	newManifest := testManifest(t, "2026-04-10T00:00:00Z", []moat.ContentEntry{
		{Type: "skill", Name: "alpha", ContentHash: testHash("9")},
		{Type: "skill", Name: "beta", ContentHash: testHash("2")},
		{Type: "agent", Name: "same", ContentHash: testHash("4")},
		{Type: "command", Name: "delta", ContentHash: testHash("5")},
	})

	got := MOATDiff("example", oldManifest, newManifest)
	want := Diff{
		Registry: "example",
		OldRef:   "2026-04-09T00:00:00Z",
		NewRef:   "2026-04-10T00:00:00Z",
		Changes: []ItemChange{
			{Type: "command", Name: "delta", Kind: KindAdded},
			{Type: "rules", Name: "gamma", Kind: KindRemoved},
			{Type: "skill", Name: "alpha", Kind: KindModified},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MOATDiff() mismatch:\n got: %#v\nwant: %#v", got, want)
	}
	for _, change := range got.Changes {
		if change.Paths != nil {
			t.Fatalf("change %s/%s Paths = %#v; want nil", change.Type, change.Name, change.Paths)
		}
	}
}

func TestMOATDiff_FirstSyncNoBaseline(t *testing.T) {
	t.Parallel()
	newManifest := testManifest(t, "2026-04-10T00:00:00Z", []moat.ContentEntry{
		{Type: "skill", Name: "alpha", ContentHash: testHash("1")},
	})

	got := MOATDiff("example", nil, newManifest)
	want := Diff{
		Registry: "example",
		OldRef:   "",
		NewRef:   "2026-04-10T00:00:00Z",
		UpToDate: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MOATDiff() mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestMOATDiff_NoDifferences(t *testing.T) {
	t.Parallel()
	oldManifest := testManifest(t, "2026-04-09T00:00:00Z", []moat.ContentEntry{
		{Type: "skill", Name: "alpha", ContentHash: testHash("1")},
	})
	newManifest := testManifest(t, "2026-04-10T00:00:00Z", []moat.ContentEntry{
		{Type: "skill", Name: "alpha", ContentHash: testHash("1")},
	})

	got := MOATDiff("example", oldManifest, newManifest)
	if !got.UpToDate {
		t.Fatalf("UpToDate = false; want true")
	}
	if len(got.Changes) != 0 {
		t.Fatalf("Changes = %#v; want none", got.Changes)
	}
}

func TestLoadCachedManifest(t *testing.T) {
	t.Parallel()

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		got, err := LoadCachedManifest(t.TempDir(), "example")
		if err != nil {
			t.Fatalf("LoadCachedManifest: %v", err)
		}
		if got != nil {
			t.Fatalf("LoadCachedManifest() = %#v; want nil", got)
		}
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		cacheDir := t.TempDir()
		path, err := moat.ManifestCachePath(cacheDir, "example")
		if err != nil {
			t.Fatalf("ManifestCachePath: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir cache: %v", err)
		}
		if err := os.WriteFile(path, []byte(minimalManifestJSON), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		got, err := LoadCachedManifest(cacheDir, "example")
		if err != nil {
			t.Fatalf("LoadCachedManifest: %v", err)
		}
		if got == nil {
			t.Fatal("LoadCachedManifest() = nil; want manifest")
		}
		if got.Name != "Example Registry" {
			t.Fatalf("Name = %q; want Example Registry", got.Name)
		}
	})

	t.Run("corrupt", func(t *testing.T) {
		t.Parallel()
		cacheDir := t.TempDir()
		path, err := moat.ManifestCachePath(cacheDir, "example")
		if err != nil {
			t.Fatalf("ManifestCachePath: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir cache: %v", err)
		}
		if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		_, err = LoadCachedManifest(cacheDir, "example")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func testManifest(t *testing.T, updatedAt string, content []moat.ContentEntry) *moat.Manifest {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return &moat.Manifest{
		UpdatedAt: ts,
		Content:   content,
	}
}

func testHash(digit string) string {
	return "sha256:" + stringsRepeat(digit, 64)
}

func stringsRepeat(s string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		out += s
	}
	return out
}

const minimalManifestJSON = `{
  "schema_version": 1,
  "manifest_uri": "https://example.com/moat-manifest.json",
  "name": "Example Registry",
  "operator": "Example Operator",
  "updated_at": "2026-04-09T00:00:00Z",
  "registry_signing_profile": {
    "issuer": "https://token.actions.githubusercontent.com",
    "subject": "repo:owner/repo:ref:refs/heads/main"
  },
  "content": [],
  "revocations": []
}`
