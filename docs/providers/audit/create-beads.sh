#!/bin/bash
# Create beads for all provider audit findings
# Each bead has: context (WHY), success criteria, testing strategy, docs updates
set -e

# Track created bead IDs for dependency wiring
declare -A BEAD_IDS

create_bead() {
  local key="$1"
  local title="$2"
  local type="$3"
  local priority="$4"
  local description="$5"

  local output
  output=$(bd create --title="$title" --type="$type" --priority="$priority" --description="$description" 2>&1)
  local id
  id=$(echo "$output" | grep -oP 'syllago-\w+')
  BEAD_IDS["$key"]="$id"
  echo "Created $key -> $id: $title"
}

# ============================================================
# CROSS-CUTTING (do first — everything else depends on these)
# ============================================================

create_bead "XX-1" \
  "Rename canonical Task tool to Agent in toolmap" \
  "task" 1 \
  "WHY: Claude Code renamed Task to Agent in v2.1.63. Our canonical name is stale — every provider mapping that references 'Task' is using an outdated name. This affects tool permission translation in exported skills/agents across all providers.

WHAT: In toolmap.go, rename the 'Task' key in ToolNames to 'Agent'. Keep reverse-translation from 'Task' for backwards compatibility (old content referencing Task should still resolve). Update all provider entries that map to Task/Agent equivalents.

SUCCESS CRITERIA:
- ToolNames has 'Agent' key (not 'Task')
- ReverseTranslateTool('Task', any-provider) still returns 'Agent' (backwards compat)
- All provider entries under the old 'Task' key are moved to 'Agent'
- No code references canonical 'Task' tool name

TESTING:
- Unit: Add test in toolmap_test.go verifying Agent key exists and Task reverse-translates
- Unit: Verify all provider mappings under Agent are correct
- Integration: Run converter tests to ensure no regressions

DOCS (syllago-docs/): Update any tool mapping tables or canonical format docs referencing Task

EVIDENCE: Claude Code tools.md line 275, 796 — Task renamed to Agent in v2.1.63"

create_bead "XX-2" \
  "Add canonical WebFetch tool to toolmap" \
  "task" 1 \
  "WHY: 6 providers have URL-fetching tools (Claude Code WebFetch, Zed fetch, OpenCode webfetch, Kiro web_fetch, Windsurf read_url_content, Copilot web_fetch) but no canonical WebFetch entry exists in ToolNames. Skills/agents referencing WebFetch get no translation.

WHAT: Add 'WebFetch' entry to ToolNames in toolmap.go with mappings for all providers that have an equivalent tool.

SUCCESS CRITERIA:
- ToolNames['WebFetch'] exists with entries for gemini-cli, copilot-cli, kiro, opencode, zed, cline (if applicable), roo-code, windsurf, cursor
- TranslateTool('WebFetch', 'zed') returns 'fetch'
- ReverseTranslateTool('fetch', 'zed') returns 'WebFetch'

TESTING:
- Unit: Test forward and reverse translation for all mapped providers
- Unit: Test providers with no mapping pass through unchanged

DOCS (syllago-docs/): Add WebFetch to canonical tool reference table

EVIDENCE: Multiple provider research docs confirm URL-fetching tools exist across providers"

# ============================================================
# CLAUDE CODE
# ============================================================

create_bead "CC-toolmap" \
  "Fix Claude Code toolmap: SubagentCompleted, missing events and tools" \
  "bug" 1 \
  "WHY: The Claude Code canonical toolmap has wrong event names and is missing 12 of 22 hook events plus key tools. This means hook conversion silently drops events and tool permissions don't translate for newer tools.

FINDINGS:
- CC-1: HookEvents uses 'SubagentCompleted' but correct name is 'SubagentStop' (toolmap.go:86)
- CC-8: Missing 12 hook events: PostToolUseFailure, PermissionRequest, PostCompact, InstructionsLoaded, ConfigChange, WorktreeCreate/Remove, Elicitation/Result, TeammateIdle, TaskCompleted, StopFailure
- CC-9: ToolNames missing WebFetch, NotebookEdit, Skill, AskUserQuestion (Agent covered by XX-1)

SUCCESS CRITERIA:
- SubagentCompleted renamed to SubagentStop in HookEvents
- All 22 Claude Code hook events have entries (even if empty map for CC-only events)
- ToolNames has entries for WebFetch (covered by XX-2), NotebookEdit, Skill, AskUserQuestion

TESTING:
- Unit: Verify all 22 events exist in HookEvents
- Unit: Test TranslateHookEvent for each new event
- Unit: Test TranslateTool for each new tool

DOCS (syllago-docs/): Update hook event mapping table, tool mapping table

EVIDENCE: hooks.md lines 27-54 (22 events), tools.md line 275"

create_bead "CC-rules" \
  "Fix Claude Code rules: support paths frontmatter and CLAUDE.md distinction" \
  "bug" 2 \
  "WHY: When rendering rules to Claude Code, glob-scoped rules get prose embedding instead of native YAML 'paths' frontmatter. Rules load unconditionally instead of conditionally. Also no distinction between CLAUDE.md (always-apply) and .claude/rules/ (conditional).

FINDINGS:
- CC-2: Rules lose paths frontmatter when rendered (rules.go:63-64)
- CC-10: No distinction between CLAUDE.md vs .claude/rules/ (rules.go:339-360)
- CC-13: Opportunity to leverage native paths for cross-provider scoping

SUCCESS CRITERIA:
- Rendering to Claude Code: always-apply rules target CLAUDE.md, glob-scoped rules target .claude/rules/<name>.md with paths frontmatter
- Round-trip: import .claude/rules/ rule with paths -> export back preserves paths exactly
- Canonical globs field maps to Claude Code paths field

TESTING:
- Unit: Test renderClaudeCodeRule with always-apply (no globs) -> produces CLAUDE.md content
- Unit: Test renderClaudeCodeRule with globs -> produces .md with paths frontmatter
- Integration: Round-trip test import/export preserves paths

DOCS (syllago-docs/): Document Claude Code rule rendering behavior"

create_bead "CC-hooks-timeout" \
  "Fix canonical hook timeout unit inconsistency" \
  "bug" 1 \
  "WHY: Claude Code stores hook timeouts in seconds, Copilot canonicalizer stores milliseconds. Round-tripping a hook through syllago produces wrong timeout values depending on source provider. This is a data corruption bug.

FINDINGS:
- CC-4: hooks.go:296-297 (Claude Code seconds), hooks.go:434 (Copilot ms conversion)

SUCCESS CRITERIA:
- Define canonical timeout unit explicitly (recommend seconds, matching Claude Code)
- All canonicalizers normalize to the same unit
- All renderers convert from canonical to provider-specific unit
- Round-trip: Claude Code -> canonical -> Copilot preserves correct timeout
- Round-trip: Copilot -> canonical -> Claude Code preserves correct timeout

TESTING:
- Unit: Test canonicalize from each provider with known timeout values
- Unit: Test render to each provider produces correct unit
- Unit: Round-trip test across providers

DOCS (syllago-docs/): Document canonical timeout unit in format spec"

create_bead "CC-allowed-tools" \
  "Fix SkillMeta.AllowedTools comma-separated string parsing" \
  "bug" 1 \
  "WHY: Claude Code allowed-tools is a comma-separated string ('Read, Grep, Glob') but YAML unmarshals it into a single-element []string instead of three elements. This breaks tool name translation — the entire string gets passed to TranslateTool as one unit.

FINDINGS:
- CC-5: skills.go:22 — AllowedTools []string doesn't handle comma-separated input

SUCCESS CRITERIA:
- Canonicalizing a skill with 'allowed-tools: Read, Grep, Glob' produces AllowedTools with 3 elements
- Each element gets individually translated during render
- Space-delimited format (Agent Skills spec) also handled correctly
- YAML list format also handled correctly

TESTING:
- Unit: Test canonicalize with comma-separated, space-delimited, and YAML list formats
- Unit: Test render translates each tool individually
- Integration: Round-trip test with multi-tool allowed-tools

DOCS (syllago-docs/): Document allowed-tools parsing behavior across providers"

create_bead "CC-hook-types" \
  "Add http, prompt, and agent hook type support" \
  "feature" 2 \
  "WHY: Claude Code supports 4 hook types (command, http, prompt, agent) but syllago only handles command. HTTP hooks lose url/headers/allowedEnvVars, prompt hooks lose prompt/model, agent hooks lose agent config. These hook types are increasingly used in enterprise setups.

FINDINGS:
- CC-6: HookEntry struct only has command fields (hooks.go:19-25)

SUCCESS CRITERIA:
- HookEntry struct has fields for all 4 types: command (existing), http (url, headers, allowedEnvVars), prompt (prompt, model), agent (agent config)
- Canonicalize preserves all fields for all types
- Render outputs correct type-specific fields
- Unknown hook types pass through with warning

TESTING:
- Unit: Test canonicalize for each hook type
- Unit: Test render for each hook type
- Unit: Test round-trip for each type

DOCS (syllago-docs/): Document all 4 hook types in canonical format spec"

create_bead "CC-agent-fields" \
  "Add effort, hooks, and color fields to AgentMeta" \
  "bug" 2 \
  "WHY: Three Claude Code agent fields are silently dropped during canonicalization: effort (controls reasoning level), hooks (critical for workflow agents), and color (display). Exported agents lose functionality.

FINDINGS:
- CC-7: agents.go:19-36 missing effort, hooks, color

SUCCESS CRITERIA:
- AgentMeta struct has effort, hooks, color fields
- Canonicalize preserves these from Claude Code agents
- Render outputs them for Claude Code; embeds as prose notes for other providers
- color field preserved for round-trips

TESTING:
- Unit: Test canonicalize with agent that has effort/hooks/color
- Unit: Test render to Claude Code includes all three
- Unit: Test render to non-Claude Code embeds as notes

DOCS (syllago-docs/): Add effort/hooks/color to agent format spec"

create_bead "CC-mcp-oauth" \
  "Add OAuth support to MCP config for Claude Code" \
  "feature" 3 \
  "WHY: OAuth-configured MCP servers lose auth config during conversion. The struct has no OAuth field for Claude Code (only OpenCode). Enterprise MCP servers increasingly use OAuth.

FINDINGS:
- CC-11: mcp.go:16-44 missing OAuth for Claude Code

SUCCESS CRITERIA:
- MCP canonical struct has OAuth config fields (clientId, callbackPort, authServerMetadataUrl)
- Claude Code MCP configs with OAuth round-trip correctly
- OAuth config preserved or warned about for other providers

TESTING:
- Unit: Test canonicalize with OAuth MCP config
- Unit: Test render preserves OAuth for Claude Code
- Unit: Test render to non-OAuth providers warns

DOCS (syllago-docs/): Document OAuth MCP config support"

create_bead "CC-mcp-discovery" \
  "Fix Claude Code MCP discovery to use .mcp.json" \
  "bug" 2 \
  "WHY: Team-shared project-level MCP configs in .mcp.json are invisible to syllago. Only .claude.json (user-local) is discovered.

FINDINGS:
- CC-3: claude.go:50 uses .claude.json not .mcp.json

SUCCESS CRITERIA:
- Claude Code provider discovers MCP from both .mcp.json (project) and .claude.json (user)
- .mcp.json takes precedence for project-level discovery

TESTING:
- Unit: Test DiscoveryPaths returns .mcp.json path for MCP type
- Integration: Test MCP discovery finds servers from .mcp.json

DOCS (syllago-docs/): Document MCP discovery sources"

create_bead "CC-hook-matchers" \
  "Fix hook matcher regex handling in translation" \
  "bug" 3 \
  "WHY: Matchers like 'Edit|Write' or 'mcp__github__.*' are passed to TranslateTool as single strings, breaking regex patterns. The matcher should have its tool name components translated individually while preserving the regex syntax.

FINDINGS:
- CC-12: hooks.go matcher handling treats regex as literal

SUCCESS CRITERIA:
- Regex matchers with | (alternation) get each component translated
- Wildcard matchers (.*) preserved
- MCP prefix patterns handled correctly
- Simple string matchers still work

TESTING:
- Unit: Test matcher translation with 'Edit|Write' -> provider equivalents
- Unit: Test matcher with 'mcp__.*' preserved
- Unit: Test simple matcher 'Bash' still works

DOCS (syllago-docs/): Document matcher translation behavior"

# ============================================================
# GEMINI CLI
# ============================================================

create_bead "GC-toolmap" \
  "Fix Gemini CLI toolmap: Glob, WebSearch, MCP format, missing tools" \
  "bug" 1 \
  "WHY: Multiple wrong tool name mappings produce incorrect output in exported content. Glob maps to list_directory (wrong tool), WebSearch maps to google_search (wrong name), MCP tool format uses wrong separator pattern.

FINDINGS:
- GC-1: Glob -> list_directory should be glob (toolmap.go:46)
- GC-2: WebSearch -> google_search should be google_web_search (toolmap.go:64)
- GC-3: MCP format server__tool should be mcp_server_tool (toolmap.go:147,157,181-187)
- GC-4: Grep -> grep_search may need to be search_file_content (needs verification)
- GC-7: list_directory has no canonical mapping after Glob fix
- GC-8: No reverse alias handling for search_file_content/grep_search

SUCCESS CRITERIA:
- Glob maps to 'glob' for gemini-cli
- WebSearch maps to 'google_web_search' for gemini-cli
- MCP tool names use mcp_ prefix with single underscores
- Grep mapping verified and updated if needed

TESTING:
- Unit: Test all Gemini CLI tool translations forward and reverse
- Unit: Test MCP tool name translation for Gemini format
- Integration: Test hook with Gemini MCP matcher

DOCS (syllago-docs/): Update Gemini CLI tool mapping table"

create_bead "GC-hooks" \
  "Add missing Gemini CLI hook events: BeforeModel, AfterModel, BeforeToolSelection" \
  "feature" 3 \
  "WHY: 3 Gemini-unique hook events are silently dropped during import with no warning. Users importing Gemini hooks that use these events get unexpected behavior.

FINDINGS:
- GC-5: BeforeModel, AfterModel, BeforeToolSelection have no canonical mapping

SUCCESS CRITERIA:
- These events have HookEvents entries (even if empty maps — they're Gemini-only)
- Import/canonicalize warns when these events have no target-provider equivalent
- Export to Gemini preserves these events from canonical

TESTING:
- Unit: Test TranslateHookEvent for these 3 events
- Unit: Test import warns about untranslatable events

DOCS (syllago-docs/): Document Gemini-unique hook events"

create_bead "GC-provider" \
  "Fix Gemini CLI provider: commands path, skills discovery" \
  "bug" 3 \
  "WHY: Commands InstallDir returns .gemini/ instead of .gemini/commands/, and skills discovery misses .agents/skills/ alternate path.

FINDINGS:
- GC-6: Commands install to wrong location (gemini.go:23-24)
- GC-9: Skills discovery missing .agents/skills/ (gemini.go:42)

SUCCESS CRITERIA:
- Commands InstallDir returns ~/.gemini/commands/
- Skills DiscoveryPaths includes both .gemini/skills/ and .agents/skills/

TESTING:
- Unit: Test InstallDir for Commands returns correct path
- Unit: Test DiscoveryPaths for Skills returns both paths

DOCS (syllago-docs/): Update Gemini CLI provider reference"

# ============================================================
# CURSOR
# ============================================================

create_bead "CU-toolmap" \
  "Add Cursor to toolmap: 20 tools + hook events" \
  "feature" 1 \
  "WHY: Cursor is completely absent from ToolNames and HookEvents. No tool or event translation exists. This is the most popular AI code editor — zero support for tool translation is a critical gap.

FINDINGS:
- CU-3: No cursor entries in ToolNames or HookEvents (toolmap.go)
- CU-10: 20+ hook events need mapping
- CU-13: MCP tool format unknown (appears to be 'MCP:toolname')

SUCCESS CRITERIA:
- ToolNames has cursor entries for: read_file->Read, edit_file->Edit, run_terminal_cmd->Bash, grep_search->Grep, file_search->Glob, web_search->WebSearch, codebase_search (no equiv), list_dir, delete_file, fetch_rules, etc.
- HookEvents has cursor entries for common events
- MCP tool name format documented and handled

TESTING:
- Unit: Test all Cursor tool translations forward and reverse
- Unit: Test Cursor hook event translations

DOCS (syllago-docs/): Add Cursor tool mapping table"

create_bead "CU-provider" \
  "Expand Cursor provider: support Skills, Commands, Hooks, MCP, Agents" \
  "feature" 1 \
  "WHY: Cursor is treated as rules-only but actually supports all 6 non-loadout content types. Users can't import/export skills, agents, hooks, or MCP from Cursor.

FINDINGS:
- CU-2: SupportsType only returns true for Rules (cursor.go:44-51)
- CU-4: InstallDir only handles Rules (cursor.go:14-19)
- CU-5: DiscoveryPaths only handles Rules (cursor.go:25-32)
- CU-11: .cursorrules legacy not discovered
- CU-12: AGENTS.md not associated with Cursor

SUCCESS CRITERIA:
- SupportsType returns true for Rules, Skills, Commands, Hooks, MCP, Agents
- InstallDir returns correct paths: Skills (.cursor/skills/), Commands (.cursor/commands/), Hooks (__json_merge__), MCP (__json_merge__), Agents (.cursor/agents/)
- DiscoveryPaths finds content from all type directories
- .cursorrules legacy and AGENTS.md included in rules discovery

TESTING:
- Unit: Test SupportsType for all content types
- Unit: Test InstallDir returns correct paths
- Unit: Test DiscoveryPaths returns correct paths
- Integration: Test discovery finds existing Cursor content

DOCS (syllago-docs/): Update Cursor provider reference, add supported content types"

create_bead "CU-hooks-converter" \
  "Add Cursor hooks converter (canonicalize and render)" \
  "feature" 2 \
  "WHY: After removing Cursor from hookless list and expanding the provider def, we need actual conversion logic. Cursor hooks use a unique schema (hooks.json with version, type, failClosed, loop_limit, matcher).

FINDINGS:
- CU-1: Cursor marked hookless (hooks.go:209) — must remove
- CU-6: No canonicalize case for cursor

SUCCESS CRITERIA:
- Cursor removed from hooklessProviders
- Canonicalize handles cursor hooks.json format
- Render produces valid cursor hooks.json with version field
- JSON stdin/stdout protocol differences documented (not converted — runtime behavior)

TESTING:
- Unit: Test canonicalize from Cursor hooks.json
- Unit: Test render to Cursor hooks.json
- Unit: Round-trip test

DOCS (syllago-docs/): Document Cursor hook conversion limitations"

create_bead "CU-skills-converter" \
  "Add Cursor skills converter" \
  "feature" 2 \
  "WHY: Cursor has SKILL.md files with slightly different frontmatter than Claude Code. After provider def expansion, skills need conversion logic.

FINDINGS:
- CU-7: No case 'cursor' in skills canonicalize/render
- CU-14: Format is close to canonical — key diffs: has license/compatibility/metadata, lacks allowed-tools/context/agent/model

SUCCESS CRITERIA:
- Canonicalize handles Cursor SKILL.md (preserves license, compatibility, metadata)
- Render produces valid Cursor SKILL.md
- Fields with no Cursor equivalent embedded as prose notes

TESTING:
- Unit: Test canonicalize from Cursor SKILL.md
- Unit: Test render to Cursor SKILL.md
- Unit: Round-trip test

DOCS (syllago-docs/): Document Cursor skill format differences"

create_bead "CU-mcp-converter" \
  "Add Cursor MCP converter" \
  "feature" 2 \
  "WHY: Cursor uses .cursor/mcp.json with mcpServers key, variable interpolation, and OAuth. After provider def expansion, MCP needs conversion.

FINDINGS:
- CU-8: No MCP support

SUCCESS CRITERIA:
- Canonicalize handles .cursor/mcp.json format
- Variable interpolation syntax documented (not converted — runtime behavior)
- Render produces valid .cursor/mcp.json

TESTING:
- Unit: Test canonicalize from Cursor MCP config
- Unit: Test render to Cursor MCP config
- Unit: Round-trip test

DOCS (syllago-docs/): Document Cursor MCP format"

create_bead "CU-agents-converter" \
  "Add Cursor agents converter" \
  "feature" 2 \
  "WHY: Cursor supports .cursor/agents/<name>.md with frontmatter (name, description, model, readonly, is_background). After provider def expansion, agents need conversion.

FINDINGS:
- CU-9: No agent converter for Cursor

SUCCESS CRITERIA:
- Canonicalize handles Cursor agent .md files
- Render produces valid Cursor agent files
- readonly/is_background fields preserved or noted

TESTING:
- Unit: Test canonicalize from Cursor agent
- Unit: Test render to Cursor agent
- Unit: Round-trip test

DOCS (syllago-docs/): Document Cursor agent format"

# ============================================================
# WINDSURF
# ============================================================

create_bead "WS-toolmap" \
  "Add Windsurf to toolmap: 23 tools + hook events" \
  "feature" 1 \
  "WHY: Windsurf completely absent from ToolNames and HookEvents. 23 tools documented with clear cross-provider equivalents. Zero translation exists.

FINDINGS:
- WS-5: No windsurf entries (toolmap.go)
- WS-7: Hook events use per-tool-category pattern (structural mismatch)

SUCCESS CRITERIA:
- ToolNames has windsurf entries for core tools: view_line_range->Read, write_to_file->Write, edit_file->Edit, run_command->Bash, find_by_name->Glob, grep_search->Grep, search_web->WebSearch, read_url_content->WebFetch
- HookEvents documents the structural mapping challenge (per-tool events vs generic+matcher)

TESTING:
- Unit: Test all Windsurf tool translations
- Unit: Document hook event mapping approach

DOCS (syllago-docs/): Add Windsurf tool mapping table"

create_bead "WS-provider" \
  "Expand Windsurf provider: modern discovery, Skills, Hooks, MCP" \
  "feature" 1 \
  "WHY: Windsurf treated as legacy-rules-only. Modern rules use .windsurf/rules/*.md with trigger field. Skills, Hooks, MCP all supported but not modeled.

FINDINGS:
- WS-2: SupportsType only Rules (windsurf.go:39-46)
- WS-3: Discovery only finds .windsurfrules (windsurf.go:26-29)
- WS-4: EmitPath targets legacy format (windsurf.go:36-38)
- WS-6: InstallDir only handles Rules (windsurf.go:14-19)

SUCCESS CRITERIA:
- SupportsType true for Rules, Skills, Hooks, MCP
- DiscoveryPaths includes .windsurf/rules/ (modern) and .windsurfrules (legacy)
- InstallDir returns paths for Skills, Hooks (__json_merge__), MCP (__json_merge__)
- EmitPath targets .windsurf/rules/<name>.md (modern)

TESTING:
- Unit: Test all SupportsType, InstallDir, DiscoveryPaths
- Integration: Test discovery finds modern format rules

DOCS (syllago-docs/): Update Windsurf provider reference"

create_bead "WS-hooks-converter" \
  "Add Windsurf hooks converter (remove from hookless)" \
  "feature" 2 \
  "WHY: Windsurf falsely listed as hookless but has 12 hook events. Structural challenge: Windsurf uses per-tool-category events (pre_read_code, pre_write_code) while syllago uses event+matcher.

FINDINGS:
- WS-1: Listed in hooklessProviders (hooks.go:210)

SUCCESS CRITERIA:
- Windsurf removed from hooklessProviders
- Canonicalize converts per-tool events to PreToolUse+matcher pattern
- Render converts PreToolUse+matcher back to per-tool events
- Event types that don't map cleanly get warnings

TESTING:
- Unit: Test canonicalize from Windsurf hooks.json
- Unit: Test render to Windsurf hooks.json
- Unit: Test per-tool-event splitting/merging

DOCS (syllago-docs/): Document Windsurf hook conversion approach"

create_bead "WS-skills-mcp" \
  "Add Windsurf skills and MCP converters" \
  "feature" 3 \
  "WHY: After provider expansion, Windsurf skills and MCP need conversion logic.

FINDINGS:
- WS-8: No skills converter (follows Agent Skills standard)
- WS-9: No MCP converter (uses mcp_config.json)

SUCCESS CRITERIA:
- Skills canonicalize/render for Windsurf SKILL.md
- MCP canonicalize/render for mcp_config.json

TESTING:
- Unit: Test skills round-trip
- Unit: Test MCP round-trip

DOCS (syllago-docs/): Document Windsurf format differences"

# ============================================================
# CODEX
# ============================================================

create_bead "CX-agent-schema" \
  "Fix Codex agent TOML schema and missing fields" \
  "bug" 1 \
  "WHY: Agents rendered for Codex use wrong field structure and are missing 5 supported fields. Exported agents likely won't load their instructions.

FINDINGS:
- CX-1: Uses nested [agent.instructions].content instead of developer_instructions (codex_agents.go:132-138)
- CX-4: Missing model_reasoning_effort, sandbox_mode, nickname_candidates, mcp_servers, skills.config
- CX-2: Tool translation uses wrong slug 'copilot-cli' producing wrong names
- CX-10: Multi-agent config incomplete

SUCCESS CRITERIA:
- developer_instructions as top-level string field
- All 5 missing fields supported in the struct
- Codex has its own ToolNames entries (not piggybacking copilot-cli)
- False warnings about 'not supported' removed

TESTING:
- Unit: Test render produces correct TOML with developer_instructions
- Unit: Test all supported fields are rendered
- Unit: Test tool translation uses codex-specific names
- Integration: Validate rendered TOML against Codex format

DOCS (syllago-docs/): Update Codex agent format documentation"

create_bead "CX-toolmap" \
  "Add Codex to toolmap: core tools + hook events" \
  "feature" 1 \
  "WHY: Zero Codex entries in ToolNames despite 32 documented tools. Core 8 need mappings at minimum. Hook events also unmapped (though names happen to match canonical by coincidence).

FINDINGS:
- CX-5: No codex entries in ToolNames (toolmap.go:8-73)
- CX-8: No codex entries in HookEvents (toolmap.go:76-87)

SUCCESS CRITERIA:
- ToolNames has codex entries: read_file->Read, apply_patch->Edit, shell->Bash, list_dir->Glob, grep_files->Grep, web_search->WebSearch, spawn_agent->Agent
- HookEvents has codex entries (identity mapping for matching names)

TESTING:
- Unit: Test all Codex tool translations forward and reverse

DOCS (syllago-docs/): Add Codex tool mapping table"

create_bead "CX-provider" \
  "Expand Codex provider: Skills, MCP, Hooks support" \
  "feature" 2 \
  "WHY: Codex has full skills system and MCP in config.toml but neither modeled as supported.

FINDINGS:
- CX-3: Hooks falsely marked unsupported (hooks.go:211)
- CX-6: Skills not supported despite .agents/skills/ with SKILL.md
- CX-7: MCP not supported (uses TOML format in config.toml)
- CX-9: DiscoveryPaths missing skills and hooks

SUCCESS CRITERIA:
- Codex removed from hooklessProviders
- SupportsType true for Skills, MCP, Hooks
- DiscoveryPaths includes .agents/skills/ and .codex/hooks.json

TESTING:
- Unit: Test updated provider definition
- Integration: Test discovery finds Codex content

DOCS (syllago-docs/): Update Codex provider reference"

# ============================================================
# COPILOT CLI
# ============================================================

create_bead "CP-toolmap" \
  "Fix Copilot CLI toolmap: Write/Edit/Grep/Bash all wrong" \
  "bug" 1 \
  "WHY: 4 of 6 Copilot tool mappings are wrong. Write/Edit map to 'apply_patch' (internal name), Grep maps to 'rg' (not tool name), Bash maps to 'shell' (permission category). Exported content references wrong tool names.

FINDINGS:
- CP-1: Write->apply_patch should be create, Edit->apply_patch should be edit
- CP-2: Grep->rg should be grep
- CP-3: Bash->shell should be bash

SUCCESS CRITERIA:
- Write maps to 'create' for copilot-cli
- Edit maps to 'edit' for copilot-cli
- Grep maps to 'grep' for copilot-cli
- Bash maps to 'bash' for copilot-cli
- Missing events added: subagentStop, agentStop, errorOccurred (CP-11)

TESTING:
- Unit: Test all updated translations forward and reverse

DOCS (syllago-docs/): Update Copilot CLI tool mapping table"

create_bead "CP-provider" \
  "Fix Copilot CLI provider: MCP filename, hooks directory, add skills" \
  "bug" 1 \
  "WHY: MCP config filename is wrong (mcp.json vs mcp-config.json), hooks directory structure wrong (single file vs directory), and skills not supported despite Copilot having SKILL.md support.

FINDINGS:
- CP-4: MCP filename mcp.json should be mcp-config.json (copilot.go:54)
- CP-5: Hooks expects .copilot/hooks.json but should be .github/hooks/*.json (copilot.go:56)
- CP-12: Skills not in SupportsType
- CP-13: Instructions discovery incomplete

SUCCESS CRITERIA:
- MCP discovery/install uses mcp-config.json
- Hooks discovery scans .github/hooks/ directory
- SupportsType true for Skills
- DiscoveryPaths includes .github/skills/ and ~/.copilot/skills/

TESTING:
- Unit: Test corrected MCP filename
- Unit: Test hooks directory scanning
- Unit: Test skills support added

DOCS (syllago-docs/): Update Copilot CLI provider reference"

create_bead "CP-hooks-converter" \
  "Fix Copilot CLI hooks converter: version field, matchers, entry fields" \
  "bug" 2 \
  "WHY: Rendered hooks missing required version field, matchers falsely dropped, and entry struct incomplete.

FINDINGS:
- CP-6: Missing 'version: 1' top-level field (hooks.go:147-148)
- CP-9: Matcher support wrongly claimed unsupported (hooks.go:409,413)
- CP-10: Missing env, cwd, type fields in entry

SUCCESS CRITERIA:
- Rendered Copilot hooks have 'version': 1
- Matchers preserved (not dropped)
- Hook entries include env, cwd, type: 'command'

TESTING:
- Unit: Test render produces version field
- Unit: Test matchers are preserved
- Unit: Test entry has all required fields

DOCS (syllago-docs/): Document Copilot hook format requirements"

create_bead "CP-agents-converter" \
  "Fix Copilot CLI agents: missing fields, wrong filename" \
  "bug" 2 \
  "WHY: Agent struct missing model/target/mcp-servers fields, and output uses 'agent.md' instead of '<name>.agent.md'.

FINDINGS:
- CP-7: Missing model, target, mcp-servers, disable-model-invocation (agents.go:50-55)
- CP-8: Filename should be *.agent.md (agents.go:260)

SUCCESS CRITERIA:
- AgentMeta has model, target, mcp-servers fields for Copilot
- Rendered agent files use <name>.agent.md naming convention
- All fields round-trip correctly

TESTING:
- Unit: Test render produces correct filename
- Unit: Test all fields preserved

DOCS (syllago-docs/): Document Copilot agent format"

# ============================================================
# ZED
# ============================================================

create_bead "ZD-toolmap" \
  "Fix Zed toolmap: spawn_agent, MCP format, add WebFetch" \
  "bug" 1 \
  "WHY: Task maps to 'subagent' (wrong) instead of 'spawn_agent'. MCP tool format produces 'server/tool' but Zed uses 'mcp:server:tool'. WebFetch mapping missing.

FINDINGS:
- ZD-1: Task -> subagent should be spawn_agent (toolmap.go:71)
- ZD-2: MCP format server/tool should be mcp:server:tool (toolmap.go:147,159,189)
- ZD-5: Missing WebFetch -> fetch mapping

SUCCESS CRITERIA:
- Task maps to spawn_agent for zed
- TranslateMCPToolName produces mcp:server:tool for zed
- WebFetch maps to fetch for zed

TESTING:
- Unit: Test all Zed tool translations
- Unit: Test MCP tool name format

DOCS (syllago-docs/): Update Zed tool mapping"

create_bead "ZD-discovery" \
  "Fix Zed discovery: .rules file, MCP extraction from context_servers" \
  "bug" 2 \
  "WHY: Zed's primary rule file .rules is not discovered. MCP extraction only checks mcpServers key, missing Zed's context_servers.

FINDINGS:
- ZD-3: .rules file not in native scan (native_scan.go:282-283)
- ZD-4: MCP extraction doesn't check context_servers (native_scan.go:228-231)
- ZD-6: extractEmbeddedMCP hardcoded to mcpServers key

SUCCESS CRITERIA:
- Native scan discovers .rules file at project root
- extractEmbeddedMCP checks provider-specific keys (context_servers for Zed, mcpServers for Claude Code)

TESTING:
- Unit: Test native scan finds .rules
- Unit: Test MCP extraction from context_servers
- Integration: Test full Zed content discovery

DOCS (syllago-docs/): Document Zed discovery behavior"

# ============================================================
# CLINE
# ============================================================

create_bead "CL-toolmap" \
  "Fix Cline toolmap: replace_in_file, add missing tools" \
  "bug" 1 \
  "WHY: Edit maps to apply_diff (legacy name) instead of replace_in_file (current name). Three Cline-specific tools unmapped.

FINDINGS:
- CL-1: Edit -> apply_diff should be replace_in_file (toolmap.go:33)
- CL-8: browser_action, use_mcp_tool, access_mcp_resource unmapped

SUCCESS CRITERIA:
- Edit maps to replace_in_file for cline
- Unmapped tools documented (no canonical equivalent)

TESTING:
- Unit: Test updated translation

DOCS (syllago-docs/): Update Cline tool mapping"

create_bead "CL-hooks" \
  "Enable Cline hooks: remove from hookless, add events, converter" \
  "feature" 1 \
  "WHY: Cline falsely listed as hookless but has 8 hook events since v3.36. Hooks use file-based executables (fundamentally different from JSON config).

FINDINGS:
- CL-2: Listed in hooklessProviders (hooks.go:207)
- CL-3: Missing Hooks in SupportsType/InstallDir (cline.go:48-55)
- CL-4: No Cline entries in HookEvents
- CL-5: File-based executables, not JSON config

SUCCESS CRITERIA:
- Cline removed from hooklessProviders
- SupportsType true for Hooks
- HookEvents has cline mappings (PreToolUse->PreToolUse, PostToolUse->PostToolUse, SessionStart->TaskStart, Stop->TaskComplete, etc.)
- Converter handles file-based executable format or warns about format difference

TESTING:
- Unit: Test hook event translations
- Unit: Test provider definition
- Integration: Test hooks discovery from .clinerules/hooks/

DOCS (syllago-docs/): Document Cline hook format differences"

create_bead "CL-provider" \
  "Fix Cline provider: global rules path, platform-aware MCP path" \
  "bug" 3 \
  "WHY: Global rules path and MCP settings path missing/wrong.

FINDINGS:
- CL-6: Missing ~/Documents/Cline/Rules/ (cline.go)
- CL-7: MCP path needs platform detection for VS Code globalStorage

SUCCESS CRITERIA:
- DiscoveryPaths includes global rules path
- MCP settings path is platform-aware

TESTING:
- Unit: Test DiscoveryPaths includes global path
- Unit: Test MCP path resolution per platform

DOCS (syllago-docs/): Update Cline provider paths"

# ============================================================
# ROO CODE
# ============================================================

create_bead "RC-toolmap" \
  "Fix Roo Code toolmap: PascalCase to snake_case, add missing tools" \
  "bug" 1 \
  "WHY: All 6 Roo Code tool entries use TypeScript class names (ReadFileTool) instead of AI-facing snake_case names (read_file). Every tool translation produces wrong output. Also only 6 of 25+ tools mapped.

FINDINGS:
- RC-1: PascalCase class names instead of snake_case (toolmap.go:16-62)
- RC-7: Only 6 of 25+ tools mapped

SUCCESS CRITERIA:
- All entries use snake_case: read_file, write_to_file, replace_in_file, execute_command, list_files, search_files
- Additional common tools mapped: apply_diff, apply_patch, new_task, codebase_search, browser_action

TESTING:
- Unit: Test all translations produce snake_case names
- Unit: Test reverse translation from snake_case

DOCS (syllago-docs/): Update Roo Code tool mapping"

create_bead "RC-mcp" \
  "Fix Roo Code MCP: add alwaysAllow field, remove false warning" \
  "bug" 1 \
  "WHY: MCP config silently drops alwaysAllow (auto-approve lists) and emits a misleading warning that Roo Code doesn't support it — but it does.

FINDINGS:
- RC-2: Missing alwaysAllow field, false warning (mcp.go:51-60,501,517)

SUCCESS CRITERIA:
- alwaysAllow field preserved in Roo Code MCP configs
- False 'not supported' warning removed
- Round-trip preserves auto-approve lists

TESTING:
- Unit: Test MCP with alwaysAllow round-trips
- Unit: Verify no false warning emitted

DOCS (syllago-docs/): Update MCP format docs with alwaysAllow"

create_bead "RC-provider" \
  "Expand Roo Code provider: add Skills, fix discovery paths" \
  "feature" 2 \
  "WHY: Skills not supported despite Roo Code having full skills system. Discovery missing .roomodes, global rules paths.

FINDINGS:
- RC-4: Skills not in SupportsType (roocode.go:61-68)
- RC-5: .roomodes not discoverable (roocode.go:30-47)
- RC-6: Global rules paths missing

SUCCESS CRITERIA:
- SupportsType true for Skills
- DiscoveryPaths includes .roo/skills/, .agents/skills/
- Global paths included for rules

TESTING:
- Unit: Test updated provider definition

DOCS (syllago-docs/): Update Roo Code provider reference"

create_bead "RC-agents" \
  "Fix Roo Code agent converter: add mcp tool group" \
  "bug" 2 \
  "WHY: renderRooCodeAgent omits mcp tool group. Agents using MCP tools lose that capability.

FINDINGS:
- RC-3: No mcp group mapping (agents.go:304-315,319)

SUCCESS CRITERIA:
- mcp group included in rendered Roo Code agents when MCP tools referenced
- Stable-order emit list includes 'mcp'

TESTING:
- Unit: Test agent render includes mcp group
- Unit: Test round-trip preserves MCP capability

DOCS (syllago-docs/): Update Roo Code agent format"

# ============================================================
# OPENCODE
# ============================================================

create_bead "OC-toolmap" \
  "Fix OpenCode toolmap: Read, WebSearch, Task all wrong" \
  "bug" 1 \
  "WHY: 3 of 6 OpenCode tool mappings produce wrong tool names in exported content.

FINDINGS:
- OC-1: Read -> view should be read (toolmap.go:13)
- OC-2: WebSearch -> fetch should be webfetch (toolmap.go:65)
- OC-3: Task -> agent should be task (toolmap.go:70)

SUCCESS CRITERIA:
- Read maps to 'read' for opencode
- WebSearch maps to 'webfetch' for opencode (note: websearch is separate Exa tool)
- Task maps to 'task' for opencode

TESTING:
- Unit: Test all corrected translations

DOCS (syllago-docs/): Update OpenCode tool mapping"

create_bead "OC-provider" \
  "Fix OpenCode provider: skills path plural, discovery improvements" \
  "bug" 2 \
  "WHY: Skills directory uses singular 'skill' instead of canonical plural 'skills'. Discovery misses CLAUDE.md fallback.

FINDINGS:
- OC-4: singular skill vs plural skills (opencode.go:25,50)
- OC-7: Missing CLAUDE.md fallback and config instructions discovery

SUCCESS CRITERIA:
- Skills paths use plural 'skills'
- Discovery includes both singular (backwards compat) and plural paths

TESTING:
- Unit: Test updated paths

DOCS (syllago-docs/): Update OpenCode provider reference"

# ============================================================
# KIRO
# ============================================================

create_bead "KI-toolmap" \
  "Fix Kiro toolmap: Glob/Grep mapped to read, add missing tools" \
  "bug" 1 \
  "WHY: Glob and Grep both map to 'read' — wrong tool, grants overly broad permissions. Kiro has separate glob and grep built-in tools. Also missing WebSearch, WebFetch, Task mappings.

FINDINGS:
- KI-1: Glob -> read should be glob, Grep -> read should be grep (toolmap.go:48-49,58-59)
- KI-4: Missing web_search, web_fetch, use_subagent/delegate

SUCCESS CRITERIA:
- Glob maps to 'glob' for kiro
- Grep maps to 'grep' for kiro
- WebSearch maps to 'web_search', WebFetch to 'web_fetch', Agent to 'use_subagent'

TESTING:
- Unit: Test all Kiro translations

DOCS (syllago-docs/): Update Kiro tool mapping"

create_bead "KI-agents" \
  "Fix Kiro agent format: JSON to markdown with YAML frontmatter" \
  "bug" 1 \
  "WHY: Syllago outputs Kiro agents as JSON but Kiro uses markdown with YAML frontmatter. Exported agents are in the wrong format and won't load.

FINDINGS:
- KI-2: kiroAgentConfig uses JSON tags, renderKiroAgent outputs .json (agents.go:386-393,449-492)
- KI-3: FileFormat returns JSON for agents (kiro.go:52-59)
- KI-5: Missing 10+ Kiro-specific agent fields (allowedTools, toolAliases, etc.)

SUCCESS CRITERIA:
- FileFormat returns FormatMarkdown for Kiro agents
- renderKiroAgent outputs .md with YAML frontmatter
- All documented Kiro agent fields supported in struct
- allowedTools (security-critical) properly handled

TESTING:
- Unit: Test render produces valid markdown agent file
- Unit: Test all Kiro-specific fields preserved
- Unit: Round-trip test

DOCS (syllago-docs/): Update Kiro agent format documentation"

# ============================================================
# PRINT SUMMARY
# ============================================================

echo ""
echo "=== BEAD CREATION COMPLETE ==="
echo "Total beads created: ${#BEAD_IDS[@]}"
echo ""
echo "Bead ID mapping:"
for key in $(echo "${!BEAD_IDS[@]}" | tr ' ' '\n' | sort); do
  echo "  $key -> ${BEAD_IDS[$key]}"
done
