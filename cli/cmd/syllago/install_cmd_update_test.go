package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// appendLibraryRuleVersion is a thin test helper that appends a new canonical
// version to a library rule directory (D13 AppendVersion semantics).
func appendLibraryRuleVersion(dir string, newBody []byte) error {
	return rulestore.AppendVersion(dir, newBody)
}

// setupAppendReinstallEnv installs a rule once and returns cleanup-ready paths
// for a second install. Caller runs the second RunE and inspects the result.
func setupAppendReinstallEnv(t *testing.T, projectRoot, globalDir string) {
	t.Helper()
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
	installCmd.Flags().Set("no-input", "true")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("method", "symlink")
		installCmd.Flags().Set("type", "")
		installCmd.Flags().Set("no-input", "false")
		installCmd.Flags().Set("on-clean", "")
		installCmd.Flags().Set("on-modified", "")
	})

	if err := installCmd.RunE(installCmd, []string{"foo"}); err != nil {
		t.Fatalf("first install: %v", err)
	}
}

// TestInstall_CleanState_RequiresOnCleanFlagWhenNonInteractive: a second
// install against an already-installed-clean target must error with the exact
// D17 message rather than mutating anything.
func TestInstall_CleanState_RequiresOnCleanFlagWhenNonInteractive(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	setupAppendReinstallEnv(t, projectRoot, globalDir)

	// Second install — no --on-clean flag, non-interactive.
	err := installCmd.RunE(installCmd, []string{"foo"})
	if err == nil {
		t.Fatalf("expected error on second install without --on-clean, got nil")
	}
	want := "rule already installed at clean state; specify --on-clean=replace|skip"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error message mismatch\n got: %v\nwant substring: %s", err, want)
	}
}

// TestInstall_CleanState_OnCleanSkip_NoFileChange: second install with
// --on-clean=skip prints "skipping: already installed" and leaves the file
// bytes unchanged.
func TestInstall_CleanState_OnCleanSkip_NoFileChange(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	setupAppendReinstallEnv(t, projectRoot, globalDir)

	target := filepath.Join(projectRoot, "CLAUDE.md")
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	installCmd.Flags().Set("on-clean", "skip")
	if err := installCmd.RunE(installCmd, []string{"foo"}); err != nil {
		t.Fatalf("install --on-clean=skip: %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("target bytes changed under --on-clean=skip\n before: %q\n after:  %q", before, after)
	}
}

// TestInstall_CleanState_OnCleanReplace_UpdatesRecord: --on-clean=replace with
// a new rule version rewrites the block in place and updates installed.json's
// VersionHash.
func TestInstall_CleanState_OnCleanReplace_UpdatesRecord(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	setupAppendReinstallEnv(t, projectRoot, globalDir)

	// Overwrite the library rule body with a new version so replace has work.
	rulesRoot := filepath.Join(globalDir, string(catalog.Rules))
	ruleDir := filepath.Join(rulesRoot, "claude-code", "foo")
	// AppendVersion appends a new canonical version to the existing rule.
	newBody := []byte("# foo rule body\n\nAppended V2.\n")
	if err := appendLibraryRuleVersion(ruleDir, newBody); err != nil {
		t.Fatalf("appendLibraryRuleVersion: %v", err)
	}

	installCmd.Flags().Set("on-clean", "replace")
	if err := installCmd.RunE(installCmd, []string{"foo"}); err != nil {
		t.Fatalf("install --on-clean=replace: %v", err)
	}

	target := filepath.Join(projectRoot, "CLAUDE.md")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !strings.Contains(string(got), "Appended V2.") {
		t.Errorf("target should contain new body after replace; got: %q", got)
	}
	if strings.Contains(string(got), "Append me.") {
		t.Errorf("target should not still contain old body; got: %q", got)
	}

	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.RuleAppends) != 1 {
		t.Fatalf("expected 1 RuleAppend after replace, got %d", len(inst.RuleAppends))
	}
}

// TestInstall_ModifiedState_NonInteractiveErrors: table-driven — each of the
// three Modified reasons errors with the same D17-exact message.
func TestInstall_ModifiedState_NonInteractiveErrors(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, target string)
	}{
		{
			"edited",
			func(t *testing.T, target string) {
				// Overwrite file with unrelated content so the recorded hash
				// bytes are no longer present.
				if err := os.WriteFile(target, []byte("UNRELATED CONTENT\n"), 0644); err != nil {
					t.Fatalf("overwrite target: %v", err)
				}
			},
		},
		{
			"missing",
			func(t *testing.T, target string) {
				if err := os.Remove(target); err != nil {
					t.Fatalf("remove target: %v", err)
				}
			},
		},
		{
			"unreadable",
			func(t *testing.T, target string) {
				// Make the file unreadable. On WSL/Linux chmod 0 on a file
				// owned by the test user still blocks os.ReadFile for that
				// user. Cleanup restores perms so t.TempDir can remove it.
				if err := os.Chmod(target, 0); err != nil {
					t.Fatalf("chmod 0: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(target, 0644) })
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			globalDir := t.TempDir()
			setupAppendReinstallEnv(t, projectRoot, globalDir)

			target := filepath.Join(projectRoot, "CLAUDE.md")
			tc.prepare(t, target)

			// Second install — no --on-modified flag, non-interactive.
			err := installCmd.RunE(installCmd, []string{"foo"})
			if err == nil {
				t.Fatalf("expected error in %s state without --on-modified, got nil", tc.name)
			}
			want := "rule install record is stale; specify --on-modified=drop-record|append-fresh|keep"
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error message mismatch\n got: %v\nwant substring: %s", err, want)
			}
		})
	}
}
