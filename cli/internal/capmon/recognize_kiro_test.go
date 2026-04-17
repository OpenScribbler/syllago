package capmon_test

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realKiroSkillsLandmarks is a snapshot of the headings extracted from kiro's
// powers doc (.capmon-cache/kiro/skills.0/extracted.json) as of 2026-04-16.
// Includes the AWS docs cookie-banner boilerplate that prefixes every kiro
// page — these landmarks are noise but the recognizer must ignore them.
var realKiroSkillsLandmarks = []string{
	"Select your cookie preferences",
	"Customize cookie preferences",
	"Essential", "Performance", "Functional", "Advertising",
	"Your privacy choices",
	"Unable to save cookie preferences",
	"Create powers",
	"What you need",
	"Creating POWER.md",
	"Frontmatter: When to activate",
	"Onboarding instructions",
	"Steering instructions",
	"Adding MCP servers",
	"Directory structure",
	"Testing locally",
	"Sharing your power",
	"Examples",
}

// realKiroNonSkillsLandmarks is a sample drawn from kiro's non-skills,
// non-rules, non-hooks content-type docs (agents, mcp). Required anchors for
// the SKILLS, RULES, and HOOKS recognizers must NOT match any of these.
// Partial hooks landmarks ("Hooks", "What are agent hooks?") are included to
// verify the hooks required-anchor guard suppresses cleanly when "Setting up
// agent hooks" is absent.
var realKiroNonSkillsLandmarks = []string{
	"Select your cookie preferences", "Essential", "Performance",
	"Agent configuration reference", "Name field", "Description field",
	"Hooks", "What are agent hooks?", "How agent hooks work",
	"Configuration", "Configuration file structure", "Remote server", "Local server",
}

// realKiroHooksLandmarks is a snapshot of the headings extracted from kiro's
// agent hooks doc (.capmon-cache/kiro/hooks.0/extracted.json) as of 2026-04-16.
// Includes the AWS docs cookie-banner boilerplate.
var realKiroHooksLandmarks = []string{
	"Select your cookie preferences",
	"Customize cookie preferences",
	"Essential", "Performance", "Functional", "Advertising",
	"Your privacy choices",
	"Unable to save cookie preferences",
	"Hooks",
	"What are agent hooks?",
	"How agent hooks work",
	"Setting up agent hooks",
	"Creating a hook",
	"Ask Kiro to create a hook",
	"Manually create a hook",
	"Next steps",
}

// realKiroRulesLandmarks is a snapshot of kiro's steering doc landmarks
// (.capmon-cache/kiro/rules.0/extracted.json). Includes the cookie-banner
// boilerplate for false-positive checking, plus the substantive Steering
// section + Inclusion modes + Agents.md fallback.
var realKiroRulesLandmarks = []string{
	"Select your cookie preferences",
	"Customize cookie preferences",
	"Essential", "Performance", "Functional", "Advertising",
	"Your privacy choices",
	"Unable to save cookie preferences",
	"Steering",
	"What is steering?",
	"Key benefits",
	"Steering file scope",
	"Workspace steering",
	"Global steering",
	"Team steering",
	"Foundational steering files",
	"Creating custom steering files",
	"Agents.md",
	"Inclusion modes",
	"Always included (default)",
	"Conditional inclusion",
	"Manual inclusion",
	"Auto inclusion",
	"File references",
	"Best practices",
	"Common steering file strategies",
	"Related documentation",
}

func TestRecognizeKiro_RealLandmarks(t *testing.T) {
	merged := append([]string{}, realKiroSkillsLandmarks...)
	merged = append(merged, realKiroRulesLandmarks...)
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: merged,
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	inferred := []string{
		"frontmatter", "onboarding_instructions", "steering_instructions",
		"mcp_integration", "directory_structure", "testing", "sharing",
	}
	for _, c := range inferred {
		if caps["skills.capabilities."+c+".supported"] != "true" {
			t.Errorf("%s.supported missing", c)
		}
		if caps["skills.capabilities."+c+".confidence"] != "inferred" {
			t.Errorf("%s.confidence = %q, want inferred", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
	// Kiro has project_scope and canonical_filename, but NOT global_scope
	// (powers install via UI panel, no fixed filesystem path).
	for _, c := range []string{"project_scope", "canonical_filename"} {
		if caps["skills.capabilities."+c+".confidence"] != "confirmed" {
			t.Errorf("%s.confidence = %q, want confirmed", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
	if _, has := caps["skills.capabilities.global_scope.supported"]; has {
		t.Error("global_scope should NOT be present for kiro (no global filesystem path)")
	}

	// Rules content type
	if caps["rules.supported"] != "true" {
		t.Error("rules.supported missing")
	}
	rulesInferred := []string{
		"activation_mode.always_on",
		"activation_mode.frontmatter_globs",
		"activation_mode.manual",
		"activation_mode.model_decision",
		"file_imports",
		"cross_provider_recognition.agents_md",
		"hierarchical_loading",
	}
	for _, c := range rulesInferred {
		key := "rules.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
	}
	// auto_memory must NOT be emitted — kiro has no auto-memory feature
	if _, has := caps["rules.capabilities.auto_memory.supported"]; has {
		t.Error("rules.capabilities.auto_memory should NOT be present for kiro")
	}

	// Sanity: the cookie banner landmarks should not have produced any
	// capabilities — confirms no accidental substring match.
	for k := range caps {
		for _, bad := range []string{"cookie", "essential", "performance", "advertising"} {
			if strings.Contains(k, bad) {
				t.Errorf("capability %q appears derived from cookie banner noise", k)
			}
		}
	}
}

// TestRecognizeKiro_NonSkillsLandmarks proves the multi-source false-positive
// guardrail: kiro's agents/hooks/mcp/rules landmarks (with shared cookie
// banner) must NOT trigger skills recognition.
func TestRecognizeKiro_NonSkillsLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: realKiroNonSkillsLandmarks,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d: %v",
			len(result.Capabilities), result.Capabilities)
	}
}

func TestRecognizeKiro_AnchorsMissing(t *testing.T) {
	// Strip "Creating POWER.md" — required anchor.
	mutated := []string{}
	for _, lm := range realKiroSkillsLandmarks {
		if lm == "Creating POWER.md" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

func TestRecognizeKiro_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{Provider: "kiro", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}

// realKiroMcpLandmarks is a snapshot of the headings extracted from kiro's
// MCP configuration doc (.capmon-cache/kiro/mcp.0/extracted.json) as of
// 2026-04-16. Includes the AWS docs cookie-banner boilerplate.
var realKiroMcpLandmarks = []string{
	"Select your cookie preferences",
	"Customize cookie preferences",
	"Essential", "Performance", "Functional", "Advertising",
	"Your privacy choices",
	"Unable to save cookie preferences",
	"Configuration",
	"Configuration file structure",
	"Configuration properties",
	"Remote server",
	"Local server",
	"Configuration locations",
	"Creating configuration files",
	"Using the command palette",
	"Using the Kiro panel",
	"Environment variables",
	"Disabling servers temporarily",
	"Security considerations",
	"Troubleshooting configuration issues",
}

// TestRecognizeKiro_RealMcpLandmarks proves MCP recognition emits 2 canonical
// MCP keys at "inferred" confidence: transport_types and env_var_expansion.
// The other 6 keys (oauth_support, tool_filtering, auto_approve, marketplace,
// resource_referencing, enterprise_management) must NOT be emitted — none
// have heading-level evidence in the MCP doc. tool_filtering and auto_approve
// are documented only as JSON config fields (not landmarks); the rest are
// absent from the kiro MCP surface entirely.
//
// Test merges skills + rules + hooks + MCP fixtures to mirror real-world
// cache merging — the MCP recognizer must distinguish its capabilities from
// the others via the required-anchor uniqueness gate.
func TestRecognizeKiro_RealMcpLandmarks(t *testing.T) {
	merged := append([]string{}, realKiroSkillsLandmarks...)
	merged = append(merged, realKiroRulesLandmarks...)
	merged = append(merged, realKiroHooksLandmarks...)
	merged = append(merged, realKiroMcpLandmarks...)
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["mcp.supported"] != "true" {
		t.Error("mcp.supported missing")
	}
	mcpInferred := []string{
		"transport_types",
		"env_var_expansion",
	}
	for _, c := range mcpInferred {
		key := "mcp.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["mcp.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("mcp.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"mcp.capabilities.oauth_support.supported",
		"mcp.capabilities.tool_filtering.supported",
		"mcp.capabilities.auto_approve.supported",
		"mcp.capabilities.marketplace.supported",
		"mcp.capabilities.resource_referencing.supported",
		"mcp.capabilities.enterprise_management.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (no heading evidence)", absent)
		}
	}
}

// TestRecognizeKiro_McpAnchorsMissing proves the required-anchor guard
// suppresses MCP emission when "Configuration properties" is absent —
// preventing MCP patterns from firing on contexts that contain the more
// generic "Configuration file structure" landmark alone.
func TestRecognizeKiro_McpAnchorsMissing(t *testing.T) {
	mutated := []string{}
	for _, lm := range realKiroMcpLandmarks {
		if lm == "Configuration properties" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["mcp.supported"]; has {
		t.Error("mcp.supported should NOT be present when 'Configuration properties' anchor is missing")
	}
}

// TestRecognizeKiro_RealHooksLandmarks proves hooks recognition emits
// hooks.supported = true on the merged skills+rules+hooks landmarks. Per the
// curated format YAML, ALL 9 canonical hooks keys are unsupported in kiro
// (observational shell-only, no matchers/JSON I/O/decision control). The
// recognizer therefore emits ONLY hooks.supported via the bare anchor-only
// pattern — no specific canonical capabilities are mapped.
func TestRecognizeKiro_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realKiroSkillsLandmarks...)
	merged = append(merged, realKiroRulesLandmarks...)
	merged = append(merged, realKiroHooksLandmarks...)
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: merged,
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["hooks.supported"] != "true" {
		t.Error("hooks.supported missing")
	}
	// All 9 canonical hooks keys are curated as unsupported for kiro — none
	// must be emitted.
	for _, absent := range []string{
		"hooks.capabilities.handler_types.supported",
		"hooks.capabilities.matcher_patterns.supported",
		"hooks.capabilities.decision_control.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.json_io_protocol.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
		"hooks.capabilities.input_modification.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for kiro (curated as unsupported)", absent)
		}
	}
}
