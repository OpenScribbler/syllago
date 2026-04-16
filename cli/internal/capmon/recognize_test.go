package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestRecognizeContentTypeDotPaths_SkillGoStruct(t *testing.T) {
	// Phase 6: crush recognizer is now wired and calls recognizeGoStruct(SkillsGoStructOptions()) internally.
	fields := map[string]capmon.FieldValue{
		"Skill.Name": {
			Value:     "name",
			ValueHash: "sha256:abc",
		},
		"Skill.Description": {
			Value:     "description",
			ValueHash: "sha256:def",
		},
		"Skill.License": {
			Value:     "license",
			ValueHash: "sha256:ghi",
		},
		// Non-Skill fields should not generate skills paths
		"MaxNameLength": {
			Value:     "64",
			ValueHash: "sha256:jkl",
		},
		"SkillFileName": {
			Value:     "SKILL.md",
			ValueHash: "sha256:mno",
		},
	}

	result := capmon.RecognizeContentTypeDotPaths("crush", fields)

	// Phase 6: crush recognizer now calls recognizeGoStruct, so Skill.* fields produce paths.
	if result["skills.capabilities.display_name.supported"] != "true" {
		t.Errorf("expected skills.capabilities.display_name.supported=true, got %q", result["skills.capabilities.display_name.supported"])
	}
	if result["skills.capabilities.description.supported"] != "true" {
		t.Errorf("expected skills.capabilities.description.supported=true, got %q", result["skills.capabilities.description.supported"])
	}
	if result["skills.supported"] != "true" {
		t.Errorf("expected skills.supported=true, got %q", result["skills.supported"])
	}
}

func TestRecognizeCrushSkills(t *testing.T) {
	fields := map[string]capmon.FieldValue{
		"Skill.Name":          {Value: "name"},
		"Skill.Description":   {Value: "description"},
		"Skill.License":       {Value: "license"},
		"Skill.Compatibility": {Value: "compatibility"},
		"Skill.MetadataMap":   {Value: "metadata"},
	}

	result := capmon.RecognizeContentTypeDotPaths("crush", fields)

	checks := map[string]string{
		"skills.capabilities.display_name.supported":  "true",
		"skills.capabilities.display_name.confidence": "confirmed",
		"skills.capabilities.description.supported":   "true",
		"skills.capabilities.license.confidence":      "confirmed",
		"skills.supported":                            "true",
	}
	for key, want := range checks {
		if got := result[key]; got != want {
			t.Errorf("crush: result[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestRecognizeRooCodeSkills(t *testing.T) {
	fields := map[string]capmon.FieldValue{
		"Skill.Name":          {Value: "name"},
		"Skill.Description":   {Value: "description"},
		"Skill.License":       {Value: "license"},
		"Skill.Compatibility": {Value: "compatibility"},
		"Skill.MetadataMap":   {Value: "metadata"},
	}

	result := capmon.RecognizeContentTypeDotPaths("roo-code", fields)

	checks := map[string]string{
		"skills.capabilities.display_name.supported":  "true",
		"skills.capabilities.display_name.confidence": "confirmed",
		"skills.capabilities.description.supported":   "true",
		"skills.capabilities.license.confidence":      "confirmed",
		"skills.supported":                            "true",
	}
	for key, want := range checks {
		if got := result[key]; got != want {
			t.Errorf("roo-code: result[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestRecognizeContentTypeDotPaths_EmptyFields(t *testing.T) {
	result := capmon.RecognizeContentTypeDotPaths("crush", map[string]capmon.FieldValue{})
	if len(result) != 0 {
		t.Errorf("empty fields should produce empty result, got %v", result)
	}
}

func TestRecognizeContentTypeDotPaths_NoSkillStruct(t *testing.T) {
	// Consts only, no Skill.* keys — should not produce skills entries
	fields := map[string]capmon.FieldValue{
		"MaxNameLength": {Value: "64"},
		"SomeConst":     {Value: "value"},
	}
	result := capmon.RecognizeContentTypeDotPaths("crush", fields)
	for k := range result {
		if k == "skills.supported" {
			t.Error("should not produce skills.supported without Skill struct fields")
		}
	}
}
