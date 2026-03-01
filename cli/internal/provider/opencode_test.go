package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestOpenCodeDetect(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if OpenCode.Detect(dir) {
		t.Fatal("expected no detection in empty temp dir")
	}

	if err := os.MkdirAll(filepath.Join(dir, ".config", "opencode"), 0755); err != nil {
		t.Fatal(err)
	}
	if !OpenCode.Detect(dir) {
		t.Fatal("expected detection when ~/.config/opencode/ exists")
	}
}

func TestOpenCodeSupportsType(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Commands, catalog.Agents, catalog.Skills, catalog.MCP} {
		if !OpenCode.SupportsType(ct) {
			t.Errorf("OpenCode must support %s", ct)
		}
	}
	if OpenCode.SupportsType(catalog.Hooks) {
		t.Error("OpenCode must not support Hooks")
	}
}

func TestOpenCodeFileFormat(t *testing.T) {
	t.Parallel()
	if OpenCode.FileFormat(catalog.MCP) != FormatJSONC {
		t.Error("OpenCode MCP format must be FormatJSONC")
	}
	if OpenCode.FileFormat(catalog.Rules) != FormatMarkdown {
		t.Error("OpenCode Rules format must be FormatMarkdown")
	}
}

func TestOpenCodeDiscoveryPaths(t *testing.T) {
	t.Parallel()
	paths := OpenCode.DiscoveryPaths("/project", catalog.Rules)
	if len(paths) != 1 || paths[0] != filepath.Join("/project", "AGENTS.md") {
		t.Errorf("expected /project/AGENTS.md, got %v", paths)
	}

	paths = OpenCode.DiscoveryPaths("/project", catalog.MCP)
	if len(paths) != 2 {
		t.Fatalf("expected 2 MCP discovery paths, got %d", len(paths))
	}
}

func TestOpenCodeInstallDir(t *testing.T) {
	t.Parallel()
	skillDir := OpenCode.InstallDir("/home/user", catalog.Skills)
	expected := filepath.Join("/home/user", ".config", "opencode", "skill")
	if skillDir != expected {
		t.Errorf("expected %q, got %q", expected, skillDir)
	}
	if OpenCode.InstallDir("/home/user", catalog.MCP) != JSONMergeSentinel {
		t.Error("MCP must return JSONMergeSentinel")
	}
}
