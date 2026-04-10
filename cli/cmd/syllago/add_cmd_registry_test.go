package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// setupRegistryClone creates a fake registry clone at CacheDirOverride/<name>/
// with a registry.yaml index and one skill directory. Returns the clone dir path.
func setupRegistryClone(t *testing.T, cacheDir, registryName string) string {
	t.Helper()
	cloneDir := filepath.Join(cacheDir, filepath.FromSlash(registryName))
	os.MkdirAll(cloneDir, 0755)

	// Create a skill directory with SKILL.md
	skillDir := filepath.Join(cloneDir, "skills", "canary-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Canary skill\nDo you know CARDINAL-ZEBRA-7742?"), 0644)

	// Create a second skill
	skill2Dir := filepath.Join(cloneDir, "skills", "probe-skill")
	os.MkdirAll(skill2Dir, 0755)
	os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte("# Probe skill\nTesting probing."), 0644)

	// Write registry.yaml using directory-based scan (no items key → directory walk)
	// This exercises the walk-based path in ScanRegistriesOnly.
	os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte("name: test-registry\n"), 0644)

	return cloneDir
}

// setupProjectWithRegistry creates a temp project with a .syllago/config.json
// that registers the given registry name (full org/repo format).
func setupProjectWithRegistry(t *testing.T, registryName string) string {
	t.Helper()
	root := t.TempDir()

	cfg := &config.Config{
		Registries: []config.Registry{
			{
				Name:       registryName,
				URL:        "https://github.com/" + registryName + ".git",
				Visibility: "public",
			},
		},
	}
	if err := config.Save(root, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	return root
}

// TestAddFromRegistry_ByType verifies "syllago add skills --from <registry>" writes skills.
func TestAddFromRegistry_ByType(t *testing.T) {
	const regName = "test-org/test-registry"

	// Override registry cache dir.
	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	// Create project with registry in config.
	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	// Override global content dir.
	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	stdout, _ := output.SetForTest(t)

	addCmd.Flags().Set("from", regName)
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
		addCmd.Flags().Set("force", "false")
		addCmd.Flags().Set("dry-run", "false")
	})

	err := addCmd.RunE(addCmd, []string{"skills"})
	if err != nil {
		t.Fatalf("add from registry: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "canary-skill") {
		t.Errorf("expected output to mention canary-skill; got:\n%s", out)
	}

	// Verify the skill was written to the library.
	skillPath := filepath.Join(globalDir, "skills", "canary-skill", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Errorf("expected skill written to %s but it does not exist", skillPath)
	}
}

// TestAddFromRegistry_ShortName verifies matching by short name (last segment).
func TestAddFromRegistry_ShortName(t *testing.T) {
	const regName = "test-org/test-registry"
	const shortName = "test-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", shortName) // use short name
	addCmd.Flags().Set("all", "true")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
	})

	err := addCmd.RunE(addCmd, []string{})
	if err != nil {
		t.Fatalf("add from registry (short name): %v", err)
	}

	// At least one skill should have been written.
	entries, _ := os.ReadDir(filepath.Join(globalDir, "skills"))
	if len(entries) == 0 {
		t.Errorf("expected at least one skill written to library")
	}
}

// TestAddFromRegistry_DryRun verifies --dry-run does not write files.
func TestAddFromRegistry_DryRun(t *testing.T) {
	const regName = "test-org/test-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	stdout, _ := output.SetForTest(t)

	addCmd.Flags().Set("from", regName)
	addCmd.Flags().Set("all", "true")
	addCmd.Flags().Set("dry-run", "true")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
		addCmd.Flags().Set("dry-run", "false")
	})

	err := addCmd.RunE(addCmd, []string{})
	if err != nil {
		t.Fatalf("dry-run add from registry: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run output; got:\n%s", out)
	}

	// Nothing should be written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) > 0 {
		t.Errorf("dry-run should write nothing, but found: %v", entries)
	}
}

// TestAddFromRegistry_Discovery verifies no-arg invocation shows discovery output.
func TestAddFromRegistry_Discovery(t *testing.T) {
	const regName = "test-org/test-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	stdout, _ := output.SetForTest(t)

	addCmd.Flags().Set("from", regName)
	addCmd.Flags().Set("all", "false")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
	})

	err := addCmd.RunE(addCmd, []string{})
	if err != nil {
		t.Fatalf("discovery from registry: %v", err)
	}

	out := stdout.String()
	// Should show discovered content without writing.
	if !strings.Contains(out, "canary-skill") && !strings.Contains(out, "probe-skill") {
		t.Errorf("expected discovery output to list skills; got:\n%s", out)
	}

	// Nothing should be written in discovery mode.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) > 0 {
		t.Errorf("discovery mode should write nothing, but found: %v", entries)
	}
}

// TestAddFromRegistry_NotCloned verifies a helpful error when the registry isn't cloned.
func TestAddFromRegistry_NotCloned(t *testing.T) {
	const regName = "test-org/not-cloned-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	// Do NOT create the clone dir — registry exists in config but not on disk.
	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", regName)
	addCmd.Flags().Set("all", "true")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
	})

	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Fatal("expected error for uncloned registry, got nil")
	}
	if !strings.Contains(err.Error(), "not cloned") && !strings.Contains(err.Error(), "sync") {
		t.Errorf("expected error mentioning 'not cloned' or 'sync'; got: %v", err)
	}
}

// TestAddFromRegistry_UnknownName verifies the error mentions both providers and registries.
func TestAddFromRegistry_UnknownName(t *testing.T) {
	root := t.TempDir()
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", "completely-unknown")
	addCmd.Flags().Set("all", "true")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
	})

	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unknown name, got nil")
	}
	if !strings.Contains(err.Error(), "unknown provider or registry") {
		t.Errorf("expected error to mention 'unknown provider or registry'; got: %v", err)
	}
}

// TestAddFromRegistry_MetadataSourceRegistry verifies that added items get
// SourceRegistry set in their .syllago.yaml.
func TestAddFromRegistry_MetadataSourceRegistry(t *testing.T) {
	const regName = "test-org/test-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", regName)
	addCmd.Flags().Set("all", "true")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
		addCmd.Flags().Set("force", "false")
		addCmd.Flags().Set("dry-run", "false")
	})

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add from registry: %v", err)
	}

	// Read and parse the metadata for canary-skill.
	metaPath := filepath.Join(globalDir, "skills", "canary-skill", ".syllago.yaml")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}

	// Verify SourceRegistry is set in the metadata (YAML, check as string).
	if !strings.Contains(string(data), regName) {
		t.Errorf("expected metadata to contain registry name %q; got:\n%s", regName, string(data))
	}
}

// TestFindRegistryForSlug verifies name matching (full and short name).
func TestFindRegistryForSlug(t *testing.T) {
	reg := config.Registry{Name: "acme/my-tools", URL: "https://github.com/acme/my-tools.git"}
	cfg := &config.Config{Registries: []config.Registry{reg}}

	tests := []struct {
		slug  string
		found bool
	}{
		{"acme/my-tools", true}, // full name match
		{"my-tools", true},      // short name match
		{"acme", false},         // not a valid match (only last segment)
		{"other-tools", false},  // no match
		{"", false},             // empty
	}

	for _, tc := range tests {
		got := findRegistryForSlug(tc.slug, cfg)
		if (got != nil) != tc.found {
			t.Errorf("findRegistryForSlug(%q): got %v, want found=%v", tc.slug, got, tc.found)
		}
	}
}

// TestAddFromRegistry_AllAndPositionalIsError verifies --all + positional fails.
func TestAddFromRegistry_AllAndPositionalIsError(t *testing.T) {
	const regName = "test-org/test-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", regName)
	addCmd.Flags().Set("all", "true")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("all", "false")
	})

	// JSON only used to get structured error code — check error is returned.
	err := addCmd.RunE(addCmd, []string{"skills"})
	if err == nil {
		t.Fatal("expected error for --all + positional, got nil")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Errorf("expected error to mention --all; got: %v", err)
	}
}

// Ensure the json import is used (via json.Marshal in a helper).
var _ = json.Marshal
