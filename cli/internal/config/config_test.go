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
