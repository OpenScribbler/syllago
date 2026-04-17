package capmon_test

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// TestRecognizeCodex_MultiStructAllowList exercises the codex 5-prefix allow-list:
// fields under SkillMetadata/Policy/Interface/Dependencies/ToolDependency emit
// capabilities; fields under SkillError/LoadOutcome/FileSystemsByPath do not.
//
// Reproduces the bug from .panels/recognizer-api-evolution/seed.md:
// > codex — BROKEN. Rust source, 8 structs. 32 fields total. StructPrefix "Skill."
// > matches all 32 — but SkillError{message,path} and SkillLoadOutcome{...} are
// > runtime-error types, not skill-format capabilities.
func TestRecognizeCodex_MultiStructAllowList(t *testing.T) {
	fields := map[string]capmon.FieldValue{
		// Included structs — should produce capabilities
		"SkillMetadata.Name":         {Value: "name"},
		"SkillMetadata.Description":  {Value: "description"},
		"SkillPolicy.Allow":          {Value: "allow"},
		"SkillInterface.Tool":        {Value: "tool"},
		"SkillDependencies.Required": {Value: "required"},
		"SkillToolDependency.Name":   {Value: "name"},
		// Excluded structs — MUST NOT produce capabilities
		"SkillError.Message":            {Value: "message"},
		"SkillError.Path":               {Value: "path"},
		"SkillLoadOutcome.DisabledPath": {Value: "disabled_paths"},
		"SkillLoadOutcome.Errors":       {Value: "errors"},
		"SkillFileSystemsByPath.Path":   {Value: "path"},
	}

	result := capmon.RecognizeContentTypeDotPaths("codex", fields)

	// Sanity: skills are recognized and the 5 included structs contributed
	if result["skills.supported"] != "true" {
		t.Fatal("expected skills.supported = true")
	}
	if result["skills.capabilities.display_name.supported"] != "true" {
		t.Error("expected display_name from SkillMetadata.Name")
	}
	if result["skills.capabilities.description.supported"] != "true" {
		t.Error("expected description from SkillMetadata.Description")
	}

	// Critical: excluded struct field VALUES must not appear as capability keys.
	// SkillError.Message has Value="message" — if it leaked through, we'd see
	// skills.capabilities.message.* which is wrong (message is not a skill capability).
	for k := range result {
		// Any capability key derived from an excluded struct's Value field would
		// surface here; map them back to source field names that uniquely identify
		// excluded prefixes.
		for _, leaked := range []string{
			"skills.capabilities.message.",        // SkillError.Message
			"skills.capabilities.path.",           // SkillError.Path / SkillFileSystemsByPath.Path
			"skills.capabilities.disabled_paths.", // SkillLoadOutcome.DisabledPath
			"skills.capabilities.errors.",         // SkillLoadOutcome.Errors
		} {
			if strings.HasPrefix(k, leaked) {
				t.Errorf("leaked capability from excluded struct: %q starts with %q", k, leaked)
			}
		}
	}
}

// TestRecognizeCodex_OnlyExcludedStructs proves that a payload containing
// ONLY runtime-type fields produces zero capabilities — the recognizer reports
// "not_evaluated" status (no signal). This is the regression guard: if someone
// reverts to a single "Skill." prefix, this test catches it because runtime
// fields would now produce capability paths.
func TestRecognizeCodex_OnlyExcludedStructs(t *testing.T) {
	fields := map[string]capmon.FieldValue{
		"SkillError.Message":            {Value: "message"},
		"SkillError.Path":               {Value: "path"},
		"SkillLoadOutcome.DisabledPath": {Value: "disabled_paths"},
		"SkillFileSystemsByPath.Path":   {Value: "path"},
	}

	result := capmon.RecognizeContentTypeDotPaths("codex", fields)

	if len(result) != 0 {
		t.Errorf("excluded-only fields produced %d capability paths, want 0: %v", len(result), result)
	}
}
