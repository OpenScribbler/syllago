package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCodexDetect mirrors the four-case shape used for Claude Code:
// empty/regression/binary/marker. Codex's marker is ~/.codex/auth.json,
// which Codex writes when the user authenticates.
func TestCodexDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Codex.Detect(home) {
			t.Error("expected false on empty home with no codex binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		// Codex stores rules/agents/commands directly under ~/.codex (file mode),
		// so syllago could leave just the dir + a few files there.
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".codex", "AGENTS.md"), []byte("syllago"), 0644); err != nil {
			t.Fatal(err)
		}
		if Codex.Detect(home) {
			t.Error("expected false when ~/.codex/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "codex")
		if !Codex.Detect(home) {
			t.Error("expected true when codex binary is on PATH")
		}
	})

	t.Run("marker file present", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		if !Codex.Detect(home) {
			t.Error("expected true when ~/.codex/auth.json marker file exists")
		}
	})
}
