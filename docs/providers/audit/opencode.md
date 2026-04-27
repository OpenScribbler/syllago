# OpenCode Provider Audit

Audit of syllago codebase accuracy against OpenCode provider research.
Date: 2026-03-21

## Inaccuracies

### 1. Tool mapping: Read mapped to "view" instead of "read"

**File:** `cli/internal/converter/toolmap.go:13`
**Current:** `"opencode": "view"`
**Research says:** OpenCode's file reading tool is called `read` (tools.md line 17, research table line 47)
**Impact:** Tool name references in converted rules/skills will use the wrong name.

### 2. Tool mapping: WebSearch mapped to "fetch" instead of "webfetch"

**File:** `cli/internal/converter/toolmap.go:65`
**Current:** `"opencode": "fetch"`
**Research says:** OpenCode's web fetching tool is called `webfetch` (tools.md line 21). There is also a separate `websearch` tool (Exa AI). Neither is called "fetch".
**Impact:** Converted tool references will use a non-existent tool name.

### 3. Tool mapping: Task mapped to "agent" instead of "task"

**File:** `cli/internal/converter/toolmap.go:70`
**Current:** `"opencode": "agent"`
**Research says:** OpenCode's subagent delegation tool is called `task` (tools.md line 28, research mapping table line 161). Agents are a separate concept (the configured AI personas); `task` is the tool that invokes subagents.
**Impact:** Converted tool references will use the wrong name.

### 4. Skills install directory uses "skill" (singular) instead of "skills" (plural)

**File:** `cli/internal/provider/opencode.go:25`
**Current:** `return filepath.Join(base, "skill")`
**Research says:** OpenCode uses `~/.config/opencode/skills/` (plural) per content-types.md directory structure (line 207). The research also notes singular directory names are supported for backwards compatibility (line 222), but the primary convention is plural.

**File:** `cli/internal/provider/opencode.go:50`
**Current:** `return []string{filepath.Join(projectRoot, ".opencode", "skill")}`
**Research says:** Project-level path is `.opencode/skills/` (plural, content-types.md line 209).
**Impact:** Content installed to the singular path will work (backwards compat), but discovery only checks singular -- will miss content at the canonical plural path.

### 5. Hook events have no OpenCode mappings

**File:** `cli/internal/converter/toolmap.go:76-87`
**Current:** The `HookEvents` map has zero entries for `"opencode"`.
**Research says:** OpenCode has 25+ plugin events (hooks.md). While they are programmatic (TypeScript) rather than declarative (JSON), partial mappings exist:
- `PreToolUse` maps to `tool.execute.before`
- `PostToolUse` maps to `tool.execute.after`
- `SessionStart` maps to `session.created`
- `Stop` maps to `session.idle`

**Impact:** Hook conversion to/from OpenCode silently drops all events. This is arguably correct since OpenCode hooks are fundamentally different (programmatic vs declarative), but the research doc explicitly maps these events. At minimum, this should be a documented gap, not a silent failure.

## Gaps

### 1. No Hooks/Plugins content type support

**File:** `cli/internal/provider/opencode.go:71-77`
**Current:** `SupportsType` returns false for `catalog.Hooks`.
**Research says:** OpenCode has a rich plugin/event system (hooks.md). While the format is fundamentally different (TypeScript files vs JSON config), the research doc identifies clear event-level mappings. Cross-provider hook conversion would require code generation, which is a significant architectural choice.
**Severity:** Expected gap given the format mismatch. Worth tracking as a future opportunity.

### 2. No Prompts content type support

**Current:** `SupportsType` returns false for prompts.
**Research says:** OpenCode commands with `template` fields are the closest equivalent to prompts (content-types.md line 234). Commands already supported; prompts could potentially map to commands.
**Severity:** Low -- commands cover the use case.

### 3. Missing OpenCode tools with no syllago canonical equivalent

The research identifies 8 OpenCode tools with no syllago mapping:
- `patch` -- diff/patch application (related to Edit but distinct)
- `list` -- directory listing (related to Glob but distinct)
- `lsp` -- LSP code intelligence (unique)
- `skill` -- loads SKILL.md content (unique)
- `todowrite` / `todoread` -- task tracking (unique)
- `question` -- user interaction (unique)
- `websearch` -- Exa AI web search (distinct from webfetch)

**Impact:** Rules referencing these tools won't get translated. Acceptable since syllago's canonical tool set is based on Claude Code's tools.

### 4. MCP "remote" type mapping may be incomplete

**File:** `cli/internal/converter/mcp.go:185-188`
**Current:** `canonicalizeOpencodeMCP` maps `"remote"` to `"sse"`.
**Research says:** OpenCode supports `"remote"` type which covers both SSE and streamable-http transports. The research mentions OAuth and Dynamic Client Registration (RFC 7591) for remote servers. Mapping all remote servers to "sse" may lose the distinction.
**Severity:** Minor -- most remote MCP servers use SSE today.

### 5. Discovery misses CLAUDE.md fallback

**File:** `cli/internal/provider/opencode.go:43-44`
**Current:** Discovery only checks `AGENTS.md`.
**Research says:** OpenCode also reads `CLAUDE.md` as a fallback (content-types.md lines 68-72), disabled via `OPENCODE_DISABLE_CLAUDE_CODE` env var.
**Severity:** Low for syllago. We discover Claude Code content separately via the Claude Code provider. But if a user only has OpenCode installed, we won't discover their CLAUDE.md-based rules as OpenCode content.

### 6. Config-based instructions not discovered

**Research says:** OpenCode supports an `instructions` array in `opencode.json` pointing to glob paths and remote URLs (content-types.md lines 75-83). These are additional rule sources beyond AGENTS.md.
**Severity:** Medium -- teams using config-based instructions won't have them discovered.

## Opportunities

### 1. Add `patch` to canonical tool set

OpenCode's `patch` tool (apply diff/patch files) is distinct from `edit` (string replacement). Several providers have diff-based editing. Adding a canonical `Patch` tool could improve conversion fidelity for providers that distinguish between replacement-based and diff-based editing.

### 2. Add `list` to canonical tool set

OpenCode's `list` tool (directory listing) overlaps with `glob` but is semantically different (list contents of a path vs find files matching a pattern). Claude Code's `LS` built-in is similar. This is a minor gap.

### 3. Skill permission support in provider config

OpenCode has glob-pattern-based skill permissions (`"internal-*": "deny"`). If syllago ever supports permission metadata on content, this is a rich source of patterns.
