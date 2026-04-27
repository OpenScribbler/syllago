# Cursor Provider Audit

Audit of syllago's Cursor implementation against provider research.
Date: 2026-03-21

Research files:
- `docs/providers/cursor/tools.md`
- `docs/providers/cursor/content-types.md`
- `docs/providers/cursor/hooks.md`
- `docs/providers/cursor/skills-agents.md`

---

## Inaccuracies

### 1. Cursor marked as hookless provider (WRONG)

**File:** `cli/internal/converter/hooks.go:209`

Cursor is listed in `hooklessProviders`, meaning hook conversion to Cursor emits a warning and produces no output. However, the research shows Cursor has a full hooks system (`hooks.json`) with 20+ event types, JSON stdin/stdout protocol, and blocking/non-blocking semantics.

Cursor's hook schema is structurally different from Claude Code's (separate `hooks.json` file vs. embedded in `settings.json`, JSON stdin vs. env vars, JSON stdout vs. exit codes), but it IS a hooks provider. The hookless designation is factually wrong.

**Impact:** Users cannot convert hooks TO Cursor. Hooks FROM Cursor also have no canonicalization path (no `case "cursor":` in hooks `Canonicalize` switch).

### 2. Cursor provider only supports Rules (INCOMPLETE)

**File:** `cli/internal/provider/cursor.go:44-51`

`SupportsType` returns `true` only for `catalog.Rules`. Research shows Cursor supports:

| Content Type | Cursor Support | syllago Support |
|-------------|---------------|-----------------|
| Rules (.mdc) | Yes | Yes |
| Skills (SKILL.md) | Yes (.cursor/skills/, .agents/skills/) | **No** |
| Commands (.cursor/commands/) | Yes | **No** |
| Hooks (hooks.json) | Yes | **No** (marked hookless) |
| MCP (mcp.json) | Yes (.cursor/mcp.json) | **No** |
| Agents/Subagents (.cursor/agents/) | Yes | **No** |

This is confirmed in the test at `cli/internal/provider/provider_test.go:54`:
```go
if Cursor.SupportsType(catalog.Skills) {
    t.Error("Cursor.SupportsType(Skills) = true, want false")
}
```
The test asserts the wrong behavior.

### 3. Cursor missing from toolmap entirely

**File:** `cli/internal/converter/toolmap.go`

Cursor has no entries in either `ToolNames` or `HookEvents` maps. Research documents 20 tools with clear cross-provider mappings:

| Cursor Tool | Canonical (Claude Code) |
|------------|------------------------|
| `read_file` | `Read` |
| `edit_file` | `Edit` / `Write` |
| `run_terminal_cmd` | `Bash` |
| `grep_search` / `grep` | `Grep` |
| `file_search` / `glob_file_search` | `Glob` |
| `web_search` | `WebSearch` |
| `codebase_search` | (no direct canonical) |
| `edit_notebook` | (NotebookEdit equivalent) |

And hook events:

| Cursor Event | Canonical |
|-------------|-----------|
| `preToolUse` | `PreToolUse` |
| `postToolUse` | `PostToolUse` |
| `beforeSubmitPrompt` / `beforeShellExecution` | `UserPromptSubmit` / (no match) |
| `sessionStart` | `SessionStart` |
| `sessionEnd` | `SessionEnd` |
| `stop` | `Stop` |
| `subagentStart` | `SubagentStart` |
| `subagentStop` | `SubagentCompleted` |
| `preCompact` | `PreCompact` |

### 4. InstallDir only handles Rules

**File:** `cli/internal/provider/cursor.go:14-19`

`InstallDir` returns a path only for `catalog.Rules` (and returns `.cursor` rather than `.cursor/rules`). Missing install paths for Skills, MCP, Hooks, and Commands.

Expected install paths from research:

| Content Type | Install Path |
|-------------|-------------|
| Rules | `.cursor/rules/` |
| Skills | `.cursor/skills/` |
| Commands | `.cursor/commands/` |
| Hooks | `__json_merge__` (merge into `.cursor/hooks.json`) |
| MCP | `__json_merge__` (merge into `.cursor/mcp.json`) |
| Agents | `.cursor/agents/` |

### 5. DiscoveryPaths only handles Rules

**File:** `cli/internal/provider/cursor.go:25-32`

Only discovers rules from `.cursor/rules`. Missing discovery paths:

- Skills: `.cursor/skills/`, `.agents/skills/`
- Commands: `.cursor/commands/`
- Hooks: `.cursor/hooks.json`
- MCP: `.cursor/mcp.json`
- Agents: `.cursor/agents/`

---

## Gaps

### G1. No Cursor skills support

Cursor has a full skills system with `SKILL.md` files, YAML frontmatter (`name`, `description`, `license`, `compatibility`, `metadata`, `disable-model-invocation`), and directory structure (`scripts/`, `references/`, `assets/`). The skills converter (`cli/internal/converter/skills.go`) has no `case "cursor":` in either `Canonicalize` or `Render`.

Cursor skills share the `SKILL.md` filename convention with Claude Code but have a different frontmatter schema. The canonical `SkillMeta` struct includes Claude-specific fields (`allowed-tools`, `disallowed-tools`, `context`, `agent`, `model`, `effort`, `hooks`) that Cursor does not support. Cursor has `license`, `compatibility`, `metadata`, and `disable-model-invocation` which are NOT in the canonical struct.

### G2. No Cursor commands support

Cursor commands (`.cursor/commands/*.md`) are simple markdown files with no frontmatter. syllago has a `catalog.Commands` content type but no Cursor-specific handling. Commands are being superseded by Skills in Cursor, but still functional.

### G3. No MCP config support for Cursor

Cursor uses `.cursor/mcp.json` with `mcpServers` key, supporting STDIO, SSE, and Streamable HTTP transports plus variable interpolation (`${env:NAME}`, `${userHome}`, etc.). No converter or installer path exists.

### G4. No Cursor subagent/agent support

Cursor supports custom subagents (`.cursor/agents/<name>.md`) with frontmatter fields: `name`, `description`, `model` (inherit/fast/model-id), `readonly`, `is_background`. syllago has `catalog.Agents` but no Cursor-specific handling.

Cursor also reads agents from cross-provider directories: `.claude/agents/`, `.codex/agents/`.

### G5. No hooks.json canonicalization FROM Cursor

The hooks converter `Canonicalize` method has no `case "cursor":` branch. Cursor hooks use a unique schema (`version`, `hooks` map, with `command`, `type`, `timeout`, `failClosed`, `loop_limit`, `matcher` fields) distinct from Claude Code's format.

### G6. Cursor hook event set is much richer than mapped

Research documents 20+ hook events including agent lifecycle, tool use, shell execution, MCP execution, file operations, subagent events, tab events, and reasoning events. The `HookEvents` map has no Cursor entries at all.

### G7. .cursorrules legacy file not discovered

The deprecated `.cursorrules` file (project root, no frontmatter, always-on) is documented in research but not in `DiscoveryPaths`. Users migrating from older Cursor setups would have this file.

### G8. AGENTS.md cross-provider file not associated with Cursor

Research confirms Cursor reads `AGENTS.md` files from project root and subdirectories. This cross-provider format is not reflected in Cursor's discovery paths.

### G9. Cursor MCP tool name format unknown

`TranslateMCPToolName` in `toolmap.go` maps MCP tool naming conventions per provider but has no Cursor case. Research shows Cursor hooks reference MCP tools as `"MCP:toolname"` in matchers, suggesting a colon-separated format.

---

## Opportunities

### O1. Quick win: Add Cursor to toolmap

The cross-provider tool mapping table in `tools.md` gives exact names. Adding Cursor entries to `ToolNames` and `HookEvents` in `toolmap.go` is mechanical work with clear mappings. This enables tool name translation in rules/skills conversion notes.

### O2. Quick win: Enable Cursor hooks support

Remove Cursor from `hooklessProviders`. Add Cursor-specific canonicalization (parse `hooks.json` with `version` + `hooks` map structure) and rendering (emit `hooks.json` format with Cursor-specific fields like `failClosed`, `loop_limit`, `matcher`). The hooks converter already handles nested hook formats for other providers.

### O3. Expand SupportsType for Cursor

Cursor supports 6 of syllago's 7 content types (all except Prompts). Expanding `SupportsType`, `InstallDir`, `DiscoveryPaths`, and `FileFormat` is the foundation for all other Cursor gaps.

### O4. Skills converter for Cursor

Cursor's `SKILL.md` format is close enough to the canonical format that conversion is straightforward. Key differences to handle:
- Cursor has `license`, `compatibility`, `metadata`, `disable-model-invocation`
- Cursor lacks `allowed-tools`, `disallowed-tools`, `context`, `agent`, `model`, `effort`, `hooks`
- `disable-model-invocation` maps conceptually to Claude Code's usage patterns but has no structural equivalent

### O5. MCP config converter

Cursor's `.cursor/mcp.json` schema (`mcpServers` key with STDIO/SSE configs) is very similar to other providers' MCP formats. Variable interpolation syntax (`${env:NAME}`) may need translation.

### O6. Subagent converter

Cursor's subagent format is simple markdown with YAML frontmatter. The `model`, `readonly`, and `is_background` fields are Cursor-specific but map to behavioral concepts in other providers' agent systems.

---

## Summary

syllago's Cursor support is rules-only. The research reveals Cursor has grown into a full-featured platform with skills, commands, hooks, MCP, and subagents -- all of which syllago's architecture already models as content types but has no Cursor implementation for. The most impactful fix is correcting the `hooklessProviders` designation (factually wrong), followed by expanding `SupportsType` and adding converter cases for each content type.
