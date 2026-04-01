package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func setupFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	os.MkdirAll(filepath.Dir(abs), 0755)
	os.WriteFile(abs, []byte(content), 0644)
}

// TestSimpleDetectors_Slugs verifies ProviderSlug() for all simple detectors.
func TestSimpleDetectors_Slugs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		det  ContentDetector
		slug string
	}{
		{&WindsurfDetector{}, "windsurf"},
		{&ClineDetector{}, "cline"},
		{&RooCodeDetector{}, "roo-code"},
		{&CodexDetector{}, "codex"},
		{&GeminiDetector{}, "gemini-cli"},
	}
	for _, tt := range tests {
		if got := tt.det.ProviderSlug(); got != tt.slug {
			t.Errorf("%T.ProviderSlug() = %q, want %q", tt.det, got, tt.slug)
		}
	}
}

// TestSimpleDetectors_PatternCounts verifies each detector returns the expected number of patterns.
func TestSimpleDetectors_PatternCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		det   ContentDetector
		count int
	}{
		{"windsurf", &WindsurfDetector{}, 1},
		{"cline", &ClineDetector{}, 2},
		{"roo-code", &RooCodeDetector{}, 2},
		{"codex", &CodexDetector{}, 2},
		{"gemini-cli", &GeminiDetector{}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pats := tt.det.Patterns()
			if len(pats) != tt.count {
				t.Errorf("Patterns() returned %d, want %d", len(pats), tt.count)
			}
		})
	}
}

// TestSimpleDetectors_Classify verifies Classify returns items for valid content.
func TestSimpleDetectors_Classify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		det      ContentDetector
		relPath  string
		content  string
		wantType catalog.ContentType
	}{
		{
			name:     "windsurf rules",
			det:      &WindsurfDetector{},
			relPath:  ".windsurfrules",
			content:  "# Windsurf rules\nBe concise.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "cline rules",
			det:      &ClineDetector{},
			relPath:  ".clinerules",
			content:  "# Cline rules\nFollow best practices.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "cline directory rule",
			det:      &ClineDetector{},
			relPath:  ".clinerules/style.md",
			content:  "# Style Guide\nUse tabs.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "roo-code rules",
			det:      &RooCodeDetector{},
			relPath:  ".roo/rules/coding.md",
			content:  "# Coding Rules\nTest everything.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "roo-code roomodes",
			det:      &RooCodeDetector{},
			relPath:  ".roomodes",
			content:  "# Roo Modes\nconfiguration here\n",
			wantType: catalog.Rules,
		},
		{
			name:     "codex agents.md",
			det:      &CodexDetector{},
			relPath:  "AGENTS.md",
			content:  "# Agents\nAgent-like content with headers.\n",
			wantType: catalog.Rules, // default classification without agent headers
		},
		{
			name:     "codex agents toml",
			det:      &CodexDetector{},
			relPath:  ".codex/agents/reviewer.toml",
			content:  "[agent]\nname = \"Reviewer\"\n",
			wantType: catalog.Agents,
		},
		{
			name:     "gemini rules",
			det:      &GeminiDetector{},
			relPath:  "GEMINI.md",
			content:  "# Gemini Instructions\nProject-level rules.\n",
			wantType: catalog.Rules,
		},
		{
			name:     "gemini skills",
			det:      &GeminiDetector{},
			relPath:  ".gemini/skills/my-skill/SKILL.md",
			content:  "---\nname: My Skill\ndescription: Does things\n---\nBody.\n",
			wantType: catalog.Skills,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			setupFile(t, root, tt.relPath, tt.content)

			items, err := tt.det.Classify(tt.relPath, root)
			if err != nil {
				t.Fatalf("Classify error: %v", err)
			}
			if items == nil || len(items) == 0 {
				t.Fatal("expected non-nil items, got nil")
			}
			if items[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", items[0].Type, tt.wantType)
			}
		})
	}
}

// TestSimpleDetectors_EmptyFile verifies Classify returns nil for empty files.
func TestSimpleDetectors_EmptyFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		det     ContentDetector
		relPath string
	}{
		{"windsurf", &WindsurfDetector{}, ".windsurfrules"},
		{"cline", &ClineDetector{}, ".clinerules"},
		{"roo-code", &RooCodeDetector{}, ".roo/rules/empty.md"},
		{"codex", &CodexDetector{}, "AGENTS.md"},
		{"gemini", &GeminiDetector{}, "GEMINI.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			setupFile(t, root, tt.relPath, "") // empty file

			items, err := tt.det.Classify(tt.relPath, root)
			if err != nil {
				t.Fatalf("Classify error: %v", err)
			}
			if items != nil {
				t.Errorf("expected nil items for empty file, got %d", len(items))
			}
		})
	}
}

// TestSimpleDetectors_MissingFile verifies Classify returns nil for missing files.
func TestSimpleDetectors_MissingFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		det     ContentDetector
		relPath string
	}{
		{"windsurf", &WindsurfDetector{}, ".windsurfrules"},
		{"cline", &ClineDetector{}, ".clinerules"},
		{"roo-code", &RooCodeDetector{}, ".roo/rules/ghost.md"},
		{"codex", &CodexDetector{}, "AGENTS.md"},
		{"gemini", &GeminiDetector{}, "GEMINI.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir() // no files created

			items, err := tt.det.Classify(tt.relPath, root)
			if err != nil {
				t.Fatalf("Classify error: %v", err)
			}
			if items != nil {
				t.Errorf("expected nil items for missing file, got %d", len(items))
			}
		})
	}
}
