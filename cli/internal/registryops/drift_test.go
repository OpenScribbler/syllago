package registryops

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestInstalledGitDrift_MixedDrift(t *testing.T) {
	env := setupInstalledGitDriftTest(t)

	writeDriftCloneFile(t, env.cloneDir("test-reg"), "skills/foo/SKILL.md", "upstream foo\n")
	fooLibrary := writeDriftLibraryItem(t, env.globalDir, catalog.Skills, "", "foo", "SKILL.md", "library foo\n", driftSourceHash("old foo\n"))
	barLibrary := writeDriftLibraryItem(t, env.globalDir, catalog.Rules, "claude-code", "bar", "rule.md", "library bar\n", driftSourceHash("old bar\n"))
	saveDriftInstallStore(t,
		driftRecord("test-reg", string(catalog.Skills), "foo", fooLibrary, "claude-code"),
		driftRecord("test-reg", string(catalog.Rules), "bar", barLibrary, "cursor", "claude-code", "cursor"),
	)

	got := InstalledGitDrift("test-reg")
	want := []InstalledDrift{
		{
			Registry:  "test-reg",
			Type:      string(catalog.Rules),
			Name:      "bar",
			Kind:      DriftMissing,
			Providers: []string{"claude-code", "cursor"},
		},
		{
			Registry:  "test-reg",
			Type:      string(catalog.Skills),
			Name:      "foo",
			Kind:      DriftChanged,
			Providers: []string{"claude-code"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InstalledGitDrift() = %#v; want %#v", got, want)
	}
}

func TestInstalledGitDrift_UpToDateItemSkipped(t *testing.T) {
	env := setupInstalledGitDriftTest(t)

	const content = "current foo\n"
	writeDriftCloneFile(t, env.cloneDir("test-reg"), "skills/foo/SKILL.md", content)
	fooLibrary := writeDriftLibraryItem(t, env.globalDir, catalog.Skills, "", "foo", "SKILL.md", content, driftSourceHash(content))
	saveDriftInstallStore(t, driftRecord("test-reg", string(catalog.Skills), "foo", fooLibrary, "claude-code"))

	if got := InstalledGitDrift("test-reg"); len(got) != 0 {
		t.Fatalf("InstalledGitDrift() = %#v; want empty", got)
	}
}

func TestInstalledGitDrift_NoRecordsOrUnknownRegistry(t *testing.T) {
	env := setupInstalledGitDriftTest(t)

	writeDriftCloneFile(t, env.cloneDir("test-reg"), "skills/foo/SKILL.md", "current foo\n")
	if got := InstalledGitDrift("test-reg"); got != nil {
		t.Fatalf("InstalledGitDrift() with no records = %#v; want nil", got)
	}

	otherLibrary := writeDriftLibraryItem(t, env.globalDir, catalog.Skills, "", "other", "SKILL.md", "other\n", driftSourceHash("other\n"))
	saveDriftInstallStore(t, driftRecord("other-reg", string(catalog.Skills), "other", otherLibrary, "claude-code"))
	if got := InstalledGitDrift("test-reg"); got != nil {
		t.Fatalf("InstalledGitDrift() for unknown registry = %#v; want nil", got)
	}
}

func TestInstalledGitDrift_UninstalledRegistryItemsIgnored(t *testing.T) {
	env := setupInstalledGitDriftTest(t)

	writeDriftCloneFile(t, env.cloneDir("test-reg"), "skills/foo/SKILL.md", "upstream foo\n")
	writeDriftLibraryItem(t, env.globalDir, catalog.Skills, "", "foo", "SKILL.md", "library foo\n", driftSourceHash("old foo\n"))

	const trackedContent = "tracked current\n"
	writeDriftCloneFile(t, env.cloneDir("test-reg"), "skills/tracked/SKILL.md", trackedContent)
	trackedLibrary := writeDriftLibraryItem(t, env.globalDir, catalog.Skills, "", "tracked", "SKILL.md", trackedContent, driftSourceHash(trackedContent))
	saveDriftInstallStore(t, driftRecord("test-reg", string(catalog.Skills), "tracked", trackedLibrary, "claude-code"))

	if got := InstalledGitDrift("test-reg"); len(got) != 0 {
		t.Fatalf("InstalledGitDrift() = %#v; want empty", got)
	}
}

type installedGitDriftTestEnv struct {
	cacheDir  string
	globalDir string
	configDir string
}

func setupInstalledGitDriftTest(t *testing.T) installedGitDriftTestEnv {
	t.Helper()
	env := installedGitDriftTestEnv{
		cacheDir:  t.TempDir(),
		globalDir: t.TempDir(),
		configDir: t.TempDir(),
	}

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = env.cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	origContent := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = env.globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origContent })

	origConfig := config.GlobalDirOverride
	config.GlobalDirOverride = env.configDir
	t.Cleanup(func() { config.GlobalDirOverride = origConfig })

	return env
}

func (e installedGitDriftTestEnv) cloneDir(registryName string) string {
	return filepath.Join(e.cacheDir, registryName)
}

func writeDriftCloneFile(t *testing.T, cloneDir, rel, contents string) string {
	t.Helper()
	path := filepath.Join(cloneDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", rel, err)
	}
	return path
}

func writeDriftLibraryItem(t *testing.T, globalDir string, ct catalog.ContentType, providerSlug, name, filename, contents, sourceHash string) string {
	t.Helper()
	parts := []string{globalDir, string(ct)}
	if !ct.IsUniversal() {
		parts = append(parts, providerSlug)
	}
	parts = append(parts, name)
	itemDir := filepath.Join(parts...)
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", itemDir, err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, filename), []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile library %s/%s: %v", itemDir, filename, err)
	}
	if err := metadata.Save(itemDir, &metadata.Meta{
		ID:             "test-" + name,
		Name:           name,
		Type:           string(ct),
		SourceRegistry: "test-reg",
		SourceHash:     sourceHash,
	}); err != nil {
		t.Fatalf("metadata.Save %s: %v", itemDir, err)
	}
	return itemDir
}

func saveDriftInstallStore(t *testing.T, records ...installstore.Record) {
	t.Helper()
	path, err := installstore.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	store, err := installstore.Load(path)
	if err != nil {
		t.Fatalf("installstore.Load: %v", err)
	}
	for _, rec := range records {
		store.Upsert(rec)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("installstore.Save: %v", err)
	}
}

func driftRecord(registryName, contentType, name, libraryPath string, providers ...string) installstore.Record {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	placements := make([]installstore.Placement, 0, len(providers))
	for _, providerSlug := range providers {
		placements = append(placements, installstore.Placement{
			Provider:    providerSlug,
			Mechanism:   installstore.MechanismSymlink,
			Path:        filepath.Join("/providers", providerSlug, name),
			InstalledAt: now,
		})
	}
	return installstore.Record{
		Coord: installstore.Coord{
			Registry: registryName,
			Type:     contentType,
			Name:     name,
		},
		ContentHash: "sha256:test",
		LibraryPath: libraryPath,
		InstalledAt: now,
		UpdatedAt:   now,
		Placements:  placements,
	}
}

func driftSourceHash(contents string) string {
	sum := sha256.Sum256([]byte(contents))
	return fmt.Sprintf("sha256:%x", sum)
}
