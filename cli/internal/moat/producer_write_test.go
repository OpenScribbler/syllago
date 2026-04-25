package moat

// Tests for WriteManifestCache + atomicWriteFile (producer.go).
//
// The writer side is the missing half of EnrichFromMOATManifests's contract:
// the reader expects <cacheDir>/moat/registries/<name>/{manifest.json,signature.bundle}
// to exist after a successful sync, and silently downgrades trust to Unknown
// when they don't. Coverage focuses on:
//
//   - Validation: empty cacheDir/name/bytes, invalid registry name (path
//     traversal). These are programmer-error cases — production callers
//     should never trip them, but the function defends in depth because
//     IsValidRegistryName has been loosened in past PRs without anyone
//     remembering this code path depends on it.
//   - Round-trip: write then read back via the same cache layout the
//     enrich-time reader uses; bytes round-trip exactly.
//   - Atomicity: temp files aren't left behind on success; mid-write
//     interruption can't produce a partial file (modeled here by
//     verifying os.Rename semantics — the destination either has the
//     full new bytes or remains the previous content).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteManifestCache_RoundTrip(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	name := "example-reg"
	manifest := []byte(`{"schema_version":"1"}`)
	bundle := []byte("bundle-bytes")

	if err := WriteManifestCache(cacheDir, name, manifest, bundle); err != nil {
		t.Fatalf("WriteManifestCache: %v", err)
	}

	// Cache layout matches the constants used by the reader.
	manifestPath := filepath.Join(cacheDir, manifestCacheDirName, manifestCacheSubDir, name, manifestFileName)
	bundlePath := filepath.Join(cacheDir, manifestCacheDirName, manifestCacheSubDir, name, bundleFileName)

	gotManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if string(gotManifest) != string(manifest) {
		t.Errorf("manifest mismatch: got %q want %q", gotManifest, manifest)
	}
	gotBundle, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	if string(gotBundle) != string(bundle) {
		t.Errorf("bundle mismatch: got %q want %q", gotBundle, bundle)
	}
}

func TestWriteManifestCache_OverwriteAtomic(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	name := "example-reg"

	if err := WriteManifestCache(cacheDir, name, []byte("v1-manifest"), []byte("v1-bundle")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteManifestCache(cacheDir, name, []byte("v2-manifest"), []byte("v2-bundle")); err != nil {
		t.Fatalf("second write: %v", err)
	}

	manifestPath := filepath.Join(cacheDir, manifestCacheDirName, manifestCacheSubDir, name, manifestFileName)
	got, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "v2-manifest" {
		t.Errorf("expected overwrite, got %q", got)
	}

	// Verify no leftover temp files (atomicWriteFile cleanup).
	regDir := filepath.Join(cacheDir, manifestCacheDirName, manifestCacheSubDir, name)
	entries, err := os.ReadDir(regDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp file leaked: %s", e.Name())
		}
	}
	// Should be exactly the two final files.
	if len(entries) != 2 {
		t.Errorf("want 2 files, got %d: %v", len(entries), entries)
	}
}

func TestWriteManifestCache_Validation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		cacheDir    string
		regName     string
		manifest    []byte
		bundle      []byte
		errContains string
	}{
		{"empty cacheDir", "", "reg", []byte("m"), []byte("b"), "cacheDir is empty"},
		{"empty registry name", t.TempDir(), "", []byte("m"), []byte("b"), "registry name is empty"},
		{"empty manifest", t.TempDir(), "reg", []byte{}, []byte("b"), "manifestBytes is empty"},
		{"empty bundle", t.TempDir(), "reg", []byte("m"), []byte{}, "bundleBytes is empty"},
		{"invalid name dotdot", t.TempDir(), "../escape", []byte("m"), []byte("b"), "not valid"},
		{"invalid name nested slashes", t.TempDir(), "a/b/c", []byte("m"), []byte("b"), "not valid"},
		{"invalid name with space", t.TempDir(), "bad name", []byte("m"), []byte("b"), "not valid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := WriteManifestCache(tc.cacheDir, tc.regName, tc.manifest, tc.bundle)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
			}
		})
	}
}

// TestWriteManifestCache_ReaderCompatibility proves the writer and the
// reader (EnrichFromMOATManifests) agree on cache layout. After a write,
// the reader must find the cache present and not emit "missing" or
// "incomplete" warnings.
func TestWriteManifestCache_ReaderCompatibility(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	name := "reader-compat-reg"

	if err := WriteManifestCache(cacheDir, name, []byte(fixtureManifestJSON), []byte("bundle-bytes")); err != nil {
		t.Fatalf("WriteManifestCache: %v", err)
	}

	// Reader-side path resolution (mirrors enrichment loop in producer.go).
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	manifestPath, bundlePath, err := manifestCachePathsFor(absCache, name)
	if err != nil {
		t.Fatalf("manifestCachePathsFor: %v", err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("manifest not found at reader path: %v", err)
	}
	if _, err := os.Stat(bundlePath); err != nil {
		t.Errorf("bundle not found at reader path: %v", err)
	}
}

// TestAtomicWriteFile_Perms verifies the destination file ends up at 0o644
// regardless of os.CreateTemp's default perms (typically 0o600).
func TestAtomicWriteFile_Perms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := atomicWriteFile(path, []byte("hello")); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("perm = %o; want 0644", got)
	}
}
