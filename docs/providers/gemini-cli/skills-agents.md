# Gemini CLI: Skills, Agents, and Extensions Reference

Comprehensive documentation of Gemini CLI's skill, agent, and extension metadata
systems. Research date: 2026-03-20.

---

## Table of Contents

1. [Extensions (Custom Tools)](#1-extensions-custom-tools)
2. [Subagents](#2-subagents)
3. [Agent Skills](#3-agent-skills)
4. [Custom Commands](#4-custom-commands)
5. [Hooks](#5-hooks)
6. [Policy Engine (Tool Permissions)](#6-policy-engine-tool-permissions)
7. [Custom Tool Discovery](#7-custom-tool-discovery)
8. [Gemini-Specific Concepts](#8-gemini-specific-concepts)

---

## 1. Extensions (Custom Tools)

Extensions are the primary packaging format for extending Gemini CLI. An extension
is a directory containing a `gemini-extension.json` manifest file, plus optional
subdirectories for commands, skills, agents, hooks, policies, and themes.
`[Official]`

**Source:** https://geminicli.com/docs/extensions/reference/

### 1.1 Extension Directory Structure

```
my-extension/
├── gemini-extension.json    (Required — manifest)
├── GEMINI.md                (Optional — context/instructions)
├── commands/                (Optional — custom slash commands)
│   └── group/
│       └── name.toml
├── skills/                  (Optional — agent skills)
│   └── skill-name/
│       └── SKILL.md
├── agents/                  (Optional — subagent definitions)
│   └── agent-name.md
├── hooks/                   (Optional — lifecycle hooks)
│   └── hooks.json
├── policies/                (Optional — tool permission rules)
│   └── rules.toml
└── themes/                  (Optional — UI themes)
```

`[Official]`

### 1.2 gemini-extension.json — Complete Schema

```json
{
  "name": "my-extension",
  "version": "1.0.0",
  "description": "Short description for the gallery",
  "mcpServers": {
    "server-name": {
      "command": "node",
      "args": ["${extensionPath}${/}server.js"],
      "cwd": "${extensionPath}",
      "env": { "KEY": "value" },
      "url": "https://example.com/sse",
      "httpUrl": "https://example.com/mcp",
      "headers": { "Authorization": "Bearer token" },
      "timeout": 30000,
      "description": "Server description",
      "includeTools": ["tool1", "tool2"],
      "excludeTools": ["tool3"]
    }
  },
  "contextFileName": "GEMINI.md",
  "excludeTools": ["run_shell_command"],
  "migratedTo": "https://github.com/new-owner/new-repo",
  "plan": {
    "directory": ".gemini/plans"
  },
  "settings": [
    {
      "name": "API Key",
      "description": "Your API key for the service",
      "envVar": "MY_API_KEY",
      "sensitive": true
    }
  ],
  "themes": [
    {
      "name": "my-theme",
      "type": "custom",
      "background": { "primary": "#1a362a" },
      "text": {
        "primary": "#a6e3a1",
        "secondary": "#6e8e7a",
        "link": "#89e689"
      },
      "status": {
        "success": "#76c076",
        "warning": "#d9e689",
        "error": "#b34e4e"
      },
      "border": { "default": "#4a6c5a" },
      "ui": { "comment": "#6e8e7a" }
    }
  ]
}
```

`[Official]`

### 1.3 Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Lowercase alphanumeric with dashes; must match directory name |
| `version` | string | No | Semantic version (e.g., `"1.0.0"`) |
| `description` | string | No | Short description for gallery display |
| `mcpServers` | object | No | Map of MCP server definitions (see below) |
| `contextFileName` | string | No | Context file to load; defaults to `GEMINI.md` if present |
| `excludeTools` | string[] | No | Tool names to remove from model access |
| `migratedTo` | string | No | URL for relocated extensions; CLI checks this for updates |
| `plan.directory` | string | No | Artifact storage; defaults to `~/.gemini/tmp/<project>/<session>/plans/` |
| `settings` | object[] | No | User-configurable values prompted on install |
| `themes` | object[] | No | Custom color themes |

`[Official]`

### 1.4 MCP Server Configuration Fields

These fields apply to each entry in `mcpServers`:

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable for stdio-based server |
| `args` | string[] | Command arguments |
| `cwd` | string | Working directory |
| `env` | object | Environment variables for server process |
| `url` | string | Server-Sent Events URL (SSE transport) |
| `httpUrl` | string | Streamable HTTP URL |
| `headers` | object | HTTP headers for url/httpUrl requests |
| `timeout` | number | Request timeout in milliseconds |
| `trust` | boolean | Bypass tool confirmation (ignored in extensions for security) |
| `description` | string | Display description |
| `includeTools` | string[] | Allowlist of tool names |
| `excludeTools` | string[] | Blocklist (takes precedence over includeTools) |

**Note:** Avoid underscores in server aliases — use hyphens (e.g., `my-server` not
`my_server`). The parser splits on the first underscore after `mcp_` prefix.
`[Official]`

### 1.5 Extension Settings Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Display label for the setting |
| `description` | string | Yes | Explanation of purpose |
| `envVar` | string | Yes | Environment variable name for storage |
| `sensitive` | boolean | No | When true, stored in system keychain, obfuscated in UI |

Settings are prompted on install and injected as environment variables into MCP
server processes. Manage after install with `gemini extensions config <name>`.
Available since v0.28.0+. `[Official]`

### 1.6 Variable Substitution

Supported in `gemini-extension.json` and `hooks/hooks.json`:

| Variable | Value |
|----------|-------|
| `${extensionPath}` | Absolute path to the extension directory |
| `${workspacePath}` | Absolute path to the workspace root |
| `${/}` | Platform-specific path separator |

`[Official]`

### 1.7 excludeTools Syntax

The `excludeTools` array supports:

- Simple tool name: `"run_shell_command"` — removes tool entirely
- Command-specific: `"run_shell_command(rm -rf)"` — restricts specific arguments

`[Official]`

### 1.8 Extension Security

Extensions cannot:
- Set `trust: true` on MCP servers (ignored by CLI)
- Include `allow` decisions in policies (ignored by CLI)
- Configure `yolo` mode (ignored by CLI)

This prevents extensions from auto-approving tool calls or bypassing security.
`[Official]`

### 1.9 Extension Management Commands

| Command | Description |
|---------|-------------|
| `gemini extensions install <source>` | Install from GitHub URL or local path |
| `gemini extensions uninstall <name...>` | Remove extensions |
| `gemini extensions update <name>` | Update specific extension |
| `gemini extensions update --all` | Update all extensions |
| `gemini extensions disable <name>` | Disable (with `--scope user\|workspace`) |
| `gemini extensions enable <name>` | Enable |
| `gemini extensions new <path> [template]` | Create from template |
| `gemini extensions link <path>` | Symlink for local development |
| `gemini extensions config <name> [setting]` | Manage extension settings |

Install flags: `--ref <ref>`, `--auto-update`, `--pre-release`, `--consent`,
`--skip-settings`. `[Official]`

---

## 2. Subagents

Subagents are specialized agents that operate within the main Gemini CLI session,
each with their own system prompt, tool set, and context window. They are exposed
to the main agent as callable tools. `[Official]`

**Source:** https://geminicli.com/docs/core/subagents/

### 2.1 Agent Definition Format

Agents are defined as Markdown files (`.md`) with YAML frontmatter. The markdown
body becomes the agent's system prompt.

```markdown
---
name: security-auditor
description: Finds security vulnerabilities in code
kind: local
tools:
  - read_file
  - grep_search
model: gemini-3-flash-preview
temperature: 0.2
max_turns: 10
timeout_mins: 5
---

You are a security auditor. When given code, analyze it for:
- OWASP Top 10 vulnerabilities
- Hardcoded secrets and credentials
- SQL injection vectors
- XSS vulnerabilities

Report findings with severity ratings.
```

`[Official]`

### 2.2 File Locations

| Location | Scope | Path |
|----------|-------|------|
| Project-level | Shared with team (version controlled) | `.gemini/agents/*.md` |
| User-level | Personal agents | `~/.gemini/agents/*.md` |
| Extensions | Bundled in extension | `<extension>/agents/*.md` |

`[Official]`

### 2.3 YAML Frontmatter Fields — Local Agents

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | — | Unique ID: lowercase letters, numbers, hyphens, underscores |
| `description` | string | Yes | — | Brief explanation of purpose (shown to main agent) |
| `kind` | string | No | `"local"` | `"local"` or `"remote"` |
| `tools` | string[] | No | All parent tools | List of accessible tool names |
| `model` | string | No | Parent model | Specific model ID (e.g., `gemini-3-flash-preview`) |
| `temperature` | number | No | `1` | Sampling temperature, range 0.0–2.0 |
| `max_turns` | number | No | `30` | Maximum conversation turns |
| `timeout_mins` | number | No | `10` | Execution timeout in minutes |

`[Official]`

### 2.4 Tool Permission Syntax

Subagent `tools` arrays support wildcards:

| Pattern | Matches |
|---------|---------|
| `*` | All available tools |
| `mcp_*` | All MCP server tools |
| `mcp_server-name_*` | All tools from a specific MCP server |
| `read_file` | Specific tool by exact name |

**Constraint:** Subagents cannot invoke other subagents, even with wildcard
permissions. `[Official]`

### 2.5 Remote Agents (A2A Protocol)

Remote agents connect to external services via the Agent-to-Agent (A2A) protocol.

```markdown
---
kind: remote
name: my-remote-agent
agent_card_url: https://example.com/agent-card
auth:
  type: apiKey
  key: $MY_API_KEY
  name: X-API-Key
---
```

**Source:** https://geminicli.com/docs/core/remote-agents/

#### Remote Agent Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `kind` | string | Yes | Must be `"remote"` |
| `name` | string | Yes | Valid slug identifier |
| `agent_card_url` | string | Yes | A2A agent card endpoint URL |
| `auth` | object | No | Authentication configuration |

`[Official]`

#### Authentication Methods

**API Key:**
```yaml
auth:
  type: apiKey
  key: $MY_API_KEY      # env var reference
  name: X-API-Key       # optional header name
```

**HTTP Bearer:**
```yaml
auth:
  type: http
  scheme: Bearer
  token: $MY_TOKEN
```

**HTTP Basic:**
```yaml
auth:
  type: http
  scheme: Basic
  username: $USER
  password: $PASS
```

**Google Credentials (ADC):**
```yaml
auth:
  type: google-credentials
  scopes:
    - https://www.googleapis.com/auth/cloud-platform
```
Uses access tokens for `*.googleapis.com`, identity tokens for `*.run.app`.

**OAuth 2.0 (Authorization Code + PKCE):**
```yaml
auth:
  type: oauth2
  client_id: my-client-id.apps.example.com
  client_secret: $SECRET   # optional for public clients
```

`[Official]`

#### Dynamic Value Resolution

Secret values support special syntax:

| Pattern | Behavior |
|---------|----------|
| `$ENV_VAR` | Read from environment variable |
| `!command` | Execute shell command, use output |
| `literal` | Use as-is |
| `$$prefix` | Escape `$` to literal dollar sign |

`[Official]`

#### Multi-Agent Files

Multiple remote agents can be defined in a single `.md` file using YAML document
separators (`---`). `[Official]`

### 2.6 Agent Configuration Overrides (settings.json)

```json
{
  "agents": {
    "overrides": {
      "agent-name": {
        "enabled": true,
        "modelConfig": {
          "model": "gemini-3-flash-preview"
        },
        "runConfig": {
          "maxTurns": 50
        }
      }
    }
  },
  "experimental": {
    "enableAgents": true
  }
}
```

`[Official]`

### 2.7 Browser Agent Configuration

The built-in `browser_agent` has additional settings:

```json
{
  "agents": {
    "browser": {
      "sessionMode": "persistent",
      "headless": false,
      "allowedDomains": ["github.com", "*.google.com", "localhost"],
      "confirmSensitiveActions": false
    }
  }
}
```

| Field | Type | Values | Default |
|-------|------|--------|---------|
| `sessionMode` | enum | `"persistent"`, `"isolated"`, `"existing"` | `"persistent"` |
| `headless` | boolean | — | `false` |
| `allowedDomains` | string[] | Supports wildcards | — |
| `confirmSensitiveActions` | boolean | — | `false` |

Session modes:
- `persistent` — profile saved at `~/.gemini/cli-browser-profile/`
- `isolated` — temporary profile, deleted after session
- `existing` — attach to a running Chrome instance

`[Official]`

### 2.8 Invocation Patterns

| Method | Syntax | Behavior |
|--------|--------|----------|
| Automatic | (natural language) | Main agent decides when to delegate |
| Explicit | `@agent-name <task>` | Bypasses main agent, goes directly to specialist |
| Management | `/agents list` | View all agents |
| Management | `/agents reload` | Refresh agent registry |
| Management | `/agents enable <name>` | Enable agent |
| Management | `/agents disable <name>` | Disable agent |

`[Official]`

### 2.9 Built-in Subagents

| Name | Purpose |
|------|---------|
| `codebase_investigator` | Analyzes dependencies and code relationships |
| `cli_help` | Gemini CLI documentation and configuration expert |
| `generalist_agent` | Routes tasks to appropriate specialists |
| `browser_agent` | Web automation via accessibility tree (experimental) |

`[Official]`

---

## 3. Agent Skills

Skills package complex workflows into reusable, self-contained directories. Only
skill metadata is loaded initially — detailed instructions are disclosed only when
the model activates the skill, saving context tokens. `[Official]`

**Source:** https://geminicli.com/docs/cli/skills/

### 3.1 Skill Directory Structure

```
my-skill/
├── SKILL.md          (Required — metadata + instructions)
├── scripts/          (Optional — helper scripts)
├── references/       (Optional — reference materials)
└── assets/           (Optional — templates, data files)
```

Only `SKILL.md` is required. `[Official]`

### 3.2 SKILL.md Format

```markdown
---
name: security-audit
description: Audit code for security vulnerabilities when requested
---

# Security Auditor

Review code for OWASP Top 10 vulnerabilities and hardcoded secrets.

## Steps
1. Scan all source files for hardcoded credentials
2. Check for SQL injection vectors
3. Verify input validation
4. Report findings with severity ratings
```

`[Official]`

### 3.3 SKILL.md Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier, should match directory name |
| `description` | string | Yes | Explains purpose and when to use (shown to model for matching) |

The documentation does not list additional frontmatter fields beyond `name` and
`description`. `[Official]`

### 3.4 Discovery Tiers (Precedence Order)

| Tier | Location | Scope |
|------|----------|-------|
| 1 (highest) | `.gemini/skills/` or `.agents/skills/` | Workspace (project) |
| 2 | `~/.gemini/skills/` or `~/.agents/skills/` | User (global) |
| 3 (lowest) | `<extension>/skills/` | Extension-bundled |

`[Official]`

### 3.5 Activation Flow

1. CLI scans discovery tiers at startup, injecting skill metadata into system prompt
2. Model identifies a matching task and calls the `activate_skill` tool
3. User is prompted for confirmation before activation
4. On approval: `SKILL.md` content and folder structure loaded into conversation
5. Skill directory gains file-access permissions for bundled assets

`[Official]`

### 3.6 Skill Management Commands

**Interactive session:**

| Command | Description |
|---------|-------------|
| `/skills list` | View all discovered skills |
| `/skills link <path>` | Symlink an agent skill |
| `/skills disable <name>` | Prevent skill usage |
| `/skills enable <name>` | Re-enable a skill |
| `/skills reload` | Refresh skill discovery |

**Terminal:**

| Command | Description |
|---------|-------------|
| `gemini skills list` | List all skills |
| `gemini skills link <path>` | Symlink skill |
| `gemini skills install <source>` | Install from Git repo, local dir, or `.skill` file |
| `gemini skills uninstall <name>` | Remove skill |
| `gemini skills enable <name>` | Enable |
| `gemini skills disable <name>` | Disable |

The `--scope` flag targets workspace or user scope. `[Official]`

---

## 4. Custom Commands

Custom commands are TOML files that define reusable prompt shortcuts, invokable as
slash commands. `[Official]`

**Source:** https://geminicli.com/docs/cli/custom-commands/

### 4.1 TOML Format

```toml
description = "Summarize grep results for a pattern"
prompt = """Summarize findings for pattern `{{args}}`.
Search Results:
!{grep -r {{args}} .}"""
```

`[Official]`

### 4.2 Command Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | Yes | The prompt sent to the model |
| `description` | string | No | One-line description for `/help` menu |

`[Official]`

### 4.3 File Locations and Naming

| Location | Scope | Invocation |
|----------|-------|------------|
| `~/.gemini/commands/test.toml` | User (global) | `/test` |
| `.gemini/commands/test.toml` | Project | `/test` (overrides user) |
| `.gemini/commands/git/commit.toml` | Project | `/git:commit` (namespaced) |
| `<extension>/commands/group/name.toml` | Extension | `/group:name` |

Project commands override user commands of the same name. Nested directories
create colon-namespaced commands. `[Official]`

### 4.4 Argument Handling

| Method | Trigger | Behavior |
|--------|---------|----------|
| Placeholder | `{{args}}` in prompt | Replaced with user's text |
| Appended | No `{{args}}` | Arguments appended after prompt |

`[Official]`

### 4.5 Shell Command Injection

Use `!{...}` syntax to execute shell commands and inject output into prompts:

```toml
prompt = """Review the diff:
!{git diff HEAD~1}"""
```

- `{{args}}` inside `!{...}` is automatically shell-escaped
- Nested braces (e.g., JSON payloads) are handled correctly
- User is prompted for confirmation before shell execution (security measure)

Reload after changes: `/commands reload`. `[Official]`

---

## 5. Hooks

Hooks are scripts executed at specific points in the agentic loop, acting as
middleware for the CLI. Enabled by default since v0.26.0+. `[Official]`

**Source:** https://geminicli.com/docs/hooks/reference/

### 5.1 Hook Configuration (settings.json)

```json
{
  "hooks": {
    "BeforeTool": [
      {
        "matcher": "write_file|replace",
        "sequential": false,
        "hooks": [
          {
            "name": "security-check",
            "type": "command",
            "command": "$GEMINI_PROJECT_DIR/.gemini/hooks/security.sh",
            "timeout": 5000,
            "description": "Check for security issues"
          }
        ]
      }
    ]
  }
}
```

`[Official]`

### 5.2 Hook Group Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `matcher` | string | No | Regex (tool events) or exact string (lifecycle events); `"*"` or `""` matches all |
| `sequential` | boolean | No | Controls parallel vs. sequential execution |
| `hooks` | object[] | Yes | Array of hook configurations |

`[Official]`

### 5.3 Hook Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Currently only `"command"` |
| `command` | string | Yes | Shell command to execute |
| `name` | string | No | Friendly identifier for logs |
| `timeout` | number | No | Milliseconds; default 60000 |
| `description` | string | No | Purpose explanation |

`[Official]`

### 5.4 Hook Events

| Event | When | Key Input Fields | Key Output Actions |
|-------|------|-------------------|--------------------|
| `SessionStart` | Session begins | `source` (`startup`, `resume`, `clear`) | `additionalContext` injected as first turn |
| `BeforeAgent` | After user prompt, before planning | `prompt` | `additionalContext`, `decision: deny` |
| `BeforeToolSelection` | Before tool selection | `llm_request` | `toolConfig.mode` (`AUTO`, `ANY`, `NONE`), `allowedFunctionNames` |
| `BeforeModel` | Before LLM request | `llm_request` | Override request, provide synthetic `llm_response` |
| `AfterModel` | After LLM response | `llm_request`, `llm_response` | Replace response, fires per-chunk during streaming |
| `BeforeTool` | Before tool execution | `tool_name`, `tool_input` | `decision: deny`, modify `tool_input` |
| `AfterTool` | After tool execution | `tool_name`, `tool_input`, `tool_response` | `decision: deny`, `additionalContext`, `tailToolCallRequest` |
| `AfterAgent` | After agent completes | `prompt`, `prompt_response` | `decision: deny` forces retry with `reason` as new prompt |
| `PreCompress` | Before history compression | `trigger` (`auto`, `manual`) | Observability only; async, cannot block |
| `SessionEnd` | Session ending | `reason` (`exit`, `clear`, `logout`, etc.) | `systemMessage` display only; non-blocking |
| `Notification` | On alerts | `notification_type`, `message`, `details` | Observability only |

`[Official]`

### 5.5 Communication Protocol

- **Input:** JSON via stdin
- **Output:** JSON via stdout (ONLY — no other stdout output)
- **Logging:** Use stderr for debug output
- **Exit 0:** Success; stdout parsed as JSON
- **Exit 2:** System block; stderr becomes rejection reason
- **Other exit codes:** Non-fatal warning

All hooks receive base fields: `session_id`, `transcript_path`, `cwd`,
`hook_event_name`, `timestamp`. `[Official]`

### 5.6 Environment Variables in Hooks

| Variable | Description |
|----------|-------------|
| `GEMINI_PROJECT_DIR` | Absolute path to project root |
| `GEMINI_SESSION_ID` | Unique session ID |
| `GEMINI_CWD` | Current working directory |
| `CLAUDE_PROJECT_DIR` | Alias for compatibility |

`[Official]`

---

## 6. Policy Engine (Tool Permissions)

The policy engine controls tool access through TOML rule files. Rules are evaluated
by priority to determine whether a tool call is allowed, denied, or requires user
confirmation. `[Official]`

**Source:** https://geminicli.com/docs/reference/policy-engine/

### 6.1 TOML Rule Format

```toml
[[rule]]
toolName = "run_shell_command"
subagent = "generalist"
mcpName = "my-custom-server"
toolAnnotations = { readOnlyHint = true }
argsPattern = '"command":"(git|npm)'
commandPrefix = "git"
commandRegex = "git (commit|push)"
decision = "ask_user"
priority = 10
deny_message = "Deletion is permanent"
modes = ["autoEdit"]
interactive = true
```

`[Official]`

### 6.2 Rule Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `toolName` | string or string[] | Yes | Tool name(s) to match |
| `decision` | string | Yes | `"allow"`, `"deny"`, or `"ask_user"` |
| `priority` | number | Yes | 0–999 within the TOML file |
| `argsPattern` | string | No | Regex tested against JSON-serialized arguments |
| `commandPrefix` | string or string[] | No | Shell command prefix match (sugar for shell tools) |
| `commandRegex` | string | No | Regex for shell commands (cannot combine with `commandPrefix`) |
| `subagent` | string | No | Restrict rule to a specific subagent |
| `mcpName` | string | No | MCP server name; `"*"` for all MCP tools |
| `toolAnnotations` | object | No | Match on tool metadata hints |
| `deny_message` | string | No | Custom message shown on denial |
| `modes` | string[] | No | Approval modes: `"default"`, `"autoEdit"`, `"plan"`, `"yolo"` |
| `interactive` | boolean | No | Scope to interactive/non-interactive mode |

`[Official]`

### 6.3 Decision Values

| Decision | Behavior |
|----------|----------|
| `allow` | Auto-approve without user confirmation |
| `deny` | Block execution; global denials remove tool from model's context |
| `ask_user` | Prompt user for confirmation (treated as deny in headless mode) |

`[Official]`

### 6.4 Priority Tiers

Final priority = `tier_base + (toml_priority / 1000)`

| Tier | Base | Source |
|------|------|--------|
| Default | 1 | Built-in defaults |
| Extension | 2 | Extension `policies/` directories |
| Workspace | 3 | `.gemini/policies/*.toml` |
| User | 4 | `~/.gemini/policies/*.toml` |
| Admin | 5 | System-wide (`/etc/gemini-cli/policies`, etc.) |

Higher tiers always override lower tiers. `[Official]`

### 6.5 MCP Tool Matching

| Pattern | Matches |
|---------|---------|
| `mcpName = "server"` + `toolName = "search"` | Specific tool on specific server |
| `mcpName = "untrusted"` (no toolName) | All tools from that server |
| `mcpName = "*"` | All tools from all MCP servers |
| `mcpName = "*"` + `toolName = "search"` | Tool named "search" on any server |

`[Official]`

### 6.6 Tool Annotation Matching

```toml
toolAnnotations = { readOnlyHint = true, destructiveHint = false }
```

Matches if all specified key-value pairs exist in the tool's metadata. Based on
MCP tool annotation hints: `readOnlyHint`, `destructiveHint`, `idempotentHint`.
`[Official]`

### 6.7 Policy File Locations

| Scope | Path |
|-------|------|
| User | `~/.gemini/policies/*.toml` |
| Workspace | `$PROJECT_ROOT/.gemini/policies/*.toml` |
| Admin (Linux) | `/etc/gemini-cli/policies` |
| Admin (macOS) | `/Library/Application Support/GeminiCli/policies` |
| Admin (Windows) | `C:\ProgramData\gemini-cli\policies` |
| Extension | `<extension>/policies/*.toml` |

`[Official]`

---

## 7. Custom Tool Discovery

Beyond MCP servers, Gemini CLI supports command-based tool discovery for
registering entirely custom tools. `[Official]`

**Source:** https://google-gemini.github.io/gemini-cli/docs/core/tools-api.html

### 7.1 Configuration (settings.json)

```json
{
  "tools": {
    "discoveryCommand": "bin/get_tools",
    "callCommand": "bin/call_tool",
    "core": ["read_file", "write_file", "grep_search"],
    "exclude": ["web_search"],
    "allowed": ["run_shell_command(git)", "run_shell_command(npm test)"],
    "sandbox": "docker"
  }
}
```

`[Official]`

### 7.2 Tools Settings Fields

| Field | Type | Description |
|-------|------|-------------|
| `tools.discoveryCommand` | string | Command that outputs JSON array of `FunctionDeclaration` objects |
| `tools.callCommand` | string | Command to execute discovered tools; receives tool name as first arg, JSON on stdin |
| `tools.core` | string[] | Allowlist restricting which built-in tools are available |
| `tools.exclude` | string[] | Tool names to remove from discovery |
| `tools.allowed` | string[] | Tool names that bypass confirmation dialogs |
| `tools.sandbox` | string\|boolean | Sandbox mode: `true`, `false`, `"docker"`, `"podman"`, `"lxc"`, `"windows-native"`, or custom command |
| `tools.shell.inactivityTimeout` | number | Max seconds without shell output; default 300 |

`[Official]`

### 7.3 FunctionDeclaration Schema

The `discoveryCommand` must output a JSON array where each element is a
`FunctionDeclaration` — the same schema used by the Gemini API:

```json
[
  {
    "name": "my_custom_tool",
    "description": "What this tool does",
    "parameters": {
      "type": "OBJECT",
      "properties": {
        "query": {
          "type": "STRING",
          "description": "The search query"
        },
        "limit": {
          "type": "INTEGER",
          "description": "Max results"
        }
      },
      "required": ["query"]
    }
  }
]
```

The `callCommand` receives the tool name as first argument and reads parameter
JSON from stdin. It must output JSON on stdout. `[Inferred from docs + API schema]`

### 7.4 Built-in Tools

| Tool | Description |
|------|-------------|
| `list_dir` | List directory contents |
| `read_file` | Read a single file (absolute path) |
| `read_many_files` | Read/concatenate from multiple files or globs |
| `write_file` | Write content to a file |
| `edit_file` | In-place file modifications |
| `grep_search` | Search for patterns in files |
| `glob` | Find files matching glob patterns |
| `run_shell_command` | Execute shell commands |
| `web_fetch` | Fetch web content |
| `web_search` | Search the web |
| `memory` | Read/write persistent memory |

`[Official]`

---

## 8. Gemini-Specific Concepts

These features are unique to Gemini CLI or have no direct equivalent in other AI
coding tool providers.

### 8.1 Extensions as a First-Class Package Format

Gemini CLI has a dedicated extension packaging system with its own manifest format
(`gemini-extension.json`), installation commands, a public gallery, settings
management with keychain integration, and variable substitution. This goes beyond
what Claude Code or Cursor offer — it is closer to a VS Code extension model
applied to a CLI agent. `[Inferred]`

### 8.2 Policy Engine with Priority Tiers

The TOML-based policy engine with its five-tier priority system (Default,
Extension, Workspace, User, Admin) is unique. It provides enterprise-grade tool
access control with:
- Per-argument regex matching
- MCP server-level wildcards
- Tool annotation matching (read-only, destructive, idempotent hints)
- Approval mode scoping (default, autoEdit, plan, yolo)
- Admin policies that cannot be overridden by users

No other AI CLI tool has this level of policy granularity. `[Inferred]`

### 8.3 Agent-to-Agent (A2A) Protocol for Remote Agents

Gemini CLI is the first major AI CLI to support the A2A protocol for connecting
to remote agent services. This allows agents running on different infrastructure
to collaborate, with built-in support for multiple auth methods (API key, HTTP
bearer/basic, Google credentials, OAuth 2.0). `[Official]`

### 8.4 Hook-Based Tool Selection Override

The `BeforeToolSelection` hook event allows external scripts to dynamically
control which tools the model can use on a per-turn basis. The `toolConfig.mode`
field (`AUTO`, `ANY`, `NONE`) and `allowedFunctionNames` whitelist provide
runtime tool filtering that no other CLI tool offers. `[Official]`

### 8.5 Command-Based Tool Discovery

The `tools.discoveryCommand` / `tools.callCommand` pattern allows registering
tools without MCP servers — just a script that outputs JSON tool declarations and
another that executes them. This is simpler than standing up a full MCP server for
simple custom tools. `[Official]`

### 8.6 Skill Activation Model

Unlike Claude Code skills (which are always-loaded markdown files), Gemini CLI
skills use a lazy activation model:
1. Only metadata (name + description) is loaded at startup
2. The model calls `activate_skill` when it identifies a matching task
3. User confirms activation
4. Full SKILL.md content and assets are then loaded

This saves context tokens by deferring full skill loading. `[Official]`

### 8.7 Extension Migration (`migratedTo`)

Extensions can declare a `migratedTo` URL in their manifest, allowing the CLI to
automatically redirect updates when an extension moves to a new repository. This
is a package management feature not found in other AI CLI tools. `[Official]`

### 8.8 Multi-File Agent Definitions

Remote agents support defining multiple agents in a single `.md` file using YAML
document separators. This is unique to Gemini CLI. `[Official]`

### 8.9 Context File Hierarchy

Gemini CLI's `GEMINI.md` context system supports hierarchical loading:
- Global: `~/.gemini/GEMINI.md`
- Upward search: current directory to project root (`.git` boundary)
- Downward search: subdirectories below CWD (respects `.gitignore`, `.geminiignore`)
- Configurable filename via `context.fileName` (can be an array of filenames)
- Discovery depth limited by `context.discoveryMaxDirs` (default: 200)

The bidirectional (up + down) search and configurable filename array are unique.
`[Official]`

---

## Sources

### Official Documentation
- [Extension Reference](https://geminicli.com/docs/extensions/reference/)
- [Writing Extensions](https://geminicli.com/docs/extensions/writing-extensions/)
- [Subagents](https://geminicli.com/docs/core/subagents/)
- [Remote Subagents](https://geminicli.com/docs/core/remote-agents/)
- [Agent Skills](https://geminicli.com/docs/cli/skills/)
- [Creating Skills](https://geminicli.com/docs/cli/creating-skills)
- [Custom Commands](https://geminicli.com/docs/cli/custom-commands/)
- [Hooks Reference](https://geminicli.com/docs/hooks/reference/)
- [Policy Engine](https://geminicli.com/docs/reference/policy-engine/)
- [Configuration Reference](https://geminicli.com/docs/reference/configuration/)
- [Tools API](https://google-gemini.github.io/gemini-cli/docs/core/tools-api.html)
- [GitHub Repository](https://github.com/google-gemini/gemini-cli)

### Community / Blog
- [Making Gemini CLI Extensions Easier to Use](https://developers.googleblog.com/making-gemini-cli-extensions-easier-to-use/)
- [Multi-Agent Architecture Proposal](https://github.com/google-gemini/gemini-cli/discussions/7637)
- [How I Turned Gemini CLI into a Multi-Agent System](https://aipositive.substack.com/p/how-i-turned-gemini-cli-into-a-multi)
- [Gemini CLI Extensions Blog Post](https://blog.google/innovation-and-ai/technology/developers-tools/gemini-cli-extensions/)
