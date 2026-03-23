# Agents: Cross-Provider Deep Dive

> Research compiled 2026-03-22. Covers agent systems across 13 AI coding tools.
> Output feeds into: converter/compat.go unified compat scorer (syllago-ohdb)

---

## Table of Contents

1. [Overview](#overview)
2. [Per-Provider Deep Dive](#per-provider-deep-dive)
   - [Claude Code](#claude-code)
   - [Cursor](#cursor)
   - [Gemini CLI](#gemini-cli)
   - [Copilot CLI (GitHub Copilot)](#copilot-cli)
   - [VS Code Copilot](#vs-code-copilot)
   - [Windsurf](#windsurf)
   - [Kiro](#kiro)
   - [Codex CLI](#codex-cli)
   - [Cline](#cline)
   - [OpenCode](#opencode)
   - [Roo Code](#roo-code)
   - [Zed](#zed)
   - [Amp](#amp)
3. [Cross-Platform Normalization Problem](#cross-platform-normalization-problem)
4. [Canonical Mapping](#canonical-mapping)
5. [Feature/Capability Matrix](#featurecapability-matrix)
6. [Compat Scoring Implications](#compat-scoring-implications)
7. [Converter Coverage Audit](#converter-coverage-audit)

---

## Overview

Agents (also called sub-agents, custom agents, or custom modes) are specialized AI assistants that handle specific tasks within AI coding tools. They typically have their own system prompt, tool access restrictions, and model configuration. Each agent runs in its own context window, separate from the main conversation.

The landscape splits into three architectural patterns:
1. **Markdown + YAML frontmatter** (Claude Code, Cursor, Gemini CLI, Copilot, Kiro, OpenCode) -- the dominant pattern
2. **TOML configuration** (Codex CLI) -- unique outlier
3. **Custom modes** (Roo Code, Cline) -- JSON/YAML mode definitions with tool groups instead of individual tools

### Summary Table

| Provider | Agents Supported | Format | Location | Invocation |
|----------|-----------------|--------|----------|------------|
| Claude Code | Yes | MD + YAML frontmatter | `.claude/agents/` | `@agent` mention, `--agent` flag |
| Cursor | Yes | MD + YAML frontmatter | `.cursor/agents/` | Agent mode selection, `/name` |
| Gemini CLI | Yes | MD + YAML frontmatter | `.gemini/agents/` | `@agent` mention |
| Copilot CLI | Yes | MD + YAML frontmatter | `.github/agents/` | `@agent` mention |
| VS Code Copilot | Yes | MD + YAML frontmatter | `.github/agents/` | `@agent` in chat |
| Windsurf | Partial (workflows) | MD + optional YAML | `.windsurf/workflows/` | `/workflow` slash command |
| Kiro | Yes | MD + YAML frontmatter (IDE) / JSON (CLI) | `.kiro/agents/` | Agent panel, auto-delegation |
| Codex CLI | Yes | TOML | `.codex/agents/` | `--agent` flag |
| Cline | Partial (Plan/Act modes) | N/A (no custom agent files) | N/A | Mode switching |
| OpenCode | Yes | MD + YAML frontmatter | `.opencode/agents/` | Tab cycling, `@agent` mention |
| Roo Code | Yes (custom modes) | YAML/JSON | `.roo/modes/` or `.roomodes` | Mode switching |
| Zed | No (external agents via ACP) | JSON settings | `settings.json` | Agent panel |
| Amp | Partial (subagents, skills) | MD (AGENTS.md / SKILL.md) | `.agents/skills/` | Auto-delegation |

---

## Per-Provider Deep Dive

### Claude Code

**Status:** Full agent support -- the most feature-rich agent system of any provider.

**Format:** Markdown with YAML frontmatter.

**Directory:** `.claude/agents/*.md` (project-level) or `~/.claude/agents/*.md` (user-level). Plugin agents in plugin `agents/` directories.

**Invocation:** `@<name>` mention in conversation, `--agent <name>` CLI flag, natural language delegation. Claude auto-delegates based on description.

**Priority order:** CLI flag (session) > project > user > plugin.

**Frontmatter Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier, lowercase letters and hyphens |
| `description` | string | Yes | When Claude should delegate to this agent |
| `tools` | string/string[] | No | Allowlist of tools. Inherits all if omitted. Supports `Agent(type1, type2)` syntax |
| `disallowedTools` | string[] | No | Denylist, removed from inherited or allowlisted tools |
| `model` | string | No | `sonnet`, `opus`, `haiku`, full model ID, or `inherit` (default) |
| `permissionMode` | string | No | `default`, `acceptEdits`, `dontAsk`, `bypassPermissions`, `plan` |
| `maxTurns` | int | No | Max agentic turns before stopping |
| `skills` | string[] | No | Skill content injected at startup (full content, not just reference) |
| `mcpServers` | list | No | Server name references or inline MCP server definitions |
| `hooks` | object | No | `PreToolUse`, `PostToolUse`, `Stop` hooks with matchers |
| `memory` | string | No | Persistent memory scope: `user`, `project`, or `local` |
| `background` | bool | No | Run concurrently, pre-approve permissions upfront |
| `isolation` | string | No | `worktree` -- temporary git worktree, auto-cleaned if no changes |
| `effort` | string | No | `low`, `medium`, `high`, `max` (Opus 4.6 only) |
| `color` | string | No | Background color for UI identification |

**Claude Code-Unique Features:**
- **Agent(type) tool syntax**: `tools: Agent(worker, researcher), Read, Bash` restricts which sub-agents can be spawned
- **Inline MCP definitions**: `mcpServers` supports both string references and full inline server configs
- **Permission modes**: Five distinct modes including `dontAsk` (auto-deny) and `bypassPermissions`
- **Memory scopes**: Three levels (user/project/local) with auto-curated MEMORY.md
- **Hook lifecycle**: Full hook system with matchers, command hooks, and exit code behaviors
- **Concurrency**: Up to 10 simultaneous subagents; Ctrl+B to background a running task
- **Resume**: Subagents can be resumed with full conversation history preserved
- **Auto-compaction**: Subagents auto-compact at ~95% capacity

**Notes:**
- Claude Code is the canonical superset for syllago's AgentMeta format.
- Subagents cannot spawn other subagents (max depth 1).
- Plugin agents do not support `hooks`, `mcpServers`, or `permissionMode` for security.
- `effort: max` is exclusive to Opus 4.6.

**Source:** [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

---

### Cursor

**Status:** Subagent support added in Cursor 2.4 (January 2026).

**Format:** Markdown with YAML frontmatter.

**Directory:** `.cursor/agents/*.md` (project-level).

**Invocation:** Auto-delegation, explicit `/name`, natural language mention.

**Frontmatter Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Display name (derived from filename if omitted) |
| `description` | string | No | Delegation hint -- agent reads this to decide when to use |
| `model` | string | No | `inherit` (default), `fast`, or specific model ID |
| `readonly` | bool | No | Restrict write permissions (no edits, no state-changing shell) |
| `is_background` | bool | No | Run in background without blocking parent |

**Field Mapping to Canonical:**
- `readonly: true` -> `permissionMode: "plan"`
- `is_background: true` -> `background: true`
- `model: "fast"` -> no direct canonical equivalent (provider-specific alias)

**Notes:**
- Cursor has a deliberately minimal agent schema -- 5 fields total.
- No support for: maxTurns, tools (allowlist), disallowedTools, skills, mcpServers, memory, isolation/worktree, effort, hooks, color.
- The `model` field accepts `inherit`, `fast`, or specific IDs -- the `fast` alias has no direct canonical equivalent.
- Cursor also has "Custom Modes" (beta, UI-based) separate from file-based agents -- these are tool/instruction combinations configured through settings, not files.

**Source:** [cursor.com/docs/context/subagents](https://cursor.com/docs/context/subagents)

---

### Gemini CLI

**Status:** Full agent support with remote agent capabilities.

**Format:** Markdown with YAML frontmatter.

**Directory:** `.gemini/agents/*.md` (project-level) or `~/.gemini/agents/*.md` (user-level).

**Invocation:** `@<agent-name>` mention in conversation.

**Frontmatter Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Slug identifier (lowercase, hyphens, underscores only) |
| `description` | string | Yes | Short description of what the agent does |
| `kind` | string | No | `local` (default) or `remote` |
| `tools` | string[] | No | Tool names with wildcard support (`*`, `mcp_*`, `mcp_server_*`) |
| `model` | string | No | Specific model ID (e.g., `gemini-3-flash-preview`). Inherits if omitted |
| `temperature` | float | No | 0.0-2.0 range, default 1 |
| `max_turns` | int | No | Default 30 |
| `timeout_mins` | int | No | Default 10 minutes |

**Remote agent additional fields:**
- `agent_card_url`: URL to the agent card for remote agents
- `auth`: Authentication block supporting `apiKey`, `http` auth types with `$ENV_VAR` or `!command` secret resolution

**Tool Name Mapping (Gemini -> Canonical):**
- `shell` -> `Bash`
- `read_file` -> `Read`
- `write_file` -> `Write`
- `edit_file` -> `Edit`
- `search_web` -> `WebSearch`
- `list_directory` -> `Glob`

**Gemini-Specific Features:**
- **Remote agents** (`kind: remote`): Run on Google's infrastructure, unique to Gemini
- **Temperature**: Direct control in frontmatter (no other provider exposes this)
- **Tool wildcards**: `*` (all), `mcp_*` (all MCP), `mcp_server_*` (specific server)
- **Timeout**: Hard execution time limit
- Uses **snake_case** for field names (`max_turns`, `timeout_mins`)

**Notes:**
- No support for: permissionMode, skills, memory, background, isolation, effort, hooks, color, disallowedTools.
- Remote agent auth is Gemini-unique -- no canonical equivalent.
- Multiple remote agents can be defined in a single .md file; mixed local/remote not supported.

**Source:** [geminicli.com/docs/core/subagents/](https://geminicli.com/docs/core/subagents/)

---

### Copilot CLI

**Status:** Agent support for GitHub Copilot (shared across CLI and VS Code).

**Format:** Markdown with YAML frontmatter. Files use `.agent.md` extension.

**Directory:** `.github/agents/` (project-level). Note: formerly `.github/copilot/agents/`.

**Invocation:** `@<agent-name>` mention in Copilot chat. Automatic delegation based on task context.

**Frontmatter Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Display name (optional) |
| `description` | string | Yes | Agent purpose and capabilities |
| `tools` | string/string[] | No | Comma-separated or YAML array. Supports `mcp-server/tool` namespacing. All tools if omitted |
| `model` | string | No | Model override. Inherits default if unset |
| `target` | string | No | Environment context: `vscode` or `github-copilot`. Both if omitted |
| `mcp-servers` | object | No | Inline MCP server definitions with type, command, args, env, tools |
| `disable-model-invocation` | bool | No | Prevent auto-delegation by Copilot coding agent. Default false |
| `user-invocable` | bool | No | Allow user to select this agent directly. Default true |
| `metadata` | object | No | Arbitrary name/value annotation pairs |

**Max prompt length:** 30,000 characters in the markdown body.

**Copilot-Specific Features:**
- **`.agent.md` extension**: Unique naming convention (not just `.md`)
- **`target` field**: Restricts agent to specific environment (VS Code only, GitHub only, or both)
- **`disable-model-invocation`**: Prevents automatic delegation -- agent-only user-invoked
- **`user-invocable`**: Can hide agents from user selection (used for background/system agents)
- **`mcp-servers` with secrets**: Supports `${{ secrets.NAME }}` syntax for environment variables
- **Handoffs**: VS Code supports agent-to-agent handoff workflows
- **`metadata`**: Arbitrary annotation (no Copilot runtime use, but useful for tooling)

**Notes:**
- Directory changed from `.github/copilot/agents/` to `.github/agents/` -- converter should handle both.
- `mcp-servers` uses hyphenated key with inline server definitions (not just name references).
- No support for: maxTurns, permissionMode, skills, memory, background, effort, hooks, color, disallowedTools, isolation.
- The retired `infer` field has been split into `disable-model-invocation` + `user-invocable`.

**Source:** [docs.github.com/en/copilot/reference/custom-agents-configuration](https://docs.github.com/en/copilot/reference/custom-agents-configuration)

---

### VS Code Copilot

**Status:** Shares the GitHub Copilot agent format. Same file schema as Copilot CLI.

**Format:** Markdown with YAML frontmatter (`.agent.md` extension).

**Directory:** `.github/agents/` (same as Copilot CLI).

**Invocation:** `@<agent-name>` in VS Code Copilot Chat panel.

**Additional VS Code-Specific Features:**
- **Chat participants**: Extension-based agents via the VS Code extension API (separate from file-based agents).
- **Handoffs**: Agent-to-agent transitions via single-click UI for guided workflows.
- **Requires VS 2026 v18.4+** for custom agents in Visual Studio (not VS Code).

**Notes:**
- For syllago's purposes, VS Code Copilot and Copilot CLI share the same file format and directory. They are treated as the same provider for agent conversion.
- The `target` field in frontmatter can restrict an agent to VS Code only (`target: vscode`).

**Source:** [code.visualstudio.com/docs/copilot/customization/custom-agents](https://code.visualstudio.com/docs/copilot/customization/custom-agents)

---

### Windsurf

**Status:** Partial agent support via Workflows and AGENTS.md. No dedicated sub-agent system comparable to other providers.

**Format:** Markdown with optional YAML frontmatter (for workflows). AGENTS.md uses plain markdown.

**Directory:** `.windsurf/workflows/` (workflow files). AGENTS.md files at any directory level.

**Invocation:** `/workflow-name` slash commands for workflows. AGENTS.md is auto-included.

**Workflow Frontmatter Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `auto_execution_mode` | string | No | `safe` (prompt before commands) or `turbo` (auto-execute) |

**AGENTS.md:**
- Plain markdown, no frontmatter, no special fields.
- Directory-scoped: root-level is always-on; subdirectory files auto-scope to `<directory>/**`.
- Provides instructions to Cascade, not agent definitions.

**Windsurf-Specific Features:**
- **Fast Context subagent**: Built-in subagent for fast file discovery (not user-configurable).
- **Workflows**: Closest analogue to agents -- task-specific instruction files with slash command invocation.
- **Cascade Flows**: Iterative AI interaction pattern (generate -> approve -> follow-up), not configurable agents.

**Notes:**
- Windsurf does NOT have a true custom agent system. Workflows are more like Claude Code slash commands than agents.
- No agent metadata fields (no tools, model, permissions, etc.).
- The `auto_execution_mode` in workflows is the only structured field -- maps loosely to `permissionMode`.
- Now owned by Cognition AI (Devin).
- **Not a conversion target for agents** -- content would need to be rendered as workflows or AGENTS.md instructions.

**Source:** [docs.windsurf.com/windsurf/cascade/agents-md](https://docs.windsurf.com/windsurf/cascade/agents-md)

---

### Kiro

**Status:** Full agent support across both IDE and CLI, with the most unique provider-specific fields.

**Format:** Markdown with YAML frontmatter (IDE) or JSON configuration (CLI).

**Directory:** `.kiro/agents/*.md` (project-level) or `~/.kiro/agents/*.md` (user-level).

**Invocation:** Agent panel in Kiro IDE. Auto-delegation based on description. Keyboard shortcuts.

**IDE Frontmatter Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Agent identifier |
| `description` | string | No | Agent description for delegation decisions |
| `model` | string | No | Model override (e.g., `claude-sonnet-4`) |
| `tools` | string[] | No | Tool access. Supports categories: `read`, `write`, `shell`, `web`, `spec`, `@builtin` |
| `allowedTools` | string[] | No | Auto-approve list. Supports `@server/tool` granular patterns |
| `toolAliases` | map[string]string | No | Remap tool names (resolve MCP naming collisions) |
| `toolsSettings` | map[string]any | No | Per-tool configuration parameters |
| `mcpServers` | string[]/object | No | MCP server references or inline definitions |
| `resources` | string[] | No | `file://`, `skill://`, knowledge base URIs loaded at startup |
| `includeMcpJson` | bool | No | Include all MCP tools from project config. Default false |
| `includePowers` | bool | No | Include MCP tools from Kiro Powers. Default false |
| `keyboardShortcut` | string | No | `[modifier+]key` (modifiers: ctrl, shift; keys: a-z, 0-9) |
| `welcomeMessage` | string | No | Custom greeting on agent activation |
| `hooks` | object | No | `agentSpawn`, `userPromptSubmit`, `preToolUse`, `postToolUse`, `stop` |

**CLI Configuration Fields (JSON):**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Agent name |
| `description` | string | Description |
| `prompt` | string | System prompt. Supports `file://` URIs |
| `mcpServers` | object | MCP server configurations |
| `tools` | string[] | Tool access with wildcard support |
| `toolAliases` | map | Tool name remapping |
| `allowedTools` | string[] | Auto-approve patterns |
| `toolsSettings` | map | Per-tool configuration |
| `resources` | string[] | Resource URIs |
| `includeMcpJson` | bool | Include MCP config |
| `model` | string | Model override |
| `keyboardShortcut` | string | Shortcut binding |
| `welcomeMessage` | string | Greeting message |
| `hooks` | object | Lifecycle hooks |

**Kiro-Unique Features:**
- **Tool aliases**: Remap tool names to resolve MCP server naming collisions -- unique to Kiro
- **Tool categories**: `read`, `write`, `shell`, `web`, `spec`, `@builtin` as shorthand groups
- **Resources with URI schemes**: `file://README.md`, `skill://.kiro/skills/**/SKILL.md`
- **includePowers**: Dynamically load Kiro Powers (MCP tools + framework expertise bundles)
- **Keyboard shortcuts**: Direct keyboard binding per agent
- **Welcome messages**: Custom greeting text
- **Progressive skill loading**: Skills loaded lazily -- only metadata at startup, full content on demand

**Notes:**
- Kiro has the most provider-specific fields of any tool.
- IDE uses MD+YAML frontmatter; CLI uses JSON -- two distinct formats for the same provider.
- `allowedTools` supports `@server/tool` granular patterns unique to Kiro.
- During canonicalization, `tools` + `allowedTools` merge, namespaced forms collapse to base tool.
- Hooks differ from Claude Code: Kiro has `agentSpawn` and `userPromptSubmit` triggers.
- Subagents run in parallel; main agent waits for all to complete. Hooks do NOT trigger in subagents.

**Source:** [kiro.dev/docs/chat/subagents/](https://kiro.dev/docs/chat/subagents/), [kiro.dev/docs/cli/custom-agents/configuration-reference/](https://kiro.dev/docs/cli/custom-agents/configuration-reference/)

---

### Codex CLI

**Status:** Full agent support via TOML configuration. GA as of March 2026.

**Format:** TOML (unique among all providers).

**Directory:** `.codex/agents/*.toml` (project-scoped) or `~/.codex/agents/*.toml` (personal). Multi-agent `AGENTS.toml` in project root.

**Invocation:** `codex --agent <name>` flag. Agent selection in interactive mode.

**Single-Agent TOML Schema (`.codex/agents/<name>.toml`):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent.name` | string | Yes | Agent identifier (source of truth, not filename) |
| `agent.description` | string | Yes | When Codex should use this agent |
| `agent.developer_instructions` | string | Yes | System prompt (body equivalent) |
| `agent.model` | string | No | Model override. Inherits from parent if omitted |
| `agent.model_reasoning_effort` | string | No | Reasoning effort level. Inherits if omitted |
| `agent.tools` | string[] | No | Allowed tools |
| `agent.sandbox_mode` | string | No | Sandbox configuration. Inherits if omitted |
| `agent.nickname_candidates` | string[] | No | Pool of display nicknames for spawned agents |
| `agent.mcp_servers` | table | No | MCP server configurations. Inherits if omitted |
| `agent.skills.config` | table | No | Skills configuration map. Inherits if omitted |

**Multi-Agent TOML Schema (`AGENTS.toml`):**

| Field | Type | Description |
|-------|------|-------------|
| `features.multi_agent` | bool | Enable multi-agent mode |
| `agents.<name>.model` | string | Model per agent |
| `agents.<name>.prompt` | string | System prompt |
| `agents.<name>.tools` | string[] | Allowed tools |

**Global Agent Settings (`config.toml`):**

| Field | Default | Description |
|-------|---------|-------------|
| `agents.max_threads` | 6 | Maximum concurrent agents |
| `agents.max_depth` | 1 | Max nesting depth |
| `agents.job_max_runtime_seconds` | 1800 | Per-worker timeout |

**Codex-Specific Features:**
- **TOML format**: Only provider using TOML
- **nickname_candidates**: Display name pool for UI variety -- Codex-unique
- **sandbox_mode**: Security sandbox config (different from `isolation` -- sandbox controls permissions, not git worktree)
- **max_threads/max_depth**: Global concurrency configuration
- **Two formats**: Single-agent (per-file) and multi-agent (AGENTS.toml)

**Known Issue:** In tool-backed Codex sessions, repo-local custom agents are visible on disk but the `spawn_agent` interface only accepts generic `agent_type`, not custom agent names.

**Notes:**
- `model_reasoning_effort` maps to canonical `effort`.
- `developer_instructions` is the body equivalent (single-agent); `prompt` is the body equivalent (multi-agent).
- No support for: maxTurns, permissionMode, memory, background, isolation, hooks, color, disallowedTools.
- Custom agent names matching built-in agents (`default`, `worker`, `explorer`) take precedence.

**Source:** [developers.openai.com/codex/subagents](https://developers.openai.com/codex/subagents)

---

### Cline

**Status:** No dedicated custom agent file format. Uses Plan/Act mode paradigm and MCP extensibility.

**Format:** N/A -- Cline does not support file-based custom agent definitions.

**Directory:** N/A.

**Invocation:** Plan mode vs. Act mode switching. Custom workflows via markdown files in workflows directory.

**Agent-Adjacent Features:**
- **Plan/Act modes**: Plan mode analyzes without modifying; Act mode executes with approval. Not configurable per-agent.
- **Custom workflows**: Drop a `.md` file into the workflows directory to create a `/slash-command`. No structured metadata.
- **MCP extensibility**: Add custom tools via MCP servers. "Ask Cline to add a tool" creates and installs MCP servers.
- **CLI 2.0** (Feb 2026): Full terminal agent with tmux-based multi-agent via separate processes. Each CLI instance is fully isolated.

**Notes:**
- Cline takes a framework-oriented approach (MCP tools, workflows, Plan/Act) rather than named custom agents.
- **Roo Code** (a Cline fork) added the custom modes system that Cline itself does not have.
- Cline CLI 2.0 achieves multi-agent via process isolation (tmux panes), not a config-driven agent system.
- `.clinerules` files exist for instructions but are analogous to rules, not agents.
- **Not a conversion target for agents** -- no structured format to render to.

**Source:** [docs.cline.bot/home](https://docs.cline.bot/home)

---

### OpenCode

**Status:** Agent support with a rich feature set -- more capable than the current syllago converter handles.

**Format:** Markdown with YAML frontmatter.

**Directory:** `.opencode/agents/*.md` (project-level) or `~/.config/opencode/agents/` (user-level). Both plural and singular directory names supported.

**Invocation:** Tab key to cycle agents, `@agent` mention, `default_agent` config setting.

**Frontmatter Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | No | Brief description of purpose |
| `mode` | string | No | `primary`, `subagent`, or `all` (default) |
| `model` | string | No | `provider/model-id` format override |
| `temperature` | float | No | 0.0-1.0 response randomness |
| `top_p` | float | No | 0.0-1.0 alternative diversity control |
| `prompt` | string | No | System prompt (alternative to body) |
| `steps` | int | No | Max agentic iterations (equivalent to `maxTurns`) |
| `disable` | bool | No | Disable this agent |
| `hidden` | bool | No | Hide from `@` autocomplete (subagents only) |
| `color` | string | No | Hex color or theme color name |
| `permission.edit` | string | No | `ask`, `allow`, or `deny` |
| `permission.bash` | object/string | No | Command-specific or wildcard bash permissions |
| `permission.webfetch` | string | No | `ask`, `allow`, or `deny` |
| `permission.task` | object | No | Subagent invocation control |
| `tools` | map[string]bool | No | Enable/disable specific tools |

**OpenCode-Specific Features:**
- **`mode` field**: Distinguishes primary agents from subagents -- unique categorization
- **`temperature` and `top_p`**: Model parameter overrides in frontmatter
- **Granular permissions**: Per-tool permission levels (`ask`/`allow`/`deny`) for edit, bash, webfetch, task
- **`hidden` flag**: Hide subagents from autocomplete
- **`disable` flag**: Temporarily disable agents without deleting
- **`steps`** (not `maxTurns`): Different field name for max iterations
- **Provider-specific passthrough**: Any unrecognized field is passed to the model provider

**Notes:**
- OpenCode is significantly more capable than the current syllago converter handles. The converter only maps name, description, tools, model, and maxTurns.
- OpenCode supports `color` -- currently dropped by the converter.
- OpenCode's `permission` system is more granular than Claude Code's `permissionMode` -- per-tool instead of global.
- The `tools` field is a map[string]bool (enable/disable), not a string array -- different from Claude Code.
- Built-in agents: `build` (full access) and `plan` (read-only).

**Source:** [opencode.ai/docs/agents/](https://opencode.ai/docs/agents/)

---

### Roo Code

**Status:** Agent support via "custom modes" system -- conceptually similar to agents but with different terminology and architecture.

**Format:** YAML (`.roo/modes/*.yaml`, preferred) or JSON (`.roomodes` file). Both formats supported.

**Directory:** `.roo/modes/` (individual YAML files), `.roomodes` (single file, project root), or global `settings/custom_modes.yaml`.

**Invocation:** Mode switching in Roo Code UI. Orchestrator mode auto-delegates based on `whenToUse`.

**Custom Mode Schema:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `slug` | string | Yes | URL-safe identifier (`/^[a-zA-Z0-9-]+$/`) |
| `name` | string | Yes | Display name (can include emojis) |
| `description` | string | No | Short summary for mode selector UI |
| `roleDefinition` | string | Yes | System prompt / role identity (prepended as "You are Roo, {roleDefinition}") |
| `whenToUse` | string | No | Orchestrator delegation hint |
| `customInstructions` | string | No | Additional behavioral instructions |
| `groups` | array | Yes | Tool group permissions |
| `source` | string | No | `project` or `global` |

**Tool Groups:**

Groups can be simple strings or tuples with restrictions:
- `read` -- File reading, search, glob
- `edit` -- File writing, editing. Supports `[edit, { fileRegex: "\\.md$", description: "..." }]` restriction tuples
- `command` -- Shell/terminal commands
- `browser` -- Web browsing
- `mcp` -- MCP server tools

**Roo Code-Specific Features:**
- **Tool group restrictions**: `[edit, { fileRegex, description }]` tuples constrain which files a group can operate on -- unique to Roo Code
- **Orchestrator delegation**: `whenToUse` field enables Orchestrator mode to auto-switch modes
- **`roleDefinition` vs body**: System prompt is a field, not the markdown body
- **Mode precedence**: Project modes override global modes with same slug
- **Built-in modes**: Architect, Code, Debug, Ask plus custom modes

**Notes:**
- Tool access is group-based, not individual tools -- lossy conversion from providers with per-tool control.
- No support for: model, maxTurns, permissionMode, skills, mcpServers (directly), memory, background, isolation, effort, hooks, color, disallowedTools.
- The `.roomodes` file wraps modes in a `customModes` array.
- Roo Code is a fork of Cline with added custom modes system.
- `roleDefinition` maps to the markdown body in canonical format.
- `whenToUse` maps to `description` in canonical format.

**Source:** [docs.roocode.com/features/custom-modes](https://docs.roocode.com/features/custom-modes)

---

### Zed

**Status:** No file-based custom agent definitions. Agents are configured through settings or the ACP registry.

**Format:** JSON settings or ACP registry entries.

**Directory:** No `.zed/agents/` directory. Agent servers configured in `settings.json` under `agent_servers`.

**Invocation:** Agent panel. Agent selection via dropdown.

**Configuration (settings.json):**

```json
{
  "agent_servers": {
    "My Custom Agent": {
      "type": "custom",
      "command": "node",
      "args": ["~/projects/agent/index.js", "--acp"],
      "env": {}
    }
  }
}
```

**Agent Profiles (tool groups, not agent definitions):**
- **Write**: All tools enabled (file editing + terminal)
- **Ask**: Read-only tools
- **Minimal**: No tools (general conversation)
- Custom profiles can be created to group tools

**Zed-Specific Features:**
- **ACP (Agent Client Protocol)**: Open protocol for external agents (Claude Code, Gemini CLI, Codex, etc.)
- **ACP Registry**: Preferred method for installing agents since v0.221.x
- **External agents**: First-class support for CLI-based agents via ACP
- **Priority**: Registry > Extension > Custom settings
- **`disable_ai` project setting**: Disable AI for specific projects

**Notes:**
- Zed does NOT have file-based agent definitions like other providers.
- Zed's "agents" are external processes speaking ACP, not configuration files with system prompts.
- Agent Profiles (Write/Ask/Minimal) are tool group presets, not custom agents.
- **Not a meaningful conversion target for agents** -- Zed agents are executables, not prompt files.
- Rules (.rules files) provide project-level instructions but are a separate concept from agents.

**Source:** [zed.dev/docs/ai/agent-settings](https://zed.dev/docs/ai/agent-settings), [zed.dev/docs/ai/external-agents](https://zed.dev/docs/ai/external-agents)

---

### Amp

**Status:** Partial agent support. Subagents exist but are not user-configurable via files. Skills provide task specialization.

**Format:** Markdown (AGENTS.md for instructions, SKILL.md for skills).

**Directory:** `.agents/skills/` (project-level) or `~/.config/agents/skills/` (global). AGENTS.md at any directory level.

**Invocation:** Auto-delegation. Natural language ("use subagents", "work in parallel").

**AGENTS.md:**
- Plain markdown with optional `globs` frontmatter for file-specific inclusion
- Auto-included from cwd, parent dirs (up to $HOME), and subtree dirs
- Falls back to CLAUDE.md if no AGENTS.md exists

**SKILL.md Frontmatter:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Skill identifier |
| `description` | string | Yes | When to load this skill |

**Subagent Behavior:**
- Subagents are generic "mini-Amps" with full tool access (read, write, terminal)
- Work in isolation: no communication between subagents, fresh context, no user guidance mid-task
- Main agent only receives final summary
- Amp may auto-spawn subagents or user can suggest parallel work

**Amp-Specific Features:**
- **Toolboxes**: Executable tools that describe themselves (`TOOLBOX_ACTION=describe`) and handle inputs (`TOOLBOX_ACTION=execute`)
- **Skills with toolbox executables**: Skills can include `toolbox/` directories with custom executables
- **Built-in agent types**: Oracle (GPT-5.2, expert advisor), Smart (primary coding agent), Deep (extended thinking)
- **CLAUDE.md fallback**: Reads CLAUDE.md when AGENTS.md absent -- good cross-tool compatibility

**Notes:**
- Amp does NOT have user-configurable custom agent files with metadata (no model, tools, permissions).
- Subagents are spawned automatically, not defined in configuration files.
- Skills (.agents/skills/) are the closest to agents but lack tool restrictions, model selection, etc.
- **Not a meaningful conversion target for agents** -- no structured agent definition format to render to. Content would go into AGENTS.md or SKILL.md.
- Migration path from Claude Code: `.claude/skills/` -> `.agents/skills/` (Amp also reads `.claude/skills/` for compatibility).

**Source:** [ampcode.com/agents-for-the-agent](https://ampcode.com/agents-for-the-agent), [ampcode.com/manual](https://ampcode.com/manual)

---

## Cross-Platform Normalization Problem

### Format Divergence

The agent ecosystem splits into fundamentally different paradigms:

1. **Structured agents** (Claude Code, Cursor, Gemini CLI, Copilot, Kiro, OpenCode, Codex): Define agents with metadata fields + system prompt. Varying levels of richness.
2. **Mode-based** (Roo Code): System prompt is a field (`roleDefinition`), tools are groups not individuals, no metadata beyond identity.
3. **Instruction-only** (Windsurf, Amp, Cline): No structured agent format -- instructions in plain markdown.
4. **External process** (Zed): Agents are executables, not configuration files.

### Key Normalization Challenges

**1. Tool Granularity Mismatch**
- Claude Code/Gemini/Codex: Individual tool names (`Read`, `Write`, `Bash`)
- Roo Code: Broad groups (`read`, `edit`, `command`) -- can't express "allow Read but not Glob"
- Kiro: Category shortcuts (`read`, `write`, `shell`) PLUS individual tools PLUS `@server/tool` patterns

**2. Permission Model Differences**
- Claude Code: Global `permissionMode` (5 modes)
- OpenCode: Per-tool permissions (`permission.edit: "allow"`, `permission.bash.pattern: "deny"`)
- Cursor: Binary `readonly` flag
- Roo Code: Group-level file regex restrictions

**3. Body vs. Field for System Prompt**
- Most providers: Markdown body after frontmatter IS the system prompt
- Roo Code: `roleDefinition` field IS the system prompt (no markdown body)
- Codex: `developer_instructions` (single) or `prompt` (multi) field in TOML

**4. Field Naming Conventions**
- camelCase: `maxTurns`, `mcpServers`, `permissionMode` (Claude Code, Kiro, OpenCode)
- snake_case: `max_turns`, `timeout_mins`, `mcp_servers` (Gemini CLI, Codex)
- kebab-case: `mcp-servers`, `disable-model-invocation` (Copilot)
- flat: `readonly`, `is_background` (Cursor)

**5. MCP Server Specification**
- Claude Code: String references OR inline definitions in `mcpServers` list
- Copilot: Inline definitions with `${{ secrets.NAME }}` support in `mcp-servers` object
- Kiro: References or inline + `includeMcpJson` flag to bulk-include
- Codex: TOML table under `agent.mcp_servers`

---

## Canonical Mapping

The canonical format (AgentMeta) maps to each provider as follows. "Body" means the markdown content after frontmatter.

| Canonical Field | Claude Code | Cursor | Gemini CLI | Copilot | Kiro | Codex | OpenCode | Roo Code |
|----------------|-------------|--------|------------|---------|------|-------|----------|----------|
| `name` | `name` | `name` | `name` | `name` | `name` | `agent.name` | filename | `name` |
| `description` | `description` | `description` | `description` | `description` | `description` | `agent.description` | `description` | `whenToUse` |
| `tools` | `tools` | -- | `tools` | `tools` | `tools`+`allowedTools` | `agent.tools` | `tools` (map) | `groups` (lossy) |
| `disallowedTools` | `disallowedTools` | -- | -- | -- | -- | -- | -- | -- |
| `model` | `model` | `model` | `model` | `model` | `model` | `agent.model` | `model` | -- |
| `maxTurns` | `maxTurns` | -- | `max_turns` | -- | -- | -- | `steps` | -- |
| `permissionMode` | `permissionMode` | `readonly`->`plan` | -- | -- | -- | -- | `permission.*` (granular) | -- |
| `skills` | `skills` | -- | -- | -- | `resources` (partial) | `agent.skills.config` | -- | -- |
| `mcpServers` | `mcpServers` | -- | -- | `mcp-servers` | `mcpServers` | `agent.mcp_servers` | -- | -- |
| `memory` | `memory` | -- | -- | -- | -- | -- | -- | -- |
| `background` | `background` | `is_background` | -- | -- | -- | -- | -- | -- |
| `isolation` | `isolation` | -- | -- | `target` | -- | `agent.sandbox_mode` (different) | -- | -- |
| `effort` | `effort` | -- | -- | -- | -- | `agent.model_reasoning_effort` | -- | -- |
| `hooks` | `hooks` | -- | -- | -- | `hooks` | -- | -- | -- |
| `color` | `color` | -- | -- | -- | -- | -- | `color` | -- |
| `temperature` | -- | -- | `temperature` | -- | -- | -- | `temperature` | -- |
| `timeout_mins` | -- | -- | `timeout_mins` | -- | -- | -- | -- | -- |
| `kind` | -- | -- | `kind` | -- | -- | -- | -- | -- |
| (body) | body | body | body | body (30K max) | body | `developer_instructions` | `prompt` or body | `roleDefinition` |

---

## Feature/Capability Matrix

Support levels: Full (native field), Partial (lossy conversion), Prose (embedded as text), None (dropped).

| Feature | Claude Code | Cursor | Gemini CLI | Copilot | Kiro | Codex | OpenCode | Roo Code | Windsurf | Cline | Zed | Amp |
|---------|------------|--------|------------|---------|------|-------|----------|----------|----------|-------|-----|-----|
| Custom agent files | Full | Full | Full | Full | Full | Full | Full | Full | Partial | None | None | None |
| Name/Description | Full | Full | Full | Full | Full | Full | Full | Full | -- | -- | -- | -- |
| System prompt (body) | Full | Full | Full | Full | Full | Full | Full | Full | Partial | -- | -- | -- |
| Tool allowlist | Full | None | Full | Full | Full | Full | Full | Partial | None | None | None | None |
| Tool denylist | Full | None | None | None | None | None | None | None | None | None | None | None |
| Model override | Full | Full | Full | Full | Full | Full | Full | None | None | None | None | None |
| Max turns/steps | Full | None | Full | None | None | None | Full | None | None | None | None | None |
| Permission modes | Full (5) | Partial (readonly) | None | None | None | None | Full (granular) | None | Partial | None | None | None |
| Skills | Full | None | None | None | Full | Full | None | None | None | None | None | None |
| MCP servers | Full | None | None | Full | Full | Full | None | None | None | None | None | None |
| Memory | Full | None | None | None | None | None | None | None | None | None | None | None |
| Background | Full | Full | None | None | None | None | None | None | None | None | None | None |
| Isolation/worktree | Full | None | None | Partial | None | Partial | None | None | None | None | None | None |
| Effort | Full | None | None | None | None | Full | None | None | None | None | None | None |
| Hooks | Full | None | None | None | Full | None | None | None | None | None | None | None |
| Color | Full | None | None | None | None | None | Full | None | None | None | None | None |
| Temperature | None | None | Full | None | None | None | Full | None | None | None | None | None |
| Timeout | None | None | Full | None | None | None | None | None | None | None | None | None |
| Remote agents | None | None | Full | None | None | None | None | None | None | None | None | None |

---

## Compat Scoring Implications

### Tier Classification for Agent Conversion

**Tier 1 -- Full structured agents (high compat):**
- Claude Code, Kiro, Codex, OpenCode: Rich metadata, most canonical fields have direct mappings

**Tier 2 -- Partial structured agents (medium compat):**
- Gemini CLI, Copilot: Reasonable subset of fields, some unique features
- Cursor: Very limited metadata (5 fields), but clean mapping for what exists

**Tier 3 -- Fundamentally different model (low compat):**
- Roo Code: Mode-based with tool groups, not individual tools. Lossy conversion.

**Tier 4 -- No agent format (no compat):**
- Windsurf, Cline, Zed, Amp: Cannot meaningfully receive agent definitions

### Scoring Factors

1. **Field coverage**: What percentage of canonical fields have native equivalents?
   - Claude Code: ~100% (it IS the canonical source)
   - Kiro: ~60% (name, description, tools, model, mcpServers, hooks + many unique fields)
   - Codex: ~50% (name, description, tools, model, effort, skills, mcpServers)
   - OpenCode: ~55% (name, description, tools, model, steps, color, temperature, permissions)
   - Gemini CLI: ~45% (name, description, tools, model, max_turns, temperature, timeout)
   - Copilot: ~35% (name, description, tools, model, target, mcp-servers)
   - Cursor: ~25% (name, description, model, readonly, is_background)
   - Roo Code: ~20% (slug/name, roleDefinition, whenToUse, groups)

2. **Lossy conversions**: Fields that lose fidelity during conversion
   - Individual tools -> Roo Code groups (lossy: can't express `Read` without `Glob`)
   - `permissionMode` (5 values) -> `readonly` (bool) (lossy: only `plan` maps cleanly)
   - `hooks` (structured) -> most providers (dropped entirely)
   - `mcpServers` (inline defs) -> most providers (name-only references)

3. **Unique fields lost**: Provider-specific fields with no canonical equivalent
   - Kiro: `toolAliases`, `toolsSettings`, `resources`, `includeMcpJson`, `includePowers`, `keyboardShortcut`, `welcomeMessage`
   - Codex: `nickname_candidates`, `sandbox_mode`
   - Copilot: `target` (vscode/github-copilot), `disable-model-invocation`, `user-invocable`, `metadata`
   - Gemini: `auth` block for remote agents
   - OpenCode: `mode`, `top_p`, `disable`, `hidden`, granular `permission.*`

---

## Converter Coverage Audit

### Current State (agents.go + codex_agents.go)

**Canonicalize (source -> canonical):**
| Provider | Covered | Notes |
|----------|---------|-------|
| claude-code | Yes | Direct YAML frontmatter parse |
| cursor | Yes | `readonly` -> `permissionMode: plan`, `is_background` -> `background` |
| gemini-cli | Yes | snake_case fields handled by YAML tags |
| copilot-cli | Yes | Generic frontmatter parse |
| kiro | Yes | Merges `tools` + `allowedTools`, translates tool names |
| codex | Yes | Both single-agent and multi-agent TOML |
| roo-code | **No** | Not covered -- would need to parse YAML/JSON mode format |
| opencode | **No** | Not covered -- would need frontmatter parse with different field names |
| windsurf | N/A | No structured agent format |
| cline | N/A | No structured agent format |
| zed | N/A | No file-based agent format |
| amp | N/A | No structured agent format |

**Render (canonical -> target):**
| Provider | Covered | Notes |
|----------|---------|-------|
| claude-code | Yes | Full frontmatter preserved |
| cursor | Yes | `permissionMode: plan` -> `readonly`, `background` -> `is_background` |
| gemini-cli | Yes | Tool name translation, behavioral notes for unsupported fields |
| copilot-cli | Yes | `isolation: worktree` -> `target: workspace`, `mcp-servers` key |
| kiro | Yes | Tool name translation, warnings for unsupported fields |
| codex | Yes | Renders single-agent TOML format |
| roo-code | Yes | Tool -> group mapping, roleDefinition from body |
| opencode | Yes | Subset frontmatter (name, description, tools, model, maxTurns) |
| windsurf | **No** | Could render as `.windsurf/workflows/` or AGENTS.md |
| cline | **No** | No target format |
| zed | **No** | No target format |
| amp | **No** | Could render as `.agents/skills/SKILL.md` |

### Gaps and Issues Found

**1. Copilot directory path changed** (Medium priority)
- Current converter uses `.github/copilot/agents/` naming convention
- Copilot now uses `.github/agents/` with `.agent.md` extension
- Should support both paths for backwards compatibility

**2. Copilot missing new fields** (Low priority)
- `disable-model-invocation`, `user-invocable`, `metadata` not in converter
- These are Copilot-specific with no canonical equivalent -- acceptable to ignore

**3. OpenCode renderer is too minimal** (High priority)
- Current: Only maps name, description, tools, model, maxTurns
- Missing: `color` (has canonical equivalent), `temperature` (has canonical equivalent), `steps` field name (uses `maxTurns` instead), `mode` (primary/subagent), `permission.*` granular permissions
- OpenCode's `tools` is a `map[string]bool`, not `string[]` -- current renderer may output wrong format

**4. OpenCode canonicalize not implemented** (Medium priority)
- No inbound conversion from OpenCode agent format
- Would need to handle: `steps` -> `maxTurns`, `permission.edit: deny` -> `permissionMode: plan`, `temperature` -> canonical `temperature`, `color` -> canonical `color`

**5. Roo Code canonicalize not implemented** (Medium priority)
- No inbound conversion from `.roomodes` or `.roo/modes/*.yaml`
- Would need to handle: `roleDefinition` -> body, `whenToUse` -> description, `groups` -> reverse tool group mapping, `slug` -> name

**6. Kiro canonicalize drops fields** (Low priority)
- `toolAliases`, `toolsSettings`, `resources`, `includeMcpJson`, `includePowers`, `keyboardShortcut`, `welcomeMessage` all dropped silently
- These have no canonical equivalent, so dropping is correct, but warnings should be emitted

**7. Kiro CLI JSON format not handled** (Low priority)
- Converter handles Kiro IDE format (MD + YAML frontmatter) but not CLI format (JSON)
- Low priority since IDE format is more common

**8. Codex missing `sandbox_mode` and `nickname_candidates` in canonicalize** (Low priority)
- These Codex-specific fields are parsed but not stored in canonical
- No canonical equivalent, dropping is correct

**9. Claude Code `permissionMode` values expanded** (Medium priority)
- Converter handles `plan`, `acceptEdits` but docs show 5 modes: `default`, `acceptEdits`, `dontAsk`, `bypassPermissions`, `plan`
- `dontAsk` and `bypassPermissions` should be preserved in canonical and rendered where possible

**10. Gemini `temperature` range** (Info)
- Gemini supports 0.0-2.0; OpenCode supports 0.0-1.0
- Cross-provider conversion may need clamping

**11. `effort` values expanded** (Low priority)
- Claude Code now supports `max` (Opus 4.6 only) in addition to `low`, `medium`, `high`
- Canonical should accept `max` and render it where supported

### Is Claude Code Still the Superset?

**Mostly yes, but with caveats:**

Claude Code remains the canonical reference because it has the broadest native field coverage. However, several providers have unique fields with no Claude Code equivalent:

| Provider | Unique Fields (no Claude Code equivalent) |
|----------|------------------------------------------|
| Gemini CLI | `temperature`, `timeout_mins`, `kind`, `auth` |
| Codex | `nickname_candidates`, `sandbox_mode` |
| Kiro | `toolAliases`, `toolsSettings`, `resources`, `includeMcpJson`, `includePowers`, `keyboardShortcut`, `welcomeMessage` |
| OpenCode | `mode`, `top_p`, `disable`, `hidden`, `permission.*` (granular) |
| Copilot | `target`, `disable-model-invocation`, `user-invocable`, `metadata` |
| Roo Code | `groups` (tool group model), `customInstructions` (separate from body) |

The canonical AgentMeta already stores Gemini-specific fields (`temperature`, `timeout_mins`, `kind`) for lossless round-trips. The same pattern could be extended for other provider-specific fields if round-trip fidelity becomes important. For now, dropping provider-specific fields with warnings is the correct approach.

### Recommended Priority Actions

1. **Fix OpenCode renderer** to use correct field names (`steps` not `maxTurns`) and include `color`, `temperature`
2. **Add OpenCode canonicalize** for inbound conversion
3. **Add Roo Code canonicalize** for inbound conversion from `.roomodes` / `.roo/modes/`
4. **Update Copilot directory** to `.github/agents/` with `.agent.md` extension support
5. **Add Kiro canonicalize warnings** for dropped unique fields
6. **Extend `permissionMode`** to handle all 5 Claude Code values and `effort: max`

---

*Sources:*
- *[Claude Code sub-agents docs](https://code.claude.com/docs/en/sub-agents)*
- *[Cursor subagents docs](https://cursor.com/docs/context/subagents)*
- *[Gemini CLI subagents docs](https://geminicli.com/docs/core/subagents/)*
- *[GitHub Copilot custom agents reference](https://docs.github.com/en/copilot/reference/custom-agents-configuration)*
- *[VS Code custom agents](https://code.visualstudio.com/docs/copilot/customization/custom-agents)*
- *[Windsurf AGENTS.md docs](https://docs.windsurf.com/windsurf/cascade/agents-md)*
- *[Kiro IDE subagents](https://kiro.dev/docs/chat/subagents/)*
- *[Kiro CLI agent config reference](https://kiro.dev/docs/cli/custom-agents/configuration-reference/)*
- *[Codex subagents](https://developers.openai.com/codex/subagents)*
- *[Cline docs](https://docs.cline.bot/home)*
- *[OpenCode agents docs](https://opencode.ai/docs/agents/)*
- *[Roo Code custom modes](https://docs.roocode.com/features/custom-modes)*
- *[Zed agent settings](https://zed.dev/docs/ai/agent-settings)*
- *[Zed external agents](https://zed.dev/docs/ai/external-agents)*
- *[Amp agents for the agent](https://ampcode.com/agents-for-the-agent)*
- *[Amp manual](https://ampcode.com/manual)*
