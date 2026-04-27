# Roo Code Provider Audit

Audit of syllago codebase accuracy against Roo Code provider research (March 2026).

Research sources: `docs/providers/roo-code/{tools,content-types,hooks,skills-agents}.md`

---

## Inaccuracies

### 1. Tool names use PascalCase class names instead of snake_case tool-use names

**File:** `cli/internal/converter/toolmap.go:16-62`

The `ToolNames` map uses PascalCase source-code class names (`ReadFileTool`, `WriteToFileTool`, `ExecuteCommandTool`, etc.) for the `roo-code` entries. Research confirms that Roo Code's AI-facing tool interface uses **snake_case** names: `read_file`, `write_to_file`, `execute_command`, `search_files`, `list_files`.

The PascalCase names are internal TypeScript class names (e.g., `src/core/tools/ReadFileTool.ts`), not the names used in tool-use XML or rules/instructions.

**Current:**
```go
"roo-code": "ReadFileTool",    // class name
"roo-code": "WriteToFileTool", // class name
"roo-code": "ExecuteCommandTool", // class name
```

**Should be:**
```go
"roo-code": "read_file",
"roo-code": "write_to_file",
"roo-code": "execute_command",
```

**Impact:** Tool name translation in rules/instructions would produce names the AI model doesn't recognize. Any rule referencing `ReadFileTool` instead of `read_file` would fail to match Roo Code's tool-use interface.

### 2. MCP config missing `alwaysAllow` field

**File:** `cli/internal/converter/mcp.go:51-60`

The `rooCodeMCPServerConfig` struct lacks an `alwaysAllow` field. Research confirms Roo Code supports `alwaysAllow` (array of tool names to auto-approve) in its MCP config, identical to Cline's format. The Cline struct (`clineMCPServerConfig`, line 68) correctly includes it.

The `renderRooCodeMCP` function (line 501) silently drops `autoApprove` data when converting to Roo Code format. Worse, it warns that `autoApprove` is "dropped (not supported by Roo Code)" (line 517) -- but Roo Code does support it as `alwaysAllow`.

**Current struct:**
```go
type rooCodeMCPServerConfig struct {
    Command  string            `json:"command,omitempty"`
    Args     []string          `json:"args,omitempty"`
    Env      map[string]string `json:"env,omitempty"`
    Disabled bool              `json:"disabled,omitempty"`
    Type     string            `json:"type,omitempty"`
    URL      string            `json:"url,omitempty"`
}
```

**Should include:** `AlwaysAllow []string \`json:"alwaysAllow,omitempty"\``

**Impact:** Converting MCP configs to Roo Code silently drops auto-approval lists and emits a misleading warning.

### 3. Agent converter omits `mcp` tool group

**File:** `cli/internal/converter/agents.go:304-315`

The `renderRooCodeAgent` function maps canonical tools to Roo Code tool groups but has no mapping that produces the `mcp` group. Research shows `mcp` (`use_mcp_tool`, `access_mcp_resource`) is a standard tool group available in most built-in modes. Any agent that uses MCP tools would lose that capability on conversion to a Roo Code custom mode.

The stable-order emit list (line 319) also omits `"mcp"` from the iteration order.

---

## Gaps

### 4. Skills content type not supported

**File:** `cli/internal/provider/roocode.go:61-68`

`SupportsType` returns `false` for `catalog.Skills`. Research documents a full skills system: `SKILL.md` files with YAML frontmatter (`name`, `description`), stored in `.roo/skills/{name}/SKILL.md` with mode-specific variants (`.roo/skills-{mode}/`), global paths (`~/.roo/skills/`), and cross-agent paths (`.agents/skills/`).

Skills are a distinct content type from rules -- they use progressive disclosure (frontmatter only at startup, full content on demand), have bundled resource files, and are invoked via the `skill` tool.

**Impact:** Cannot import/export Roo Code skills. This is the most mature on-demand instruction system among providers.

### 5. Custom Tools content type not represented

Research documents custom tools (`.roo/tools/` TypeScript/JavaScript files using `defineCustomTool()`). These are user-defined tools the AI can invoke. No content type maps to this in syllago's model.

**Impact:** Low priority since this is experimental in Roo Code and TypeScript execution isn't portable across providers. Worth noting for completeness.

### 6. `.roomodes` not in DiscoveryPaths for Agents

**File:** `cli/internal/provider/roocode.go:30-47`

`DiscoveryPaths` returns `nil` for `catalog.Agents`. Custom modes are stored in `.roomodes` (project) and global settings YAML. The provider should discover these for catalog listing even if the format is complex.

### 7. Global rules paths missing from DiscoveryPaths

**File:** `cli/internal/provider/roocode.go:33-41`

Research documents global rule paths: `~/.roo/rules/` and `~/.roo/rules-{mode}/`. The current `DiscoveryPaths` only covers project-level paths. Global rules would not be discovered by the catalog.

### 8. Legacy `.clinerules` / `.roorules-{mode}` paths not discovered

**File:** `cli/internal/provider/roocode.go:33-41`

The discovery paths include `.roorules` (single file) but not mode-specific single-file fallbacks (`.roorules-{mode}`) or Cline legacy files (`.clinerules`, `.clinerules-{mode}`). These are lower priority but exist in the wild.

### 9. Tool map covers only 6 of 25+ tools

**File:** `cli/internal/converter/toolmap.go:8-73`

Only 6 tools are mapped (Read, Write, Edit, Bash, Glob, Grep). Research documents 25+ tools including: `codebase_search`, `apply_diff`, `apply_patch`, `edit_file`, `insert_content`, `browser_action`, `use_mcp_tool`, `access_mcp_resource`, `switch_mode`, `new_task`, `skill`, `generate_image`, etc.

Missing mappings that have cross-provider equivalents:
- `WebSearch` has no roo-code entry (Roo Code has `browser_action` for web access)
- `Task` has no roo-code entry (Roo Code has `new_task` for subtask delegation)
- No canonical equivalent for `codebase_search` (semantic search) or `apply_patch` (multi-file diff)

### 10. No hooks support confirmed correctly, but no conversion warning

**File:** `cli/internal/converter/toolmap.go:76-87`

The `HookEvents` map correctly has no `roo-code` entries -- research confirms Roo Code has no hooks system. However, when converting hook content targeting Roo Code, there's no explicit unsupported-provider handling. The `TranslateHookEvent` function returns `(event, false)` which callers may or may not handle.

### 11. Detection heuristic may false-positive

**File:** `cli/internal/provider/roocode.go:26-29`

Detection checks for `~/.roo/` directory existence. This is reasonable but could false-positive if the directory was created manually or by another tool. Cline legacy users might also have `.roo/` from migration attempts. Low severity.

---

## Opportunities

### 12. Skills as a differentiator for Roo Code support

Roo Code's skills system (SKILL.md with frontmatter, progressive disclosure, bundled resources, mode-specific paths) is the most structured on-demand instruction format among supported providers. Adding `catalog.Skills` support for Roo Code would enable:
- Import skills from Roo Code for conversion to other providers' formats
- Export skills to Roo Code from syllago's canonical format
- Round-trip skills through the hub-and-spoke model

### 13. Custom modes as the agent conversion target

The `renderRooCodeAgent` function already converts to custom modes. The YAML format is well-documented with clear field mappings. Extending this to support mode import (parsing `.roomodes` YAML/JSON) would complete the bidirectional agent conversion.

### 14. Roo Code MCP format is nearly identical to Cline

The Roo Code MCP format shares the same `mcpServers` key and `alwaysAllow` field as Cline. Once the `alwaysAllow` fix is applied, the Roo Code renderer could potentially share code with the Cline renderer, reducing duplication. The only Roo Code additions are `type` (for SSE servers) and `url` fields, plus Streamable HTTP support.

---

## Summary

| Category | Count | Severity |
|----------|-------|----------|
| Inaccuracies | 3 | High (tool names), Medium (MCP field), Low (mcp group) |
| Gaps | 8 | High (skills), Medium (discovery paths, tool coverage), Low (legacy paths, detection) |
| Opportunities | 3 | -- |

**Highest priority fixes:**
1. Fix tool name casing (snake_case, not PascalCase) -- blocks correct rule conversion
2. Add `alwaysAllow` to Roo Code MCP struct -- data loss on conversion
3. Add Skills support -- major Roo Code feature completely missing
