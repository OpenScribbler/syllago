package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestCursorDetector_ProviderSlug(t *testing.T) {
	t.Parallel()
	d := &CursorDetector{}
	if got := d.ProviderSlug(); got != "cursor" {
		t.Errorf("ProviderSlug() = %q, want %q", got, "cursor")
	}
}

func TestCursorDetector_Patterns(t *testing.T) {
	t.Parallel()
	d := &CursorDetector{}
	pats := d.Patterns()
	if len(pats) != 7 {
		t.Fatalf("Patterns() returned %d, want 7", len(pats))
	}
}

func TestCursorDetector_Classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		relPath   string
		content   string
		wantType  catalog.ContentType
		wantNil   bool
		wantLabel string
	}{
		{
			name:     ".cursorrules",
			relPath:  ".cursorrules",
			content:  "# Cursor Rules\nBe concise.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "cursor rules mdc",
			relPath:  ".cursor/rules/style.mdc",
			content:  "# Style Guide\nFormat consistently.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "cursor rules md",
			relPath:  ".cursor/rules/coding.md",
			content:  "# Coding Rules\nTest everything.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "cursor agents",
			relPath:  ".cursor/agents/reviewer.md",
			content:  "---\nname: Reviewer\ndescription: Code review agent\n---\nBody.\n",
			wantType: catalog.Agents,
		},
		{
			name:     "cursor skills",
			relPath:  ".cursor/skills/my-skill/SKILL.md",
			content:  "---\nname: My Skill\ndescription: Does things\n---\nBody.\n",
			wantType: catalog.Skills,
		},
		{
			name:    "cursor hooks.json returns nil (wiring)",
			relPath: ".cursor/hooks.json",
			content: `{"PreToolUse": []}`,
			wantNil: true,
		},
		{
			name:      "cursor hook script",
			relPath:   ".cursor/hooks/lint.sh",
			content:   "#!/bin/sh\neslint .\n",
			wantType:  catalog.Hooks,
			wantLabel: "hook-script",
		},
		{
			name:    "empty file returns nil",
			relPath: ".cursorrules",
			content: "",
			wantNil: true,
		},
	}

	d := &CursorDetector{}

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
			if items[0].Provider != "cursor" {
				t.Errorf("Provider = %q, want %q", items[0].Provider, "cursor")
			}
			if tt.wantLabel != "" && items[0].InternalLabel != tt.wantLabel {
				t.Errorf("InternalLabel = %q, want %q", items[0].InternalLabel, tt.wantLabel)
			}
		})
	}
}
