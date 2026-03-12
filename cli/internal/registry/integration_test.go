package registry

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// ---------------------------------------------------------------------------
// Test Helpers
// ---------------------------------------------------------------------------

// setupCacheOverride redirects the registry cache to a temp dir and restores
// the original value when the test finishes.
func setupCacheOverride(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := CacheDirOverride
	CacheDirOverride = dir
	t.Cleanup(func() { CacheDirOverride = old })
	return dir
}

// requireGit skips the test if git is not on PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH, skipping integration test")
	}
}

// run executes a command in the given directory and fails the test on error.
func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s\n%s", name, args, err, string(out))
	}
}

// writeFile creates a file (and parent directories) in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// createBareRepo creates a local bare git repo with content matching the
// given layout. Returns the path to the bare repo (usable as a clone URL).
func createBareRepo(t *testing.T, layout string) string {
	t.Helper()

	// Create a working repo, add content, then clone --bare
	work := filepath.Join(t.TempDir(), "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	run(t, work, "git", "init")
	run(t, work, "git", "config", "user.email", "test@test.com")
	run(t, work, "git", "config", "user.name", "Test")

	switch layout {
	case "valid":
		writeFile(t, work, "registry.yaml", "name: test-registry\ndescription: Test registry\nversion: \"1.0.0\"\n")
		writeFile(t, work, "skills/hello-world/SKILL.md", "---\nname: Hello World\ndescription: A test skill\n---\n\n# Hello World\n\nTest skill body.\n")
		writeFile(t, work, "agents/test-agent/AGENT.md", "---\nname: Test Agent\ndescription: A test agent\n---\n\n# Test Agent\n\nTest agent body.\n")
		writeFile(t, work, "rules/claude-code/test-rule/rule.md", "---\ndescription: Test rule\nalwaysApply: true\n---\n\n# Test Rule\n\nTest rule body.\n")
		writeFile(t, work, "rules/claude-code/test-rule/README.md", "# test-rule\n")

	case "kitchen-sink":
		// Full coverage: all 7 content types, multiple providers
		writeFile(t, work, "registry.yaml", "name: kitchen-sink\ndescription: Comprehensive test registry\nversion: \"1.0.0\"\nmin_syllago_version: \"0.6.0\"\nmaintainers:\n  - test\n")

		// Skills (4)
		writeFile(t, work, "skills/minimal-skill/SKILL.md", "---\nname: Minimal Skill\ndescription: Minimal\n---\n\nBody.\n")
		writeFile(t, work, "skills/full-skill/SKILL.md", "---\nname: Full Skill\ndescription: Full\nallowed-tools:\n  - Read\n  - Bash\ncontext: fork\nagent: Explore\nmodel: claude-sonnet-4-20250514\n---\n\nBody.\n")
		writeFile(t, work, "skills/full-skill/.syllago.yaml", "id: full-skill\nname: full-skill\ndescription: Full\nversion: \"1.0.0\"\nauthor: Test\ntags:\n  - test\n  - comprehensive\n")
		writeFile(t, work, "skills/user-invocable-skill/SKILL.md", "---\nname: User Invocable\ndescription: Invocable\nuser-invocable: true\nargument-hint: <target>\n---\n\nBody.\n")
		writeFile(t, work, "skills/restricted-skill/SKILL.md", "---\nname: Restricted\ndescription: Restricted\ndisallowed-tools:\n  - Bash\n  - Write\ndisable-model-invocation: true\n---\n\nBody.\n")

		// Agents (4)
		writeFile(t, work, "agents/minimal-agent/AGENT.md", "---\nname: Minimal Agent\ndescription: Minimal\n---\n\nBody.\n")
		writeFile(t, work, "agents/claude-full-agent/AGENT.md", "---\nname: Claude Full Agent\ndescription: Claude-specific\ntools:\n  - Read\n  - Glob\nmodel: claude-sonnet-4-20250514\nmaxTurns: 25\npermissionMode: plan\nskills:\n  - full-skill\nmcpServers:\n  - stdio-server\nmemory: project\nbackground: false\nisolation: worktree\n---\n\nBody.\n")
		writeFile(t, work, "agents/gemini-style-agent/AGENT.md", "---\nname: Gemini Style Agent\ndescription: Gemini-specific\ntools:\n  - Read\nmodel: gemini-2.5-pro\nmaxTurns: 10\ntemperature: 0.7\ntimeout_mins: 30\nkind: coding\n---\n\nBody.\n")
		writeFile(t, work, "agents/multi-tool-agent/AGENT.md", "---\nname: Multi-Tool Agent\ndescription: Multi-tool\ntools:\n  - Read\n  - Write\n  - Bash\ndisallowedTools:\n  - NotebookEdit\n---\n\nBody.\n")

		// MCP (3)
		writeFile(t, work, "mcp/stdio-server/config.json", `{"mcpServers":{"filesystem":{"command":"npx","args":["-y","@mcp/server-filesystem"],"type":"stdio","autoApprove":["read_file"]}}}`)
		writeFile(t, work, "mcp/http-server/config.json", `{"mcpServers":{"remote":{"url":"https://mcp.example.com","headers":{"Authorization":"Bearer tok"},"type":"streamable-http"}}}`)
		writeFile(t, work, "mcp/filtered-server/config.json", `{"mcpServers":{"filtered":{"command":"npx","args":["@example/server"],"type":"stdio","trust":"full","includeTools":["search"],"excludeTools":["admin_delete"]}}}`)

		// Rules (11 — one per provider)
		providers := []string{"claude-code", "gemini-cli", "cursor", "windsurf", "codex", "copilot-cli", "zed", "cline", "roo-code", "opencode", "kiro"}
		for _, p := range providers {
			writeFile(t, work, "rules/"+p+"/code-style/rule.md", "---\ndescription: Code style\nalwaysApply: true\n---\n\n# Code Style\n\nTest rule.\n")
			writeFile(t, work, "rules/"+p+"/code-style/README.md", "# code-style\n")
		}

		// Hooks (4)
		hookProviders := []string{"claude-code", "gemini-cli", "copilot-cli", "kiro"}
		for _, p := range hookProviders {
			writeFile(t, work, "hooks/"+p+"/post-edit-lint/hook.json", `{"hooks":{"PostToolUse":[{"matcher":"Write|Edit","hooks":[{"type":"command","command":"echo lint","timeout":15000}]}]}}`)
			writeFile(t, work, "hooks/"+p+"/post-edit-lint/README.md", "# post-edit-lint\n")
		}

		// Commands (5)
		cmdProviders := []string{"claude-code", "gemini-cli", "codex", "copilot-cli", "opencode"}
		for _, p := range cmdProviders {
			writeFile(t, work, "commands/"+p+"/greet/command.md", "---\nname: greet\ndescription: Greet command\nuser-invocable: true\n---\n\n# /greet\n\nGreet the user.\n")
			writeFile(t, work, "commands/"+p+"/greet/README.md", "# greet\n")
		}

		// Loadouts (1)
		writeFile(t, work, "loadouts/claude-code/full-stack/loadout.yaml", "kind: loadout\nversion: 1\nprovider: claude-code\nname: full-stack\ndescription: Full loadout\nrules:\n  - code-style\nskills:\n  - full-skill\nagents:\n  - claude-full-agent\n")
		writeFile(t, work, "loadouts/claude-code/full-stack/README.md", "# full-stack\n")

	case "native":
		writeFile(t, work, ".cursorrules", "Be concise.\n")
		writeFile(t, work, "CLAUDE.md", "# Instructions\n\nBe helpful.\n")

	case "empty":
		writeFile(t, work, "README.md", "# Empty Registry\n")
	}

	run(t, work, "git", "add", "-A")
	run(t, work, "git", "commit", "-m", "initial")

	// Clone to bare repo
	bare := filepath.Join(t.TempDir(), "bare.git")
	run(t, "", "git", "clone", "--bare", work, bare)

	return bare
}

// ---------------------------------------------------------------------------
// Core Registry Operation Tests (local bare repos, offline)
// ---------------------------------------------------------------------------

func TestIntegration_CloneAndDiscover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "valid")
	if err := Clone(bare, "test-reg", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	if !IsCloned("test-reg") {
		t.Fatal("expected IsCloned=true after clone")
	}

	m, err := LoadManifest("test-reg")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m == nil || m.Name != "test-registry" {
		t.Fatalf("manifest name = %v, want 'test-registry'", m)
	}
	if m.Version != "1.0.0" {
		t.Errorf("manifest version = %q, want '1.0.0'", m.Version)
	}

	dir, _ := CloneDir("test-reg")
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: "test-reg", Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}

	count := cat.CountRegistry("test-reg")
	if count != 3 {
		t.Errorf("CountRegistry = %d, want 3 (skill + agent + rule)", count)
	}
}

func TestIntegration_Remove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "valid")
	if err := Clone(bare, "test-reg", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	dir, _ := CloneDir("test-reg")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("clone dir should exist: %v", err)
	}

	if err := Remove("test-reg"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if IsCloned("test-reg") {
		t.Fatal("expected IsCloned=false after remove")
	}
}

func TestIntegration_Sync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "valid")
	if err := Clone(bare, "test-reg", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Push a new skill to the bare repo via a temp workspace
	workspace := filepath.Join(t.TempDir(), "workspace")
	run(t, "", "git", "clone", bare, workspace)
	run(t, workspace, "git", "config", "user.email", "test@test.com")
	run(t, workspace, "git", "config", "user.name", "Test")
	writeFile(t, workspace, "skills/new-skill/SKILL.md", "---\nname: New Skill\ndescription: Added after clone\n---\n\nNew skill.\n")
	run(t, workspace, "git", "add", "-A")
	run(t, workspace, "git", "commit", "-m", "add new skill")
	run(t, workspace, "git", "push")

	// Sync should pull the new commit
	if err := Sync("test-reg"); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Re-scan — new skill should appear
	dir, _ := CloneDir("test-reg")
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: "test-reg", Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}
	count := cat.CountRegistry("test-reg")
	if count != 4 {
		t.Errorf("CountRegistry after sync = %d, want 4", count)
	}
}

func TestIntegration_DuplicateClone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "valid")
	if err := Clone(bare, "test-reg", ""); err != nil {
		t.Fatalf("first Clone: %v", err)
	}

	err := Clone(bare, "test-reg", "")
	if err == nil {
		t.Fatal("expected error on duplicate clone, got nil")
	}
}

func TestIntegration_ConfigPersistence(t *testing.T) {
	dir := t.TempDir()
	syllagoDir := filepath.Join(dir, ".syllago")
	if err := os.MkdirAll(syllagoDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "acme/tools", URL: "https://github.com/acme/tools.git"},
			{Name: "bob/rules", URL: "https://github.com/bob/rules.git"},
		},
	}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Registries) != 2 {
		t.Fatalf("loaded %d registries, want 2", len(loaded.Registries))
	}
	if loaded.Registries[0].Name != "acme/tools" {
		t.Errorf("registry[0].Name = %q, want 'acme/tools'", loaded.Registries[0].Name)
	}

	// Remove one and round-trip
	loaded.Registries = loaded.Registries[:1]
	if err := config.Save(dir, loaded); err != nil {
		t.Fatalf("Save after remove: %v", err)
	}
	reloaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load after remove: %v", err)
	}
	if len(reloaded.Registries) != 1 {
		t.Fatalf("reloaded %d registries, want 1", len(reloaded.Registries))
	}
}

func TestIntegration_CloneEmptyRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "empty")
	if err := Clone(bare, "test-empty", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	dir, _ := CloneDir("test-empty")
	result := catalog.ScanNativeContent(dir)
	if result.HasSyllagoStructure {
		t.Error("expected HasSyllagoStructure=false for empty repo")
	}
	if len(result.Providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(result.Providers))
	}

	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: "test-empty", Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}
	if cat.CountRegistry("test-empty") != 0 {
		t.Errorf("expected 0 items in empty registry, got %d", cat.CountRegistry("test-empty"))
	}
}

func TestIntegration_CloneNativeRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "native")
	if err := Clone(bare, "test-native", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	dir, _ := CloneDir("test-native")
	result := catalog.ScanNativeContent(dir)
	if result.HasSyllagoStructure {
		t.Error("expected HasSyllagoStructure=false for native repo")
	}
	if len(result.Providers) == 0 {
		t.Error("expected providers to be detected in native repo")
	}
}

// ---------------------------------------------------------------------------
// Kitchen-Sink Tests — All 7 Content Types
// ---------------------------------------------------------------------------

func TestIntegration_KitchenSink_AllContentTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "kitchen-sink")
	if err := Clone(bare, "kitchen-sink", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	dir, _ := CloneDir("kitchen-sink")
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: "kitchen-sink", Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}

	total := cat.CountRegistry("kitchen-sink")
	if total != 32 {
		t.Errorf("total items = %d, want 32", total)
	}

	// Verify counts per content type
	tests := []struct {
		ct   catalog.ContentType
		want int
	}{
		{catalog.Skills, 4},
		{catalog.Agents, 4},
		{catalog.MCP, 3},
		{catalog.Rules, 11},
		{catalog.Hooks, 4},
		{catalog.Commands, 5},
		{catalog.Loadouts, 1},
	}
	for _, tt := range tests {
		t.Run(string(tt.ct), func(t *testing.T) {
			items := cat.ByType(tt.ct)
			if len(items) != tt.want {
				t.Errorf("ByType(%s) = %d items, want %d", tt.ct, len(items), tt.want)
			}
		})
	}
}

func TestIntegration_KitchenSink_Manifest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "kitchen-sink")
	if err := Clone(bare, "kitchen-sink", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	m, err := LoadManifest("kitchen-sink")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.Name != "kitchen-sink" {
		t.Errorf("Name = %q, want 'kitchen-sink'", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want '1.0.0'", m.Version)
	}
	if m.MinSyllagoVersion != "0.6.0" {
		t.Errorf("MinSyllagoVersion = %q, want '0.6.0'", m.MinSyllagoVersion)
	}
	if len(m.Maintainers) != 1 {
		t.Errorf("Maintainers = %d, want 1", len(m.Maintainers))
	}
}

func TestIntegration_KitchenSink_SkillMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "kitchen-sink")
	if err := Clone(bare, "kitchen-sink", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	dir, _ := CloneDir("kitchen-sink")
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: "kitchen-sink", Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}

	skills := cat.ByType(catalog.Skills)
	var fullSkill *catalog.ContentItem
	for i := range skills {
		if skills[i].Name == "full-skill" {
			fullSkill = &skills[i]
			break
		}
	}
	if fullSkill == nil {
		t.Fatal("full-skill not found in scan results")
	}
	if fullSkill.Description != "Full" {
		t.Errorf("description = %q, want 'Full'", fullSkill.Description)
	}
	if fullSkill.Registry != "kitchen-sink" {
		t.Errorf("registry = %q, want 'kitchen-sink'", fullSkill.Registry)
	}
}

func TestIntegration_KitchenSink_RulesAllProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "kitchen-sink")
	if err := Clone(bare, "kitchen-sink", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	dir, _ := CloneDir("kitchen-sink")
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: "kitchen-sink", Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}

	rules := cat.ByType(catalog.Rules)
	if len(rules) != 11 {
		t.Fatalf("expected 11 rules (one per provider), got %d", len(rules))
	}

	// Verify each rule has a provider set
	providers := make(map[string]bool)
	for _, r := range rules {
		if r.Provider == "" {
			t.Errorf("rule %q has empty Provider", r.Name)
		}
		providers[r.Provider] = true
	}

	expected := []string{"claude-code", "gemini-cli", "cursor", "windsurf", "codex", "copilot-cli", "zed", "cline", "roo-code", "opencode", "kiro"}
	for _, p := range expected {
		if !providers[p] {
			t.Errorf("missing rule for provider %q", p)
		}
	}
}

func TestIntegration_KitchenSink_ByRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireGit(t)
	setupCacheOverride(t)

	bare := createBareRepo(t, "kitchen-sink")
	if err := Clone(bare, "kitchen-sink", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	dir, _ := CloneDir("kitchen-sink")
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: "kitchen-sink", Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}

	items := cat.ByRegistry("kitchen-sink")
	if len(items) != 32 {
		t.Errorf("ByRegistry = %d, want 32", len(items))
	}

	// All items should be tagged with the registry name
	for _, item := range items {
		if item.Registry != "kitchen-sink" {
			t.Errorf("item %q has registry=%q, want 'kitchen-sink'", item.Name, item.Registry)
		}
	}
}

// ---------------------------------------------------------------------------
// GitHub-backed Comprehensive Test (network required)
// ---------------------------------------------------------------------------

func TestIntegration_GitHubKitchenSink(t *testing.T) {
	if os.Getenv("SYLLAGO_TEST_NETWORK") == "" {
		t.Skip("set SYLLAGO_TEST_NETWORK=1 to run network-dependent tests")
	}
	requireGit(t)
	setupCacheOverride(t)

	url := "https://github.com/OpenScribbler/test-registry-kitchen-sink"
	name := "OpenScribbler/test-registry-kitchen-sink"

	if err := Clone(url, name, ""); err != nil {
		t.Fatalf("Clone from GitHub: %v", err)
	}

	// Verify manifest
	m, err := LoadManifest(name)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m == nil || m.Version != "1.0.0" {
		t.Errorf("unexpected manifest: %+v", m)
	}

	// Scan and verify all 32 items
	dir, _ := CloneDir(name)
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{Name: name, Path: dir}})
	if err != nil {
		t.Fatalf("ScanRegistriesOnly: %v", err)
	}

	total := cat.CountRegistry(name)
	if total != 32 {
		t.Errorf("GitHub kitchen-sink: total items = %d, want 32", total)
	}

	// Verify all 7 content types present
	for _, ct := range catalog.AllContentTypes() {
		items := cat.ByType(ct)
		if len(items) == 0 {
			t.Errorf("no items found for content type %s", ct)
		}
	}

	// Sync should succeed (no new commits, but no error)
	if err := Sync(name); err != nil {
		t.Errorf("Sync: %v", err)
	}

	// Clean up
	if err := Remove(name); err != nil {
		t.Errorf("Remove: %v", err)
	}
	if IsCloned(name) {
		t.Error("expected IsCloned=false after remove")
	}
}
