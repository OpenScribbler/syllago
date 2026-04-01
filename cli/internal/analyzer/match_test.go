package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// stubDetector implements ContentDetector for testing.
type stubDetector struct {
	slug     string
	patterns []DetectionPattern
}

func (s *stubDetector) ProviderSlug() string                             { return s.slug }
func (s *stubDetector) Patterns() []DetectionPattern                     { return s.patterns }
func (s *stubDetector) Classify(string, string) ([]*DetectedItem, error) { return nil, nil }

func TestMatchPatterns(t *testing.T) {
	t.Parallel()

	ccDetector := &stubDetector{
		slug: "claude-code",
		patterns: []DetectionPattern{
			{Glob: "skills/*/SKILL.md", ContentType: catalog.Skills, Confidence: 0.95},
			{Glob: ".claude/agents/*.md", ContentType: catalog.Agents, Confidence: 0.90},
		},
	}

	cursorDetector := &stubDetector{
		slug: "cursor",
		patterns: []DetectionPattern{
			{Glob: ".cursorrules", ContentType: catalog.Rules, Confidence: 0.85},
			{Glob: "hooks/*/*/hook.json", ContentType: catalog.Hooks, Confidence: 0.80},
		},
	}

	tests := []struct {
		name    string
		paths   []string
		want    int    // expected match count
		wantAny string // at least one match should have this path (empty = skip check)
	}{
		{
			name:    "SKILL.md matches skills/*/SKILL.md",
			paths:   []string{"skills/my-skill/SKILL.md"},
			want:    1,
			wantAny: "skills/my-skill/SKILL.md",
		},
		{
			name:    ".cursorrules matches cursor pattern",
			paths:   []string{".cursorrules"},
			want:    1,
			wantAny: ".cursorrules",
		},
		{
			name:    "hook.json matches hooks/*/*/hook.json",
			paths:   []string{"hooks/claude-code/my-hook/hook.json"},
			want:    1,
			wantAny: "hooks/claude-code/my-hook/hook.json",
		},
		{
			name:    ".claude/agents/*.md matches agent pattern",
			paths:   []string{".claude/agents/reviewer.md"},
			want:    1,
			wantAny: ".claude/agents/reviewer.md",
		},
		{
			name:  "no false positive for unrelated path",
			paths: []string{"node_modules/foo.md"},
			want:  0,
		},
		{
			name:  "multiple detectors match same path independently",
			paths: []string{"skills/my-skill/SKILL.md", ".cursorrules"},
			want:  2, // one from each detector
		},
	}

	detectors := []ContentDetector{ccDetector, cursorDetector}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matches := MatchPatterns(tt.paths, detectors)
			if len(matches) != tt.want {
				t.Errorf("got %d matches, want %d; matches: %v", len(matches), tt.want, matches)
			}
			if tt.wantAny != "" {
				found := false
				for _, m := range matches {
					if m.Path == tt.wantAny {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected a match with path %q, got none", tt.wantAny)
				}
			}
		})
	}
}
