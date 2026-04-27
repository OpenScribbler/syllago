package provider

import (
	"os"
	"testing"
)

// TestCopilotDetect: Copilot CLI ships as a gh extension (`gh copilot`).
// Detection shells out to `gh extension list` via the package-level
// ghLookPath/ghRunCommand overrides defined in detect_marker.go.
//
// homeDir is irrelevant — the only signal is the gh extension state.
func TestCopilotDetect(t *testing.T) {
	t.Run("gh not on PATH", func(t *testing.T) {
		origLook := ghLookPath
		t.Cleanup(func() { ghLookPath = origLook })
		ghLookPath = func(_ string) (string, error) { return "", os.ErrNotExist }

		if CopilotCLI.Detect(t.TempDir()) {
			t.Error("expected false when gh is not on PATH")
		}
	})

	t.Run("gh on PATH but no copilot extension", func(t *testing.T) {
		origLook, origRun := ghLookPath, ghRunCommand
		t.Cleanup(func() {
			ghLookPath = origLook
			ghRunCommand = origRun
		})
		ghLookPath = func(_ string) (string, error) { return "/usr/bin/gh", nil }
		ghRunCommand = func(_ string, _ ...string) ([]byte, error) {
			return []byte("github/some-other-extension\nuser/another\n"), nil
		}

		if CopilotCLI.Detect(t.TempDir()) {
			t.Error("expected false when gh extension list lacks gh-copilot")
		}
	})

	t.Run("gh-copilot installed", func(t *testing.T) {
		origLook, origRun := ghLookPath, ghRunCommand
		t.Cleanup(func() {
			ghLookPath = origLook
			ghRunCommand = origRun
		})
		ghLookPath = func(_ string) (string, error) { return "/usr/bin/gh", nil }
		ghRunCommand = func(_ string, _ ...string) ([]byte, error) {
			return []byte("github/gh-copilot\n"), nil
		}

		if !CopilotCLI.Detect(t.TempDir()) {
			t.Error("expected true when gh extension list contains gh-copilot")
		}
	})

	t.Run("gh extension list errors out", func(t *testing.T) {
		origLook, origRun := ghLookPath, ghRunCommand
		t.Cleanup(func() {
			ghLookPath = origLook
			ghRunCommand = origRun
		})
		ghLookPath = func(_ string) (string, error) { return "/usr/bin/gh", nil }
		ghRunCommand = func(_ string, _ ...string) ([]byte, error) {
			return nil, os.ErrPermission
		}

		if CopilotCLI.Detect(t.TempDir()) {
			t.Error("expected false when gh extension list errors out (must not panic)")
		}
	})
}
