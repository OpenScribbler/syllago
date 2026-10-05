package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestSyncAndExportCommandRegisters(t *testing.T) {
	// Verify the command is registered on rootCmd.
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "sync-install" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sync-install command not registered on rootCmd")
	}
}

func TestSyncAndExportFlagsDefined(t *testing.T) {
	flags := syncInstallCmd.Flags()
	for _, name := range []string{"to", "type", "name", "source", "llm-hooks"} {
		if flags.Lookup(name) == nil {
			t.Errorf("missing --%s flag on sync-install", name)
		}
	}
}

// --- runInstallOp direct tests ---

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

	err := runInstallOp(root, "nonexistent-provider", "", "", "local", "", "", false)
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
	err := runInstallOp(root, "test-prov", "", "does-not-exist", "shared", "", "", false)
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

	err := runInstallOp(root, "test-prov", "skills", "", "shared", "", "", true)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected '[dry-run]' prefix, got: %s", out)
	}
	if !strings.Contains(out, "would install") {
		t.Errorf("expected 'would install' message, got: %s", out)
	}
}

func TestRunExportOp_HappyPath(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)

	err := runInstallOp(root, "test-prov", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Installed") {
		t.Errorf("expected 'Installed' message, got: %s", out)
	}
}

func TestRunExportOp_ProviderDoesNotSupportType(t *testing.T) {
	// Provider supports only Rules; the repo has skills and mcp, so all items are skipped.
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Rules})
	_, stderr := output.SetForTest(t)

	err := runInstallOp(root, "test-prov", "", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error for all-skipped case, got: %v", err)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "does not support") && !strings.Contains(errOut, "No items were installed") {
		t.Errorf("expected skip messages in stderr, got: %s", errOut)
	}
}

func TestRunExportOp_JSONOutput(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := runInstallOp(root, "test-prov", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	// JSON output should be structured.
	if !strings.Contains(stdout.String(), `"installed"`) {
		t.Errorf("expected JSON 'exported' key, got: %s", stdout.String())
	}
}

func TestRunExportOp_TypeFilterNoMatch(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	_, stderr := output.SetForTest(t)

	// Filter by a type not present in the repo.
	err := runInstallOp(root, "test-prov", "agents", "", "shared", "", "", false)
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

	err := runInstallAll(root, "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "prov-a") || !strings.Contains(out, "prov-b") {
		t.Errorf("expected both provider slugs in output, got: %s", out)
	}
	if !strings.Contains(out, "Install All Summary") {
		t.Errorf("expected summary section, got: %s", out)
	}
}

func TestRunExportAll_DryRun(t *testing.T) {
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	stdout, _ := output.SetForTest(t)

	err := runInstallAll(root, "skills", "", "shared", "", "", true)
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

	err := runInstallAll(root, "skills", "greeting", "shared", "", "", false)
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

	err := runInstallOp(root, "empty-dir", "skills", "", "shared", "", "", false)
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

	err := runInstallOp(root, "json-merge", "hooks", "", "shared", "", "", false)
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

	err := runInstallOp(root, "project-scope", "skills", "", "shared", "", "", false)
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

	err := runInstallOp(root, "cursor", "skills", "cross-skill", "shared", "", installBase, false)
	if err != nil {
		t.Fatalf("cross-provider export failed: %v%s", err, structuredTestDetails(err))
	}
	out := stdout.String()
	if !strings.Contains(out, "(converted)") {
		t.Errorf("expected '(converted)' in output, got: %s", out)
	}
}

func structuredTestDetails(err error) string {
	if se, ok := err.(output.StructuredError); ok && se.Details != "" {
		return ": " + se.Details
	}
	return ""
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

	err := runInstallOp(root, "test-prov", "skills", "example-skill", "shared", "", "", false)
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

	err := runInstallOp(root, "test-prov", "skills", "private-skill", "shared", "", "", false)
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

	err := runInstallOp(root, "project-scope", "skills", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Success path reaches the direct-copy fallback.
	if !strings.Contains(stdout.String(), "Installed") {
		t.Errorf("expected 'Installed' in output, got: %s", stdout.String())
	}
}

func TestRunExportOp_JSONMergeCrossProvider(t *testing.T) {
	// A hook item with source_provider set triggers the JSON-merge cross-provider
	// converter branch. The hooks converter renders to the target provider.
	root := setupExportRepo(t)
	hookDir := filepath.Join(root, "hooks", "claude-code", "x-hook")
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

	stdout, stderr := output.SetForTest(t)

	err := runInstallOp(root, "gemini-cli", "hooks", "", "shared", "", "", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout.String(), "(converted, merge manually)") {
		t.Errorf("expected a converted export, got stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

// Regression: sync-install rendered hooks through a second encoder, so the
// exported file could differ from what install writes, and a Library hook
// manifest went out unconverted.
func TestRunExportOp_HookExportUsesTheInstallEncoder(t *testing.T) {
	root := setupExportRepo(t)
	hookDir := filepath.Join(root, "hooks", "claude-code", "x-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","matcher":"shell","handler":{"type":"prompt","prompt":"Is this safe?"}}]}`), 0644)
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
	stdout, stderr := output.SetForTest(t)

	if err := runInstallOp(root, "gemini-cli", "hooks", "", "shared", converter.LLMHooksModeGenerate, "", false); err != nil {
		t.Fatalf("runInstallOp: %v", err)
	}
	exported, _ := filepath.Glob(filepath.Join(hookDir, "exported-gemini-cli-*"))
	if len(exported) != 1 {
		t.Fatalf("want one exported file, got %v\nstdout: %s\nstderr: %s", exported, stdout, stderr)
	}
	data, _ := os.ReadFile(exported[0])
	for _, want := range []string{`"BeforeTool"`, `"run_shell_command"`, `./syllago-llm-hook-`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("exported hooks missing %s:\n%s", want, data)
		}
	}
	scripts, _ := filepath.Glob(filepath.Join(hookDir, "syllago-llm-hook-*.sh"))
	if len(scripts) != 1 {
		t.Errorf("want one wrapper script, got %v", scripts)
	}
}

func TestRunExportOp_SkipModeSuggestsGenerate(t *testing.T) {
	root := setupExportRepo(t)
	hookDir := filepath.Join(root, "hooks", "claude-code", "x-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","handler":{"type":"command","command":"./check.sh"}},{"event":"before_tool_execute","handler":{"type":"prompt","prompt":"Is this safe?"}}]}`), 0644)
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
	stdout, stderr := output.SetForTest(t)

	if err := runInstallOp(root, "gemini-cli", "hooks", "", "shared", converter.LLMHooksModeSkip, "", false); err != nil {
		t.Fatalf("runInstallOp: %v", err)
	}
	if !strings.Contains(stdout.String()+stderr.String(), "--llm-hooks=generate") {
		t.Errorf("skip mode should suggest --llm-hooks=generate\nstdout: %q\nstderr: %q", stdout.String(), stderr.String())
	}
}

func TestRunExportOp_AllHooksDroppedExplainsWhy(t *testing.T) {
	root := setupExportRepo(t)
	hookDir := filepath.Join(root, "hooks", "claude-code", "x-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","handler":{"type":"prompt","prompt":"Is this safe?"}}]}`), 0644)
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

	if err := runInstallOp(root, "gemini-cli", "hooks", "", "shared", converter.LLMHooksModeSkip, "", false); err != nil {
		t.Fatalf("runInstallOp: %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "--llm-hooks=generate") || strings.Contains(got, "use the TUI") {
		t.Errorf("want the drop explained with the --llm-hooks hint, got:\n%s", got)
	}
}

func TestRunExportOp_AllHooksDroppedExplainsWhy_FileProvider(t *testing.T) {
	root := setupExportRepo(t)
	hookDir := filepath.Join(root, "hooks", "claude-code", "x-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","handler":{"type":"prompt","prompt":"Is this safe?"}}]}`), 0644)
	os.WriteFile(filepath.Join(hookDir, ".syllago.yaml"), []byte("id: x-hook\nname: x-hook\nsource_provider: claude-code\n"), 0644)

	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	// --base-dir bypasses the install matrix, which would send Gemini
	// hooks down the JSON-merge branch instead of the file branch.
	baseDir := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "Gemini",
			Slug: "gemini-cli",
			InstallDir: func(base string, _ catalog.ContentType) string {
				return filepath.Join(base, "hooks")
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Hooks },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })
	_, stderr := output.SetForTest(t)

	if err := runInstallOp(root, "gemini-cli", "hooks", "", "shared", converter.LLMHooksModeSkip, baseDir, false); err != nil {
		t.Fatalf("runInstallOp: %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "nothing in it converts to Gemini") || !strings.Contains(got, "--llm-hooks=generate") {
		t.Errorf("want the drop explained with the --llm-hooks hint, got:\n%s", got)
	}
}

func TestRunExportOp_HookWithNoEncoderIsSkippedNotCopied(t *testing.T) {
	root := setupExportRepo(t)
	hookDir := filepath.Join(root, "hooks", "claude-code", "x-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","handler":{"type":"command","command":"./check.sh"}}]}`), 0644)
	os.WriteFile(filepath.Join(hookDir, ".syllago.yaml"), []byte("id: x-hook\nname: x-hook\nsource_provider: claude-code\n"), 0644)

	withFakeRepoRoot(t, root)
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	baseDir := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "Codex",
			Slug: "codex",
			InstallDir: func(base string, _ catalog.ContentType) string {
				return filepath.Join(base, ".codex")
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Hooks },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })
	_, stderr := output.SetForTest(t)

	if err := runInstallOp(root, "codex", "hooks", "", "shared", converter.LLMHooksModeSkip, baseDir, false); err != nil {
		t.Fatalf("runInstallOp: %v", err)
	}
	if !strings.Contains(stderr.String(), "cannot write hooks") {
		t.Errorf("want the skip to say syllago cannot write Codex hooks, got:\n%s", stderr.String())
	}
	if entries, _ := os.ReadDir(filepath.Join(baseDir, ".codex")); len(entries) != 0 {
		t.Errorf("nothing should be placed for codex, found %d entries", len(entries))
	}
}

func TestRunExportOp_FilterBySourceExcludes(t *testing.T) {
	// With source=library and no library items, filterBySource skips every item.
	root := setupExportEnv(t, "test-prov", []catalog.ContentType{catalog.Skills})
	_, stderr := output.SetForTest(t)

	err := runInstallOp(root, "test-prov", "", "", "library", "", "", false)
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

	// toSlug="all" dispatches to runInstallAll.
	err := runInstallOp(root, "all", "skills", "", "shared", "", "", true)
	if err != nil {
		t.Fatalf("export to 'all' failed: %v", err)
	}
	// runInstallAll prints the summary section.
	if !strings.Contains(stdout.String(), "Install All Summary") {
		t.Errorf("expected 'Install All Summary' from runInstallAll, got: %s", stdout.String())
	}
}

// withStubbedSyncAll overrides syncAllRegistries for the test duration.
func withStubbedSyncAll(t *testing.T, stub func([]string) []registry.SyncResult) {
	t.Helper()
	orig := syncAllRegistries
	syncAllRegistries = stub
	t.Cleanup(func() { syncAllRegistries = orig })
}

func TestSyncAndExport_WithRegistries_Success(t *testing.T) {
	// Registries configured → sync runs, all succeed, then export runs.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)
	installBase := t.TempDir()
	origP := append([]provider.Provider(nil), provider.AllProviders...)
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
	t.Cleanup(func() { provider.AllProviders = origP })

	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"registries":[{"name":"foo","url":"https://example.invalid"}]}`), 0644)

	var syncedNames []string
	withStubbedSyncAll(t, func(names []string) []registry.SyncResult {
		syncedNames = names
		out := make([]registry.SyncResult, len(names))
		for i, n := range names {
			out[i] = registry.SyncResult{Name: n, Err: nil}
		}
		return out
	})

	stdout, _ := output.SetForTest(t)

	syncInstallCmd.Flags().Set("to", "test-prov")
	defer syncInstallCmd.Flags().Set("to", "")
	syncInstallCmd.Flags().Set("type", "skills")
	defer syncInstallCmd.Flags().Set("type", "")
	syncInstallCmd.Flags().Set("source", "shared")
	defer syncInstallCmd.Flags().Set("source", "local")

	err := syncInstallCmd.RunE(syncInstallCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syncedNames) != 1 || syncedNames[0] != "foo" {
		t.Errorf("expected syncAllRegistries([\"foo\"]), got %v", syncedNames)
	}
	out := stdout.String()
	if !strings.Contains(out, "Syncing 1 registries") {
		t.Errorf("expected 'Syncing 1 registries' in output, got: %s", out)
	}
	if !strings.Contains(out, "Synced: foo") {
		t.Errorf("expected 'Synced: foo' in output, got: %s", out)
	}
}

func TestSyncAndExport_SyncFailure_ReturnsError(t *testing.T) {
	// Registry sync fails → runSyncAndExport returns structured error without exporting.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)
	origP := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name:         "TestProv",
			Slug:         "test-prov",
			SupportsType: func(catalog.ContentType) bool { return true },
		},
	}
	t.Cleanup(func() { provider.AllProviders = origP })

	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"registries":[{"name":"broken","url":"https://example.invalid"}]}`), 0644)

	withStubbedSyncAll(t, func(names []string) []registry.SyncResult {
		return []registry.SyncResult{{Name: names[0], Err: fmt.Errorf("network unreachable")}}
	})

	output.SetForTest(t)
	syncInstallCmd.Flags().Set("to", "test-prov")
	defer syncInstallCmd.Flags().Set("to", "")

	err := syncInstallCmd.RunE(syncInstallCmd, []string{})
	if err == nil {
		t.Fatal("expected error from sync failure, got nil")
	}
	if !strings.Contains(err.Error(), "sync failed") {
		t.Errorf("expected 'sync failed' in error, got: %v", err)
	}
}

func TestSyncAndExport_JSONOutputSuppressesSyncBanner(t *testing.T) {
	// In JSON mode, "Syncing N registries" text is suppressed from stdout.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)
	installBase := t.TempDir()
	origP := append([]provider.Provider(nil), provider.AllProviders...)
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
	t.Cleanup(func() { provider.AllProviders = origP })

	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"registries":[{"name":"foo","url":"https://example.invalid"}]}`), 0644)

	withStubbedSyncAll(t, func(names []string) []registry.SyncResult {
		return []registry.SyncResult{{Name: "foo", Err: nil}}
	})

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	syncInstallCmd.Flags().Set("to", "test-prov")
	defer syncInstallCmd.Flags().Set("to", "")
	syncInstallCmd.Flags().Set("type", "skills")
	defer syncInstallCmd.Flags().Set("type", "")
	syncInstallCmd.Flags().Set("source", "shared")
	defer syncInstallCmd.Flags().Set("source", "local")

	err := syncInstallCmd.RunE(syncInstallCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, "Syncing") {
		t.Errorf("expected no 'Syncing' banner in JSON output, got: %s", out)
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

	syncInstallCmd.Flags().Set("to", "test-prov")
	defer syncInstallCmd.Flags().Set("to", "")
	syncInstallCmd.Flags().Set("type", "skills")
	defer syncInstallCmd.Flags().Set("type", "")
	syncInstallCmd.Flags().Set("source", "shared")
	defer syncInstallCmd.Flags().Set("source", "local")

	err := syncInstallCmd.RunE(syncInstallCmd, []string{})
	if err != nil {
		t.Fatalf("sync-install failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected exported skill 'greeting' in output, got: %s", out)
	}
}

func TestConvertHooksForExport_GenerateHintOnlyWhereGenerateHelps(t *testing.T) {
	content := []byte(`{"hooks":{"PreToolUse":[{"hooks":[{"type":"prompt","prompt":"Check safety"}]}]}}`)
	cases := []struct {
		toSlug   string
		wantHint bool
	}{
		{"gemini-cli", true},
		// Crush runs only shell commands, so generate mode drops the hook too.
		{"crush", false},
	}
	for _, tc := range cases {
		t.Run(tc.toSlug, func(t *testing.T) {
			res, err := convertHooksForExport(content, "claude-code", tc.toSlug, converter.LLMHooksModeSkip)
			if err != nil {
				t.Fatalf("convertHooksForExport: %v", err)
			}
			gotHint := strings.Contains(strings.Join(res.Warnings, "\n"), "--llm-hooks=generate")
			if gotHint != tc.wantHint {
				t.Errorf("generate hint = %v, want %v; warnings: %v", gotHint, tc.wantHint, res.Warnings)
			}
		})
	}
}
