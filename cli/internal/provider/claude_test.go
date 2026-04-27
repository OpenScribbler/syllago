package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClaudeCodeDetect covers the four branches we care about:
//
//  1. Empty home, no binary on PATH → false. The default state of a fresh
//     machine with neither Claude Code installed nor any syllago content.
//  2. ~/.claude/ exists but contains only syllago-managed subdirs → false.
//     This is the regression case for the bug syllago-a6ibm: syllago itself
//     creates ~/.claude/skills/, ~/.claude/rules/, etc., and the old
//     "is dir present?" Detect would report Claude Code as installed even
//     though it isn't.
//  3. claude binary on PATH → true.
//  4. ~/.claude.json marker file present → true. Claude Code writes this
//     on first launch; syllago never writes it.
func TestClaudeCodeDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if ClaudeCode.Detect(home) {
			t.Error("expected false on empty home with no claude binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(home, ".claude", "rules"), 0755); err != nil {
			t.Fatal(err)
		}
		if ClaudeCode.Detect(home) {
			t.Error("expected false when ~/.claude/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "claude")
		if !ClaudeCode.Detect(home) {
			t.Error("expected true when claude binary is on PATH")
		}
	})

	t.Run("marker file present", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		if !ClaudeCode.Detect(home) {
			t.Error("expected true when ~/.claude.json marker file exists")
		}
	})
}
