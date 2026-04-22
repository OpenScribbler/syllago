package moat

// Tests for LoadAndScan (ADR 0007 Phase 2c follow-up, bead syllago-nmjrm).
//
// LoadAndScan is a thin composition over already-tested primitives
// (config.LoadGlobal/Load/Merge + registry.IsCloned + LoadLockfile +
// ScanAndEnrich + BuildGateInputs). The tests here are narrow: they
// pin the composition contract — that each output field is populated
// from the right source and that missing inputs degrade gracefully —
// without re-testing the underlying primitives.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// withIsolatedGlobals redirects every real-world lookup LoadAndScan performs
// at the dev's home directory into fresh temp dirs: the global config dir,
// the registry clone cache, and the global content library. Without all
// three, tests leak in the dev's real ~/.syllago state (skills, registries,
// cached manifests) and stop being deterministic.
func withIsolatedGlobals(t *testing.T) {
	t.Helper()
	origCfg := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origCfg })
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = t.TempDir()
	t.Cleanup(func() { registry.CacheDirOverride = origCache })
	origGlobalContent := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = t.TempDir()
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobalContent })
}

// TestLoadAndScan_EmptyProject verifies the zero-config case: a project root
// with a single skill, no registries, no global config. All ScanResult
// fields must be non-nil (GateInputs is non-nil-but-empty by contract).
func TestLoadAndScan_EmptyProject(t *testing.T) {
	withIsolatedGlobals(t)
	root := t.TempDir()

	// A minimal skill so the catalog is not empty — proves ScanAndEnrich ran.
	skill := filepath.Join(root, "skills", "hello")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"),
		[]byte("---\nname: Hello\ndescription: greeter\n---\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	res, err := LoadAndScan(root, root, time.Now())
	if err != nil {
		t.Fatalf("LoadAndScan: %v", err)
	}
	if res == nil {
		t.Fatal("ScanResult is nil")
	}
	if res.Catalog == nil {
		t.Fatal("Catalog is nil")
	}
	if len(res.Catalog.ByType(catalog.Skills)) != 1 {
		t.Errorf("expected 1 skill, got %d", len(res.Catalog.ByType(catalog.Skills)))
	}
	if res.Lockfile == nil {
		t.Error("Lockfile is nil — LoadLockfile must return an empty lockfile on missing file")
	}
	if res.GateInputs == nil {
		t.Error("GateInputs is nil — BuildGateInputs must return an empty non-nil struct")
	}
	if res.Config == nil {
		t.Error("Config is nil — Merge must return a non-nil config even with no sources")
	}
	if len(res.RegistrySources) != 0 {
		t.Errorf("expected no registry sources with no registries configured, got %d", len(res.RegistrySources))
	}
}

// TestLoadAndScan_EnumeratesClonedRegistry drops a registry into the merged
// config AND materializes its clone directory, then asserts the registry
// appears in ScanResult.RegistrySources. Without this, catalog scanning
// would silently miss registry content that the user configured.
func TestLoadAndScan_EnumeratesClonedRegistry(t *testing.T) {
	withIsolatedGlobals(t)
	root := t.TempDir()

	// Write a project config that references the registry by name. Config
	// is JSON under <root>/.syllago/config.json — LoadAndScan calls
	// config.Load(projectRoot) which resolves that exact path.
	cfgDir := filepath.Join(root, ".syllago")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfgJSON := `{"registries":[{"name":"fixtures","url":"https://example.com/fixtures"}]}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	// Materialize the clone directory under the test CacheDirOverride so
	// registry.IsCloned returns true.
	cloneDir, err := registry.CloneDir("fixtures")
	if err != nil {
		t.Fatalf("CloneDir: %v", err)
	}
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatalf("mkdir clone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cloneDir, "registry.yaml"),
		[]byte("name: fixtures\nvisibility: public\n"), 0o644); err != nil {
		t.Fatalf("write registry.yaml: %v", err)
	}

	res, err := LoadAndScan(root, root, time.Now())
	if err != nil {
		t.Fatalf("LoadAndScan: %v", err)
	}
	if len(res.RegistrySources) != 1 {
		t.Fatalf("expected 1 registry source, got %d", len(res.RegistrySources))
	}
	if res.RegistrySources[0].Name != "fixtures" {
		t.Errorf("expected registry name 'fixtures', got %q", res.RegistrySources[0].Name)
	}
	if len(res.Config.Registries) != 1 {
		t.Errorf("expected merged config to carry the registry entry, got %d", len(res.Config.Registries))
	}
}
