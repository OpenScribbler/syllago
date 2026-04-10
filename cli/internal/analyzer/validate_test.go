package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestValidateManifest_Valid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/my-skill/SKILL.md", "# My Skill\n")

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "my-skill", Type: "skills", Path: "skills/my-skill/SKILL.md"},
		},
	}

	issues := ValidateManifest(m, root)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d: %v", len(issues), issues)
	}
}

func TestValidateManifest_MissingPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "ghost", Type: "skills", Path: "skills/ghost/SKILL.md"},
		},
	}

	issues := ValidateManifest(m, root)
	foundError := false
	for _, i := range issues {
		if i.Severity == "error" && i.ItemName == "ghost" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected error issue for missing path")
	}
}

func TestValidateManifest_HookWithMdExtension(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "hooks/bad-hook.md", "# Not a hook\n")

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "bad-hook", Type: "hooks", Path: "hooks/bad-hook.md"},
		},
	}

	issues := ValidateManifest(m, root)
	foundWarning := false
	for _, i := range issues {
		if i.Severity == "warning" && i.ItemName == "bad-hook" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning for hook with .md extension")
	}
}

func TestValidateManifest_RuleWithMd(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "rules/my-rule.md", "# Rule\n")

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "my-rule", Type: "rules", Path: "rules/my-rule.md"},
		},
	}

	issues := ValidateManifest(m, root)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for rule with .md, got %d", len(issues))
	}
}

func TestValidateManifest_MCPWithYaml(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "mcp/server.yaml", "command: node\n")

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "server", Type: "mcp", Path: "mcp/server.yaml"},
		},
	}

	issues := ValidateManifest(m, root)
	foundWarning := false
	for _, i := range issues {
		if i.Severity == "warning" && i.ItemName == "server" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning for MCP with .yaml extension")
	}
}

func TestValidateManifest_Nil(t *testing.T) {
	t.Parallel()
	issues := ValidateManifest(nil, "/tmp")
	if issues != nil {
		t.Errorf("expected nil issues for nil manifest, got %d", len(issues))
	}
}

func TestIsWithinRoot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		resolved string
		root     string
		want     bool
	}{
		{"/tmp/repo", "/tmp/repo", true},
		{"/tmp/repo/subdir/file.md", "/tmp/repo", true},
		{"/tmp/repomalicious", "/tmp/repo", false},
		{"/tmp/other", "/tmp/repo", false},
		{"/tmp", "/tmp/repo", false},
		{"/tmp/repo_evil", "/tmp/repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.resolved, func(t *testing.T) {
			if got := isWithinRoot(tt.resolved, tt.root); got != tt.want {
				t.Errorf("isWithinRoot(%q, %q) = %v, want %v", tt.resolved, tt.root, got, tt.want)
			}
		})
	}
}
