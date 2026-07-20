package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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

func TestPiInstallDir(t *testing.T) {
	t.Parallel()
	if got := Pi.InstallDir("/home/user", catalog.Hooks); got != JSONMergeSentinel {
		t.Errorf("Pi hooks install dir = %q, want JSONMergeSentinel", got)
	}
	if got := Pi.InstallDir("/home/user", catalog.Skills); got != filepath.Join("/home/user", ".pi", "agent", "skills") {
		t.Errorf("Pi skills install dir = %q", got)
	}
	if got := Pi.InstallDir("/home/user", catalog.Commands); got != filepath.Join("/home/user", ".pi", "agent", "prompts") {
		t.Errorf("Pi commands install dir = %q", got)
	}
}

func TestPiHooksAreAdapterRouted(t *testing.T) {
	t.Parallel()
	if Pi.SymlinkSupport[catalog.Hooks] {
		t.Error("Pi hooks should not be symlinked; they are adapter-encoded TypeScript")
	}
	paths := Pi.DiscoveryPaths("/project", catalog.Hooks)
	if len(paths) != 1 || paths[0] != filepath.Join("/project", ".pi", "extensions") {
		t.Errorf("unexpected project hook discovery paths: %v", paths)
	}
}
