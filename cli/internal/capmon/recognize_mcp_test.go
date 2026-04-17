package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMcpContentType(t *testing.T) {
	if McpContentType != "mcp" {
		t.Errorf("McpContentType = %q, want %q", McpContentType, "mcp")
	}
}

func TestCanonicalMcpKeys_MatchesCanonicalKeysYAML(t *testing.T) {
	// Drift guard: the Go-side CanonicalMcpKeys list MUST stay in sync with
	// the mcp section of docs/spec/canonical-keys.yaml. If they diverge, the
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
	mcpKeys, ok := doc.ContentTypes["mcp"]
	if !ok {
		t.Fatal("canonical-keys.yaml has no content_types.mcp section")
	}

	yamlSet := make(map[string]struct{}, len(mcpKeys))
	for k := range mcpKeys {
		yamlSet[k] = struct{}{}
	}
	goSet := make(map[string]struct{}, len(CanonicalMcpKeys))
	for _, k := range CanonicalMcpKeys {
		goSet[k] = struct{}{}
	}

	for k := range yamlSet {
		if _, ok := goSet[k]; !ok {
			t.Errorf("canonical-keys.yaml declares mcp key %q but Go CanonicalMcpKeys does not", k)
		}
	}
	for k := range goSet {
		if _, ok := yamlSet[k]; !ok {
			t.Errorf("Go CanonicalMcpKeys declares %q but canonical-keys.yaml does not", k)
		}
	}
}

func TestIsCanonicalMcpKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"transport_types", true},
		{"oauth_support", true},
		{"env_var_expansion", true},
		{"tool_filtering", true},
		{"auto_approve", true},
		{"marketplace", true},
		{"resource_referencing", true},
		{"enterprise_management", true},
		{"unknown_key", false},
		{"", false},
		{"TRANSPORT_TYPES", false}, // case-sensitive
		{"transport-types", false}, // hyphen vs underscore
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := IsCanonicalMcpKey(c.key); got != c.want {
				t.Errorf("IsCanonicalMcpKey(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

func TestMcpLandmarkOptions_SetsContentType(t *testing.T) {
	opts := McpLandmarkOptions()
	if opts.ContentType != "mcp" {
		t.Errorf("ContentType = %q, want %q", opts.ContentType, "mcp")
	}
	if len(opts.Patterns) != 0 {
		t.Errorf("Patterns len = %d, want 0", len(opts.Patterns))
	}
}

func TestMcpLandmarkOptions_PreservesPatterns(t *testing.T) {
	pats := []LandmarkPattern{
		{Capability: "transport_types", Mechanism: "m1", Matchers: []StringMatcher{{Value: "x"}}},
		{Capability: "oauth_support", Mechanism: "m2", Matchers: []StringMatcher{{Value: "y"}}},
	}
	opts := McpLandmarkOptions(pats...)
	if len(opts.Patterns) != 2 {
		t.Fatalf("Patterns len = %d, want 2", len(opts.Patterns))
	}
	if opts.Patterns[0].Capability != "transport_types" || opts.Patterns[1].Capability != "oauth_support" {
		t.Errorf("patterns out of order: %v", opts.Patterns)
	}
}

func TestMcpLandmarkOptions_AcceptsEmptyCapability(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("McpLandmarkOptions panicked on empty Capability: %v", r)
		}
	}()
	_ = McpLandmarkOptions(LandmarkPattern{Capability: "", Matchers: []StringMatcher{{Value: "x"}}})
}

func TestMcpLandmarkOptions_AcceptsNestedCanonicalKey(t *testing.T) {
	// Object-typed canonical keys (transport_types, tool_filtering) may emit
	// nested sub-segments. Validation only checks the first segment.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("McpLandmarkOptions panicked on nested canonical key: %v", r)
		}
	}()
	_ = McpLandmarkOptions(
		LandmarkPattern{Capability: "transport_types.stdio", Mechanism: "m"},
		LandmarkPattern{Capability: "transport_types.http", Mechanism: "m"},
		LandmarkPattern{Capability: "tool_filtering.allowlist", Mechanism: "m"},
	)
}

func TestMcpLandmarkOptions_PanicsOnUnknownKey(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on unknown canonical mcp key, got none")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "unknown canonical mcp key") {
			t.Errorf("panic message = %q, want substring %q", msg, "unknown canonical mcp key")
		}
		if !strings.Contains(msg, "bogus_key") {
			t.Errorf("panic message = %q, should mention the offending key", msg)
		}
	}()
	_ = McpLandmarkOptions(LandmarkPattern{Capability: "bogus_key", Mechanism: "m"})
}

func TestMcpLandmarkOptions_PanicsOnUnknownNestedHead(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when nested capability head is unknown")
		}
	}()
	_ = McpLandmarkOptions(LandmarkPattern{Capability: "bogus_root.sub_field", Mechanism: "m"})
}

func TestMcpLandmarkPattern_BuildsExpectedShape(t *testing.T) {
	pat := McpLandmarkPattern("transport_types", "Transport types", "documented under 'Transport types' heading", nil)
	if pat.Capability != "transport_types" {
		t.Errorf("Capability = %q, want %q", pat.Capability, "transport_types")
	}
	if pat.Mechanism != "documented under 'Transport types' heading" {
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
	if m.Value != "Transport types" {
		t.Errorf("Matcher.Value = %q", m.Value)
	}
	if !m.CaseInsensitive {
		t.Error("Matcher.CaseInsensitive = false, want true")
	}
}

func TestMcpLandmarkPattern_PassesThroughRequired(t *testing.T) {
	required := []StringMatcher{{Kind: "substring", Value: "MCP", CaseInsensitive: true}}
	pat := McpLandmarkPattern("oauth_support", "OAuth", "OAuth 2.0 with token refresh", required)
	if len(pat.Required) != 1 || pat.Required[0].Value != "MCP" {
		t.Errorf("Required = %+v", pat.Required)
	}
}

// TestMcpHelpers_ComposeWithRecognizeLandmarks confirms that the helpers and
// recognizeLandmarks integrate correctly: the resulting capabilities map uses
// the mcp.* dot-path prefix, hits the canonical key, and carries the
// "inferred" confidence baked in by recognizeLandmarks.
func TestMcpHelpers_ComposeWithRecognizeLandmarks(t *testing.T) {
	opts := McpLandmarkOptions(
		McpLandmarkPattern("transport_types", "Transport types", "documented under 'Transport types'", nil),
		McpLandmarkPattern("oauth_support", "OAuth callback", "documented under 'OAuth callback'", nil),
	)
	ctx := RecognitionContext{
		Provider:  "test-provider",
		Landmarks: []string{"How MCP works", "Transport types", "Best practices"},
	}
	res := recognizeLandmarks(ctx, opts)

	if res.Status != StatusRecognized {
		t.Errorf("Status = %q, want %q", res.Status, StatusRecognized)
	}

	// transport_types matched.
	if v := res.Capabilities["mcp.capabilities.transport_types.supported"]; v != "true" {
		t.Errorf("transport_types.supported = %q, want %q", v, "true")
	}
	if v := res.Capabilities["mcp.capabilities.transport_types.confidence"]; v != confidenceInferred {
		t.Errorf("transport_types.confidence = %q, want %q", v, confidenceInferred)
	}
	if v := res.Capabilities["mcp.capabilities.transport_types.mechanism"]; v != "documented under 'Transport types'" {
		t.Errorf("transport_types.mechanism = %q", v)
	}
	if v := res.Capabilities["mcp.supported"]; v != "true" {
		t.Errorf("mcp.supported = %q, want %q", v, "true")
	}

	// oauth_support did not match (no "OAuth callback" landmark) → key absent.
	if _, ok := res.Capabilities["mcp.capabilities.oauth_support.supported"]; ok {
		t.Error("oauth_support.supported should not be emitted (no matching landmark)")
	}
}
