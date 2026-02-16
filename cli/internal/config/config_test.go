package config

import (
	"os"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on missing dir: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty providers, got %v", cfg.Providers)
	}
}

func TestSaveAndLoad(t *testing.T) {
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
	tmp := t.TempDir()
	cfg := &Config{Providers: []string{"gemini-cli"}}
	if err := Save(tmp, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Verify .nesco directory was created
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
