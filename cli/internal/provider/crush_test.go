package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCrushDetect: binary-only marker for v1.
func TestCrushDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Crush.Detect(home) {
			t.Error("expected false on empty home with no crush binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".config", "crush", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if Crush.Detect(home) {
			t.Error("expected false when ~/.config/crush/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "crush")
		if !Crush.Detect(home) {
			t.Error("expected true when crush binary is on PATH")
		}
	})
}
