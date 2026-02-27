package provider

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestZedSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.MCP} {
		if !Zed.SupportsType(ct) {
			t.Errorf("Zed.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Skills, catalog.Agents, catalog.Hooks, catalog.Commands} {
		if Zed.SupportsType(ct) {
			t.Errorf("Zed.SupportsType(%s) = true, want false", ct)
		}
	}
}

func TestZedDiscoveryPaths(t *testing.T) {
	t.Parallel()
	paths := Zed.DiscoveryPaths("/tmp/project", catalog.Rules)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Rules, got none")
	}
	found := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".rules") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a discovery path ending in .rules, got: %v", paths)
	}
}

func TestZedEmitPath(t *testing.T) {
	t.Parallel()
	path := Zed.EmitPath("/tmp/project")
	if path == "" {
		t.Fatal("expected non-empty emit path")
	}
	if !strings.HasSuffix(path, ".rules") {
		t.Errorf("expected emit path ending in .rules, got %q", path)
	}
}

func TestZedFileFormat(t *testing.T) {
	t.Parallel()
	if got := Zed.FileFormat(catalog.Rules); got != FormatMarkdown {
		t.Errorf("Zed.FileFormat(Rules) = %q, want %q", got, FormatMarkdown)
	}
	if got := Zed.FileFormat(catalog.MCP); got != FormatJSON {
		t.Errorf("Zed.FileFormat(MCP) = %q, want %q", got, FormatJSON)
	}
}
