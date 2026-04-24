package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// monolithicH2Source is a CLAUDE.md-style file with five ## sections and
// enough lines (>= 30) to bypass D4's too-small / too-few-h2 skip-split gates.
const monolithicH2Source = `# My Rules

Top-level preamble before the first H2.

## Security Rules

- Rule 1
- Rule 2
- Rule 3
- Rule 4

## Testing Rules

- Test A
- Test B
- Test C
- Test D

## Logging Rules

- Log format is JSON
- Include a timestamp
- Include a level
- Include a request ID

## Style Rules

- gofmt all Go code
- Tabs not spaces
- One sentence per line

## Process Rules

- Commit small changes
- Write tests first
- Document edge cases
`

// tooSmallSource is a monolithic file well under D4's 30-line gate.
const tooSmallSource = `# Tiny

A short note.
`

func resetAddFromFlag(t *testing.T) {
	t.Helper()
	// StringArray flags accumulate on Set — walk explicit reset so subsequent
	// tests start clean. The idiom for pflag StringArray: Set("") appends an
	// empty entry; the best-available reset is to re-initialize via a helper.
	f := addCmd.Flags().Lookup("from")
	if f == nil {
		return
	}
	// pflag's StringArrayValue implements the sliceValue interface via
	// Replace([]string{}) — this is the safe way to clear accumulated values.
	type sliceValue interface{ Replace([]string) error }
	if sv, ok := f.Value.(sliceValue); ok {
		_ = sv.Replace([]string{})
	}
	f.Changed = false
}

func setupMonolithicFixture(t *testing.T, name, body string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, name)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("seed fixture %q: %v", name, err)
	}
	return path
}

func TestAdd_FromMonolithicFile_H2(t *testing.T) {
	fixturePath := setupMonolithicFixture(t, "CLAUDE.md", monolithicH2Source)

	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	resetAddFromFlag(t)
	t.Cleanup(func() { resetAddFromFlag(t) })
	addCmd.Flags().Set("from", fixturePath)
	addCmd.Flags().Set("split", "h2")
	t.Cleanup(func() { addCmd.Flags().Set("split", "") })

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --from <path> --split=h2 failed: %v", err)
	}

	// After import, library should have rules under globalDir/rules/claude-code/.
	libraryRulesDir := filepath.Join(globalDir, "rules", "claude-code")
	entries, err := os.ReadDir(libraryRulesDir)
	if err != nil {
		t.Fatalf("reading library rules dir: %v", err)
	}
	if len(entries) < 3 {
		t.Errorf("expected at least 3 rules written, got %d", len(entries))
	}
}

func TestAdd_FromMonolithicFile_SkipSplitErrorsWithoutSingleFlag(t *testing.T) {
	fixturePath := setupMonolithicFixture(t, "CLAUDE.md", tooSmallSource)

	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	resetAddFromFlag(t)
	t.Cleanup(func() { resetAddFromFlag(t) })
	addCmd.Flags().Set("from", fixturePath)
	addCmd.Flags().Set("split", "h2")
	t.Cleanup(func() { addCmd.Flags().Set("split", "") })

	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Fatal("expected error on skip-split without --split=single")
	}
	if !strings.Contains(err.Error(), "--split=single") {
		t.Errorf("error must mention --split=single, got: %v", err)
	}
}

func TestAdd_FromMonolithicFile_Single(t *testing.T) {
	fixturePath := setupMonolithicFixture(t, "CLAUDE.md", tooSmallSource)

	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	resetAddFromFlag(t)
	t.Cleanup(func() { resetAddFromFlag(t) })
	addCmd.Flags().Set("from", fixturePath)
	addCmd.Flags().Set("split", "single")
	t.Cleanup(func() { addCmd.Flags().Set("split", "") })

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --from <path> --split=single failed: %v", err)
	}

	libraryRulesDir := filepath.Join(globalDir, "rules", "claude-code")
	entries, err := os.ReadDir(libraryRulesDir)
	if err != nil {
		t.Fatalf("reading library rules dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 rule written for single mode, got %d", len(entries))
	}
}

func TestAdd_LLMSplitWithoutSkill_ErrorsWithInstallPointer(t *testing.T) {
	fixturePath := setupMonolithicFixture(t, "CLAUDE.md", monolithicH2Source)

	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	resetAddFromFlag(t)
	t.Cleanup(func() { resetAddFromFlag(t) })
	addCmd.Flags().Set("from", fixturePath)
	addCmd.Flags().Set("split", "llm")
	t.Cleanup(func() { addCmd.Flags().Set("split", "") })

	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --split=llm is used without split-rules-llm skill")
	}
	if !strings.Contains(err.Error(), "syllago add split-rules-llm") {
		t.Errorf("error must point to 'syllago add split-rules-llm', got: %v", err)
	}
}
