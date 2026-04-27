package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestWindsurfDetect: Windsurf is an Electron IDE. ~/.codeium/windsurf/ is
// shared with syllago install paths (skills/, global_workflows/), so trust
// the windsurf binary on PATH or the Electron app-data dir.
func TestWindsurfDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Windsurf.Detect(home) {
			t.Error("expected false on empty home with no windsurf binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".codeium", "windsurf", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if Windsurf.Detect(home) {
			t.Error("expected false when ~/.codeium/windsurf/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "windsurf")
		if !Windsurf.Detect(home) {
			t.Error("expected true when windsurf binary is on PATH")
		}
	})

	t.Run("app-data dir present", func(t *testing.T) {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			t.Skipf("app-data dir test only runs on linux/darwin (got %s)", runtime.GOOS)
		}
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(appDataDir(home, "Windsurf"), 0755); err != nil {
			t.Fatal(err)
		}
		if !Windsurf.Detect(home) {
			t.Errorf("expected true when %s exists", appDataDir(home, "Windsurf"))
		}
	})
}

func TestWindsurfSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Skills, catalog.Hooks, catalog.MCP, catalog.Commands} {
		if !Windsurf.SupportsType(ct) {
			t.Errorf("Windsurf.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Agents, catalog.Loadouts} {
		if Windsurf.SupportsType(ct) {
			t.Errorf("Windsurf.SupportsType(%s) = true, want false", ct)
		}
	}
}

func TestWindsurfCommandsSupport(t *testing.T) {
	t.Parallel()

	home := "/home/testuser"
	installDir := Windsurf.InstallDir(home, catalog.Commands)
	wantInstall := filepath.Join(home, ".codeium", "windsurf", "global_workflows")
	if installDir != wantInstall {
		t.Errorf("Windsurf.InstallDir(Commands) = %q, want %q", installDir, wantInstall)
	}

	paths := Windsurf.DiscoveryPaths("/tmp/project", catalog.Commands)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Commands")
	}
	foundProject := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".windsurf/workflows") && strings.Contains(p, "/tmp/project") {
			foundProject = true
		}
	}
	if !foundProject {
		t.Errorf("expected project discovery path ending in .windsurf/workflows, got: %v", paths)
	}

	if got := Windsurf.FileFormat(catalog.Commands); got != FormatMarkdown {
		t.Errorf("Windsurf.FileFormat(Commands) = %q, want %q", got, FormatMarkdown)
	}

	if !Windsurf.SymlinkSupport[catalog.Commands] {
		t.Errorf("Windsurf.SymlinkSupport[Commands] = false, want true")
	}
}

func TestWindsurfEmitPath(t *testing.T) {
	t.Parallel()
	path := Windsurf.EmitPath("/tmp/project")
	if path == "" {
		t.Fatal("expected non-empty emit path")
	}
	if !strings.Contains(path, ".windsurf") {
		t.Errorf("expected emit path containing .windsurf, got %q", path)
	}
}
