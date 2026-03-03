package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestSettingsScope_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		scope SettingsScope
		want  string
	}{
		{ScopeGlobal, "global"},
		{ScopeProject, "project"},
	}
	for _, tc := range tests {
		got := tc.scope.String()
		if got != tc.want {
			t.Errorf("SettingsScope(%d).String() = %q, want %q", tc.scope, got, tc.want)
		}
	}
}

func TestFindSettingsLocations_GlobalOnly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	prov := provider.Provider{
		Name:      "Test Provider",
		Slug:      "test",
		ConfigDir: ".testprovider",
	}

	// Create global settings.json only
	globalDir := filepath.Join(tmpDir, prov.ConfigDir)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	projectRoot := t.TempDir()
	locations, err := FindSettingsLocations(prov, projectRoot)
	if err != nil {
		t.Fatalf("FindSettingsLocations: %v", err)
	}

	if len(locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locations))
	}
	if locations[0].Scope != ScopeGlobal {
		t.Errorf("expected ScopeGlobal, got %v", locations[0].Scope)
	}
	if locations[0].Path != filepath.Join(globalDir, "settings.json") {
		t.Errorf("unexpected path: %s", locations[0].Path)
	}
}

func TestFindSettingsLocations_BothScopes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	prov := provider.Provider{
		Name:      "Test Provider",
		Slug:      "test",
		ConfigDir: ".testprovider",
	}

	// Create global settings.json
	globalDir := filepath.Join(tmpDir, prov.ConfigDir)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("MkdirAll global: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile global: %v", err)
	}

	// Create project settings.json
	projectRoot := t.TempDir()
	projectDir := filepath.Join(projectRoot, prov.ConfigDir)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("MkdirAll project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile project: %v", err)
	}

	locations, err := FindSettingsLocations(prov, projectRoot)
	if err != nil {
		t.Fatalf("FindSettingsLocations: %v", err)
	}

	if len(locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(locations))
	}
	if locations[0].Scope != ScopeGlobal {
		t.Errorf("first location: expected ScopeGlobal, got %v", locations[0].Scope)
	}
	if locations[1].Scope != ScopeProject {
		t.Errorf("second location: expected ScopeProject, got %v", locations[1].Scope)
	}
}

func TestFindSettingsLocations_NoneExist(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	prov := provider.Provider{
		Name:      "Test Provider",
		Slug:      "test",
		ConfigDir: ".testprovider",
	}

	projectRoot := t.TempDir()
	locations, err := FindSettingsLocations(prov, projectRoot)
	if err != nil {
		t.Fatalf("FindSettingsLocations: %v", err)
	}

	if len(locations) != 0 {
		t.Errorf("expected 0 locations, got %d: %v", len(locations), locations)
	}
}

func TestHookSettingsPathForScope_Global(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	prov := provider.Provider{
		Name:      "Test Provider",
		Slug:      "test",
		ConfigDir: ".testprovider",
	}

	got, err := hookSettingsPathForScope(prov, ScopeGlobal, "")
	if err != nil {
		t.Fatalf("hookSettingsPathForScope: %v", err)
	}

	want := filepath.Join(tmpDir, prov.ConfigDir, "settings.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHookSettingsPathForScope_Project(t *testing.T) {
	t.Parallel()

	prov := provider.Provider{
		Name:      "Test Provider",
		Slug:      "test",
		ConfigDir: ".testprovider",
	}

	projectRoot := t.TempDir()
	got, err := hookSettingsPathForScope(prov, ScopeProject, projectRoot)
	if err != nil {
		t.Fatalf("hookSettingsPathForScope: %v", err)
	}

	want := filepath.Join(projectRoot, prov.ConfigDir, "settings.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHookSettingsPathForScope_ProjectNoRoot(t *testing.T) {
	t.Parallel()

	prov := provider.Provider{
		Name:      "Test Provider",
		Slug:      "test",
		ConfigDir: ".testprovider",
	}

	_, err := hookSettingsPathForScope(prov, ScopeProject, "")
	if err == nil {
		t.Fatal("expected error when project scope with empty projectRoot, got nil")
	}
}
