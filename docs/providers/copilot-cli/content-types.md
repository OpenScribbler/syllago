# Copilot CLI — Content Type Configurations

Research date: 2026-03-21

## 1. Instructions

Copilot CLI supports four instruction scopes, all in Markdown format.

### 1a. Repository-Wide Instructions

- **File**: `.github/copilot-instructions.md`
- **Format**: Markdown (plain text, no frontmatter)
- **Scope**: All requests within a repository
- **Behavior**: Automatically attached to every prompt. Whitespace between instructions is ignored.
- **Notes**: The `/init` slash command auto-generates a starter file.

`[Official]` [Adding custom instructions for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions)

### 1b. Path-Specific Instructions

- **Files**: `NAME.instructions.md` inside `.github/instructions/` (recursive subdirectories allowed)
- **Format**: Markdown with YAML frontmatter
- **Required frontmatter**: `applyTo` (glob pattern)
- **Optional frontmatter**: `excludeAgent` (`"code-review"` or `"coding-agent"`)
- **Scope**: Requests involving files matching the `applyTo` glob
- **Behavior**: Combined with repository-wide instructions when both apply

**Glob examples:**
| Pattern | Matches |
|---|---|
| `*` | Files in current directory |
| `**/*.py` | All Python files recursively |
| `src/**/*.py` | Python files within `src/` |

**Example** (`.github/instructions/python-style.instructions.md`):
```markdown
---
applyTo: "**/*.py"
---

Use type hints on all function signatures.
Prefer dataclasses over plain dicts for structured data.
```

`[Official]` [Adding custom instructions for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions)

### 1c. Agent Instructions (AGENTS.md)

- **Files**: `AGENTS.md`, `CLAUDE.md`, or `GEMINI.md`
- **Format**: Markdown (plain text, no frontmatter)
- **Locations**:
  - Repository root (treated as **primary** instructions — higher influence)
  - Current working directory
  - Directories listed in `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` env var (comma-separated)
- **Notes**: `CLAUDE.md` and `GEMINI.md` must be at repository root. Copilot CLI reads all three filenames.

`[Official]` [Adding custom instructions for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions)

### 1d. Personal/Local Instructions

- **File**: `$HOME/.copilot/copilot-instructions.md`
- **Format**: Markdown (plain text)
- **Scope**: All repositories for this user
- **Alternative**: Set `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` to a comma-separated list of directories. Copilot CLI looks for `AGENTS.md` and `.github/instructions/**/*.instructions.md` in each.

`[Official]` [Adding custom instructions for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions)

---

## 2. MCP Server Configuration

### File Locations

| Scope | File |
|---|---|
| Global (persistent) | `~/.copilot/mcp-config.json` |
| Project-local | `.copilot/mcp-config.json` (in project root) |
| Session-only | `--additional-mcp-config PATH` flag |

The global config path can be changed via `COPILOT_HOME` env var.

### JSON Schema

```json
{
  "mcpServers": {
    "SERVER-NAME": { ... }
  }
}
```

### Server Types & Fields

**STDIO/Local servers:**

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | `"local"` or `"stdio"` | Yes | Server transport type |
| `command` | string | Yes | Executable to start |
| `args` | string[] | No | Command arguments |
| `env` | object | No | Environment variables |
| `tools` | string or string[] | No | `"*"` for all, or specific tool names. Omit for all. |

**HTTP servers (Streamable HTTP):**

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | `"http"` | Yes | Server transport type |
| `url` | string | Yes | Remote endpoint URL |
| `headers` | object | No | HTTP headers (auth tokens, etc.) |
| `tools` | string or string[] | No | Tool filter |

**SSE servers (deprecated):**

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | `"sse"` | Yes | Server transport type |
| `url` | string | Yes | Remote endpoint URL |
| `headers` | object | No | HTTP headers |
| `tools` | string or string[] | No | Tool filter |

**Additional fields:**
- `filterMapping` — Controls how MCP tool output is processed. Referenced in CLI command reference but schema details are not publicly documented.

`[Official]` [Adding MCP servers for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers)
`[Official]` [Configure GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/configure-copilot-cli)

### Full Example

```json
{
  "mcpServers": {
    "playwright": {
      "type": "local",
      "command": "npx",
      "args": ["@playwright/mcp@latest"],
      "env": {},
      "tools": ["*"]
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

### Management Commands

| Command | Description |
|---|---|
| `/mcp add` | Interactive server setup |
| `/mcp show` | List all servers and status |
| `/mcp show NAME` | View server details |
| `/mcp edit NAME` | Edit a server config |
| `/mcp delete NAME` | Remove a server |
| `/mcp disable NAME` | Disable without removing |
| `/mcp enable NAME` | Re-enable a disabled server |

### CLI Flags

| Flag | Description |
|---|---|
| `--additional-mcp-config PATH` | Add servers for one session |
| `--disable-builtin-mcps` | Disable all built-in MCP servers |
| `--disable-mcp-server NAME` | Disable a specific server |

`[Official]` [CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference)

---

## 3. Skills

Skills are markdown-based instruction packages that Copilot loads contextually.

### File Format

- **File**: `SKILL.md` (must be named exactly this)
- **Format**: Markdown with YAML frontmatter
- **Container**: Each skill lives in its own directory

### Frontmatter Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Lowercase, hyphens for spaces. Typically matches directory name. |
| `description` | string | Yes | What the skill does and when to use it |
| `license` | string | No | Applicable license |

### Directory Structure

**Project skills (repository-specific):**
```
.github/skills/<skill-name>/SKILL.md
.claude/skills/<skill-name>/SKILL.md
```

**Personal skills (cross-project):**
```
~/.copilot/skills/<skill-name>/SKILL.md
~/.claude/skills/<skill-name>/SKILL.md
```

**Cross-tool compatible:**
```
.agents/skills/<skill-name>/SKILL.md
```

### Body Content

The Markdown body below frontmatter contains:
- Step-by-step instructions
- Tool/resource usage guidance
- Examples demonstrating the skill
- Scripts or supplementary files can be co-located in the skill directory

### Invocation

Use `/skill-name` in a prompt, or Copilot auto-loads relevant skills based on the `description` field.

### Cross-Provider Compatibility

Skills in `.claude/skills/` are automatically picked up by Copilot CLI. The Agent Skills spec is an open standard used across Copilot CLI, Copilot coding agent, and VS Code.

### Example

```
.github/skills/frontend-design/SKILL.md
```

```markdown
---
name: frontend-design
description: Use this skill when creating or modifying React UI components. Provides design system conventions and accessibility guidelines.
---

## Guidelines

1. Use the project's design tokens from `src/tokens/`
2. All interactive elements must have ARIA labels
3. Prefer CSS modules over inline styles
```

`[Official]` [Creating agent skills for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-skills)
`[Official]` [About agent skills](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills)

---

## 4. Custom Agents

Custom agents are specialized Copilot profiles for particular workflows.

### File Format

- **File**: `<agent-name>.agent.md`
- **Format**: Markdown with YAML frontmatter
- **Filename constraints**: Only `.`, `-`, `_`, `a-z`, `A-Z`, `0-9`
- **Max prompt content**: 30,000 characters

### Directory Structure

| Scope | Location |
|---|---|
| Repository | `.github/agents/<name>.agent.md` |
| Organization/Enterprise | `.github-private/agents/<name>.agent.md` |

### Frontmatter Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | No | Identifier (defaults to filename without `.agent.md`) |
| `description` | string | Yes | Brief explanation of agent capabilities |
| `tools` | string[] | No | Tool names/aliases. Omit for all tools. Empty `[]` disables all. |
| `mcp-servers` | object | No | MCP server config specific to this agent |
| `model` | string | No | AI model to use (supported in VS Code, JetBrains, Eclipse, Xcode) |
| `target` | string | No | `"vscode"` or `"github-copilot"` to restrict availability. Omit for both. |

### Tool References

Tools can reference built-in tools or MCP server tools:
```yaml
tools: ["read", "edit", "search", "some-mcp-server/tool-1"]
```

### Example

```markdown
---
name: api-reviewer
description: Reviews API endpoint implementations for REST conventions, error handling, and security best practices.
tools: ["read", "search"]
model: gpt-4o
---

You are an API review specialist. When asked to review code:

1. Check REST naming conventions
2. Verify error responses use standard HTTP status codes
3. Ensure authentication middleware is applied
4. Flag any SQL injection or XSS vulnerabilities
```

`[Official]` [Creating custom agents for Copilot coding agent](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/create-custom-agents)
`[Official]` [Custom agents configuration](https://docs.github.com/en/copilot/reference/custom-agents-configuration)

---

## 5. Hooks

Hooks execute custom shell commands at lifecycle points during agent sessions.

### File Location

| Context | Location |
|---|---|
| Copilot CLI | `.github/hooks/<name>.json` in **current working directory** |
| Coding agent | `.github/hooks/<name>.json` on repository's **default branch** |

Key difference: CLI loads hooks from the local working directory; coding agent requires them on the default branch.

`[Official]` [Using hooks with GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/use-hooks)

### Root Configuration Schema

```json
{
  "version": 1,
  "hooks": {
    "sessionStart": [...],
    "sessionEnd": [...],
    "userPromptSubmitted": [...],
    "preToolUse": [...],
    "postToolUse": [...],
    "errorOccurred": [...]
  }
}
```

### Hook Definition Object

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | `"command"` | Yes | Hook type |
| `bash` | string | Conditional | Script path or inline command (Linux/macOS) |
| `powershell` | string | Conditional | Script path or inline command (Windows) |
| `cwd` | string | No | Working directory for the script |
| `timeoutSec` | number | No | Timeout in seconds (default: 30) |
| `comment` | string | No | Human-readable description |

At least one of `bash` or `powershell` is required.

### Event Types & Input Schemas

**sessionStart** — New/resumed session:
```json
{
  "timestamp": 1234567890,
  "cwd": "/path/to/project",
  "source": "new" | "resume" | "startup",
  "initialPrompt": "optional user prompt"
}
```
Output: ignored.

**sessionEnd** — Session completes:
```json
{
  "timestamp": 1234567890,
  "cwd": "/path/to/project",
  "reason": "complete" | "error" | "abort" | "timeout" | "user_exit"
}
```
Output: ignored.

**userPromptSubmitted** — User enters a prompt:
```json
{
  "timestamp": 1234567890,
  "cwd": "/path/to/project",
  "prompt": "exact user text"
}
```
Output: ignored.

**preToolUse** — Before tool execution (can block):
```json
{
  "timestamp": 1234567890,
  "cwd": "/path/to/project",
  "toolName": "bash",
  "toolArgs": "{\"command\":\"rm -rf /\"}"
}
```
Output (optional):
```json
{
  "permissionDecision": "allow" | "deny" | "ask",
  "permissionDecisionReason": "Reason string"
}
```
This is the **only** hook that can block agent actions.

**postToolUse** — After tool execution:
```json
{
  "timestamp": 1234567890,
  "cwd": "/path/to/project",
  "toolName": "bash",
  "toolArgs": "{\"command\":\"ls\"}",
  "toolResult": {
    "resultType": "success" | "failure" | "denied",
    "textResultForLlm": "output text"
  }
}
```
Output: ignored.

**errorOccurred** — Error during session:
```json
{
  "timestamp": 1234567890,
  "cwd": "/path/to/project",
  "error": {
    "message": "error description",
    "name": "ErrorType",
    "stack": "trace if available"
  }
}
```
Output: ignored.

### Script I/O

Hooks receive JSON input via **stdin** and emit JSON output via **stdout**.

```bash
#!/bin/bash
INPUT=$(cat)
TOOL=$(echo "$INPUT" | jq -r '.toolName')
if [ "$TOOL" = "bash" ]; then
  echo '{"permissionDecision":"deny","permissionDecisionReason":"bash blocked by policy"}'
fi
```

### Execution Model

- Multiple hooks of the same event type execute **sequentially** in array order
- Default timeout: 30 seconds (configurable via `timeoutSec`)
- Exit code 0 = success; non-zero = failure

`[Official]` [Hooks configuration reference](https://docs.github.com/en/copilot/reference/hooks-configuration)
`[Official]` [About hooks](https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks)

---

## 6. Prompt Files

Prompt files are reusable, invocable prompt templates (slash commands).

- **Files**: `<name>.prompt.md` in `.github/prompts/`
- **Format**: Markdown with optional YAML frontmatter
- **Invocation**: `/promptname` in chat

### Frontmatter Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `description` | string | No | Brief description of the prompt |
| `model` | string | No | AI model to use |
| `tools` | string[] | No | Tools the prompt can use |
| `agent` | string | No | e.g. `"agent"` for agent mode |

### Body Features

- Standard Markdown instructions
- File references via Markdown links (relative paths)
- Tool references via `#tool:<tool-name>` syntax
- Variable interpolation via `${input:variableName}` or `${input:variableName:placeholder}`

### CLI Support

Prompt files are **primarily a VS Code / IDE feature** (public preview). Copilot CLI support for `.prompt.md` files is not confirmed in official CLI docs.

`[Official]` [Prompt files](https://docs.github.com/en/copilot/tutorials/customization-library/prompt-files)
`[Unverified]` CLI support for `.prompt.md` — not confirmed in CLI-specific documentation.

---

## 7. General CLI Configuration

### config.json

- **Location**: `~/.copilot/config.json` (override via `COPILOT_HOME`)
- **Format**: JSON

| Field | Type | Description |
|---|---|---|
| `trusted_folders` | string[] | Directories where Copilot can read/modify/execute files |

### Environment Variables

| Variable | Description |
|---|---|
| `COPILOT_HOME` | Override `~/.copilot` config directory location |
| `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` | Comma-separated directories to search for `AGENTS.md` and `.instructions.md` files |

`[Official]` [Configure GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/configure-copilot-cli)

---

## Summary Table

| Content Type | File(s) | Format | Location(s) |
|---|---|---|---|
| Instructions (repo) | `copilot-instructions.md` | Markdown | `.github/` |
| Instructions (path) | `*.instructions.md` | Markdown + YAML frontmatter | `.github/instructions/` |
| Instructions (agent) | `AGENTS.md` / `CLAUDE.md` / `GEMINI.md` | Markdown | Repo root, cwd, `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` |
| Instructions (personal) | `copilot-instructions.md` | Markdown | `~/.copilot/` |
| MCP servers | `mcp-config.json` | JSON | `~/.copilot/`, `.copilot/`, CLI flag |
| Skills | `SKILL.md` | Markdown + YAML frontmatter | `.github/skills/`, `~/.copilot/skills/`, `.claude/skills/` |
| Agents | `*.agent.md` | Markdown + YAML frontmatter | `.github/agents/`, `.github-private/agents/` |
| Hooks | `*.json` | JSON | `.github/hooks/` (cwd for CLI, default branch for coding agent) |
| Prompts | `*.prompt.md` | Markdown + YAML frontmatter | `.github/prompts/` (IDE only, CLI support unverified) |
| Config | `config.json` | JSON | `~/.copilot/` |
