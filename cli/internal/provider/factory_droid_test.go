package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFactoryDroidDetect: Factory ships its CLI as `droid`, not `factory`.
// Binary-only marker for v1.
func TestFactoryDroidDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if FactoryDroid.Detect(home) {
			t.Error("expected false on empty home with no droid binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".factory", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if FactoryDroid.Detect(home) {
			t.Error("expected false when ~/.factory/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "droid")
		if !FactoryDroid.Detect(home) {
			t.Error("expected true when droid binary is on PATH")
		}
	})
}
