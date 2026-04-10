# Claude Code — Format Reference

Provider slug: `claude-code`

Supports: Rules, Commands, Skills, Agents, Hooks, MCP

Sources: [Anthropic docs](https://docs.anthropic.com/en/docs/claude-code), [Claude Code GitHub](https://github.com/anthropics/claude-code)

---

## Rules

**Location:** `CLAUDE.md` (project root) or `.claude/rules/*.md`

**Format:** Markdown with optional YAML frontmatter

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | No | When this rule should apply (used for auto-activation) |
| `alwaysApply` | bool | No | `true` = always active, `false` = conditional |
| `globs` | string[] | No | File patterns for scoped activation |

**Activation model:** Claude Code's `.claude/rules/` directory supports per-file rules with frontmatter. The root `CLAUDE.md` is always-active (no frontmatter). Rules without frontmatter default to always-apply.

**Example:**

```markdown
---
description: TypeScript conventions for API routes
alwaysApply: false
globs:
  - "src/api/**/*.ts"
---

Use Zod for input validation on all API routes.
Always return typed error responses.
```

---

## Commands

**Location:** `.claude/commands/<name>.md` or `.claude/commands/<name>/command.md`

**Format:** YAML frontmatter + markdown body

**Naming:** Filename or directory name becomes the `/command` name. Subdirectories create namespaced commands (e.g., `.claude/commands/git/commit.md` -> `/git:commit`).

**Frontmatter fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | No | filename | Command identifier (lowercase, numbers, hyphens) |
| `description` | string | Recommended | — | What the command does; drives auto-invocation |
| `argument-hint` | string | No | — | Autocomplete hint, e.g. `[file-pattern]` |
| `allowed-tools` | string | No | — | Comma-separated tool allowlist: `"Read, Grep, Glob"` |
| `disable-model-invocation` | bool | No | `false` | If `true`, only user can invoke (not Claude) |
| `user-invocable` | bool | No | `true` | Show in `/` menu |
| `model` | string | No | inherit | Model override: `sonnet`, `opus`, `haiku` |
| `context` | string | No | — | `"fork"` = run in isolated subagent |
| `agent` | string | No | — | Agent type for forked context: `Bash`, `Explore`, `Plan`, `general-purpose` |

**Body:** Markdown prompt/instructions sent to Claude when command is invoked.

**String substitutions in body:**
- `$ARGUMENTS` — all arguments passed by user
- `$ARGUMENTS[N]` or `$N` — specific argument (0-indexed)
- `${CLAUDE_SESSION_ID}` — current session ID

**Example:**

```markdown
---
description: Review code changes for quality and security
argument-hint: "[target]"
allowed-tools: "Read, Grep, Glob, Bash"
context: fork
agent: Explore
---

Review the code changes specified by the user. Focus on:
1. Correctness and logic errors
2. Security vulnerabilities
3. Performance issues
4. Test coverage gaps

Target: $ARGUMENTS
```

---

## Skills

**Location:** `.claude/skills/<name>/SKILL.md` (project) or `~/.claude/skills/<name>/SKILL.md` (user)

**Format:** YAML frontmatter + markdown body

**Frontmatter fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | No | directory name | Skill identifier |
| `description` | string | Recommended | — | When/why to use this skill; drives auto-invocation |
| `argument-hint` | string | No | — | CLI autocomplete hint |
| `allowed-tools` | string | No | — | Comma-separated tool allowlist |
| `disallowedTools` | string | No | — | Comma-separated tool denylist |
| `disable-model-invocation` | bool | No | `false` | Prevent Claude from auto-invoking |
| `user-invocable` | bool | No | `true` | Show in `/` menu |
| `model` | string | No | inherit | Model override |
| `context` | string | No | — | `"fork"` = run in subagent |
| `agent` | string | No | — | Agent type for fork context |

**Body:** Markdown instructions Claude follows when skill is active.

**Directory structure:**

```
my-skill/
  SKILL.md          # Required: frontmatter + instructions
  scripts/          # Optional: helper scripts
  references/       # Optional: reference docs
  examples/         # Optional: example outputs
```

**Example:**

```markdown
---
name: code-review
description: Systematic code review workflow. Use when reviewing PRs or code changes.
allowed-tools: "Read, Grep, Glob, Bash"
---

Follow this review checklist:
1. Correctness — does it work?
2. Security — any vulnerabilities?
3. Performance — obvious inefficiencies?
4. Maintainability — will future devs understand this?
```

---

## Agents

**Location:** `.claude/agents/<name>.md` or `.claude/agents/<name>/AGENT.md` (project); `~/.claude/agents/` (user)

**Format:** YAML frontmatter + markdown body

**Frontmatter fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | — | Unique identifier (lowercase, hyphens) |
| `description` | string | Yes | — | When to delegate to this agent |
| `tools` | string/list | No | all | Tool allowlist: `"Read, Grep, Bash"` or `["Read", "Grep"]` |
| `disallowedTools` | string/list | No | — | Tool denylist |
| `model` | string | No | inherit | `sonnet`, `opus`, `haiku`, or `inherit` |
| `permissionMode` | string | No | `"default"` | `default`, `acceptEdits`, `dontAsk`, `bypassPermissions`, `plan` |
| `maxTurns` | int | No | 20 | Max agentic turns before stopping |
| `skills` | string[] | No | — | Preloaded skills |
| `mcpServers` | object | No | — | MCP server configs for this agent |
| `memory` | string | No | — | Persistent memory scope: `user`, `project`, `local` |
| `background` | bool | No | `false` | Run as background task |
| `isolation` | string | No | — | `"worktree"` = use git worktree |

**Body:** System prompt defining agent behavior.

**Example:**

```markdown
---
name: security-auditor
description: Specialized security review agent. Delegate when analyzing code for vulnerabilities.
tools: "Read, Grep, Glob, Bash"
model: sonnet
permissionMode: plan
maxTurns: 30
---

You are a security auditor. Analyze code for:
- OWASP Top 10 vulnerabilities
- Injection risks (SQL, XSS, command)
- Authentication/authorization flaws
- Secrets in source code

Report findings with severity ratings.
```

---

## Hooks

**Location:** `.claude/settings.json` (project) or `~/.claude/settings.json` (user), under `"hooks"` key

**Format:** JSON

**Top-level structure:**

```json
{
  "hooks": {
    "<EventName>": [
      {
        "matcher": "<pattern>",
        "hooks": [
          {
            "type": "command|prompt|agent",
            "command": "shell-command",
            "timeout": 600,
            "async": false,
            "statusMessage": "Spinner text"
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
| `PreToolUse` | tool name regex | Yes | Before tool execution |
| `PostToolUse` | tool name regex | No | After tool execution |
| `PostToolUseFailure` | tool name regex | No | After tool failure |
| `PermissionRequest` | tool name regex | Yes | Permission prompt intercept |
| `UserPromptSubmit` | N/A | Yes | After user submits prompt |
| `Stop` | N/A | Yes | Agent loop complete |
| `SessionStart` | session source | No | Session begins (`startup`, `resume`, `clear`, `compact`) |
| `SessionEnd` | exit reason | No | Session ends (`clear`, `logout`, `other`) |
| `Notification` | notification type | No | System alerts |
| `SubagentStart` | agent type | No | Subagent launches |
| `SubagentStop` | agent type | Yes | Subagent completes |
| `PreCompact` | trigger | No | Before context compression (`manual`, `auto`) |
| `TaskCompleted` | N/A | Yes | Background task done |
| `TeammateIdle` | N/A | Yes | Multi-session idle |
| `ConfigChange` | config source | Yes | Settings change |
| `WorktreeCreate` | N/A | Yes | Git worktree created |
| `WorktreeRemove` | N/A | No | Git worktree removed |

**Hook types:**

| Type | Fields | Description |
|------|--------|-------------|
| `command` | `command`, `timeout`, `async`, `statusMessage` | Execute shell command |
| `prompt` | `prompt`, `model`, `timeout` | LLM evaluates prompt |
| `agent` | `prompt`, `model`, `timeout` | Agent evaluates prompt |

**Hook I/O (command type):**

Input via stdin:
```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/project/root",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "npm test" }
}
```

Output via stdout:
```json
{
  "decision": "allow|block",
  "reason": "explanation",
  "continue": true,
  "suppressOutput": false
}
```

**Exit codes:** `0` = allow, `2` = block (stderr = feedback), other = warning (non-blocking)

**Example:**

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./.claude/hooks/validate-bash.sh",
            "timeout": 5000
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "npx prettier --write $(jq -r '.tool_input.file_path')",
            "async": false
          }
        ]
      }
    ]
  }
}
```

---

## MCP Servers

**Location:** `.mcp.json` (project) or `~/.claude.json` (user), under `"mcpServers"` key

**Format:** JSON

**Server entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | No | Transport: `"stdio"`, `"http"`, `"sse"` (default inferred from other fields) |
| `command` | string | Conditional | Executable for stdio transport |
| `args` | string[] | No | Arguments for stdio command |
| `env` | object | No | Environment variables (supports `${VAR:-default}` expansion) |
| `url` | string | Conditional | Endpoint for HTTP/SSE transport |
| `headers` | object | No | HTTP headers (supports env var expansion) |
| `oauth` | object | No | OAuth config: `{ clientId, clientSecret, callbackPort }` |
| `cwd` | string | No | Working directory for stdio |

At least one of `command` or `url` is required.

**Example:**

```json
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      }
    },
    "filesystem": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"],
      "env": { "DEBUG": "true" }
    }
  }
}
```
