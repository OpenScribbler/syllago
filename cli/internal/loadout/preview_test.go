package loadout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestPreview_AllNew(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	repoRoot := t.TempDir()

	// Create .syllago dir for installed.json
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(home, ".test", "rules")
			case catalog.Skills:
				return filepath.Join(home, ".test", "skills")
			}
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Rules, Name: "my-rule", Item: catalog.ContentItem{
			Name: "my-rule", Type: catalog.Rules, Path: "/repo/content/rules/test/my-rule",
		}},
		{Type: catalog.Skills, Name: "my-skill", Item: catalog.ContentItem{
			Name: "my-skill", Type: catalog.Skills, Path: "/repo/content/skills/my-skill",
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, homeDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	for _, a := range actions {
		if a.Action != "create-symlink" {
			t.Errorf("expected create-symlink for %s, got %s", a.Name, a.Action)
		}
	}
}

func TestPreview_ExistingSameTarget(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	skillsDir := filepath.Join(homeDir, ".test", "skills")
	os.MkdirAll(skillsDir, 0755)

	// Create source directory
	sourceDir := filepath.Join(repoRoot, "content", "skills", "my-skill")
	os.MkdirAll(sourceDir, 0755)

	// Create symlink pointing to the same source
	targetPath := filepath.Join(skillsDir, "my-skill")
	os.Symlink(sourceDir, targetPath)

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return filepath.Join(home, ".test", "skills")
			}
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Skills, Name: "my-skill", Item: catalog.ContentItem{
			Name: "my-skill", Type: catalog.Skills, Path: sourceDir,
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, homeDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "skip-exists" {
		t.Errorf("expected skip-exists, got %s", actions[0].Action)
	}
}

func TestPreview_Conflict(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	skillsDir := filepath.Join(homeDir, ".test", "skills")
	os.MkdirAll(skillsDir, 0755)

	// Create a symlink pointing to a DIFFERENT source
	targetPath := filepath.Join(skillsDir, "my-skill")
	os.Symlink("/some/other/path", targetPath)

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return filepath.Join(home, ".test", "skills")
			}
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Skills, Name: "my-skill", Item: catalog.ContentItem{
			Name: "my-skill", Type: catalog.Skills, Path: "/repo/content/skills/my-skill",
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, homeDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "error-conflict" {
		t.Errorf("expected error-conflict, got %s", actions[0].Action)
	}
	if actions[0].Problem == "" {
		t.Error("expected non-empty Problem for conflict")
	}
}

func TestPreview_HookAlreadyInstalled(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()

	// Write installed.json with an existing hook
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)
	inst := &installer.Installed{
		Hooks: []installer.InstalledHook{
			{Name: "my-hook", Event: "PostToolUse", Command: "echo test", Source: "export"},
		},
	}
	if err := installer.SaveInstalled(repoRoot, inst); err != nil {
		t.Fatalf("failed to save installed.json: %v", err)
	}

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Hooks, Name: "my-hook", Item: catalog.ContentItem{
			Name: "my-hook", Type: catalog.Hooks,
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "skip-exists" {
		t.Errorf("expected skip-exists for already-installed hook, got %s", actions[0].Action)
	}
}

func TestPreview_NewHook(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Hooks, Name: "new-hook", Item: catalog.ContentItem{
			Name: "new-hook", Type: catalog.Hooks,
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "merge-hook" {
		t.Errorf("expected merge-hook for new hook, got %s", actions[0].Action)
	}
}

func TestPreview_NewMCP(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.MCP, Name: "new-server", Item: catalog.ContentItem{
			Name: "new-server", Type: catalog.MCP,
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "merge-mcp" {
		t.Errorf("expected merge-mcp for new MCP, got %s", actions[0].Action)
	}
}

func TestPreview_MCPAlreadyInstalled(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	// Write installed.json with an existing MCP entry
	inst := &installer.Installed{
		MCP: []installer.InstalledMCP{
			{Name: "existing-server", Source: "export"},
		},
	}
	installer.SaveInstalled(repoRoot, inst)

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.MCP, Name: "existing-server", Item: catalog.ContentItem{
			Name: "existing-server", Type: catalog.MCP,
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "skip-exists" {
		t.Errorf("expected skip-exists for already-installed MCP, got %s", actions[0].Action)
	}
}

func TestPreview_RegularFileConflict(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	rulesDir := filepath.Join(homeDir, ".test", "rules")
	os.MkdirAll(rulesDir, 0755)

	// Create a regular file at the target path
	os.WriteFile(filepath.Join(rulesDir, "my-rule"), []byte("content"), 0644)

	prov := provider.Provider{
		Name: "test-provider",
		Slug: "test",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(home, ".test", "rules")
			}
			return ""
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Rules, Name: "my-rule", Item: catalog.ContentItem{
			Name: "my-rule", Type: catalog.Rules, Path: "/repo/content/rules/test/my-rule",
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, homeDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Action != "error-conflict" {
		t.Errorf("expected error-conflict for regular file, got %s", actions[0].Action)
	}
}

// TestPreview_HookUnsupportedEvent: a hook whose event the target provider
// has no settings key for is planned as "skip-unsupported" instead of
// "merge-hook", so Apply can gate on it (syllago-xqlc1). before_tool_execute
// has no windsurf mapping — merging it would write dead config windsurf
// never reads.
func TestPreview_HookUnsupportedEvent(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	hookDir := filepath.Join(repoRoot, "hooks", "dead-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","handler":{"type":"command","command":"echo hi"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	prov := provider.Provider{
		Name: "Windsurf",
		Slug: "windsurf",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return "__json_merge__"
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Hooks, Name: "dead-hook", Item: catalog.ContentItem{
			Name: "dead-hook", Type: catalog.Hooks, Path: hookDir,
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "skip-unsupported" {
		t.Errorf("expected skip-unsupported, got %s", actions[0].Action)
	}
	if !strings.Contains(actions[0].Problem, "before_tool_execute") {
		t.Errorf("problem should name the event, got %q", actions[0].Problem)
	}
}

// TestPreview_HookInvalidEvent: a hook with an unknown/malformed event name is
// NOT plannable as skip-unsupported — it must stay a merge-hook action so
// applyHook raises the hard injection-guard error, which fires even under
// --skip-unsupported. Only real-but-unmapped events are skippable
// (syllago-xqlc1, codex review finding).
func TestPreview_HookInvalidEvent(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	hookDir := filepath.Join(repoRoot, "hooks", "bad-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"not_a_real_event","handler":{"type":"command","command":"echo hi"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	prov := provider.Provider{
		Name: "Windsurf",
		Slug: "windsurf",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return "__json_merge__"
		},
	}

	refs := []ResolvedRef{
		{Type: catalog.Hooks, Name: "bad-hook", Item: catalog.ContentItem{
			Name: "bad-hook", Type: catalog.Hooks, Path: hookDir,
		}},
	}

	actions, err := Preview(refs, prov, repoRoot, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Action != "merge-hook" {
		t.Errorf("invalid event should stay merge-hook (so applyHook errors), got %s", actions[0].Action)
	}
}
