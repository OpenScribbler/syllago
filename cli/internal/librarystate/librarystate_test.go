package librarystate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// withIsolatedGlobals points the global config dir, the registry cache and
// the global content library at temp dirs, so Load never reads the real
// ~/.syllago. It returns the library dir. Tests that call it cannot run in
// parallel, because the overrides are package variables.
func withIsolatedGlobals(t *testing.T) string {
	t.Helper()
	origCfg := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origCfg })
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = t.TempDir()
	t.Cleanup(func() { registry.CacheDirOverride = origCache })
	origLib := catalog.GlobalContentDirOverride
	lib := t.TempDir()
	catalog.GlobalContentDirOverride = lib
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origLib })
	return lib
}

// TestLoad_VerifiesInstalledRules installs a library rule into a project
// target, then checks that Load reports it installed, and that it stops
// reporting it once the target file is edited.
func TestLoad_VerifiesInstalledRules(t *testing.T) {
	lib := withIsolatedGlobals(t)
	projectRoot := t.TempDir()
	homeDir := t.TempDir()

	meta := metadata.RuleMetadata{ID: "lib-snapshot-test", Name: "snapshot-test"}
	rulesRoot := filepath.Join(lib, "rules")
	if err := rulestore.WriteRule(rulesRoot, "claude-code", "snapshot-test", meta, []byte("# Test rule\n\nHelpful text.\n")); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	loaded, err := rulestore.LoadRule(filepath.Join(rulesRoot, "claude-code", "snapshot-test"))
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}
	target := filepath.Join(projectRoot, "CLAUDE.md")
	if err := installer.InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loaded); err != nil {
		t.Fatalf("InstallRuleAppend: %v", err)
	}

	snap, err := Load(projectRoot, projectRoot, time.Now())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap.Catalog == nil || snap.Config == nil {
		t.Fatalf("Load returned no catalog or config: %+v", snap.ScanResult)
	}
	if n := len(snap.Installed.RuleAppends); n != 1 {
		t.Fatalf("Installed.RuleAppends: want 1, got %d", n)
	}
	if got := snap.Verification.MatchSet["lib-snapshot-test"]; len(got) != 1 || got[0] != target {
		t.Fatalf("after install want MatchSet[lib-snapshot-test] = [%s], got %v", target, got)
	}

	// Edit the target so the block no longer matches the installed body,
	// and push its mtime forward so the mtime cache cannot reuse Clean.
	if err := os.WriteFile(target, []byte("# Completely different contents\n"), 0o644); err != nil {
		t.Fatalf("edit target: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(target, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	snap, err = Load(projectRoot, projectRoot, time.Now())
	if err != nil {
		t.Fatalf("Load after edit: %v", err)
	}
	if got := snap.Verification.MatchSet["lib-snapshot-test"]; len(got) != 0 {
		t.Fatalf("after edit want MatchSet[lib-snapshot-test] empty, got %v", got)
	}
}

// TestLoad_MalformedInstalledIsEmpty checks that an unreadable
// installed.json yields empty installed state rather than an error or nil.
func TestLoad_MalformedInstalledIsEmpty(t *testing.T) {
	withIsolatedGlobals(t)
	projectRoot := t.TempDir()
	dir := filepath.Join(projectRoot, ".syllago")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "installed.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	snap, err := Load(projectRoot, projectRoot, time.Now())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap.Installed == nil {
		t.Fatal("Installed is nil, want empty")
	}
	if snap.Verification == nil {
		t.Fatal("Verification is nil, want a result for a project root")
	}
}

// TestLoad_NoProjectRootSkipsVerification checks the one case Verification
// stays nil.
func TestLoad_NoProjectRootSkipsVerification(t *testing.T) {
	withIsolatedGlobals(t)
	root := t.TempDir()

	snap, err := Load(root, "", time.Now())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap.Verification != nil {
		t.Fatalf("Verification: want nil without a project root, got %+v", snap.Verification)
	}
	if snap.Installed == nil {
		t.Fatal("Installed is nil, want empty")
	}
}

// TestLoad_ScanError checks that a content root the scan cannot read
// returns the error and no snapshot.
func TestLoad_ScanError(t *testing.T) {
	withIsolatedGlobals(t)
	missing := filepath.Join(t.TempDir(), "missing")

	snap, err := Load(missing, t.TempDir(), time.Now())
	if err == nil || snap != nil {
		t.Fatalf("want (nil, error) for a missing content root, got (%v, %v)", snap, err)
	}
}
