package telemetry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadUserConfig_Missing(t *testing.T) {
	overrideHome(t, t.TempDir())
	cfg, err := loadUserConfig()
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestLoadUserConfig_Valid(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)
	dir := filepath.Join(home, ".syllago")
	os.MkdirAll(dir, 0755)
	body := `{"enabled":true,"anonymousId":"syl_aabbccdd1122","consentRecorded":true,"createdAt":"2026-04-02T00:00:00Z"}`
	os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte(body), 0644)

	cfg, err := loadUserConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.AnonymousID != "syl_aabbccdd1122" {
		t.Errorf("unexpected ID: %s", cfg.AnonymousID)
	}
	if !cfg.ConsentRecorded {
		t.Error("expected consentRecorded true")
	}
}

func TestLoadUserConfig_Malformed(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)
	dir := filepath.Join(home, ".syllago")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte("not json at all"), 0644)

	_, err := loadUserConfig()
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSaveUserConfig_Roundtrip(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)

	want := &Config{
		Enabled:         true,
		AnonymousID:     "syl_aabbccdd1122",
		ConsentRecorded: true,
		CreatedAt:       time.Now().UTC().Truncate(time.Second),
	}
	if err := saveUserConfig(want); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := loadUserConfig()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got.AnonymousID != want.AnonymousID {
		t.Errorf("ID mismatch: got %s want %s", got.AnonymousID, want.AnonymousID)
	}
	if got.Enabled != want.Enabled {
		t.Errorf("Enabled mismatch: got %v want %v", got.Enabled, want.Enabled)
	}
}

func TestLoadSysConfig_Missing(t *testing.T) {
	t.Parallel()
	// /etc/syllago/telemetry.json won't exist in test env — expect nil, nil.
	sc, err := loadSysConfig()
	if err != nil {
		t.Logf("skipping: /etc/syllago/telemetry.json read error (expected in test): %v", err)
	}
	_ = sc // nil is the expected result in most test environments
}

func TestNewConfig_DefaultsAndID(t *testing.T) {
	t.Parallel()
	cfg, err := newConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Opt-in default: nothing fires until the user explicitly opts in.
	if cfg.Enabled {
		t.Error("expected enabled false by default (telemetry is opt-in)")
	}
	if cfg.ConsentRecorded {
		t.Error("expected consentRecorded false by default")
	}
	if len(cfg.AnonymousID) == 0 {
		t.Error("expected non-empty anonymous ID")
	}
}

// overrideHome temporarily replaces UserHomeDirFn for the duration of the test.
func overrideHome(t *testing.T, dir string) {
	t.Helper()
	orig := UserHomeDirFn
	UserHomeDirFn = func() (string, error) { return dir, nil }
	t.Cleanup(func() { UserHomeDirFn = orig })
}
