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
	convertCmd.Flags().Set("from", "")
	convertCmd.Flags().Set("type", "rules")
	convertCmd.Flags().Set("output", "")
}

// --- Library item mode (existing behavior) ---

func TestConvertRequiresTo(t *testing.T) {
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
	_, _ = output.SetForTest(t)

	convertCmd.Flags().Set("to", "totally-unknown-provider-xyz")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{"my-skill"})
	if err == nil {
		t.Error("convert with unknown provider should fail")
	}
	if !strings.Contains(err.Error(), "unknown target provider") {
		t.Errorf("expected 'unknown target provider' in error, got: %v", err)
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

	data, readErr := os.ReadFile(outFile)
	if readErr != nil {
		t.Fatalf("expected output file at %s, got error: %v", outFile, readErr)
	}
	if len(data) == 0 {
		t.Error("output file should not be empty")
	}
	if !strings.Contains(stdout.String(), outFile) {
		t.Errorf("expected output path in stdout, got: %s", stdout.String())
	}
}

func TestConvertUnsupportedType(t *testing.T) {
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

// --- File mode (new behavior) ---

func TestConvertFileMode(t *testing.T) {
	_, _ = output.SetForTest(t)

	dir := t.TempDir()
	ruleFile := filepath.Join(dir, "my-rule.mdc")
	os.WriteFile(ruleFile, []byte("---\ndescription: Test rule\nalwaysApply: true\n---\n\nAlways follow this rule.\n"), 0644)

	convertCmd.Flags().Set("to", "windsurf")
	convertCmd.Flags().Set("from", "cursor")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{ruleFile})
	if err != nil {
		t.Fatalf("file-mode convert should succeed, got: %v", err)
	}
}

func TestConvertFileModeToOutputFile(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	dir := t.TempDir()
	ruleFile := filepath.Join(dir, "cursor-rule.mdc")
	os.WriteFile(ruleFile, []byte("---\ndescription: TS conventions\nalwaysApply: false\nglobs: \"*.ts, *.tsx\"\n---\n\nUse strict TypeScript.\n"), 0644)

	outFile := filepath.Join(dir, "windsurf-rule.md")
	convertCmd.Flags().Set("to", "windsurf")
	convertCmd.Flags().Set("from", "cursor")
	convertCmd.Flags().Set("output", outFile)
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{ruleFile})
	if err != nil {
		t.Fatalf("file-mode convert with --output should succeed, got: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "trigger: glob") {
		t.Errorf("expected Windsurf glob trigger, got:\n%s", out)
	}
	if !strings.Contains(out, "*.ts") {
		t.Errorf("expected glob pattern in output, got:\n%s", out)
	}
	if !strings.Contains(stdout.String(), "Converted") {
		t.Errorf("expected 'Converted' confirmation, got: %s", stdout.String())
	}
}

func TestConvertFileModeRequiresFrom(t *testing.T) {
	_, _ = output.SetForTest(t)

	dir := t.TempDir()
	ruleFile := filepath.Join(dir, "rule.md")
	os.WriteFile(ruleFile, []byte("# A rule\n"), 0644)

	convertCmd.Flags().Set("to", "cursor")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{ruleFile})
	if err == nil {
		t.Fatal("file-mode convert without --from should fail")
	}
	if !strings.Contains(err.Error(), "--from is required") {
		t.Errorf("expected '--from is required' in error, got: %v", err)
	}
}

func TestConvertFileModeUnknownFromProvider(t *testing.T) {
	_, _ = output.SetForTest(t)

	dir := t.TempDir()
	ruleFile := filepath.Join(dir, "rule.md")
	os.WriteFile(ruleFile, []byte("# A rule\n"), 0644)

	convertCmd.Flags().Set("to", "cursor")
	convertCmd.Flags().Set("from", "nonexistent-provider")
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{ruleFile})
	if err == nil {
		t.Fatal("file-mode convert with unknown --from should fail")
	}
	if !strings.Contains(err.Error(), "unknown source provider") {
		t.Errorf("expected 'unknown source provider' in error, got: %v", err)
	}
}

// --- Cross-provider file conversion round-trips ---

func TestConvertFileCursorToClaudeCode(t *testing.T) {
	_, _ = output.SetForTest(t)

	dir := t.TempDir()
	ruleFile := filepath.Join(dir, "rule.mdc")
	os.WriteFile(ruleFile, []byte("---\ndescription: Go conventions\nalwaysApply: true\n---\n\nUse gofmt.\n"), 0644)

	outFile := filepath.Join(dir, "claude-rule.md")
	convertCmd.Flags().Set("to", "claude-code")
	convertCmd.Flags().Set("from", "cursor")
	convertCmd.Flags().Set("output", outFile)
	defer resetConvertFlags(t)

	err := convertCmd.RunE(convertCmd, []string{ruleFile})
	if err != nil {
		t.Fatalf("cursor->claude convert failed: %v", err)
	}

	data, _ := os.ReadFile(outFile)
	out := string(data)
	if strings.Contains(out, "---") {
		t.Errorf("Claude Code always-apply rule should not have frontmatter, got:\n%s", out)
	}
	if !strings.Contains(out, "Use gofmt.") {
		t.Errorf("expected body content, got:\n%s", out)
	}
}

func TestConvertFlagsRegistered(t *testing.T) {
	for _, name := range []string{"to", "from", "type", "output"} {
		if convertCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected --%s flag on convertCmd", name)
		}
	}
}
