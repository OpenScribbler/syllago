package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRulesContentType(t *testing.T) {
	if RulesContentType != "rules" {
		t.Errorf("RulesContentType = %q, want %q", RulesContentType, "rules")
	}
}

func TestCanonicalRulesKeys_MatchesCanonicalKeysYAML(t *testing.T) {
	// Drift guard: the Go-side CanonicalRulesKeys list MUST stay in sync with
	// the rules section of docs/spec/canonical-keys.yaml. If they diverge, the
	// validator (formatdoc_validate.go) and recognizers (recognize_*.go) will
	// disagree about which keys are legal — silent acceptance is the failure
	// mode this test guards against.
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
	rulesKeys, ok := doc.ContentTypes["rules"]
	if !ok {
		t.Fatal("canonical-keys.yaml has no content_types.rules section")
	}

	yamlSet := make(map[string]struct{}, len(rulesKeys))
	for k := range rulesKeys {
		yamlSet[k] = struct{}{}
	}
	goSet := make(map[string]struct{}, len(CanonicalRulesKeys))
	for _, k := range CanonicalRulesKeys {
		goSet[k] = struct{}{}
	}

	for k := range yamlSet {
		if _, ok := goSet[k]; !ok {
			t.Errorf("canonical-keys.yaml declares rules key %q but Go CanonicalRulesKeys does not", k)
		}
	}
	for k := range goSet {
		if _, ok := yamlSet[k]; !ok {
			t.Errorf("Go CanonicalRulesKeys declares %q but canonical-keys.yaml does not", k)
		}
	}
}

func TestIsCanonicalRulesKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"activation_mode", true},
		{"file_imports", true},
		{"cross_provider_recognition", true},
		{"auto_memory", true},
		{"hierarchical_loading", true},
		{"unknown_key", false},
		{"", false},
		{"ACTIVATION_MODE", false}, // case-sensitive
		{"activation-mode", false}, // hyphen vs underscore
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := IsCanonicalRulesKey(c.key); got != c.want {
				t.Errorf("IsCanonicalRulesKey(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

func TestRulesLandmarkOptions_SetsContentType(t *testing.T) {
	opts := RulesLandmarkOptions()
	if opts.ContentType != "rules" {
		t.Errorf("ContentType = %q, want %q", opts.ContentType, "rules")
	}
	if len(opts.Patterns) != 0 {
		t.Errorf("Patterns len = %d, want 0", len(opts.Patterns))
	}
}

func TestRulesLandmarkOptions_PreservesPatterns(t *testing.T) {
	pats := []LandmarkPattern{
		{Capability: "activation_mode", Mechanism: "m1", Matchers: []StringMatcher{{Value: "x"}}},
		{Capability: "file_imports", Mechanism: "m2", Matchers: []StringMatcher{{Value: "y"}}},
	}
	opts := RulesLandmarkOptions(pats...)
	if len(opts.Patterns) != 2 {
		t.Fatalf("Patterns len = %d, want 2", len(opts.Patterns))
	}
	if opts.Patterns[0].Capability != "activation_mode" || opts.Patterns[1].Capability != "file_imports" {
		t.Errorf("patterns out of order: %v", opts.Patterns)
	}
}

func TestRulesLandmarkOptions_AcceptsEmptyCapability(t *testing.T) {
	// An empty Capability is the supported() of a pattern that contributes
	// only to the top-level rules.supported emission. It must NOT trigger
	// the canonical-key check.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RulesLandmarkOptions panicked on empty Capability: %v", r)
		}
	}()
	_ = RulesLandmarkOptions(LandmarkPattern{Capability: "", Matchers: []StringMatcher{{Value: "x"}}})
}

func TestRulesLandmarkOptions_AcceptsNestedCanonicalKey(t *testing.T) {
	// Object-typed canonical keys (activation_mode, cross_provider_recognition)
	// may emit nested sub-segments. Validation only checks the first segment.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RulesLandmarkOptions panicked on nested canonical key: %v", r)
		}
	}()
	_ = RulesLandmarkOptions(
		LandmarkPattern{Capability: "activation_mode.always_on", Mechanism: "m"},
		LandmarkPattern{Capability: "cross_provider_recognition.agents_md", Mechanism: "m"},
	)
}

func TestRulesLandmarkOptions_PanicsOnUnknownKey(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on unknown canonical rules key, got none")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "unknown canonical rules key") {
			t.Errorf("panic message = %q, want substring %q", msg, "unknown canonical rules key")
		}
		if !strings.Contains(msg, "bogus_key") {
			t.Errorf("panic message = %q, should mention the offending key", msg)
		}
	}()
	_ = RulesLandmarkOptions(LandmarkPattern{Capability: "bogus_key", Mechanism: "m"})
}

func TestRulesLandmarkOptions_PanicsOnUnknownNestedHead(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when nested capability head is unknown")
		}
	}()
	_ = RulesLandmarkOptions(LandmarkPattern{Capability: "bogus_root.sub_field", Mechanism: "m"})
}

func TestRulesLandmarkPattern_BuildsExpectedShape(t *testing.T) {
	pat := RulesLandmarkPattern("activation_mode", "Activation Modes", "documented under 'Activation Modes' heading", nil)
	if pat.Capability != "activation_mode" {
		t.Errorf("Capability = %q, want %q", pat.Capability, "activation_mode")
	}
	if pat.Mechanism != "documented under 'Activation Modes' heading" {
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
	if m.Value != "Activation Modes" {
		t.Errorf("Matcher.Value = %q", m.Value)
	}
	if !m.CaseInsensitive {
		t.Error("Matcher.CaseInsensitive = false, want true")
	}
}

func TestRulesLandmarkPattern_PassesThroughRequired(t *testing.T) {
	required := []StringMatcher{{Kind: "substring", Value: "Rules", CaseInsensitive: true}}
	pat := RulesLandmarkPattern("file_imports", "Import additional files", "@-import syntax", required)
	if len(pat.Required) != 1 || pat.Required[0].Value != "Rules" {
		t.Errorf("Required = %+v", pat.Required)
	}
}

// TestRulesHelpers_ComposeWithRecognizeLandmarks confirms that the helpers and
// recognizeLandmarks integrate correctly: the resulting capabilities map uses
// the rules.* dot-path prefix, hits the canonical key, and carries the
// "inferred" confidence baked in by recognizeLandmarks.
func TestRulesHelpers_ComposeWithRecognizeLandmarks(t *testing.T) {
	opts := RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode", "Activation Modes", "documented under 'Activation Modes'", nil),
		RulesLandmarkPattern("auto_memory", "Auto memory", "documented under 'Auto memory'", nil),
	)
	ctx := RecognitionContext{
		Provider:  "test-provider",
		Landmarks: []string{"How rules work", "Activation Modes", "Best practices"},
	}
	res := recognizeLandmarks(ctx, opts)

	if res.Status != StatusRecognized {
		t.Errorf("Status = %q, want %q", res.Status, StatusRecognized)
	}

	// activation_mode matched.
	if v := res.Capabilities["rules.capabilities.activation_mode.supported"]; v != "true" {
		t.Errorf("activation_mode.supported = %q, want %q", v, "true")
	}
	if v := res.Capabilities["rules.capabilities.activation_mode.confidence"]; v != confidenceInferred {
		t.Errorf("activation_mode.confidence = %q, want %q", v, confidenceInferred)
	}
	if v := res.Capabilities["rules.capabilities.activation_mode.mechanism"]; v != "documented under 'Activation Modes'" {
		t.Errorf("activation_mode.mechanism = %q", v)
	}
	if v := res.Capabilities["rules.supported"]; v != "true" {
		t.Errorf("rules.supported = %q, want %q", v, "true")
	}

	// auto_memory did not match (no "Auto memory" landmark) → key absent.
	if _, ok := res.Capabilities["rules.capabilities.auto_memory.supported"]; ok {
		t.Error("auto_memory.supported should not be emitted (no matching landmark)")
	}
}
