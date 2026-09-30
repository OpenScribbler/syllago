package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestDevinDetect: Devin is an Electron IDE. ~/.codeium/windsurf/ is
// shared with syllago install paths (skills/, global_workflows/), so trust
// the windsurf binary on PATH or the Electron app-data dir.
func TestDevinDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Devin.Detect(home) {
			t.Error("expected false on empty home with no windsurf binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".codeium", "windsurf", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if Devin.Detect(home) {
			t.Error("expected false when ~/.codeium/windsurf/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "windsurf")
		if !Devin.Detect(home) {
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
		if !Devin.Detect(home) {
			t.Errorf("expected true when %s exists", appDataDir(home, "Windsurf"))
		}
	})
}

func TestDevinSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Skills, catalog.Hooks, catalog.MCP, catalog.Commands, catalog.Agents} {
		if !Devin.SupportsType(ct) {
			t.Errorf("Devin.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Loadouts} {
		if Devin.SupportsType(ct) {
			t.Errorf("Devin.SupportsType(%s) = true, want false", ct)
		}
	}
}

func TestDevinCommandsSupport(t *testing.T) {
	t.Parallel()

	home := "/home/testuser"
	installDir := Devin.InstallDir(home, catalog.Commands)
	wantInstall := filepath.Join(home, ".codeium", "windsurf", "global_workflows")
	if installDir != wantInstall {
		t.Errorf("Devin.InstallDir(Commands) = %q, want %q", installDir, wantInstall)
	}

	paths := Devin.DiscoveryPaths("/tmp/project", catalog.Commands)
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

	if got := Devin.FileFormat(catalog.Commands); got != FormatMarkdown {
		t.Errorf("Devin.FileFormat(Commands) = %q, want %q", got, FormatMarkdown)
	}

	if !Devin.SymlinkSupport[catalog.Commands] {
		t.Errorf("Devin.SymlinkSupport[Commands] = false, want true")
	}
}

func TestDevinEmitPath(t *testing.T) {
	t.Parallel()
	path := Devin.EmitPath("/tmp/project")
	if path == "" {
		t.Fatal("expected non-empty emit path")
	}
	if !strings.Contains(path, ".windsurf") {
		t.Errorf("expected emit path containing .windsurf, got %q", path)
	}
}

func TestDevinAgentsSupport(t *testing.T) {
	t.Parallel()

	home := "/home/testuser"
	if got, want := Devin.InstallDir(home, catalog.Agents), filepath.Join(home, ".config", "devin", "agents"); got != want {
		t.Errorf("Devin.InstallDir(Agents) = %q, want %q", got, want)
	}
	paths := Devin.DiscoveryPaths("/proj", catalog.Agents)
	want := []string{filepath.Join("/proj", ".devin", "agents"), filepath.Join("/proj", ".agents", "agents")}
	if len(paths) != len(want) {
		t.Fatalf("Devin.DiscoveryPaths(Agents) = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("Devin.DiscoveryPaths(Agents)[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
	if supported, ok := Devin.SymlinkSupport[catalog.Agents]; !ok || supported {
		t.Error("Devin.SymlinkSupport[Agents] should be present and false: agents install as rendered copies")
	}
	if got := Devin.FileFormat(catalog.Agents); got != FormatMarkdown {
		t.Errorf("Devin.FileFormat(Agents) = %q, want %q", got, FormatMarkdown)
	}
}
