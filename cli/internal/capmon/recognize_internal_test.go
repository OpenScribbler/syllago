package capmon

import (
	"strings"
	"testing"
)

// TestRecognizeGoStruct_ContentTypeAgnostic proves the Phase 6 generalization:
// the helper works for any content type, not just skills. Uses a synthetic
// "widgets" content type with a "Widget." prefix and an identity key mapper.
func TestRecognizeGoStruct_ContentTypeAgnostic(t *testing.T) {
	opts := GoStructOptions{
		ContentType:    "widgets",
		StructPrefixes: []string{"Widget."},
		KeyMapper:      func(s string) string { return s },
	}
	fields := map[string]FieldValue{
		"Widget.Name":  {Value: "name"},
		"Widget.Color": {Value: "color"},
		// Non-Widget fields must not produce widget paths
		"Skill.Name": {Value: "name"},
	}
	result := recognizeGoStruct(fields, opts)

	checks := map[string]string{
		"widgets.supported":                     "true",
		"widgets.capabilities.name.supported":   "true",
		"widgets.capabilities.name.mechanism":   "yaml frontmatter key: name",
		"widgets.capabilities.name.confidence":  "confirmed",
		"widgets.capabilities.color.supported":  "true",
		"widgets.capabilities.color.confidence": "confirmed",
	}
	for key, want := range checks {
		if got := result[key]; got != want {
			t.Errorf("result[%q] = %q, want %q", key, got, want)
		}
	}
	if _, ok := result["skills.supported"]; ok {
		t.Error("Skill.* fields should not produce skills paths when prefix is Widget.")
	}
}

// TestRecognizeGoStruct_MechanismPrefixOverride proves the MechanismPrefix option
// overrides the default "yaml frontmatter key: " — needed for providers whose
// source format is JSON or TOML, not YAML.
func TestRecognizeGoStruct_MechanismPrefixOverride(t *testing.T) {
	opts := GoStructOptions{
		ContentType:     "skills",
		StructPrefixes:  []string{"Skill."},
		KeyMapper:       skillsKeyMapper,
		MechanismPrefix: "toml key: ",
	}
	fields := map[string]FieldValue{
		"Skill.Name": {Value: "name"},
	}
	result := recognizeGoStruct(fields, opts)
	if got, want := result["skills.capabilities.display_name.mechanism"], "toml key: name"; got != want {
		t.Errorf("mechanism = %q, want %q", got, want)
	}
}

// TestSkillsGoStructOptions_Preset locks in the preset values. If someone changes
// the preset accidentally, every skills recognizer output changes shape.
func TestSkillsGoStructOptions_Preset(t *testing.T) {
	opts := SkillsGoStructOptions()
	if opts.ContentType != "skills" {
		t.Errorf("ContentType = %q, want %q", opts.ContentType, "skills")
	}
	if len(opts.StructPrefixes) != 1 || opts.StructPrefixes[0] != "Skill." {
		t.Errorf("StructPrefixes = %v, want [\"Skill.\"]", opts.StructPrefixes)
	}
	// skillsKeyMapper translates "name" → "display_name"
	if got := opts.KeyMapper("name"); got != "display_name" {
		t.Errorf("KeyMapper(\"name\") = %q, want %q", got, "display_name")
	}
}

// TestGoStructOptions_MutualExclusion enforces the prefix-selection invariant:
// exactly one of StructPrefixes or PrefixMatcher must be set. Both-set and
// neither-set both panic with descriptive messages.
func TestGoStructOptions_MutualExclusion(t *testing.T) {
	cases := []struct {
		name      string
		opts      GoStructOptions
		wantPanic string
	}{
		{
			name: "both set",
			opts: GoStructOptions{
				ContentType:    "skills",
				StructPrefixes: []string{"Skill."},
				PrefixMatcher:  func(string) bool { return true },
				KeyMapper:      skillsKeyMapper,
			},
			wantPanic: "both StructPrefixes and PrefixMatcher",
		},
		{
			name: "neither set",
			opts: GoStructOptions{
				ContentType: "skills",
				KeyMapper:   skillsKeyMapper,
			},
			wantPanic: "neither StructPrefixes nor PrefixMatcher",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic, got none")
				}
				msg, ok := r.(string)
				if !ok {
					t.Fatalf("panic value not string: %T %v", r, r)
				}
				if !strings.Contains(msg, tc.wantPanic) {
					t.Errorf("panic message %q does not contain %q", msg, tc.wantPanic)
				}
			}()
			recognizeGoStruct(map[string]FieldValue{"Skill.Name": {Value: "name"}}, tc.opts)
		})
	}
}

// TestGoStructOptions_PrefixMatcher exercises the escape hatch: a recognizer
// configured with PrefixMatcher (no StructPrefixes) selects fields by arbitrary logic.
func TestGoStructOptions_PrefixMatcher(t *testing.T) {
	opts := GoStructOptions{
		ContentType:   "skills",
		PrefixMatcher: func(k string) bool { return strings.HasPrefix(k, "Skill.") || strings.HasPrefix(k, "MetaSkill.") },
		KeyMapper:     skillsKeyMapper,
	}
	fields := map[string]FieldValue{
		"Skill.Name":     {Value: "name"},
		"MetaSkill.Name": {Value: "name"},
		"Other.Name":     {Value: "name"},
	}
	result := recognizeGoStruct(fields, opts)
	if result["skills.capabilities.display_name.supported"] != "true" {
		t.Error("expected display_name.supported = true via PrefixMatcher")
	}
}

// TestGoStructOptions_MultiplePrefixes exercises StructPrefixes with several entries.
// Used by codex (PR3) where 5 distinct Rust struct names are accepted.
func TestGoStructOptions_MultiplePrefixes(t *testing.T) {
	opts := GoStructOptions{
		ContentType:    "skills",
		StructPrefixes: []string{"SkillA.", "SkillB."},
		KeyMapper:      skillsKeyMapper,
	}
	fields := map[string]FieldValue{
		"SkillA.Name":        {Value: "name"},
		"SkillB.Description": {Value: "description"},
		"SkillC.License":     {Value: "license"},
	}
	result := recognizeGoStruct(fields, opts)
	if result["skills.capabilities.display_name.supported"] != "true" {
		t.Error("expected display_name from SkillA.")
	}
	if result["skills.capabilities.description.supported"] != "true" {
		t.Error("expected description from SkillB.")
	}
	if _, has := result["skills.capabilities.license.supported"]; has {
		t.Error("did not expect license from SkillC. (not in StructPrefixes)")
	}
}
