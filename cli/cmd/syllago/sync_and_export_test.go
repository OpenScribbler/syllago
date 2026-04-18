package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestSyncAndExportCommandRegisters(t *testing.T) {
	// Verify the command is registered on rootCmd.
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "sync-and-export" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sync-and-export command not registered on rootCmd")
	}
}

func TestSyncAndExportFlagsDefined(t *testing.T) {
	flags := syncAndExportCmd.Flags()
	for _, name := range []string{"to", "type", "name", "source", "llm-hooks"} {
		if flags.Lookup(name) == nil {
			t.Errorf("missing --%s flag on sync-and-export", name)
		}
	}
}

// --- runExportOp direct tests ---

// setupExportEnv creates a project root with a test provider registered.
// It writes an empty .syllago/config.json so config.Load succeeds.
func setupExportEnv(t *testing.T, slug string, supports []catalog.ContentType) string {
	t.Helper()
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "TestProv",
			Slug: slug,
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				for _, s := range supports {
					if s == ct {
						return filepath.Join(installBase, string(ct))
					}
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool {
				for _, s := range supports {
					if s == ct {
						return true
					}
				}
				return false
			},
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })
	return root
}

func TestRunExportOp_UnknownProvider(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	_, _ = output.SetForTest(t)

	err := runExportOp(root, "nonexistent-provider", "", "", "local", "", "", false)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider' in error, got: %v", err)
	}
}

func TestRunExportOp_NoItemsMatchingFilter(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	_, stderr := output.SetForTest(t)

	// Name filter won't match any item in setupExportRepo.
	err := runExportOp(root, "test-prov", "", "does-not-exist", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error for no-match case, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "no items found") {
		t.Errorf("expected 'no items found' in stderr, got: %s", stderr.String())
	}
}

func TestRunExportOp_DryRun(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)

	err := runExportOp(root, "test-prov", "skills", "", "shared", "", "", true)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected '[dry-run]' prefix, got: %s", out)
	}
	if !strings.Contains(out, "would export") {
		t.Errorf("expected 'would export' message, got: %s", out)
	}
}

func TestRunExportOp_HappyPath(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)

	err := runExportOp(root, "test-prov", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Exported") {
		t.Errorf("expected 'Exported' message, got: %s", out)
	}
}

func TestRunExportOp_ProviderDoesNotSupportType(t *testing.T) {
	// Provider supports only Rules; the repo has skills and mcp, so all items are skipped.
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Rules})
	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "test-prov", "", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error for all-skipped case, got: %v", err)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "does not support") && !strings.Contains(errOut, "No items were exported") {
		t.Errorf("expected skip messages in stderr, got: %s", errOut)
	}
}

func TestRunExportOp_JSONOutput(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := runExportOp(root, "test-prov", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	// JSON output should be structured.
	if !strings.Contains(stdout.String(), `"exported"`) {
		t.Errorf("expected JSON 'exported' key, got: %s", stdout.String())
	}
}

func TestRunExportOp_TypeFilterNoMatch(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	_, stderr := output.SetForTest(t)

	// Filter by a type not present in the repo.
	err := runExportOp(root, "test-prov", "agents", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "no items found") {
		t.Errorf("expected 'no items found' in stderr, got: %s", stderr.String())
	}
}

func TestRunExportAll_IteratesProviders(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	mkProv := func(slug string) provider.Provider {
		return provider.Provider{
			Name: slug,
			Slug: slug,
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase, slug, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		}
	}
	provider.AllProviders = []provider.Provider{mkProv("prov-a"), mkProv("prov-b")}
	t.Cleanup(func() { provider.AllProviders = orig })

	stdout, _ := output.SetForTest(t)

	err := runExportAll(root, "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "prov-a") || !strings.Contains(out, "prov-b") {
		t.Errorf("expected both provider slugs in output, got: %s", out)
	}
	if !strings.Contains(out, "Export All Summary") {
		t.Errorf("expected summary section, got: %s", out)
	}
}

func TestRunExportAll_DryRun(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)

	err := runExportAll(root, "skills", "", "shared", "", "", true)
	if err != nil {
		t.Fatalf("dry-run all failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "[dry-run]") {
		t.Errorf("expected dry-run output, got: %s", stdout.String())
	}
}

func TestRunExportAll_FilterReminder(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)

	err := runExportAll(root, "skills", "greeting", "shared", "", "", false)
	if err != nil {
		t.Fatalf("export-all with filters failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "filtered by") {
		t.Errorf("expected filter reminder, got: %s", stdout.String())
	}
}

func TestRunExportOp_InstallDirEmptySkips(t *testing.T) {
	// Provider supports Skills but returns empty InstallDir — items are skipped
	// at line ~320 rather than exported.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name:         "EmptyDir",
			Slug:         "empty-dir",
			InstallDir:   func(string, catalog.ContentType) string { return "" },
			SupportsType: func(catalog.ContentType) bool { return true },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "empty-dir", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "does not support") && !strings.Contains(stderr.String(), "Skipping") {
		t.Errorf("expected skip message in stderr, got: %s", stderr.String())
	}
}

func TestRunExportOp_JSONMergeSkipsWithoutConverter(t *testing.T) {
	// Provider returns JSONMergeSentinel for Hooks; no cross-provider converter
	// source means the item is skipped with a JSON-merge message.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	// Add a hook item so the JSON merge path is traversed.
	hookDir := filepath.Join(root, "hooks", "my-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.yaml"), []byte("events: []\n"), 0644)

	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "JSONMerge",
			Slug: "json-merge",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Hooks {
					return provider.JSONMergeSentinel
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Hooks },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "json-merge", "hooks", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Should skip with a JSON-merge-specific message.
	if !strings.Contains(stderr.String(), "JSON merge") && !strings.Contains(stderr.String(), "Skipping") {
		t.Errorf("expected JSON merge skip message, got: %s", stderr.String())
	}
}

func TestRunExportOp_ProjectScopeWithoutDiscovery(t *testing.T) {
	// Provider returns ProjectScopeSentinel but has no DiscoveryPaths → item is skipped.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "ProjectScope",
			Slug: "project-scope",
			InstallDir: func(string, catalog.ContentType) string {
				return provider.ProjectScopeSentinel
			},
			SupportsType: func(catalog.ContentType) bool { return true },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "project-scope", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "requires a project directory") {
		t.Errorf("expected project-directory message, got: %s", stderr.String())
	}
}

func TestRunExportOp_CrossProviderConversion(t *testing.T) {
	// Create a skill with metadata marking it as coming from a different source
	// provider. This forces the converter path and exercises the handled=true branch.
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "cross-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: cross-skill\ndescription: cross provider skill\n---\n# Cross Skill\n"), 0644)
	// Metadata marks source as a different provider to trigger conversion.
	os.WriteFile(filepath.Join(skillDir, ".syllago.yaml"), []byte("id: cross-skill\nname: cross-skill\nsource_provider: claude-code\n"), 0644)

	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	// Use cursor as target — it has a Render implementation in skills.go.
	provider.AllProviders = []provider.Provider{
		{
			Name: "Cursor",
			Slug: "cursor",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	stdout, _ := output.SetForTest(t)

	err := runExportOp(root, "cursor", "skills", "cross-skill", "shared", "", "", false)
	if err != nil {
		t.Fatalf("cross-provider export failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "(converted)") {
		t.Errorf("expected '(converted)' in output, got: %s", out)
	}
}

func TestRunExportOp_ExampleWarning(t *testing.T) {
	// Create a skill tagged as "example" — export should emit a warning before proceeding.
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "example-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: example-skill\ndescription: example\n---\n# Example\n"), 0644)
	os.WriteFile(filepath.Join(skillDir, ".syllago.yaml"), []byte("id: example-skill\nname: example-skill\ntags:\n  - example\n"), 0644)

	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "TestProv",
			Slug: "test-prov",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "test-prov", "skills", "example-skill", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "warning:") {
		t.Errorf("expected warning message for example content, got: %s", stderr.String())
	}
}

func TestRunExportOp_PrivateRegistryWarning(t *testing.T) {
	// Create a skill with SourceRegistry set and visibility=private.
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "private-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: private-skill\ndescription: private\n---\n# Private\n"), 0644)
	os.WriteFile(filepath.Join(skillDir, ".syllago.yaml"), []byte(`id: private-skill
name: private-skill
source_registry: my-private-reg
source_visibility: private
`), 0644)

	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "TestProv",
			Slug: "test-prov",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "test-prov", "skills", "private-skill", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "private registry") {
		t.Errorf("expected private-registry warning, got: %s", stderr.String())
	}
}

func TestRunExportOp_ProjectScopeWithDiscovery(t *testing.T) {
	// Provider returns ProjectScopeSentinel but DiscoveryPaths resolves to a real dir,
	// so export falls through to the direct-copy path.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "ProjectScope",
			Slug: "project-scope",
			InstallDir: func(string, catalog.ContentType) string {
				return provider.ProjectScopeSentinel
			},
			DiscoveryPaths: func(cwd string, ct catalog.ContentType) []string {
				return []string{filepath.Join(installBase, string(ct))}
			},
			SupportsType: func(catalog.ContentType) bool { return true },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	stdout, _ := output.SetForTest(t)

	err := runExportOp(root, "project-scope", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Success path reaches the direct-copy fallback.
	if !strings.Contains(stdout.String(), "Exported") {
		t.Errorf("expected 'Exported' in output, got: %s", stdout.String())
	}
}

func TestRunExportOp_JSONMergeCrossProvider(t *testing.T) {
	// A hook item with source_provider set triggers the JSON-merge cross-provider
	// converter branch. The hooks converter renders to the target provider.
	root := setupExportRepo(t)
	hookDir := filepath.Join(root, "hooks", "x-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hooks.json"), []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`), 0644)
	os.WriteFile(filepath.Join(hookDir, ".syllago.yaml"), []byte("id: x-hook\nname: x-hook\nsource_provider: claude-code\n"), 0644)

	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "Gemini",
			Slug: "gemini-cli",
			InstallDir: func(string, catalog.ContentType) string {
				return provider.JSONMergeSentinel
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Hooks },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "gemini-cli", "hooks", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Exercises the JSON-merge branch: either "Skipping ... requires JSON merge" or
	// "(converted, merge manually)" depending on whether metadata was loaded.
	if !strings.Contains(stderr.String(), "JSON merge") && !strings.Contains(stderr.String(), "Skipping") {
		t.Errorf("expected JSON merge or skip message, got: %s", stderr.String())
	}
}

func TestRunExportOp_FilterBySourceExcludes(t *testing.T) {
	// With source=library and no library items, filterBySource skips every item.
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	_, stderr := output.SetForTest(t)

	err := runExportOp(root, "test-prov", "", "", "library", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// No items match after filterBySource excludes them all.
	if !strings.Contains(stderr.String(), "no items found") {
		t.Errorf("expected 'no items found' in stderr, got: %s", stderr.String())
	}
}

func TestRunExportOp_DelegatesAllToExportAll(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)

	// toSlug="all" dispatches to runExportAll.
	err := runExportOp(root, "all", "skills", "", "shared", "", "", true)
	if err != nil {
		t.Fatalf("export to 'all' failed: %v", err)
	}
	// runExportAll prints the summary section.
	if !strings.Contains(stdout.String(), "Export All Summary") {
		t.Errorf("expected 'Export All Summary' from runExportAll, got: %s", stdout.String())
	}
}

func TestSyncAndExportNoRegistries(t *testing.T) {
	// When there are no registries, sync is a no-op and export runs normally.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "TestProv",
			Slug: "test-prov",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Skills
			},
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	// Create .syllago/config.json with no registries.
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	stdout, _ := output.SetForTest(t)

	syncAndExportCmd.Flags().Set("to", "test-prov")
	defer syncAndExportCmd.Flags().Set("to", "")
	syncAndExportCmd.Flags().Set("type", "skills")
	defer syncAndExportCmd.Flags().Set("type", "")
	syncAndExportCmd.Flags().Set("source", "shared")
	defer syncAndExportCmd.Flags().Set("source", "local")

	err := syncAndExportCmd.RunE(syncAndExportCmd, []string{})
	if err != nil {
		t.Fatalf("sync-and-export failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected exported skill 'greeting' in output, got: %s", out)
	}
}
