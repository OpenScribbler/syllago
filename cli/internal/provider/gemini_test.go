package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGeminiDetect: binary-only marker for v1. Gemini's home dir is also a
// syllago install target for skills, so the old "dir exists" check is
// unreliable.
func TestGeminiDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if GeminiCLI.Detect(home) {
			t.Error("expected false on empty home with no gemini binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".gemini", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if GeminiCLI.Detect(home) {
			t.Error("expected false when ~/.gemini/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "gemini")
		if !GeminiCLI.Detect(home) {
			t.Error("expected true when gemini binary is on PATH")
		}
	})
}
