package capmon

func init() {
	RegisterRecognizer("opencode", RecognizerKindGoStruct, recognizeOpencode)
}

// MCP recognition is intentionally NOT wired for opencode.
//
// The cached MCP sources are unusable for landmark recognition:
//   - mcp.0 (.capmon-cache/opencode/mcp.0/extracted.json) points at
//     https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json
//     — this is the Crush JSON Schema, not opencode's. The seeder spec for
//     opencode has the wrong source URL. Even if it were the right schema,
//     JSON-Schema fields land in Fields not Landmarks (the listed landmarks
//     are crush struct names like Permissions, Tools, LSPConfig, MCPConfig).
//   - mcp.1 (https://raw.githubusercontent.com/opencode-ai/opencode/main/internal/llm/agent/mcp-tools.go)
//     yields a single landmark "MCPClient" — a Go struct name, not heading
//     evidence and not aligned to any of the 8 canonical MCP keys.
//
// docs/provider-formats/opencode.yaml has no curated mcp section either —
// opencode is archived and the human-edited curator skipped MCP entirely.
//
// Recognizer silence is the right move — emitting any canonical MCP key
// based on these landmarks would either be a false positive (crush schema
// fields incorrectly attributed to opencode) or unanchored (struct name
// without semantic meaning). MCP recognition can be wired once a correct
// opencode MCP source is added to the cache.

// Agents recognition is intentionally NOT wired for opencode.
//
// The cached agents source (.capmon-cache/opencode/agents.0/extracted.json,
// fetched from opencode-ai/opencode/main/internal/llm/agent/agent.go) is a
// Go runtime-event implementation file. Landmarks are AgentEvent,
// AgentEventType, Service — runtime types, not user-facing capability
// vocabulary. Fields are AgentEvent.Done / .Error / .Message / .Progress /
// .SessionID / .Type and AgentEventTypeError / .Response / .Summarize —
// event constants, not agent-definition or scope or tool-restriction
// vocabulary.
//
// None of the 7 canonical agents keys (definition_format, tool_restrictions,
// invocation_patterns, agent_scopes, model_selection, per_agent_mcp,
// subagent_spawning) can be anchored on these landmarks or fields. The
// source describes how the runtime emits events about ONE agent's
// processing, not how multiple custom agents are defined or invoked.
//
// docs/provider-formats/opencode.yaml has no curated agents section —
// opencode is archived (charmbracelet/crush is its evolution) and the
// human-edited curator skipped agents entirely.
//
// Recognizer silence is the right move — emitting any canonical agents key
// based on AgentEvent runtime types would conflate "agent runtime exists"
// with "user-defined custom agents are supported". The latter is not
// documented in any cached opencode source.

// recognizeOpencode recognizes skills capabilities for the OpenCode provider.
// OpenCode is archived; it has no native skill implementation, so this
// recognizer uses the cross-provider SKILL.md convention. GoStruct pattern
// will produce output only if upstream extraction surfaces Skill.* fields
// (unlikely for an archived project). MCP and agents recognition are
// intentionally absent — see the comment blocks immediately above this
// function for rationale.
func recognizeOpencode(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "cross-provider SKILL.md convention at .opencode/skill/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "cross-provider convention at ~/.config/opencode/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (Agent Skills spec)", "confirmed"))
	return wrapCapabilities(result)
}
