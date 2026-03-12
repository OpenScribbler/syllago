package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanProviderDirectoryFormat(t *testing.T) {
	t.Parallel()
	// Create a temp repo with directory-format hooks
	tmp := t.TempDir()

	// hooks/claude-code/my-hook/hook.json
	hookDir := filepath.Join(tmp, "hooks", "claude-code", "my-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{"hooks":{"PostToolUse":[{"matcher":"Write","command":"echo hi"}]}}`), 0644)
	os.WriteFile(filepath.Join(hookDir, ".syllago.yaml"), []byte("name: my-hook\ndescription: Runs a linter after file edits\nversion: \"1.0\"\n"), 0644)

	// rules/claude-code/my-rule/rule.md
	ruleDir := filepath.Join(tmp, "rules", "claude-code", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# My Rule\n\nKeep it short.\n"), 0644)

	// commands/claude-code/my-cmd/command.md
	cmdDir := filepath.Join(tmp, "commands", "claude-code", "my-cmd")
	os.MkdirAll(cmdDir, 0755)
	os.WriteFile(filepath.Join(cmdDir, "command.md"), []byte("# My Command\n\nDo something.\n"), 0644)

	// Also need a skills/ dir for findRepoRoot
	os.MkdirAll(filepath.Join(tmp, "skills"), 0755)

	cat, err := Scan(tmp, tmp)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	counts := cat.CountByType()
	if counts[Hooks] != 1 {
		t.Errorf("expected 1 hook, got %d", counts[Hooks])
	}
	if counts[Rules] != 1 {
		t.Errorf("expected 1 rule, got %d", counts[Rules])
	}
	if counts[Commands] != 1 {
		t.Errorf("expected 1 command, got %d", counts[Commands])
	}

	// Check hook details
	hooks := cat.ByType(Hooks)
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook item, got %d", len(hooks))
	}
	h := hooks[0]
	if h.Name != "my-hook" {
		t.Errorf("hook name = %q, want %q", h.Name, "my-hook")
	}
	if h.Provider != "claude-code" {
		t.Errorf("hook provider = %q, want %q", h.Provider, "claude-code")
	}
	if len(h.Files) == 0 {
		t.Error("hook Files is empty, expected file listing")
	}
	if h.Path != hookDir {
		t.Errorf("hook path = %q, want %q", h.Path, hookDir)
	}
	if h.Description != "Runs a linter after file edits" {
		t.Errorf("hook description = %q, want %q", h.Description, "Runs a linter after file edits")
	}

	// Check rule details
	rules := cat.ByType(Rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule item, got %d", len(rules))
	}
	r := rules[0]
	if r.Name != "my-rule" {
		t.Errorf("rule name = %q, want %q", r.Name, "my-rule")
	}
	if r.Description != "Keep it short." {
		t.Errorf("rule description = %q, want %q", r.Description, "Keep it short.")
	}

	// Check command details
	cmds := cat.ByType(Commands)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command item, got %d", len(cmds))
	}
	c := cmds[0]
	if c.Name != "my-cmd" {
		t.Errorf("command name = %q, want %q", c.Name, "my-cmd")
	}
}

func TestScanProviderLegacyAndDirectoryMixed(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Legacy single-file rule
	provDir := filepath.Join(tmp, "rules", "claude-code")
	os.MkdirAll(provDir, 0755)
	os.WriteFile(filepath.Join(provDir, "old-rule.md"), []byte("# Old Rule\n\nLegacy format.\n"), 0644)

	// New directory-format rule
	newDir := filepath.Join(provDir, "new-rule")
	os.MkdirAll(newDir, 0755)
	os.WriteFile(filepath.Join(newDir, "rule.md"), []byte("# New Rule\n\nDirectory format.\n"), 0644)

	os.MkdirAll(filepath.Join(tmp, "skills"), 0755)

	cat, err := Scan(tmp, tmp)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	rules := cat.ByType(Rules)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules (1 legacy + 1 directory), got %d", len(rules))
	}

	names := map[string]bool{}
	for _, r := range rules {
		names[r.Name] = true
	}
	if !names["old-rule.md"] {
		t.Error("missing legacy rule 'old-rule.md'")
	}
	if !names["new-rule"] {
		t.Error("missing directory rule 'new-rule'")
	}
}
