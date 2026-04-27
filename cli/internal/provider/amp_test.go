package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestAmpSupportsType(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Skills, catalog.MCP, catalog.Hooks} {
		if !Amp.SupportsType(ct) {
			t.Errorf("Amp should support %s", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Agents, catalog.Commands} {
		if Amp.SupportsType(ct) {
			t.Errorf("Amp should NOT support %s", ct)
		}
	}
}

func TestAmpDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Amp.Detect(home) {
			t.Error("expected false on empty home with no amp binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".config", "amp", "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		if Amp.Detect(home) {
			t.Error("expected false when ~/.config/amp/ contains only syllago content (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "amp")
		if !Amp.Detect(home) {
			t.Error("expected true when amp binary is on PATH")
		}
	})
}

func TestAmpInstallDir(t *testing.T) {
	t.Parallel()
	home := "/home/testuser"

	rules := Amp.InstallDir(home, catalog.Rules)
	if rules != filepath.Join(home, ".config", "amp") {
		t.Errorf("Rules install dir = %q, want %q", rules, filepath.Join(home, ".config", "amp"))
	}

	skills := Amp.InstallDir(home, catalog.Skills)
	if skills != filepath.Join(home, ".config", "agents", "skills") {
		t.Errorf("Skills install dir = %q, want %q", skills, filepath.Join(home, ".config", "agents", "skills"))
	}

	mcp := Amp.InstallDir(home, catalog.MCP)
	if mcp != JSONMergeSentinel {
		t.Errorf("MCP install dir = %q, want %q", mcp, JSONMergeSentinel)
	}

	hooks := Amp.InstallDir(home, catalog.Hooks)
	if hooks != JSONMergeSentinel {
		t.Errorf("Hooks install dir = %q, want %q", hooks, JSONMergeSentinel)
	}
}

func TestAmpHooksSupport(t *testing.T) {
	t.Parallel()

	paths := Amp.DiscoveryPaths("/project", catalog.Hooks)
	if len(paths) != 1 {
		t.Fatalf("Hooks discovery paths = %d, want 1", len(paths))
	}
	wantPath := filepath.Join("/project", ".amp", "settings.json")
	if paths[0] != wantPath {
		t.Errorf("Hooks[0] = %q, want %q", paths[0], wantPath)
	}

	if got := Amp.ConfigLocations[catalog.Hooks]; got != ".amp/settings.json" {
		t.Errorf("ConfigLocations[Hooks] = %q, want .amp/settings.json (hooks live in settings.json, not .amp/hooks.json)", got)
	}

	if Amp.SymlinkSupport[catalog.Hooks] {
		t.Error("Hooks should NOT support symlinks (JSON merge)")
	}

	if len(Amp.HookTypes) == 0 {
		t.Error("HookTypes should be non-empty")
	}
}

func TestAmpDiscoveryPaths(t *testing.T) {
	t.Parallel()
	root := "/project"

	rules := Amp.DiscoveryPaths(root, catalog.Rules)
	// Amp discovers AGENTS.md, AGENT.md (fallback), and CLAUDE.md (fallback)
	if len(rules) != 3 {
		t.Fatalf("Rules discovery paths = %d, want 3", len(rules))
	}
	if rules[0] != filepath.Join(root, "AGENTS.md") {
		t.Errorf("Rules[0] = %q, want AGENTS.md", rules[0])
	}
	if rules[1] != filepath.Join(root, "AGENT.md") {
		t.Errorf("Rules[1] = %q, want AGENT.md", rules[1])
	}
	if rules[2] != filepath.Join(root, "CLAUDE.md") {
		t.Errorf("Rules[2] = %q, want CLAUDE.md", rules[2])
	}

	skills := Amp.DiscoveryPaths(root, catalog.Skills)
	if len(skills) != 2 {
		t.Fatalf("Skills discovery paths = %d, want 2", len(skills))
	}
	if skills[0] != filepath.Join(root, ".agents", "skills") {
		t.Errorf("Skills[0] = %q, want .agents/skills", skills[0])
	}
	if skills[1] != filepath.Join(root, ".claude", "skills") {
		t.Errorf("Skills[1] = %q, want .claude/skills", skills[1])
	}

	mcp := Amp.DiscoveryPaths(root, catalog.MCP)
	if len(mcp) != 1 {
		t.Fatalf("MCP discovery paths = %d, want 1", len(mcp))
	}
	if mcp[0] != filepath.Join(root, ".amp", "settings.json") {
		t.Errorf("MCP[0] = %q, want .amp/settings.json", mcp[0])
	}
}

func TestAmpFileFormat(t *testing.T) {
	t.Parallel()
	if Amp.FileFormat(catalog.Rules) != FormatMarkdown {
		t.Error("Rules format should be Markdown")
	}
	if Amp.FileFormat(catalog.Skills) != FormatMarkdown {
		t.Error("Skills format should be Markdown")
	}
	if Amp.FileFormat(catalog.MCP) != FormatJSON {
		t.Error("MCP format should be JSON")
	}
	if Amp.FileFormat(catalog.Hooks) != FormatJSON {
		t.Error("Hooks format should be JSON (hooks live in .amp/settings.json via JSON merge)")
	}
}

func TestAmpEmitPath(t *testing.T) {
	t.Parallel()
	root := "/project"
	if Amp.EmitPath(root) != filepath.Join(root, "AGENTS.md") {
		t.Errorf("EmitPath = %q, want AGENTS.md", Amp.EmitPath(root))
	}
}

func TestAmpSymlinkSupport(t *testing.T) {
	t.Parallel()
	if !Amp.SymlinkSupport[catalog.Rules] {
		t.Error("Rules should support symlinks")
	}
	if !Amp.SymlinkSupport[catalog.Skills] {
		t.Error("Skills should support symlinks")
	}
	if Amp.SymlinkSupport[catalog.MCP] {
		t.Error("MCP should NOT support symlinks (JSON merge)")
	}
}
