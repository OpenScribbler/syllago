package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realGeminiCliRulesLandmarks is a snapshot of headings extracted from
// .capmon-cache/gemini-cli/rules.0/extracted.json (gemini-md.md) as of
// 2026-04-16. Update when the doc evolves.
var realGeminiCliRulesLandmarks = []string{
	"Provide context with GEMINI.md files",
	"Understand the context hierarchy",
	"Example GEMINI.md file",
	"Manage context with the /memory command",
	"Modularize context with imports",
	"Customize the context file name",
	"Next steps",
}

func TestRecognizeGeminiCli_RealRulesLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: realGeminiCliRulesLandmarks,
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
		"file_imports",
		"auto_memory",
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
	// Per seeder spec, gemini-cli does NOT document AGENTS.md or other
	// foreign-format recognition.
	if _, has := caps["rules.capabilities.cross_provider_recognition.agents_md.supported"]; has {
		t.Error("cross_provider_recognition.agents_md should NOT be present for gemini-cli")
	}
}

func TestRecognizeGeminiCli_AnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realGeminiCliRulesLandmarks))
	for _, lm := range realGeminiCliRulesLandmarks {
		if lm == "Understand the context hierarchy" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}

func TestRecognizeGeminiCli_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider: "gemini-cli",
		Format:   "markdown",
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}
