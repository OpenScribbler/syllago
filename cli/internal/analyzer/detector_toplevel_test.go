package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestTopLevelDetector_ProviderSlug(t *testing.T) {
	t.Parallel()
	d := &TopLevelDetector{}
	if got := d.ProviderSlug(); got != "top-level" {
		t.Errorf("ProviderSlug() = %q, want %q", got, "top-level")
	}
}

func TestTopLevelDetector_Patterns(t *testing.T) {
	t.Parallel()
	d := &TopLevelDetector{}
	pats := d.Patterns()
	if len(pats) != 13 {
		t.Fatalf("Patterns() returned %d, want 13", len(pats))
	}
}

func TestTopLevelDetector_Classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		relPath       string
		content       string
		wantType      catalog.ContentType
		wantNil       bool
		wantLabel     string
		wantConfident float64
	}{
		{
			name:          "agents markdown",
			relPath:       "agents/reviewer.md",
			content:       "---\nname: Reviewer\ndescription: Reviews code\n---\nBody.\n",
			wantType:      catalog.Agents,
			wantConfident: 0.85,
		},
		{
			name:          "hook script ts",
			relPath:       "hooks/lint.ts",
			content:       "console.log('lint hook')\n",
			wantType:      catalog.Hooks,
			wantLabel:     "hook-script",
			wantConfident: 0.60,
		},
		{
			name:    "hook wiring returns nil",
			relPath: "hooks/hooks.json",
			content: `{"PreToolUse": []}`,
			wantNil: true,
		},
		{
			name:    "empty file returns nil",
			relPath: "agents/empty.md",
			content: "",
			wantNil: true,
		},
		{
			name:          "prompts mapped to rules",
			relPath:       "prompts/style.md",
			content:       "# Style\nBe concise.\n",
			wantType:      catalog.Rules,
			wantConfident: 0.75,
		},
		{
			name:          "rules mdc file",
			relPath:       "rules/coding.mdc",
			content:       "# Coding rules\nFollow conventions.\n",
			wantType:      catalog.Rules,
			wantConfident: 0.80,
		},
	}

	d := &TopLevelDetector{}

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
			if tt.wantLabel != "" && items[0].InternalLabel != tt.wantLabel {
				t.Errorf("InternalLabel = %q, want %q", items[0].InternalLabel, tt.wantLabel)
			}
			if tt.wantConfident != 0 && items[0].Confidence != tt.wantConfident {
				t.Errorf("Confidence = %v, want %v", items[0].Confidence, tt.wantConfident)
			}
		})
	}
}
