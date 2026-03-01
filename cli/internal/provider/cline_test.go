package provider

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestClineSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.MCP} {
		if !Cline.SupportsType(ct) {
			t.Errorf("Cline.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Skills, catalog.Agents, catalog.Hooks, catalog.Commands} {
		if Cline.SupportsType(ct) {
			t.Errorf("Cline.SupportsType(%s) = true, want false", ct)
		}
	}
}

func TestClineDiscoveryPaths(t *testing.T) {
	t.Parallel()
	paths := Cline.DiscoveryPaths("/tmp/project", catalog.Rules)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Rules, got none")
	}
	found := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".clinerules") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a discovery path ending in .clinerules, got: %v", paths)
	}
}

func TestClineEmitPath(t *testing.T) {
	t.Parallel()
	path := Cline.EmitPath("/tmp/project")
	if path == "" {
		t.Fatal("expected non-empty emit path")
	}
	if !strings.HasSuffix(path, ".clinerules") {
		t.Errorf("expected emit path ending in .clinerules, got %q", path)
	}
}

func TestClineFileFormat(t *testing.T) {
	t.Parallel()
	if got := Cline.FileFormat(catalog.Rules); got != FormatMarkdown {
		t.Errorf("Cline.FileFormat(Rules) = %q, want %q", got, FormatMarkdown)
	}
	if got := Cline.FileFormat(catalog.MCP); got != FormatJSON {
		t.Errorf("Cline.FileFormat(MCP) = %q, want %q", got, FormatJSON)
	}
}
