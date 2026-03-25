package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestDoctorCheckLibrary(t *testing.T) {
	// With a valid library dir
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	os.MkdirAll(filepath.Join(dir, "skills", "my-skill"), 0755)

	c := checkLibrary()
	if c.Status != checkOK {
		t.Errorf("expected ok, got %s: %s", c.Status, c.Message)
	}
	if !strings.Contains(c.Message, "1 items") {
		t.Errorf("expected item count in message, got: %s", c.Message)
	}
}

func TestDoctorCheckLibraryMissing(t *testing.T) {
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(t.TempDir(), "nonexistent")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	c := checkLibrary()
	if c.Status != checkErr {
		t.Errorf("expected err for missing library, got %s", c.Status)
	}
}

func TestDoctorCheckProviders(t *testing.T) {
	c := checkProviders()
	// On any dev machine, at least some providers should be detected
	if c.Status == checkErr {
		t.Errorf("unexpected error status: %s", c.Message)
	}
	if !strings.Contains(c.Message, "detected") {
		t.Errorf("expected 'detected' in message, got: %s", c.Message)
	}
}

func TestDoctorCheckSymlinksEmpty(t *testing.T) {
	dir := t.TempDir()
	c := checkSymlinks(dir)
	if c.Status != checkOK {
		t.Errorf("expected ok for empty installed.json, got %s", c.Status)
	}
}

func TestDoctorCheckContentDriftClean(t *testing.T) {
	dir := t.TempDir()
	c := checkContentDrift(dir)
	if c.Status != checkOK {
		t.Errorf("expected ok for no installed content, got %s: %s", c.Status, c.Message)
	}
}

func TestDoctorCheckRegistriesNone(t *testing.T) {
	c := checkRegistriesWith("")
	if c.Status != checkOK {
		t.Errorf("expected ok with no registries, got %s", c.Status)
	}
}

func TestDoctorJSONOutput(t *testing.T) {
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return dir, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	// We can't test the full command because os.Exit would kill the test,
	// but we can test the check functions produce valid JSON-serializable results.
	checks := []checkResult{
		checkLibrary(),
		checkProviders(),
		checkSymlinks(dir),
		checkContentDrift(dir),
		checkRegistriesWith(""),
	}

	result := doctorResult{Checks: checks, Summary: "test"}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal doctor result: %v", err)
	}

	// Verify it round-trips
	var parsed doctorResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal doctor result: %v", err)
	}
	if len(parsed.Checks) != len(checks) {
		t.Errorf("expected %d checks, got %d", len(checks), len(parsed.Checks))
	}

	_ = stdout // output captured but not used for this test
}

func TestDoctorCmdRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected doctor command registered on rootCmd")
	}
}
