# Gemini CLI Content Types Reference

Comprehensive documentation of all content types supported by Gemini CLI (google-gemini/gemini-cli).

**Last updated:** 2026-03-20
**Gemini CLI version basis:** v0.26.0+ (hooks enabled by default), with experimental features noted.

---

## Table of Contents

1. [Instructions/Context Files (GEMINI.md)](#1-instructionscontext-files-geminimd)
2. [Settings (settings.json)](#2-settings-settingsjson)
3. [MCP Servers](#3-mcp-servers)
4. [Custom Commands (TOML)](#4-custom-commands-toml)
5. [Hooks](#5-hooks)
6. [Subagents (Local)](#6-subagents-local)
7. [Subagents (Remote / A2A)](#7-subagents-remote--a2a)
8. [Skills](#8-skills)
9. [Extensions](#9-extensions)
10. [Ignore Files (.geminiignore)](#10-ignore-files-geminiignore)
11. [Environment Files (.env)](#11-environment-files-env)
12. [Built-in Tools Reference](#12-built-in-tools-reference)

---

## 1. Instructions/Context Files (GEMINI.md)

Persistent instructional context provided to the Gemini model with every prompt. Equivalent to Claude Code's `CLAUDE.md` or Cursor's `.cursorrules`.

### File Format

- **Extension:** `.md` (Markdown)
- **Encoding:** UTF-8
- **Default filename:** `GEMINI.md`
- **Configurable filename:** Yes, via `context.fileName` in `settings.json` [Official]

The filename can be a single string or an array of strings:

```json
{
  "context": {
    "fileName": ["AGENTS.md", "CONTEXT.md", "GEMINI.md"]
  }
}
```

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html

### Directory Structure

| Scope | Path | Purpose |
|-------|------|---------|
| Global | `~/.gemini/GEMINI.md` | Applies to all projects |
| Project/Ancestor | `<project-root>/GEMINI.md` and parent dirs | Project-specific instructions |
| Subdirectory | `<project-root>/**/GEMINI.md` | Component-specific instructions |

[Official] Source: https://geminicli.com/docs/cli/gemini-md/

### Discovery & Loading Order

1. **Global Context** -- `~/.gemini/GEMINI.md` [Official]
2. **Workspace Context** -- configured workspace directories and parent directories up to project root (`.git` boundary or home directory) [Official]
3. **Just-in-Time (JIT) Context** -- auto-scanned when tools access files in directories containing context files; discovered from accessed directory and ancestors up to trusted root [Official]

The CLI footer displays the count of loaded context files. [Official]

**Merge behavior:** All found files are concatenated and sent to the model with every prompt. More specific files supplement (not replace) general ones. [Official]

**Discovery limit:** Subdirectory scanning is capped at 200 directories by default, configurable via `context.discoveryMaxDirs`. [Official]

### Import Syntax

Context files support modular imports using `@path` syntax:

```markdown
# Main GEMINI.md file
This is the main content.
@./components/instructions.md
More content here.
@../shared/style-guide.md
```

Paths can be relative or absolute. Imports are resolved recursively. [Official]

### Memory Management Commands

| Command | Purpose |
|---------|---------|
| `/memory show` | Display concatenated context content |
| `/memory reload` | Re-scan all context files from all locations |
| `/memory add <text>` | Append text to global `~/.gemini/GEMINI.md` |

[Official] Source: https://geminicli.com/docs/cli/gemini-md/

### Example

```markdown
# Project: My TypeScript Library

## General Instructions
- When you generate new TypeScript code, follow the existing coding style.
- Ensure all new functions and classes have JSDoc comments.
- Prefer functional programming paradigms where appropriate.

## Coding Style
- Use 2 spaces for indentation.
- Prefix interface names with `I` (for example, `IUserService`).
- Always use strict equality (`===` and `!==`).
```

[Official] Source: https://geminicli.com/docs/cli/gemini-md/

---

## 2. Settings (settings.json)

Central configuration file controlling all CLI behavior: model selection, tools, UI, security, telemetry.

### File Format

- **Extension:** `.json`
- **Encoding:** UTF-8
- **Supports environment variable expansion:** `$VAR_NAME` or `${VAR_NAME}` syntax [Official]

### Directory Structure & Precedence

Settings are merged from multiple sources. Higher numbers override lower:

| Priority | Type | Linux Path | Windows Path | macOS Path |
|----------|------|------------|--------------|------------|
| 1 (lowest) | Hardcoded defaults | -- | -- | -- |
| 2 | System defaults | `/etc/gemini-cli/system-defaults.json` | `C:\ProgramData\gemini-cli\system-defaults.json` | `/Library/Application Support/GeminiCli/system-defaults.json` |
| 3 | User settings | `~/.gemini/settings.json` | `~/.gemini/settings.json` | `~/.gemini/settings.json` |
| 4 | Project settings | `<project>/.gemini/settings.json` | Same | Same |
| 5 (highest file) | System settings | `/etc/gemini-cli/settings.json` | `C:\ProgramData\gemini-cli\settings.json` | `/Library/Application Support/GeminiCli/settings.json` |
| 6 | Environment variables | -- | -- | -- |
| 7 (highest) | Command-line arguments | -- | -- | -- |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html

### Complete Config Schema

#### `general`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `preferredEditor` | string | undefined | Editor to open files |
| `vimMode` | boolean | false | Enable Vim keybindings |
| `disableAutoUpdate` | boolean | false | Disable auto-update checks |
| `disableUpdateNag` | boolean | false | Suppress update notifications |
| `checkpointing.enabled` | boolean | false | Session recovery support |

#### `output`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | `"text"` | Output format: `"text"` or `"json"` |

#### `ui`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `theme` | string | undefined | UI theme name |
| `customThemes` | object | `{}` | Custom theme definitions |
| `hideWindowTitle` | boolean | false | Hide terminal window title |
| `hideTips` | boolean | false | Hide tip messages |
| `hideBanner` | boolean | false | Hide startup banner |
| `hideFooter` | boolean | false | Hide footer bar |
| `showMemoryUsage` | boolean | false | Display memory usage stats |
| `showLineNumbers` | boolean | false | Show line numbers in output |
| `showCitations` | boolean | true | Show source citations |
| `accessibility.disableLoadingPhrases` | boolean | false | Disable animated loading text |
| `customWittyPhrases` | string[] | `[]` | Custom loading phrases |

#### `ide`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | false | Enable IDE integration |
| `hasSeenNudge` | boolean | false | Track IDE nudge display |

#### `privacy`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `usageStatisticsEnabled` | boolean | true | Send usage statistics |

#### `model`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | undefined | Gemini model identifier |
| `maxSessionTurns` | number | -1 | Max turns per session (-1 = unlimited) |
| `summarizeToolOutput` | object | undefined | Per-tool token budgets, e.g. `{"run_shell_command": {"tokenBudget": 2000}}` |
| `chatCompression.contextPercentageThreshold` | number | 0.7 | Context compression threshold (0-1) |
| `skipNextSpeakerCheck` | boolean | false | Skip speaker validation |

#### `context`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fileName` | string or string[] | undefined | Context file name(s) to discover |
| `importFormat` | string | undefined | Import format for context files |
| `discoveryMaxDirs` | number | 200 | Max subdirectories to scan |
| `includeDirectories` | string[] | `[]` | Additional workspace directories |
| `loadFromIncludeDirectories` | boolean | false | Scan include dirs on `/memory refresh` |
| `fileFiltering.respectGitIgnore` | boolean | true | Honor `.gitignore` rules |
| `fileFiltering.respectGeminiIgnore` | boolean | true | Honor `.geminiignore` rules |
| `fileFiltering.enableRecursiveFileSearch` | boolean | true | Recursive `@` reference search |

#### `tools`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sandbox` | boolean or string | undefined | Sandbox mode: `true`/`false` or path/command |
| `shell.enableInteractiveShell` | boolean | false | Use node-pty for interactive shell |
| `core` | string[] | undefined | Allowlist of built-in tools (e.g. `["run_shell_command(git)"]`) |
| `exclude` | string[] | undefined | Tools to disable |
| `allowed` | string[] | undefined | Tools that bypass confirmation prompts |
| `discoveryCommand` | string | undefined | Custom tool discovery command |
| `callCommand` | string | undefined | Custom tool execution command |

#### `mcp` (global MCP settings)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `serverCommand` | string | undefined | Global MCP server start command |
| `allowed` | string[] | undefined | MCP server allowlist |
| `excluded` | string[] | undefined | MCP server blocklist |

#### `security`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `folderTrust.enabled` | boolean | false | Enable folder trust system |
| `auth.selectedType` | string | undefined | Selected auth type |
| `auth.enforcedType` | string | undefined | Enforced auth type |
| `auth.useExternal` | boolean | undefined | Use external auth |

#### `telemetry`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | -- | Enable telemetry |
| `target` | string | -- | Target: `"local"` or `"gcp"` |
| `otlpEndpoint` | string | -- | OpenTelemetry endpoint |
| `otlpProtocol` | string | -- | Protocol: `"grpc"` or `"http"` |
| `logPrompts` | boolean | -- | Log prompt content |
| `outfile` | string | -- | Local telemetry output file |
| `useCollector` | boolean | -- | Use OTLP collector |

#### `advanced`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `autoConfigureMemory` | boolean | false | Auto-configure memory |
| `dnsResolutionOrder` | string | undefined | DNS resolution preference |
| `excludedEnvVars` | string[] | `["DEBUG","DEBUG_MODE"]` | Env vars excluded from context |
| `bugCommand` | object | undefined | Custom bug report command |

#### `experimental`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enableAgents` | boolean | false | Enable subagent system |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html

---

## 3. MCP Servers

MCP (Model Context Protocol) server configurations extend Gemini CLI with external tools and services. Configured within `settings.json` under the `mcpServers` key.

### File Format

MCP servers are not standalone files -- they are defined as entries within `settings.json` (or `gemini-extension.json` for extensions). [Official]

### Config Schema

Each server is a named entry under `mcpServers`:

```json
{
  "mcpServers": {
    "<server-name>": {
      // Transport (exactly one required)
      "command": "path/to/executable",    // Stdio transport
      "url": "https://example.com/sse",   // SSE transport
      "httpUrl": "https://example.com/",  // Streamable HTTP transport

      // Common optional fields
      "args": ["--flag", "value"],
      "env": {
        "KEY": "$ENV_VAR"
      },
      "cwd": "./relative/path",
      "headers": {"Authorization": "Bearer $TOKEN"},
      "timeout": 600000,
      "trust": false,
      "description": "What this server does",
      "includeTools": ["tool_a", "tool_b"],
      "excludeTools": ["dangerous_tool"],

      // OAuth (optional)
      "oauth": {
        "enabled": true,
        "clientId": "...",
        "clientSecret": "$SECRET",
        "authorizationUrl": "https://...",
        "tokenUrl": "https://...",
        "scopes": ["scope1"]
      },

      // Google Cloud auth (optional)
      "authProviderType": "google_credentials",
      "targetAudience": "...",
      "targetServiceAccount": "..."
    }
  }
}
```

### Transport Types

| Transport | Config Key | Description |
|-----------|-----------|-------------|
| Stdio | `command` (+`args`, `cwd`) | Spawns subprocess, communicates via stdin/stdout |
| SSE | `url` (+`headers`) | Connects to Server-Sent Events endpoint |
| Streamable HTTP | `httpUrl` (+`headers`) | HTTP streaming transport |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html

### Field Reference

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `command` | string | One transport required | -- | Executable path (Stdio) |
| `args` | string[] | No | `[]` | Command arguments (Stdio) |
| `env` | object | No | `{}` | Environment variables (supports `$VAR` expansion) |
| `cwd` | string | No | -- | Working directory (Stdio) |
| `url` | string | One transport required | -- | SSE endpoint URL |
| `httpUrl` | string | One transport required | -- | Streamable HTTP endpoint |
| `headers` | object | No | `{}` | HTTP headers (SSE/HTTP transports) |
| `timeout` | number | No | 600000 | Request timeout in milliseconds |
| `trust` | boolean | No | false | Bypass tool confirmation dialogs |
| `description` | string | No | -- | Human-readable description |
| `includeTools` | string[] | No | -- | Allowlist of tools to enable |
| `excludeTools` | string[] | No | -- | Blocklist of tools to disable |

**Naming constraint:** Do not use underscores in server names. Use hyphens instead (e.g., `my-server` not `my_server`). The policy parser splits Fully Qualified Names on the first underscore after the `mcp_` prefix. [Official]

**Tool policy precedence:** `excludeTools` always takes precedence over `includeTools`. When merging with extensions, exclusions are unioned and inclusions are intersected. [Official]

### Verification Commands

| Command | Purpose |
|---------|---------|
| `/mcp` | Show connected MCP servers |
| `/mcp desc` | Show detailed tool descriptions |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html

---

## 4. Custom Commands (TOML)

Reusable slash commands defined as TOML files. Equivalent to Claude Code's custom slash commands.

### File Format

- **Extension:** `.toml`
- **Encoding:** UTF-8

### Directory Structure

| Scope | Path | Precedence |
|-------|------|------------|
| Project | `<project-root>/.gemini/commands/*.toml` | Higher (overrides user) |
| User (global) | `~/.gemini/commands/*.toml` | Lower |

Subdirectories create namespaced commands using colon syntax:
- `.gemini/commands/test.toml` --> `/test`
- `.gemini/commands/git/commit.toml` --> `/git:commit`

[Official] Source: https://geminicli.com/docs/cli/custom-commands/

### Config Schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | **Yes** | Prompt sent to the model when command is executed |
| `description` | string | No | One-line description shown in `/help` menu |

[Official] Source: https://geminicli.com/docs/cli/custom-commands/

### Template Syntax

| Syntax | Purpose | Notes |
|--------|---------|-------|
| `{{args}}` | Inject user arguments | Auto shell-escaped inside `!{...}` blocks |
| `!{command}` | Execute shell command, inject output | Requires user confirmation |
| `@{path}` | Embed file/directory content | Supports images, PDFs, audio, video |

Without `{{args}}`, user arguments are appended after two newlines. [Official]

### Example

```toml
# ~/.gemini/commands/refactor/pure.toml
description = "Refactor the current context into a pure function."
prompt = """Please analyze the code I've provided in the current context.
Refactor it into a pure function.

Your response should include:
1. The refactored, pure function code block.
2. A brief explanation of the key changes you made and why they contribute to purity."""
```

### Example with Shell Execution

```toml
# .gemini/commands/changelog.toml
description = "Generate a changelog from recent commits"
prompt = """Based on these recent commits:

!{git log --oneline -20}

Generate a changelog entry for version {{args}}."""
```

### Management

After creating or modifying command files, run `/commands reload` to apply changes without restarting. [Official]

---

## 5. Hooks

Scripts that execute at specific lifecycle points in the CLI's agentic loop. Configured in `settings.json` under the `hooks` key. Available since v0.26.0+. [Official]

### File Format

Hook scripts can be written in any language (bash, Python, Node.js, PowerShell). They communicate via:
- **Input:** JSON on `stdin`
- **Output:** JSON on `stdout` (only valid JSON; no other stdout output)
- **Logging:** `stderr` for human-readable feedback

[Official] Source: https://geminicli.com/docs/hooks/reference/

### Directory Structure

Hook scripts are typically placed in:
- `<project-root>/.gemini/hooks/` (project-level)
- `~/.gemini/hooks/` (user-level, by convention)

Hooks are registered in `settings.json`, not discovered by directory scanning. [Official]

### Configuration Schema

```json
{
  "hooks": {
    "<EventName>": [
      {
        "matcher": "regex_or_exact_string",
        "sequential": false,
        "hooks": [
          {
            "type": "command",
            "command": "path/to/script.sh",
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

#### Matcher Group Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `matcher` | string | No | `"*"` | Regex (tool events) or exact string (lifecycle events) |
| `sequential` | boolean | No | false | Run hooks sequentially vs. parallel |
| `hooks` | array | Yes | -- | Array of hook definitions |

#### Hook Definition Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | Yes | -- | Currently only `"command"` |
| `command` | string | Yes | -- | Shell command to execute |
| `name` | string | No | -- | Friendly identifier for logs |
| `timeout` | number | No | 60000 | Timeout in milliseconds |
| `description` | string | No | -- | Brief explanation |

[Official] Source: https://geminicli.com/docs/hooks/reference/

### Hook Event Types

#### Tool Events (regex matchers)

| Event | When | Can Modify | Can Block |
|-------|------|-----------|-----------|
| `BeforeTool` | Before tool execution | Tool input args | Yes (exit 2 or `decision: "deny"`) |
| `AfterTool` | After tool execution | Tool output, chain tools | Yes (hide result) |

**MCP tool matcher pattern:** `mcp_<server-name>_<tool-name>` [Official]

#### Agent Events

| Event | When | Can Modify | Can Block |
|-------|------|-----------|-----------|
| `BeforeAgent` | After user prompt, before agent planning | Inject context | Yes |
| `AfterAgent` | After model's final response | Force retry | Yes |

#### Model Events

| Event | When | Can Modify | Can Block |
|-------|------|-----------|-----------|
| `BeforeModel` | Before LLM request | Override request, inject synthetic response | Yes |
| `BeforeToolSelection` | Before tool selection | Filter available tools, force tool mode | No (advisory) |
| `AfterModel` | After each LLM response chunk | Replace/redact chunk | Yes |

#### Lifecycle Events (exact string matchers)

| Event | When | Can Block | Notes |
|-------|------|-----------|-------|
| `SessionStart` | Startup, resume, or `/clear` | No | Can inject initial context |
| `SessionEnd` | Exit or session clear | No | Best-effort, CLI won't wait |
| `Notification` | System alerts (e.g., tool permissions) | No | Observability only |
| `PreCompress` | Before history compression | No | Async, advisory only |

[Official] Source: https://geminicli.com/docs/hooks/reference/

### Exit Codes

| Code | Meaning | Behavior |
|------|---------|----------|
| 0 | Success | Parse stdout as JSON output |
| 2 | System block | Block action; stderr becomes rejection reason; turn continues |
| Other | Warning | Non-fatal failure; CLI continues with warning |

[Official] Source: https://geminicli.com/docs/hooks/reference/

### Common Output Fields

| Field | Type | Description |
|-------|------|-------------|
| `systemMessage` | string | Displayed to user in terminal |
| `suppressOutput` | boolean | Hide hook metadata from logs |
| `continue` | boolean | `false` stops entire agent loop |
| `stopReason` | string | Displayed when `continue` is false |
| `decision` | string | `"allow"` or `"deny"` |
| `reason` | string | Feedback message for denials |

[Official] Source: https://geminicli.com/docs/hooks/reference/

### Environment Variables Available to Hooks

| Variable | Description |
|----------|-------------|
| `GEMINI_PROJECT_DIR` | Absolute path to project root |
| `GEMINI_SESSION_ID` | Unique session identifier |
| `GEMINI_CWD` | Current working directory |
| `CLAUDE_PROJECT_DIR` | Alias (compatibility with Claude Code) |

[Official] Source: https://geminicli.com/docs/hooks/writing-hooks/

### Base Input Schema (all hooks receive)

```json
{
  "session_id": "string",
  "transcript_path": "/absolute/path/to/transcript.json",
  "cwd": "/current/working/directory",
  "hook_event_name": "BeforeTool",
  "timestamp": "2026-01-15T10:30:00Z"
}
```

[Official] Source: https://geminicli.com/docs/hooks/reference/

### Hook Precedence

Merged from multiple layers (highest to lowest):
1. Project settings (`.gemini/settings.json`)
2. User settings (`~/.gemini/settings.json`)
3. Extensions (hooks defined by installed extensions)

[Official] Source: https://geminicli.com/docs/hooks/

---

## 6. Subagents (Local)

Specialized agents that operate within the main Gemini CLI session, handling complex tasks in isolated context loops. **Experimental feature** -- requires `experimental.enableAgents: true` in settings. [Official]

### File Format

- **Extension:** `.md` (Markdown with YAML frontmatter)
- **Encoding:** UTF-8

### Directory Structure

| Scope | Path |
|-------|------|
| Project | `<project-root>/.gemini/agents/*.md` |
| User | `~/.gemini/agents/*.md` |

[Official] Source: https://geminicli.com/docs/core/subagents/

### YAML Frontmatter Schema

```yaml
---
name: agent-identifier
description: What the agent does
kind: local
tools:
  - tool_name
  - mcp_*
model: gemini-3-flash-preview
temperature: 0.2
max_turns: 10
timeout_mins: 10
---
System prompt content goes here in the Markdown body.
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **Yes** | -- | Unique slug (lowercase, hyphens, underscores only) |
| `description` | string | **Yes** | -- | Short description for agent selection |
| `kind` | string | No | `"local"` | `"local"` or `"remote"` |
| `tools` | string[] | No | all tools | Specific tools or wildcards |
| `model` | string | No | inherited | Specific model to use |
| `temperature` | number | No | 1.0 | Temperature (0.0-2.0) |
| `max_turns` | number | No | 30 | Max conversation turns |
| `timeout_mins` | number | No | 10 | Execution time limit in minutes |

[Official] Source: https://geminicli.com/docs/core/subagents/

### Tool Wildcards

| Pattern | Matches |
|---------|---------|
| `*` | All available tools |
| `mcp_*` | All MCP server tools |
| `mcp_server-name_*` | Tools from a specific MCP server |

[Official] Source: https://geminicli.com/docs/core/subagents/

### Invocation

Use `@` prefix to direct a task to a specific subagent:

```
@codebase_investigator Map out the relationship between the auth module classes.
```

The model can also autonomously delegate to subagents. [Official]

### Built-in Subagents

| Name | Purpose |
|------|---------|
| `codebase_investigator` | Analyze and reverse-engineer code dependencies |
| `cli_help` | Gemini CLI documentation expertise |
| `generalist_agent` | Routes tasks to specialized subagents |
| `browser_agent` | Web automation using accessibility trees |

[Official] Source: https://geminicli.com/docs/core/subagents/

### Constraints

- Subagents run in isolated context loops (separate from main conversation) [Official]
- Subagents **cannot invoke other subagents** (prevents recursion and token bloat) [Official]

### Management Commands

| Command | Purpose |
|---------|---------|
| `/agents list` | View all registered agents |
| `/agents reload` | Refresh agent registry |
| `/agents enable <name>` | Activate an agent |
| `/agents disable <name>` | Deactivate an agent |

[Official] Source: https://geminicli.com/docs/core/subagents/

---

## 7. Subagents (Remote / A2A)

Remote agents accessed via the Agent-to-Agent (A2A) protocol. Same file format and directory structure as local subagents. **Experimental.** [Official]

### YAML Frontmatter Schema

```yaml
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

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `kind` | string | **Yes** | -- | Must be `"remote"` |
| `name` | string | **Yes** | -- | Unique slug |
| `agent_card_url` | string | **Yes** | -- | A2A agent card endpoint |
| `auth` | object | No | -- | Authentication config |

[Official] Source: https://geminicli.com/docs/core/remote-agents/

### Authentication Methods

**API Key:**
```yaml
auth:
  type: apiKey
  key: $MY_API_KEY
  name: X-API-Key  # optional header name
```

**HTTP Bearer Token:**
```yaml
auth:
  type: http
  scheme: Bearer
  token: $MY_BEARER_TOKEN
```

**HTTP Basic Auth:**
```yaml
auth:
  type: http
  scheme: Basic
  username: $MY_USERNAME
  password: $MY_PASSWORD
```

**Google Credentials:**
```yaml
auth:
  type: google-credentials
  scopes:
    - https://www.googleapis.com/auth/cloud-platform
```

**OAuth 2.0:**
```yaml
auth:
  type: oauth2
  client_id: my-client-id.apps.example.com
  client_secret: $OAUTH_SECRET
  scopes:
    - scope1
    - scope2
```

[Official] Source: https://geminicli.com/docs/core/remote-agents/

### Dynamic Value Resolution

| Syntax | Behavior |
|--------|----------|
| `$ENV_VAR` | Read from environment |
| `!command` | Execute shell command, use output |
| `literal` | Use as-is |
| `$$VAR` | Escape: becomes literal `$VAR` |

[Official] Source: https://geminicli.com/docs/core/remote-agents/

### Multi-Agent File

Multiple remote agents can be defined in a single file:

```yaml
---
- kind: remote
  name: remote-1
  agent_card_url: https://example.com/1
- kind: remote
  name: remote-2
  agent_card_url: https://example.com/2
---
```

[Official] Source: https://geminicli.com/docs/core/remote-agents/

---

## 8. Skills

On-demand specialized expertise that the model activates autonomously when relevant. Unlike context files (always loaded), skills use progressive disclosure -- only metadata loads at startup; full content loads upon activation. [Official]

### File Format

- **Required file:** `SKILL.md` (Markdown)
- **Encoding:** UTF-8
- **Structure:** Name, description (metadata for discovery), and body (procedural guidance)

### Directory Structure

Discovered from three tiers with precedence:

| Priority | Scope | Path(s) |
|----------|-------|---------|
| 1 (highest) | Workspace | `.gemini/skills/` or `.agents/skills/` |
| 2 | User | `~/.gemini/skills/` or `~/.agents/skills/` |
| 3 (lowest) | Extension | Bundled within installed extensions |

Within the same tier, `.agents/skills/` takes precedence over `.gemini/skills/`. [Official]

Each skill is a directory containing at minimum a `SKILL.md` file. The directory can also contain supporting files and resources that load when the skill is activated. [Official]

### How Skills Are Loaded

1. **Discovery:** Skill names and descriptions are injected into the system prompt at session start [Official]
2. **Activation:** Model calls the `activate_skill` built-in tool when a task matches a skill's description [Official]
3. **Consent:** User confirmation prompt shows skill name, purpose, and directory access [Official]
4. **Injection:** `SKILL.md` content and folder structure load into conversation history [Official]
5. **Execution:** Model proceeds with skill guidance [Official]

Source: https://geminicli.com/docs/cli/skills/

### Management Commands

**Interactive session:**

| Command | Purpose |
|---------|---------|
| `/skills list` | Display all discovered skills |
| `/skills link <path>` | Symlink skills from a directory |
| `/skills disable <name>` | Prevent skill usage |
| `/skills enable <name>` | Re-enable a skill |
| `/skills reload` | Refresh skill discovery |

**Terminal:**

| Command | Purpose |
|---------|---------|
| `gemini skills list` | View all skills |
| `gemini skills install <source>` | Install from Git/local/zip |
| `gemini skills uninstall <name>` | Remove a skill |
| `gemini skills link <path>` | Create symlinks |

Use `--scope workspace` for workspace-specific management. [Official]

Source: https://geminicli.com/docs/cli/skills/

---

## 9. Extensions

Packages that bundle prompts, MCP servers, custom commands, and skills into an installable unit.

### File Format

- **Manifest:** `gemini-extension.json` (JSON)
- **Encoding:** UTF-8

### Directory Structure

```
<home-or-project>/.gemini/extensions/<extension-name>/
  gemini-extension.json     # Required manifest
  GEMINI.md                  # Optional context file
  commands/                  # Optional custom commands
    deploy.toml
    gcs/
      sync.toml
  skills/                    # Optional skills
    SKILL.md
```

Extensions install to `~/.gemini/extensions/` (user-level) or `<project>/.gemini/extensions/` (project-level). [Official]

Source: https://google-gemini.github.io/gemini-cli/docs/extensions/

### Manifest Schema (`gemini-extension.json`)

```json
{
  "name": "my-extension",
  "version": "1.0.0",
  "mcpServers": {
    "my-server": {
      "command": "node my-server.js"
    }
  },
  "contextFileName": "GEMINI.md",
  "excludeTools": ["run_shell_command"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **Yes** | Identifier (lowercase, numbers, hyphens); must match directory name |
| `version` | string | No | Extension version |
| `mcpServers` | object | No | MCP servers loaded at startup (same schema as settings.json `mcpServers`) |
| `contextFileName` | string | No | Context file to load from extension directory (defaults to `GEMINI.md` if present) |
| `excludeTools` | string[] | No | Tools to block; supports command-specific restrictions like `"run_shell_command(rm -rf)"` |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/extensions/

### Variable Substitution

| Variable | Description |
|----------|-------------|
| `${extensionPath}` | Full filesystem path to extension directory |
| `${workspacePath}` | Current workspace path |
| `${/}` or `${pathSeparator}` | OS-specific path separator |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/extensions/

### Installation Methods

```bash
# From GitHub
gemini extensions install https://github.com/gemini-cli-extensions/security

# From local path
gemini extensions install /path/to/extension
```

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/extensions/

### Management Commands

| Command | Purpose |
|---------|---------|
| `gemini extensions install` | Install from GitHub URL or local path |
| `gemini extensions uninstall` | Remove extension |
| `gemini extensions disable/enable` | Control activation; supports `--scope=workspace` |
| `gemini extensions update` | Sync with source; `--all` updates everything |
| `gemini extensions link` | Create symbolic link for development |
| `gemini extensions new` | Generate boilerplate from examples |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/extensions/

### Conflict Resolution

- Extension commands have lowest precedence [Official]
- Naming conflicts cause prefixing: `/gcp.deploy` (extension) vs `/deploy` (user/project) [Official]
- MCP `excludeTools` arrays are unioned (either source blocking = blocked) [Official]
- MCP `includeTools` arrays are intersected (both sources must allow) [Official]
- `excludeTools` always takes precedence over `includeTools` [Official]

---

## 10. Ignore Files (.geminiignore)

Controls which files and directories the Gemini CLI agent can access, independent of version control.

### File Format

- **Filename:** `.geminiignore`
- **Encoding:** UTF-8
- **Syntax:** Same as `.gitignore` (glob patterns, `#` comments, blank lines ignored) [Official]

### Directory Structure

Placed alongside `.gitignore` in the project root or subdirectories. [Official]

### Behavior

- By default, Gemini CLI also respects `.gitignore` rules [Official]
- `.geminiignore` provides additional exclusions independent of version control [Official]
- Both `respectGitIgnore` and `respectGeminiIgnore` are configurable in `settings.json` under `context.fileFiltering` [Official]
- Currently, `.geminiignore` cannot un-ignore items from `.gitignore` (no negative pattern support across files) [Community -- GitHub issue #5259]

### Example

```gitignore
# Exclude large datasets
data/
*.csv

# Exclude build artifacts not in .gitignore
.cache/
*.wasm

# Exclude sensitive configs
secrets/
.env.production
```

Source: https://geminicli.com/docs/cli/gemini-ignore/

---

## 11. Environment Files (.env)

Environment variable files for configuring API keys and runtime settings.

### File Format

- **Filename:** `.env`
- **Encoding:** UTF-8
- **Syntax:** `KEY=VALUE` pairs, one per line

### Discovery Order

The CLI searches upward from the current directory, preferring `.gemini/.env` over `.env` at each level: [Official]

1. `<current-dir>/.gemini/.env`
2. `<current-dir>/.env`
3. `<parent-dir>/.gemini/.env`
4. `<parent-dir>/.env`
5. ... up to `~/.env`

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html

### Key Environment Variables

| Variable | Purpose |
|----------|---------|
| `GEMINI_API_KEY` | API authentication |
| `GEMINI_MODEL` | Default model override |
| `GOOGLE_API_KEY` | Google Cloud API key |
| `GOOGLE_CLOUD_PROJECT` | GCP project ID |
| `GOOGLE_APPLICATION_CREDENTIALS` | Service account key file path |
| `GOOGLE_CLOUD_LOCATION` | GCP region (e.g., `us-central1`) |
| `GEMINI_SANDBOX` | Enable/configure sandbox |
| `SEATBELT_PROFILE` | macOS sandbox profile |
| `NO_COLOR` | Disable colored output |
| `CLI_TITLE` | Custom CLI window title |
| `CODE_ASSIST_ENDPOINT` | Development endpoint override |

[Official] Source: https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html

---

## 12. Built-in Tools Reference

These are the tools available to the Gemini model by default. They are not user-configurable content types, but are relevant for understanding the `tools.core`, `tools.exclude`, and hook matcher patterns.

### Complete Tool List

| Display Name | Internal Name | Kind | Description |
|-------------|---------------|------|-------------|
| Shell | `run_shell_command` | Execute | Execute shell commands (requires confirmation) |
| ReadFile | `read_file` | Read | Read file content (text, images, audio, PDF) |
| ReadFolder | `list_directory` | Read | List files and subdirectories |
| Multi-File Read | `read_many_files` | Read | Read/concatenate multiple files (triggered by `@`) |
| FindFiles | `glob` | Search | Find files by glob pattern |
| SearchText | `grep_search` / `search_file_content` | Search | Regex search within file contents |
| WriteFile | `write_file` | Edit | Create or overwrite files (requires confirmation) |
| Edit | `replace` | Edit | Precise text replacement within files (requires confirmation) |
| GoogleSearch | `google_web_search` | Search | Google Search for current information |
| WebFetch | `web_fetch` | Fetch | Retrieve and process URL content |
| SaveMemory | `save_memory` | Think | Persist facts to GEMINI.md |
| Skill Activation | `activate_skill` | Other | Load specialized skill expertise |
| Internal Docs | `get_internal_docs` | Think | Access CLI's own documentation |
| Plan Mode Enter | `enter_plan_mode` | Plan | Switch to read-only planning mode |
| Plan Mode Exit | `exit_plan_mode` | Plan | Finalize plan and request approval |
| Ask User | `ask_user` | Communicate | Request clarification from user |
| Todos | `write_todos` | Other | Track internal subtask progress |
| Complete Task | `complete_task` | Other | Return subagent result to parent (system only) |
| Codebase Investigator | `codebase_investigator` | Agent | Analyze code dependencies (built-in subagent) |

[Official] Source: https://geminicli.com/docs/reference/tools/

### Tool Configuration in settings.json

```json
{
  "tools": {
    "core": ["run_shell_command(git)", "read_file"],
    "exclude": ["web_fetch"],
    "allowed": ["read_file", "glob"]
  }
}
```

- `core`: Allowlist restricting which built-in tools are available. Supports command-specific restrictions like `"run_shell_command(git)"`. [Official]
- `exclude`: Blocklist of tools to disable entirely. [Official]
- `allowed`: Tools that bypass confirmation prompts (auto-approve). [Official]

### Manual Trigger Syntax

| Prefix | Tool Triggered | Example |
|--------|---------------|---------|
| `@` | `read_many_files` | `@src/main.ts` |
| `!` | `run_shell_command` | `!git status` |

[Official] Source: https://geminicli.com/docs/reference/tools/

---

## Sources

- [Official Configuration Docs](https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html)
- [GEMINI.md Context Files](https://geminicli.com/docs/cli/gemini-md/)
- [MCP Server Configuration](https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html)
- [Custom Commands](https://geminicli.com/docs/cli/custom-commands/)
- [Custom Commands Blog Post](https://cloud.google.com/blog/topics/developers-practitioners/gemini-cli-custom-slash-commands)
- [Hooks Overview](https://geminicli.com/docs/hooks/)
- [Hooks Reference](https://geminicli.com/docs/hooks/reference/)
- [Writing Hooks](https://geminicli.com/docs/hooks/writing-hooks/)
- [Subagents](https://geminicli.com/docs/core/subagents/)
- [Remote Subagents](https://geminicli.com/docs/core/remote-agents/)
- [Skills](https://geminicli.com/docs/cli/skills/)
- [Extensions](https://google-gemini.github.io/gemini-cli/docs/extensions/)
- [Ignoring Files](https://geminicli.com/docs/cli/gemini-ignore/)
- [Tools Reference](https://geminicli.com/docs/reference/tools/)
- [GitHub Repository](https://github.com/google-gemini/gemini-cli)
- [Gemini CLI Cheatsheet](https://www.philschmid.de/gemini-cli-cheatsheet)
