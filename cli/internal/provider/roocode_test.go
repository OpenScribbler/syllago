package provider

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestRooCodeSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.MCP, catalog.Agents} {
		if !RooCode.SupportsType(ct) {
			t.Errorf("RooCode.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Skills, catalog.Hooks, catalog.Commands} {
		if RooCode.SupportsType(ct) {
			t.Errorf("RooCode.SupportsType(%s) = true, want false", ct)
		}
	}
}

func TestRooCodeDiscoveryPaths(t *testing.T) {
	t.Parallel()
	paths := RooCode.DiscoveryPaths("/tmp/project", catalog.Rules)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Rules, got none")
	}
	// Should include base .roo/rules and mode-specific dirs
	foundBase := false
	foundModeSpecific := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".roo/rules") {
			foundBase = true
		}
		if strings.Contains(p, "rules-code") || strings.Contains(p, "rules-architect") {
			foundModeSpecific = true
		}
	}
	if !foundBase {
		t.Errorf("expected a discovery path for .roo/rules, got: %v", paths)
	}
	if !foundModeSpecific {
		t.Errorf("expected mode-specific discovery paths, got: %v", paths)
	}

	// MCP discovery
	mcpPaths := RooCode.DiscoveryPaths("/tmp/project", catalog.MCP)
	if len(mcpPaths) == 0 {
		t.Fatal("expected MCP discovery path")
	}
	foundMCP := false
	for _, p := range mcpPaths {
		if strings.HasSuffix(p, "mcp.json") {
			foundMCP = true
		}
	}
	if !foundMCP {
		t.Errorf("expected mcp.json in discovery paths, got: %v", mcpPaths)
	}
}

func TestRooCodeEmitPath(t *testing.T) {
	t.Parallel()
	path := RooCode.EmitPath("/tmp/project")
	if path == "" {
		t.Fatal("expected non-empty emit path")
	}
	if !strings.Contains(path, ".roo") {
		t.Errorf("expected emit path containing .roo, got %q", path)
	}
}

func TestRooCodeFileFormat(t *testing.T) {
	t.Parallel()
	if got := RooCode.FileFormat(catalog.Rules); got != FormatMarkdown {
		t.Errorf("RooCode.FileFormat(Rules) = %q, want %q", got, FormatMarkdown)
	}
	if got := RooCode.FileFormat(catalog.MCP); got != FormatJSON {
		t.Errorf("RooCode.FileFormat(MCP) = %q, want %q", got, FormatJSON)
	}
	if got := RooCode.FileFormat(catalog.Agents); got != FormatYAML {
		t.Errorf("RooCode.FileFormat(Agents) = %q, want %q", got, FormatYAML)
	}
}
