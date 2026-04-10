package provider

import (
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestGlobalSharedReadPaths_SkillsConflictProviders verifies that the four
// providers which read ~/.agents/skills/ globally (beyond their own InstallDir)
// declare that path via GlobalSharedReadPaths. This is used by conflict detection
// to warn when multiple providers would receive the same content.
func TestGlobalSharedReadPaths_SkillsConflictProviders(t *testing.T) {
	t.Parallel()
	home := "/home/user"
	want := filepath.Join(home, ".agents", "skills")

	cases := []struct {
		name     string
		provider Provider
	}{
		{"GeminiCLI", GeminiCLI},
		{"Windsurf", Windsurf},
		{"RooCode", RooCode},
		{"OpenCode", OpenCode},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.provider.GlobalSharedReadPaths == nil {
				t.Fatalf("%s: GlobalSharedReadPaths is nil", tc.name)
			}
			paths := tc.provider.GlobalSharedReadPaths(home, catalog.Skills)
			if len(paths) == 0 {
				t.Fatalf("%s: expected at least one path for Skills, got empty", tc.name)
			}
			found := false
			for _, p := range paths {
				if p == want {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: expected %q in GlobalSharedReadPaths for Skills, got %v", tc.name, want, paths)
			}
		})
	}
}

// TestGlobalSharedReadPaths_NonConflictProviders verifies that providers which
// do NOT read ~/.agents/skills/ globally return nil or empty for Skills.
func TestGlobalSharedReadPaths_NonConflictProviders(t *testing.T) {
	t.Parallel()
	home := "/home/user"

	cases := []struct {
		name     string
		provider Provider
	}{
		{"ClaudeCode", ClaudeCode},
		{"Codex", Codex},
		{"Cursor", Cursor},
		{"CopilotCLI", CopilotCLI},
		{"Zed", Zed},
		{"Cline", Cline},
		{"Kiro", Kiro},
		{"Amp", Amp},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.provider.GlobalSharedReadPaths == nil {
				return // nil is fine — means no shared paths
			}
			paths := tc.provider.GlobalSharedReadPaths(home, catalog.Skills)
			if len(paths) > 0 {
				t.Errorf("%s: expected no GlobalSharedReadPaths for Skills, got %v", tc.name, paths)
			}
		})
	}
}

// TestGlobalSharedReadPaths_NonSkillsTypes verifies that GlobalSharedReadPaths
// returns nil/empty for content types other than Skills — the .agents/
// cross-provider convention is skills-only.
func TestGlobalSharedReadPaths_NonSkillsTypes(t *testing.T) {
	t.Parallel()
	home := "/home/user"
	nonSkillTypes := []catalog.ContentType{
		catalog.Rules, catalog.Agents, catalog.Commands, catalog.Hooks, catalog.MCP,
	}

	cases := []struct {
		name     string
		provider Provider
	}{
		{"GeminiCLI", GeminiCLI},
		{"Windsurf", Windsurf},
		{"RooCode", RooCode},
		{"OpenCode", OpenCode},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.provider.GlobalSharedReadPaths == nil {
				return
			}
			for _, ct := range nonSkillTypes {
				paths := tc.provider.GlobalSharedReadPaths(home, ct)
				if len(paths) > 0 {
					t.Errorf("%s: expected no shared read paths for %s, got %v", tc.name, ct, paths)
				}
			}
		})
	}
}
