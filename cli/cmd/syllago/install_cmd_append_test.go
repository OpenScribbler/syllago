package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// seedLibraryRule writes a single rule to <globalDir>/rules/<provider>/<slug>
// via rulestore.WriteRule (D11 layout: rule.md + .syllago.yaml + .history/).
func seedLibraryRule(t *testing.T, globalDir, providerSlug, slug, body string) {
	t.Helper()
	rulesRoot := filepath.Join(globalDir, string(catalog.Rules))
	meta := metadata.RuleMetadata{
		ID:   "lib-" + slug,
		Name: slug,
	}
	if err := rulestore.WriteRule(rulesRoot, providerSlug, slug, meta, []byte(body)); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
}

func TestInstall_MethodAppend_WritesMonolithicFile(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	seedLibraryRule(t, globalDir, "claude-code", "foo", "# foo rule body\n\nAppend me.\n")

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to", "claude-code")
	installCmd.Flags().Set("method", "append")
	installCmd.Flags().Set("type", "rules")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("method", "symlink")
		installCmd.Flags().Set("type", "")
	})

	if err := installCmd.RunE(installCmd, []string{"foo"}); err != nil {
		t.Fatalf("install --method=append failed: %v", err)
	}

	// CLAUDE.md at project root should contain the rule body.
	target := filepath.Join(projectRoot, "CLAUDE.md")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(got), "foo rule body") {
		t.Errorf("CLAUDE.md missing appended body:\n%s", got)
	}

	// installed.json must have one ruleAppends entry.
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.RuleAppends) != 1 {
		t.Fatalf("expected 1 RuleAppend, got %d", len(inst.RuleAppends))
	}
	if inst.RuleAppends[0].Provider != "claude-code" {
		t.Errorf("provider: got %q, want claude-code", inst.RuleAppends[0].Provider)
	}
	if inst.RuleAppends[0].TargetFile != target {
		t.Errorf("targetFile: got %q, want %q", inst.RuleAppends[0].TargetFile, target)
	}
}

// TestInstall_MethodAppend_QuietSuppressesNote verifies D10: providers with a
// MonolithicHint (e.g., windsurf) print a "NOTE:" line to stderr after append,
// but --quiet suppresses it. Uses windsurf because it has a non-empty hint.
func TestInstall_MethodAppend_QuietSuppressesNote(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	seedLibraryRule(t, globalDir, "windsurf", "foo", "# foo rule body\n\nAppend me.\n")

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	_, stderr := output.SetForTest(t)
	output.Quiet = true

	installCmd.Flags().Set("to", "windsurf")
	installCmd.Flags().Set("method", "append")
	installCmd.Flags().Set("type", "rules")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("method", "symlink")
		installCmd.Flags().Set("type", "")
	})

	if err := installCmd.RunE(installCmd, []string{"foo"}); err != nil {
		t.Fatalf("install --method=append --quiet failed: %v", err)
	}

	if strings.Contains(stderr.String(), "NOTE:") {
		t.Errorf("--quiet should suppress NOTE in stderr, got: %s", stderr.String())
	}
}
