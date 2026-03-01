package provider

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestDiscoveryPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider Provider
		ct       catalog.ContentType
		wantLen  int
	}{
		{ClaudeCode, catalog.Rules, 2},
		{ClaudeCode, catalog.MCP, 1},
		{ClaudeCode, catalog.Skills, 1},
		{Cursor, catalog.Rules, 1},
		{GeminiCLI, catalog.Rules, 1},
		{Windsurf, catalog.Rules, 1},
		{Codex, catalog.Rules, 1},
	}
	for _, tt := range tests {
		paths := tt.provider.DiscoveryPaths("/tmp/project", tt.ct)
		if len(paths) < tt.wantLen {
			t.Errorf("%s.DiscoveryPaths(%s): got %d paths, want >= %d", tt.provider.Name, tt.ct, len(paths), tt.wantLen)
		}
	}
}

func TestEmitPath(t *testing.T) {
	t.Parallel()
	for _, p := range AllProviders {
		path := p.EmitPath("/tmp/project")
		if path == "" {
			t.Errorf("%s.EmitPath returned empty string", p.Name)
		}
	}
}

func TestSupportsType(t *testing.T) {
	t.Parallel()
	// Claude Code supports Rules, Skills, Agents, Commands, MCP, Hooks
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Skills, catalog.Agents, catalog.Commands, catalog.MCP, catalog.Hooks} {
		if !ClaudeCode.SupportsType(ct) {
			t.Errorf("ClaudeCode.SupportsType(%s) = false, want true", ct)
		}
	}
	// Cursor only supports Rules
	if !Cursor.SupportsType(catalog.Rules) {
		t.Error("Cursor.SupportsType(Rules) = false, want true")
	}
	if Cursor.SupportsType(catalog.Skills) {
		t.Error("Cursor.SupportsType(Skills) = true, want false")
	}
}

func TestDetectedOnly(t *testing.T) {
	t.Parallel()
	// Use a path that won't match real providers
	detected := DetectedOnly("/nonexistent/path")
	for _, p := range detected {
		if !p.Detect("/nonexistent/path") {
			t.Errorf("provider %s returned but Detect is false", p.Name)
		}
	}
}
