package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCommandsContentType(t *testing.T) {
	if CommandsContentType != "commands" {
		t.Errorf("CommandsContentType = %q, want %q", CommandsContentType, "commands")
	}
}

func TestCanonicalCommandsKeys_MatchesCanonicalKeysYAML(t *testing.T) {
	// Drift guard: the Go-side CanonicalCommandsKeys list MUST stay in sync
	// with the commands section of docs/spec/canonical-keys.yaml. If they
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
	cmdKeys, ok := doc.ContentTypes["commands"]
	if !ok {
		t.Fatal("canonical-keys.yaml has no content_types.commands section")
	}

	yamlSet := make(map[string]struct{}, len(cmdKeys))
	for k := range cmdKeys {
		yamlSet[k] = struct{}{}
	}
	goSet := make(map[string]struct{}, len(CanonicalCommandsKeys))
	for _, k := range CanonicalCommandsKeys {
		goSet[k] = struct{}{}
	}

	for k := range yamlSet {
		if _, ok := goSet[k]; !ok {
			t.Errorf("canonical-keys.yaml declares commands key %q but Go CanonicalCommandsKeys does not", k)
		}
	}
	for k := range goSet {
		if _, ok := yamlSet[k]; !ok {
			t.Errorf("Go CanonicalCommandsKeys declares %q but canonical-keys.yaml does not", k)
		}
	}
}

func TestIsCanonicalCommandsKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"argument_substitution", true},
		{"builtin_commands", true},
		{"unknown_key", false},
		{"", false},
		{"ARGUMENT_SUBSTITUTION", false}, // case-sensitive
		{"argument-substitution", false}, // hyphen vs underscore
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := IsCanonicalCommandsKey(c.key); got != c.want {
				t.Errorf("IsCanonicalCommandsKey(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

func TestCommandsLandmarkOptions_SetsContentType(t *testing.T) {
	opts := CommandsLandmarkOptions()
	if opts.ContentType != "commands" {
		t.Errorf("ContentType = %q, want %q", opts.ContentType, "commands")
	}
	if len(opts.Patterns) != 0 {
		t.Errorf("Patterns len = %d, want 0", len(opts.Patterns))
	}
}

func TestCommandsLandmarkOptions_PreservesPatterns(t *testing.T) {
	pats := []LandmarkPattern{
		{Capability: "argument_substitution", Mechanism: "m1", Matchers: []StringMatcher{{Value: "x"}}},
		{Capability: "builtin_commands", Mechanism: "m2", Matchers: []StringMatcher{{Value: "y"}}},
	}
	opts := CommandsLandmarkOptions(pats...)
	if len(opts.Patterns) != 2 {
		t.Fatalf("Patterns len = %d, want 2", len(opts.Patterns))
	}
	if opts.Patterns[0].Capability != "argument_substitution" || opts.Patterns[1].Capability != "builtin_commands" {
		t.Errorf("patterns out of order: %v", opts.Patterns)
	}
}

func TestCommandsLandmarkOptions_AcceptsEmptyCapability(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CommandsLandmarkOptions panicked on empty Capability: %v", r)
		}
	}()
	_ = CommandsLandmarkOptions(LandmarkPattern{Capability: "", Matchers: []StringMatcher{{Value: "x"}}})
}

func TestCommandsLandmarkOptions_AcceptsNestedCanonicalKey(t *testing.T) {
	// Object-typed canonical keys (argument_substitution) may emit nested
	// sub-segments. Validation only checks the first segment.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CommandsLandmarkOptions panicked on nested canonical key: %v", r)
		}
	}()
	_ = CommandsLandmarkOptions(
		LandmarkPattern{Capability: "argument_substitution.dollar_arguments", Mechanism: "m"},
		LandmarkPattern{Capability: "argument_substitution.positional", Mechanism: "m"},
	)
}

func TestCommandsLandmarkOptions_PanicsOnUnknownKey(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on unknown canonical commands key, got none")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "unknown canonical commands key") {
			t.Errorf("panic message = %q, want substring %q", msg, "unknown canonical commands key")
		}
		if !strings.Contains(msg, "bogus_key") {
			t.Errorf("panic message = %q, should mention the offending key", msg)
		}
	}()
	_ = CommandsLandmarkOptions(LandmarkPattern{Capability: "bogus_key", Mechanism: "m"})
}

func TestCommandsLandmarkOptions_PanicsOnUnknownNestedHead(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when nested capability head is unknown")
		}
	}()
	_ = CommandsLandmarkOptions(LandmarkPattern{Capability: "bogus_root.sub_field", Mechanism: "m"})
}

func TestCommandsLandmarkPattern_BuildsExpectedShape(t *testing.T) {
	pat := CommandsLandmarkPattern("argument_substitution", "$ARGUMENTS", "documented under 'Argument substitution' heading", nil)
	if pat.Capability != "argument_substitution" {
		t.Errorf("Capability = %q, want %q", pat.Capability, "argument_substitution")
	}
	if pat.Mechanism != "documented under 'Argument substitution' heading" {
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
	if m.Value != "$ARGUMENTS" {
		t.Errorf("Matcher.Value = %q", m.Value)
	}
	if !m.CaseInsensitive {
		t.Error("Matcher.CaseInsensitive = false, want true")
	}
}

func TestCommandsLandmarkPattern_PassesThroughRequired(t *testing.T) {
	required := []StringMatcher{{Kind: "substring", Value: "Slash Commands", CaseInsensitive: true}}
	pat := CommandsLandmarkPattern("builtin_commands", "/help", "built-in slash commands", required)
	if len(pat.Required) != 1 || pat.Required[0].Value != "Slash Commands" {
		t.Errorf("Required = %+v", pat.Required)
	}
}

// TestCommandsHelpers_ComposeWithRecognizeLandmarks confirms that the helpers
// and recognizeLandmarks integrate correctly: the resulting capabilities map
// uses the commands.* dot-path prefix, hits the canonical key, and carries
// the "inferred" confidence baked in by recognizeLandmarks.
func TestCommandsHelpers_ComposeWithRecognizeLandmarks(t *testing.T) {
	opts := CommandsLandmarkOptions(
		CommandsLandmarkPattern("argument_substitution", "$ARGUMENTS", "documented under '$ARGUMENTS'", nil),
		CommandsLandmarkPattern("builtin_commands", "Built-in commands", "documented under 'Built-in commands'", nil),
	)
	ctx := RecognitionContext{
		Provider:  "test-provider",
		Landmarks: []string{"Slash Commands", "$ARGUMENTS expansion", "Custom commands"},
	}
	res := recognizeLandmarks(ctx, opts)

	if res.Status != StatusRecognized {
		t.Errorf("Status = %q, want %q", res.Status, StatusRecognized)
	}

	// argument_substitution matched.
	if v := res.Capabilities["commands.capabilities.argument_substitution.supported"]; v != "true" {
		t.Errorf("argument_substitution.supported = %q, want %q", v, "true")
	}
	if v := res.Capabilities["commands.capabilities.argument_substitution.confidence"]; v != confidenceInferred {
		t.Errorf("argument_substitution.confidence = %q, want %q", v, confidenceInferred)
	}
	if v := res.Capabilities["commands.capabilities.argument_substitution.mechanism"]; v != "documented under '$ARGUMENTS'" {
		t.Errorf("argument_substitution.mechanism = %q", v)
	}
	if v := res.Capabilities["commands.supported"]; v != "true" {
		t.Errorf("commands.supported = %q, want %q", v, "true")
	}

	// builtin_commands did not match → key absent.
	if _, ok := res.Capabilities["commands.capabilities.builtin_commands.supported"]; ok {
		t.Error("builtin_commands.supported should not be emitted (no matching landmark)")
	}
}
