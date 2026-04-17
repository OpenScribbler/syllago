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

// realCodexRulesLandmarks is the snapshot of headings extracted from codex's
// AGENTS.md spec doc (.capmon-cache/codex/rules.0/extracted.json) as of
// 2026-04-16. The cached doc is intentionally short — it redirects to the
// developers.openai.com AGENTS.md spec which was not cached. rules.1 is
// codex's own AGENTS.md instance file (their internal dev rules) and
// intentionally NOT used as evidence.
var realCodexRulesLandmarks = []string{
	"AGENTS.md",
	"Hierarchical agents message",
}

// TestRecognizeCodex_RealRulesLandmarks proves rules recognition on the
// minimal landmark set codex's spec doc provides. Codex supports
// activation_mode.always_on, cross_provider_recognition.agents_md, and
// hierarchical_loading. file_imports and auto_memory are intentionally absent.
func TestRecognizeCodex_RealRulesLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("codex", capmon.RecognitionContext{
		Provider:  "codex",
		Format:    "markdown",
		Landmarks: realCodexRulesLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["rules.supported"] != "true" {
		t.Error("rules.supported missing")
	}
	rulesInferred := []string{
		"activation_mode.always_on",
		"cross_provider_recognition.agents_md",
		"hierarchical_loading",
	}
	for _, c := range rulesInferred {
		key := "rules.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["rules.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("rules.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"rules.capabilities.file_imports.supported",
		"rules.capabilities.auto_memory.supported",
		"rules.capabilities.activation_mode.manual.supported",
		"rules.capabilities.activation_mode.model_decision.supported",
		"rules.capabilities.activation_mode.frontmatter_globs.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for codex", absent)
		}
	}
}

// TestRecognizeCodex_RulesAnchorsMissing proves the anchor-missing guardrail.
// Stripping "Hierarchical agents message" — one of the required anchors —
// suppresses recognition.
func TestRecognizeCodex_RulesAnchorsMissing(t *testing.T) {
	result := capmon.RecognizeWithContext("codex", capmon.RecognitionContext{
		Provider:  "codex",
		Format:    "markdown",
		Landmarks: []string{"AGENTS.md"},
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d: %v", len(result.Capabilities), result.Capabilities)
	}
}

// realCodexHooksLandmarks is the snapshot of type names extracted from codex's
// 16 hooks cache sources (.capmon-cache/codex/hooks.{0..15}/extracted.json) as
// of 2026-04-16. Sources span:
//   - hooks.0-9 : JSON Schema files (5 events × input + output schemas)
//   - hooks.10-13: TypeScript v2 protocol enums (HookEventName, HookHandlerType,
//     HookExecutionMode, HookScope)
//   - hooks.14-15: Rust source (engine config + types)
//
// Type names are emitted as landmarks by the JSON Schema and TypeScript
// extractors. Update this fixture when upstream codex evolves.
var realCodexHooksLandmarks = []string{
	// JSON Schema event input/output type names
	"PreToolUseToolInput",
	"PreToolUseDecisionWire",
	"PreToolUseHookSpecificOutputWire",
	"PreToolUsePermissionDecisionWire",
	"PostToolUseToolInput",
	"BlockDecisionWire",
	"PostToolUseHookSpecificOutputWire",
	"SessionStartHookSpecificOutputWire",
	"UserPromptSubmitHookSpecificOutputWire",
	"HookEventNameWire",
	"NullableString",
	// TypeScript protocol enum names
	"HookEventName",
	"HookHandlerType",
	"HookExecutionMode",
	"HookScope",
	// Rust engine + types struct names
	"HooksFile",
	"HookEvents",
	"MatcherGroup",
	"HookHandlerConfig",
	"HookFn",
	"HookResult",
	"HookResponse",
	"Hook",
	"HookPayload",
	"HookEventAfterAgent",
	"HookToolKind",
	"HookToolInputLocalShell",
	"HookToolInput",
	"HookEventAfterToolUse",
	"HookEvent",
}

// TestRecognizeCodex_RealHooksLandmarks proves hooks recognition fires against
// the merged 16-source landmark snapshot. Per the curated format YAML
// (docs/provider-formats/codex.yaml), 8 of the 9 canonical hooks keys are
// supported — only json_io_protocol is curated as unsupported (codex hooks use
// exit codes + stdout text, not structured JSON I/O).
func TestRecognizeCodex_RealHooksLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("codex", capmon.RecognitionContext{
		Provider:  "codex",
		Format:    "markdown",
		Landmarks: realCodexHooksLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["hooks.supported"] != "true" {
		t.Error("hooks.supported missing")
	}
	hooksInferred := []string{
		"handler_types",
		"matcher_patterns",
		"decision_control",
		"input_modification",
		"async_execution",
		"hook_scopes",
		"context_injection",
		"permission_control",
	}
	for _, c := range hooksInferred {
		key := "hooks.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["hooks.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("hooks.%s.confidence = %q, want inferred", c, got)
		}
	}
	if _, has := caps["hooks.capabilities.json_io_protocol.supported"]; has {
		t.Error("hooks.capabilities.json_io_protocol should NOT be present for codex (curated as unsupported)")
	}
}

// TestRecognizeCodex_HooksAnchorsMissing proves the required-anchor guard
// suppresses hooks emission when "HookScope" is absent — without the guard,
// substring matchers like "HookHandlerType" would fire on partial cache states.
func TestRecognizeCodex_HooksAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCodexHooksLandmarks))
	for _, lm := range realCodexHooksLandmarks {
		if lm == "HookScope" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("codex", capmon.RecognitionContext{
		Provider:  "codex",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["hooks.supported"]; has {
		t.Error("hooks.supported should NOT be present when 'HookScope' anchor is missing")
	}
}
