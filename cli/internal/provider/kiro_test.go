package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestKiroDetect: Kiro is an Electron IDE (AWS-built). ~/.kiro/ is shared
// with syllago install paths (agents/), so trust the kiro binary on PATH
// or the Electron app-data dir.
func TestKiroDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Kiro.Detect(home) {
			t.Error("expected false on empty home with no kiro binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".kiro", "agents"), 0755); err != nil {
			t.Fatal(err)
		}
		if Kiro.Detect(home) {
			t.Error("expected false when ~/.kiro/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "kiro")
		if !Kiro.Detect(home) {
			t.Error("expected true when kiro binary is on PATH")
		}
	})

	t.Run("app-data dir present", func(t *testing.T) {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			t.Skipf("app-data dir test only runs on linux/darwin (got %s)", runtime.GOOS)
		}
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(appDataDir(home, "Kiro"), 0755); err != nil {
			t.Fatal(err)
		}
		if !Kiro.Detect(home) {
			t.Errorf("expected true when %s exists", appDataDir(home, "Kiro"))
		}
	})
}

func TestKiroSupportsType(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Agents, catalog.Hooks, catalog.MCP, catalog.Skills} {
		if !Kiro.SupportsType(ct) {
			t.Errorf("Kiro must support %s", ct)
		}
	}
	if Kiro.SupportsType(catalog.Commands) {
		t.Error("Kiro must not support Commands")
	}
}

func TestKiroDiscoveryPaths(t *testing.T) {
	t.Parallel()
	paths := Kiro.DiscoveryPaths("/project", catalog.Rules)
	if len(paths) != 1 || paths[0] != filepath.Join("/project", ".kiro", "steering") {
		t.Errorf("unexpected rules paths: %v", paths)
	}
	paths = Kiro.DiscoveryPaths("/project", catalog.MCP)
	if len(paths) != 1 || paths[0] != filepath.Join("/project", ".kiro", "settings", "mcp.json") {
		t.Errorf("unexpected MCP paths: %v", paths)
	}
	paths = Kiro.DiscoveryPaths("/project", catalog.Hooks)
	if len(paths) != 1 || paths[0] != filepath.Join("/project", ".kiro", "agents") {
		t.Errorf("unexpected hooks paths: %v", paths)
	}
}

func TestKiroInstallDir(t *testing.T) {
	t.Parallel()
	if Kiro.InstallDir("/home/user", catalog.MCP) != JSONMergeSentinel {
		t.Error("Kiro MCP must return JSONMergeSentinel")
	}
	if Kiro.InstallDir("/home/user", catalog.Hooks) != JSONMergeSentinel {
		t.Error("Kiro Hooks must return JSONMergeSentinel")
	}
	agentDir := Kiro.InstallDir("/home/user", catalog.Agents)
	expected := filepath.Join("/home/user", ".kiro", "agents")
	if agentDir != expected {
		t.Errorf("expected %q, got %q", expected, agentDir)
	}
}

func TestKiroFileFormat(t *testing.T) {
	t.Parallel()
	if got := Kiro.FileFormat(catalog.Rules); got != FormatMarkdown {
		t.Errorf("Kiro.FileFormat(Rules) = %q, want %q", got, FormatMarkdown)
	}
	if got := Kiro.FileFormat(catalog.MCP); got != FormatJSON {
		t.Errorf("Kiro.FileFormat(MCP) = %q, want %q", got, FormatJSON)
	}
	if got := Kiro.FileFormat(catalog.Agents); got != FormatMarkdown {
		t.Errorf("Kiro.FileFormat(Agents) = %q, want %q", got, FormatMarkdown)
	}
}
