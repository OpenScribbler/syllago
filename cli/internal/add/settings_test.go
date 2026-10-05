package add

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Two PreToolUse handlers that derive the same name, and one PostToolUse
// handler that runs a script beside the settings file.
const settingsHooksJSON = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo one", "statusMessage": "guard"}]},
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "echo two", "statusMessage": "guard"}]}
    ],
    "PostToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "./check.sh"}]}
    ]
  }
}`

// settingsEnv isolates HOME so discovery never reads the real user's
// provider files, and returns a project root and a Library directory.
func settingsEnv(t *testing.T) (projectRoot, globalDir string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return t.TempDir(), t.TempDir()
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemNames(items []SettingsItem) []string {
	var names []string
	for _, it := range items {
		names = append(names, it.Name)
	}
	return names
}

func TestDiscoverSettings_HooksSuffixSameNameWithinFile(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), settingsHooksJSON)

	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	got := strings.Join(itemNames(items), ",")
	if got != "after-tool-execute-check,guard,guard-2" {
		t.Fatalf("names = %s, want after-tool-execute-check,guard,guard-2", got)
	}
	for _, it := range items {
		if it.Scope != "project" || it.Status != StatusNew {
			t.Errorf("%s: scope %q status %v, want project and new", it.Name, it.Scope, it.Status)
		}
		if want := filepath.Join(globalDir, "hooks", "claude-code", it.Name); it.Dest != want {
			t.Errorf("%s: Dest = %s, want %s", it.Name, it.Dest, want)
		}
		if it.Hook == nil {
			t.Errorf("%s: Hook is nil", it.Name)
		}
	}
	if items[1].Hook.Matcher == items[2].Hook.Matcher {
		t.Error("the two guard items hold the same hook")
	}
}

// Same-named hooks under different events keep their suffixes from one
// discovery to the next, so a re-run finds each hook where it was added.
func TestDiscoverSettings_HookSuffixesAreStable(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo pre", "statusMessage": "guard"}]}],
    "PostToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo post", "statusMessage": "guard"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "echo stop", "statusMessage": "guard"}]}]
  }
}`)
	var first string
	for i := 0; i < 30; i++ {
		items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
		if err != nil {
			t.Fatalf("DiscoverSettings: %v", err)
		}
		var b strings.Builder
		for _, it := range items {
			b.WriteString(it.Name + "=" + it.Hook.Hooks[0].Command + ";")
		}
		if i == 0 {
			first = b.String()
		} else if b.String() != first {
			t.Fatalf("discovery %d numbered hooks %s, first numbered %s", i, b.String(), first)
		}
	}
}

// A suffix never lands on a name another hook in the file derives, so
// each hook keeps a name of its own.
func TestDiscoverSettings_SuffixSkipsADerivedName(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo one", "statusMessage": "guard"}]},
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "echo two", "statusMessage": "guard"}]},
      {"matcher": "Write", "hooks": [{"type": "command", "command": "echo three", "statusMessage": "guard-2"}]}
    ]
  }
}`)
	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	if got := strings.Join(itemNames(items), ","); got != "guard,guard-3,guard-2" {
		t.Fatalf("names = %s, want guard,guard-3,guard-2", got)
	}
}

func TestDiscoverSettings_MCPServers(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{
  "mcpServers": {
    "db": {"command": "db-server", "env": {"TOKEN": "secret"}},
    "web": {"url": "https://example.com/mcp"}
  },
  "unrelated": {"keep": "out"}
}`)

	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	if got := strings.Join(itemNames(items), ","); got != "db,web" {
		t.Fatalf("names = %s, want db,web", got)
	}
	db := items[0]
	if db.ServerKey != "db" {
		t.Errorf("ServerKey %q, want db", db.ServerKey)
	}
	if !strings.Contains(string(db.Server), "db-server") || strings.Contains(string(db.Server), "web") {
		t.Errorf("Server = %s, want the db entry alone", db.Server)
	}
	if want := filepath.Join(globalDir, "mcp", "claude-code", "db"); db.Dest != want {
		t.Errorf("Dest = %s, want %s", db.Dest, want)
	}
}

func TestDiscoverSettings_OpenCodeJSONC(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{
  // servers for this repo
  "mcp": {
    "lint": {"type": "local", "command": ["lint-mcp"]}
  }
}`)

	items, _, err := DiscoverSettings(provider.OpenCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	if len(items) != 1 || items[0].Name != "lint" || !strings.Contains(string(items[0].Server), "lint-mcp") {
		t.Fatalf("items = %+v, want one lint server under mcp", items)
	}
	if items[0].Scope != "project" {
		t.Errorf("Scope = %q, want project", items[0].Scope)
	}
}

func TestDiscoverSettings_RejectsOtherTypes(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	if _, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Rules); err == nil {
		t.Error("DiscoverSettings for rules: err = nil, want an error")
	}
}

// saveScoped writes a Library item directory holding an item added from scope.
func saveScoped(t *testing.T, dir, scope string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(dir, &metadata.Meta{Name: filepath.Base(dir), SourceScope: scope}); err != nil {
		t.Fatal(err)
	}
}

// saveNamed records dir as an item added from settings in scope under
// sourceName.
func saveNamed(t *testing.T, dir, scope, sourceName string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(dir, &metadata.Meta{Name: filepath.Base(dir), SourceScope: scope, SourceName: sourceName}); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsDest(t *testing.T) {
	base := func(globalDir string) string { return filepath.Join(globalDir, "hooks", "claude-code", "guard") }
	tests := []struct {
		name       string
		setup      func(t *testing.T, globalDir string)
		claimed    func(globalDir string) map[string]bool
		wantSuffix string
		wantStatus ItemStatus
	}{
		{
			name:       "nothing there is new at the base",
			wantStatus: StatusNew,
		},
		{
			name:       "same scope is in the Library at the base",
			setup:      func(t *testing.T, g string) { saveScoped(t, base(g), "project") },
			wantStatus: StatusInLibrary,
		},
		{
			name:       "no recorded scope counts as this item",
			setup:      func(t *testing.T, g string) { saveScoped(t, base(g), "") },
			wantStatus: StatusInLibrary,
		},
		{
			name:       "another scope moves to -2",
			setup:      func(t *testing.T, g string) { saveScoped(t, base(g), "global") },
			wantSuffix: "-2",
			wantStatus: StatusNew,
		},
		{
			name: "a re-run finds the earlier -2",
			setup: func(t *testing.T, g string) {
				saveScoped(t, base(g), "global")
				saveNamed(t, base(g)+"-2", "project", "guard")
			},
			wantSuffix: "-2",
			wantStatus: StatusInLibrary,
		},
		{
			name:       "the base holding another item of this scope moves to -2",
			setup:      func(t *testing.T, g string) { saveNamed(t, base(g), "project", "other") },
			wantSuffix: "-2",
			wantStatus: StatusNew,
		},
		{
			name: "a -2 holding a real item of that name is passed over",
			setup: func(t *testing.T, g string) {
				saveScoped(t, base(g), "global")
				saveNamed(t, base(g)+"-2", "project", "guard-2")
			},
			wantSuffix: "-3",
			wantStatus: StatusNew,
		},
		{
			name: "a -2 that records no name is a real item of that name",
			setup: func(t *testing.T, g string) {
				saveScoped(t, base(g), "global")
				saveScoped(t, base(g)+"-2", "project")
			},
			wantSuffix: "-3",
			wantStatus: StatusNew,
		},
		{
			name:       "a held -2 is found though the base is free",
			setup:      func(t *testing.T, g string) { saveNamed(t, base(g)+"-2", "project", "guard") },
			wantSuffix: "-2",
			wantStatus: StatusInLibrary,
		},
		{
			name: "a held -3 is found past a free -2",
			setup: func(t *testing.T, g string) {
				saveScoped(t, base(g), "global")
				saveNamed(t, base(g)+"-3", "project", "guard")
			},
			wantSuffix: "-3",
			wantStatus: StatusInLibrary,
		},
		{
			name:       "a directory claimed in this run is skipped",
			claimed:    func(g string) map[string]bool { return map[string]bool{base(g): true} },
			wantSuffix: "-2",
			wantStatus: StatusNew,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			globalDir := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, globalDir)
			}
			claimed := map[string]bool{}
			if tc.claimed != nil {
				claimed = tc.claimed(globalDir)
			}
			item := SettingsItem{DiscoveryItem: DiscoveryItem{Name: "guard", Type: catalog.Hooks, Scope: "project"}}
			dest, status := settingsDest(globalDir, "claude-code", &item, claimed)
			if want := base(globalDir) + tc.wantSuffix; dest != want {
				t.Errorf("dest = %s, want %s", dest, want)
			}
			if status != tc.wantStatus {
				t.Errorf("status = %v, want %v", status, tc.wantStatus)
			}
		})
	}
}

func discoverHooks(t *testing.T) (items []SettingsItem, projectRoot, globalDir string) {
	t.Helper()
	projectRoot, globalDir = settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), settingsHooksJSON)
	writeFile(t, filepath.Join(projectRoot, ".claude", "check.sh"), "#!/bin/sh\nexit 0\n")
	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	return items, projectRoot, globalDir
}

func TestAddFromSettings_HooksWriteManifestsAndScripts(t *testing.T) {
	items, projectRoot, globalDir := discoverHooks(t)
	results := AddFromSettings(items, AddOptions{Provider: "claude-code"}, projectRoot, globalDir)

	for _, r := range results {
		if r.Status != AddStatusAdded {
			t.Fatalf("%s: status %v err %v, want added", r.Name, r.Status, r.Error)
		}
	}
	var manifest map[string]any
	data, err := os.ReadFile(filepath.Join(results[2].Dest, "hook.json"))
	if err != nil {
		t.Fatalf("read guard-2 hook.json: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("hook.json is not JSON: %v", err)
	}
	if !strings.Contains(string(data), "before_tool_execute") || !strings.Contains(string(data), "echo two") {
		t.Errorf("guard-2 hook.json = %s, want the second PreToolUse handler", data)
	}

	script := results[0]
	if script.Bundled != 1 || script.BundleErr != nil {
		t.Errorf("bundled %d err %v, want 1 script", script.Bundled, script.BundleErr)
	}
	if _, err := os.Stat(filepath.Join(script.Dest, "check.sh")); err != nil {
		t.Errorf("check.sh was not copied beside the hook: %v", err)
	}
	meta, err := metadata.Load(script.Dest)
	if err != nil || meta == nil {
		t.Fatalf("load metadata: %v", err)
	}
	if len(meta.BundledScripts) != 1 || meta.BundledScripts[0].Filename != "check.sh" {
		t.Errorf("BundledScripts = %+v, want check.sh", meta.BundledScripts)
	}
	if meta.Name != "after-tool-execute-check" || meta.SourceScope != "project" || meta.SourceProject != filepath.Base(projectRoot) {
		t.Errorf("meta name %q scope %q project %q", meta.Name, meta.SourceScope, meta.SourceProject)
	}
	if meta.SourceProvider != "claude-code" || meta.SourceType != "provider" || meta.SourceFormat != "json" {
		t.Errorf("meta source %q/%q/%q, want claude-code/provider/json", meta.SourceProvider, meta.SourceType, meta.SourceFormat)
	}
}

// Bundling a script rewrites the manifest's command, never the item the
// caller passed in, so the item can be added again elsewhere.
func TestAddFromSettings_LeavesTheItemAsDiscovered(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{
  "hooks": {"PostToolUse": [{"matcher": "Write", "hooks": [{"type": "command", "command": "./scripts/check.sh"}]}]}
}`)
	writeFile(t, filepath.Join(projectRoot, ".claude", "scripts", "check.sh"), "#!/bin/sh\nexit 0\n")
	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil || len(items) != 1 {
		t.Fatalf("DiscoverSettings: %d items, %v", len(items), err)
	}
	results := AddFromSettings(items, AddOptions{Provider: "claude-code"}, projectRoot, globalDir)
	if results[0].Bundled != 1 {
		t.Fatalf("bundled %d scripts, want 1", results[0].Bundled)
	}
	if got := items[0].Hook.Hooks[0].Command; got != "./scripts/check.sh" {
		t.Errorf("command after the add = %q, want ./scripts/check.sh", got)
	}
}

func TestAddFromSettings_MCPConfigHoldsOneServer(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{
  "mcpServers": {
    "db": {"command": "db-server"},
    "web": {"url": "https://example.com/mcp"}
  },
  "permissions": {"allow": ["Bash"]}
}`)
	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	results := AddFromSettings(items[:1], AddOptions{Provider: "claude-code"}, projectRoot, globalDir)
	if results[0].Status != AddStatusAdded {
		t.Fatalf("status %v err %v, want added", results[0].Status, results[0].Error)
	}

	data, err := os.ReadFile(filepath.Join(results[0].Dest, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var cfg map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config.json is not JSON: %v\n%s", err, data)
	}
	if len(cfg) != 1 || len(cfg["mcpServers"]) != 1 || cfg["mcpServers"]["db"] == nil {
		t.Errorf("config.json = %s, want mcpServers holding db alone", data)
	}

	// The scanner reads the provider-grouped layout as one MCP item.
	cat, err := catalog.Scan(globalDir, t.TempDir())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	found := false
	for _, it := range cat.Items {
		if it.Type == catalog.MCP && it.Name == "db" {
			found = true
		}
	}
	if !found {
		t.Error("the scanned Library holds no db MCP item")
	}
}

func TestAddFromSettings_UpToDateWithoutForce(t *testing.T) {
	items, projectRoot, globalDir := discoverHooks(t)
	AddFromSettings(items, AddOptions{Provider: "claude-code"}, projectRoot, globalDir)
	hookPath := filepath.Join(items[0].Dest, "hook.json")
	writeFile(t, hookPath, `{"edited": true}`)

	again, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", filepath.Dir(filepath.Dir(filepath.Dir(items[0].Dest))), catalog.Hooks)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	if again[0].Status != StatusInLibrary || again[0].Dest != items[0].Dest {
		t.Fatalf("re-discovery: status %v dest %s, want in library at %s", again[0].Status, again[0].Dest, items[0].Dest)
	}

	r := AddFromSettings(again[:1], AddOptions{Provider: "claude-code"}, projectRoot, globalDir)[0]
	if r.Status != AddStatusUpToDate {
		t.Errorf("status = %v, want up to date", r.Status)
	}
	if data, _ := os.ReadFile(hookPath); string(data) != `{"edited": true}` {
		t.Errorf("hook.json changed without force: %s", data)
	}

	r = AddFromSettings(again[:1], AddOptions{Provider: "claude-code", Force: true}, projectRoot, globalDir)[0]
	if r.Status != AddStatusUpdated {
		t.Errorf("forced status = %v, want updated", r.Status)
	}
	if data, _ := os.ReadFile(hookPath); !strings.Contains(string(data), "check.sh") {
		t.Errorf("forced hook.json = %s, want the settings handler", data)
	}
}

func TestAddFromSettings_DryRunWritesNothing(t *testing.T) {
	items, projectRoot, globalDir := discoverHooks(t)
	results := AddFromSettings(items, AddOptions{Provider: "claude-code", DryRun: true}, projectRoot, globalDir)
	for i, r := range results {
		if r.Status != AddStatusAdded {
			t.Errorf("%s: status %v, want added", r.Name, r.Status)
		}
		if _, err := os.Stat(items[i].Dest); !os.IsNotExist(err) {
			t.Errorf("%s: dry run created %s", r.Name, items[i].Dest)
		}
	}
}

func TestAddFromSettings_RegistryAndDisplayName(t *testing.T) {
	items, projectRoot, globalDir := discoverHooks(t)
	item := items[0]
	item.DisplayName = "Bash guard"
	r := AddFromSettings([]SettingsItem{item}, AddOptions{
		Provider:         "claude-code",
		SourceRegistry:   "team-reg",
		SourceVisibility: "private",
		SourceSHA:        "abc123",
	}, projectRoot, globalDir)[0]
	if r.Status != AddStatusAdded {
		t.Fatalf("status %v err %v, want added", r.Status, r.Error)
	}
	meta, err := metadata.Load(item.Dest)
	if err != nil || meta == nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta.Name != "Bash guard" {
		t.Errorf("Name = %q, want the display name", meta.Name)
	}
	if meta.SourceType != "registry" || meta.SourceRegistry != "team-reg" || meta.SourceSHA != "abc123" || meta.SourceVisibility != "private" {
		t.Errorf("registry fields = %q %q %q %q", meta.SourceType, meta.SourceRegistry, meta.SourceSHA, meta.SourceVisibility)
	}
}

func TestAddFromSettings_UnwritableDestIsAnError(t *testing.T) {
	items, projectRoot, globalDir := discoverHooks(t)
	// A file where the provider directory should be blocks MkdirAll.
	writeFile(t, filepath.Join(globalDir, "hooks", "claude-code"), "not a dir")
	r := AddFromSettings(items[:1], AddOptions{Provider: "claude-code"}, projectRoot, globalDir)[0]
	if r.Status != AddStatusError || r.Error == nil {
		t.Errorf("status %v err %v, want an error", r.Status, r.Error)
	}
}

// A hook added beside one from another scope is named after its own
// directory, so the two read apart in the Library.
func TestAddFromSettings_CrossScopeHookNamedForItsDir(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), settingsHooksJSON)
	saveScoped(t, filepath.Join(globalDir, "hooks", "claude-code", "guard"), "global")

	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	guard := items[1]
	if filepath.Base(guard.Dest) != "guard-2" {
		t.Fatalf("Dest = %s, want guard-2 beside the global guard", guard.Dest)
	}
	AddFromSettings([]SettingsItem{guard}, AddOptions{Provider: "claude-code"}, projectRoot, globalDir)
	meta, err := metadata.Load(guard.Dest)
	if err != nil || meta == nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta.Name != "guard-2" {
		t.Errorf("Name = %q, want guard-2", meta.Name)
	}
}

// A server named like another's -2 directory is never taken for it, so
// forcing the add cannot overwrite it.
func TestAddFromSettings_RealDashTwoServerKeptApart(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{"mcpServers": {"db": {"command": "db-server"}}}`)
	dir := filepath.Join(globalDir, "mcp", "claude-code")
	saveNamed(t, filepath.Join(dir, "db"), "global", "db")
	saveNamed(t, filepath.Join(dir, "db-2"), "project", "db-2")
	writeFile(t, filepath.Join(dir, "db-2", "config.json"), `{"mcpServers": {"db-2": {"command": "other"}}}`)

	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	r := AddFromSettings(items, AddOptions{Provider: "claude-code", Force: true}, projectRoot, globalDir)[0]
	if r.Status != AddStatusAdded || filepath.Base(r.Dest) != "db-3" {
		t.Errorf("status %v dest %s, want added at db-3", r.Status, r.Dest)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "db-2", "config.json")); !strings.Contains(string(data), "other") {
		t.Errorf("db-2 was overwritten: %s", data)
	}
}

// A discovered item the caller leaves out claims no directory, so the
// items it does add land where a later add of them alone finds them.
func TestAddFromSettings_LeftOutItemClaimsNothing(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	const servers = `{"mcpServers": {"db": {"command": "db-server"}}}`
	writeFile(t, filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), servers)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), servers)

	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	var project []SettingsItem
	for _, item := range items {
		if item.Scope == "project" {
			project = append(project, item)
		}
	}
	if len(items) != 2 || len(project) != 1 {
		t.Fatalf("discovered %d items, %d project; want 2 and 1", len(items), len(project))
	}
	r := AddFromSettings(project, AddOptions{Provider: "claude-code"}, projectRoot, globalDir)[0]
	if filepath.Base(r.Dest) != "db" {
		t.Errorf("Dest = %s, want db", r.Dest)
	}
}

// A settings file that does not parse is reported rather than read as
// holding nothing.
func TestDiscoverSettings_ReportsUnparseableFile(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{"hooks": `)

	items, unread, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	if len(items) != 0 || len(unread) != 1 || !strings.Contains(unread[0].Error(), "settings.json") {
		t.Errorf("items %d unread %v, want none and one error naming the file", len(items), unread)
	}
}

// gjson reads truncated JSON as holding no servers, so an MCP settings
// file is checked for validity before its servers are read.
func TestDiscoverSettings_ReportsUnparseableMCPFile(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{"mcpServers": {"db": {"command": "db-server"}`)

	items, unread, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	if len(items) != 0 || len(unread) != 1 || !strings.Contains(unread[0].Error(), "settings.json") {
		t.Errorf("items %d unread %v, want none and one error naming the file", len(items), unread)
	}
}

// Regression: a settings file with no hooks key, such as one kept only for
// permissions, was reported as unreadable.
func TestDiscoverSettings_FileWithoutHooksIsNotUnread(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{"permissions": {"allow": ["Bash"]}}`)

	items, unread, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil || len(items) != 0 || len(unread) != 0 {
		t.Errorf("items %v unread %v err %v, want none of each", itemNames(items), unread, err)
	}
}

// Regression: a server placed at a -N directory was looked up under that
// directory's name, while the Library and its install records list it
// under its server key, so a pin on it went unseen.
func TestAddFromSettings_LibraryNameIsWhatTheLibraryLists(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), settingsHooksJSON)
	writeFile(t, filepath.Join(projectRoot, ".mcp.json"), `{"mcpServers": {"db": {"command": "db-server"}}}`)
	saveNamed(t, filepath.Join(globalDir, "mcp", "claude-code", "db"), "global", "db")

	servers, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	hooks, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.Hooks)
	if err != nil {
		t.Fatalf("DiscoverSettings: %v", err)
	}
	r := AddFromSettings(append(servers, hooks[1]), AddOptions{Provider: "claude-code"}, projectRoot, globalDir)

	cat, err := catalog.Scan(globalDir, t.TempDir())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Precedence hides the second db as overridden; it keeps its name.
	listed := map[string]string{}
	for _, it := range append(cat.Items, cat.Overridden...) {
		listed[it.Path] = it.Name
	}
	for _, res := range r {
		if res.Status != AddStatusAdded {
			t.Fatalf("%s: status %v err %v", res.Name, res.Status, res.Error)
		}
		// The scanner gives a hook its hook.json path and a server its directory.
		name, ok := listed[res.Dest]
		if !ok {
			name = listed[filepath.Join(res.Dest, "hook.json")]
		}
		if res.LibraryName != name {
			t.Errorf("%s at %s: LibraryName %q, the Library lists %q", res.Name, filepath.Base(res.Dest), res.LibraryName, name)
		}
	}
	if filepath.Base(r[0].Dest) != "db-2" || r[0].LibraryName != "db" {
		t.Errorf("server at %s named %q, want db-2 named db", filepath.Base(r[0].Dest), r[0].LibraryName)
	}
}

// Regression: settings adds dropped the laundering defense every other add
// applies, so private content re-added from a provider's settings came
// back with no registry.
func TestAddFromSettings_KeepsPrivateTaint(t *testing.T) {
	const servers = `{"mcpServers": {"db": {"command": "db-server"}}}`
	privateItem := func(t *testing.T, globalDir string) string {
		dir := filepath.Join(globalDir, "mcp", "acme-db")
		writeFile(t, filepath.Join(dir, "config.json"), servers)
		if err := metadata.Save(dir, &metadata.Meta{Name: "acme-db", SourceRegistry: "acme/private", SourceVisibility: "private", SourceHash: sourceHash([]byte(servers))}); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, projectRoot, globalDir string)
	}{
		{"same content", func(t *testing.T, projectRoot, globalDir string) {
			privateItem(t, globalDir)
			writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), servers)
		}},
		{"symlink into the Library", func(t *testing.T, projectRoot, globalDir string) {
			if err := os.Symlink(privateItem(t, globalDir), filepath.Join(projectRoot, ".claude")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(projectRoot, ".claude", "settings.json"), []byte(servers+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot, globalDir := settingsEnv(t)
			tc.setup(t, projectRoot, globalDir)
			items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
			if err != nil || len(items) != 1 {
				t.Fatalf("DiscoverSettings: items %v err %v", itemNames(items), err)
			}
			r := AddFromSettings(items, AddOptions{Provider: "claude-code"}, projectRoot, globalDir)[0]
			meta, err := metadata.Load(r.Dest)
			if err != nil || meta == nil {
				t.Fatalf("metadata.Load: %v", err)
			}
			if meta.SourceRegistry != "acme/private" || meta.SourceVisibility != "private" {
				t.Errorf("registry %q visibility %q, want acme/private private", meta.SourceRegistry, meta.SourceVisibility)
			}
		})
	}
}

// Regression: an item added from a settings file through a symlink into
// private Library content kept the taint but recorded no hash, and the
// Library index never looked inside mcp/<provider>/, so the same content
// added again from a plain file lost the taint once the original was gone.
func TestAddFromSettings_TaintSurvivesASecondAdd(t *testing.T) {
	const servers = `{"mcpServers": {"db": {"command": "db-server"}}}` + "\n"
	projectRoot, globalDir := settingsEnv(t)
	private := filepath.Join(globalDir, "mcp", "acme-db")
	writeFile(t, filepath.Join(private, "config.json"), servers)
	if err := metadata.Save(private, &metadata.Meta{Name: "acme-db", SourceRegistry: "acme/private", SourceVisibility: "private"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(private, filepath.Join(projectRoot, ".claude")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), servers)
	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil || len(items) != 1 {
		t.Fatalf("DiscoverSettings: items %v err %v", itemNames(items), err)
	}
	opts := AddOptions{Provider: "claude-code"}
	if r := AddFromSettings(items, opts, projectRoot, globalDir)[0]; r.Status != AddStatusAdded {
		t.Fatalf("first add: status %v err %v", r.Status, r.Error)
	}
	if err := os.RemoveAll(private); err != nil {
		t.Fatal(err)
	}

	plain := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, plain, servers)
	again := items[0]
	again.Path, again.Scope = plain, "global"
	r := AddFromSettings([]SettingsItem{again}, opts, projectRoot, globalDir)[0]
	if r.Status != AddStatusAdded {
		t.Fatalf("second add: status %v err %v", r.Status, r.Error)
	}
	meta, err := metadata.Load(r.Dest)
	if err != nil || meta == nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if meta.SourceRegistry != "acme/private" || meta.SourceVisibility != "private" {
		t.Errorf("registry %q visibility %q, want acme/private private", meta.SourceRegistry, meta.SourceVisibility)
	}
}

// Regression: a server discovered beside a same-scope, same-name server
// already in the Library was shown as new at db-2, but adding it alone
// placed it again, on the first server's directory, and skipped it.
func TestAddPlacedSettings_LandsWhereDiscoveryPlacedIt(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{"mcpServers": {"db": {"command": "one"}}}`)
	opts := AddOptions{Provider: "claude-code"}
	first, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil || len(first) != 1 {
		t.Fatalf("DiscoverSettings: items %v err %v", itemNames(first), err)
	}
	AddFromSettings(first, opts, projectRoot, globalDir)

	writeFile(t, filepath.Join(projectRoot, ".mcp.json"), `{"mcpServers": {"db": {"command": "two"}}}`)
	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil || len(items) != 2 || items[1].Status != StatusNew {
		t.Fatalf("DiscoverSettings: items %v err %v, want the .mcp.json server second and new", itemNames(items), err)
	}
	r := AddPlacedSettings(items[1:], opts, projectRoot, globalDir)[0]
	if want := filepath.Join(globalDir, "mcp", "claude-code", "db-2"); r.Status != AddStatusAdded || r.Dest != want {
		t.Fatalf("status %v dest %s, want added at %s", r.Status, r.Dest, want)
	}
	if data, _ := os.ReadFile(filepath.Join(r.Dest, "config.json")); !strings.Contains(string(data), `"two"`) {
		t.Errorf("config.json = %s, want the second server", data)
	}
}

// Regression: an add naming its registry recorded no hash, so the same
// settings added again from a plain file did not find the private item
// and lost its registry.
func TestAddFromSettings_RegistryAddRecordsItsHash(t *testing.T) {
	const servers = `{"mcpServers": {"db": {"command": "db-server"}}}` + "\n"
	projectRoot, globalDir := settingsEnv(t)
	writeFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), servers)
	items, _, err := DiscoverSettings(provider.ClaudeCode, projectRoot, "", globalDir, catalog.MCP)
	if err != nil || len(items) != 1 {
		t.Fatalf("DiscoverSettings: items %v err %v", itemNames(items), err)
	}
	private := AddOptions{Provider: "claude-code", SourceRegistry: "acme/private", SourceVisibility: "private"}
	if r := AddFromSettings(items, private, projectRoot, globalDir)[0]; r.Status != AddStatusAdded {
		t.Fatalf("first add: status %v err %v", r.Status, r.Error)
	}

	plain := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, plain, servers)
	again := items[0]
	again.Path, again.Scope = plain, "global"
	r := AddFromSettings([]SettingsItem{again}, AddOptions{Provider: "claude-code"}, projectRoot, globalDir)[0]
	if r.Status != AddStatusAdded {
		t.Fatalf("second add: status %v err %v", r.Status, r.Error)
	}
	meta, err := metadata.Load(r.Dest)
	if err != nil || meta == nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if meta.SourceRegistry != "acme/private" || meta.SourceVisibility != "private" {
		t.Errorf("registry %q visibility %q, want acme/private private", meta.SourceRegistry, meta.SourceVisibility)
	}
}

// Regression: a Zed server written beside another under db-2 kept Zed's
// context_servers key, which the catalog does not read, so the Library
// listed it as db-2 and installing it found no such server.
func TestAddFromSettings_SuffixedServerKeepsItsKey(t *testing.T) {
	projectRoot, globalDir := settingsEnv(t)
	saveNamed(t, filepath.Join(globalDir, "mcp", "zed", "db"), "global", "db")
	s := SettingsItem{
		DiscoveryItem: DiscoveryItem{Name: "db", Type: catalog.MCP, Path: filepath.Join(projectRoot, ".zed", "settings.json"), Scope: "project"},
		ServerKey:     "db",
		Server:        json.RawMessage(`{"command": "db-server"}`),
	}
	r := AddFromSettings([]SettingsItem{s}, AddOptions{Provider: "zed"}, projectRoot, globalDir)[0]
	if want := filepath.Join(globalDir, "mcp", "zed", "db-2"); r.Dest != want {
		t.Fatalf("dest %s, want %s", r.Dest, want)
	}
	cat, err := catalog.Scan(globalDir, t.TempDir())
	if err != nil {
		t.Fatalf("catalog.Scan: %v", err)
	}
	for _, it := range append(cat.Items, cat.Overridden...) {
		if it.Path == r.Dest {
			if it.ServerKey != "db" || it.Name != r.LibraryName {
				t.Errorf("Library lists server %q as %q, want db as %q", it.ServerKey, it.Name, r.LibraryName)
			}
			return
		}
	}
	t.Fatalf("the Library lists nothing at %s", r.Dest)
}
