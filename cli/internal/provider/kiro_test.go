package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestKiroDetect(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if Kiro.Detect(dir) {
		t.Fatal("expected no detection in empty temp dir")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".kiro"), 0755); err != nil {
		t.Fatal(err)
	}
	if !Kiro.Detect(dir) {
		t.Fatal("expected detection when ~/.kiro/ exists")
	}
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
	if got := Kiro.FileFormat(catalog.Agents); got != FormatJSON {
		t.Errorf("Kiro.FileFormat(Agents) = %q, want %q", got, FormatJSON)
	}
}
