package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHooksContentType(t *testing.T) {
	if HooksContentType != "hooks" {
		t.Errorf("HooksContentType = %q, want %q", HooksContentType, "hooks")
	}
}

func TestCanonicalHooksKeys_MatchesCanonicalKeysYAML(t *testing.T) {
	// Drift guard: the Go-side CanonicalHooksKeys list MUST stay in sync with
	// the hooks section of docs/spec/canonical-keys.yaml. If they diverge, the
	// validator and recognizers will disagree about which keys are legal —
	// silent acceptance is the failure mode this test guards against.
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
	hooksKeys, ok := doc.ContentTypes["hooks"]
	if !ok {
		t.Fatal("canonical-keys.yaml has no content_types.hooks section")
	}

	yamlSet := make(map[string]struct{}, len(hooksKeys))
	for k := range hooksKeys {
		yamlSet[k] = struct{}{}
	}
	goSet := make(map[string]struct{}, len(CanonicalHooksKeys))
	for _, k := range CanonicalHooksKeys {
		goSet[k] = struct{}{}
	}

	for k := range yamlSet {
		if _, ok := goSet[k]; !ok {
			t.Errorf("canonical-keys.yaml declares hooks key %q but Go CanonicalHooksKeys does not", k)
		}
	}
	for k := range goSet {
		if _, ok := yamlSet[k]; !ok {
			t.Errorf("Go CanonicalHooksKeys declares %q but canonical-keys.yaml does not", k)
		}
	}
}

func TestIsCanonicalHooksKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"handler_types", true},
		{"matcher_patterns", true},
		{"decision_control", true},
		{"input_modification", true},
		{"async_execution", true},
		{"hook_scopes", true},
		{"json_io_protocol", true},
		{"context_injection", true},
		{"permission_control", true},
		{"unknown_key", false},
		{"", false},
		{"DECISION_CONTROL", false}, // case-sensitive
		{"decision-control", false}, // hyphen vs underscore
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := IsCanonicalHooksKey(c.key); got != c.want {
				t.Errorf("IsCanonicalHooksKey(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

func TestHooksLandmarkOptions_SetsContentType(t *testing.T) {
	opts := HooksLandmarkOptions()
	if opts.ContentType != "hooks" {
		t.Errorf("ContentType = %q, want %q", opts.ContentType, "hooks")
	}
	if len(opts.Patterns) != 0 {
		t.Errorf("Patterns len = %d, want 0", len(opts.Patterns))
	}
}

func TestHooksLandmarkOptions_PreservesPatterns(t *testing.T) {
	pats := []LandmarkPattern{
		{Capability: "handler_types", Mechanism: "m1", Matchers: []StringMatcher{{Value: "x"}}},
		{Capability: "matcher_patterns", Mechanism: "m2", Matchers: []StringMatcher{{Value: "y"}}},
	}
	opts := HooksLandmarkOptions(pats...)
	if len(opts.Patterns) != 2 {
		t.Fatalf("Patterns len = %d, want 2", len(opts.Patterns))
	}
	if opts.Patterns[0].Capability != "handler_types" || opts.Patterns[1].Capability != "matcher_patterns" {
		t.Errorf("patterns out of order: %v", opts.Patterns)
	}
}

func TestHooksLandmarkOptions_AcceptsEmptyCapability(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HooksLandmarkOptions panicked on empty Capability: %v", r)
		}
	}()
	_ = HooksLandmarkOptions(LandmarkPattern{Capability: "", Matchers: []StringMatcher{{Value: "x"}}})
}

func TestHooksLandmarkOptions_AcceptsNestedCanonicalKey(t *testing.T) {
	// Object-typed canonical keys (handler_types, decision_control, hook_scopes)
	// may emit nested sub-segments. Validation only checks the first segment.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HooksLandmarkOptions panicked on nested canonical key: %v", r)
		}
	}()
	_ = HooksLandmarkOptions(
		LandmarkPattern{Capability: "decision_control.block", Mechanism: "m"},
		LandmarkPattern{Capability: "handler_types.shell", Mechanism: "m"},
		LandmarkPattern{Capability: "hook_scopes.project", Mechanism: "m"},
	)
}

func TestHooksLandmarkOptions_PanicsOnUnknownKey(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on unknown canonical hooks key, got none")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "unknown canonical hooks key") {
			t.Errorf("panic message = %q, want substring %q", msg, "unknown canonical hooks key")
		}
		if !strings.Contains(msg, "bogus_key") {
			t.Errorf("panic message = %q, should mention the offending key", msg)
		}
	}()
	_ = HooksLandmarkOptions(LandmarkPattern{Capability: "bogus_key", Mechanism: "m"})
}

func TestHooksLandmarkOptions_PanicsOnUnknownNestedHead(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when nested capability head is unknown")
		}
	}()
	_ = HooksLandmarkOptions(LandmarkPattern{Capability: "bogus_root.sub_field", Mechanism: "m"})
}

func TestHooksLandmarkPattern_BuildsExpectedShape(t *testing.T) {
	pat := HooksLandmarkPattern("handler_types", "Handler types", "documented under 'Handler types' heading", nil)
	if pat.Capability != "handler_types" {
		t.Errorf("Capability = %q, want %q", pat.Capability, "handler_types")
	}
	if pat.Mechanism != "documented under 'Handler types' heading" {
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
	if m.Value != "Handler types" {
		t.Errorf("Matcher.Value = %q", m.Value)
	}
	if !m.CaseInsensitive {
		t.Error("Matcher.CaseInsensitive = false, want true")
	}
}

func TestHooksLandmarkPattern_PassesThroughRequired(t *testing.T) {
	required := []StringMatcher{{Kind: "substring", Value: "Hooks", CaseInsensitive: true}}
	pat := HooksLandmarkPattern("decision_control", "Decision control", "exit-code contract", required)
	if len(pat.Required) != 1 || pat.Required[0].Value != "Hooks" {
		t.Errorf("Required = %+v", pat.Required)
	}
}

// TestHooksHelpers_ComposeWithRecognizeLandmarks confirms that the helpers and
// recognizeLandmarks integrate correctly: the resulting capabilities map uses
// the hooks.* dot-path prefix, hits the canonical key, and carries the
// "inferred" confidence baked in by recognizeLandmarks.
func TestHooksHelpers_ComposeWithRecognizeLandmarks(t *testing.T) {
	opts := HooksLandmarkOptions(
		HooksLandmarkPattern("handler_types", "Handler types", "documented under 'Handler types'", nil),
		HooksLandmarkPattern("async_execution", "Async hooks", "documented under 'Async hooks'", nil),
	)
	ctx := RecognitionContext{
		Provider:  "test-provider",
		Landmarks: []string{"How hooks work", "Handler types", "Best practices"},
	}
	res := recognizeLandmarks(ctx, opts)

	if res.Status != StatusRecognized {
		t.Errorf("Status = %q, want %q", res.Status, StatusRecognized)
	}

	// handler_types matched.
	if v := res.Capabilities["hooks.capabilities.handler_types.supported"]; v != "true" {
		t.Errorf("handler_types.supported = %q, want %q", v, "true")
	}
	if v := res.Capabilities["hooks.capabilities.handler_types.confidence"]; v != confidenceInferred {
		t.Errorf("handler_types.confidence = %q, want %q", v, confidenceInferred)
	}
	if v := res.Capabilities["hooks.capabilities.handler_types.mechanism"]; v != "documented under 'Handler types'" {
		t.Errorf("handler_types.mechanism = %q", v)
	}
	if v := res.Capabilities["hooks.supported"]; v != "true" {
		t.Errorf("hooks.supported = %q, want %q", v, "true")
	}

	// async_execution did not match (no "Async hooks" landmark) → key absent.
	if _, ok := res.Capabilities["hooks.capabilities.async_execution.supported"]; ok {
		t.Error("async_execution.supported should not be emitted (no matching landmark)")
	}
}
