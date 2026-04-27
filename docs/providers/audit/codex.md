# Codex Provider Audit

Audit date: 2026-03-21
Research source: `docs/providers/codex/` (tools.md, content-types.md, hooks.md, skills-agents.md)
Codebase checked: `cli/internal/provider/codex.go`, `cli/internal/converter/`

---

## Inaccuracies

### 1. Agent TOML struct missing `developer_instructions` field

**File:** `cli/internal/converter/codex_agents.go:132-138`

The `codexSingleAgentBody` struct uses an `Instructions.Content` field modeled as a nested `[agent.instructions]` TOML table. Research shows the actual Codex agent format uses a top-level `developer_instructions` string field (not a nested table):

```toml
# Research says (from official docs):
developer_instructions = """
You explore codebases to gather evidence.
"""

# Syllago currently models:
[agent.instructions]
content = "You explore codebases..."
```

The `[agent.instructions]` sub-table format is not documented in official Codex sources. The converter may produce TOML that Codex doesn't recognize. The `description` field IS correct (it exists in the real format).

**Impact:** Agents rendered for Codex may not load their instructions.

### 2. Agent TOML struct missing key fields

**File:** `cli/internal/converter/codex_agents.go:132-138`

The `codexSingleAgentBody` struct only has: `Name`, `Description`, `Model`, `Tools`, `Instructions`. Research documents these additional fields that Codex actually supports:

| Field | Type | Purpose |
|-------|------|---------|
| `model_reasoning_effort` | string | `"low"` / `"medium"` / `"high"` |
| `sandbox_mode` | string | `"read-only"` / `"workspace-write"` / `"danger-full-access"` |
| `nickname_candidates` | string[] | Display names for spawned instances |
| `mcp_servers` | table | Per-agent MCP configuration |
| `skills.config` | array | Per-agent skill overrides |

The render function warns that `mcpServers` and `skills` are "not supported by Codex" (lines 158, 161) -- but research shows both ARE supported via `[mcp_servers]` and `[[skills.config]]` sub-tables in agent TOML.

**Impact:** Lossy conversion drops fields Codex actually supports.

### 3. Tool name translation uses wrong provider slug

**File:** `cli/internal/converter/codex_agents.go:59, 93, 179`

Tool translation for Codex agents uses `"copilot-cli"` as the slug:
```go
canonical[i] = ReverseTranslateTool(t, "copilot-cli")  // line 59
codexTools = TranslateTools(meta.Tools, "copilot-cli")  // line 179
```

This piggybacks on Copilot CLI's tool names (e.g., `view`, `shell`, `apply_patch`). But Codex's actual tool names from `spec.rs` are different:

| Canonical | Copilot CLI (current) | Codex actual |
|-----------|-----------------------|-------------|
| Read | `view` | `read_file` |
| Write/Edit | `apply_patch` | `apply_patch` (correct) |
| Bash | `shell` | `shell` / `shell_command` / `exec_command` |
| Glob | `glob` | `list_dir` |
| Grep | `rg` | `grep_files` |
| WebSearch | (none) | `web_search` |
| Task | `task` | `spawn_agent` |

The `ToolNames` map in `toolmap.go` has no `"codex"` entries at all. The workaround of reusing `"copilot-cli"` produces wrong names for Read (`view` vs `read_file`), Glob (`glob` vs `list_dir`), Grep (`rg` vs `grep_files`), and Task (`task` vs `spawn_agent`).

**Impact:** Converted agents reference tool names Codex won't recognize.

### 4. Hooks marked as unsupported -- but Codex has hooks

**File:** `cli/internal/converter/hooks.go:211`

Codex is listed in `hooklessProviders`, which causes hook conversion to emit a warning and skip. But research shows Codex has an experimental hooks system (`hooks.json`) with 3 events: `SessionStart`, `Stop`, `UserPromptSubmit`.

The hook events overlap with Claude Code's canonical events. At minimum, `SessionStart`, `Stop`, and `UserPromptSubmit` could be converted.

**Impact:** Valid hook content is silently dropped when targeting Codex.

### 5. HookEvents map has no Codex entries

**File:** `cli/internal/converter/toolmap.go:76-87`

The `HookEvents` map has entries for `gemini-cli`, `copilot-cli`, and `kiro` but none for `codex`. Even if hooks.go were fixed to attempt conversion, the event translation would fall through to identity mapping. Codex event names happen to match the canonical names (`SessionStart`, `Stop`, `UserPromptSubmit`) so identity mapping would work for those three, but this is accidental rather than intentional.

---

## Gaps

### G1. No tool name mappings for Codex

`toolmap.go` `ToolNames` has zero `"codex"` entries. Research documents 32 tools. The core 7 (Read, Write, Edit, Bash, Glob, Grep, WebSearch) plus Task need mappings at minimum.

### G2. Skills not modeled as a supported content type

`codex.go` `SupportsType` returns true only for Rules, Commands, and Agents. Research shows Codex has a full skills system at `.agents/skills/` with `SKILL.md` + directory structure. Skills are a first-class content type in Codex -- arguably more developed than in most other providers.

### G3. MCP not modeled as a supported content type

Codex supports MCP server configuration in `config.toml` under `[mcp_servers.<id>]` tables (TOML, not JSON). The provider definition doesn't support MCP (`SupportsType` returns false). This is a significant gap since MCP is one of syllago's core content types.

### G4. Hooks not modeled as a supported content type

Related to inaccuracy #4. Codex has `hooks.json` (JSON format, same as Claude Code's hooks structure in `settings.json`). The provider could support hooks with a TOML-merge or JSON-merge install strategy for `hooks.json`.

### G5. No `DiscoveryPaths` for skills or hooks

`DiscoveryPaths` only covers Rules (`AGENTS.md`), Commands (`.codex/commands`), and Agents (`.codex/agents`). Missing:
- Skills: `.agents/skills/` (note: NOT `.codex/skills/` -- Codex uses the `.agents/` convention)
- Hooks: `.codex/hooks.json`

### G6. Profiles and permissions profiles not modeled

Research documents profiles (`[profiles.<name>]`) and permission profiles (`[permissions.<name>]`) in `config.toml`. These are configuration-level constructs that could be shareable content types, though they're lower priority than skills/MCP/hooks.

### G7. Multi-agent config format incomplete

The `codexConfig` struct (line 19) models `[features]` and `[agents.<name>]` with only `model`, `prompt`, and `tools`. Codex's actual multi-agent config in `config.toml` uses `[agents]` for global settings (`max_threads`, `max_depth`, `job_max_runtime_seconds`) and `[agents.<name>]` with `description`, `config_file`, and `nickname_candidates`.

### G8. MCP tool name translation doesn't handle Codex

`TranslateMCPToolName` in `toolmap.go` has no case for `"codex"`. Codex's MCP tool naming convention needs research -- it likely uses a format tied to `mcp_servers.<id>` config names.

---

## Opportunities

### O1. Codex tool mappings (high value, low effort)

Add `"codex"` entries to `ToolNames` for the 8 core tools. This is a one-line-per-tool change that fixes tool references in agent conversion immediately.

```go
"Read": {
    "codex": "read_file",
    // ...
},
```

### O2. Fix agent TOML schema (high value, medium effort)

Update `codexSingleAgentBody` to match the real schema: `developer_instructions` as a top-level string, plus `sandbox_mode`, `model_reasoning_effort`, and `nickname_candidates`. Remove the `Instructions` nested struct. This fixes the core agent conversion path.

### O3. Enable skills support (high value, medium effort)

Codex skills use `SKILL.md` with YAML frontmatter -- structurally similar to Claude Code's skills. Add `catalog.Skills` to `SupportsType` and add `.agents/skills/` to `DiscoveryPaths`. The converter can likely reuse existing skill conversion logic with minor adaptations for the `agents/openai.yaml` metadata file.

### O4. Enable MCP support (high value, medium effort)

Codex MCP uses TOML in `config.toml` rather than JSON in `settings.json`. This needs a TOML-merge install strategy (new territory -- current merge installs are JSON only). The converter would translate between Claude Code's JSON MCP format and Codex's TOML `[mcp_servers.<id>]` tables.

### O5. Enable hooks support (medium value, medium effort)

Remove Codex from `hooklessProviders`. Add Codex entries to `HookEvents` (identity mappings for the 3 supported events). The hooks converter can then produce `hooks.json` output. Note: only 3 of Claude Code's events map -- the rest would generate warnings.

### O6. Codex as MCP tool name source/target

Add `"codex"` case to `parseMCPToolName` and `TranslateMCPToolName`. Needs research on Codex's actual MCP tool naming format first.
