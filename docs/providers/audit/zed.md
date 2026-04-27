# Zed Provider Audit

Audit date: 2026-03-21
Research sources: docs/providers/zed/{tools,content-types,hooks,skills-agents}.md

---

## Inaccuracies

### 1. Tool name: `subagent` should be `spawn_agent`

**File:** `cli/internal/converter/toolmap.go:71`
**Current:** `"zed": "subagent"`
**Research says:** The tool is called `spawn_agent`. The research notes it is "labeled 'subagent' in some docs" but the canonical Zed tool name is `spawn_agent` (tools.md line 43).
**Impact:** Tool name translation in rules/agent content will produce the wrong name when targeting Zed. Any rule that says "use the subagent tool" would work colloquially but formal tool permission configs (`agent.tool_permissions.tools`) require exact names.
**Fix:** Change line 71 to `"zed": "spawn_agent"`.

### 2. MCP tool name separator: Zed uses colon, not slash

**File:** `cli/internal/converter/toolmap.go:147,159,189`
**Current:** Zed is grouped with `copilot-cli` using slash format `server/tool`.
**Research says:** Zed's MCP tool permission format is `mcp:<server>:<tool_name>` (content-types.md line 100, skills-agents.md line 185-198). This is a colon-separated triple, not a slash-separated pair.
**Impact:** `TranslateMCPToolName` targeting Zed produces `server/tool` but Zed expects `mcp:server:tool`. Tool permission configs generated for Zed will not match.
**Fix:** Add Zed as its own case in `TranslateMCPToolName` and `parseMCPToolName` using the `mcp:<server>:<tool>` format.

### 3. Missing `WebFetch` / `fetch` tool mapping for Zed

**File:** `cli/internal/converter/toolmap.go:63-66`
**Current:** `WebSearch` has a Zed mapping, but there is no `WebFetch` canonical tool. Zed's `fetch` tool (URL content retrieval) has no mapping at all.
**Research says:** Zed has both `web_search` (mapped) and `fetch` (unmapped). `fetch` is the equivalent of Claude Code's `WebFetch`.
**Impact:** Rules referencing `WebFetch` won't translate to Zed's `fetch` tool. Low severity since `WebFetch` is not commonly referenced in tool permission lists.
**Fix:** Add a `"WebFetch"` entry to ToolNames with `"zed": "fetch"`.

---

## Gaps

### 4. Native scan does not discover Zed's `.rules` file

**File:** `cli/internal/catalog/native_scan.go:282-283`
**Current:** The Zed native pattern is `{path: ".zed", typeLabel: "settings"}`, which scans the `.zed` directory. There is no pattern for `.rules` at the project root.
**Research says:** Zed's primary rule file is `.rules` at the project root. Zed also reads 8 other filenames (`.cursorrules`, `.windsurfrules`, `.clinerules`, `.github/copilot-instructions.md`, `AGENT.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`).
**Impact:** When importing from a directory containing a `.rules` file, syllago's native scan won't attribute it to Zed. Some of those other filenames are already scanned under their respective providers (`.cursorrules` -> Cursor, etc.), so cross-attribution is partially covered, but `.rules` itself is Zed-specific and completely missed.
**Fix:** Add `{providerSlug: "zed", providerName: "Zed", path: ".rules", typeLabel: "rules"}` to `providerNativePatterns()`.

### 5. Missing tool mappings for 10 Zed-specific tools

**File:** `cli/internal/converter/toolmap.go`
**Research says:** Zed has 19 built-in tools. Syllago maps 7 (read_file, edit_file, find_path, grep, terminal, web_search, subagent). The following have no canonical equivalent and thus no mapping:
- `list_directory` (could map to a hypothetical ListDir, or note as Bash equivalent)
- `diagnostics` (no cross-provider equivalent)
- `now` (no equivalent)
- `thinking` (maps to extended thinking, not a tool)
- `copy_path`, `create_directory`, `delete_path`, `move_path` (all Bash equivalents)
- `open`, `save_file`, `restore_file_from_disk` (editor-specific, no equivalent)

**Impact:** Low. These tools are Zed-specific and don't appear in cross-provider content. Tool name translation only matters for tools referenced in rules or agent configs. If a Zed rule references `list_directory`, converting to Claude Code would leave it untranslated (which is the correct fallback behavior -- returns original name).

### 6. No Zed MCP scan in native patterns

**File:** `cli/internal/catalog/native_scan.go:282-283`
**Current:** Zed scan only looks at `.zed` directory as generic "settings". No embedded MCP extraction from `.zed/settings.json`.
**Research says:** Zed MCP configs live in `settings.json` under `context_servers`.
**Impact:** Native scan won't discover MCP servers configured in a project-local `.zed/settings.json`. The converter and installer handle `context_servers` correctly, but discovery is missing.
**Fix:** Add `{providerSlug: "zed", providerName: "Zed", path: ".zed/settings.json", typeLabel: "mcp", embedded: true}` and ensure `extractEmbeddedMCP` handles the `context_servers` key (currently it only looks for `mcpServers`).

### 7. `extractEmbeddedMCP` only checks `mcpServers` key

**File:** `cli/internal/catalog/native_scan.go:228-231`
**Current:** `gjson.GetBytes(data, "mcpServers")` -- hardcoded to the Claude Code key.
**Impact:** Even if gap #6 is fixed and Zed's settings.json is scanned, MCP servers under `context_servers` would not be extracted. Also affects OpenCode which uses a `"mcp"` key.
**Fix:** Accept a key parameter or check provider-specific keys.

---

## Opportunities

### 8. Zed tool permissions as exportable content

Research shows Zed has a granular tool permissions system (`agent.tool_permissions` in settings.json) with pattern-based allow/deny/confirm rules. This is richer than most providers. If syllago ever supports a "tool permissions" or "security policy" content type, Zed's format would be a good reference.

### 9. Zed Agent Profiles as potential content type

Agent profiles (Write/Ask/Minimal/Custom) group tools into permission sets. No other provider has this exact concept, but it maps loosely to a combination of tool permissions and rules. Could be a future syllago content type for Zed-specific loadouts.

### 10. `.rules` file as universal exchange format

Zed reads 9 different rule filenames including most other providers' native formats. This means exporting as `.rules` gives Zed-native content, while exporting as `.cursorrules` or `CLAUDE.md` also works in Zed. The current approach of emitting `.rules` is correct and optimal.
