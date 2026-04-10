# Tool Coverage Expansion — Design Document

*Design date: 2026-02-26*
*Status: Brainstorm complete*
*Source: docs/plans/2026-02-25-v1-release-strategy.md, Section 3*

---

## Goal

Add 5 new providers to syllago: **Cline**, **Kiro**, **Roo Code**, **Zed**, and **OpenCode**. Target: 11 providers at launch. Each provider supports its actual capabilities — no artificial reduction to a common denominator.

---

## Provider Support Matrix (All 11)

| Type | Claude Code | Gemini CLI | Cursor | Windsurf | Codex | Copilot CLI | **Cline** | **Kiro** | **Roo Code** | **Zed** | **OpenCode** |
|------|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| Rules | Y | Y | Y | Y | Y | Y | **Y** | **Y** | **Y** | **Y** | **Y** |
| Skills | Y | Y | - | - | - | - | - | **Y** | - | - | **Y** |
| Agents | Y | Y | - | - | - | Y | - | **Y** | **Y** | - | **Y** |
| Commands | Y | Y | - | - | Y | Y | - | - | - | - | **Y** |
| MCP | Y | Y | - | - | - | Y | **Y** | **Y** | **Y** | **Y** | **Y** |
| Hooks | Y | Y | - | - | - | Y | - | **Y** | - | - | - |

---

## Complexity Tiers

| Tier | Provider | Work Summary |
|------|----------|-------------|
| **Simple** | Zed | Rules (`.rules` file) + MCP (`context_servers` in settings.json) |
| **Simple** | Cline | Rules (`.clinerules/` directory) + MCP (global JSON) |
| **Medium** | Roo Code | Rules (`.roo/rules/` + mode-specific) + MCP + Agents (custom modes as YAML) |
| **Medium** | OpenCode | Rules + Commands + Agents + Skills + MCP. JSONC format. Different MCP field names. |
| **Complex** | Kiro | Rules + Agents (JSON + file:// prompts) + Hooks (in agent files) + MCP + Skills (steering). Most converter work. |

---

## Design Decisions

### D1: JSONC Support

**Decision:** Add first-class `FormatJSONC` format constant.

**Why:** OpenCode uses JSONC (JSON with comments) for its config files. Comments are intentional user annotations. Stripping them on read loses context; round-trips (read → modify → write) would destroy user comments. JSONC support preserves fidelity.

**Implementation:**
- Add `FormatJSONC Format = "jsonc"` to `provider.go`
- Add JSONC parser (strip `//` and `/* */` comments before `json.Unmarshal`)
- When writing to OpenCode targets, output standard JSON (comments can't be programmatically preserved in modified content — but we don't destroy existing comments in files we don't modify)

### D2: OpenCode MCP Canonical Extension

**Decision:** Extend `mcpServerConfig` with OpenCode-specific fields, same pattern as Gemini's `httpUrl`/`trust`/`includeTools`.

**New fields on `mcpServerConfig`:**
```go
// OpenCode-specific
Environment map[string]string `json:"environment,omitempty"` // OpenCode uses "environment" not "env"
CommandArray []string          `json:"commandArray,omitempty"` // OpenCode command as array
Enabled     *bool             `json:"enabled,omitempty"`     // OpenCode uses "enabled" not "disabled"
Timeout     int               `json:"timeout,omitempty"`     // OpenCode timeout in ms
OAuth       json.RawMessage   `json:"oauth,omitempty"`       // OpenCode OAuth config
```

**Canonicalize:** Normalize `environment` → `env`, split `commandArray` → `command` + `args`, flip `enabled` → `disabled`.
**Render:** Reverse the normalization for OpenCode targets.

### D3: Kiro Hooks via Dedicated Agent File

**Decision:** When installing hooks to Kiro, create/update `.kiro/agents/syllago-hooks.json`.

**Why:** Kiro embeds hooks in agent JSON files. Creating a dedicated syllago agent file is non-invasive (doesn't modify user's existing agents), predictable (always the same file), and easy to uninstall (delete one file).

**Format:**
```json
{
  "name": "syllago-hooks",
  "description": "Hooks installed by syllago",
  "prompt": "",
  "hooks": {
    "preToolUse": [{ "command": "...", "matcher": "..." }]
  }
}
```

### D4: Roo Code Mode-Aware Install

**Decision:** Offer install destination choice: global (`.roo/rules/`) or mode-specific (`.roo/rules-{mode}/`).

**Why:** Roo Code's mode system is a core differentiator. Forcing everything to global ignores this. But auto-detecting the right mode is fragile. Let the user choose.

**Implementation:**
- Default install target: `.roo/rules/` (all modes)
- TUI presents mode selection when installing rules to Roo Code
- Available modes: `code`, `architect`, `ask`, `debug`, `orchestrator`, plus any custom modes detected in the project

### D5: Zed MCP Surgical Merge

**Decision:** Use gjson/sjson to merge into `context_servers` key within `~/.config/zed/settings.json`.

**Why:** Zed's MCP config uses `context_servers` (not `mcpServers`) and lives inside the global settings.json alongside other settings. Must merge surgically without clobbering other settings.

**Translation from canonical:**
```json
// Canonical (syllago)
{ "mcpServers": { "server": { "command": "npx", "args": [...] } } }

// Zed output
{ "context_servers": { "server": { "source": "custom", "command": "npx", "args": [...] } } }
```

Note: `"source": "custom"` is required for all user-defined MCP servers in Zed.

### D6: Kiro Agent Format (JSON + file:// prompt)

**Decision:** When converting canonical markdown agents to Kiro, generate both a JSON config and a separate markdown prompt file.

**Why:** This is Kiro's native pattern. Users expect to see their prompt in a readable `.md` file, not embedded as a JSON string.

**Output structure:**
```
.kiro/
  agents/
    my-agent.json         ← metadata from frontmatter
  prompts/
    my-agent.md           ← body content (system prompt)
```

**JSON config maps frontmatter fields:**
| Canonical (Claude Code) | Kiro | Notes |
|------------------------|------|-------|
| `name` | `name` | Direct |
| `description` | `description` | Direct |
| `tools` | `tools` | Translate tool names |
| `model` | `model` | Translate model IDs |
| `maxTurns` | — | Not supported, warn |
| `permissionMode` | — | Not supported, warn |
| body | `prompt: "file://./prompts/<name>.md"` | Extracted to file |

### D7: Detection Strategy

**Decision:** Project-level directory check + home config directory check. No VS Code extension probing.

| Provider | Project Detection | Home Detection |
|----------|------------------|----------------|
| Cline | `.clinerules/` exists | N/A (VS Code globalStorage — skip) |
| Kiro | `.kiro/` exists | `~/.kiro/` exists |
| Roo Code | `.roo/` exists | N/A (VS Code globalStorage — skip) |
| Zed | N/A (no project config dir) | `~/.config/zed/` exists |
| OpenCode | `opencode.json` or `.opencode/` exists | `~/.config/opencode/` exists |

### D8: Kiro Skills → Steering Files

**Decision:** Map Skills to `.kiro/steering/` markdown files. Do not attempt to generate full Powers (POWER.md + mcp.json + steering/) for v1.0.

**Why:** Powers are complex self-contained bundles with auto-activation logic. Synthesizing them from syllago Skills would be fragile. Steering files are the natural 1:1 mapping for instruction-based content.

### D9: Tool Name Translation Maps

**Decision:** Add comprehensive tool name mappings upfront for all providers that have programmable tool references. Cross-reference azat-io/ai-config adapters and each provider's official docs.

**Providers needing entries:**
- Kiro: `read`, `write`, `shell`, `@git`, `fs_write`, `@builtin`
- OpenCode: Needs research (likely similar to Codex/Claude Code)
- Cline, Roo Code, Zed: Don't have programmable tool references in content — no entries needed

### D10: Hook Event Translation

**Decision:** Add Kiro hook events to `HookEvents` map.

| Canonical (Claude Code) | Kiro |
|------------------------|------|
| `PreToolUse` | `preToolUse` |
| `PostToolUse` | `postToolUse` |
| `UserPromptSubmit` | `userPromptSubmit` |
| `Stop` | `stop` |
| `SessionStart` | `agentSpawn` |

---

## Implementation Order

Recommended execution order based on complexity and dependencies:

1. **Zed** (Simple) — New provider file + MCP converter for `context_servers`
2. **Cline** (Simple) — New provider file + MCP converter for global-only path
3. **Roo Code** (Medium) — New provider file + mode-aware rules + custom modes as agents
4. **OpenCode** (Medium) — JSONC support + new provider file + MCP converter extensions + commands/agents/skills
5. **Kiro** (Complex) — New provider file + JSON agents with file:// + hooks in agent files + steering + MCP + hook event translations

**Infrastructure first:**
- Add `FormatJSONC` constant
- Add JSONC strip-comments helper
- Extend `mcpServerConfig` with OpenCode fields
- Add tool name and hook event entries to `toolmap.go`

---

## Files to Create/Modify

### New Files (5 providers × 1 file each)
- `cli/internal/provider/cline.go`
- `cli/internal/provider/kiro.go`
- `cli/internal/provider/roocode.go`
- `cli/internal/provider/zed.go`
- `cli/internal/provider/opencode.go`

### Modified Files
- `cli/internal/provider/provider.go` — Add FormatJSONC, register 5 new providers in AllProviders
- `cli/internal/converter/toolmap.go` — Add Kiro + OpenCode tool/event translations
- `cli/internal/converter/mcp.go` — Add render functions for Cline, Kiro, Roo Code, Zed, OpenCode; extend mcpServerConfig
- `cli/internal/converter/rules.go` — Add render functions for new markdown-based providers (Cline directory, Roo Code mode-aware, Zed .rules, OpenCode AGENTS.md)
- `cli/internal/converter/agents.go` — Add Kiro JSON+file:// renderer, Roo Code YAML mode renderer, OpenCode agent renderer
- `cli/internal/converter/hooks.go` — Add Kiro hooks-in-agent-file renderer
- `cli/internal/converter/skills.go` — Add Kiro steering renderer, OpenCode skill renderer
- `cli/internal/converter/commands.go` — Add OpenCode command renderer
- `cli/internal/converter/jsonc.go` (NEW) — JSONC comment-stripping utility

### Test Files
- `cli/internal/provider/cline_test.go`
- `cli/internal/provider/kiro_test.go`
- `cli/internal/provider/roocode_test.go`
- `cli/internal/provider/zed_test.go`
- `cli/internal/provider/opencode_test.go`
- `cli/internal/converter/mcp_test.go` — Extend with new provider cases
- `cli/internal/converter/rules_test.go` — Extend with new provider cases
- `cli/internal/converter/agents_test.go` — Extend with Kiro/Roo Code/OpenCode cases
- `cli/internal/converter/jsonc_test.go` — JSONC parsing tests

---

## Research References

All provider format details documented in:
- `docs/provider-formats/cline.md`
- `docs/provider-formats/kiro.md`
- `docs/provider-formats/roo-code.md`
- `docs/provider-formats/zed.md`
- `docs/provider-formats/opencode.md`
- `docs/provider-architecture.md` — Existing provider system reference

External references:
- azat-io/ai-config adapters — tool name translation tables, MCP merge patterns
- Each provider's official documentation (links in format docs)
