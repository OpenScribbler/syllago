package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Round-trip tests verify that canonicalize(render(canonicalize(input))) preserves
// core fields. They catch unintended data loss when converters are modified.
//
// The flow for each provider:
//   1. Load kitchen-sink example
//   2. Canonicalize (pass 1) -> parse -> meta1, body1
//   3. Render to target provider
//   4. Canonicalize again (pass 2) -> parse -> meta2, body2
//   5. Assert core fields survived

// skillRoundTripTarget defines a provider to test for skill round-trips.
type skillRoundTripTarget struct {
	name          string
	provider      provider.Provider
	sourceSlug    string // provider slug used when re-canonicalizing the rendered output
	lossyName     bool   // true if name is expected to be lost
	lossyDesc     bool   // true if description is expected to be lost
	bodySubstring string // required substring in the round-tripped body
}

func TestKitchenSinkSkillRoundTrip(t *testing.T) {
	root := repoRoot(t)
	skillPath := filepath.Join(root, "content", "skills", "example-kitchen-sink-skill", "SKILL.md")

	original, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("reading kitchen-sink skill: %v", err)
	}

	conv := &SkillsConverter{}

	// Pass 1: canonicalize the original
	canonical1, err := conv.Canonicalize(original, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize pass 1: %v", err)
	}

	meta1, body1, err := parseSkillCanonical(canonical1.Content)
	if err != nil {
		t.Fatalf("parsing canonical1: %v", err)
	}
	if meta1.Name == "" {
		t.Fatal("pass 1: Name is empty")
	}
	if body1 == "" {
		t.Fatal("pass 1: body is empty")
	}

	// Providers to round-trip through.
	//
	// Claude Code (default renderer) preserves all frontmatter, so name and
	// description survive. Gemini CLI keeps name+description in frontmatter
	// but drops other fields. OpenCode and Kiro render to plain markdown
	// (no frontmatter), so structured meta is lost — name and description
	// are embedded as prose headings, not YAML fields.
	targets := []skillRoundTripTarget{
		{
			name:          "claude-code",
			provider:      provider.ClaudeCode,
			sourceSlug:    "claude-code",
			bodySubstring: "Kitchen Sink Skill",
		},
		{
			name:          "gemini-cli",
			provider:      provider.GeminiCLI,
			sourceSlug:    "gemini-cli",
			bodySubstring: "Kitchen Sink Skill",
		},
		{
			name:          "opencode",
			provider:      provider.OpenCode,
			sourceSlug:    "opencode",
			lossyName:     true, // rendered as # heading, not YAML field
			lossyDesc:     true, // rendered as prose paragraph, not YAML field
			bodySubstring: "Kitchen Sink Skill",
		},
		{
			name:          "kiro",
			provider:      provider.Kiro,
			sourceSlug:    "kiro",
			lossyName:     true,
			lossyDesc:     true,
			bodySubstring: "Kitchen Sink Skill",
		},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			// Render to target provider
			rendered, err := conv.Render(canonical1.Content, tt.provider)
			if err != nil {
				t.Fatalf("Render to %s: %v", tt.name, err)
			}
			if len(rendered.Content) == 0 {
				t.Fatalf("Render to %s produced empty content", tt.name)
			}

			// Pass 2: re-canonicalize the rendered output
			canonical2, err := conv.Canonicalize(rendered.Content, tt.sourceSlug)
			if err != nil {
				t.Fatalf("Canonicalize pass 2 from %s: %v", tt.name, err)
			}

			meta2, body2, err := parseSkillCanonical(canonical2.Content)
			if err != nil {
				t.Fatalf("parsing canonical2 from %s: %v", tt.name, err)
			}

			// Assert name survived (if not expected to be lossy)
			if !tt.lossyName {
				if meta2.Name != meta1.Name {
					t.Errorf("name mismatch: got %q, want %q", meta2.Name, meta1.Name)
				}
			}

			// Assert description survived (if not expected to be lossy)
			if !tt.lossyDesc {
				if meta2.Description != meta1.Description {
					t.Errorf("description mismatch: got %q, want %q", meta2.Description, meta1.Description)
				}
			}

			// Assert body content survived (not lost entirely)
			if body2 == "" {
				t.Errorf("body is empty after round-trip through %s", tt.name)
			}
			if !strings.Contains(body2, tt.bodySubstring) {
				t.Errorf("body missing %q after round-trip through %s:\n%s",
					tt.bodySubstring, tt.name, body2)
			}
		})
	}
}

// agentRoundTripTarget defines a provider to test for agent round-trips.
type agentRoundTripTarget struct {
	name          string
	provider      provider.Provider
	sourceSlug    string // provider slug used when re-canonicalizing
	lossyName     bool   // true if name is expected to be lost in round-trip
	lossyDesc     bool   // true if description is expected to be lost
	bodySubstring string // required substring in the round-tripped body
	minWarnings   int    // minimum warnings from the Render step (lossy providers drop fields)
}

func TestKitchenSinkAgentRoundTrip(t *testing.T) {
	root := repoRoot(t)
	agentPath := filepath.Join(root, "content", "agents", "example-kitchen-sink-agent", "AGENT.md")

	original, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("reading kitchen-sink agent: %v", err)
	}

	conv := &AgentsConverter{}

	// Pass 1: canonicalize the original
	canonical1, err := conv.Canonicalize(original, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize pass 1: %v", err)
	}

	meta1, body1, err := parseAgentCanonical(canonical1.Content)
	if err != nil {
		t.Fatalf("parsing canonical1: %v", err)
	}
	if meta1.Name == "" {
		t.Fatal("pass 1: Name is empty")
	}
	if body1 == "" {
		t.Fatal("pass 1: body is empty")
	}

	// Providers to round-trip through.
	//
	// Roo Code is excluded: it renders to a YAML custom-mode file that can't
	// be re-canonicalized through the standard markdown frontmatter parser.
	// It is tested separately in agents_test.go for field preservation and warnings.
	targets := []agentRoundTripTarget{
		{
			name:          "claude-code",
			provider:      provider.ClaudeCode,
			sourceSlug:    "claude-code",
			bodySubstring: "Kitchen Sink Agent",
		},
		{
			name:          "gemini-cli",
			provider:      provider.GeminiCLI,
			sourceSlug:    "gemini-cli",
			bodySubstring: "Kitchen Sink Agent",
		},
		{
			name:          "copilot-cli",
			provider:      provider.CopilotCLI,
			sourceSlug:    "copilot-cli",
			bodySubstring: "Kitchen Sink Agent",
		},
		{
			name:          "opencode",
			provider:      provider.OpenCode,
			sourceSlug:    "opencode",
			bodySubstring: "Kitchen Sink Agent",
			minWarnings:   1, // permissionMode is dropped
		},
		{
			name:          "kiro",
			provider:      provider.Kiro,
			sourceSlug:    "kiro",
			bodySubstring: "kiro:prompt-file", // Kiro moves body to ExtraFiles; re-canonicalized body is a file reference
			minWarnings:   1,                  // maxTurns, permissionMode, disallowedTools
		},
		{
			name:          "codex",
			provider:      provider.Codex,
			sourceSlug:    "codex",
			lossyName:     true, // Codex slugifies name → "kitchen-sink-agent" (loses casing/spaces)
			lossyDesc:     true, // Codex TOML has no description field
			bodySubstring: "Kitchen Sink Agent",
			minWarnings:   5, // many fields not supported by Codex
		},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			// Render to target provider
			rendered, err := conv.Render(canonical1.Content, tt.provider)
			if err != nil {
				t.Fatalf("Render to %s: %v", tt.name, err)
			}
			if len(rendered.Content) == 0 {
				t.Fatalf("Render to %s produced empty content", tt.name)
			}

			// Assert minimum warning count for lossy providers
			if len(rendered.Warnings) < tt.minWarnings {
				t.Errorf("expected at least %d warnings from Render to %s, got %d: %v",
					tt.minWarnings, tt.name, len(rendered.Warnings), rendered.Warnings)
			}

			// Pass 2: re-canonicalize the rendered output
			canonical2, err := conv.Canonicalize(rendered.Content, tt.sourceSlug)
			if err != nil {
				t.Fatalf("Canonicalize pass 2 from %s: %v", tt.name, err)
			}

			meta2, body2, err := parseAgentCanonical(canonical2.Content)
			if err != nil {
				t.Fatalf("parsing canonical2 from %s: %v", tt.name, err)
			}

			// Assert name survived
			if !tt.lossyName && meta2.Name != meta1.Name {
				t.Errorf("name mismatch: got %q, want %q", meta2.Name, meta1.Name)
			}

			// Assert description survived
			if !tt.lossyDesc && meta2.Description != meta1.Description {
				t.Errorf("description mismatch: got %q, want %q", meta2.Description, meta1.Description)
			}

			// Assert body content survived
			if body2 == "" {
				t.Errorf("body is empty after round-trip through %s", tt.name)
			}
			if !strings.Contains(body2, tt.bodySubstring) {
				t.Errorf("body missing %q after round-trip through %s:\n%s",
					tt.bodySubstring, tt.name, body2)
			}
		})
	}
}
