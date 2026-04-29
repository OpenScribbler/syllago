package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSkillsContentType(t *testing.T) {
	if SkillsContentType != "skills" {
		t.Errorf("SkillsContentType = %q, want %q", SkillsContentType, "skills")
	}
}

func TestCanonicalSkillsKeys_MatchesCanonicalKeysYAML(t *testing.T) {
	// Drift guard: the Go-side CanonicalSkillsKeys list MUST stay in sync with
	// the skills section of docs/spec/canonical-keys.yaml. If they diverge, the
	// validator (formatdoc_validate.go) and recognizers (recognize_*.go) will
	// disagree about which keys are legal.
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
	skillsKeys, ok := doc.ContentTypes["skills"]
	if !ok {
		t.Fatal("canonical-keys.yaml has no content_types.skills section")
	}

	yamlSet := make(map[string]struct{}, len(skillsKeys))
	for k := range skillsKeys {
		yamlSet[k] = struct{}{}
	}
	goSet := make(map[string]struct{}, len(CanonicalSkillsKeys))
	for _, k := range CanonicalSkillsKeys {
		goSet[k] = struct{}{}
	}

	for k := range yamlSet {
		if _, ok := goSet[k]; !ok {
			t.Errorf("canonical-keys.yaml declares skills key %q but Go CanonicalSkillsKeys does not", k)
		}
	}
	for k := range goSet {
		if _, ok := yamlSet[k]; !ok {
			t.Errorf("Go CanonicalSkillsKeys declares %q but canonical-keys.yaml does not", k)
		}
	}
}

func TestIsCanonicalSkillsKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"display_name", true},
		{"description", true},
		{"license", true},
		{"compatibility", true},
		{"metadata_map", true},
		{"disable_model_invocation", true},
		{"user_invocable", true},
		{"version", true},
		{"project_scope", true},
		{"global_scope", true},
		{"shared_scope", true},
		{"canonical_filename", true},
		{"custom_filename", true},
		{"unknown_key", false},
		{"", false},
		{"DISPLAY_NAME", false}, // case-sensitive
		{"display-name", false}, // hyphen vs underscore
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := IsCanonicalSkillsKey(c.key); got != c.want {
				t.Errorf("IsCanonicalSkillsKey(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

func TestSkillsLandmarkOptions_SetsContentType(t *testing.T) {
	opts := SkillsLandmarkOptions()
	if opts.ContentType != "skills" {
		t.Errorf("ContentType = %q, want %q", opts.ContentType, "skills")
	}
	if len(opts.Patterns) != 0 {
		t.Errorf("Patterns len = %d, want 0", len(opts.Patterns))
	}
}

func TestSkillsLandmarkOptions_PreservesPatterns(t *testing.T) {
	pats := []LandmarkPattern{
		{Capability: "display_name", Mechanism: "m1", Matchers: []StringMatcher{{Value: "x"}}},
		{Capability: "version", Mechanism: "m2", Matchers: []StringMatcher{{Value: "y"}}},
	}
	opts := SkillsLandmarkOptions(pats...)
	if len(opts.Patterns) != 2 {
		t.Fatalf("Patterns len = %d, want 2", len(opts.Patterns))
	}
	if opts.Patterns[0].Capability != "display_name" || opts.Patterns[1].Capability != "version" {
		t.Errorf("patterns out of order: %v", opts.Patterns)
	}
}

func TestSkillsLandmarkOptions_AcceptsEmptyCapability(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SkillsLandmarkOptions panicked on empty Capability: %v", r)
		}
	}()
	_ = SkillsLandmarkOptions(LandmarkPattern{Capability: "", Matchers: []StringMatcher{{Value: "x"}}})
}

func TestSkillsLandmarkOptions_AcceptsNestedCanonicalKey(t *testing.T) {
	// Object-typed canonical keys (compatibility, metadata_map) may emit nested
	// sub-segments. Validation only checks the first segment.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SkillsLandmarkOptions panicked on nested canonical key: %v", r)
		}
	}()
	_ = SkillsLandmarkOptions(
		LandmarkPattern{Capability: "compatibility.platforms", Mechanism: "m"},
		LandmarkPattern{Capability: "metadata_map.custom", Mechanism: "m"},
	)
}

func TestSkillsLandmarkOptions_PanicsOnUnknownKey(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on unknown canonical skills key, got none")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "unknown canonical skills key") {
			t.Errorf("panic message = %q, want substring %q", msg, "unknown canonical skills key")
		}
		if !strings.Contains(msg, "bogus_key") {
			t.Errorf("panic message = %q, should mention the offending key", msg)
		}
	}()
	_ = SkillsLandmarkOptions(LandmarkPattern{Capability: "bogus_key", Mechanism: "m"})
}

func TestSkillsLandmarkOptions_PanicsOnUnknownNestedHead(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when nested capability head is unknown")
		}
	}()
	_ = SkillsLandmarkOptions(LandmarkPattern{Capability: "bogus_root.sub_field", Mechanism: "m"})
}

func TestSkillsLandmarkPattern_BuildsExpectedShape(t *testing.T) {
	pat := SkillsLandmarkPattern("display_name", "Display name", "documented under 'Display name' heading", nil)
	if pat.Capability != "display_name" {
		t.Errorf("Capability = %q, want %q", pat.Capability, "display_name")
	}
	if pat.Mechanism != "documented under 'Display name' heading" {
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
	if m.Value != "Display name" {
		t.Errorf("Matcher.Value = %q", m.Value)
	}
	if !m.CaseInsensitive {
		t.Error("Matcher.CaseInsensitive = false, want true")
	}
}

func TestSkillsLandmarkPattern_PassesThroughRequired(t *testing.T) {
	required := []StringMatcher{{Kind: "substring", Value: "Skills", CaseInsensitive: true}}
	pat := SkillsLandmarkPattern("project_scope", ".skills/", "project-scope path", required)
	if len(pat.Required) != 1 || pat.Required[0].Value != "Skills" {
		t.Errorf("Required = %+v", pat.Required)
	}
}

func TestSkillsHelpers_ComposeWithRecognizeLandmarks(t *testing.T) {
	opts := SkillsLandmarkOptions(
		SkillsLandmarkPattern("display_name", "Display name", "documented under 'Display name'", nil),
		SkillsLandmarkPattern("version", "Version", "documented under 'Version'", nil),
	)
	ctx := RecognitionContext{
		Provider:  "test-provider",
		Landmarks: []string{"How skills work", "Display name", "Best practices"},
	}
	res := recognizeLandmarks(ctx, opts)

	if res.Status != StatusRecognized {
		t.Errorf("Status = %q, want %q", res.Status, StatusRecognized)
	}

	if v := res.Capabilities["skills.capabilities.display_name.supported"]; v != "true" {
		t.Errorf("display_name.supported = %q, want %q", v, "true")
	}
	if v := res.Capabilities["skills.capabilities.display_name.confidence"]; v != confidenceInferred {
		t.Errorf("display_name.confidence = %q, want %q", v, confidenceInferred)
	}
	if v := res.Capabilities["skills.capabilities.display_name.mechanism"]; v != "documented under 'Display name'" {
		t.Errorf("display_name.mechanism = %q", v)
	}
	if v := res.Capabilities["skills.supported"]; v != "true" {
		t.Errorf("skills.supported = %q, want %q", v, "true")
	}

	// version did not match → key absent.
	if _, ok := res.Capabilities["skills.capabilities.version.supported"]; ok {
		t.Error("version.supported should not be emitted (no matching landmark)")
	}
}
