# Format Conversion Fidelity - Design Document

**Goal:** Complete the format conversion infrastructure so all 11 providers have accurate tool name translation, MCP merge support, hook event mappings, and Codex multi-agent TOML bidirectional conversion.

**Decision Date:** 2026-02-26

---

## Problem Statement

Syllago's format conversion currently has incomplete tool name translation tables (4 of 11 providers), missing MCP install paths for new providers, no Codex multi-agent format support, and gaps in hook event mappings. The v1 launch narrative promises "automatic format conversion... tells you if anything was lost" — that promise doesn't hold if 7 providers can't translate tool references or merge MCP configs.

## Proposed Solution

Six deliverables that complete the conversion infrastructure:

1. **Tool name translation tables** — Add mappings for OpenCode, Zed, Roo Code, Cline to `toolmap.go`. Verify existing Kiro mappings.
2. **Hook event translation tables** — Emit warnings for providers that don't support hooks (OpenCode, Zed, Cline, Roo Code).
3. **MCP tool name format** — Add MCP tool name patterns for new providers to `TranslateMCPToolName()`.
4. **MCP merge paths** — Extend `mcpConfigPath()` in `installer/mcp.go` for all 11 providers.
5. **Codex multi-agent TOML** — Full bidirectional converter: canonicalize TOML roles → canonical agents AND render canonical → TOML.
6. **Test coverage** — Roundtrip tests for every new provider pair and format.

---

## Architecture

### Tool Name Translation

**Current architecture** (stays the same): `toolmap.go` has a `ToolNames` map keyed by canonical name → provider slug → provider-specific name. Functions `TranslateTool()`, `ReverseTranslateTool()`, and `TranslateTools()` do the lookup.

**New mappings:**

| Canonical | OpenCode | Zed | Cline | Roo Code |
|-----------|----------|-----|-------|----------|
| Read | view | read_file | read_file | ReadFileTool |
| Write | write | edit_file | write_to_file | WriteToFileTool |
| Edit | edit | edit_file | apply_diff | EditFileTool |
| Bash | bash | terminal | execute_command | ExecuteCommandTool |
| Glob | glob | find_path | list_files | ListFilesTool |
| Grep | grep | grep | search_files | SearchFilesTool |
| WebSearch | fetch | web_search | — | — |
| Task | agent | subagent | — | — |

**Design decisions:**
- Cline and Roo Code don't have web search or sub-task tools — translate with warning
- Zed's `edit_file` serves both Write and Edit — reverse translator handles disambiguation
- Roo Code uses PascalCase class names — that's what content authors reference in custom modes
- Unknown tools pass through with a warning (existing behavior preserved)

**MCP tool name format additions:**

| Provider | Format | Example |
|----------|--------|---------|
| OpenCode | `servername__toolname` | `myserver__read` |
| Zed | `servername/toolname` | `myserver/read` |
| Cline | `servername__toolname` | `myserver__read` |
| Roo Code | `servername__toolname` | `myserver__read` |

### Hook Event Translation

OpenCode, Cline, Roo Code, and Zed don't have documented hook systems comparable to Claude/Gemini/Copilot/Kiro. When converting hooks TO these providers, the converter emits a data loss warning: "Target provider does not support hooks."

No new hook event mappings needed — just warning emission for unsupported targets.

### MCP Merge Strategies

**Current state:** `installer/mcp.go` has `mcpConfigPath()` mapping provider slugs to config paths. Only Claude Code and Gemini CLI implemented.

**New provider config paths:**

| Provider | Config Path | JSON Key | Format | Scope |
|----------|-----------|----------|--------|-------|
| Claude Code | `~/.claude.json` | `mcpServers` | JSON | User (existing) |
| Gemini CLI | `~/.gemini/settings.json` | `mcpServers` | JSON | User (existing) |
| Copilot CLI | `.copilot/mcp.json` | `mcpServers` | JSON | Project |
| Kiro | `.kiro/settings/mcp.json` | `mcpServers` | JSON | Project |
| OpenCode | `opencode.json` | `mcpServers` | JSONC | Project |
| Zed | `~/.config/zed/settings.json` | `context_servers` | JSON | User |
| Cline | `.vscode/mcp.json` | `mcpServers` | JSON | Project |
| Roo Code | `.roo/mcp.json` | `mcpServers` | JSON | Project |
| Cursor | — | — | — | Not applicable |
| Windsurf | — | — | — | Not applicable |
| Codex | — | — | — | Not applicable |

**Implementation changes:**
1. Extend `mcpConfigPath()` signature to accept `projectDir string` alongside `homeDir` (project-scoped providers need project root)
2. Add switch cases for each new provider
3. For Zed, add `mcpConfigKey()` helper returning `context_servers` instead of `mcpServers`
4. For OpenCode, JSONC read → JSON write (strip comments on merge using existing `ParseJSONC()`)

### Codex Multi-Agent TOML

**Format (from ai-config reference):**

```toml
[features]
multi_agent = true

[agents.reviewer]
model = "o4-mini"
prompt = "You are a code reviewer..."
tools = ["shell", "apply_patch"]

[agents.planner]
model = "o3"
prompt = "You are a planning agent..."
tools = ["shell", "view"]
```

**Canonicalization (TOML → canonical):**
- Parse TOML with `pelletier/go-toml` (v2)
- Map `[agents.<name>]` sections to `AgentMeta` structs
- `model` → `model`, `prompt` → markdown body, `tools` → reverse-translated to canonical names
- `multi_agent = true` feature flag preserved as conversion note

**Rendering (canonical → TOML):**
- Convert `AgentMeta` to TOML agent section
- Translate tool names to Codex equivalents (shared with Copilot CLI lineage)
- Generate `[features] multi_agent = true` block
- Emit warnings for unsupported fields: maxTurns, permissionMode, skills, mcpServers, memory, background, isolation

**File structure:**
- New: `cli/internal/converter/codex_agents.go`
- New dependency: `github.com/pelletier/go-toml/v2` in `go.mod`
- Update: `cli/internal/provider/codex.go` — add `Agents` to supported types
- Update: `cli/internal/converter/agents.go` — dispatch to Codex renderer

**Multi-agent handling:** One TOML file can contain multiple agents. Canonicalization produces one canonical agent per `[agents.*]` section. Rendering collects all agents destined for Codex into a single TOML file.

---

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Tool name mapping approach | Translate to provider-internal names | Content should reference what users see in the target tool's UI |
| MCP merge abstraction | Extend `mcpConfigPath()` | Minimal change, consistent with existing pattern, avoids interface changes |
| Codex TOML direction | Full bidirectional | Matches the "bidirectional conversion" promise. Export-only would be inconsistent. |
| Hookless providers | Warn on conversion, don't fake it | Honest data loss reporting over simulated features (per syllago philosophy) |
| TOML library | `pelletier/go-toml/v2` | Well-maintained, v2 API, clean struct marshaling |
| Roo Code tool names | PascalCase class names | That's the actual tool name in Roo Code's tool system (ReadFileTool, etc.) |

---

## Data Flow

### Tool Name Translation (existing flow, expanded)

```
Content with tool references
  → Canonicalize: provider-specific names → canonical names (ReverseTranslateTool)
  → Store as canonical
  → Render: canonical names → target provider names (TranslateTool)
  → Emit warnings for unmappable tools
```

### MCP Install Flow (existing flow, expanded)

```
Canonical mcp.json
  → Read target provider's existing config (mcpConfigPath)
  → Parse (JSON or JSONC depending on provider)
  → Merge at correct JSON key (mcpServers or context_servers)
  → Add _syllago marker for tracking
  → Write with .bak backup
```

### Codex Agent Flow (new)

```
Codex config.toml
  → Parse TOML, extract [agents.*] sections
  → For each agent: map fields to AgentMeta, reverse-translate tools
  → Store as canonical agent.md files

Canonical agent.md
  → Parse AgentMeta from frontmatter
  → Translate tools to Codex names
  → Generate TOML [agents.<name>] section
  → Wrap in [features] multi_agent = true
  → Emit warnings for dropped fields
```

---

## Error Handling

All error handling uses the existing `Result.Warnings` pattern:

| Scenario | Warning Message |
|----------|----------------|
| Tool name has no mapping | `"Tool 'WebSearch' has no equivalent in <provider>; kept as-is"` |
| Hook → hookless provider | `"Target provider '<provider>' does not support hooks; hook content was not converted"` |
| Codex agent drops fields | `"Fields dropped for Codex: maxTurns, permissionMode, skills (not supported in TOML agent format)"` |
| MCP HTTP → stdio-only provider | `"MCP server '<name>' uses HTTP transport; <provider> only supports stdio (skipped)"` |
| Roo Code lossy tool group | `"Tool 'Grep' mapped to Roo Code's 'read' group (broader than original)"` |

---

## Success Criteria

1. Every cell in the tool name mapping table has a forward and reverse translation test
2. `TranslateMCPToolName()` handles all 11 provider formats
3. MCP install works for all providers with MCP support (8 of 11)
4. Codex agent TOML roundtrips: TOML → canonical → TOML preserves all mappable fields
5. Converting hooks to hookless providers emits clear warnings (not silent drops)
6. All existing tests still pass (no regressions)
7. `make test` passes with new test files included

---

## Open Questions

None — all decisions resolved during brainstorm.

---

## Files Modified

| File | Change |
|------|--------|
| `cli/internal/converter/toolmap.go` | Add tool name + MCP name + hook event mappings for 4 providers |
| `cli/internal/converter/toolmap_test.go` | Add forward/reverse tests for all new mappings |
| `cli/internal/converter/codex_agents.go` | New — Codex TOML agent canonicalize/render |
| `cli/internal/converter/codex_agents_test.go` | New — TOML roundtrip tests |
| `cli/internal/converter/agents.go` | Dispatch to Codex renderer |
| `cli/internal/converter/mcp.go` | Add MCP tool name formats for new providers |
| `cli/internal/converter/mcp_test.go` | Add MCP tests for new providers |
| `cli/internal/converter/hooks.go` | Add warnings for hookless providers |
| `cli/internal/installer/mcp.go` | Extend `mcpConfigPath()` for all providers |
| `cli/internal/installer/installer.go` | Pass projectDir to MCP installer |
| `cli/internal/provider/codex.go` | Add Agents to supported types |
| `go.mod` / `go.sum` | Add `pelletier/go-toml/v2` |

---

## Next Steps

Ready for implementation planning with `Plan` skill.
