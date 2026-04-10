package installer

import (
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// stubInstaller returns a provider whose InstallDir for Skills resolves to the
// given path. Represents a provider that WRITES to a path (e.g. Codex → ~/.agents/skills/).
func stubInstaller(slug, installPath string) provider.Provider {
	return provider.Provider{
		Slug: slug,
		Name: slug,
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return installPath
			}
			return ""
		},
	}
}

// stubReader returns a provider with its own InstallDir AND a GlobalSharedReadPaths
// that includes sharedPath. Represents a provider that installs to its own dir
// but ALSO reads from a shared path (e.g. Gemini → reads ~/.agents/skills/).
func stubReader(slug, ownInstallPath, sharedPath string) provider.Provider {
	return provider.Provider{
		Slug: slug,
		Name: slug,
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return ownInstallPath
			}
			return ""
		},
		GlobalSharedReadPaths: func(home string, ct catalog.ContentType) []string {
			if ct == catalog.Skills {
				return []string{sharedPath}
			}
			return nil
		},
	}
}

const home = "/home/user"

func agentsSkills() string { return filepath.Join(home, ".agents", "skills") }

// TestDetectConflicts_SingleConflict: Codex installs to ~/.agents/skills/,
// Gemini also reads from there — one conflict with one reader.
func TestDetectConflicts_SingleConflict(t *testing.T) {
	t.Parallel()
	codex := stubInstaller("codex", agentsSkills())
	gemini := stubReader("gemini-cli", filepath.Join(home, ".gemini", "skills"), agentsSkills())

	conflicts := DetectConflicts([]provider.Provider{codex, gemini}, catalog.Skills, home)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %v", len(conflicts), conflicts)
	}
	c := conflicts[0]
	if c.SharedPath != agentsSkills() {
		t.Errorf("SharedPath: want %q, got %q", agentsSkills(), c.SharedPath)
	}
	if c.InstallingTo.Slug != "codex" {
		t.Errorf("InstallingTo: want codex, got %q", c.InstallingTo.Slug)
	}
	if len(c.AlsoReadBy) != 1 || c.AlsoReadBy[0].Slug != "gemini-cli" {
		t.Errorf("AlsoReadBy: want [gemini-cli], got %v", slugs(c.AlsoReadBy))
	}
}

// TestDetectConflicts_MultipleReaders: Codex installs, Gemini + Windsurf both read.
// Should be one Conflict with two entries in AlsoReadBy.
func TestDetectConflicts_MultipleReaders(t *testing.T) {
	t.Parallel()
	codex := stubInstaller("codex", agentsSkills())
	gemini := stubReader("gemini-cli", filepath.Join(home, ".gemini", "skills"), agentsSkills())
	windsurf := stubReader("windsurf", filepath.Join(home, ".codeium", "windsurf", "skills"), agentsSkills())

	conflicts := DetectConflicts([]provider.Provider{codex, gemini, windsurf}, catalog.Skills, home)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if len(conflicts[0].AlsoReadBy) != 2 {
		t.Errorf("expected 2 readers, got %d: %v", len(conflicts[0].AlsoReadBy), slugs(conflicts[0].AlsoReadBy))
	}
}

// TestDetectConflicts_NoConflict_OnlyReader: Only Gemini selected (no Codex).
// Gemini's InstallDir is ~/.gemini/skills/ — nobody installs to ~/.agents/skills/.
func TestDetectConflicts_NoConflict_OnlyReader(t *testing.T) {
	t.Parallel()
	gemini := stubReader("gemini-cli", filepath.Join(home, ".gemini", "skills"), agentsSkills())
	claudeCode := stubInstaller("claude-code", filepath.Join(home, ".claude", "skills"))

	conflicts := DetectConflicts([]provider.Provider{gemini, claudeCode}, catalog.Skills, home)

	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

// TestDetectConflicts_NoConflict_NonSkillsType: Conflict detection only applies
// to Skills — the .agents/ convention is skills-only.
func TestDetectConflicts_NoConflict_NonSkillsType(t *testing.T) {
	t.Parallel()
	codex := stubInstaller("codex", agentsSkills())
	gemini := stubReader("gemini-cli", filepath.Join(home, ".gemini", "skills"), agentsSkills())

	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Agents, catalog.Commands, catalog.Hooks, catalog.MCP} {
		conflicts := DetectConflicts([]provider.Provider{codex, gemini}, ct, home)
		if len(conflicts) != 0 {
			t.Errorf("type %s: expected no conflicts, got %d", ct, len(conflicts))
		}
	}
}

// TestDetectConflicts_Empty: empty or single-provider lists never conflict.
func TestDetectConflicts_Empty(t *testing.T) {
	t.Parallel()
	codex := stubInstaller("codex", agentsSkills())

	if got := DetectConflicts(nil, catalog.Skills, home); len(got) != 0 {
		t.Errorf("nil providers: expected no conflicts, got %d", len(got))
	}
	if got := DetectConflicts([]provider.Provider{codex}, catalog.Skills, home); len(got) != 0 {
		t.Errorf("single provider: expected no conflicts, got %d", len(got))
	}
}

// TestDetectConflicts_RealProviders: integration check using the actual configured
// provider vars. Codex + GeminiCLI should produce a conflict at ~/.agents/skills/.
func TestDetectConflicts_RealProviders(t *testing.T) {
	t.Parallel()
	conflicts := DetectConflicts(
		[]provider.Provider{provider.Codex, provider.GeminiCLI},
		catalog.Skills,
		home,
	)
	if len(conflicts) == 0 {
		t.Fatal("expected a conflict between Codex and GeminiCLI for Skills, got none")
	}
}

// TestApplyConflictResolution_SharedOnly: keep the path owner (Codex), drop readers.
// Readers still get the content because they read from the shared path.
func TestApplyConflictResolution_SharedOnly(t *testing.T) {
	t.Parallel()
	codex := stubInstaller("codex", agentsSkills())
	gemini := stubReader("gemini-cli", filepath.Join(home, ".gemini", "skills"), agentsSkills())
	windsurf := stubReader("windsurf", filepath.Join(home, ".codeium", "windsurf", "skills"), agentsSkills())
	claudeCode := stubInstaller("claude-code", filepath.Join(home, ".claude", "skills"))

	allProviders := []provider.Provider{codex, gemini, windsurf, claudeCode}
	conflicts := DetectConflicts(allProviders, catalog.Skills, home)

	result := ApplyConflictResolution(allProviders, conflicts, ResolutionSharedOnly)

	got := slugs(result)
	// Codex (installer) and ClaudeCode (not in conflict) stay; Gemini + Windsurf (readers) removed.
	if !containsSlug(got, "codex") {
		t.Errorf("SharedOnly: expected codex to be kept, got %v", got)
	}
	if !containsSlug(got, "claude-code") {
		t.Errorf("SharedOnly: expected claude-code to be kept, got %v", got)
	}
	if containsSlug(got, "gemini-cli") {
		t.Errorf("SharedOnly: expected gemini-cli to be removed (reads shared path), got %v", got)
	}
	if containsSlug(got, "windsurf") {
		t.Errorf("SharedOnly: expected windsurf to be removed (reads shared path), got %v", got)
	}
}

// TestApplyConflictResolution_OwnDirsOnly: drop the path owner (Codex), keep readers.
// Each reader installs to its own canonical dir; no write to the shared path.
func TestApplyConflictResolution_OwnDirsOnly(t *testing.T) {
	t.Parallel()
	codex := stubInstaller("codex", agentsSkills())
	gemini := stubReader("gemini-cli", filepath.Join(home, ".gemini", "skills"), agentsSkills())
	windsurf := stubReader("windsurf", filepath.Join(home, ".codeium", "windsurf", "skills"), agentsSkills())
	claudeCode := stubInstaller("claude-code", filepath.Join(home, ".claude", "skills"))

	allProviders := []provider.Provider{codex, gemini, windsurf, claudeCode}
	conflicts := DetectConflicts(allProviders, catalog.Skills, home)

	result := ApplyConflictResolution(allProviders, conflicts, ResolutionOwnDirsOnly)

	got := slugs(result)
	if containsSlug(got, "codex") {
		t.Errorf("OwnDirsOnly: expected codex to be removed (installs to shared path), got %v", got)
	}
	if !containsSlug(got, "gemini-cli") {
		t.Errorf("OwnDirsOnly: expected gemini-cli to be kept, got %v", got)
	}
	if !containsSlug(got, "windsurf") {
		t.Errorf("OwnDirsOnly: expected windsurf to be kept, got %v", got)
	}
	if !containsSlug(got, "claude-code") {
		t.Errorf("OwnDirsOnly: expected claude-code to be kept, got %v", got)
	}
}

// TestApplyConflictResolution_All: current behavior — return all providers unchanged.
func TestApplyConflictResolution_All(t *testing.T) {
	t.Parallel()
	codex := stubInstaller("codex", agentsSkills())
	gemini := stubReader("gemini-cli", filepath.Join(home, ".gemini", "skills"), agentsSkills())

	allProviders := []provider.Provider{codex, gemini}
	conflicts := DetectConflicts(allProviders, catalog.Skills, home)

	result := ApplyConflictResolution(allProviders, conflicts, ResolutionAll)

	if len(result) != len(allProviders) {
		t.Errorf("All: expected %d providers, got %d: %v", len(allProviders), len(result), slugs(result))
	}
}

// TestApplyConflictResolution_NoConflicts: with no conflicts, all resolutions return
// the provider list unchanged.
func TestApplyConflictResolution_NoConflicts(t *testing.T) {
	t.Parallel()
	claudeCode := stubInstaller("claude-code", filepath.Join(home, ".claude", "skills"))
	cursor := stubInstaller("cursor", filepath.Join(home, ".cursor", "skills"))
	allProviders := []provider.Provider{claudeCode, cursor}

	for _, res := range []ConflictResolution{ResolutionSharedOnly, ResolutionOwnDirsOnly, ResolutionAll} {
		result := ApplyConflictResolution(allProviders, nil, res)
		if len(result) != len(allProviders) {
			t.Errorf("resolution %d with no conflicts: expected %d providers, got %d", res, len(allProviders), len(result))
		}
	}
}

// slugs extracts provider slugs for readable test output.
func slugs(ps []provider.Provider) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Slug
	}
	return out
}

func containsSlug(ss []string, slug string) bool {
	for _, s := range ss {
		if s == slug {
			return true
		}
	}
	return false
}
