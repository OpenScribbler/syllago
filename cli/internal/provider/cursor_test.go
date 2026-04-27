package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCursorDetect: Cursor is an Electron IDE. ~/.cursor/ is shared with
// syllago install paths (skills/, agents/, mcp.json), so we cannot trust
// it as evidence Cursor is installed. Trust the cursor binary on PATH or
// the Electron app-data dir (~/.config/Cursor/ on Linux).
func TestCursorDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Cursor.Detect(home) {
			t.Error("expected false on empty home with no cursor binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".cursor", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(home, ".cursor", "agents"), 0755); err != nil {
			t.Fatal(err)
		}
		if Cursor.Detect(home) {
			t.Error("expected false when ~/.cursor/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "cursor")
		if !Cursor.Detect(home) {
			t.Error("expected true when cursor binary is on PATH")
		}
	})

	t.Run("app-data dir present", func(t *testing.T) {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			t.Skipf("app-data dir test only runs on linux/darwin (got %s)", runtime.GOOS)
		}
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(appDataDir(home, "Cursor"), 0755); err != nil {
			t.Fatal(err)
		}
		if !Cursor.Detect(home) {
			t.Errorf("expected true when %s exists", appDataDir(home, "Cursor"))
		}
	})
}
