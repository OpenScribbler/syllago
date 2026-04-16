package capmon_test

import (
	"strings"
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

// TestRecognizeGoStructBatch_WithSkillFields verifies each Agent Skills GoStruct
// provider produces the standard capability set when Skill.* fields are present.
// Providers whose real-world extraction does not surface Skill.* fields (non-Go
// sources) will emit empty output in production; this test proves the plumbing
// so those providers light up automatically when upstream extractors improve.
//
// codex, claude-code, amp, cline, copilot-cli, factory-droid, kiro are excluded
// from this batch. codex uses a multi-struct allow-list; the rest use landmark
// recognition with no Skill.* field input. Each has its own dedicated test file.
func TestRecognizeGoStructBatch_WithSkillFields(t *testing.T) {
	providers := []struct {
		slug     string
		filename string // substring expected in canonical_filename.mechanism
	}{
		{"pi", "SKILL.md"},
	}
	fields := map[string]capmon.FieldValue{
		"Skill.Name":          {Value: "name"},
		"Skill.Description":   {Value: "description"},
		"Skill.License":       {Value: "license"},
		"Skill.Compatibility": {Value: "compatibility"},
		"Skill.MetadataMap":   {Value: "metadata"},
	}
	for _, p := range providers {
		t.Run(p.slug, func(t *testing.T) {
			result := capmon.RecognizeContentTypeDotPaths(p.slug, fields)
			if result["skills.supported"] != "true" {
				t.Errorf("%s: skills.supported = %q, want true", p.slug, result["skills.supported"])
			}
			if result["skills.capabilities.display_name.supported"] != "true" {
				t.Errorf("%s: display_name.supported missing", p.slug)
			}
			if result["skills.capabilities.canonical_filename.supported"] != "true" {
				t.Errorf("%s: canonical_filename.supported missing", p.slug)
			}
			mech := result["skills.capabilities.canonical_filename.mechanism"]
			if !strings.Contains(mech, p.filename) {
				t.Errorf("%s: canonical_filename.mechanism = %q, want containing %q", p.slug, mech, p.filename)
			}
			if result["skills.capabilities.project_scope.supported"] != "true" {
				t.Errorf("%s: project_scope.supported missing", p.slug)
			}
		})
	}
}

// TestRecognizeGoStructBatch_EmptyFields verifies the GoStruct providers
// return an empty map when no Skill.* fields are present (same guard as crush).
// codex, claude-code, amp, cline, copilot-cli, factory-droid, kiro excluded —
// they use a different mechanism and have dedicated NoLandmarks tests in their own files.
func TestRecognizeGoStructBatch_EmptyFields(t *testing.T) {
	for _, slug := range []string{"pi"} {
		result := capmon.RecognizeContentTypeDotPaths(slug, map[string]capmon.FieldValue{})
		if len(result) != 0 {
			t.Errorf("%s: expected empty result, got %v", slug, result)
		}
	}
}

// TestRecognizeCustomBatch_Supported covers windsurf, gemini-cli, opencode.
// Each uses the GoStruct pattern under the hood plus provider-specific scope.
func TestRecognizeCustomBatch_Supported(t *testing.T) {
	providers := []struct {
		slug               string
		projectScopeSubstr string
	}{
		{"windsurf", ".windsurf/skills"},
		{"gemini-cli", ".gemini/skills"},
		{"opencode", ".opencode/skill"},
	}
	fields := map[string]capmon.FieldValue{
		"Skill.Name":        {Value: "name"},
		"Skill.Description": {Value: "description"},
	}
	for _, p := range providers {
		t.Run(p.slug, func(t *testing.T) {
			result := capmon.RecognizeContentTypeDotPaths(p.slug, fields)
			if result["skills.supported"] != "true" {
				t.Errorf("%s: skills.supported = %q, want true", p.slug, result["skills.supported"])
			}
			mech := result["skills.capabilities.project_scope.mechanism"]
			if !strings.Contains(mech, p.projectScopeSubstr) {
				t.Errorf("%s: project_scope.mechanism = %q, want containing %q", p.slug, mech, p.projectScopeSubstr)
			}
			if result["skills.capabilities.canonical_filename.supported"] != "true" {
				t.Errorf("%s: canonical_filename.supported missing", p.slug)
			}
		})
	}
}

// TestRecognizeCustomBatch_Unsupported verifies cursor and zed return empty
// even when given Skill.* fields — they do not support Agent Skills per FormatDoc.
func TestRecognizeCustomBatch_Unsupported(t *testing.T) {
	fields := map[string]capmon.FieldValue{
		"Skill.Name":        {Value: "name"},
		"Skill.Description": {Value: "description"},
	}
	for _, slug := range []string{"cursor", "zed"} {
		result := capmon.RecognizeContentTypeDotPaths(slug, fields)
		if len(result) != 0 {
			t.Errorf("%s: unsupported provider should return empty map, got %v", slug, result)
		}
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
