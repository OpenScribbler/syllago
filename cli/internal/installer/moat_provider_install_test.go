package installer

// Tests for InstallCachedMOATToProvider — the bridge between MOAT's source
// cache and the provider-side install machinery (bead syllago-kdxus).
//
// Strategy: drive the function with a real on-disk cache dir (t.TempDir()),
// a stub provider whose InstallDir/SupportsType is hand-crafted, and assert
// the symlink/copy lands at the expected target path. The MOAT entry is a
// plain struct literal — no signing fixtures needed because the function
// accepts an already-fetched cache dir and trusts the caller's verification.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// makeMOATCacheDir mirrors what fetchAndRecord produces: a directory holding
// the extracted source-artifact tree. Tests don't care what's inside as long
// as Install() finds the path on disk.
func makeMOATCacheDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Cached Skill"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func TestInstallCachedMOATToProvider_Happy_Skills_Symlink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	prov := testProvider("test")
	cacheDir := makeMOATCacheDir(t, "my-skill")
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		DisplayName: "My Skill",
		Type:        "skill",
		ContentHash: "sha256:deadbeef",
	}

	desc, err := InstallCachedMOATToProvider(cacheDir, entry, prov, tmp, MethodSymlink, "")
	if err != nil {
		t.Fatalf("InstallCachedMOATToProvider: %v", err)
	}

	want := filepath.Join(tmp, ".testprovider", "skills", "my-skill")
	if desc != want {
		t.Errorf("desc = %q; want %q", desc, want)
	}
	info, err := os.Lstat(want)
	if err != nil {
		t.Fatalf("Lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink, got %v", info.Mode())
	}
}

func TestInstallCachedMOATToProvider_Happy_Rules_Copy(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	prov := testProvider("test")
	cacheDir := makeMOATCacheDir(t, "my-rule")
	entry := &moat.ContentEntry{
		Name:        "my-rule",
		DisplayName: "My Rule",
		Type:        "rules",
		ContentHash: "sha256:cafef00d",
	}

	desc, err := InstallCachedMOATToProvider(cacheDir, entry, prov, tmp, MethodCopy, "")
	if err != nil {
		t.Fatalf("InstallCachedMOATToProvider: %v", err)
	}

	want := filepath.Join(tmp, ".testprovider", "rules", "my-rule")
	if desc != want {
		t.Errorf("desc = %q; want %q", desc, want)
	}
	info, err := os.Lstat(want)
	if err != nil {
		t.Fatalf("Lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("expected regular file, got symlink")
	}
}

func TestInstallCachedMOATToProvider_RespectsBaseDir(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "custom-base")

	prov := testProvider("test")
	cacheDir := makeMOATCacheDir(t, "based-skill")
	entry := &moat.ContentEntry{
		Name: "based-skill",
		Type: "skill",
	}

	desc, err := InstallCachedMOATToProvider(cacheDir, entry, prov, tmp, MethodSymlink, baseDir)
	if err != nil {
		t.Fatalf("InstallCachedMOATToProvider: %v", err)
	}

	want := filepath.Join(baseDir, ".testprovider", "skills", "based-skill")
	if desc != want {
		t.Errorf("desc = %q; want %q", desc, want)
	}
}

func TestInstallCachedMOATToProvider_RejectsNilEntry(t *testing.T) {
	prov := testProvider("test")
	cacheDir := makeMOATCacheDir(t, "x")
	_, err := InstallCachedMOATToProvider(cacheDir, nil, prov, t.TempDir(), MethodSymlink, "")
	if err == nil {
		t.Fatal("expected error on nil entry")
	}
}

func TestInstallCachedMOATToProvider_RejectsEmptyCacheDir(t *testing.T) {
	prov := testProvider("test")
	entry := &moat.ContentEntry{Name: "x", Type: "skill"}
	_, err := InstallCachedMOATToProvider("", entry, prov, t.TempDir(), MethodSymlink, "")
	if err == nil {
		t.Fatal("expected error on empty cacheDir")
	}
}

func TestInstallCachedMOATToProvider_RejectsMissingCacheDir(t *testing.T) {
	prov := testProvider("test")
	entry := &moat.ContentEntry{Name: "x", Type: "skill"}
	_, err := InstallCachedMOATToProvider("/nonexistent/path/does-not-exist", entry, prov, t.TempDir(), MethodSymlink, "")
	if err == nil {
		t.Fatal("expected error on missing cacheDir")
	}
}

func TestInstallCachedMOATToProvider_RejectsUnknownType(t *testing.T) {
	prov := testProvider("test")
	cacheDir := makeMOATCacheDir(t, "x")
	entry := &moat.ContentEntry{Name: "x", Type: "loadout"} // not MOAT-emittable
	_, err := InstallCachedMOATToProvider(cacheDir, entry, prov, t.TempDir(), MethodSymlink, "")
	if err == nil {
		t.Fatal("expected error on unknown MOAT type")
	}
}

func TestInstallCachedMOATToProvider_RejectsUnsupportedType(t *testing.T) {
	// Provider stub that supports only skills, not commands.
	prov := provider.Provider{
		Name: "Skills-Only",
		Slug: "skills-only",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return filepath.Join(home, ".skills-only", "skills")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
	}
	cacheDir := makeMOATCacheDir(t, "x")
	entry := &moat.ContentEntry{Name: "x", Type: "command"}
	_, err := InstallCachedMOATToProvider(cacheDir, entry, prov, t.TempDir(), MethodSymlink, "")
	if err == nil {
		t.Fatal("expected error when provider does not support content type")
	}
}
