package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	t.Parallel()
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on missing dir: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty providers, got %v", cfg.Providers)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cfg := &Config{
		Providers: []string{"claude-code", "cursor"},
	}
	if err := Save(tmp, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Providers) != 2 || loaded.Providers[0] != "claude-code" {
		t.Errorf("loaded providers = %v, want [claude-code cursor]", loaded.Providers)
	}
}

func TestExists(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if Exists(tmp) {
		t.Error("Exists returned true before Save")
	}
	Save(tmp, &Config{})
	if !Exists(tmp) {
		t.Error("Exists returned false after Save")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cfg := &Config{Providers: []string{"gemini-cli"}}
	if err := Save(tmp, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Verify .syllago directory was created
	info, err := os.Stat(DirPath(tmp))
	if err != nil {
		t.Fatalf("DirPath stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("DirPath is not a directory")
	}
}

func TestPreferences(t *testing.T) {
	tmp := t.TempDir()
	cfg := &Config{
		Providers:   []string{"claude-code"},
		Preferences: map[string]string{"output-format": "json"},
	}
	if err := Save(tmp, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Preferences["output-format"] != "json" {
		t.Errorf("preferences = %v, want output-format=json", loaded.Preferences)
	}
}

func TestConfigContentRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := &Config{
		Providers:   []string{"claude-code"},
		ContentRoot: "content",
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ContentRoot != "content" {
		t.Errorf("ContentRoot = %q, want %q", loaded.ContentRoot, "content")
	}

	// Verify it's present in raw JSON when set
	data, _ := os.ReadFile(FilePath(dir))
	if !strings.Contains(string(data), "content_root") {
		t.Error("JSON should contain content_root key when set")
	}
}

func TestConfigContentRootOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := &Config{Providers: []string{"claude-code"}}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, _ := os.ReadFile(FilePath(dir))
	if strings.Contains(string(data), "content_root") {
		t.Error("JSON should not contain content_root when empty")
	}
}

func TestAllowedRegistriesRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := &Config{
		Providers:         []string{"claude-code"},
		AllowedRegistries: []string{"https://github.com/acme/tools.git", "https://github.com/acme/prompts.git"},
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.AllowedRegistries) != 2 {
		t.Fatalf("AllowedRegistries = %v, want 2 entries", loaded.AllowedRegistries)
	}
	if loaded.AllowedRegistries[0] != "https://github.com/acme/tools.git" {
		t.Errorf("AllowedRegistries[0] = %q, want %q", loaded.AllowedRegistries[0], "https://github.com/acme/tools.git")
	}

	// Verify key is present in raw JSON when set
	data, _ := os.ReadFile(FilePath(dir))
	if !strings.Contains(string(data), "allowed_registries") {
		t.Error("JSON should contain allowed_registries key when set")
	}
}

func TestAllowedRegistriesOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := &Config{Providers: []string{"claude-code"}}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, _ := os.ReadFile(FilePath(dir))
	if strings.Contains(string(data), "allowed_registries") {
		t.Error("JSON should not contain allowed_registries when empty")
	}
}

func TestGlobalDirPath_ContainsHomeDotSyllago(t *testing.T) {
	t.Parallel()
	got, err := GlobalDirPath()
	if err != nil {
		t.Fatalf("GlobalDirPath: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".syllago")
	if got != want {
		t.Errorf("GlobalDirPath() = %q, want %q", got, want)
	}
}

func TestGlobalFilePath_EndsWithConfigJSON(t *testing.T) {
	t.Parallel()
	got, err := GlobalFilePath()
	if err != nil {
		t.Fatalf("GlobalFilePath: %v", err)
	}
	if !strings.HasSuffix(got, "config.json") {
		t.Errorf("GlobalFilePath() = %q, want suffix 'config.json'", got)
	}
}

func TestLoadFromPath_ReturnsEmptyConfigWhenMissing(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cfg, err := LoadFromPath(filepath.Join(tmp, "config.json"))
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFromPath should return empty config, not nil")
	}
}

func TestLoadFromPath_RoundTrip(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.json")

	// Write JSON directly to the file
	os.WriteFile(path, []byte(`{"providers":["test-provider"]}`), 0644)

	loaded, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if len(loaded.Providers) != 1 || loaded.Providers[0] != "test-provider" {
		t.Errorf("loaded config providers = %v, want [test-provider]", loaded.Providers)
	}
}

func TestIsRegistryAllowed(t *testing.T) {
	empty := &Config{}
	if !empty.IsRegistryAllowed("https://any-url.git") {
		t.Error("empty AllowedRegistries should allow any URL")
	}

	restricted := &Config{
		AllowedRegistries: []string{"https://github.com/acme/tools.git"},
	}
	if !restricted.IsRegistryAllowed("https://github.com/acme/tools.git") {
		t.Error("allowed URL should pass")
	}
	if restricted.IsRegistryAllowed("https://github.com/random/other.git") {
		t.Error("non-allowed URL should be rejected")
	}
}

func TestMerge_ProjectProvidersOverrideGlobal(t *testing.T) {
	t.Parallel()
	global := &Config{Providers: []string{"claude-code", "cursor"}}
	project := &Config{Providers: []string{"gemini-cli"}}
	merged := Merge(global, project)
	if len(merged.Providers) != 1 || merged.Providers[0] != "gemini-cli" {
		t.Errorf("Merge: project providers should win, got %v", merged.Providers)
	}
}

func TestMerge_RegistriesMerged(t *testing.T) {
	t.Parallel()
	global := &Config{
		Registries: []Registry{{Name: "global-reg", URL: "https://github.com/g/g.git"}},
	}
	project := &Config{
		Registries: []Registry{{Name: "project-reg", URL: "https://github.com/p/p.git"}},
	}
	merged := Merge(global, project)
	if len(merged.Registries) != 2 {
		t.Errorf("Merge: registries should be merged, got %d", len(merged.Registries))
	}
}

func TestMerge_EmptyProjectUsesGlobal(t *testing.T) {
	t.Parallel()
	global := &Config{Providers: []string{"claude-code"}}
	project := &Config{}
	merged := Merge(global, project)
	if len(merged.Providers) != 1 || merged.Providers[0] != "claude-code" {
		t.Errorf("Merge: empty project should inherit global providers, got %v", merged.Providers)
	}
}

func TestMerge_NilInputs(t *testing.T) {
	t.Parallel()
	merged := Merge(nil, nil)
	if merged == nil {
		t.Fatal("Merge(nil, nil) should return non-nil Config")
	}
}

func TestProviderPathsRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := &Config{
		Providers: []string{"claude-code"},
		ProviderPaths: map[string]ProviderPathConfig{
			"claude-code": {
				BaseDir: "/custom/base",
				Paths:   map[string]string{"skills": "/custom/skills", "hooks": "/custom/hooks"},
			},
		},
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ppc, ok := loaded.ProviderPaths["claude-code"]
	if !ok {
		t.Fatal("ProviderPaths missing claude-code entry")
	}
	if ppc.BaseDir != "/custom/base" {
		t.Errorf("BaseDir = %q, want /custom/base", ppc.BaseDir)
	}
	if ppc.Paths["skills"] != "/custom/skills" {
		t.Errorf("Paths[skills] = %q, want /custom/skills", ppc.Paths["skills"])
	}
	if ppc.Paths["hooks"] != "/custom/hooks" {
		t.Errorf("Paths[hooks] = %q, want /custom/hooks", ppc.Paths["hooks"])
	}
}

func TestProviderPathsOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := &Config{Providers: []string{"claude-code"}}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, _ := os.ReadFile(FilePath(dir))
	if strings.Contains(string(data), "provider_paths") {
		t.Error("JSON should not contain provider_paths when empty")
	}
}

func TestMerge_ProviderPathsGlobalOnly(t *testing.T) {
	t.Parallel()
	global := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"claude-code": {BaseDir: "/global/base"},
		},
	}
	project := &Config{}
	merged := Merge(global, project)
	ppc, ok := merged.ProviderPaths["claude-code"]
	if !ok {
		t.Fatal("expected claude-code in merged ProviderPaths")
	}
	if ppc.BaseDir != "/global/base" {
		t.Errorf("BaseDir = %q, want /global/base", ppc.BaseDir)
	}
}

func TestMerge_ProviderPathsProjectOverride(t *testing.T) {
	t.Parallel()
	global := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"claude-code": {
				BaseDir: "/global/base",
				Paths:   map[string]string{"skills": "/global/skills"},
			},
		},
	}
	project := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"claude-code": {BaseDir: "/project/base"},
		},
	}
	merged := Merge(global, project)
	ppc := merged.ProviderPaths["claude-code"]
	if ppc.BaseDir != "/project/base" {
		t.Errorf("BaseDir = %q, want /project/base (project override)", ppc.BaseDir)
	}
	// Global per-type path should survive when project doesn't override it
	if ppc.Paths["skills"] != "/global/skills" {
		t.Errorf("Paths[skills] = %q, want /global/skills (preserved from global)", ppc.Paths["skills"])
	}
}

func TestLoadGlobal_MissingFile(t *testing.T) {
	// Uses a temp HOME so no real config is touched
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal on missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty providers, got %v", cfg.Providers)
	}
}

func TestSaveGlobal_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{
		Providers:   []string{"claude-code", "gemini-cli"},
		Preferences: map[string]string{"theme": "dark"},
	}
	if err := SaveGlobal(cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal after save: %v", err)
	}
	if len(loaded.Providers) != 2 || loaded.Providers[0] != "claude-code" {
		t.Errorf("Providers = %v, want [claude-code gemini-cli]", loaded.Providers)
	}
	if loaded.Preferences["theme"] != "dark" {
		t.Errorf("Preferences[theme] = %q, want dark", loaded.Preferences["theme"])
	}
}

func TestSaveGlobal_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{Providers: []string{"test"}}
	if err := SaveGlobal(cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	// Verify .syllago directory was created
	syllagoDir := filepath.Join(tmpDir, ".syllago")
	info, err := os.Stat(syllagoDir)
	if err != nil {
		t.Fatalf("Stat .syllago dir: %v", err)
	}
	if !info.IsDir() {
		t.Error(".syllago should be a directory")
	}

	// Verify config.json exists
	configPath := filepath.Join(syllagoDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.json not found: %v", err)
	}
}

func TestSaveGlobal_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Save initial config
	cfg1 := &Config{Providers: []string{"initial"}}
	if err := SaveGlobal(cfg1); err != nil {
		t.Fatalf("SaveGlobal (initial): %v", err)
	}

	// Overwrite with different config
	cfg2 := &Config{Providers: []string{"updated", "more"}}
	if err := SaveGlobal(cfg2); err != nil {
		t.Fatalf("SaveGlobal (update): %v", err)
	}

	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if len(loaded.Providers) != 2 || loaded.Providers[0] != "updated" {
		t.Errorf("Providers = %v, want [updated more]", loaded.Providers)
	}
}

func TestLoadFromPath_InvalidJSON(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	os.WriteFile(path, []byte("{invalid json}"), 0644)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// Create .syllago/config.json with invalid JSON
	dir := filepath.Join(tmp, ".syllago")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{bad}"), 0644)

	_, err := Load(tmp)
	if err == nil {
		t.Fatal("expected error for invalid JSON in config file")
	}
}

func TestExpandHome_BareHome(t *testing.T) {
	t.Parallel()
	result, err := ExpandHome("~")
	if err != nil {
		t.Fatalf("ExpandHome(~): %v", err)
	}
	home, _ := os.UserHomeDir()
	if result != home {
		t.Errorf("ExpandHome(~) = %q, want %q", result, home)
	}
}

func TestExpandHome_WithSubpath(t *testing.T) {
	t.Parallel()
	result, err := ExpandHome("~/Documents/stuff")
	if err != nil {
		t.Fatalf("ExpandHome: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "Documents", "stuff")
	if result != want {
		t.Errorf("ExpandHome(~/Documents/stuff) = %q, want %q", result, want)
	}
}

func TestExpandHome_AbsolutePath(t *testing.T) {
	t.Parallel()
	result, err := ExpandHome("/usr/local/bin")
	if err != nil {
		t.Fatalf("ExpandHome: %v", err)
	}
	if result != "/usr/local/bin" {
		t.Errorf("ExpandHome(/usr/local/bin) = %q, want /usr/local/bin", result)
	}
}

func TestExpandHome_RelativePath(t *testing.T) {
	t.Parallel()
	result, err := ExpandHome("relative/path")
	if err != nil {
		t.Fatalf("ExpandHome: %v", err)
	}
	if result != "relative/path" {
		t.Errorf("ExpandHome(relative/path) = %q, want relative/path", result)
	}
}

func TestMerge_PreferencesDeepMerge(t *testing.T) {
	t.Parallel()
	global := &Config{
		Preferences: map[string]string{"theme": "dark", "lang": "en"},
	}
	project := &Config{
		Preferences: map[string]string{"theme": "light"},
	}
	merged := Merge(global, project)
	if merged.Preferences["theme"] != "light" {
		t.Errorf("theme should be overridden by project, got %q", merged.Preferences["theme"])
	}
	if merged.Preferences["lang"] != "en" {
		t.Errorf("lang should be preserved from global, got %q", merged.Preferences["lang"])
	}
}

func TestMerge_SandboxProjectWins(t *testing.T) {
	t.Parallel()
	global := &Config{
		Sandbox: SandboxConfig{AllowedDomains: []string{"global.com"}},
	}
	project := &Config{
		Sandbox: SandboxConfig{AllowedDomains: []string{"project.com"}},
	}
	merged := Merge(global, project)
	if len(merged.Sandbox.AllowedDomains) != 1 || merged.Sandbox.AllowedDomains[0] != "project.com" {
		t.Errorf("project sandbox should win, got %v", merged.Sandbox.AllowedDomains)
	}
}

func TestMerge_SandboxGlobalFallback(t *testing.T) {
	t.Parallel()
	global := &Config{
		Sandbox: SandboxConfig{AllowedDomains: []string{"global.com"}},
	}
	project := &Config{}
	merged := Merge(global, project)
	if len(merged.Sandbox.AllowedDomains) != 1 || merged.Sandbox.AllowedDomains[0] != "global.com" {
		t.Errorf("global sandbox should be fallback, got %v", merged.Sandbox.AllowedDomains)
	}
}

func TestMerge_ContentRootProjectWins(t *testing.T) {
	t.Parallel()
	global := &Config{ContentRoot: "global-content"}
	project := &Config{ContentRoot: "project-content"}
	merged := Merge(global, project)
	if merged.ContentRoot != "project-content" {
		t.Errorf("ContentRoot = %q, want project-content", merged.ContentRoot)
	}
}

func TestMerge_AllowedRegistriesProjectWins(t *testing.T) {
	t.Parallel()
	global := &Config{AllowedRegistries: []string{"https://global.git"}}
	project := &Config{AllowedRegistries: []string{"https://project.git"}}
	merged := Merge(global, project)
	if len(merged.AllowedRegistries) != 1 || merged.AllowedRegistries[0] != "https://project.git" {
		t.Errorf("project AllowedRegistries should win, got %v", merged.AllowedRegistries)
	}
}

func TestMerge_RegistriesDeduplicatedByName(t *testing.T) {
	t.Parallel()
	global := &Config{
		Registries: []Registry{{Name: "shared", URL: "https://global.git"}},
	}
	project := &Config{
		Registries: []Registry{{Name: "shared", URL: "https://project.git"}},
	}
	merged := Merge(global, project)
	// Same name: global entry wins (first-seen dedup)
	if len(merged.Registries) != 1 {
		t.Errorf("expected 1 registry (deduplicated), got %d", len(merged.Registries))
	}
	if merged.Registries[0].URL != "https://global.git" {
		t.Errorf("global entry should win, got URL %q", merged.Registries[0].URL)
	}
}

func TestMerge_ProviderPathsDeepMerge(t *testing.T) {
	t.Parallel()
	global := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"claude-code": {
				Paths: map[string]string{"skills": "/global/skills", "hooks": "/global/hooks"},
			},
			"cursor": {
				BaseDir: "/cursor/base",
			},
		},
	}
	project := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"claude-code": {
				Paths: map[string]string{"skills": "/project/skills"}, // override skills, keep hooks
			},
		},
	}
	merged := Merge(global, project)

	// claude-code: skills overridden, hooks preserved
	cc := merged.ProviderPaths["claude-code"]
	if cc.Paths["skills"] != "/project/skills" {
		t.Errorf("claude-code skills = %q, want /project/skills", cc.Paths["skills"])
	}
	if cc.Paths["hooks"] != "/global/hooks" {
		t.Errorf("claude-code hooks = %q, want /global/hooks (preserved)", cc.Paths["hooks"])
	}

	// cursor: untouched by project
	cur, ok := merged.ProviderPaths["cursor"]
	if !ok {
		t.Fatal("expected cursor in merged ProviderPaths")
	}
	if cur.BaseDir != "/cursor/base" {
		t.Errorf("cursor BaseDir = %q, want /cursor/base", cur.BaseDir)
	}
}
