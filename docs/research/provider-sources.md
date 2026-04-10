# Provider Source Manifest — Research Findings

**Date:** 2026-03-27
**Purpose:** Exact fetchable URLs for every provider's content type definitions. Used to build the automated provider monitoring pipeline.

---

## How to Read This Document

Each provider section lists:
- **Fetch tier:** gh-api (best), llms-txt (good), html-scrape (worst)
- **Change detection:** How to know when content changed
- **Per content type:** Primary source URL, format, and what data it contains

Content types tracked: **Rules, Hooks, MCP, Skills, Agents, Commands**

Where a provider has both a **schema file** (machine-readable) and **docs** (human-readable), the schema is listed as primary and docs as secondary. Schema files are preferred for automated parsing; docs fill gaps.

---

## Gemini CLI

- **Repo:** `google-gemini/gemini-cli`
- **Fetch tier:** gh-api
- **Change detection:** GitHub Releases API — latest stable v0.35.2 (2026-03-26), also has preview + nightly tags
- **Changelog:** `docs/changelogs/latest.md`, `docs/changelogs/preview.md`

### Config Schema (master reference)

| Source | URL |
|--------|-----|
| JSON Schema (2020-12) | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/schemas/settings.schema.json` |
| TS source | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/packages/cli/src/config/settingsSchema.ts` |
| Config docs | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/configuration.md` |

Settings file: `~/.gemini/settings.json` (user), `.gemini/settings.json` (project)

### Rules

| Source | URL |
|--------|-----|
| Docs | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/gemini-md.md` |

Format: `GEMINI.md` (markdown, no frontmatter). Hierarchy: global (`~/.gemini/GEMINI.md`) > workspace > JIT. Supports `@file.md` imports.

### Hooks

| Source | URL |
|--------|-----|
| TS types (definitive) | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/packages/core/src/hooks/types.ts` |
| Schema (in settings) | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/schemas/settings.schema.json` (under `properties.hooks`) |
| Docs — overview | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/hooks/index.md` |
| Docs — I/O reference | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/hooks/reference.md` |
| Docs — writing hooks | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/hooks/writing-hooks.md` |
| Docs — best practices | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/hooks/best-practices.md` |

11 events: `BeforeTool`, `AfterTool`, `BeforeAgent`, `AfterAgent`, `SessionStart`, `SessionEnd`, `PreCompress`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `Notification`

Hook types: `command` only (external). Config fields: `type`, `command`, `name`, `description`, `timeout`, `env`, `source`. Matcher: regex for tools, exact string for lifecycle.

### MCP

| Source | URL |
|--------|-----|
| Schema (in settings) | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/schemas/settings.schema.json` (under `$defs/MCPServerConfig`) |
| Docs | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/tools/mcp-server.md` |
| Tutorial | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/tutorials/mcp-setup.md` |

Transports: stdio (`command`+`args`), SSE (`url`), HTTP (`httpUrl`), WebSocket (`tcp`). Fields include `trust`, `includeTools`, `excludeTools`, OAuth support.

### Skills

| Source | URL |
|--------|-----|
| Docs | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/creating-skills.md` |

Format: `SKILL.md` with YAML frontmatter (`name`, `description`). Dirs: `.gemini/skills/` (project), `~/.gemini/skills/` (user). Also discovers `.agents/skills/`.

### Commands

| Source | URL |
|--------|-----|
| Docs | `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/custom-commands.md` |

Format: TOML files in `.gemini/commands/`. Fields: `prompt` (required), `description` (optional). Supports `{{args}}` placeholder, `!{...}` shell injection, subdirectory namespacing.

### Agents

No dedicated agent system. Subagent support exists (`docs/core/subagents.md`) but no user-definable agent format.

---

## Codex (OpenAI)

- **Repo:** `openai/codex`
- **Fetch tier:** gh-api
- **Change detection:** GitHub Releases API — latest stable `rust-v0.117.0`, alpha `rust-v0.118.0-alpha.3`
- **Changelog:** `cliff.toml` (git-cliff generator), releases on GitHub. `CHANGELOG.md` is a stub redirecting to releases page.
- **Note:** This is a Rust monorepo (`codex-rs/`). Legacy Node CLI in `codex-cli/` appears deprecated.

### Config Schema (master reference)

| Source | URL |
|--------|-----|
| JSON Schema (draft-07) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/config.schema.json` |
| Generated TS types | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/app-server-protocol/schema/typescript/Settings.ts` |

Config file: `~/.codex/config.toml` (TOML, not JSON)

### Rules

| Source | URL |
|--------|-----|
| Docs (stub, redirects to developers.openai.com) | `https://raw.githubusercontent.com/openai/codex/main/docs/agents_md.md` |
| Example | `https://raw.githubusercontent.com/openai/codex/main/AGENTS.md` |

Format: `AGENTS.md` (hierarchical markdown). Supports `child_agents_md` feature flag for nested discovery.

### Hooks

| Source | URL |
|--------|-----|
| Event names (TS) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/app-server-protocol/schema/typescript/v2/HookEventName.ts` |
| Handler types (TS) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/app-server-protocol/schema/typescript/v2/HookHandlerType.ts` |
| Execution mode (TS) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/app-server-protocol/schema/typescript/v2/HookExecutionMode.ts` |
| Scope (TS) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/app-server-protocol/schema/typescript/v2/HookScope.ts` |
| Rust config | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/src/engine/config.rs` |
| Rust types | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/src/types.rs` |
| PreToolUse input schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/pre-tool-use.command.input.schema.json` |
| PreToolUse output schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/pre-tool-use.command.output.schema.json` |
| PostToolUse input schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/post-tool-use.command.input.schema.json` |
| PostToolUse output schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/post-tool-use.command.output.schema.json` |
| SessionStart input schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/session-start.command.input.schema.json` |
| SessionStart output schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/session-start.command.output.schema.json` |
| UserPromptSubmit input schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/user-prompt-submit.command.input.schema.json` |
| UserPromptSubmit output schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/user-prompt-submit.command.output.schema.json` |
| Stop input schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/stop.command.input.schema.json` |
| Stop output schema | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/hooks/schema/generated/stop.command.output.schema.json` |

5 events: `PreToolUse`, `PostToolUse`, `SessionStart`, `UserPromptSubmit`, `Stop` (PascalCase wire names)

Handler types: `command`, `prompt`, `agent`. Execution modes: `sync`, `async`. Scopes: `thread`, `turn`.

### MCP

| Source | URL |
|--------|-----|
| Schema (in config.schema.json) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/config.schema.json` (under `RawMcpServerConfig`) |
| MCP interface doc | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/docs/codex_mcp_interface.md` |

Config format: TOML `[mcp_servers.<name>]`. Transports: stdio (`command`+`args`), SSE/HTTP (`url`). Fields include OAuth, bearer tokens, tool enable/disable, per-tool approval mode.

### Skills

| Source | URL |
|--------|-----|
| Skill model (Rust) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core-skills/src/model.rs` |
| Skill loader (Rust) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core-skills/src/loader.rs` |
| Docs (stub) | `https://raw.githubusercontent.com/openai/codex/main/docs/skills.md` |

Format: `SKILL.md` with YAML frontmatter (`name`, `description`, `short-description`). Optional companion `agents/openai.yaml` with interface metadata (icon, brand color, default prompt). Scanned recursively (max depth 6).

### Agents

| Source | URL |
|--------|-----|
| Schema (in config.schema.json) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/config.schema.json` (under `AgentsToml` / `AgentRoleToml`) |
| Role system (Rust) | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/src/agent/role.rs` |
| Built-in: awaiter | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/src/agent/builtins/awaiter.toml` |
| Built-in: explorer | `https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/src/agent/builtins/explorer.toml` |

Agent config fields: `config_file`, `description`, `nickname_candidates`. Global settings: `max_depth`, `max_threads`, `job_max_runtime_seconds`.

### Commands

| Source | URL |
|--------|-----|
| Docs (stub) | `https://raw.githubusercontent.com/openai/codex/main/docs/slash_commands.md` |

No detailed source for custom command format found in repo — docs redirect to `developers.openai.com/codex`.

---

## Cline

- **Repo:** `cline/cline`
- **Fetch tier:** gh-api
- **Change detection:** GitHub Releases API — latest v3.76.0 (2026-03-26). `CHANGELOG.md` at root.
- **Note:** VS Code extension. Config lives in VS Code globalState, not standalone files.

### Rules

| Source | URL |
|--------|-----|
| Docs (MDX) | `https://raw.githubusercontent.com/cline/cline/main/docs/customization/cline-rules.mdx` |

Format: `.clinerules/` directory with `.md` and `.txt` files (concatenated). Global: `~/Documents/Cline/Rules/`. Also reads `.cursorrules`, `.windsurfrules`, `AGENTS.md` for cross-tool compat.

### Hooks

| Source | URL |
|--------|-----|
| Hook factory (types) | `https://raw.githubusercontent.com/cline/cline/main/src/core/hooks/hook-factory.ts` |
| Hook executor | `https://raw.githubusercontent.com/cline/cline/main/src/core/hooks/hook-executor.ts` |
| Script templates | `https://raw.githubusercontent.com/cline/cline/main/src/core/hooks/templates.ts` |
| Docs (MDX) | `https://raw.githubusercontent.com/cline/cline/main/docs/customization/hooks.mdx` |

9 events: `TaskStart`, `TaskResume`, `TaskCancel`, `TaskComplete`, `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Notification`, `PreCompact`

Hook I/O: JSON via stdin, JSON to stdout (`{ cancel, contextModification?, errorMessage? }`). Shell scripts (bash/PowerShell). 30s timeout, 50KB max context modification. Stored in `.clinerules/hooks/` (project) or `~/Documents/Cline/Hooks/` (global).

### MCP

| Source | URL |
|--------|-----|
| Zod schemas | `https://raw.githubusercontent.com/cline/cline/main/src/services/mcp/schemas.ts` |
| MCP hub | `https://raw.githubusercontent.com/cline/cline/main/src/services/mcp/McpHub.ts` |
| Types | `https://raw.githubusercontent.com/cline/cline/main/src/shared/mcp.ts` |
| Docs (MDX) | `https://raw.githubusercontent.com/cline/cline/main/docs/mcp/adding-and-configuring-servers.mdx` |

Settings file: `cline_mcp_settings.json` (VS Code globalStoragePath). Transports: stdio, SSE, streamableHttp (camelCase). Fields: `autoApprove[]`, `disabled`, `timeout` (default 60s).

### Skills

| Source | URL |
|--------|-----|
| Example | `https://raw.githubusercontent.com/cline/cline/main/.agents/skills/create-pull-request/SKILL.md` |

Format: `.agents/skills/*/SKILL.md`. No formal schema found — follows the emerging cross-tool SKILL.md convention.

### Agents

No dedicated agent system.

### Commands

| Source | URL |
|--------|-----|
| Slash commands (TS) | `https://raw.githubusercontent.com/cline/cline/main/cli/src/utils/slash-commands.ts` |

CLI slash commands only — no user-definable command files.

---

## Roo Code

- **Repo:** `RooVetGit/Roo-Code`
- **Fetch tier:** gh-api
- **Change detection:** GitHub Releases API — latest v3.51.1 (2026-03-08). `CHANGELOG.md` at root. Uses changesets (`.changeset/`).
- **Note:** Cline fork with significant divergence. Has custom modes (unique), no hooks.

### Rules

| Source | URL |
|--------|-----|
| Global rules example | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.roo/rules/rules.md` |
| Per-mode rules example | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.roo/rules-code/use-safeWriteJson.md` |
| Ignore file | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.rooignore` |

Format: `.roo/rules/` (global to all modes) + `.roo/rules-{mode-slug}/` (per-mode, e.g. `.roo/rules-code/`). Markdown and XML files. Also reads `.clinerules/` for backward compat.

### Hooks

**Not supported.** Major divergence from Cline — Roo Code has no lifecycle hook system.

### MCP

| Source | URL |
|--------|-----|
| MCP hub (inline schemas) | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/src/services/mcp/McpHub.ts` |
| Types | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/packages/types/src/mcp.ts` |
| Global file names | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/src/shared/globalFileNames.ts` |

Settings file: `mcp_settings.json`. Transports: stdio, SSE, `streamable-http` (kebab-case — differs from Cline's `streamableHttp`). Fields: `alwaysAllow[]` (not `autoApprove`), `disabledTools[]`, `watchPaths[]`, `disabled`, `timeout`.

### Skills

| Source | URL |
|--------|-----|
| Example | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.roo/skills/roo-translation/SKILL.md` |

Format: `.roo/skills/*/SKILL.md`.

### Agents (Custom Modes — unique to Roo Code)

| Source | URL |
|--------|-----|
| Zod schema + built-in modes | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/packages/types/src/mode.ts` |
| Mode manager | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/src/core/config/CustomModesManager.ts` |
| Project modes example | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.roomodes` |

Format: `.roomodes` (project, YAML) or `custom_modes.yaml` (global). Schema: `slug`, `name`, `roleDefinition`, `whenToUse?`, `description?`, `customInstructions?`, `groups[]` (tool access: read, edit, command, mcp). 5 built-in: architect, code, ask, debug, orchestrator.

### Commands

| Source | URL |
|--------|-----|
| Example | `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.roo/commands/commit.md` |

Format: `.roo/commands/*.md`. Markdown command files.

---

## OpenCode → Crush

- **Repo (archived):** `opencode-ai/opencode`
- **Repo (active successor):** `charmbracelet/crush`
- **Fetch tier:** gh-api
- **Change detection:** GitHub Releases API — OpenCode v0.0.55 (archived), Crush v0.53.0 (active). Crush has auto-updated `schema.json` via CI workflow.
- **Note:** OpenCode is archived. Renamed to Crush under Charm. Both have root-level JSON schemas. **Neither has hooks.**

### Config Schema (master reference)

| Source | URL |
|--------|-----|
| OpenCode JSON Schema (draft-07) | `https://raw.githubusercontent.com/opencode-ai/opencode/main/opencode-schema.json` |
| OpenCode Go structs | `https://raw.githubusercontent.com/opencode-ai/opencode/main/internal/config/config.go` |
| Crush JSON Schema (2020-12) | `https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json` |
| Crush Go structs | `https://raw.githubusercontent.com/charmbracelet/crush/main/internal/config/config.go` |
| Crush hosted schema | `https://charm.land/crush.json` |

Config file: `.opencode.json` (OpenCode) → `crush.json` (Crush, no leading dot)

### Rules

| Source | URL |
|--------|-----|
| Context loader | `https://raw.githubusercontent.com/opencode-ai/opencode/main/internal/llm/prompt/prompt.go` |

No dedicated rules system. Uses `contextPaths` config — a list of files auto-loaded into system prompt. OpenCode defaults: `.github/copilot-instructions.md`, `.cursorrules`, `.cursor/rules/`, `CLAUDE.md`, `opencode.md`, etc. Crush adds: `GEMINI.md`, `crush.md`, `AGENTS.md`.

### Hooks

**Not supported.** No hook system in either OpenCode or Crush.

### MCP

| Source | URL |
|--------|-----|
| OpenCode schema (in opencode-schema.json) | `https://raw.githubusercontent.com/opencode-ai/opencode/main/opencode-schema.json` (under `mcpServers`) |
| OpenCode MCP client | `https://raw.githubusercontent.com/opencode-ai/opencode/main/internal/llm/agent/mcp-tools.go` |
| Crush schema (in schema.json) | `https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json` (under `mcp`) |

OpenCode: `command`, `args[]`, `env[]` (string array), `type` (stdio/sse), `url`, `headers`. Crush adds: `http` type, `disabled`, `disabled_tools[]`, `timeout`, `env` changed to `map[string]string`.

### Skills

No formal skill system. The `azat-io/ai-config` adapter maps skills to a `skill/` directory with `SKILL.md` files, but this appears to be an external convention, not native.

### Agents

| Source | URL |
|--------|-----|
| Agent system | `https://raw.githubusercontent.com/opencode-ai/opencode/main/internal/llm/agent/agent.go` |

4 built-in agents (not user-definable): `coder`, `summarizer`, `task`, `title`. Configured via `config.Agent` struct (`model`, `maxTokens`, `reasoningEffort`). No user-facing agent file format.

### Commands

| Source | URL |
|--------|-----|
| Custom commands (TUI) | `https://raw.githubusercontent.com/opencode-ai/opencode/main/internal/tui/components/dialog/custom_commands.go` |

Markdown files in `$XDG_CONFIG_HOME/opencode/commands/` (user, prefix `user:`) or `<project>/.opencode/commands/` (project, prefix `project:`). Supports named arguments (`$ISSUE_NUMBER`, `$AUTHOR_NAME`).

---

## Claude Code

- **Repo:** `anthropics/claude-code`
- **Fetch tier:** gh-api
- **Change detection:** GitHub Releases API — latest v2.1.86. Very frequent releases (multiple per week).
- **Changelog:** `https://code.claude.com/docs/en/changelog` (hosted docs site, no raw markdown equivalent found)
- **Note:** Repo is public but contains examples and plugins, not the core source. No formal JSON Schema for settings. Official docs at `docs.anthropic.com` do not have a public GitHub source repo.

### Rules

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://code.claude.com/docs/en/memory` |

Format: `CLAUDE.md` (root) + `.claude/rules/*.md` (rule files with optional YAML frontmatter: `description`, `alwaysApply`, `globs`). Hierarchy: enterprise > organization > user > project > local. No raw markdown source in repo for the docs page.

### Hooks

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://code.claude.com/docs/en/hooks` |
| Hookify plugin schema | `https://raw.githubusercontent.com/anthropics/claude-code/main/plugins/hookify/hooks/hooks.json` |
| Settings examples | `https://raw.githubusercontent.com/anthropics/claude-code/main/examples/settings/settings-strict.json` |
| Settings examples | `https://raw.githubusercontent.com/anthropics/claude-code/main/examples/settings/settings-lax.json` |
| Hook example (Python) | `https://raw.githubusercontent.com/anthropics/claude-code/main/examples/hooks/bash_command_validator_example.py` |

Events (from our existing research): `PreToolUse`, `PostToolUse`, `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `Stop`, `PreCompact`, `PostCompact`, `Notification`, `SubagentStart`, `SubagentStop`, `PermissionRequest`, `WorktreeCreate`, `WorktreeRemove`, `FileChanged`, plus CC-specific events (TeammateIdle, TaskCreated, TaskCompleted, ConfigChange, InstructionsLoaded, CwdChanged, Elicitation, ElicitationResult, StopFailure). 26 total.

Hook config: JSON in `.claude/settings.json` under `hooks`. Fields: `type` (command), `bash`/`powershell` (platform-specific), `cwd`, `timeoutSec`. Matchers supported.

### MCP

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://code.claude.com/docs/en/mcp` |

Config file: `.mcp.json`. Transports: stdio, SSE, streamable-http. OAuth support. No schema file in repo.

### Skills

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://code.claude.com/docs/en/skills` |

Format: `.claude/skills/{name}/SKILL.md` with YAML frontmatter. Invoked via slash commands.

### Agents (Sub-agents)

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://code.claude.com/docs/en/sub-agents` |

Format: `.claude/agents/{name}.md` with YAML frontmatter.

### Commands

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://code.claude.com/docs/en/commands` |

Format: `.claude/commands/{name}.md` with YAML frontmatter. Invoked as `/{name}`.

---

## Copilot CLI

- **Repo (docs):** `github/docs` (content at `content/copilot/`)
- **Repo (CLI):** `github/copilot-cli` (sparse — install script and templates only, CLI is closed-source binary)
- **Fetch tier:** gh-api (via github/docs repo)
- **Change detection:** File commit SHAs — `gh api repos/github/docs/commits?path=content/copilot/reference/hooks-configuration.md`. Very active repo.
- **Note:** Docs use Liquid template variables (`{% data %}`) that need stripping during extraction.

### Rules

| Source | URL |
|--------|-----|
| CLI instructions how-to | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions.md` |
| Personal instructions | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/configure-custom-instructions/add-personal-instructions.md` |
| Repo instructions | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/configure-custom-instructions/add-repository-instructions.md` |
| Reference | `https://raw.githubusercontent.com/github/docs/main/content/copilot/reference/custom-instructions-support.md` |
| Cheat sheet | `https://raw.githubusercontent.com/github/docs/main/content/copilot/reference/customization-cheat-sheet.md` |

Format: `.github/copilot-instructions.md` (repo), personal instructions via settings.

### Hooks

| Source | URL |
|--------|-----|
| Hook config reference | `https://raw.githubusercontent.com/github/docs/main/content/copilot/reference/hooks-configuration.md` |
| CLI hooks how-to | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/copilot-cli/customize-copilot/use-hooks.md` |
| Coding agent hooks | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/use-copilot-agents/coding-agent/use-hooks.md` |
| About hooks (concept) | `https://raw.githubusercontent.com/github/docs/main/content/copilot/concepts/agents/coding-agent/about-hooks.md` |

The `hooks-configuration.md` reference is the primary source — contains inline JSON schemas for every hook event type.

### MCP

| Source | URL |
|--------|-----|
| CLI MCP how-to | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers.md` |
| MCP concept | `https://raw.githubusercontent.com/github/docs/main/content/copilot/concepts/context/mcp.md` |
| MCP management | `https://raw.githubusercontent.com/github/docs/main/content/copilot/concepts/mcp-management.md` |
| Coding agent MCP | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/use-copilot-agents/coding-agent/extend-coding-agent-with-mcp.md` |

### Skills

| Source | URL |
|--------|-----|
| About skills | `https://raw.githubusercontent.com/github/docs/main/content/copilot/concepts/agents/about-agent-skills.md` |
| CLI create skills | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/copilot-cli/customize-copilot/create-skills.md` |
| Coding agent skills | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/use-copilot-agents/coding-agent/create-skills.md` |

### Agents

| Source | URL |
|--------|-----|
| Agent config reference | `https://raw.githubusercontent.com/github/docs/main/content/copilot/reference/custom-agents-configuration.md` |
| CLI create agents | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/copilot-cli/customize-copilot/create-custom-agents-for-cli.md` |
| About custom agents | `https://raw.githubusercontent.com/github/docs/main/content/copilot/concepts/agents/coding-agent/about-custom-agents.md` |

The `custom-agents-configuration.md` is the primary source — defines YAML frontmatter schema for `.agent.md` files (name, description, tools, model, mcp-servers) plus tool alias table.

### Commands

No dedicated commands content type. Copilot CLI has "plugins" which serve a similar role:

| Source | URL |
|--------|-----|
| About CLI plugins | `https://raw.githubusercontent.com/github/docs/main/content/copilot/concepts/agents/copilot-cli/about-cli-plugins.md` |
| Plugin reference | `https://raw.githubusercontent.com/github/docs/main/content/copilot/reference/copilot-cli-reference/cli-plugin-reference.md` |
| Creating plugins | `https://raw.githubusercontent.com/github/docs/main/content/copilot/how-tos/copilot-cli/customize-copilot/plugins-creating.md` |

---

## Zed

- **Repo:** `zed-industries/zed` (fully open source, Rust)
- **Fetch tier:** gh-api
- **Change detection:** GitHub Releases API — latest v0.229.0. Very frequent releases.
- **Note:** Full source available. Config types derive `JsonSchema` via schemars crate. No standalone schema file, but `assets/settings/default.json` (101KB) serves as de facto reference.

### Config (master reference)

| Source | URL |
|--------|-----|
| Default settings (101KB) | `https://raw.githubusercontent.com/zed-industries/zed/main/assets/settings/default.json` |
| Agent settings (Rust) | `https://raw.githubusercontent.com/zed-industries/zed/main/crates/agent_settings/src/agent_settings.rs` |
| Agent profiles (Rust) | `https://raw.githubusercontent.com/zed-industries/zed/main/crates/agent_settings/src/agent_profile.rs` |

### Rules

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://zed.dev/docs/ai/rules` |
| Example (repo's own rules) | `https://raw.githubusercontent.com/zed-industries/zed/main/.rules` |

Format: `.rules` file (plain markdown, no frontmatter). Single file, no directory structure.

### Hooks

**Not supported.** Zed has no lifecycle hook system.

### MCP

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://zed.dev/docs/ai/mcp` |
| Context server crate | `https://raw.githubusercontent.com/zed-industries/zed/main/crates/context_server/src/context_server.rs` |

MCP configured in Zed settings JSON under agent profiles. Source code defines the transport/config handling.

### Skills

**Not supported.** No skill system.

### Agents (Profiles)

| Source | URL |
|--------|-----|
| Agent settings (Rust) | `https://raw.githubusercontent.com/zed-industries/zed/main/crates/agent_settings/src/agent_settings.rs` |
| Agent profiles (Rust) | `https://raw.githubusercontent.com/zed-industries/zed/main/crates/agent_settings/src/agent_profile.rs` |
| Docs (hosted) | `https://zed.dev/docs/ai/agent-settings` |
| Docs (hosted) | `https://zed.dev/docs/ai/configuration` |

Builtin profiles: `write`, `ask`, `minimal`. Profiles define: tools, context servers, model selection.

### Commands

| Source | URL |
|--------|-----|
| Slash command crate | `https://raw.githubusercontent.com/zed-industries/zed/main/crates/assistant_slash_command/src/assistant_slash_command.rs` |

Slash commands are built into the binary — no user-definable command files.

---

## Windsurf

- **Repo:** None useful (`codeiumdev/windsurf` is an empty POC)
- **Fetch tier:** llms-txt
- **Change detection:** Content hashing of fetched pages. No releases API.
- **Docs index:** `https://docs.windsurf.com/llms.txt` (97 URLs, Mintlify-hosted, all `.md` endpoints)
- **Note:** Docs site has two parallel hierarchies: `/plugins/` (IDE plugin) and `/windsurf/` (standalone IDE). The `/windsurf/` paths are the primary ones for the standalone product.

### Rules / Memories

| Source | URL |
|--------|-----|
| Memories (standalone) | `https://docs.windsurf.com/windsurf/cascade/memories.md` |
| Memories (plugin) | `https://docs.windsurf.com/plugins/cascade/memories.md` |
| AGENTS.md support | `https://docs.windsurf.com/windsurf/cascade/agents-md.md` |

Format: `.windsurfrules` (plain text) + Cascade memories system. Also reads `AGENTS.md`.

### Hooks

| Source | URL |
|--------|-----|
| Hooks docs | `https://docs.windsurf.com/windsurf/cascade/hooks.md` |

Windsurf hooks use per-tool-category events (split model): `pre_run_command`/`post_run_command`, `pre_mcp_tool_use`/`post_mcp_tool_use`, `pre_read_code`/`post_read_code`, `session_start`, `session_end`.

### MCP

| Source | URL |
|--------|-----|
| MCP (standalone) | `https://docs.windsurf.com/windsurf/cascade/mcp.md` |
| MCP (plugin) | `https://docs.windsurf.com/plugins/cascade/mcp.md` |

### Skills

| Source | URL |
|--------|-----|
| Skills docs | `https://docs.windsurf.com/windsurf/cascade/skills.md` |

### Agents

Covered by `agents-md.md` under Rules above. No separate agent system beyond AGENTS.md support.

### Commands

| Source | URL |
|--------|-----|
| Command overview | `https://docs.windsurf.com/command/windsurf-overview.md` |

### Additional (Windsurf-specific)

| Source | URL |
|--------|-----|
| Workflows | `https://docs.windsurf.com/windsurf/cascade/workflows.md` |
| Modes | `https://docs.windsurf.com/windsurf/cascade/modes.md` |
| Worktrees | `https://docs.windsurf.com/windsurf/cascade/worktrees.md` |

---

## Amp

- **Repo:** `sourcegraph/amp-examples-and-guides` (examples), `sourcegraph/amp-contrib` (community tools)
- **Fetch tier:** gh-api + llms-txt (stub)
- **Change detection:** Commit SHAs on GitHub repos. `ampcode.com/llms.txt` exists but only lists 2 URLs (manual + SDK).
- **Docs:** `https://ampcode.com/manual` (HTML, no raw markdown endpoints)

### Rules

| Source | URL |
|--------|-----|
| AGENT.md example | `https://raw.githubusercontent.com/sourcegraph/amp-examples-and-guides/main/AGENT.md` |
| AGENT.md best practices | `https://raw.githubusercontent.com/sourcegraph/amp-examples-and-guides/main/guides/agent-file/AGENT.md` |
| Docs (hosted) | `https://ampcode.com/manual` |

Format: `AGENT.md` (plain markdown). Amp's equivalent of CLAUDE.md/.cursorrules.

### Hooks

**Limited.** Amp has "checks" (not traditional hooks). No formal hook documentation found in repo. The competitive landscape research notes Amp has no hook support per se.

### MCP

| Source | URL |
|--------|-----|
| MCP setup guide | `https://raw.githubusercontent.com/sourcegraph/amp-examples-and-guides/main/guides/mcp/amp-mcp-setup-guide.md` |

### Skills

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://ampcode.com/news/agent-skills` |

Amp has skills but documentation is thin (announcement blog post, not a reference page).

### Agents

No dedicated agent file format beyond `AGENT.md`.

### Commands

No formal command system found.

### Settings

| Source | URL |
|--------|-----|
| Settings example | `https://raw.githubusercontent.com/sourcegraph/amp-contrib/main/sandbox/amp-srt-settings.json` |

---

## Kiro

- **Repo:** None (closed source). `aws-samples/sample-kiro-assistant` has minimal samples.
- **Fetch tier:** html-scrape
- **Change detection:** Content hashing. No releases API, no public repo.
- **Docs:** `https://kiro.dev/docs/` (redirects to `www.kiro.dev/docs/`). No `llms.txt` found.
- **Note:** Fastest-changing provider. Docs structure and features are unstable. Treat all data as best-effort.

### Rules (Steering)

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://kiro.dev/docs/steering/` |

Format: `.kiro/steering/` directory.

### Hooks

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://kiro.dev/docs/hooks/` |

Events: `Pre Tool Use`, `Post Tool Use`, `Prompt Submit`, `Agent Stop`, `File Save`, `File Create`, `File Delete`, `Manual Trigger`, `Pre/Post Task Execution`.

### MCP

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://kiro.dev/docs/mcp/configuration/` |

### Skills (Powers)

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://kiro.dev/docs/powers/create/` |

### Agents

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://kiro.dev/docs/cli/custom-agents/configuration-reference/` |

### Commands

No formal command system documented for Kiro.

---

## Cursor

- **Repo:** None (closed source, no public repos under `getcursor` or `anysphere`)
- **Fetch tier:** html-scrape
- **Change detection:** Content hashing. No releases API, no public repo.
- **Docs:** `cursor.com/docs` (no `llms.txt`, no raw markdown)
- **Changelog:** `https://cursor.com/changelog` (hosted)
- **Note:** Most opaque provider. Minimal official docs. Community knowledge fills gaps.

### Rules

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://cursor.com/docs/rules` |

Format: `.cursorrules` (legacy, plain text) and `.cursor/rules/*.mdc` (MDC = markdown with frontmatter). Frontmatter fields: `description`, `alwaysApply`, `globs`. Rules directory supports file-scoped rules via globs.

### Hooks

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://cursor.com/docs/hooks` |

Events (from competitive landscape research): `preToolUse`, `postToolUse`, `sessionStart`, `sessionEnd`, `beforeSubmitPrompt`, `preCompact`, `subagentStart`, `subagentStop`, `worktreeCreate`, `worktreeRemove`, `beforeShellExecution`, `afterShellExecution`, `beforeMCPExecution`, `afterMCPExecution`, `beforeReadFile`, `afterFileEdit`, `beforeTabFileRead`, `afterTabFileEdit`, `beforeAgentResponse`, `afterAgentResponse`, `afterAgentThought`, `beforeToolSelection`, `postToolUseFailure` (camelCase).

### MCP

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://cursor.com/docs/mcp` |

### Skills

| Source | URL |
|--------|-----|
| Docs (hosted) | `https://cursor.com/docs/skills` (unconfirmed — may not exist) |

Cursor skills support is limited/unclear from available documentation.

### Agents

No dedicated agent system documented.

### Commands

No formal command system documented for Cursor.

---

## Summary: Fetch Strategy Matrix

| Provider | Tier | Primary Source | Schema? | Change Detection |
|----------|------|----------------|---------|-----------------|
| Gemini CLI | gh-api | `google-gemini/gemini-cli` | JSON Schema (2020-12) | Releases API |
| Codex | gh-api | `openai/codex` | JSON Schema (draft-07) + per-event schemas | Releases API |
| OpenCode/Crush | gh-api | `charmbracelet/crush` | JSON Schema (2020-12) | Releases API |
| Cline | gh-api | `cline/cline` | Zod schemas in source | Releases API |
| Roo Code | gh-api | `RooVetGit/Roo-Code` | Zod schemas in source | Releases API |
| Claude Code | gh-api | `anthropics/claude-code` | Examples only (no formal schema) | Releases API |
| Copilot CLI | gh-api | `github/docs` | Inline in markdown | File commit SHAs |
| Zed | gh-api | `zed-industries/zed` | Rust structs w/ `#[derive(JsonSchema)]` | Releases API |
| Amp | gh-api + llms-txt | `sourcegraph/amp-*` | None | Commit SHAs |
| Windsurf | llms-txt | `docs.windsurf.com/llms.txt` | None | Content hashing |
| Cursor | html-scrape | `cursor.com/docs` | None | Content hashing |
| Kiro | html-scrape | `kiro.dev/docs` | None | Content hashing |

## Summary: Content Type Coverage

| Provider | Rules | Hooks | MCP | Skills | Agents | Commands |
|----------|-------|-------|-----|--------|--------|----------|
| Gemini CLI | GEMINI.md | 11 events | stdio/SSE/HTTP/WS | SKILL.md | — | TOML files |
| Codex | AGENTS.md | 5 events | TOML config | SKILL.md | TOML roles | stub docs |
| OpenCode/Crush | contextPaths | — | JSON config | — | built-in only | markdown files |
| Cline | .clinerules/ | 9 events | JSON (Zod) | SKILL.md | — | CLI slash cmds |
| Roo Code | .roo/rules/ | — | JSON (Zod) | SKILL.md | .roomodes (YAML) | markdown files |
| Claude Code | CLAUDE.md | 26 events | .mcp.json | SKILL.md | .md frontmatter | .md frontmatter |
| Copilot CLI | copilot-instructions.md | inline schemas | markdown docs | markdown docs | .agent.md (YAML fm) | plugins |
| Zed | .rules | — | settings JSON | — | profiles (Rust) | built-in only |
| Windsurf | .windsurfrules | split events | mintlify docs | mintlify docs | AGENTS.md | mintlify docs |
| Amp | AGENT.md | checks only | github guide | blog post | — | — |
| Kiro | .kiro/steering/ | ~9 events | hosted docs | powers | hosted docs | — |
| Cursor | .cursorrules/.mdc | ~23 events | hosted docs | unclear | — | — |

