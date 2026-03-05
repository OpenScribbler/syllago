package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// setupConvertLibrary creates a temp dir acting as a global library with a
// single skill item. Returns the library root path.
func setupConvertLibrary(t *testing.T) string {
	t.Helper()
	lib := t.TempDir()

	skillDir := filepath.Join(lib, "skills", "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill\nDoes something useful.\n"), 0644)

	return lib
}

// withConvertLibrary redirects GlobalContentDir to dir for the duration of the test.
func withConvertLibrary(t *testing.T, dir string) {
	t.Helper()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
}

// resetConvertFlags resets convert command flags to their zero values.
func resetConvertFlags(t *testing.T) {
	t.Helper()
	convertCmd.Flags().Set("to", "")
	convertCmd.Flags().Set("output", "")
}

func TestConvertRequiresTo(t *testing.T) {
	// --to is marked required by cobra, but RunE can still be called directly
	// with an empty flag; it should return an error for an unknown provider.
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	_, _ = output.SetForTest(t)

	convertCmd.Flags().Set("to", "")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{"my-skill"})
	if err == nil {
		t.Error("convert with empty --to should fail")
	}
}

func TestConvertUnknownProvider(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	_, stderr := output.SetForTest(t)

	convertCmd.Flags().Set("to", "totally-unknown-provider-xyz")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{"my-skill"})
	if err == nil {
		t.Error("convert with unknown provider should fail")
	}
	// stderr is captured but the error itself carries the message.
	_ = stderr
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider' in error, got: %v", err)
	}
}

func TestConvertItemNotFound(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	_, _ = output.SetForTest(t)

	convertCmd.Flags().Set("to", "cursor")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{"nonexistent-item"})
	if err == nil {
		t.Error("convert with nonexistent item should fail")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

func TestConvertSkillToStdout(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	_, _ = output.SetForTest(t)

	convertCmd.Flags().Set("to", "cursor")
	defer resetConvertFlags(t)

	// Stdout is not redirected by output.SetForTest — the command writes to
	// os.Stdout directly. We just verify it doesn't return an error.
	err := convertCmd.RunE(convertCmd, []string{"my-skill"})
	if err != nil {
		t.Errorf("convert to cursor should succeed, got: %v", err)
	}
}

func TestConvertSkillToOutputFile(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	stdout, _ := output.SetForTest(t)

	outFile := filepath.Join(t.TempDir(), "result.md")
	convertCmd.Flags().Set("to", "cursor")
	convertCmd.Flags().Set("output", outFile)
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{"my-skill"})
	if err != nil {
		t.Fatalf("convert with --output should succeed, got: %v", err)
	}

	// The output file should exist with content.
	data, readErr := os.ReadFile(outFile)
	if readErr != nil {
		t.Fatalf("expected output file at %s, got error: %v", outFile, readErr)
	}
	if len(data) == 0 {
		t.Error("output file should not be empty")
	}

	// A confirmation message should be printed to stdout.
	if !strings.Contains(stdout.String(), outFile) {
		t.Errorf("expected output path in stdout, got: %s", stdout.String())
	}
}

func TestConvertUnsupportedType(t *testing.T) {
	// Loadouts have no registered converter — convert should report the type
	// is unsupported. Loadouts are provider-specific so the path is:
	// loadouts/<provider>/<name>/loadout.yaml
	lib := t.TempDir()
	loadoutDir := filepath.Join(lib, "loadouts", "claude-code", "my-loadout")
	os.MkdirAll(loadoutDir, 0755)
	os.WriteFile(filepath.Join(loadoutDir, "loadout.yaml"), []byte("name: my-loadout\n"), 0644)
	withConvertLibrary(t, lib)

	_, _ = output.SetForTest(t)

	convertCmd.Flags().Set("to", "cursor")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{"my-loadout"})
	if err == nil {
		t.Error("convert with unsupported type should fail")
	}
	if !strings.Contains(err.Error(), "does not support format conversion") {
		t.Errorf("expected 'does not support format conversion' in error, got: %v", err)
	}
}
