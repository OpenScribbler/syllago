package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// TestUninstall_MonolithicRule_Roundtrip verifies D7 semantics via the CLI:
// install with --method=append, uninstall, assert target bytes equal the
// pre-install snapshot. The uninstall command path must detect the
// RuleAppend record in installed.json and route through UninstallRuleAppend
// rather than the generic symlink-removal path.
func TestUninstall_MonolithicRule_Roundtrip(t *testing.T) {
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

	// Seed a preamble so we can compare pre/post-install bytes meaningfully.
	target := filepath.Join(projectRoot, "CLAUDE.md")
	preamble := []byte("user preamble\n")
	if err := os.WriteFile(target, preamble, 0644); err != nil {
		t.Fatalf("seed preamble: %v", err)
	}
	preSnapshot := append([]byte{}, preamble...)

	// Install with --method=append.
	installCmd.Flags().Set("to", "claude-code")
	installCmd.Flags().Set("method", "append")
	installCmd.Flags().Set("type", "rules")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("method", "symlink")
		installCmd.Flags().Set("type", "")
	})
	if err := installCmd.RunE(installCmd, []string{"foo"}); err != nil {
		t.Fatalf("install append: %v", err)
	}
	// Sanity: target now differs from snapshot.
	afterInstall, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after install: %v", err)
	}
	if string(afterInstall) == string(preSnapshot) {
		t.Fatal("target unchanged after install — fixture wrong")
	}
	// Sanity: install record exists.
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.RuleAppends) != 1 {
		t.Fatalf("expected 1 RuleAppend after install, got %d", len(inst.RuleAppends))
	}

	// Uninstall via CLI.
	uninstallCmd.Flags().Set("from", "claude-code")
	uninstallCmd.Flags().Set("force", "true")
	uninstallCmd.Flags().Set("type", "rules")
	t.Cleanup(func() {
		uninstallCmd.Flags().Set("from", "")
		uninstallCmd.Flags().Set("force", "false")
		uninstallCmd.Flags().Set("type", "")
	})
	if err := uninstallCmd.RunE(uninstallCmd, []string{"foo"}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	// Target file bytes must equal the pre-install snapshot (byte-for-byte).
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after uninstall: %v", err)
	}
	if string(got) != string(preSnapshot) {
		t.Errorf("post-uninstall bytes mismatch\n got %q\nwant %q", got, preSnapshot)
	}

	// Record must be gone.
	inst, err = installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled post-uninstall: %v", err)
	}
	if len(inst.RuleAppends) != 0 {
		t.Errorf("expected 0 RuleAppends after uninstall, got %d", len(inst.RuleAppends))
	}
}
