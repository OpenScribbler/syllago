# Copilot CLI — Format Reference

Provider slug: `copilot-cli`

Supports: Rules, Commands, Agents, Hooks, MCP

Sources: [GitHub Copilot Docs](https://docs.github.com/en/copilot), [Custom Agents](https://docs.github.com/en/copilot/reference/custom-agents-configuration)

---

## Rules

**Location:** `.github/copilot-instructions.md` (project root)

**Format:** Plain markdown (no frontmatter)

**Schema:** Single-file, always-active. No conditional activation, globs, or per-rule descriptions.

**Example:**

```markdown
# Project Conventions

Use TypeScript strict mode in all files.
Always write tests for new features.
```

---

## Commands

**Location:** `.copilot/commands/<name>.md` (project) or `~/.copilot/commands/<name>.md` (user)

**Format:** YAML frontmatter + markdown body (Claude Code compatible format)

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | No | One-line description |

**Body:** Markdown instructions sent to the model.

**Example:**

```markdown
---
description: Review staged changes
---

Review the current staged changes and provide feedback on code quality.
```

---

## Agents

**Location:** `.copilot/agents/<name>.md`, `.github/agents/<name>.md`, or `.claude/agents/<name>.md` (compatibility fallback)

**Format:** YAML frontmatter + markdown body

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Display name |
| `description` | string | Yes | Purpose and capabilities |
| `tools` | string[] | No | Tool allowlist (or `["*"]` for all) |
| `disable-model-invocation` | bool | No | Prevent auto-delegation |
| `target` | string | No | `vscode` or `github-copilot` (runtime-only, safe to drop) |
| `mcp-servers` | object | No | Inline MCP server configs |
| `metadata` | object | No | Key-value annotations (safe to drop) |

**Example:**

```markdown
---
description: Security review agent that checks code for vulnerabilities
tools:
  - view
  - glob
  - rg
---

You are a security review agent. Analyze code for:
- OWASP Top 10 vulnerabilities
- Hardcoded secrets
- Injection risks
```

---

## Hooks

**Location:** `.copilot/hooks.json` (project) or `~/.copilot/hooks.json` (user)

**Format:** JSON

**Top-level structure:**

```json
{
  "hooks": {
    "<eventName>": [
      {
        "bash": "shell-command",
        "timeoutSec": 5,
        "comment": "What this hook does"
      }
    ]
  }
}
```

**Hook event types:**

| Event | Description |
|-------|-------------|
| `preToolUse` | Before tool execution |
| `postToolUse` | After tool execution |
| `userPromptSubmitted` | After user prompt, before agent |
| `sessionStart` | Session begins |
| `sessionEnd` | Session ends |

**Hook entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `bash` | string | Yes* | Bash command to execute |
| `powershell` | string | Yes* | PowerShell command (Windows) |
| `timeoutSec` | int | No | Timeout in seconds |
| `comment` | string | No | Description of hook purpose |

*One of `bash` or `powershell` required.

**Key differences from Claude Code/Gemini CLI:**
- No matcher support (hooks apply to all tools in an event)
- Timeout in seconds (not milliseconds)
- `comment` instead of `statusMessage`
- `bash`/`powershell` instead of `command`
- No `type: "prompt"` or `type: "agent"` (LLM-evaluated hooks)

**Example:**

```json
{
  "hooks": {
    "preToolUse": [
      {
        "bash": "echo '{\"decision\": \"allow\"}'",
        "timeoutSec": 5,
        "comment": "Auto-approve all tool use"
      }
    ]
  }
}
```

---

## MCP Servers

**Location:** `.copilot/mcp.json` (project) or `~/.copilot/mcp.json` (user)

**Format:** JSON

**Server entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Conditional | Executable for stdio transport |
| `args` | string[] | No | Arguments for command |
| `env` | object | No | Environment variables |
| `cwd` | string | No | Working directory |
| `url` | string | Conditional | HTTP endpoint URL |
| `headers` | object | No | Custom HTTP headers |
| `type` | string | No | Transport type |

**Tool name format:** `server/tool` (slash-separated, e.g., `github/search_repositories`)

**Example:**

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_TOKEN": "$GITHUB_TOKEN" }
    }
  }
}
```

---

## Tool Name Mapping

| Copilot CLI | Claude Code | Gemini CLI |
|-------------|-------------|------------|
| `view` | `Read` | `read_file` |
| `apply_patch` | `Write` / `Edit` | `write_file` / `replace` |
| `shell` | `Bash` | `run_shell_command` |
| `glob` | `Glob` | `list_directory` |
| `rg` | `Grep` | `grep_search` |
| `task` | `Task` | — |
