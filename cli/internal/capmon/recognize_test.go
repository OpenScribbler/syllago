package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestRecognizeContentTypeDotPaths_SkillGoStruct(t *testing.T) {
	// NOTE: Until Phase 6 wires the crush recognizer, this call returns an empty map.
	// The GoStruct recognizer is no longer called from dispatch — crush's init() registration
	// will call it internally in recognize_crush.go.
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

	// Until Phase 6 wires the crush recognizer, the result is empty.
	// The GoStruct recognizer is no longer called from dispatch.
	if len(result) != 0 {
		t.Errorf("expected empty result (crush recognizer not yet registered), got %v", result)
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
