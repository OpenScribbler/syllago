package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPiDetect: binary-only marker for v1.
func TestPiDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Pi.Detect(home) {
			t.Error("expected false on empty home with no pi binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".pi", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if Pi.Detect(home) {
			t.Error("expected false when ~/.pi/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "pi")
		if !Pi.Detect(home) {
			t.Error("expected true when pi binary is on PATH")
		}
	})
}
