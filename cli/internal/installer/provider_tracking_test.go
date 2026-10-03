package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

// isolateLegacyRoot points the legacy installed.json root at an empty temp
// dir, so a developer's real ~/.syllago/content records never leak in. Not
// parallel-safe (mutates a package global).
func isolateLegacyRoot(t *testing.T) string {
	t.Helper()
	legacyRoot := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = legacyRoot
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
	return legacyRoot
}

// overrideHookSettingsPaths gives each provider slug its own hook file. Not
// parallel-safe (mutates a package global).
func overrideHookSettingsPaths(t *testing.T, paths map[string]string) {
	t.Helper()
	orig := hookSettingsPath
	hookSettingsPath = func(prov provider.Provider) (string, error) { return paths[prov.Slug], nil }
	t.Cleanup(func() { hookSettingsPath = orig })
}

// overrideMCPConfigPaths gives each provider slug its own MCP config file.
// Not parallel-safe (mutates a package global).
func overrideMCPConfigPaths(t *testing.T, paths map[string]string) {
	t.Helper()
	orig := mcpConfigPath
	mcpConfigPath = func(prov provider.Provider, _ string) (string, error) { return paths[prov.Slug], nil }
	t.Cleanup(func() { mcpConfigPath = orig })
}

// hookTestPaths returns a settings file per provider under one temp dir.
func hookTestPaths(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	return map[string]string{
		"claude-code": filepath.Join(dir, "claude-settings.json"),
		"cursor":      filepath.Join(dir, "cursor-hooks.json"),
	}
}

// The same hook installs to two providers that share the PreToolUse event
// name. Before records carried a provider, the second install failed with
// "already installed" because the claude-code record matched.
func TestInstallHook_SameHookOnTwoProviders(t *testing.T) {
	isolateLegacyRoot(t)
	paths := hookTestPaths(t)
	overrideHookSettingsPaths(t, paths)
	projectRoot := t.TempDir()
	item := writeCanonicalHookItemInProject(t, projectRoot, "shared-hook", "PreToolUse", "Bash", "echo shared")

	if _, err := installHook(item, provider.ClaudeCode, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("install to claude-code: %v", err)
	}
	if _, err := installHook(item, provider.Cursor, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("install to cursor: %v", err)
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.Hooks) != 2 {
		t.Fatalf("expected 2 hook records, got %+v", inst.Hooks)
	}
	if inst.Hooks[0].Provider != "claude-code" || inst.Hooks[1].Provider != "cursor" {
		t.Errorf("record providers = %q, %q; want claude-code, cursor", inst.Hooks[0].Provider, inst.Hooks[1].Provider)
	}

	// A second install to the same provider is still a duplicate.
	_, err = installHook(item, provider.Cursor, projectRoot, ScanOptions{})
	if err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Errorf("reinstall to cursor: got %v, want an 'already installed' error", err)
	}
}

// Uninstalling from one provider removes that provider's record and hook and
// leaves the other provider's untouched.
func TestUninstallHook_LeavesOtherProvider(t *testing.T) {
	isolateLegacyRoot(t)
	paths := hookTestPaths(t)
	overrideHookSettingsPaths(t, paths)
	projectRoot := t.TempDir()
	item := writeCanonicalHookItemInProject(t, projectRoot, "shared-hook", "PreToolUse", "Bash", "echo shared")

	for _, prov := range []provider.Provider{provider.ClaudeCode, provider.Cursor} {
		if _, err := installHook(item, prov, projectRoot, ScanOptions{}); err != nil {
			t.Fatalf("install to %s: %v", prov.Slug, err)
		}
	}
	if _, err := uninstallHook(item, provider.Cursor, projectRoot); err != nil {
		t.Fatalf("uninstall from cursor: %v", err)
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.Hooks) != 1 || inst.Hooks[0].Provider != "claude-code" {
		t.Fatalf("expected only the claude-code record to remain, got %+v", inst.Hooks)
	}
	if status := CheckStatus(item, provider.ClaudeCode, projectRoot); status != StatusInstalled {
		t.Errorf("claude-code status = %v, want StatusInstalled", status)
	}
	if status := CheckStatus(item, provider.Cursor, projectRoot); status != StatusNotInstalled {
		t.Errorf("cursor status = %v, want StatusNotInstalled", status)
	}
}

// A record written before records carried a provider still uninstalls.
func TestUninstallHook_LegacyRecordWithoutProvider(t *testing.T) {
	isolateLegacyRoot(t)
	paths := hookTestPaths(t)
	overrideHookSettingsPaths(t, paths)
	projectRoot := t.TempDir()
	item := writeCanonicalHookItemInProject(t, projectRoot, "old-hook", "PreToolUse", "Bash", "echo old")

	if _, err := installHook(item, provider.ClaudeCode, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	stripRecordedProviders(t, projectRoot)

	if _, err := uninstallHook(item, provider.ClaudeCode, projectRoot); err != nil {
		t.Fatalf("uninstall legacy record: %v", err)
	}
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.Hooks) != 0 {
		t.Errorf("legacy record not removed: %+v", inst.Hooks)
	}
	data, err := os.ReadFile(paths["claude-code"])
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(data, "hooks.PreToolUse.0").Exists() {
		t.Errorf("hook not removed from settings: %s", data)
	}
}

// A legacy record blocks a reinstall only on a provider whose settings
// already hold the hook; on any other provider the install goes ahead.
func TestInstallHook_LegacyRecordDedup(t *testing.T) {
	isolateLegacyRoot(t)
	paths := hookTestPaths(t)
	overrideHookSettingsPaths(t, paths)
	projectRoot := t.TempDir()
	item := writeCanonicalHookItemInProject(t, projectRoot, "old-hook", "PreToolUse", "Bash", "echo old")

	if _, err := installHook(item, provider.ClaudeCode, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	stripRecordedProviders(t, projectRoot)

	_, err := installHook(item, provider.ClaudeCode, projectRoot, ScanOptions{})
	if err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Errorf("reinstall to claude-code: got %v, want an 'already installed' error", err)
	}
	if _, err := installHook(item, provider.Cursor, projectRoot, ScanOptions{}); err != nil {
		t.Errorf("install to cursor beside a legacy claude-code record: %v", err)
	}
}

// stripRecordedProviders rewrites installed.json as a version that predates
// provider tracking would have written it.
func stripRecordedProviders(t *testing.T, projectRoot string) {
	t.Helper()
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	for i := range inst.Hooks {
		inst.Hooks[i].Provider = ""
	}
	for i := range inst.MCP {
		inst.MCP[i].Provider = ""
	}
	if err := SaveInstalled(projectRoot, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}
}

// writeMCPItem writes a flat single-server MCP item.
func writeMCPItem(t *testing.T, root, name string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "mcp", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"command":"node"}`), 0644); err != nil {
		t.Fatal(err)
	}
	return catalog.ContentItem{Name: name, Type: catalog.MCP, Path: dir, ServerKey: name}
}

// Uninstalling an MCP server from one provider removes that provider's
// record. Before records carried a provider, it removed the first record for
// the item, which belonged to whichever provider was installed first.
func TestUninstallMCP_LeavesOtherProvider(t *testing.T) {
	isolateLegacyRoot(t)
	dir := t.TempDir()
	paths := map[string]string{
		"claude-code": filepath.Join(dir, "claude.json"),
		"cursor":      filepath.Join(dir, "cursor-mcp.json"),
	}
	overrideMCPConfigPaths(t, paths)
	projectRoot := t.TempDir()
	item := writeMCPItem(t, projectRoot, "shared-mcp")

	for _, prov := range []provider.Provider{provider.ClaudeCode, provider.Cursor} {
		if _, err := installMCP(item, prov, projectRoot); err != nil {
			t.Fatalf("install to %s: %v", prov.Slug, err)
		}
	}
	if _, err := uninstallMCP(item, provider.Cursor, projectRoot); err != nil {
		t.Fatalf("uninstall from cursor: %v", err)
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.MCP) != 1 || inst.MCP[0].Provider != "claude-code" {
		t.Fatalf("expected only the claude-code record to remain, got %+v", inst.MCP)
	}
	claude, err := os.ReadFile(paths["claude-code"])
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.GetBytes(claude, "mcpServers.shared-mcp").Exists() {
		t.Errorf("claude-code server removed: %s", claude)
	}
	cursor, err := os.ReadFile(paths["cursor"])
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(cursor, "mcpServers.shared-mcp").Exists() {
		t.Errorf("cursor server not removed: %s", cursor)
	}
}

// A server another provider's record claims is not syllago-managed on a
// provider where the user defined it by hand, so install refuses to
// overwrite it.
func TestInstallMCP_OtherProviderRecordDoesNotClaimServer(t *testing.T) {
	isolateLegacyRoot(t)
	dir := t.TempDir()
	paths := map[string]string{
		"claude-code": filepath.Join(dir, "claude.json"),
		"cursor":      filepath.Join(dir, "cursor-mcp.json"),
	}
	overrideMCPConfigPaths(t, paths)
	if err := os.WriteFile(paths["cursor"], []byte(`{"mcpServers":{"shared-mcp":{"command":"mine"}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	item := writeMCPItem(t, projectRoot, "shared-mcp")

	if _, err := installMCP(item, provider.ClaudeCode, projectRoot); err != nil {
		t.Fatalf("install to claude-code: %v", err)
	}
	_, err := installMCP(item, provider.Cursor, projectRoot)
	if err == nil || !strings.Contains(err.Error(), "was not installed by syllago") {
		t.Fatalf("install to cursor over a hand-written server: got %v, want a collision error", err)
	}
	cursor, err := os.ReadFile(paths["cursor"])
	if err != nil {
		t.Fatal(err)
	}
	if got := gjson.GetBytes(cursor, "mcpServers.shared-mcp.command").String(); got != "mine" {
		t.Errorf("hand-written server overwritten: command = %q", got)
	}
}

// Records written under a retired provider slug load under its current slug,
// so lookups by the current slug find them.
func TestLoadInstalled_ResolvesRetiredProviderSlugs(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	if err := SaveInstalled(projectRoot, &Installed{
		Hooks: []InstalledHook{{Name: "h", Event: "PreToolUse", Provider: "windsurf"}},
		MCP:   []InstalledMCP{{Name: "m", Provider: "windsurf"}},
	}); err != nil {
		t.Fatal(err)
	}
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Hooks[0].Provider != "devin" || inst.MCP[0].Provider != "devin" {
		t.Errorf("providers = %q, %q; want devin", inst.Hooks[0].Provider, inst.MCP[0].Provider)
	}
}

// When a legacy per-server record and this provider's bulk record (written by
// loadout apply) both name the item, uninstall removes this provider's record
// and leaves the legacy one.
func TestUninstallMCP_PrefersProviderRecordOverLegacyPerServer(t *testing.T) {
	isolateLegacyRoot(t)
	dir := t.TempDir()
	paths := map[string]string{"cursor": filepath.Join(dir, "cursor-mcp.json")}
	overrideMCPConfigPaths(t, paths)
	if err := os.WriteFile(paths["cursor"], []byte(`{"mcpServers":{"shared-mcp":{"command":"node"}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	item := writeMCPItem(t, projectRoot, "shared-mcp")
	if err := SaveInstalled(projectRoot, &Installed{MCP: []InstalledMCP{
		{Name: "shared-mcp", ServerKey: "shared-mcp"},
		{Name: "shared-mcp", ServerNames: []string{"shared-mcp"}, Provider: "cursor"},
	}}); err != nil {
		t.Fatal(err)
	}

	if _, err := uninstallMCP(item, provider.Cursor, projectRoot); err != nil {
		t.Fatalf("uninstall from cursor: %v", err)
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.MCP) != 1 || inst.MCP[0].Provider != "" {
		t.Fatalf("expected only the legacy record to remain, got %+v", inst.MCP)
	}
}

// A server another provider's record tracks is an orphan on a provider whose
// settings hold it untracked. A legacy record with no provider tracks it on
// every provider.
func TestCheckOrphanedMerges_ProviderScopedRecords(t *testing.T) {
	for _, tc := range []struct {
		name        string
		recordProv  string
		wantOrphans int
	}{
		{"other provider's record", "cursor", 1},
		{"legacy record", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			if err := os.MkdirAll(filepath.Join(home, ".orphan-test"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".orphan-test", "settings.json"),
				[]byte(`{"mcpServers":{"srv":{"command":"node"}}}`), 0644); err != nil {
				t.Fatal(err)
			}
			projectRoot := t.TempDir()
			if err := SaveInstalled(projectRoot, &Installed{MCP: []InstalledMCP{
				{Name: "srv", ServerKey: "srv", Provider: tc.recordProv},
			}}); err != nil {
				t.Fatal(err)
			}
			prov := provider.Provider{Name: "Test", Slug: "test", ConfigDir: ".orphan-test", Detected: true}

			orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
			if err != nil {
				t.Fatalf("CheckOrphanedMerges: %v", err)
			}
			if len(orphans) != tc.wantOrphans {
				t.Fatalf("got %d orphans %+v, want %d", len(orphans), orphans, tc.wantOrphans)
			}
		})
	}
}

// A record with no provider whose server is missing from this provider's
// config may belong to another provider, so uninstall refuses and keeps it.
func TestUninstallMCP_LegacyRecordMissingFromTarget(t *testing.T) {
	isolateLegacyRoot(t)
	dir := t.TempDir()
	paths := map[string]string{"cursor": filepath.Join(dir, "cursor-mcp.json")}
	overrideMCPConfigPaths(t, paths)
	if err := os.WriteFile(paths["cursor"], []byte(`{"mcpServers":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	item := writeMCPItem(t, projectRoot, "shared-mcp")
	if err := SaveInstalled(projectRoot, &Installed{MCP: []InstalledMCP{
		{Name: "shared-mcp", ServerKey: "shared-mcp"},
	}}); err != nil {
		t.Fatal(err)
	}

	if _, err := uninstallMCP(item, provider.Cursor, projectRoot); err == nil {
		t.Fatal("uninstall from cursor succeeded, want a not-installed error")
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.MCP) != 1 {
		t.Fatalf("legacy record removed: %+v", inst.MCP)
	}
}
