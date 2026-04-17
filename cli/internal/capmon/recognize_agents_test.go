package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAgentsContentType(t *testing.T) {
	if AgentsContentType != "agents" {
		t.Errorf("AgentsContentType = %q, want %q", AgentsContentType, "agents")
	}
}

func TestCanonicalAgentsKeys_MatchesCanonicalKeysYAML(t *testing.T) {
	// Drift guard: the Go-side CanonicalAgentsKeys list MUST stay in sync
	// with the agents section of docs/spec/canonical-keys.yaml. If they
	// diverge, the validator and recognizers will disagree about which keys
	// are legal — silent acceptance is the failure mode this test guards
	// against.
	repoRoot := filepath.Join("..", "..", "..")
	keysPath := filepath.Join(repoRoot, "docs", "spec", "canonical-keys.yaml")
	data, err := os.ReadFile(keysPath)
	if err != nil {
		t.Fatalf("read canonical-keys.yaml: %v", err)
	}
	var doc struct {
		ContentTypes map[string]map[string]any `yaml:"content_types"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse canonical-keys.yaml: %v", err)
	}
	agentsKeys, ok := doc.ContentTypes["agents"]
	if !ok {
		t.Fatal("canonical-keys.yaml has no content_types.agents section")
	}

	yamlSet := make(map[string]struct{}, len(agentsKeys))
	for k := range agentsKeys {
		yamlSet[k] = struct{}{}
	}
	goSet := make(map[string]struct{}, len(CanonicalAgentsKeys))
	for _, k := range CanonicalAgentsKeys {
		goSet[k] = struct{}{}
	}

	for k := range yamlSet {
		if _, ok := goSet[k]; !ok {
			t.Errorf("canonical-keys.yaml declares agents key %q but Go CanonicalAgentsKeys does not", k)
		}
	}
	for k := range goSet {
		if _, ok := yamlSet[k]; !ok {
			t.Errorf("Go CanonicalAgentsKeys declares %q but canonical-keys.yaml does not", k)
		}
	}
}

func TestIsCanonicalAgentsKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"definition_format", true},
		{"tool_restrictions", true},
		{"invocation_patterns", true},
		{"agent_scopes", true},
		{"model_selection", true},
		{"per_agent_mcp", true},
		{"subagent_spawning", true},
		{"unknown_key", false},
		{"", false},
		{"DEFINITION_FORMAT", false}, // case-sensitive
		{"definition-format", false}, // hyphen vs underscore
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := IsCanonicalAgentsKey(c.key); got != c.want {
				t.Errorf("IsCanonicalAgentsKey(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

func TestAgentsLandmarkOptions_SetsContentType(t *testing.T) {
	opts := AgentsLandmarkOptions()
	if opts.ContentType != "agents" {
		t.Errorf("ContentType = %q, want %q", opts.ContentType, "agents")
	}
	if len(opts.Patterns) != 0 {
		t.Errorf("Patterns len = %d, want 0", len(opts.Patterns))
	}
}

func TestAgentsLandmarkOptions_PreservesPatterns(t *testing.T) {
	pats := []LandmarkPattern{
		{Capability: "definition_format", Mechanism: "m1", Matchers: []StringMatcher{{Value: "x"}}},
		{Capability: "tool_restrictions", Mechanism: "m2", Matchers: []StringMatcher{{Value: "y"}}},
	}
	opts := AgentsLandmarkOptions(pats...)
	if len(opts.Patterns) != 2 {
		t.Fatalf("Patterns len = %d, want 2", len(opts.Patterns))
	}
	if opts.Patterns[0].Capability != "definition_format" || opts.Patterns[1].Capability != "tool_restrictions" {
		t.Errorf("patterns out of order: %v", opts.Patterns)
	}
}

func TestAgentsLandmarkOptions_AcceptsEmptyCapability(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("AgentsLandmarkOptions panicked on empty Capability: %v", r)
		}
	}()
	_ = AgentsLandmarkOptions(LandmarkPattern{Capability: "", Matchers: []StringMatcher{{Value: "x"}}})
}

func TestAgentsLandmarkOptions_AcceptsNestedCanonicalKey(t *testing.T) {
	// Object-typed canonical keys (invocation_patterns, agent_scopes) may
	// emit nested sub-segments. Validation only checks the first segment.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("AgentsLandmarkOptions panicked on nested canonical key: %v", r)
		}
	}()
	_ = AgentsLandmarkOptions(
		LandmarkPattern{Capability: "invocation_patterns.at_mention", Mechanism: "m"},
		LandmarkPattern{Capability: "invocation_patterns.slash_command", Mechanism: "m"},
		LandmarkPattern{Capability: "agent_scopes.project", Mechanism: "m"},
		LandmarkPattern{Capability: "agent_scopes.user", Mechanism: "m"},
	)
}

func TestAgentsLandmarkOptions_PanicsOnUnknownKey(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on unknown canonical agents key, got none")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "unknown canonical agents key") {
			t.Errorf("panic message = %q, want substring %q", msg, "unknown canonical agents key")
		}
		if !strings.Contains(msg, "bogus_key") {
			t.Errorf("panic message = %q, should mention the offending key", msg)
		}
	}()
	_ = AgentsLandmarkOptions(LandmarkPattern{Capability: "bogus_key", Mechanism: "m"})
}

func TestAgentsLandmarkOptions_PanicsOnUnknownNestedHead(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when nested capability head is unknown")
		}
	}()
	_ = AgentsLandmarkOptions(LandmarkPattern{Capability: "bogus_root.sub_field", Mechanism: "m"})
}

func TestAgentsLandmarkPattern_BuildsExpectedShape(t *testing.T) {
	pat := AgentsLandmarkPattern("definition_format", "Definition format", "documented under 'Definition format' heading", nil)
	if pat.Capability != "definition_format" {
		t.Errorf("Capability = %q, want %q", pat.Capability, "definition_format")
	}
	if pat.Mechanism != "documented under 'Definition format' heading" {
		t.Errorf("Mechanism = %q", pat.Mechanism)
	}
	if len(pat.Required) != 0 {
		t.Errorf("Required len = %d, want 0", len(pat.Required))
	}
	if len(pat.Matchers) != 1 {
		t.Fatalf("Matchers len = %d, want 1", len(pat.Matchers))
	}
	m := pat.Matchers[0]
	if m.Kind != "substring" {
		t.Errorf("Matcher.Kind = %q, want %q", m.Kind, "substring")
	}
	if m.Value != "Definition format" {
		t.Errorf("Matcher.Value = %q", m.Value)
	}
	if !m.CaseInsensitive {
		t.Error("Matcher.CaseInsensitive = false, want true")
	}
}

func TestAgentsLandmarkPattern_PassesThroughRequired(t *testing.T) {
	required := []StringMatcher{{Kind: "substring", Value: "Subagents", CaseInsensitive: true}}
	pat := AgentsLandmarkPattern("subagent_spawning", "Task tool", "delegation via Task tool", required)
	if len(pat.Required) != 1 || pat.Required[0].Value != "Subagents" {
		t.Errorf("Required = %+v", pat.Required)
	}
}

// TestAgentsHelpers_ComposeWithRecognizeLandmarks confirms that the helpers
// and recognizeLandmarks integrate correctly: the resulting capabilities map
// uses the agents.* dot-path prefix, hits the canonical key, and carries the
// "inferred" confidence baked in by recognizeLandmarks.
func TestAgentsHelpers_ComposeWithRecognizeLandmarks(t *testing.T) {
	opts := AgentsLandmarkOptions(
		AgentsLandmarkPattern("definition_format", "YAML frontmatter", "documented under 'YAML frontmatter'", nil),
		AgentsLandmarkPattern("model_selection", "Model override", "documented under 'Model override'", nil),
	)
	ctx := RecognitionContext{
		Provider:  "test-provider",
		Landmarks: []string{"Subagents", "YAML frontmatter", "Best practices"},
	}
	res := recognizeLandmarks(ctx, opts)

	if res.Status != StatusRecognized {
		t.Errorf("Status = %q, want %q", res.Status, StatusRecognized)
	}

	// definition_format matched.
	if v := res.Capabilities["agents.capabilities.definition_format.supported"]; v != "true" {
		t.Errorf("definition_format.supported = %q, want %q", v, "true")
	}
	if v := res.Capabilities["agents.capabilities.definition_format.confidence"]; v != confidenceInferred {
		t.Errorf("definition_format.confidence = %q, want %q", v, confidenceInferred)
	}
	if v := res.Capabilities["agents.capabilities.definition_format.mechanism"]; v != "documented under 'YAML frontmatter'" {
		t.Errorf("definition_format.mechanism = %q", v)
	}
	if v := res.Capabilities["agents.supported"]; v != "true" {
		t.Errorf("agents.supported = %q, want %q", v, "true")
	}

	// model_selection did not match → key absent.
	if _, ok := res.Capabilities["agents.capabilities.model_selection.supported"]; ok {
		t.Error("model_selection.supported should not be emitted (no matching landmark)")
	}
}
