package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestRecognizeContentTypeDotPaths_SkillGoStruct(t *testing.T) {
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

	result := capmon.RecognizeContentTypeDotPaths(fields)

	// skills.supported should be set
	if result["skills.supported"] != "true" {
		t.Errorf("skills.supported: got %q, want %q", result["skills.supported"], "true")
	}

	// Each Skill field should generate two dot-paths
	cases := []struct{ key, wantVal string }{
		{"skills.capabilities.frontmatter_name.supported", "true"},
		{"skills.capabilities.frontmatter_name.mechanism", "yaml key: name"},
		{"skills.capabilities.frontmatter_description.supported", "true"},
		{"skills.capabilities.frontmatter_description.mechanism", "yaml key: description"},
		{"skills.capabilities.frontmatter_license.supported", "true"},
		{"skills.capabilities.frontmatter_license.mechanism", "yaml key: license"},
	}
	for _, tc := range cases {
		got, ok := result[tc.key]
		if !ok {
			t.Errorf("missing key %q in result", tc.key)
			continue
		}
		if got != tc.wantVal {
			t.Errorf("key %q: got %q, want %q", tc.key, got, tc.wantVal)
		}
	}

	// Non-Skill keys should not appear as skills capabilities
	for k := range result {
		if k == "skills.capabilities.frontmatter_64.mechanism" {
			t.Error("MaxNameLength value '64' should not generate a frontmatter capability")
		}
	}
}

func TestRecognizeContentTypeDotPaths_EmptyFields(t *testing.T) {
	result := capmon.RecognizeContentTypeDotPaths(map[string]capmon.FieldValue{})
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
	result := capmon.RecognizeContentTypeDotPaths(fields)
	for k := range result {
		if k == "skills.supported" {
			t.Error("should not produce skills.supported without Skill struct fields")
		}
	}
}
