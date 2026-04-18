package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestGitClient_SatisfiesInterface is a compile-time assertion that
// GitClient fulfils RegistryClient. If this file builds, the assertion
// holds; the test body exists only so `go test` reports success.
func TestGitClient_SatisfiesInterface(t *testing.T) {
	t.Parallel()
	var _ RegistryClient = (*GitClient)(nil)
}

// TestGitClient_Type returns "git" — a stable string callers rely on for
// telemetry and UI labels (C9 trust display).
func TestGitClient_Type(t *testing.T) {
	t.Parallel()
	g := NewGitClient("anything", t.TempDir())
	if got := g.Type(); got != TypeGit {
		t.Errorf("Type() = %q; want %q", got, TypeGit)
	}
	if TypeGit != "git" {
		t.Errorf("TypeGit constant = %q; want %q — the string is part of the external contract", TypeGit, "git")
	}
}

// TestGitClient_Trust returns nil — git registries provide no cryptographic
// trust signal. The UI treats nil as "no verification was attempted",
// distinct from a populated TrustMetadata (C9).
func TestGitClient_Trust(t *testing.T) {
	t.Parallel()
	g := NewGitClient("anything", t.TempDir())
	if got := g.Trust(); got != nil {
		t.Errorf("Trust() = %+v; want nil for git-backed registry", got)
	}
}

// TestGitClient_Items_ScansLocalClone verifies Items() returns every item
// under a valid registry layout. This pins the behavior that made the
// interface valuable in the first place: enumeration stays synchronous,
// local, and cheap — the MOATClient will mirror this once its cache is
// populated by Sync.
func TestGitClient_Items_ScansLocalClone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Minimal registry layout — one universal skill and one provider rule
	// is enough to prove the scan wiring; scanner correctness is covered
	// by the catalog package's own tests.
	write := func(p, content string) {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	write("registry.yaml", "name: test\nversion: \"1.0.0\"\n")
	write("skills/hello/SKILL.md", "---\nname: Hello\ndescription: greet\n---\n\nBody.\n")
	write("rules/claude-code/be-nice/rule.md", "---\ndescription: Be nice.\nalwaysApply: true\n---\n\nBody.\n")
	write("rules/claude-code/be-nice/README.md", "# be-nice\n")

	g := NewGitClient("test", dir)
	items := g.Items()
	if len(items) == 0 {
		t.Fatal("Items() returned 0 — expected at least the skill and rule")
	}

	// Every item must be tagged with the registry name so downstream UI
	// can show which registry an item came from. This is the single most
	// important post-condition of the interface.
	for _, it := range items {
		if it.Registry != "test" {
			t.Errorf("item %q Registry=%q; want %q", it.Name, it.Registry, "test")
		}
	}

	// Spot-check that both kinds of content were discovered.
	var haveSkill, haveRule bool
	for _, it := range items {
		if it.Type == catalog.Skills && it.Name == "hello" {
			haveSkill = true
		}
		if it.Type == catalog.Rules && it.Name == "be-nice" {
			haveRule = true
		}
	}
	if !haveSkill {
		t.Error("skills/hello not discovered")
	}
	if !haveRule {
		t.Error("rules/claude-code/be-nice not discovered")
	}
}

// TestGitClient_Items_EmptyDir returns an empty slice (not nil panic) when
// the clone directory exists but has no content. Production code relies on
// the implicit contract that Items is always safe to call, even before
// Sync has finished populating the cache.
func TestGitClient_Items_EmptyDir(t *testing.T) {
	t.Parallel()
	g := NewGitClient("empty", t.TempDir())
	items := g.Items()
	if len(items) != 0 {
		t.Errorf("Items() on empty dir = %d items; want 0", len(items))
	}
}

// TestGitClient_FetchContent_Dir copies a directory-shaped item into dest.
// The existing git-registry flow materializes content on disk during
// Clone/Sync, so FetchContent is a local file copy.
func TestGitClient_FetchContent_Dir(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# hi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(src, "scripts")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "run.sh"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "out")
	g := NewGitClient("test", t.TempDir())

	item := catalog.ContentItem{Name: "s", Type: catalog.Skills, Path: src}
	if err := g.FetchContent(context.Background(), item, dest); err != nil {
		t.Fatalf("FetchContent: %v", err)
	}

	// Top-level file copied.
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md at dest: %v", err)
	}
	// Nested dir copied.
	if _, err := os.Stat(filepath.Join(dest, "scripts", "run.sh")); err != nil {
		t.Errorf("expected scripts/run.sh at dest: %v", err)
	}
}

// TestGitClient_FetchContent_File handles the case where Path points at a
// single file rather than a directory (e.g. a hook config or MCP
// config.json item).
func TestGitClient_FetchContent_File(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "hook.json")
	if err := os.WriteFile(srcFile, []byte(`{"hooks":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "out")
	g := NewGitClient("test", srcDir)

	item := catalog.ContentItem{Name: "h", Type: catalog.Hooks, Path: srcFile}
	if err := g.FetchContent(context.Background(), item, dest); err != nil {
		t.Fatalf("FetchContent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "hook.json")); err != nil {
		t.Errorf("expected hook.json at dest: %v", err)
	}
}

// TestGitClient_FetchContent_MissingSource errors cleanly when the item
// path does not exist on disk — either a manifest out of sync with the
// actual clone contents or a caller bug. Must not succeed silently.
func TestGitClient_FetchContent_MissingSource(t *testing.T) {
	t.Parallel()

	g := NewGitClient("test", t.TempDir())
	item := catalog.ContentItem{Name: "gone", Path: filepath.Join(t.TempDir(), "does-not-exist")}
	err := g.FetchContent(context.Background(), item, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing source path")
	}
}

// TestGitClient_FetchContent_EmptyPath guards against the programmer error
// of handing a zero-valued item to FetchContent.
func TestGitClient_FetchContent_EmptyPath(t *testing.T) {
	t.Parallel()

	g := NewGitClient("test", t.TempDir())
	err := g.FetchContent(context.Background(), catalog.ContentItem{Name: "x"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for item with empty Path")
	}
}

// TestOpen_ReturnsGitClient verifies the factory: in Phase 1 every registry
// resolves to a git-backed client. Phase 2 will add a type parameter and
// dispatch — this test locks the Phase 1 contract so that migration is
// explicit rather than silent.
func TestOpen_ReturnsGitClient(t *testing.T) {
	t.Parallel()
	setupCacheOverride(t)

	client, err := Open("some-registry")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if client.Type() != TypeGit {
		t.Errorf("Open().Type() = %q; want %q", client.Type(), TypeGit)
	}
	if client.Trust() != nil {
		t.Error("Open().Trust() should be nil for git registries")
	}
}

// TestGitClient_Sync_Integration is a smoke test that the interface method
// is wired to the package-level Sync (which has its own integration
// coverage). We exercise the happy path so any future regression in the
// wrapper — wrong name, swallowed error — shows up immediately.
func TestGitClient_Sync_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "valid")
	if err := Clone(bare, "sync-test", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	g := NewGitClient("sync-test", mustCloneDir(t, "sync-test"))
	if err := g.Sync(context.Background()); err != nil {
		t.Fatalf("GitClient.Sync: %v", err)
	}

	// Items should round-trip the scan.
	if len(g.Items()) == 0 {
		t.Error("expected items after Clone + Sync")
	}
}

func mustCloneDir(t *testing.T, name string) string {
	t.Helper()
	d, err := CloneDir(name)
	if err != nil {
		t.Fatalf("CloneDir: %v", err)
	}
	return d
}
