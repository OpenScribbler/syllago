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

	nescoDir := filepath.Join(tmpDir, ".nesco")
	os.MkdirAll(nescoDir, 0755)
	os.WriteFile(filepath.Join(nescoDir, "installed.json"), []byte("not json"), 0644)

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
