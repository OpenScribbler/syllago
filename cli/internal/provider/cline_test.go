package provider

import (
	"runtime"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestClineSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Hooks, catalog.MCP} {
		if !Cline.SupportsType(ct) {
			t.Errorf("Cline.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Skills, catalog.Agents, catalog.Commands} {
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

func TestClineMCPSettingsPath(t *testing.T) {
	t.Parallel()
	path := ClineMCPSettingsPath()

	// The function returns a non-empty path on all platforms (it calls os.UserHomeDir
	// which succeeds in test environments).
	if path == "" {
		t.Fatal("ClineMCPSettingsPath returned empty string")
	}

	// Verify the path ends with the expected suffix regardless of platform.
	wantSuffix := "cline_mcp_settings.json"
	if !strings.HasSuffix(path, wantSuffix) {
		t.Errorf("ClineMCPSettingsPath: got %q, want suffix %q", path, wantSuffix)
	}

	// Verify the path contains the VS Code globalStorage segment.
	if !strings.Contains(path, "globalStorage") {
		t.Errorf("ClineMCPSettingsPath: expected path to contain 'globalStorage', got %q", path)
	}

	// Platform-specific path segment check.
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(path, "Application Support") {
			t.Errorf("on darwin, expected 'Application Support' in path, got %q", path)
		}
	case "linux":
		if !strings.Contains(path, ".config") {
			t.Errorf("on linux, expected '.config' in path, got %q", path)
		}
	}
}
