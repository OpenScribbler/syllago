package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestClaudeCodePluginDetector_ProviderSlug(t *testing.T) {
	t.Parallel()
	d := &ClaudeCodePluginDetector{}
	if got := d.ProviderSlug(); got != "claude-code-plugin" {
		t.Errorf("ProviderSlug() = %q, want %q", got, "claude-code-plugin")
	}
}

func TestClaudeCodePluginDetector_Patterns(t *testing.T) {
	t.Parallel()
	d := &ClaudeCodePluginDetector{}
	pats := d.Patterns()
	if len(pats) != 5 {
		t.Fatalf("Patterns() returned %d, want 5", len(pats))
	}
}

func TestClaudeCodePluginDetector_Classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		relPath  string
		content  string
		wantType catalog.ContentType
		wantNil  bool
	}{
		{
			name:    "plugin.json returns nil",
			relPath: ".claude-plugin/plugin.json",
			content: `{"name":"my-plugin"}`,
			wantNil: true,
		},
		{
			name:    "hooks.json returns nil (wiring)",
			relPath: "plugins/my-plugin/hooks/hooks.json",
			content: `{"PreToolUse":[]}`,
			wantNil: true,
		},
		{
			name:     "plugin agent",
			relPath:  "plugins/my-plugin/agents/reviewer.md",
			content:  "---\nname: Plugin Reviewer\ndescription: Reviews code\n---\nBody.\n",
			wantType: catalog.Agents,
		},
		{
			name:     "plugin skill",
			relPath:  "plugins/my-plugin/skills/my-skill/SKILL.md",
			content:  "---\nname: Plugin Skill\ndescription: Does things\n---\nBody.\n",
			wantType: catalog.Skills,
		},
		{
			name:     "plugin command",
			relPath:  "plugins/my-plugin/commands/deploy.md",
			content:  "# Deploy\nDeploys to production.\n",
			wantType: catalog.Commands,
		},
		{
			name:    "empty file returns nil",
			relPath: "plugins/my-plugin/agents/empty.md",
			content: "",
			wantNil: true,
		},
		{
			name:    "missing file returns nil",
			relPath: "plugins/my-plugin/agents/ghost.md",
			wantNil: true,
		},
	}

	d := &ClaudeCodePluginDetector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if tt.content != "" || tt.name == "empty file returns nil" {
				setupFile(t, root, tt.relPath, tt.content)
			}

			items, err := d.Classify(tt.relPath, root)
			if err != nil {
				t.Fatalf("Classify error: %v", err)
			}
			if tt.wantNil {
				if items != nil {
					t.Errorf("expected nil, got %d items", len(items))
				}
				return
			}
			if len(items) != 1 {
				t.Fatalf("expected 1 item, got %d", len(items))
			}
			if items[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", items[0].Type, tt.wantType)
			}
			if items[0].Provider != "claude-code-plugin" {
				t.Errorf("Provider = %q, want %q", items[0].Provider, "claude-code-plugin")
			}
		})
	}
}
