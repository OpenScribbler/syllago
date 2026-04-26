package moat

// Tests for the sync-time content cache (bead syllago-i352v).
//
// Sync clones each manifest source_uri once, copies the verified
// <category>/<name>/ subtree into <cacheDir>/moat/registries/<name>/items/...
// so the TUI library preview can render content without a Peek action and
// without re-fetching at refresh time. Refresh remains disk-only — only Sync
// writes the cache.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeSourceTree builds a synthetic publisher repo on disk and returns its
// path. The single skill at <root>/skills/<name>/ has two files so callers can
// assert recursive copy and Files-collection behavior. Returns the directory
// AND the moat content_hash of skills/<name> — both are needed to construct
// the matching manifest.
func fakeSourceTree(t *testing.T, name string) (string, string) {
	t.Helper()
	root := t.TempDir()
	itemDir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "SKILL.md"), []byte("# "+name+"\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "scripts", "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		// MkdirAll the parent first.
		if err := os.MkdirAll(filepath.Join(itemDir, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(itemDir, "scripts", "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := ContentHash(itemDir)
	if err != nil {
		t.Fatalf("ContentHash: %v", err)
	}
	return root, hash
}

// stubCloner returns a CloneRepoFn that copies srcByURI[uri] into destDir for
// every clone call, and atomically counts invocations.
func stubCloner(t *testing.T, srcByURI map[string]string, count *int32) CloneRepoFunc {
	t.Helper()
	return func(_ context.Context, sourceURI, destDir string) error {
		atomic.AddInt32(count, 1)
		src, ok := srcByURI[sourceURI]
		if !ok {
			return os.ErrNotExist
		}
		return CopyTree(src, destDir)
	}
}

func TestContentCachePathFor_HappyPath(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	got, err := ContentCachePathFor(cacheDir, "myreg", "skills", "split-rules-llm")
	if err != nil {
		t.Fatalf("ContentCachePathFor: %v", err)
	}
	want := filepath.Join(cacheDir, "moat", "registries", "myreg", "items", "skills", "split-rules-llm")
	if got != want {
		t.Errorf("path mismatch:\n  got  %s\n  want %s", got, want)
	}
}

func TestContentCachePathFor_RejectsInvalidName(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cases := []string{"", "..", "../escape", "..\\evil", "name with spaces", "$shellvar"}
	for _, bad := range cases {
		bad := bad
		t.Run(bad, func(t *testing.T) {
			t.Parallel()
			if _, err := ContentCachePathFor(cacheDir, bad, "skills", "x"); err == nil {
				t.Errorf("expected error for registry name %q, got nil", bad)
			}
		})
	}
}

func TestWriteContentCache_HappyPath_SingleEntry(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	srcRoot, hash := fakeSourceTree(t, "x")

	m := &Manifest{
		Content: []ContentEntry{
			{
				Name:        "x",
				DisplayName: "X",
				Type:        "skill",
				ContentHash: hash,
				SourceURI:   "https://example.com/owner/repo",
			},
		},
	}

	var calls int32
	clone := stubCloner(t, map[string]string{"https://example.com/owner/repo": srcRoot}, &calls)

	report, err := WriteContentCache(context.Background(), cacheDir, "myreg", m, clone)
	if err != nil {
		t.Fatalf("WriteContentCache: %v", err)
	}
	if report.Cached != 1 {
		t.Errorf("Cached=%d want 1", report.Cached)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", report.Warnings)
	}
	if calls != 1 {
		t.Errorf("clone calls=%d want 1", calls)
	}

	cached := filepath.Join(cacheDir, "moat", "registries", "myreg", "items", "skills", "x")
	if _, err := os.Stat(filepath.Join(cached, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md in cache, stat err: %v", err)
	}
}

func TestWriteContentCache_HashMismatch_SkipsWithWarning(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	srcRoot, _ := fakeSourceTree(t, "x")

	m := &Manifest{
		Content: []ContentEntry{
			{
				Name:        "x",
				Type:        "skill",
				ContentHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
				SourceURI:   "https://example.com/owner/repo",
			},
		},
	}

	var calls int32
	clone := stubCloner(t, map[string]string{"https://example.com/owner/repo": srcRoot}, &calls)

	report, err := WriteContentCache(context.Background(), cacheDir, "myreg", m, clone)
	if err != nil {
		t.Fatalf("WriteContentCache returned fatal err: %v", err)
	}
	if report.Cached != 0 {
		t.Errorf("Cached=%d want 0", report.Cached)
	}
	if len(report.Warnings) == 0 {
		t.Error("expected hash-mismatch warning, got none")
	} else if !strings.Contains(strings.ToLower(report.Warnings[0]), "hash") {
		t.Errorf("warning does not mention hash: %q", report.Warnings[0])
	}

	cached := filepath.Join(cacheDir, "moat", "registries", "myreg", "items", "skills", "x")
	if _, err := os.Stat(cached); !os.IsNotExist(err) {
		t.Errorf("expected no cache dir on hash mismatch, stat err=%v", err)
	}
}

func TestWriteContentCache_GroupsByRepo_OneClonePerSource(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()

	// Two skills sharing the same source repo.
	root := t.TempDir()
	for _, n := range []string{"a", "b"} {
		dir := filepath.Join(root, "skills", n)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hashA, err := ContentHash(filepath.Join(root, "skills", "a"))
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := ContentHash(filepath.Join(root, "skills", "b"))
	if err != nil {
		t.Fatal(err)
	}

	m := &Manifest{
		Content: []ContentEntry{
			{Name: "a", Type: "skill", ContentHash: hashA, SourceURI: "https://example.com/shared/repo"},
			{Name: "b", Type: "skill", ContentHash: hashB, SourceURI: "https://example.com/shared/repo"},
		},
	}

	var calls int32
	clone := stubCloner(t, map[string]string{"https://example.com/shared/repo": root}, &calls)

	report, err := WriteContentCache(context.Background(), cacheDir, "myreg", m, clone)
	if err != nil {
		t.Fatalf("WriteContentCache: %v", err)
	}
	if report.Cached != 2 {
		t.Errorf("Cached=%d want 2", report.Cached)
	}
	if calls != 1 {
		t.Errorf("clone calls=%d want 1 (entries share source_uri)", calls)
	}
}

func TestWriteContentCache_CloneFailure_Degrades(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	srcRoot, hash := fakeSourceTree(t, "good")

	m := &Manifest{
		Content: []ContentEntry{
			{Name: "bad", Type: "skill", ContentHash: hash, SourceURI: "https://example.com/missing/repo"},
			{Name: "good", Type: "skill", ContentHash: hash, SourceURI: "https://example.com/good/repo"},
		},
	}

	var calls int32
	// Only the second source resolves; the first triggers ErrNotExist.
	clone := stubCloner(t, map[string]string{"https://example.com/good/repo": srcRoot}, &calls)

	report, err := WriteContentCache(context.Background(), cacheDir, "myreg", m, clone)
	if err != nil {
		t.Fatalf("WriteContentCache returned fatal err: %v", err)
	}
	if report.Cached != 1 {
		t.Errorf("Cached=%d want 1 (good entry should still cache)", report.Cached)
	}
	if len(report.Warnings) == 0 {
		t.Error("expected clone-failure warning, got none")
	}
}

func TestWriteContentCache_UnknownType_Skipped(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()

	m := &Manifest{
		Content: []ContentEntry{
			{Name: "x", Type: "future-type", ContentHash: "sha256:dead", SourceURI: "https://example.com/owner/repo"},
		},
	}

	var calls int32
	clone := func(_ context.Context, _, _ string) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	report, err := WriteContentCache(context.Background(), cacheDir, "myreg", m, clone)
	if err != nil {
		t.Fatalf("WriteContentCache: %v", err)
	}
	if report.Cached != 0 {
		t.Errorf("Cached=%d want 0", report.Cached)
	}
	if calls != 0 {
		t.Errorf("clone called %d times for unknown type; want 0", calls)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("unexpected warnings for unknown type: %v", report.Warnings)
	}
}

func TestRemoveContentCache_RemovesItemsTreeOnly(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	regDir := filepath.Join(cacheDir, "moat", "registries", "myreg")
	itemsDir := filepath.Join(regDir, "items", "skills", "x")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemsDir, "SKILL.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestFile := filepath.Join(regDir, "manifest.json")
	if err := os.WriteFile(manifestFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveContentCache(cacheDir, "myreg"); err != nil {
		t.Fatalf("RemoveContentCache: %v", err)
	}

	if _, err := os.Stat(filepath.Join(regDir, "items")); !os.IsNotExist(err) {
		t.Errorf("items/ should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(manifestFile); err != nil {
		t.Errorf("manifest.json should be untouched, stat err=%v", err)
	}
}

func TestRemoveContentCache_Idempotent(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	if err := RemoveContentCache(cacheDir, "never-existed"); err != nil {
		t.Errorf("expected nil for missing dir, got %v", err)
	}
}
