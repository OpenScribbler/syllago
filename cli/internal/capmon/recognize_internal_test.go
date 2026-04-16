package capmon

import "testing"

// TestRecognizeGoStruct_ContentTypeAgnostic proves the Phase 6 generalization:
// the helper works for any content type, not just skills. Uses a synthetic
// "widgets" content type with a "Widget." prefix and an identity key mapper.
func TestRecognizeGoStruct_ContentTypeAgnostic(t *testing.T) {
	opts := GoStructOptions{
		ContentType:  "widgets",
		StructPrefix: "Widget.",
		KeyMapper:    func(s string) string { return s },
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
		StructPrefix:    "Skill.",
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
	if opts.StructPrefix != "Skill." {
		t.Errorf("StructPrefix = %q, want %q", opts.StructPrefix, "Skill.")
	}
	// skillsKeyMapper translates "name" → "display_name"
	if got := opts.KeyMapper("name"); got != "display_name" {
		t.Errorf("KeyMapper(\"name\") = %q, want %q", got, "display_name")
	}
}
