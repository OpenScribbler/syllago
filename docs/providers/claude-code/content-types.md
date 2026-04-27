# Claude Code Content Types Reference

Comprehensive documentation of all content types supported by Claude Code, including file formats, directory structures, configuration schemas, and loading behavior.

**Last updated:** 2026-03-20

**Sources:**
- [Official] https://code.claude.com/docs/en/memory
- [Official] https://code.claude.com/docs/en/skills
- [Official] https://code.claude.com/docs/en/sub-agents
- [Official] https://code.claude.com/docs/en/hooks
- [Official] https://code.claude.com/docs/en/mcp
- [Official] https://code.claude.com/docs/en/settings
- [Official] https://code.claude.com/docs/en/commands
- [Official] https://code.claude.com/docs/en/plugins-reference
- [Official] https://code.claude.com/docs/en/features-overview

---

## Table of Contents

1. [Rules (CLAUDE.md and .claude/rules/)](#1-rules)
2. [Skills (.claude/skills/)](#2-skills)
3. [Agents (.claude/agents/)](#3-agents)
4. [MCP Servers (.mcp.json / settings.json)](#4-mcp-servers)
5. [Hooks (settings.json)](#5-hooks)
6. [Commands (.claude/commands/)](#6-commands)
7. [Plugins](#7-plugins)
8. [LSP Servers](#8-lsp-servers)
9. [Output Styles](#9-output-styles)
10. [Settings (settings.json)](#10-settings)

---

## 1. Rules

Rules are persistent instructions loaded into Claude's context at session start. They come in two forms: CLAUDE.md files and `.claude/rules/` files.

### 1a. CLAUDE.md Files

**File format:** Markdown (`.md`), UTF-8 encoded. No frontmatter. `[Official]`

**Purpose:** Project-wide, user-wide, or organization-wide instructions that Claude reads every session. `[Official]`

#### Directory Structure and Locations

| Scope | Location | Shared? |
|-------|----------|---------|
| Managed policy | macOS: `/Library/Application Support/ClaudeCode/CLAUDE.md`<br>Linux/WSL: `/etc/claude-code/CLAUDE.md`<br>Windows: `C:\Program Files\ClaudeCode\CLAUDE.md` | All users (org) `[Official]` |
| Project (option A) | `./CLAUDE.md` | Team via VCS `[Official]` |
| Project (option B) | `./.claude/CLAUDE.md` | Team via VCS `[Official]` |
| User | `~/.claude/CLAUDE.md` | Just you `[Official]` |
| Subdirectory | `<subdir>/CLAUDE.md` or `<subdir>/.claude/CLAUDE.md` | Team via VCS `[Official]` |

#### Features

- **`@path` imports:** Reference external files with `@path/to/file` syntax. Relative paths resolve relative to the containing file. Max depth: 5 hops. `[Official]`
- **HTML comments:** `<!-- ... -->` are hidden from Claude when auto-injected. `[Official]`
- **Recommended size:** Under 200 lines per file. `[Official]`
- **Exclusions:** `claudeMdExcludes` setting skips specific files by glob. Managed CLAUDE.md cannot be excluded. `[Official]`

#### How CLAUDE.md Files Load

1. Claude Code walks **up** the directory tree from the current working directory, loading every `CLAUDE.md` found. `[Official]`
2. User-level `~/.claude/CLAUDE.md` and managed CLAUDE.md load at startup. `[Official]`
3. Subdirectory CLAUDE.md files load **on demand** when Claude reads files in those subdirectories. `[Official]`
4. All levels are **additive** -- all contribute content simultaneously. More specific instructions take precedence when instructions conflict. `[Official]`
5. CLAUDE.md fully survives `/compact` -- re-read from disk and re-injected fresh. `[Official]`
6. Additional directories via `--add-dir` do NOT load CLAUDE.md by default. Set `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1` to enable. `[Official]`

#### Example

```markdown
# Project Instructions

See @README for project overview and @package.json for npm commands.

## Build
- Run `make build` before testing

## Code Style
- Use 2-space indentation
- Run `npm test` before committing

## Individual Preferences
- @~/.claude/my-project-instructions.md
```

### 1b. Rules Files (.claude/rules/)

**File format:** Markdown (`.md`), UTF-8 encoded. Optional YAML frontmatter for path scoping. `[Official]`

**Purpose:** Modular, topic-specific instructions. Can be scoped to specific file types. `[Official]`

#### Directory Structure

```
.claude/rules/           # Project-level rules (shared via VCS)
├── code-style.md
├── testing.md
├── security.md
├── frontend/
│   └── react-patterns.md
└── backend/
    └── api-design.md

~/.claude/rules/          # User-level rules (personal, all projects)
├── preferences.md
└── workflows.md
```

All `.md` files discovered **recursively**. Subdirectories allowed for organization. Symlinks supported (circular symlinks handled gracefully). `[Official]`

#### Frontmatter Schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `paths` | `string[]` | No | Glob patterns for path-scoped loading. If omitted, rule loads unconditionally. `[Official]` |

#### Glob Pattern Syntax for `paths`

| Pattern | Matches |
|---------|---------|
| `**/*.ts` | All TypeScript files in any directory |
| `src/**/*` | All files under `src/` |
| `*.md` | Markdown files in project root only |
| `src/components/*.tsx` | React components in specific dir |
| `src/**/*.{ts,tsx}` | Brace expansion for multiple extensions |

#### How Rules Load

- Rules **without** `paths` frontmatter load at launch with the same priority as `.claude/CLAUDE.md`. `[Official]`
- Rules **with** `paths` frontmatter load when Claude reads files matching the pattern (not on every tool use). `[Official]`
- User-level rules (`~/.claude/rules/`) load before project rules; project rules have higher priority. `[Official]`

#### Example: Path-Scoped Rule

```markdown
---
paths:
  - "src/api/**/*.ts"
  - "lib/**/*.ts"
  - "tests/**/*.test.ts"
---

# API Development Rules

- All API endpoints must include input validation
- Use the standard error response format
- Include OpenAPI documentation comments
```

---

## 2. Skills

Skills are reusable capabilities defined as markdown files that Claude can load on demand or that users invoke with `/skill-name`. Skills follow the [Agent Skills](https://agentskills.io) open standard. `[Official]`

**File format:** Markdown (`.md`) with YAML frontmatter. The entrypoint file must be named `SKILL.md`. `[Official]`

**Note:** Custom commands (`.claude/commands/`) have been merged into skills. Both work identically, but skills are the recommended approach. `[Official]`

### Directory Structure

```
.claude/skills/                     # Project-level skills
├── deploy/
│   ├── SKILL.md                    # Required entrypoint
│   ├── template.md                 # Optional supporting file
│   ├── examples/
│   │   └── sample.md
│   └── scripts/
│       └── validate.sh
└── review/
    └── SKILL.md

~/.claude/skills/                   # User-level skills (all projects)
├── explain-code/
│   └── SKILL.md
└── codebase-visualizer/
    ├── SKILL.md
    └── scripts/
        └── visualize.py

<plugin>/skills/                    # Plugin-provided skills
└── pdf-processor/
    └── SKILL.md
```

Each skill is a **directory** with `SKILL.md` as the required entrypoint. Supporting files are optional. `[Official]`

### Scope and Precedence

| Location | Scope | Priority |
|----------|-------|----------|
| Enterprise managed settings | All users in org | Highest `[Official]` |
| `~/.claude/skills/<name>/SKILL.md` | All your projects | Middle `[Official]` |
| `.claude/skills/<name>/SKILL.md` | This project only | Lower `[Official]` |
| `<plugin>/skills/<name>/SKILL.md` | Where plugin enabled | Lowest (namespaced) `[Official]` |

When skills share the same name across levels, higher-priority location wins. Plugin skills use `plugin-name:skill-name` namespace, so cannot conflict. `[Official]`

### Frontmatter Schema

All fields are optional. Only `description` is recommended. `[Official]`

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | No | Directory name | Display name. Lowercase letters, numbers, hyphens only. Max 64 chars. `[Official]` |
| `description` | `string` | Recommended | First paragraph of content | What the skill does. Claude uses this to decide when to apply it. `[Official]` |
| `argument-hint` | `string` | No | — | Hint shown during autocomplete. E.g., `[issue-number]`. `[Official]` |
| `disable-model-invocation` | `boolean` | No | `false` | If `true`, only user can invoke (not Claude). `[Official]` |
| `user-invocable` | `boolean` | No | `true` | If `false`, hidden from `/` menu. Only Claude can invoke. `[Official]` |
| `allowed-tools` | `string` | No | All tools | Comma-separated tool names Claude can use without asking permission. `[Official]` |
| `model` | `string` | No | Session model | Model to use when skill is active. `[Official]` |
| `effort` | `string` | No | Session level | Effort level: `low`, `medium`, `high`, `max`. `[Official]` |
| `context` | `string` | No | Inline | Set to `fork` to run in a forked subagent context. `[Official]` |
| `agent` | `string` | No | `general-purpose` | Which subagent type when `context: fork`. Options: `Explore`, `Plan`, `general-purpose`, or custom. `[Official]` |
| `hooks` | `object` | No | — | Hooks scoped to this skill's lifecycle. Same format as settings.json hooks. `[Official]` |

### String Substitutions

| Variable | Description |
|----------|-------------|
| `$ARGUMENTS` | All arguments passed when invoking the skill. `[Official]` |
| `$ARGUMENTS[N]` | Specific argument by 0-based index. `[Official]` |
| `$N` | Shorthand for `$ARGUMENTS[N]`. `[Official]` |
| `${CLAUDE_SESSION_ID}` | Current session ID. `[Official]` |
| `${CLAUDE_SKILL_DIR}` | Directory containing the skill's SKILL.md. `[Official]` |

### Dynamic Context Injection

The `` !`<command>` `` syntax runs shell commands before skill content is sent to Claude. Output replaces the placeholder. `[Official]`

```yaml
---
name: pr-summary
context: fork
agent: Explore
---

## Pull request context
- PR diff: !`gh pr diff`
- Changed files: !`gh pr diff --name-only`
```

### Invocation Behavior Summary

| Frontmatter | User can invoke? | Claude can invoke? | Context loading |
|-------------|-----------------|-------------------|-----------------|
| (default) | Yes | Yes | Description always in context; full content on invocation `[Official]` |
| `disable-model-invocation: true` | Yes | No | Description NOT in context; full content when user invokes `[Official]` |
| `user-invocable: false` | No | Yes | Description always in context; full content on invocation `[Official]` |

### Context Budget

Skill descriptions load at session start. Budget scales dynamically at 2% of context window, with fallback of 16,000 chars. Override with `SLASH_COMMAND_TOOL_CHAR_BUDGET` env var. `[Official]`

### Example

```yaml
---
name: deploy
description: Deploy the application to production
context: fork
agent: general-purpose
disable-model-invocation: true
allowed-tools: Bash(npm run *), Read
---

Deploy $ARGUMENTS to production:

1. Run the test suite: `npm test`
2. Build the application: `npm run build`
3. Push to deployment target
4. Verify deployment succeeded

For complete deployment procedures, see [reference.md](reference.md).
```

### Bundled Skills

These ship with Claude Code and are always available. `[Official]`

| Skill | Purpose |
|-------|---------|
| `/batch <instruction>` | Orchestrate large-scale parallel changes across codebase |
| `/claude-api` | Load Claude API reference for your language |
| `/debug [description]` | Troubleshoot current session |
| `/loop [interval] <prompt>` | Run prompt repeatedly on interval |
| `/simplify [focus]` | Review recent changes for quality issues |

---

## 3. Agents (Subagents)

Agents are specialized AI assistants that run in isolated context with custom system prompts, tool access, and permissions. `[Official]`

**File format:** Markdown (`.md`) with YAML frontmatter. The markdown body becomes the system prompt. `[Official]`

### Directory Structure

```
.claude/agents/                    # Project-level agents
├── code-reviewer.md
├── debugger.md
└── data-scientist.md

~/.claude/agents/                  # User-level agents (all projects)
├── security-reviewer.md
└── performance-tester.md

<plugin>/agents/                   # Plugin-provided agents
└── compliance-checker.md
```

Agent files are **individual markdown files** (not directories). `[Official]`

### Scope and Precedence

| Location | Scope | Priority |
|----------|-------|----------|
| `--agents` CLI flag (JSON) | Current session only | 1 (highest) `[Official]` |
| `.claude/agents/` | Current project | 2 `[Official]` |
| `~/.claude/agents/` | All your projects | 3 `[Official]` |
| `<plugin>/agents/` | Where plugin enabled | 4 (lowest) `[Official]` |

When multiple agents share the same name, higher-priority location wins. `[Official]`

### Frontmatter Schema

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | **Yes** | — | Unique identifier. Lowercase letters and hyphens. `[Official]` |
| `description` | `string` | **Yes** | — | When Claude should delegate to this agent. `[Official]` |
| `tools` | `string` | No | All tools (inherited) | Comma-separated tool names. Allowlist. `[Official]` |
| `disallowedTools` | `string` | No | — | Tools to deny. Denylist, removed from inherited set. `[Official]` |
| `model` | `string` | No | `inherit` | `sonnet`, `opus`, `haiku`, full model ID, or `inherit`. `[Official]` |
| `permissionMode` | `string` | No | `default` | `default`, `acceptEdits`, `dontAsk`, `bypassPermissions`, `plan`. `[Official]` |
| `maxTurns` | `integer` | No | — | Max agentic turns before stopping. `[Official]` |
| `skills` | `string[]` | No | — | Skills to preload into context at startup. Full content injected. `[Official]` |
| `mcpServers` | `array` | No | — | MCP servers: string references or inline definitions. `[Official]` |
| `hooks` | `object` | No | — | Lifecycle hooks scoped to this agent. Same format as settings.json. `[Official]` |
| `memory` | `string` | No | — | Persistent memory scope: `user`, `project`, or `local`. `[Official]` |
| `background` | `boolean` | No | `false` | Always run as background task. `[Official]` |
| `effort` | `string` | No | Session level | `low`, `medium`, `high`, `max`. `[Official]` |
| `isolation` | `string` | No | — | Set to `worktree` for isolated git worktree. `[Official]` |

**Note:** Plugin agents do NOT support `hooks`, `mcpServers`, or `permissionMode` fields (ignored for security). `[Official]`

### Memory Scopes

| Scope | Location | Use when |
|-------|----------|----------|
| `user` | `~/.claude/agent-memory/<agent-name>/` | Cross-project learnings `[Official]` |
| `project` | `.claude/agent-memory/<agent-name>/` | Project-specific, shareable via VCS `[Official]` |
| `local` | `.claude/agent-memory-local/<agent-name>/` | Project-specific, NOT in VCS `[Official]` |

### Built-in Agents

| Agent | Model | Tools | Purpose |
|-------|-------|-------|---------|
| `Explore` | Haiku | Read-only | Fast codebase search and analysis `[Official]` |
| `Plan` | Inherited | Read-only | Research for plan mode `[Official]` |
| `general-purpose` | Inherited | All | Complex multi-step tasks `[Official]` |
| `Bash` | Inherited | Terminal | Running terminal commands `[Official]` |
| `statusline-setup` | Sonnet | — | Status line configuration `[Official]` |
| `Claude Code Guide` | Haiku | — | Help about Claude Code features `[Official]` |

### CLI-Defined Agents (JSON)

Agents can be passed as JSON via `--agents` flag. Not saved to disk. `[Official]`

```bash
claude --agents '{
  "code-reviewer": {
    "description": "Expert code reviewer.",
    "prompt": "You are a senior code reviewer...",
    "tools": ["Read", "Grep", "Glob", "Bash"],
    "model": "sonnet"
  }
}'
```

The JSON uses `prompt` instead of the markdown body for the system prompt. All frontmatter fields are supported as JSON keys. `[Official]`

### Example

```markdown
---
name: code-reviewer
description: Expert code review specialist. Use immediately after writing or modifying code.
tools: Read, Grep, Glob, Bash
model: inherit
memory: project
---

You are a senior code reviewer ensuring high standards.

When invoked:
1. Run git diff to see recent changes
2. Focus on modified files
3. Begin review immediately

Review checklist:
- Code clarity and readability
- No duplicated code
- Proper error handling
- No exposed secrets
- Good test coverage

Provide feedback organized by priority:
- Critical issues (must fix)
- Warnings (should fix)
- Suggestions (consider improving)
```

---

## 4. MCP Servers

MCP (Model Context Protocol) servers connect Claude Code to external services and tools. `[Official]`

### Configuration Locations

| Scope | Location | Shared? |
|-------|----------|---------|
| Local (default) | `~/.claude.json` (under project path) | No `[Official]` |
| Project | `.mcp.json` in project root | Team via VCS `[Official]` |
| User | `~/.claude.json` (mcpServers field) | No `[Official]` |
| Managed | macOS: `/Library/Application Support/ClaudeCode/managed-mcp.json`<br>Linux/WSL: `/etc/claude-code/managed-mcp.json`<br>Windows: `C:\Program Files\ClaudeCode\managed-mcp.json` | Org-wide `[Official]` |
| Plugin | `.mcp.json` at plugin root, or inline in `plugin.json` | Where plugin enabled `[Official]` |
| Agent inline | `mcpServers` frontmatter in agent definition | Agent-scoped `[Official]` |

### Scope Precedence

Local > Project > User (when same server name exists at multiple scopes). `[Official]`

### .mcp.json Schema

**File format:** JSON, UTF-8 encoded. `[Official]`

```json
{
  "mcpServers": {
    "<server-name>": {
      // Server configuration (see transport types below)
    }
  }
}
```

### Transport Types

#### stdio (Local Command)

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/server",
      "args": ["--flag", "value"],
      "env": {
        "API_KEY": "${API_KEY}"
      },
      "cwd": "/working/directory"
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | `string` | **Yes** | Executable path or command name `[Official]` |
| `args` | `string[]` | No | Command-line arguments `[Official]` |
| `env` | `object` | No | Environment variables `[Official]` |
| `cwd` | `string` | No | Working directory `[Inferred]` |

#### HTTP (Remote)

```json
{
  "mcpServers": {
    "my-server": {
      "type": "http",
      "url": "https://mcp.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${API_KEY}"
      }
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `"http"` | **Yes** | Transport type `[Official]` |
| `url` | `string` | **Yes** | Server URL `[Official]` |
| `headers` | `object` | No | HTTP headers (supports `$VAR` interpolation) `[Official]` |
| `oauth` | `object` | No | OAuth configuration (see below) `[Official]` |

#### SSE (Server-Sent Events) -- Deprecated

```json
{
  "mcpServers": {
    "my-server": {
      "type": "sse",
      "url": "https://api.example.com/sse",
      "headers": {
        "X-API-Key": "your-key"
      }
    }
  }
}
```

Same fields as HTTP. SSE transport is deprecated; use HTTP instead. `[Official]`

#### OAuth Configuration

```json
{
  "oauth": {
    "clientId": "your-client-id",
    "callbackPort": 8080,
    "authServerMetadataUrl": "https://auth.example.com/.well-known/openid-configuration"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `clientId` | `string` | No | OAuth client ID `[Official]` |
| `callbackPort` | `integer` | No | Fixed port for OAuth callback `[Official]` |
| `authServerMetadataUrl` | `string` | No | Override metadata discovery URL (must be HTTPS) `[Official]` |

### Environment Variable Expansion

Supported in `command`, `args`, `env`, `url`, and `headers` fields. `[Official]`

| Syntax | Description |
|--------|-------------|
| `${VAR}` | Value of env var `VAR` `[Official]` |
| `${VAR:-default}` | `VAR` if set, otherwise `default` `[Official]` |

Plugin-specific variables:
- `${CLAUDE_PLUGIN_ROOT}` -- plugin installation directory `[Official]`
- `${CLAUDE_PLUGIN_DATA}` -- persistent plugin data directory `[Official]`

### Tool Naming Convention

MCP tools follow the pattern: `mcp__<server-name>__<tool-name>`. `[Official]`

Example: `mcp__memory__create_entities`, `mcp__filesystem__read_file`

### How MCP Loads

- All tool definitions and JSON schemas load at **session start**. `[Official]`
- Tool search (enabled by default) loads MCP tools up to 10% of context, deferring the rest. `[Official]`
- Supports `list_changed` notifications for dynamic tool updates. `[Official]`
- Connections can fail silently mid-session. Use `/mcp` to check status. `[Official]`

---

## 5. Hooks

Hooks are user-defined scripts, HTTP endpoints, LLM prompts, or agents that execute automatically at specific lifecycle points. `[Official]`

### Configuration Locations

| Location | Scope |
|----------|-------|
| `~/.claude/settings.json` | All projects (personal) `[Official]` |
| `.claude/settings.json` | Project (shared) `[Official]` |
| `.claude/settings.local.json` | Project (personal, gitignored) `[Official]` |
| Managed settings | Organization-wide `[Official]` |
| Plugin `hooks/hooks.json` | When plugin enabled `[Official]` |
| Skill/Agent frontmatter | While component active `[Official]` |

**Merging behavior:** All registered hooks fire for matching events regardless of source. They do NOT override by name like skills/agents. `[Official]`

### Configuration Schema

```json
{
  "hooks": {
    "<EventName>": [
      {
        "matcher": "regex_pattern",
        "hooks": [
          {
            "type": "command|http|prompt|agent",
            "command": "path/to/script.sh",
            "timeout": 600,
            "statusMessage": "Running validation...",
            "once": false,
            "async": false
          }
        ]
      }
    ]
  },
  "disableAllHooks": false
}
```

### Hook Events

#### Core Session Events

| Event | Fires when | Can block? | Matcher matches on |
|-------|-----------|-----------|-------------------|
| `SessionStart` | Session begins or resumes | No | `startup`, `resume`, `clear`, `compact` `[Official]` |
| `UserPromptSubmit` | User submits prompt | Yes | No matcher support `[Official]` |
| `SessionEnd` | Session terminates | No | `clear`, `resume`, `logout`, `prompt_input_exit`, `bypass_permissions_disabled`, `other` `[Official]` |

#### Tool Execution Events

| Event | Fires when | Can block? | Matcher matches on |
|-------|-----------|-----------|-------------------|
| `PreToolUse` | Before tool executes | Yes | Tool name (`Bash`, `Edit`, `Write`, `mcp__*`) `[Official]` |
| `PermissionRequest` | Permission dialog appears | Yes | Tool name `[Official]` |
| `PostToolUse` | After tool succeeds | No (but can block via decision) | Tool name `[Official]` |
| `PostToolUseFailure` | After tool fails | No | Tool name `[Official]` |

#### Agent Events

| Event | Fires when | Can block? | Matcher matches on |
|-------|-----------|-----------|-------------------|
| `SubagentStart` | Subagent spawned | No | Agent type name `[Official]` |
| `SubagentStop` | Subagent completes | Yes | Agent type name `[Official]` |
| `Stop` | Claude finishes responding | Yes | No matcher support `[Official]` |
| `StopFailure` | Turn ends due to API error | No | Error type (`rate_limit`, `authentication_failed`, etc.) `[Official]` |
| `TeammateIdle` | Agent team teammate about to idle | Yes | No matcher support `[Official]` |
| `TaskCompleted` | Task marked complete | Yes | No matcher support `[Official]` |

#### Configuration & Context Events

| Event | Fires when | Can block? | Matcher matches on |
|-------|-----------|-----------|-------------------|
| `InstructionsLoaded` | CLAUDE.md or rule file loaded | No | `session_start`, `nested_traversal`, `path_glob_match`, `include`, `compact` `[Official]` |
| `ConfigChange` | Config file changes | Yes (except policy_settings) | `user_settings`, `project_settings`, `local_settings`, `policy_settings`, `skills` `[Official]` |
| `PreCompact` | Before context compaction | No | `manual`, `auto` `[Official]` |
| `PostCompact` | After context compaction | No | `manual`, `auto` `[Official]` |

#### Worktree Events

| Event | Fires when | Can block? | Matcher matches on |
|-------|-----------|-----------|-------------------|
| `WorktreeCreate` | Worktree created | Yes | No matcher support `[Official]` |
| `WorktreeRemove` | Worktree being removed | No | No matcher support `[Official]` |

#### MCP & Notification Events

| Event | Fires when | Can block? | Matcher matches on |
|-------|-----------|-----------|-------------------|
| `Notification` | Claude sends notification | No | `permission_prompt`, `idle_prompt`, `auth_success`, `elicitation_dialog` `[Official]` |
| `Elicitation` | MCP server requests user input | Yes | MCP server name `[Official]` |
| `ElicitationResult` | User responds to MCP elicitation | Yes | MCP server name `[Official]` |

### Hook Handler Types

#### Command Hook

```json
{
  "type": "command",
  "command": "path/to/script.sh",
  "async": false,
  "timeout": 600,
  "statusMessage": "Running validation..."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `"command"` | **Yes** | Handler type `[Official]` |
| `command` | `string` | **Yes** | Shell command to execute `[Official]` |
| `async` | `boolean` | No | Run in background without blocking `[Official]` |
| `timeout` | `integer` | No | Timeout in seconds (default: 600) `[Official]` |
| `statusMessage` | `string` | No | Display message while running `[Official]` |
| `once` | `boolean` | No | Fire only once per session `[Inferred]` |

**Input:** JSON via stdin. **Output:** Exit code + stdout/stderr. `[Official]`

#### HTTP Hook

```json
{
  "type": "http",
  "url": "http://localhost:8080/hooks/pre-tool-use",
  "headers": {
    "Authorization": "Bearer $MY_TOKEN"
  },
  "allowedEnvVars": ["MY_TOKEN"],
  "timeout": 30
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `"http"` | **Yes** | Handler type `[Official]` |
| `url` | `string` | **Yes** | Endpoint URL `[Official]` |
| `headers` | `object` | No | HTTP headers (support `$VAR` interpolation) `[Official]` |
| `allowedEnvVars` | `string[]` | No | Env vars allowed in header interpolation `[Official]` |
| `timeout` | `integer` | No | Timeout in seconds `[Official]` |

#### Prompt Hook

```json
{
  "type": "prompt",
  "prompt": "Is this command safe?\n\n$ARGUMENTS",
  "model": "fast-model",
  "timeout": 30
}
```

Single-turn LLM evaluation. `$ARGUMENTS` placeholder for hook input JSON. Returns yes/no decision. `[Official]`

#### Agent Hook

```json
{
  "type": "agent",
  "prompt": "Verify this operation: $ARGUMENTS",
  "model": "claude-opus",
  "timeout": 60
}
```

Spawns subagent with tool access (Read, Grep, Glob). Returns a decision. `[Official]`

### Exit Codes (Command Hooks)

| Code | Meaning | Behavior |
|------|---------|----------|
| 0 | Success | Parse stdout for JSON `[Official]` |
| 2 | Blocking error | stderr fed back to Claude. Blocks the action `[Official]` |
| Other | Non-blocking error | stderr shown in verbose mode. Execution continues `[Official]` |

### Environment Variables Available to Hooks

| Variable | Description |
|----------|-------------|
| `$CLAUDE_PROJECT_DIR` | Project root directory `[Official]` |
| `${CLAUDE_PLUGIN_ROOT}` | Plugin installation directory `[Official]` |
| `${CLAUDE_PLUGIN_DATA}` | Plugin persistent data directory `[Official]` |
| `$CLAUDE_ENV_FILE` | (SessionStart only) File to write persistent env vars `[Official]` |
| `$CLAUDE_CODE_REMOTE` | `"true"` in web environments `[Official]` |

### Common Input Fields (All Hooks)

```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/current/working/directory",
  "permission_mode": "default",
  "hook_event_name": "PreToolUse"
}
```

Additional fields in subagent context: `agent_id`, `agent_type`. `[Official]`

### Hook-Related Settings

| Setting | Description |
|---------|-------------|
| `disableAllHooks` | Disable all hooks and custom status line `[Official]` |
| `allowManagedHooksOnly` | (Managed only) Block user/project/plugin hooks `[Official]` |
| `allowedHttpHookUrls` | Allowlist URL patterns for HTTP hooks. `*` wildcard. `[Official]` |
| `httpHookAllowedEnvVars` | Allowlist env var names for HTTP header interpolation `[Official]` |

---

## 6. Commands

Commands are the legacy form of skills. They are simple markdown files (not directories) that create `/command-name` shortcuts. `[Official]`

**Status:** Commands have been merged into skills. Existing `.claude/commands/` files continue to work. Skills are recommended for new content. If a skill and command share the same name, the skill takes precedence. `[Official]`

### Directory Structure

```
.claude/commands/          # Project-level commands
├── deploy.md
├── review.md
└── onboard.md

~/.claude/commands/        # User-level commands (all projects)
├── my-workflow.md
└── daily-report.md
```

**File format:** Markdown (`.md`), UTF-8 encoded. Optional YAML frontmatter (same fields as skills). `[Official]`

**Key difference from skills:** Commands are single `.md` files, not directories. They cannot have supporting files alongside them. Skills (`SKILL.md` in a directory) can include templates, examples, and scripts. `[Official]`

### Frontmatter

Commands support the same frontmatter fields as skills (see [Skills frontmatter](#frontmatter-schema-1)). `[Official]`

### Example

```yaml
---
description: Review a pull request
disable-model-invocation: true
---

Review PR $ARGUMENTS following our coding standards.

1. Check the PR description
2. Review each changed file
3. Look for security issues
4. Verify test coverage
5. Provide actionable feedback
```

---

## 7. Plugins

Plugins are the packaging layer that bundles skills, agents, hooks, MCP servers, LSP servers, and output styles into a single installable unit. `[Official]`

### Directory Structure

```
my-plugin/
├── .claude-plugin/              # Metadata directory (optional)
│   └── plugin.json              # Plugin manifest
├── commands/                    # Legacy commands (skills preferred)
│   └── status.md
├── agents/                      # Agent definitions
│   ├── security-reviewer.md
│   └── performance-tester.md
├── skills/                      # Skills
│   ├── code-reviewer/
│   │   └── SKILL.md
│   └── pdf-processor/
│       ├── SKILL.md
│       └── scripts/
├── hooks/                       # Hook configurations
│   └── hooks.json               # Main hook config
├── settings.json                # Default settings (only `agent` supported)
├── .mcp.json                    # MCP server definitions
├── .lsp.json                    # LSP server configurations
├── scripts/                     # Utility scripts
│   └── format-code.sh
├── LICENSE
└── CHANGELOG.md
```

Components MUST be at plugin root, NOT inside `.claude-plugin/`. Only `plugin.json` goes in `.claude-plugin/`. `[Official]`

### plugin.json Manifest Schema

The manifest is optional. If omitted, components are auto-discovered in default locations. `[Official]`

#### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Unique identifier (kebab-case, no spaces) `[Official]` |

#### Metadata Fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | `string` | Semantic version (e.g., `"2.1.0"`) `[Official]` |
| `description` | `string` | Brief plugin description `[Official]` |
| `author` | `object` | `{name, email, url}` `[Official]` |
| `homepage` | `string` | Documentation URL `[Official]` |
| `repository` | `string` | Source code URL `[Official]` |
| `license` | `string` | License identifier `[Official]` |
| `keywords` | `string[]` | Discovery tags `[Official]` |

#### Component Path Fields

| Field | Type | Description |
|-------|------|-------------|
| `commands` | `string \| string[]` | Additional command files/directories `[Official]` |
| `agents` | `string \| string[]` | Additional agent files `[Official]` |
| `skills` | `string \| string[]` | Additional skill directories `[Official]` |
| `hooks` | `string \| string[] \| object` | Hook config paths or inline config `[Official]` |
| `mcpServers` | `string \| string[] \| object` | MCP config paths or inline config `[Official]` |
| `outputStyles` | `string \| string[]` | Output style files/directories `[Official]` |
| `lspServers` | `string \| string[] \| object` | LSP server configurations `[Official]` |

Custom paths **supplement** default directories, they do not replace them. All paths must be relative and start with `./`. `[Official]`

### Installation Scopes

| Scope | Settings file | Use case |
|-------|--------------|----------|
| `user` | `~/.claude/settings.json` | Personal, all projects (default) `[Official]` |
| `project` | `.claude/settings.json` | Team-shared via VCS `[Official]` |
| `local` | `.claude/settings.local.json` | Project-specific, gitignored `[Official]` |
| `managed` | Managed settings | Org-wide (read-only, update only) `[Official]` |

---

## 8. LSP Servers

LSP (Language Server Protocol) servers give Claude real-time code intelligence. `[Official]`

### Configuration Locations

- `.lsp.json` at plugin root `[Official]`
- Inline in `plugin.json` under `lspServers` `[Official]`

### Schema

```json
{
  "<language-name>": {
    "command": "gopls",
    "args": ["serve"],
    "extensionToLanguage": {
      ".go": "go"
    }
  }
}
```

#### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `command` | `string` | LSP binary to execute (must be in PATH) `[Official]` |
| `extensionToLanguage` | `object` | Maps file extensions to language identifiers `[Official]` |

#### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `args` | `string[]` | Command-line arguments `[Official]` |
| `transport` | `string` | `stdio` (default) or `socket` `[Official]` |
| `env` | `object` | Environment variables `[Official]` |
| `initializationOptions` | `object` | Server initialization options `[Official]` |
| `settings` | `object` | Settings via `workspace/didChangeConfiguration` `[Official]` |
| `workspaceFolder` | `string` | Workspace folder path `[Official]` |
| `startupTimeout` | `integer` | Max startup wait (ms) `[Official]` |
| `shutdownTimeout` | `integer` | Max shutdown wait (ms) `[Official]` |
| `restartOnCrash` | `boolean` | Auto-restart on crash `[Official]` |
| `maxRestarts` | `integer` | Max restart attempts `[Official]` |

---

## 9. Output Styles

Output styles adjust Claude's system prompt to control response formatting. `[Official]`

### Configuration

Set via `outputStyle` in `settings.json` or via `/config` command. `[Official]`

```json
{
  "outputStyle": "Explanatory"
}
```

Plugins can provide custom output styles in an `outputStyles` directory. `[Official]`

Details on the file format for custom output styles are not extensively documented publicly beyond the plugin component path reference. `[Unverified]`

---

## 10. Settings

Settings files configure Claude Code's behavior, permissions, hooks, and more. `[Official]`

### File Locations and Precedence (Highest to Lowest)

1. **Managed settings** (server-managed > MDM/OS-level > `managed-settings.json` > HKCU registry) `[Official]`
2. **Command line arguments** `[Official]`
3. **Local project settings** (`.claude/settings.local.json`) `[Official]`
4. **Shared project settings** (`.claude/settings.json`) `[Official]`
5. **User settings** (`~/.claude/settings.json`) `[Official]`

### File Format

JSON with optional `$schema` for editor validation. `[Official]`

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": { ... },
  "hooks": { ... },
  "env": { ... }
}
```

### Key Settings Fields

| Key | Type | Description |
|-----|------|-------------|
| `permissions` | `object` | Permission rules (`allow`, `ask`, `deny` arrays, `defaultMode`, `additionalDirectories`, `disableBypassPermissionsMode`) `[Official]` |
| `hooks` | `object` | Lifecycle hook configurations `[Official]` |
| `env` | `object` | Environment variables for every session `[Official]` |
| `model` | `string` | Override default model `[Official]` |
| `availableModels` | `string[]` | Restrict selectable models `[Official]` |
| `effortLevel` | `string` | Persist effort level (`low`, `medium`, `high`) `[Official]` |
| `agent` | `string` | Run main thread as named subagent `[Official]` |
| `outputStyle` | `string` | System prompt adjustment for response format `[Official]` |
| `language` | `string` | Preferred response language `[Official]` |
| `disableAllHooks` | `boolean` | Disable all hooks and custom status line `[Official]` |
| `autoMemoryEnabled` | `boolean` | Toggle auto memory (default: `true`) `[Official]` |
| `autoMemoryDirectory` | `string` | Custom auto memory storage location `[Official]` |
| `claudeMdExcludes` | `string[]` | Glob patterns to skip specific CLAUDE.md files `[Official]` |
| `cleanupPeriodDays` | `integer` | Session cleanup threshold (default: 30) `[Official]` |
| `statusLine` | `object` | Custom status line configuration `[Official]` |
| `fileSuggestion` | `object` | Custom `@` file autocomplete script `[Official]` |
| `respectGitignore` | `boolean` | File picker respects .gitignore (default: `true`) `[Official]` |
| `sandbox` | `object` | Sandbox configuration (see docs) `[Official]` |
| `attribution` | `object` | Git commit/PR attribution customization `[Official]` |
| `spinnerVerbs` | `object` | Customize spinner action verbs `[Official]` |
| `worktree` | `object` | Worktree configuration (`symlinkDirectories`, `sparsePaths`) `[Official]` |
| `companyAnnouncements` | `string[]` | Startup announcements `[Official]` |
| `enableAllProjectMcpServers` | `boolean` | Auto-approve all project MCP servers `[Official]` |
| `plansDirectory` | `string` | Custom plan file storage location `[Official]` |

### Other Configuration Files

| File | Location | Purpose |
|------|----------|---------|
| `~/.claude.json` | Home directory | Preferences, OAuth, MCP server configs (user/local scope), per-project state, caches `[Official]` |
| `.mcp.json` | Project root | Project-scoped MCP server definitions (see [MCP Servers](#4-mcp-servers)) `[Official]` |
| `managed-settings.json` | System directories | Organization-wide enforced settings `[Official]` |
| `managed-mcp.json` | System directories | Organization-wide MCP server definitions `[Official]` |

---

## Content Type Summary

| Content Type | File Format | Entrypoint | Frontmatter | Location(s) | Loading |
|-------------|-------------|-----------|-------------|-------------|---------|
| **Rules (CLAUDE.md)** | Markdown | `CLAUDE.md` | None | Project root, `.claude/`, `~/`, managed | Every session (additive) |
| **Rules (.claude/rules/)** | Markdown | `*.md` (recursive) | Optional `paths` | `.claude/rules/`, `~/.claude/rules/` | Session start or on file match |
| **Skills** | Markdown | `SKILL.md` in directory | Yes (11 fields) | `.claude/skills/`, `~/.claude/skills/`, plugin | Descriptions at start; full on invoke |
| **Agents** | Markdown | `<name>.md` file | Yes (14 fields) | `.claude/agents/`, `~/.claude/agents/`, plugin, CLI | Session start |
| **MCP Servers** | JSON | `.mcp.json` or `settings.json` | N/A | `.mcp.json`, `~/.claude.json`, managed, plugin, agent | Session start |
| **Hooks** | JSON | `settings.json` or `hooks.json` | N/A | `settings.json` (all scopes), plugin, skill/agent | On event trigger |
| **Commands** | Markdown | `<name>.md` file | Optional (same as skills) | `.claude/commands/`, `~/.claude/commands/` | Same as skills (legacy) |
| **Plugins** | JSON + mixed | `.claude-plugin/plugin.json` | N/A | Marketplace or `--plugin-dir` | Session start |
| **LSP Servers** | JSON | `.lsp.json` or plugin.json | N/A | Plugin root | Session start |
| **Output Styles** | Unknown | Plugin `outputStyles/` dir | Unknown | Plugin | Session start |
| **Settings** | JSON | `settings.json` | N/A | `~/.claude/`, `.claude/`, managed | Session start |

---

## Auto Memory

Auto memory is a separate system from CLAUDE.md -- notes Claude writes for itself. Included here for completeness. `[Official]`

**Storage:** `~/.claude/projects/<project>/memory/MEMORY.md` (entrypoint) plus optional topic files. `[Official]`

**Loading:** First 200 lines of `MEMORY.md` loaded at session start. Topic files loaded on demand. `[Official]`

**Scope:** Per git repository (all worktrees share one directory). `[Official]`

**Configuration:**
- `autoMemoryEnabled` setting (default: `true`) `[Official]`
- `autoMemoryDirectory` setting for custom location `[Official]`
- `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` env var to disable `[Official]`

---

## Agent Skills Open Standard

Claude Code skills follow the [Agent Skills](https://agentskills.io) open standard, which works across multiple AI tools. Claude Code extends the standard with additional features like invocation control (`disable-model-invocation`, `user-invocable`), subagent execution (`context: fork`, `agent`), and dynamic context injection (`` !`command` ``). `[Official]`
