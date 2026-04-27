# Gemini CLI Provider Audit

Audit of syllago codebase accuracy against Gemini CLI provider research.
Date: 2026-03-21

Research files:
- `docs/providers/gemini-cli/tools.md`
- `docs/providers/gemini-cli/content-types.md`
- `docs/providers/gemini-cli/hooks.md`
- `docs/providers/gemini-cli/skills-agents.md`

---

## Inaccuracies

### 1. Tool name: `Glob` maps to `list_directory` -- should map to `glob`

**File:** `cli/internal/converter/toolmap.go:46`
**Current:** `"Glob": {"gemini-cli": "list_directory", ...}`
**Should be:** `"Glob": {"gemini-cli": "glob", ...}`

Gemini CLI has both `list_directory` (lists a single directory) and `glob` (finds files by glob pattern). These are distinct tools. The canonical `Glob` tool matches files by pattern, which maps to Gemini's `glob` (FindFiles), not `list_directory` (ReadFolder). `list_directory` is closer to `ls` -- Claude Code has no dedicated equivalent (it uses `Bash` with `ls`).

**Research source:** tools.md lines 139-155 (glob tool) vs lines 121-135 (list_directory tool).

**Impact:** Hooks using `Glob` matchers in BeforeTool/AfterTool would match the wrong Gemini tool. Agent tool restrictions would grant directory listing instead of file search.

### 2. Tool name: `WebSearch` maps to `google_search` -- should be `google_web_search`

**File:** `cli/internal/converter/toolmap.go:64`
**Current:** `"WebSearch": {"gemini-cli": "google_search", ...}`
**Should be:** `"WebSearch": {"gemini-cli": "google_web_search", ...}`

The official Gemini CLI tool name is `google_web_search`. The hooks research file (hooks.md line 787) lists `google_search` as a tool name, but this appears to be a simplification -- the tools research (tools.md line 213) and the built-in tools table (content-types.md line 1061) both use `google_web_search` as the canonical function name.

**Research source:** tools.md lines 213-226, content-types.md line 1061.

**Impact:** Hook matchers and agent tool allowlists targeting web search would silently fail to match.

**Note:** The hooks.md "Built-in Tool Names" section (line 787) says `google_search`, creating a conflict within our own research. The tools.md research (sourced from official tool docs) is more authoritative. This research discrepancy should be resolved by checking the Gemini CLI source.

### 3. Tool name: `Grep` maps to `grep_search` -- likely should be `search_file_content`

**File:** `cli/internal/converter/toolmap.go:55`
**Current:** `"Grep": {"gemini-cli": "grep_search", ...}`
**Likely should be:** `"Grep": {"gemini-cli": "search_file_content", ...}`

The tools research (tools.md line 158-162) documents the official function name as `search_file_content`, noting that `grep_search` appears in some config contexts (tools.core allowlists) as an alias. The content-types.md built-in tools table (line 1058) lists both: `grep_search` / `search_file_content`.

**Research source:** tools.md lines 158-162, content-types.md line 1058.

**Impact:** Moderate. If `grep_search` is a recognized alias, this may work in practice for tool allowlists and hook matchers. But the primary function name is `search_file_content`. The safest choice depends on which name the Gemini CLI actually uses in its `tool_name` field when firing BeforeTool/AfterTool events. Needs live verification.

### 4. MCP tool name format: syllago uses `server__tool` but Gemini docs say `mcp_server_tool`

**File:** `cli/internal/converter/toolmap.go:147,157,181-187`
**Current:** Gemini CLI grouped with "bare double-underscore" pattern: `server__tool`
**Docs say:** `mcp_<server_name>_<tool_name>` (single underscores with `mcp_` prefix)

The hooks research (hooks.md line 308-313, line 791) documents the Gemini CLI MCP tool pattern as `mcp_<server_name>_<tool_name>` with single underscores. The research notes (hooks.md line 313) explicitly flags this discrepancy: "syllago's toolmap uses `server__tool` (double underscores, no prefix) for the actual matching format."

**Research source:** hooks.md lines 308-313, 791, and the Syllago Mapping section lines 876-879.

**Impact:** MCP hook matchers converted to/from Gemini CLI format would produce tool names in the wrong format. For example, a Claude Code hook matching `mcp__github__get_issue` would convert to `github__get_issue` (syllago's format) instead of `mcp_github_get_issue` (Gemini's documented format).

**Note:** This needs live verification. The research marks this as `[Inferred -- verify against live behavior]`. The correct format is critical for hook interop.

---

## Gaps

### 5. Missing hook events: BeforeModel, AfterModel, BeforeToolSelection

**File:** `cli/internal/converter/toolmap.go:76-87` (HookEvents map)

The HookEvents map has no entries for Gemini CLI's three model-level events:
- `BeforeModel` -- modify/mock LLM requests
- `AfterModel` -- process/redact LLM responses
- `BeforeToolSelection` -- filter available tools

These have no Claude Code equivalent, so they can't be mapped to canonical names. The hooks research (hooks.md lines 885-891) correctly notes this. However, there's currently no handling for these events during import -- if someone imports a Gemini CLI settings.json with BeforeModel hooks, they would be silently dropped with no warning.

**Impact:** Loss of Gemini-specific hooks on import without user notification. Low priority since these events are Gemini-unique, but a warning during conversion would be appropriate.

### 6. No `list_directory` tool mapping

With the Glob fix (inaccuracy #1), there's still no canonical mapping for Gemini's `list_directory`. Claude Code handles this via `Bash` with `ls`, which isn't a clean 1:1 mapping. This means:
- Importing a Gemini agent with `list_directory` in its tools list would pass through untranslated
- Exporting to Gemini CLI loses the concept entirely

**Impact:** Low. Most agents don't explicitly list `list_directory` as a tool restriction.

### 7. Commands InstallDir returns `.gemini/` base, not `.gemini/commands/`

**File:** `cli/internal/provider/gemini.go:23-24`
**Current:** `case catalog.Commands: return base` (where base = `~/.gemini`)
**Research says:** Commands live in `~/.gemini/commands/*.toml` (content-types.md lines 380-386)

The InstallDir returns `.gemini/` for commands, but Gemini CLI expects command TOML files in `.gemini/commands/`. This may be intentional if the installer appends `commands/` itself, but it differs from the Skills case (line 18) which returns `filepath.Join(base, "skills")`.

**Impact:** Needs verification. If the installer doesn't append the `commands/` subdirectory, commands would be installed to the wrong location.

### 8. No `search_file_content` alias handling in reverse translation

**File:** `cli/internal/converter/toolmap.go:134-141`

`ReverseTranslateTool` does exact matching. If Gemini CLI uses `search_file_content` as the tool name in some contexts and `grep_search` in others, only one will reverse-translate correctly. There's no alias/fallback mechanism.

### 9. Skills research mentions `.agents/skills/` as alternate discovery path

**Research:** content-types.md line 819, skills-agents.md line 523

Gemini CLI discovers skills from both `.gemini/skills/` and `.agents/skills/`. The syllago provider definition (gemini.go line 42) only uses `.gemini/skills/` for DiscoveryPaths. Skills placed in `.agents/skills/` would not be discovered.

**Impact:** Low. `.agents/skills/` is a secondary path unlikely to be used by most projects.

### 10. No support for Gemini extensions as a content type

Gemini CLI has a first-class extension system (`gemini-extension.json` manifests bundling commands, skills, agents, MCP servers, hooks, policies, and themes). Syllago has no content type or converter for extensions.

**Impact:** Cannot import/export Gemini extensions as a unit. Individual pieces (skills, agents, commands) can be converted separately. This is a feature gap, not a bug.

### 11. No support for Gemini policy engine (TOML rules)

Gemini CLI has a unique TOML-based policy engine for tool permissions with 5-tier priority. Syllago has no content type for policies.

**Impact:** Policy rules are dropped on import. Enterprise users who rely on policy rules would lose access controls.

---

## Opportunities

### 12. Hook event coverage is strong

The 8 mapped events (PreToolUse, PostToolUse, UserPromptSubmit, Stop, SessionStart, SessionEnd, PreCompact, Notification) cover all the events that have cross-provider equivalents. The 3 unmapped events (BeforeModel, AfterModel, BeforeToolSelection) are genuinely Gemini-unique. Good design decision to not force-map them.

### 13. Agent converter handles Gemini-specific fields well

The `temperature`, `timeout_mins`, and `kind` fields are preserved in canonical format for lossless round-trips. When rendering to non-Gemini targets, they're embedded as conversion notes. This is solid.

### 14. MCP converter handles Gemini-specific fields (trust, includeTools, excludeTools)

The converter properly preserves these fields when targeting Gemini and warns when dropping them for other providers. The `httpUrl` alternate transport field is also handled.

### 15. Commands converter outputs TOML format

Research confirms Gemini CLI uses TOML for custom commands. The converter (commands.go) outputs `command.toml`, which matches the expected format.

---

## Priority Summary

| # | Type | Severity | Effort |
|---|------|----------|--------|
| 1 | Inaccuracy: Glob -> list_directory | **High** | Low (one-line fix) |
| 2 | Inaccuracy: google_search -> google_web_search | **High** | Low (one-line fix, needs verification) |
| 3 | Inaccuracy: grep_search vs search_file_content | **Medium** | Low (needs live verification) |
| 4 | Inaccuracy: MCP tool name format | **High** | Medium (format change + tests) |
| 5 | Gap: Missing model event warnings | Low | Low |
| 7 | Gap: Commands InstallDir path | **Medium** | Low (verify installer behavior) |
| 9 | Gap: .agents/skills/ discovery path | Low | Low |
| 10 | Gap: No extension content type | Low | High (new feature) |
| 11 | Gap: No policy content type | Low | High (new feature) |
