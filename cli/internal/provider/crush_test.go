package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestCrushHooksSupport: crush.yaml graduated hooks to supported (May 2026);
// Go must claim install support and route hooks through JSON merge into
// crush.json (syllago-h4cmd).
func TestCrushHooksSupport(t *testing.T) {
	if !Crush.SupportsType(catalog.Hooks) {
		t.Error("crush must support hooks (format YAML asserts supported)")
	}
	if got := Crush.InstallDir("/home/x", catalog.Hooks); got != JSONMergeSentinel {
		t.Errorf("InstallDir(hooks): got %q, want JSON merge sentinel", got)
	}
	paths := Crush.DiscoveryPaths("/proj", catalog.Hooks)
	var found bool
	for _, p := range paths {
		if p == filepath.Join("/proj", "crush.json") {
			found = true
		}
	}
	if !found {
		t.Errorf("DiscoveryPaths(hooks) should include crush.json, got %v", paths)
	}
	if Crush.FileFormat(catalog.Hooks) != FormatJSON {
		t.Error("hooks file format should be JSON")
	}
	if Crush.SymlinkSupport[catalog.Hooks] {
		t.Error("hooks use JSON merge, not symlinks")
	}
	if loc := Crush.ConfigLocations[catalog.Hooks]; loc == "" {
		t.Error("ConfigLocations[hooks] must be set (coverage assertion 3)")
	}
	if len(Crush.HookTypes) != 1 || Crush.HookTypes[0] != "command" {
		t.Errorf("HookTypes: got %v, want [command]", Crush.HookTypes)
	}
}

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
