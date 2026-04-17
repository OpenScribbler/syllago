package capmon

func init() {
	RegisterRecognizer("kiro", RecognizerKindDoc, recognizeKiro)
}

// kiroLandmarkOptions returns the landmark patterns for Kiro's "Powers" doc.
// Kiro brands skills as "Powers" (file: POWER.md); the canonical mapping
// still populates skills.* dot-paths for cross-provider portability.
//
// Anchors derived from .capmon-cache/kiro/skills.0/extracted.json. Required
// anchors "Create powers" and "Creating POWER.md" guard against false
// positives from other kiro content-type docs (agents, hooks, mcp, rules) —
// none of those mention powers/POWER.md in their landmarks.
func kiroLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Create powers", CaseInsensitive: true},
		{Kind: "substring", Value: "Creating POWER.md", CaseInsensitive: true},
	}
	pattern := func(cap, anchor, mechanism string) LandmarkPattern {
		return LandmarkPattern{
			Capability: cap,
			Required:   required,
			Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
			Mechanism:  mechanism,
		}
	}
	return LandmarkOptions{
		ContentType: "skills",
		Patterns: []LandmarkPattern{
			pattern("frontmatter", "Frontmatter: When to activate", "documented under 'Frontmatter: When to activate' heading"),
			pattern("onboarding_instructions", "Onboarding instructions", "documented under 'Onboarding instructions' heading"),
			pattern("steering_instructions", "Steering instructions", "documented under 'Steering instructions' heading"),
			pattern("mcp_integration", "Adding MCP servers", "documented under 'Adding MCP servers' heading"),
			pattern("directory_structure", "Directory structure", "documented under 'Directory structure' heading"),
			pattern("testing", "Testing locally", "documented under 'Testing locally' heading"),
			pattern("sharing", "Sharing your power", "documented under 'Sharing your power' heading"),
		},
	}
}

// kiroRulesLandmarkOptions returns the landmark patterns for Kiro's "Steering"
// (rules) doc. Anchors derived from .capmon-cache/kiro/rules.0/extracted.json.
// Required anchors "What is steering?" and "Steering file scope" are unique to
// the steering doc — they prevent rules patterns from firing on the powers
// (skills) doc or the cookie-banner noise.
//
// Note: kiro's rules-format vocabulary uses "Inclusion modes" instead of
// "activation modes". The four named modes ("Always included", "Conditional
// inclusion", "Manual inclusion", "Auto inclusion") map one-to-one onto the
// canonical activation_mode sub-vocabulary (always_on, frontmatter_globs,
// manual, model_decision).
func kiroRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "What is steering?", CaseInsensitive: true},
		{Kind: "substring", Value: "Steering file scope", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Always included",
			"steering loaded on every prompt by default (documented under 'Always included' inclusion mode)", required),
		RulesLandmarkPattern("activation_mode.frontmatter_globs", "Conditional inclusion",
			"glob-based path matching activates steering files (documented under 'Conditional inclusion' inclusion mode)", required),
		RulesLandmarkPattern("activation_mode.manual", "Manual inclusion",
			"user explicitly references the file to activate (documented under 'Manual inclusion' inclusion mode)", required),
		RulesLandmarkPattern("activation_mode.model_decision", "Auto inclusion",
			"agent decides based on context (documented under 'Auto inclusion' inclusion mode)", required),
		RulesLandmarkPattern("file_imports", "File references",
			"steering files can reference other files (documented under 'File references' heading)", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "Agents.md",
			"Agents.md fallback for cross-tool compatibility (documented under 'Agents.md' heading)", required),
		RulesLandmarkPattern("hierarchical_loading", "Workspace steering",
			"three-tier scope: workspace + global + team steering (documented under 'Workspace steering' / 'Global steering' / 'Team steering')", required),
	)
}

// kiroHooksLandmarkOptions returns the landmark patterns for Kiro's "Agent
// hooks" doc. Anchors derived from .capmon-cache/kiro/hooks.0/extracted.json.
//
// Required anchors are unique to the hooks doc:
//   - "What are agent hooks?" — H2 in hooks doc, not present in skills/rules
//   - "Setting up agent hooks" — H2 in hooks doc, not present elsewhere
//
// Per the curated format YAML (docs/provider-formats/kiro.yaml), ALL 9
// canonical hooks keys are marked supported: false for kiro — the doc
// describes hooks as observational shell-command triggers without matchers,
// JSON I/O, decision control, or any of the other documented capabilities.
//
// As a result, this recognizer only emits hooks.supported = true via a
// bare anchor-only pattern. No capability-specific patterns are mapped.
func kiroHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "What are agent hooks?", CaseInsensitive: true},
		{Kind: "substring", Value: "Setting up agent hooks", CaseInsensitive: true},
	}
	return LandmarkOptions{
		ContentType: "hooks",
		Patterns: []LandmarkPattern{
			// Bare anchor-only pattern (no Capability) ensures hooks.supported
			// is emitted when the required anchors match.
			{
				Required: required,
				Matchers: required,
			},
		},
	}
}

// kiroMcpLandmarkOptions returns the landmark patterns for Kiro's MCP
// configuration doc. Anchors derived from .capmon-cache/kiro/mcp.0/extracted.json
// (https://kiro.dev/docs/mcp/configuration/, HTML).
//
// Kiro's MCP doc maps only 2 of 8 canonical MCP keys at the heading level:
// transport_types ("Local server" + "Remote server" sub-headings) and
// env_var_expansion ("Environment variables" heading).
//
// The other 6 keys are intentionally unmapped here:
//   - tool_filtering, auto_approve: documented only as JSON config fields
//     ('disabledTools', 'autoApprove') — table-cell evidence, not headings.
//     Curator marks both supported (confirmed) via provider extensions
//     kiro_mcp_disabled_tools and kiro_mcp_auto_approve.
//   - oauth_support, marketplace, resource_referencing, enterprise_management:
//     no heading or field evidence; absent from Kiro's MCP surface.
//
// Required anchors are unique to the MCP doc:
//   - "Configuration file structure" — H2 unique to mcp.0
//   - "Configuration properties"     — H2 unique to mcp.0
//
// Neither appears in kiro's skills, rules, hooks, or agents docs. Note that
// "Adding MCP servers" appears in skills.0 (powers can bundle mcp.json) but
// is not a required anchor here, so cross-content false positives are blocked.
//
// Per docs/provider-formats/kiro.yaml, the curator marks transport_types and
// env_var_expansion as unsupported (inferred). The recognizer disagrees
// because heading-level evidence exists for both. The two YAML files are
// independent: provider-capabilities/ tracks recognizer emissions,
// provider-formats/ tracks curator judgments.
func kiroMcpLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Configuration file structure", CaseInsensitive: true},
		{Kind: "substring", Value: "Configuration properties", CaseInsensitive: true},
	}
	return McpLandmarkOptions(
		McpLandmarkPattern("transport_types", "Local server",
			"transport types (stdio for local, HTTPS/HTTP for remote) documented under 'Local server' / 'Remote server' configuration sub-headings", required),
		McpLandmarkPattern("env_var_expansion", "Environment variables",
			"environment variable expansion (${VAR} syntax) documented under 'Environment variables' heading with security warning against inline secrets", required),
	)
}

// recognizeKiro recognizes skills + rules + hooks + mcp capabilities for the
// Kiro provider. All four content types are HTML/markdown documentation;
// recognition uses landmark matching. Static facts (project_scope,
// canonical_filename) merge in at "confirmed" confidence after a successful
// skills landmark match. Note: Kiro has no global_scope for skills — Powers
// are installed via the Kiro Powers panel UI without a fixed user-wide
// filesystem path.
func recognizeKiro(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, kiroLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "Self-contained directory installed via Kiro Powers panel (UI installation, no fixed filesystem path)", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "POWER.md (fixed, all caps)", "confirmed"))
	}

	rulesResult := recognizeLandmarks(ctx, kiroRulesLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, kiroHooksLandmarkOptions())
	mcpResult := recognizeLandmarks(ctx, kiroMcpLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult, mcpResult)
}
