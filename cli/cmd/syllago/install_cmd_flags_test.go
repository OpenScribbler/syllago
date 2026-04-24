package main

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// TestInstall_OnCleanFlagAccepted verifies --on-clean accepts replace|skip and
// rejects other values with an error message referencing the allowed values.
// This exercises flag parsing + value validation before any filesystem work.
func TestInstall_OnCleanFlagAccepted(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"replace", "replace", false},
		{"skip", "skip", false},
		{"bogus", "maybe", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			installCmd.Flags().Set("on-clean", tc.value)
			t.Cleanup(func() {
				installCmd.Flags().Set("to", "")
				installCmd.Flags().Set("method", "symlink")
				installCmd.Flags().Set("type", "")
				installCmd.Flags().Set("on-clean", "")
				installCmd.Flags().Set("on-modified", "")
			})

			err := installCmd.RunE(installCmd, []string{"foo"})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for --on-clean=%q, got nil", tc.value)
				}
				if !strings.Contains(err.Error(), "--on-clean") {
					t.Errorf("error should mention --on-clean, got: %v", err)
				}
				return
			}
			// Valid values must not produce a flag-validation error. The install
			// itself may still succeed (fresh state) or return other errors, but
			// "invalid --on-clean" should never appear.
			if err != nil && strings.Contains(err.Error(), "--on-clean") {
				t.Errorf("valid --on-clean=%q rejected: %v", tc.value, err)
			}
		})
	}
}

// TestInstall_OnModifiedFlagAccepted verifies --on-modified accepts
// drop-record|append-fresh|keep and rejects other values.
func TestInstall_OnModifiedFlagAccepted(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"drop-record", "drop-record", false},
		{"append-fresh", "append-fresh", false},
		{"keep", "keep", false},
		{"bogus", "nuke", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			installCmd.Flags().Set("on-modified", tc.value)
			t.Cleanup(func() {
				installCmd.Flags().Set("to", "")
				installCmd.Flags().Set("method", "symlink")
				installCmd.Flags().Set("type", "")
				installCmd.Flags().Set("on-clean", "")
				installCmd.Flags().Set("on-modified", "")
			})

			err := installCmd.RunE(installCmd, []string{"foo"})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for --on-modified=%q, got nil", tc.value)
				}
				if !strings.Contains(err.Error(), "--on-modified") {
					t.Errorf("error should mention --on-modified, got: %v", err)
				}
				return
			}
			if err != nil && strings.Contains(err.Error(), "--on-modified") {
				t.Errorf("valid --on-modified=%q rejected: %v", tc.value, err)
			}
		})
	}
}
