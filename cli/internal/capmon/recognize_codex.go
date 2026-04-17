package capmon

func init() {
	RegisterRecognizer("codex", RecognizerKindRustStruct, recognizeCodex)
}

// codexSkillsOptions returns the codex-specific GoStruct options. Codex's Rust
// source defines 8 Skill-prefixed structs but only 5 of them describe the skill
// format itself. The other 3 (SkillError, SkillLoadOutcome, SkillFileSystemsByPath)
// are runtime types that pollute capability output if matched indiscriminately
// by a single "Skill." prefix.
//
// Included (5):
//   - SkillMetadata.       skill identity / frontmatter fields
//   - SkillPolicy.         allow/deny rules
//   - SkillInterface.      tool/capability surface description
//   - SkillDependencies.   declared dependency manifest
//   - SkillToolDependency. per-tool dependency entry
//
// Excluded (deliberately, not in StructPrefixes):
//   - SkillError.            runtime error type
//   - SkillLoadOutcome.      runtime load result
//   - SkillFileSystemsByPath. runtime filesystem state
func codexSkillsOptions() GoStructOptions {
	return GoStructOptions{
		ContentType: "skills",
		StructPrefixes: []string{
			"SkillMetadata.",
			"SkillPolicy.",
			"SkillInterface.",
			"SkillDependencies.",
			"SkillToolDependency.",
		},
		KeyMapper: skillsKeyMapper,
	}
}

// codexRulesLandmarkOptions returns the landmark patterns for Codex's rules
// (AGENTS.md) documentation. Anchors derived from
// .capmon-cache/codex/rules.0/extracted.json (docs/agents_md.md). The cached
// doc is intentionally short — it redirects to the developers.openai.com
// AGENTS.md spec which was not cached. Recognition is constrained to the two
// landmarks present in the stub spec doc.
//
// rules.1 is codex's OWN AGENTS.md instance file (their internal dev rules)
// and intentionally NOT used as evidence — instance content is not capability
// vocabulary.
//
// Required anchors are unique to the rules doc (skills.* sources have no
// AGENTS.md or "Hierarchical agents message" landmarks):
//   - "AGENTS.md"
//   - "Hierarchical agents message"
//
// Per the seeder spec, codex supports activation_mode.always_on,
// cross_provider_recognition.agents_md, and hierarchical_loading. file_imports
// and auto_memory are intentionally absent from the cached doc surface.
func codexRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "AGENTS.md", CaseInsensitive: true},
		{Kind: "substring", Value: "Hierarchical agents message", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "AGENTS.md",
			"AGENTS.md files are always_on within their scope (project root and child directories) — documented under 'AGENTS.md' (per docs/agents_md.md, redirects to developers.openai.com spec)", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"codex is a primary AGENTS.md spec implementer (per github.com/openai/codex docs); AGENTS.md is the cross-provider standard", required),
		RulesLandmarkPattern("hierarchical_loading", "Hierarchical agents message",
			"hierarchical AGENTS.md loading gated by child_agents_md feature flag in config.toml; codex emits a precedence-explanation message to the model when enabled (documented under 'Hierarchical agents message')", required),
	)
}

// codexHooksLandmarkOptions returns the landmark patterns for Codex's hooks
// evidence. Codex's hooks subsystem has the richest typed-source surface of
// any provider — 16 cache sources spanning JSON Schema files (hooks.0-9, one
// input + output schema per event), TypeScript v2 protocol enums (hooks.10-13),
// and Rust source (hooks.14 engine config + hooks.15 types).
//
// The JSON Schema and TypeScript extractors emit type names as landmarks
// (e.g. "PreToolUseDecisionWire", "HookHandlerType", "HookExecutionMode")
// rather than field-level data. The Rust extractor emits both struct names
// and fields, but landmark recognition reads only the names. Capability
// detection therefore proxies through type-name presence.
//
// Per the curated format YAML (docs/provider-formats/codex.yaml), 8 of the 9
// canonical hooks keys are supported — only json_io_protocol is curated as
// unsupported (codex hooks communicate via exit codes + stdout text rather
// than structured JSON I/O). The 8 supported keys map to type-name landmarks:
//
//	handler_types       → HookHandlerType (TS) / HookHandlerConfig (Rust)
//	matcher_patterns    → MatcherGroup (Rust config.rs)
//	decision_control    → BlockDecisionWire / PreToolUseDecisionWire (JSON Schema)
//	input_modification  → PreToolUseHookSpecificOutputWire (holds updatedInput)
//	async_execution     → HookExecutionMode (TS)
//	hook_scopes         → HookScope (TS)
//	context_injection   → UserPromptSubmitHookSpecificOutputWire (holds additionalContext)
//	permission_control  → PreToolUsePermissionDecisionWire (JSON Schema)
//
// Required anchors are unique to hooks evidence — they distinguish hooks
// landmarks from skills/rules/agents/commands/mcp landmarks within codex:
//   - "HookEventName" — substring matches "HookEventName" (TS) and
//     "HookEventNameWire" (JSON Schema); both are unmistakably hooks vocabulary
//   - "HookScope" — TS enum name unique to hooks
func codexHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "HookEventName", CaseInsensitive: true},
		{Kind: "substring", Value: "HookScope", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("handler_types", "HookHandlerType",
			"hook_handler_types: Codex supports shell command, LLM prompt, and agent handler types (HookHandlerType.ts enum, HookHandlerConfig in config.rs)", required),
		HooksLandmarkPattern("matcher_patterns", "MatcherGroup",
			"hook_matcher: Codex hooks support pattern matching to filter which tools or events trigger the hook (MatcherGroup struct in config.rs)", required),
		HooksLandmarkPattern("decision_control", "BlockDecisionWire",
			"hook_result_abort: Codex hooks can abort (block) the triggering action via decision wire types (BlockDecisionWire / PreToolUseDecisionWire in event output schemas)", required),
		HooksLandmarkPattern("input_modification", "PreToolUseHookSpecificOutputWire",
			"hook_updated_input: PreToolUse hooks return modified tool input via the updatedInput field on PreToolUseHookSpecificOutputWire", required),
		HooksLandmarkPattern("async_execution", "HookExecutionMode",
			"hook_execution_mode: Codex supports sync and async hook execution modes for fire-and-forget background hook runs (HookExecutionMode.ts enum)", required),
		HooksLandmarkPattern("hook_scopes", "HookScope",
			"hook_scope: Codex hooks can be scoped to global/user or project configuration (HookScope.ts enum)", required),
		HooksLandmarkPattern("context_injection", "UserPromptSubmitHookSpecificOutputWire",
			"hook_system_message: UserPromptSubmit and SessionStart hooks inject context via the additionalContext field on *HookSpecificOutputWire types", required),
		HooksLandmarkPattern("permission_control", "PreToolUsePermissionDecisionWire",
			"hook_permission_decision: PreToolUse hooks return allow/deny/ask permission decisions via PreToolUsePermissionDecisionWire", required),
	)
}

// MCP recognition is intentionally NOT wired for codex.
//
// Codex's MCP evidence is JSON Schema field-level data — config-key paths like
// definitions.RawMcpServerConfig.properties.enabled_tools and
// definitions.McpServerToolConfig.properties.approval_mode in
// .capmon-cache/codex/mcp.0/extracted.json (codex-rs/core/config.schema.json).
// The recognizeGoStruct field extractor reads "Type.field" prefixes (e.g.
// "SkillMetadata."), but JSON Schema paths use "definitions.X.properties.Y"
// — a different shape that the current GoStructOptions cannot match.
//
// Heading-level landmarks for codex MCP exist (RawMcpServerConfig,
// McpServerToolConfig, MarketplaceConfig) but they are struct names alone —
// they cannot distinguish between e.g. tool_filtering vs auto_approve, both
// of which are field-level distinctions inside RawMcpServerConfig.
//
// The other source, .capmon-cache/codex/mcp.1/extracted.json, is the
// codex_mcp_interface.md doc covering codex AS an MCP server (codex
// mcp-server) — not codex consuming MCP servers. Its landmarks describe
// the server interface protocol, not consumer-side capability vocabulary.
//
// Per docs/provider-formats/codex.yaml, 3 of 8 canonical MCP keys are
// curated as supported at "confirmed" confidence (oauth_support,
// tool_filtering, auto_approve). Recognizer silence here preserves that
// higher-confidence curated data — landmark "inferred" emissions would
// only be redundant noise.
//
// Wiring codex MCP recognition would require a JSON-Schema field extractor
// analogous to GoStructOptions but reading "definitions.X.properties.Y"
// paths — a separate scope from Phase 6 Epic 4.

// recognizeCodex recognizes skills + rules + hooks capabilities for the Codex
// provider. Codex implements the Agent Skills open standard. Skills source is
// Rust; rules source is markdown; hooks source spans JSON Schema, TypeScript,
// and Rust files (16 cache entries). MCP recognition is intentionally absent
// — see the comment block immediately above this function for rationale.
// Recognition fires only if the extractor surfaces fields under one of the 5
// included struct prefixes (see codexSkillsOptions), landmarks under
// codexRulesLandmarkOptions, or landmarks under codexHooksLandmarkOptions.
func recognizeCodex(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, codexSkillsOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", ".agents/skills/<name>/ under project config folder or between project root and cwd (SkillScope::Repo)", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "~/.agents/skills/<name>/ (SkillScope::User)", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (SKILLS_FILENAME constant)", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	rulesResult := recognizeLandmarks(ctx, codexRulesLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, codexHooksLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult)
}
