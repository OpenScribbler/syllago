package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestCopilotDetector_ProviderSlug(t *testing.T) {
	t.Parallel()
	d := &CopilotDetector{}
	if got := d.ProviderSlug(); got != "vs-code-copilot" {
		t.Errorf("ProviderSlug() = %q, want %q", got, "vs-code-copilot")
	}
}

func TestCopilotDetector_Patterns(t *testing.T) {
	t.Parallel()
	d := &CopilotDetector{}
	pats := d.Patterns()
	if len(pats) != 3 {
		t.Fatalf("Patterns() returned %d, want 3", len(pats))
	}

	expected := []struct {
		glob       string
		ct         catalog.ContentType
		confidence float64
	}{
		{".github/copilot-instructions.md", catalog.Rules, 0.95},
		{".github/instructions/*.instructions.md", catalog.Rules, 0.90},
		{".github/agents/*.md", catalog.Agents, 0.90},
	}

	for i, exp := range expected {
		if pats[i].Glob != exp.glob {
			t.Errorf("Patterns()[%d].Glob = %q, want %q", i, pats[i].Glob, exp.glob)
		}
		if pats[i].ContentType != exp.ct {
			t.Errorf("Patterns()[%d].ContentType = %q, want %q", i, pats[i].ContentType, exp.ct)
		}
		if pats[i].Confidence != exp.confidence {
			t.Errorf("Patterns()[%d].Confidence = %v, want %v", i, pats[i].Confidence, exp.confidence)
		}
	}
}

func TestCopilotDetector_Classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		relPath         string
		content         string
		wantType        catalog.ContentType
		wantNil         bool
		wantDisplayName string
	}{
		{
			name:     "copilot-instructions.md",
			relPath:  ".github/copilot-instructions.md",
			content:  "# Copilot Instructions\nBe helpful.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "instructions file",
			relPath:  ".github/instructions/style.instructions.md",
			content:  "# Style Guide\nUse consistent formatting.\n",
			wantType: catalog.Rules,
		},
		{
			name:            "agents file with frontmatter",
			relPath:         ".github/agents/reviewer.md",
			content:         "---\nname: Code Reviewer\ndescription: Reviews pull requests\n---\nAgent body.\n",
			wantType:        catalog.Agents,
			wantDisplayName: "Code Reviewer",
		},
		{
			name:    "empty file returns nil",
			relPath: ".github/copilot-instructions.md",
			content: "",
			wantNil: true,
		},
		{
			name:    "missing file returns nil",
			relPath: ".github/copilot-instructions.md",
			wantNil: true, // no file created
		},
	}

	d := &CopilotDetector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			if tt.content != "" || (tt.name == "empty file returns nil") {
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
			if items[0].Provider != "vs-code-copilot" {
				t.Errorf("Provider = %q, want %q", items[0].Provider, "vs-code-copilot")
			}
			if tt.wantDisplayName != "" && items[0].DisplayName != tt.wantDisplayName {
				t.Errorf("DisplayName = %q, want %q", items[0].DisplayName, tt.wantDisplayName)
			}
		})
	}
}
