# Gemini CLI — Format Reference

Provider slug: `gemini-cli`

Supports: Rules, Commands, Skills, Agents, Hooks, MCP

Sources: [geminicli.com](https://geminicli.com/docs/), [GitHub](https://github.com/google-gemini/gemini-cli)

---

## Rules

**Location:** `GEMINI.md` (project root)

**Format:** Plain markdown (no frontmatter)

**Schema:** Single-file, no structured metadata. All content is always-active. No support for conditional activation, globs, or per-rule descriptions.

**Example:**

```markdown
# Project Conventions

Use TypeScript strict mode in all files.
Run `npm run lint` before committing.

# Testing

Write tests for all new features.
Use Vitest for unit tests.
```

**Converter notes:** `alwaysApply: true` rules export as-is. Non-always rules get their activation scope embedded as actionable prose (e.g., "Apply only when working with files matching: *.ts").

---

## Commands

**Location:** `.gemini/commands/<name>.toml` (project or `~/.gemini/commands/` for user)

**Format:** TOML

**Naming:** Filename becomes command name. Subdirectories create namespaced commands with colon separators: `.gemini/commands/git/commit.toml` -> `/git:commit`

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | Yes | Instructions sent to the model |
| `description` | string | No | One-line description shown in `/help` menu |

**Argument handling:**
- `{{args}}` placeholder in prompt: replaced with user-provided text
- Without `{{args}}`: arguments appended after two newlines
- Inside `!{...}` blocks: arguments are shell-escaped

**Special syntax in prompt:**
- `!{shell command}` — execute shell command, inject stdout into prompt
- `@{path/to/file}` — embed file contents into prompt
- `@{directory/}` — embed directory contents recursively (respects `.gitignore`)

**Example:**

```toml
description = "Review code changes for quality and security"

prompt = """
Review the following code changes. Focus on:

1. Correctness and logic errors
2. Security vulnerabilities
3. Performance issues

Changes to review:
!{git diff --staged}

User instructions: {{args}}
"""
```

**Reload:** Run `/commands reload` after creating or modifying TOML files.

---

## Skills

**Location:** `.gemini/skills/<name>/SKILL.md` (project) or `~/.gemini/skills/<name>/SKILL.md` (user)

**Format:** YAML frontmatter + markdown body

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier (must match directory name) |
| `description` | string | Yes | What the skill does; drives auto-activation |

Only two fields are documented. No additional optional fields in the current spec.

**Body:** Markdown instructions guiding agent behavior when skill is active.

**Directory structure:**

```
my-skill/
  SKILL.md          # Required: frontmatter + instructions
  scripts/          # Optional: helper scripts
  references/       # Optional: reference docs
  assets/           # Optional: supporting files
```

**Example:**

```markdown
---
name: code-reviewer
description: Use this skill to review code. Supports local files and remote Pull Requests.
---

# Code Reviewer

Five-phase review process:

1. **Review Target** — Determine what to review
2. **Preparation** — Setup and preflight checks
3. **Analysis** — Evaluate across 7 dimensions
4. **Feedback** — Structured recommendations
5. **Cleanup** — Return to default branch
```

---

## Agents (Subagents)

**Location:** `.gemini/agents/<name>.md` (project) or `~/.gemini/agents/<name>.md` (user)

**Format:** YAML frontmatter + markdown body

**Status:** Experimental. Must be enabled in `settings.json`.

**Frontmatter fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | — | Unique identifier (lowercase, hyphens, underscores) |
| `description` | string | Yes | — | Purpose and expertise areas |
| `kind` | string | No | `"local"` | `"local"` or `"remote"` |
| `tools` | string[] | No | all | Tool allowlist (Gemini tool names: `read_file`, `grep_search`, etc.) |
| `model` | string | No | inherit | Model version (e.g., `gemini-2.5-pro`) |
| `temperature` | float | No | — | Response variability, 0.0-2.0 |
| `max_turns` | int | No | 15 | Max conversation turns |
| `timeout_mins` | int | No | 5 | Max execution time in minutes |

**Body:** System prompt defining agent behavior.

**Management:** `/agents refresh` reloads registry. `/agents enable <name>` and `/agents disable <name>` toggle agents.

**Example:**

```markdown
---
name: security-auditor
description: Specialized in finding security vulnerabilities in code.
kind: local
tools:
  - read_file
  - grep_search
model: gemini-2.5-pro
temperature: 0.2
max_turns: 10
---

You are a security auditor. Focus on:
- OWASP Top 10 vulnerabilities
- Injection risks
- Authentication flaws
- Secrets in source code
```

---

## Hooks

**Location:** `.gemini/settings.json` (project) or `~/.gemini/settings.json` (user), under `"hooks"` key

**Format:** JSON

**Top-level structure:**

```json
{
  "hooks": {
    "<EventName>": [
      {
        "matcher": "<pattern>",
        "sequential": false,
        "hooks": [
          {
            "type": "command",
            "command": "shell-command",
            "name": "friendly-name",
            "timeout": 60000,
            "description": "What this hook does"
          }
        ]
      }
    ]
  }
}
```

**Hook event types:**

| Event | Matcher | Blocking | Description |
|-------|---------|----------|-------------|
| `BeforeTool` | tool name regex | Yes | Before tool execution |
| `AfterTool` | tool name regex | Yes | After tool execution |
| `BeforeAgent` | N/A | Yes | After user prompt, before planning |
| `AfterAgent` | N/A | Yes | After agent completes response |
| `BeforeModel` | N/A | Yes | Before LLM API request |
| `AfterModel` | N/A | Yes | After LLM response chunk |
| `BeforeToolSelection` | N/A | Yes | Before tool decision |
| `SessionStart` | exact string | No | Session begins (`startup`, `resume`, `clear`) |
| `SessionEnd` | exact string | No | Session ends (`exit`, `clear`, `logout`) |
| `Notification` | exact string | No | System alert (`ToolPermission`) |
| `PreCompress` | exact string | No | Before context compression (`auto`, `manual`) |

**Matcher behavior:**
- Tool events (`BeforeTool`, `AfterTool`): regex against tool names (e.g., `"write_file|replace"`, `"read_.*"`)
- Lifecycle events: exact string match
- Wildcards: `"*"` or `""` (empty) matches all

**Hook entry fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | Yes | — | Only `"command"` currently supported |
| `command` | string | Yes | — | Shell command to execute |
| `name` | string | No | — | Friendly identifier for logs |
| `timeout` | int | No | 60000 | Timeout in milliseconds |
| `description` | string | No | — | Brief explanation |

**Hook I/O (command type):**

Input via stdin:
```json
{
  "session_id": "string",
  "transcript_path": "/path/to/transcript.json",
  "cwd": "/working/directory",
  "hook_event_name": "BeforeTool",
  "timestamp": "2026-02-23T15:00:00Z",
  "tool_name": "write_file",
  "tool_input": { "path": "src/main.ts", "content": "..." }
}
```

Output via stdout:
```json
{
  "decision": "allow|deny|block",
  "reason": "explanation",
  "systemMessage": "displayed to user",
  "continue": true,
  "stopReason": "why stopping",
  "suppressOutput": false
}
```

**Exit codes:** `0` = success, `2` = system block (stderr becomes feedback), other = warning

**Environment variables available to hooks:**
- `GEMINI_PROJECT_DIR` — project root
- `GEMINI_SESSION_ID` — current session ID
- `GEMINI_CWD` — current working directory
- `CLAUDE_PROJECT_DIR` — compatibility alias

**Example:**

```json
{
  "hooks": {
    "BeforeTool": [
      {
        "matcher": "write_file|replace",
        "hooks": [
          {
            "name": "security-check",
            "type": "command",
            "command": "$GEMINI_PROJECT_DIR/.gemini/hooks/security.sh",
            "timeout": 5000
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "echo '{\"systemMessage\": \"Session initialized\"}'",
            "name": "init"
          }
        ]
      }
    ]
  }
}
```

---

## MCP Servers

**Location:** `.gemini/settings.json` (project) or `~/.gemini/settings.json` (user), under `"mcpServers"` key

**Format:** JSON

**Server entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Conditional | Executable for stdio transport |
| `args` | string[] | No | Arguments for stdio command |
| `env` | object | No | Environment variables (supports `$VAR` expansion) |
| `cwd` | string | No | Working directory |
| `url` | string | Conditional | SSE endpoint URL |
| `httpUrl` | string | Conditional | HTTP streaming endpoint URL |
| `headers` | object | No | Custom HTTP headers |
| `timeout` | int | No | Request timeout in ms (default: 600000) |
| `trust` | bool | No | Bypass tool confirmation prompts |
| `includeTools` | string[] | No | Tool allowlist by name |
| `excludeTools` | string[] | No | Tool blocklist (takes precedence over include) |
| `disabled` | bool | No | Temporarily disable server |
| `autoApprove` | string[] | No | Tools to auto-approve |
| `authProviderType` | string | No | Auth provider: `dynamic_discovery`, `google_credentials`, `service_account_impersonation` |
| `oauth` | object | No | OAuth config (see below) |
| `targetAudience` | string | No | OAuth Client ID for service account impersonation |
| `targetServiceAccount` | string | No | Google Cloud Service Account email |

At least one of `command`, `url`, or `httpUrl` required. Precedence: `httpUrl` > `url` > `command`.

**OAuth config object:**

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable OAuth |
| `clientId` | string | OAuth client ID |
| `clientSecret` | string | OAuth client secret |
| `authorizationUrl` | string | Authorization endpoint |
| `tokenUrl` | string | Token endpoint |
| `scopes` | string[] | OAuth scopes |
| `redirectUri` | string | Redirect URI |

**Example:**

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_TOKEN": "$GITHUB_TOKEN" },
      "trust": true,
      "includeTools": ["search_repositories", "get_file_contents"]
    },
    "remote-api": {
      "httpUrl": "https://api.example.com/mcp",
      "headers": { "Authorization": "Bearer $API_TOKEN" },
      "timeout": 30000
    }
  }
}
```
