# Cursor Content Types Reference

Comprehensive documentation of all content types supported by Cursor, including file formats, directory structures, configuration schemas, and loading behavior.

**Last updated:** 2026-03-21

**Sources:**
- [Official] https://cursor.com/docs/context/rules
- [Official] https://cursor.com/docs/mcp
- [Official] https://cursor.com/docs/hooks
- [Official] https://cursor.com/docs/context/skills
- [Official] https://cursor.com/docs/context/commands
- [Community] https://theodoroskokosioulis.com/blog/cursor-rules-commands-skills-hooks-guide/
- [Community] https://mer.vin/2025/12/cursor-ide-rules-deep-dive/
- [Community] https://www.agentrulegen.com/guides/cursor-rules-guide
- [Community] https://www.truefoundry.com/blog/mcp-servers-in-cursor-setup-configuration-and-security-guide
- [Community] https://skywork.ai/blog/how-to-cursor-1-7-hooks-guide/

---

## Table of Contents

1. [Rules (.cursorrules, .cursor/rules/, AGENTS.md)](#1-rules)
2. [MCP Servers (.cursor/mcp.json)](#2-mcp-servers)
3. [Skills (.cursor/skills/, .agents/skills/)](#3-skills)
4. [Commands (.cursor/commands/)](#4-commands)
5. [Hooks (.cursor/hooks.json)](#5-hooks)
6. [Content Types NOT Supported](#6-content-types-not-supported)

---

## 1. Rules

Rules are persistent instructions that shape how Cursor's agent behaves. They are injected into the model's context at appropriate times based on their activation type. `[Official]`

Cursor supports three rule systems: the legacy `.cursorrules` file, the modern `.cursor/rules/` directory with `.mdc` files, and the cross-provider `AGENTS.md` format.

### 1a. Legacy .cursorrules File

**File format:** Markdown (`.md`-compatible), plain text. No frontmatter. `[Official]`

**Status:** Deprecated. Still functional but superseded by `.cursor/rules/`. `[Official]`

**Location:** Project root — `.cursorrules`

**Behavior:** Contents are injected into every Cursor AI request automatically. Equivalent to a single always-on rule. `[Community]`

**Example:**

```
You are an expert TypeScript developer.
Always use strict mode.
Prefer functional patterns over class-based approaches.
Write tests for all new functions.
```

### 1b. Modern .cursor/rules/ Directory (.mdc files)

**File format:** Markdown with optional YAML frontmatter, using `.mdc` extension. Also supports `.md` extension. `[Official]`

**Purpose:** Granular, composable rules with activation control. Version-controlled, shareable via VCS. `[Official]`

#### Directory Structure

```
PROJECT_ROOT/
├── .cursor/
│   └── rules/
│       ├── code-style.mdc        # Always-on rule
│       ├── testing.mdc           # File-scoped rule
│       ├── frontend/
│       │   └── components.mdc    # Subdirectory organization supported
│       └── manual-review.mdc     # Manual-only rule
```

**Scope locations:**

| Scope | Location |
|-------|----------|
| Project | `.cursor/rules/*.mdc` (or `.md`) `[Official]` |
| Global (User) | Cursor Settings > Rules (UI-managed, not file-based) `[Official]` |
| Team | Cursor Dashboard (Team/Enterprise plans) `[Official]` |

**Note:** Flat `.mdc` files work. The documented folder-per-rule format (`.cursor/rules/my-rule/RULE.md`) does NOT work as of early 2026. `[Community]` https://forum.cursor.com/t/project-rules-documented-rule-md-folder-format-not-working-only-undocumented-mdc-format-works/145907

#### YAML Frontmatter Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `description` | string | Recommended | `""` | Explains when the agent should apply this rule. Used by the agent to decide relevance in "Agent Requested" mode. `[Official]` |
| `globs` | string or string[] | For file-scoped rules | none | Minimatch/glob patterns determining when the rule auto-attaches. Multiple patterns supported (e.g., `"*.ts,*.tsx"` or `["**/*.ts", "src/components/**"]`). `[Official]` |
| `alwaysApply` | boolean | No | `false` | If `true`, rule is included in every chat session regardless of globs. `[Official]` |

#### Rule Activation Types

Controlled by the combination of frontmatter fields: `[Official]`

| Type | `alwaysApply` | `globs` | `description` | Behavior |
|------|---------------|---------|---------------|----------|
| **Always** | `true` | — | Optional | Included in every session |
| **Auto-Attached** | `false` | Set | Optional | Activates when open/referenced files match globs |
| **Agent-Requested** | `false` | — | Required | Agent decides to include based on description |
| **Manual** | `false` | — | — | User must invoke via `@rule-name` in chat |

#### Example .mdc File

```markdown
---
description: "Standards for TypeScript code in this project"
globs: ["*.ts", "*.tsx"]
alwaysApply: false
---

# TypeScript Standards

- Use strict mode in all files
- Prefer `const` over `let`
- Use explicit return types on exported functions
- No `any` — use `unknown` and narrow
```

#### Rule Precedence

When multiple rules conflict: **Team Rules > Project Rules > User Rules** `[Official]`

#### Best Practices

- Keep under 500 lines per rule `[Official]`
- One concern per rule file `[Community]`
- Reference files with `@filename.ts` instead of copying content `[Official]`
- Split large specs into multiple composable `.mdc` files `[Community]`

### 1c. AGENTS.md

**File format:** Standard Markdown. No frontmatter, no special schema. `[Official]`

**Purpose:** Cross-provider agent instructions. Simpler alternative to `.cursor/rules/` for projects that need straightforward instructions. `[Official]`

**Location:** Project root and/or any subdirectory. The closest `AGENTS.md` to the edited file takes precedence. `[Official]`

**Behavior:** Cursor reads `AGENTS.md` files and applies them based on directory proximity. Explicit user chat prompts override everything. `[Official]`

**Supported by:** Claude Code, Cursor, GitHub Copilot, Gemini CLI, Windsurf, Aider, Zed, Warp, RooCode, and others. `[Community]`

#### Example

```markdown
# Project Guidelines

## Code Style
- Use TypeScript strict mode
- Prefer functional patterns

## Testing
- Write Vitest tests for all new functions
- Maintain >80% coverage
```

### 1d. User Rules

**Location:** Cursor Settings > Rules (UI only, not a file on disk) `[Official]`

**Scope:** Apply across all projects, to Agent (Chat) mode only. Do NOT apply to Inline Edit. `[Official]`

**Format:** Plain text instructions entered in the settings UI.

---

## 2. MCP Servers

MCP (Model Context Protocol) extends Cursor's agent with external tools and data sources. Configuration is JSON-based. `[Official]`

### File Locations

| Scope | Location |
|-------|----------|
| Project | `.cursor/mcp.json` `[Official]` |
| Global (User) | `~/.cursor/mcp.json` `[Official]` |

### JSON Schema

**Encoding:** JSON (no comments, no trailing commas)

**Root structure:**

```json
{
  "mcpServers": {
    "<server-name>": { ... }
  }
}
```

The `mcpServers` key is required. If missing, Cursor silently ignores the entire file. `[Community]`

### Server Configuration Fields

#### STDIO Transport (Local Servers)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Yes | Executable name or full path `[Official]` |
| `args` | string[] | No | Command-line arguments `[Official]` |
| `env` | object | No | Environment variables as key-value pairs `[Official]` |
| `envFile` | string | No | Path to `.env` file (STDIO only) `[Official]` |
| `type` | string | No | `"stdio"` (default for command-based configs) `[Official]` |

**Example:**

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

#### Remote Transport (HTTP/SSE)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | Endpoint URL for the MCP server `[Official]` |
| `headers` | object | No | Custom HTTP headers `[Official]` |
| `auth` | object | No | OAuth configuration (see below) `[Official]` |

**Example:**

```json
{
  "mcpServers": {
    "remote-tools": {
      "url": "https://mcp.example.com/sse",
      "headers": {
        "Authorization": "Bearer ${env:MCP_TOKEN}"
      }
    }
  }
}
```

#### OAuth Authentication Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `CLIENT_ID` | string | Yes | OAuth 2.0 client identifier `[Official]` |
| `CLIENT_SECRET` | string | No | For confidential clients `[Official]` |
| `scopes` | string[] | No | OAuth scopes to request `[Official]` |

OAuth redirect URL: `cursor://anysphere.cursor-mcp/oauth/callback` `[Official]`

### Variable Interpolation

Supported in `command`, `args`, `env`, `url`, and `headers` fields: `[Official]`

| Variable | Description |
|----------|-------------|
| `${env:NAME}` | Environment variable value |
| `${userHome}` | User home directory |
| `${workspaceFolder}` | Project root path |
| `${workspaceFolderBasename}` | Project folder name |
| `${pathSeparator}` or `${/}` | OS-specific path separator |

### Transport Types Supported

1. **STDIO** — Local processes, communicates via stdin/stdout `[Official]`
2. **SSE** (Server-Sent Events) — Remote servers, legacy protocol `[Official]`
3. **Streamable HTTP** — Remote servers, current standard `[Official]`

---

## 3. Skills

Skills are portable, version-controlled packages that extend agents with specialized capabilities. They package domain-specific knowledge and workflows into reusable components. `[Official]`

### Directory Structure

Skills are automatically discovered from these locations: `[Official]`

| Location | Scope |
|----------|-------|
| `.agents/skills/` | Project-level |
| `.cursor/skills/` | Project-level |
| `~/.cursor/skills/` | User-level (global) |

Legacy compatibility directories also scanned: `.claude/skills/`, `.codex/skills/`, `~/.claude/skills/`, `~/.codex/skills/` `[Official]`

#### Skill Directory Layout

```
.cursor/skills/
└── deploy-app/
    ├── SKILL.md          # Required — skill definition
    ├── scripts/          # Optional — executable code
    │   └── deploy.sh
    ├── references/       # Optional — additional docs loaded on demand
    │   └── api-spec.md
    └── assets/           # Optional — templates, data files
        └── template.yaml
```

### SKILL.md Format

**File format:** Markdown with YAML frontmatter. `[Official]`

**Required file:** `SKILL.md` (must be named exactly this). `[Official]`

#### Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Skill identifier. Lowercase, numbers, hyphens only. Must match folder name. `[Official]` |
| `description` | string | Yes | What the skill does and when to use it. Helps agent determine relevance. `[Official]` |
| `license` | string | No | License name or reference to bundled license file `[Official]` |
| `compatibility` | string | No | Environment requirements (system packages, network access) `[Official]` |
| `metadata` | object | No | Arbitrary key-value pairs for additional information `[Official]` |
| `disable-model-invocation` | boolean | No | When `true`, skill acts as explicit slash command only (no auto-discovery) `[Official]` |

#### Example SKILL.md

```markdown
---
name: deploy-app
description: Deploys the application to staging or production environments.
---

# Deploy Application

## When to Use
- When deploying to staging or production
- After all tests pass on main branch

## Instructions
1. Run the test suite: `npm test`
2. Build the production bundle: `npm run build`
3. Execute deployment: `./scripts/deploy.sh <environment>`

## Configuration
- Staging: auto-deploys on merge to `develop`
- Production: requires manual approval
```

### How Skills Work

- **Automatic Discovery:** On startup, Cursor scans skill directories and makes skills available based on contextual relevance. `[Official]`
- **Progressive Loading:** Only the `name` and `description` are read initially. Full content loads on demand. `[Official]`
- **Manual Invocation:** Users can trigger skills via `/skill-name` in Agent chat. `[Official]`
- **Auto-Invocation:** Unless `disable-model-invocation: true`, the agent can invoke skills autonomously when relevant. `[Official]`

### Optional Directories

| Directory | Purpose |
|-----------|---------|
| `scripts/` | Executable code (Bash, Python, JS) the agent can run `[Official]` |
| `references/` | Additional documentation loaded on demand `[Official]` |
| `assets/` | Static resources: templates, images, data files `[Official]` |

### Migration

Use `/migrate-to-skills` command to convert existing dynamic rules and slash commands to skills format. `[Official]`

---

## 4. Commands

Commands are reusable AI prompts saved as Markdown files, invoked with `/` in chat. `[Official]`

### File Locations

| Scope | Location |
|-------|----------|
| Project | `.cursor/commands/*.md` `[Official]` |
| Global (User) | `~/.cursor/commands/*.md` `[Community]` |

### File Format

**Encoding:** Markdown (`.md`), UTF-8.

**Naming:** The filename (minus `.md`) becomes the command name. Use descriptive, hyphenated names (e.g., `code-review.md`, `fix-lint.md`). `[Community]`

**Frontmatter:** None documented. Commands are pure Markdown content. `[Community]`

**Invocation:** Type `/` in Agent chat input, then select from the dropdown. Additional text typed after the command name is passed as context. `[Official]`

### Example Command

File: `.cursor/commands/code-review.md`

```markdown
# Code Review

Review the current changes with the following criteria:

## Requirements
- Check for security vulnerabilities
- Verify error handling is comprehensive
- Ensure naming conventions are followed
- Look for performance issues

## Output
Provide a structured review with:
1. Critical issues (must fix)
2. Suggestions (nice to have)
3. Positive observations
```

### Commands vs Skills

Commands are being superseded by Skills. The `/migrate-to-skills` command converts existing commands to the skill format. Key differences: `[Official]`

| Aspect | Commands | Skills |
|--------|----------|--------|
| Format | Single `.md` file | Directory with `SKILL.md` + optional subdirs |
| Invocation | Always explicit (`/name`) | Explicit or agent-auto-discovered |
| Supporting files | None | `scripts/`, `references/`, `assets/` |
| Frontmatter | None | YAML with `name`, `description`, etc. |

---

## 5. Hooks

Hooks let you observe, control, and extend the agent loop using custom scripts. They run before or after specific events and can block, modify, or audit agent behavior. `[Official]`

### File Locations

| Scope | Location | Working directory |
|-------|----------|-------------------|
| Project | `.cursor/hooks.json` | Project root `[Official]` |
| Global (User) | `~/.cursor/hooks.json` | `~/.cursor/` `[Official]` |
| Enterprise | System directories (platform-specific) | — `[Official]` |
| Team | Cloud-distributed (Enterprise only) | — `[Official]` |

**Priority order (highest to lowest):** Enterprise > Team > Project > User `[Official]`

### JSON Schema

```json
{
  "version": 1,
  "hooks": {
    "<event-name>": [
      {
        "command": "./scripts/my-hook.sh",
        "type": "command",
        "timeout": 30,
        "failClosed": false,
        "loop_limit": 5,
        "matcher": "pattern"
      }
    ]
  }
}
```

#### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | number | Yes | Schema version. Currently `1`. `[Official]` |
| `hooks` | object | Yes | Map of event names to arrays of hook definitions. `[Official]` |

#### Hook Definition Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `command` | string | Yes | — | Script path or shell command to execute `[Official]` |
| `type` | `"command"` or `"prompt"` | No | `"command"` | Execution method `[Official]` |
| `timeout` | number | No | Platform default | Max seconds to execute `[Official]` |
| `failClosed` | boolean | No | `false` | If `true`, block the action when the hook fails (instead of allowing it) `[Official]` |
| `loop_limit` | number or null | No | `5` | Max auto follow-ups for `stop` hooks. `null` = unlimited. `[Official]` |
| `matcher` | string | No | — | Filter pattern — what the hook matches against depends on the event type `[Official]` |

#### Prompt-Based Hook Fields (when `type: "prompt"`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | Yes | Natural language condition for the LLM to evaluate `[Official]` |
| `model` | string | No | Model override for evaluation `[Official]` |

### Supported Event Types

#### Agent Events

| Event | Phase | Matcher matches on | Description |
|-------|-------|-------------------|-------------|
| `sessionStart` | Lifecycle | — | Session begins `[Official]` |
| `sessionEnd` | Lifecycle | — | Session ends `[Official]` |
| `stop` | Lifecycle | `"Stop"` | Agent stops; can trigger follow-up `[Official]` |
| `preToolUse` | Tool | Tool type (e.g., `"Shell"`, `"Read"`, `"Write"`, `"MCP:toolname"`) | Before any tool call `[Official]` |
| `postToolUse` | Tool | Tool type | After successful tool call `[Official]` |
| `postToolUseFailure` | Tool | Tool type | After failed tool call `[Official]` |
| `subagentStart` | Subagent | Subagent type (e.g., `"generalPurpose"`, `"explore"`, `"shell"`) | Before subagent spawns `[Official]` |
| `subagentStop` | Subagent | Subagent type | After subagent completes `[Official]` |
| `beforeShellExecution` | Shell | Command pattern (e.g., `"curl\|wget\|nc"`) | Before shell command runs `[Official]` |
| `afterShellExecution` | Shell | Command pattern | After shell command completes `[Official]` |
| `beforeMCPExecution` | MCP | MCP tool name | Before MCP tool call `[Official]` |
| `afterMCPExecution` | MCP | MCP tool name | After MCP tool call `[Official]` |
| `beforeReadFile` | File | Tool type | Before file read `[Official]` |
| `afterFileEdit` | File | Tool type (e.g., `"Write"`) | After file edit `[Official]` |
| `preCompact` | Context | — | Before context compaction `[Official]` |
| `beforeSubmitPrompt` | Context | `"UserPromptSubmit"` | Before user prompt is sent `[Official]` |
| `afterAgentResponse` | Reasoning | Response type | After agent produces a response `[Official]` |
| `afterAgentThought` | Reasoning | — | After agent internal reasoning `[Official]` |

#### Tab Events (Inline Completions)

| Event | Description |
|-------|-------------|
| `beforeTabFileRead` | Control file access for Tab completions `[Official]` |
| `afterTabFileEdit` | Post-process inline Tab edits `[Official]` |

**Note:** Cursor CLI (`cursor-agent`) only supports `beforeShellExecution` and `afterShellExecution`. All other events are GUI-only. `[Community]` https://forum.cursor.com/t/cursor-cli-doesnt-send-all-events-defined-in-hooks/148316

### Hook Execution Model

#### Command-Based Hooks

- Scripts receive JSON input via **stdin** `[Official]`
- Scripts output JSON to **stdout** `[Official]`
- **Exit code 0:** Success, use JSON output `[Official]`
- **Exit code 2:** Block the action (deny permission) `[Official]`
- **Other exit codes:** Hook failed; action proceeds (fail-open) unless `failClosed: true` `[Official]`

#### Input JSON (All Hooks)

Every hook receives these base fields via stdin: `[Official]`

```json
{
  "conversation_id": "stable-id-across-turns",
  "generation_id": "changes-per-user-message",
  "model": "claude-sonnet-4-20250514",
  "hook_event_name": "beforeShellExecution",
  "cursor_version": "1.7.2",
  "workspace_roots": ["/path/to/project"],
  "user_email": "user@example.com",
  "transcript_path": "/path/to/transcript.txt"
}
```

#### Output JSON (Command Hooks)

```json
{
  "permission": "allow|deny|ask",
  "user_message": "Shown in UI when denied",
  "agent_message": "Sent to agent when denied",
  "continue": true
}
```

#### Output JSON (sessionStart)

```json
{
  "env": { "KEY": "value" },
  "additional_context": "Extra context for the session"
}
```

Session-scoped env vars from `sessionStart` propagate to all subsequent hooks. `[Official]`

#### Output JSON (stop)

```json
{
  "followup_message": "Next task to perform..."
}
```

Respects `loop_limit` to prevent infinite loops. `[Official]`

### Environment Variables

Available to all hook scripts: `[Official]`

| Variable | Description |
|----------|-------------|
| `CURSOR_PROJECT_DIR` | Workspace root |
| `CURSOR_VERSION` | Cursor version string |
| `CURSOR_USER_EMAIL` | User email (if logged in) |
| `CURSOR_TRANSCRIPT_PATH` | Transcript file path |
| `CURSOR_CODE_REMOTE` | `"true"` in remote workspaces |
| `CLAUDE_PROJECT_DIR` | Alias for `CURSOR_PROJECT_DIR` |

### Example: Format After Edit

```json
{
  "version": 1,
  "hooks": {
    "afterFileEdit": [
      {
        "command": "./hooks/format.sh",
        "matcher": "Write",
        "timeout": 10
      }
    ]
  }
}
```

### Example: Block Dangerous Commands

```json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "command": "./hooks/approve-network.sh",
        "matcher": "curl|wget|nc",
        "timeout": 30,
        "failClosed": true
      }
    ]
  }
}
```

---

## 6. Content Types NOT Supported

The following content types have **no equivalent** in Cursor:

| Content Type | Notes |
|-------------|-------|
| **Agents (subagent definitions)** | Cursor has built-in multi-agent support (up to 8 parallel agents in Cursor 2.0) but does NOT support user-defined agent configuration files. No equivalent to Claude Code's `.claude/agents/*.md`. `[Official]` |
| **Prompts / Prompt templates** | No dedicated prompt template system beyond Commands and Skills. `[Inferred]` |

---

## Summary: Content Type Locations

| Content Type | Project Location | Global Location | File Format |
|-------------|-----------------|-----------------|-------------|
| Rules (legacy) | `.cursorrules` | — | Markdown (no frontmatter) |
| Rules (modern) | `.cursor/rules/*.mdc` | Settings UI | Markdown + YAML frontmatter |
| Rules (cross-provider) | `AGENTS.md` (root or subdirs) | — | Markdown (no frontmatter) |
| MCP servers | `.cursor/mcp.json` | `~/.cursor/mcp.json` | JSON |
| Skills | `.cursor/skills/*/SKILL.md` or `.agents/skills/*/SKILL.md` | `~/.cursor/skills/*/SKILL.md` | Markdown + YAML frontmatter |
| Commands | `.cursor/commands/*.md` | `~/.cursor/commands/*.md` | Markdown (no frontmatter) |
| Hooks | `.cursor/hooks.json` | `~/.cursor/hooks.json` | JSON |
