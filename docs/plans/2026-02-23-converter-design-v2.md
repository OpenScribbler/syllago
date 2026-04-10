# Converter Design v2: Behavioral Embedding

**Goal:** Convert content between AI coding tool providers without losing intent. When a structural field has no equivalent in the target provider, embed its behavioral intent as actionable prose that the target LLM can follow.

**Decision Date:** 2026-02-23

---

## Problem Statement

Syllago's rules converter (v1) drops content that can't be structurally mapped — conditional rules return `nil Content` and a warning. This is safe but leaves value on the table. Most "unmappable" fields describe behaviors an LLM can follow if told to. A command restricted to specific tools, an agent running in isolation, a rule scoped to certain files — these are all things the target LLM can respect through prose instructions even when the target format can't express them structurally.

## Proposed Solution: Behavioral Embedding

### Core Philosophy

1. **Never silently drop intent.** If a field can't be expressed structurally, express it as prose.
2. **Prose should be actionable, not historical.** Write "Use only read_file and grep_search" not "Original config included allowed-tools: Read, Grep."
3. **Translate semantics, not just syntax.** Tool names, argument placeholders, and behavioral concepts get mapped to the target provider's ecosystem.
4. **Syllago is the hub.** Exported files are one-way renders. Round-trips go through the canonicalizer, not marker recovery.

### Marker Format

A single provenance tag at the top of markdown content:

```markdown
<!-- syllago:converted from="claude-code" -->
```

This tells both humans and tools that syllago generated this file and which provider it came from. No ID registry needed — the syllago repo's `.source/` directory handles lossless round-trips.

### Conversion Notes Block

When fields need behavioral embedding, they go in a clearly-demarcated block at the **bottom** of the body:

```markdown
[Original body/instructions here...]

---
<!-- syllago:converted from="claude-code" -->
**Tool restriction:** Use only read_file and grep_search tools.
Run in an isolated context. Do not modify the main conversation.
Operate in read-only exploration mode.
Limit to 30 turns.
```

### Prose Style Rules

- **Actionable:** "Use a 5-second timeout" not "Original config included timeout: 5000"
- **Token-efficient:** One line per field, no boilerplate
- **Target-native:** Use the target provider's tool names, not the source's
- **No provenance in prose:** The `<!-- syllago:converted from="..." -->` marker handles provenance — prose stays clean

### Re-import Behavior

When syllago re-imports a file it previously exported:
1. The canonicalizer strips the conversion notes block (it was generated, not authored)
2. The `<!-- syllago:converted -->` marker is recognized and stripped
3. The remaining body is canonicalized normally
4. Structural fields are recovered from the `.source/` directory if available, or from the canonical format if not

---

## Tool Name Translation

The converter maintains a cross-provider tool name mapping:

| Claude Code | Gemini CLI | Copilot CLI |
|---|---|---|
| `Read` | `read_file` | `view` |
| `Write` | `write_file` | `apply_patch` |
| `Edit` | `replace` | `apply_patch` |
| `Bash` | `run_shell_command` | `shell` / `bash` |
| `Glob` | `list_directory` | `glob` |
| `Grep` | `grep_search` | `rg` |
| `WebSearch` | `google_search` | — |
| `WebFetch` | — | — |
| `Task` | — | `task` |

This table is used when embedding tool-related fields (like `allowed-tools`) and when translating hook matchers.

Unknown tools pass through untranslated with a warning.

---

## Per Content Type Strategy

### Rules

**Format:** Markdown with optional YAML frontmatter → target's rule format

**Updated behavior (v2):** No more dropping non-alwaysApply rules.

| Source activation | Export behavior |
|---|---|
| `alwaysApply: true` | Exported as-is (body only, no frontmatter for single-file providers) |
| `alwaysApply: false` + `globs` | Include with scope prose: "Apply only when working with files matching: ..." |
| `alwaysApply: false` + `description` | Include with scope prose: "Apply when: [description]" |
| `alwaysApply: false` bare | Include with scope prose: "Apply only when explicitly asked" |

### Commands

**Format:** YAML frontmatter + MD (Claude/Codex) ↔ TOML (Gemini)

**Structural mappings:**
- `description` ↔ `description`
- Body → `prompt` (TOML multiline string)
- `$ARGUMENTS` ↔ `{{args}}`

**Behavioral embeddings (Claude → Gemini):**
- `allowed-tools` → "Restrict to tools: [translated names]"
- `context: fork` → "Run in an isolated context. Do not modify the main conversation."
- `agent: Explore` → "Use an exploration-focused approach."
- `disable-model-invocation: true` → "Only invoke when the user explicitly requests it."
- `model` → "Designed for model: [model name]"
- `argument-hint` → Can be embedded as usage documentation
- `user-invocable` → "Intended to appear in the command menu."

**Gemini-specific syntax preservation:** `!{shell}` and `@{file}` syntax in Gemini commands pass through as literal text when exported to other providers.

**TOML output:** Use a Go TOML library (pelletier/go-toml or BurntSushi/toml) for correct serialization.

### Skills

**Format:** YAML frontmatter + MD (both providers, different field sets)

**Structural mappings:**
- `name` ↔ `name`
- `description` ↔ `description`
- Body (markdown instructions) ↔ Body

**Behavioral embeddings (Claude → Gemini):**
- `allowed-tools` / `disallowedTools` → tool restriction prose with translated names
- `model`, `context`, `agent`, `disable-model-invocation`, `user-invocable`, `argument-hint` → conversion notes at bottom

Gemini → Claude: `name` and `description` map directly; body is identical. No fields to embed (Gemini is a subset).

### Agents

**Format:** YAML frontmatter + MD (all three providers, different field sets)

**Structural mappings:**
- `name` ↔ `name`
- `description` ↔ `description`
- `tools` ↔ `tools` (with tool name translation)
- `model` ↔ `model`
- `maxTurns` (Claude) ↔ `max_turns` (Gemini)

**Behavioral embeddings (Claude → Gemini):**
- `permissionMode: plan` → "Operate in read-only exploration mode."
- `permissionMode: acceptEdits` → "Auto-approve file edits."
- `skills` → "Preload these skills: [list]"
- `mcpServers` → Informational note about expected MCP servers
- `memory` → "Use persistent memory scope: [scope]"
- `background: true` → "Run as a background task."
- `isolation: worktree` → "Work in a separate git worktree."
- `disallowedTools` → "Do not use these tools: [translated names]"

**Behavioral embeddings (Gemini → Claude):**
- `temperature` → "Use temperature: [value] for response variability."
- `timeout_mins` → "Limit execution to [N] minutes."
- `kind: remote` → Informational note about remote agent type

### Hooks

**Format:** JSON (all three providers, different event names and fields)

**Event name mapping:**

| Canonical (Claude Code) | Gemini CLI | Copilot CLI |
|---|---|---|
| `PreToolUse` | `BeforeTool` | `preToolUse` |
| `PostToolUse` | `AfterTool` | `postToolUse` |
| `UserPromptSubmit` | `BeforeAgent` | `userPromptSubmitted` |
| `Stop` | `AfterAgent` | — (drop + warn) |
| `SessionStart` | `SessionStart` | `sessionStart` |
| `SessionEnd` | `SessionEnd` | `sessionEnd` |
| `PreCompact` | `PreCompress` | — (drop + warn) |
| `Notification` | `Notification` | — (drop + warn) |

Provider-exclusive events (Claude's `SubagentStart`, Gemini's `BeforeModel`, Copilot's `errorOccurred`, etc.) → drop + warn when targeting a provider that doesn't support them.

**Matcher translation:** Hook matchers reference tool names. When converting between providers, matchers are translated using the tool name mapping table.

**Field mapping:**

| Canonical (Claude) | Gemini CLI | Copilot CLI |
|---|---|---|
| `command` | `command` | `bash` / `powershell` |
| `timeout` (ms) | `timeout` (ms) | `timeoutSec` (÷1000) |
| `matcher` | `matcher` | — (no matcher support) |
| `statusMessage` | — | `comment` |
| `async` | — | — |
| `type: "command"` | `type: "command"` | `type: "command"` |

**LLM-evaluated hooks (`type: "prompt"` / `type: "agent"`):**

User choice via `--llm-hooks` flag (or config option):

- `--llm-hooks=generate` — Generate a wrapper shell script that calls the target provider's CLI in non-interactive mode with the original prompt. No API key needed — uses the locally installed CLI.
- `--llm-hooks=skip` (default) — Drop the hook with a detailed warning including the original prompt and expected I/O format.

Generated wrapper script pattern:
```bash
#!/bin/bash
# syllago-generated: LLM-evaluated hook (from Claude Code type: prompt)
INPUT=$(cat)
TOOL_CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')
RESULT=$(echo "Evaluate: $TOOL_CMD" | gemini --output-format json)
echo "$RESULT" | jq -r '.response'
```

### MCP Servers

**Format:** JSON (all providers, similar schemas)

**Strategy:** Structural mapping only. No behavioral embedding (machine-consumed config).

**Structural mappings:**
- `command`, `args`, `env`, `cwd` — universal, map directly
- `url` / `httpUrl` — map with transport type inference
- `headers` — map directly where supported
- `timeout` — map with unit conversion if needed

**Drop + warn:**
- `trust`, `includeTools`, `excludeTools` (Gemini-specific)
- `oauth`, `authProviderType` (provider-specific auth patterns)
- `disabled`, `autoApprove` (runtime state)

---

## New Provider: Copilot CLI

GitHub Copilot CLI supports Rules, Commands (via skills/plugins), Agents, Hooks, and MCP. It reads `.claude/agents/` as a fallback location (industry convergence toward Claude Code format).

Needs:
- Provider definition file: `cli/internal/provider/copilot.go`
- Format reference doc: `docs/provider-formats/copilot-cli.md`
- Integration into all new converters

---

## Key Decisions

| Decision | Choice | Reasoning |
|---|---|---|
| Conversion philosophy | Behavioral embedding | Preserves intent, not just structure. Biggest differentiator for syllago. |
| Marker format | `<!-- syllago:converted from="provider" -->` | Simple provenance tag. No ID registry needed. |
| Registry for round-trips | Syllago repo only (`.source/`) | Simplest model. Syllago is always the hub. |
| Conversion notes placement | Bottom of body | Main instructions get full LLM attention first. |
| Prose style | Actionable instructions | Token-efficient, target-native, no origin stories. |
| Tool names in prose | Translated to target provider | "read_file" not "Read" when targeting Gemini. |
| MCP conversion | Structural only, drop + warn | Machine-consumed JSON — no body for embedding. |
| LLM-evaluated hooks | User choice: generate wrapper or skip | Wrapper calls local CLI, no API key needed. |
| Non-alwaysApply rules | Embed scope as prose | No more silent drops. Consistent with philosophy. |
| TOML output for commands | Go TOML library | Correct serialization, no edge case surprises. |

---

## Resolved Questions

### 1. Copilot CLI Agent Format (Resolved 2026-02-23)

Verified from [official docs](https://docs.github.com/en/copilot/reference/custom-agents-configuration):

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | No | Display name |
| `description` | string | Yes | Purpose and capabilities |
| `tools` | list/string | No | Tool allowlist (or `["*"]` for all) |
| `disable-model-invocation` | bool | No | Prevent auto-delegation |
| `target` | string | No | `vscode` or `github-copilot` (runtime-only, safe to drop) |
| `mcp-servers` | object | No | Inline MCP server configs |
| `metadata` | object | No | Key-value annotations (safe to drop) |

File locations: `~/.copilot/agents/`, `.github/agents/` (project), and `.claude/agents/` (compatibility fallback).

### 2. MCP Tool Name Translation (Resolved 2026-02-23)

MCP tools use different naming conventions per provider:

| Provider | Format | Example |
|---|---|---|
| Claude Code | `mcp__servername__toolname` | `mcp__github__search_repositories` |
| Gemini CLI | `servername__toolname` (prefixed) or `toolname` (unprefixed if no conflict) | `github__search_repositories` |
| Copilot CLI | `servername/toolname` | `github/search_repositories` |

**Decision:** Always use the prefixed form for safety. Translation:
- Strip `mcp__` prefix from Claude format
- Replace `__` with `/` for Copilot format
- Keep `__` for Gemini format

### 3. Gemini `!{shell}` and `@{file}` Syntax (Resolved 2026-02-23)

**Decision:** Preserve as literal text in canonical format.

- When imported, `!{git diff --staged}` and `@{config.json}` stay as-is in the canonical body
- When rendering to non-Gemini targets, append an informational note: "This command contains Gemini CLI template directives that are not natively supported by this provider."
- `.source/` handles lossless Gemini round-trips

Rationale: These are template-expansion directives (run before the LLM sees the prompt), not LLM instructions. Converting to prose equivalents changes the execution semantics — template expansion happens at different timing than LLM tool use.

### 4. LLM Hook Wrapper Script (Tested 2026-02-23)

**Verified working** with Gemini CLI v0.29.5 headless mode:

```bash
gemini -p "prompt text" --output-format json 2>/dev/null | jq -r '.response'
```

Findings:
- `gemini -p` runs non-interactive, returns JSON with `.response` field containing LLM text
- No `--system-prompt` flag — all context must be in the prompt itself
- Latency: 2-20 seconds (rate limit dependent)
- Gemini injects its own routing system prompt, but user prompt still gets through
- Config warnings (invalid keys) appear on stderr but don't block execution

Wrapper script pattern (confirmed working):
```bash
#!/bin/bash
INPUT=$(cat)
TOOL_CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')
RESPONSE=$(gemini -p "Is this command safe? Respond ONLY JSON: {\"decision\":\"allow\"} or {\"decision\":\"deny\",\"reason\":\"why\"}. Command: $TOOL_CMD" --output-format json 2>/dev/null)
echo "$RESPONSE" | jq -r '.response'
```

---

## Next Steps

Ready for implementation planning with the `Plan` skill.
