package installer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadInstalled_MissingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	inst, err := LoadInstalled(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst == nil {
		t.Fatal("expected non-nil Installed")
	}
	if len(inst.Hooks) != 0 || len(inst.MCP) != 0 || len(inst.Symlinks) != 0 {
		t.Error("expected empty Installed struct")
	}
}

func TestLoadInstalled_MalformedJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	syllagoDir := filepath.Join(tmpDir, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "installed.json"), []byte("not json"), 0644)

	_, err := LoadInstalled(tmpDir)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSaveInstalled_RoundTrip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	now := time.Now().Truncate(time.Second)
	original := &Installed{
		Hooks: []InstalledHook{
			{Name: "lint-hook", Event: "PreToolUse", Command: "./lint.sh", Source: "export", InstalledAt: now},
		},
		MCP: []InstalledMCP{
			{Name: "test-server", Source: "export", InstalledAt: now},
		},
		Symlinks: []InstalledSymlink{
			{Path: "/home/user/.claude/rules/my-rule.md", Target: "/repo/content/rules/my-rule/my-rule.md", Source: "export", InstalledAt: now},
		},
	}

	if err := SaveInstalled(tmpDir, original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadInstalled(tmpDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded.Hooks) != 1 || loaded.Hooks[0].Name != "lint-hook" {
		t.Errorf("hooks mismatch: got %+v", loaded.Hooks)
	}
	if len(loaded.MCP) != 1 || loaded.MCP[0].Name != "test-server" {
		t.Errorf("mcp mismatch: got %+v", loaded.MCP)
	}
	if len(loaded.Symlinks) != 1 || loaded.Symlinks[0].Path != "/home/user/.claude/rules/my-rule.md" {
		t.Errorf("symlinks mismatch: got %+v", loaded.Symlinks)
	}
}

func TestFindHook(t *testing.T) {
	t.Parallel()
	inst := &Installed{
		Hooks: []InstalledHook{
			{Name: "hook-a", Event: "PreToolUse"},
			{Name: "hook-b", Event: "PostToolUse"},
		},
	}

	if idx := inst.FindHook("hook-a", "PreToolUse"); idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}
	if idx := inst.FindHook("hook-b", "PostToolUse"); idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
	if idx := inst.FindHook("hook-c", "PreToolUse"); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestFindMCP(t *testing.T) {
	t.Parallel()
	inst := &Installed{
		MCP: []InstalledMCP{
			{Name: "server-a"},
			{Name: "server-b"},
		},
	}

	if idx := inst.FindMCP("server-a"); idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}
	if idx := inst.FindMCP("nonexistent"); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestInstalled_FindAndRemoveRuleAppend(t *testing.T) {
	t.Parallel()
	inst := &Installed{
		RuleAppends: []InstalledRuleAppend{
			{LibraryID: "id-a", TargetFile: "/p/CLAUDE.md"},
			{LibraryID: "id-b", TargetFile: "/p/CLAUDE.md"},
			{LibraryID: "id-a", TargetFile: "/p/AGENTS.md"},
		},
	}

	if idx := inst.FindRuleAppend("id-a", "/p/CLAUDE.md"); idx != 0 {
		t.Errorf("expected 0, got %d", idx)
	}
	if idx := inst.FindRuleAppend("id-b", "/p/CLAUDE.md"); idx != 1 {
		t.Errorf("expected 1, got %d", idx)
	}
	if idx := inst.FindRuleAppend("id-a", "/p/AGENTS.md"); idx != 2 {
		t.Errorf("expected 2, got %d", idx)
	}
	if idx := inst.FindRuleAppend("missing", "/p/CLAUDE.md"); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}

	inst.RemoveRuleAppend(1)
	if len(inst.RuleAppends) != 2 {
		t.Fatalf("expected 2 after remove, got %d", len(inst.RuleAppends))
	}
	if inst.RuleAppends[0].LibraryID != "id-a" || inst.RuleAppends[0].TargetFile != "/p/CLAUDE.md" {
		t.Errorf("spliced entry 0 wrong: %+v", inst.RuleAppends[0])
	}
	if inst.RuleAppends[1].LibraryID != "id-a" || inst.RuleAppends[1].TargetFile != "/p/AGENTS.md" {
		t.Errorf("spliced entry 1 wrong: %+v", inst.RuleAppends[1])
	}
}

func TestInstalled_RuleAppendsUniqueByLibraryIDAndTargetFile(t *testing.T) {
	t.Parallel()
	// Sanity guard: for any Installed, no two RuleAppends may share the
	// (LibraryID, TargetFile) pair per D14. This test exercises the
	// invariant against well-formed fixtures — it will fail only if a
	// future writer path produces duplicates.
	inst := &Installed{
		RuleAppends: []InstalledRuleAppend{
			{LibraryID: "id-a", TargetFile: "/p/CLAUDE.md"},
			{LibraryID: "id-b", TargetFile: "/p/CLAUDE.md"},
			{LibraryID: "id-a", TargetFile: "/p/AGENTS.md"},
		},
	}
	seen := make(map[[2]string]bool)
	for _, r := range inst.RuleAppends {
		key := [2]string{r.LibraryID, r.TargetFile}
		if seen[key] {
			t.Errorf("duplicate rule append for (LibraryID=%q, TargetFile=%q)", r.LibraryID, r.TargetFile)
		}
		seen[key] = true
	}
}

func TestInstalled_RuleAppendsJSONRoundtrip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	now := time.Now().UTC().Truncate(time.Second)
	original := &Installed{
		RuleAppends: []InstalledRuleAppend{
			{
				Name:        "my-rule",
				LibraryID:   "lib-id-123",
				Provider:    "claude-code",
				TargetFile:  "/project/CLAUDE.md",
				VersionHash: "sha256:abcdef",
				Source:      "manual",
				Scope:       "project",
				InstalledAt: now,
			},
		},
	}

	if err := SaveInstalled(tmpDir, original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadInstalled(tmpDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded.RuleAppends) != 1 {
		t.Fatalf("expected 1 rule append, got %d", len(loaded.RuleAppends))
	}
	got := loaded.RuleAppends[0]
	want := original.RuleAppends[0]
	if got.Name != want.Name ||
		got.LibraryID != want.LibraryID ||
		got.Provider != want.Provider ||
		got.TargetFile != want.TargetFile ||
		got.VersionHash != want.VersionHash ||
		got.Source != want.Source ||
		got.Scope != want.Scope ||
		!got.InstalledAt.Equal(want.InstalledAt) {
		t.Errorf("rule append mismatch:\n got %+v\nwant %+v", got, want)
	}
}
