# OpenCode Content Types

## Overview

OpenCode supports several content types for customization: rules/instructions, agents, commands, skills, custom tools, MCP server configurations, themes, and plugins. Configuration uses JSON/JSONC format with a published schema. [Official]

Source: https://opencode.ai/docs/config/

## Settings / Configuration

### Format
JSON or JSONC (JSON with Comments). Schema available at `https://opencode.ai/config.json`.

### File Locations (merge order, later overrides earlier)
1. Remote config (`.well-known/opencode`) -- organizational defaults
2. Global config (`~/.config/opencode/opencode.json`)
3. Custom path (`OPENCODE_CONFIG` env var)
4. Project config (`opencode.json` in project root)
5. `.opencode` directory contents
6. Inline config (`OPENCODE_CONFIG_CONTENT` env var)

TUI settings use separate `tui.json` files at the same locations.

### Core Fields

| Field                 | Type          | Purpose                                           |
|-----------------------|---------------|---------------------------------------------------|
| `$schema`             | string        | Schema URL for validation                         |
| `model`               | string        | Primary LLM model (e.g., `"anthropic/claude-sonnet-4-5"`) |
| `small_model`         | string        | Model for lightweight tasks                       |
| `default_agent`       | string        | Agent used when none specified                    |
| `provider`            | object        | Provider config (timeouts, options)               |
| `tools`               | object        | Enable/disable tools by name/glob                 |
| `permission`          | object/string | Tool approval rules                               |
| `mcp`                 | object        | MCP server definitions                            |
| `agent`               | object        | Inline agent definitions                          |
| `command`             | object        | Inline command definitions                        |
| `instructions`        | array         | Glob paths to instruction files                   |
| `server`              | object        | Port, hostname, mDNS, CORS                        |
| `autoupdate`          | bool/string   | Auto-update behavior                              |
| `snapshot`            | boolean       | File change tracking for undo                     |
| `share`               | string        | `"manual"`, `"auto"`, or `"disabled"`             |
| `formatter`           | object        | Code formatter configurations                     |
| `watcher`             | object        | File watching ignore patterns                     |
| `compaction`          | object        | Context management settings                       |
| `plugin`              | array         | NPM plugins to load                               |
| `disabled_providers`  | array         | Provider blocklist                                |
| `enabled_providers`   | array         | Provider allowlist                                |
| `experimental`        | object        | Unstable feature flags                            |

### Variable Substitution
- `{env:VAR_NAME}` -- environment variable expansion
- `{file:path}` -- file content injection

[Official] Source: https://opencode.ai/docs/config/

## Rules / Instructions

### Primary Format
`AGENTS.md` -- markdown file with custom instructions for the LLM.

### File Locations (searched in order)
1. Project root: `./AGENTS.md`
2. Parent directories (traverses up to git root)
3. Global: `~/.config/opencode/AGENTS.md`

### Legacy Compatibility
OpenCode reads these Claude Code files as fallbacks:
- `CLAUDE.md` (project level)
- `~/.claude/CLAUDE.md` (global level)

Disable with `OPENCODE_DISABLE_CLAUDE_CODE` environment variable.

### Config-based Instructions
In `opencode.json`, the `instructions` array accepts glob paths and remote URLs:

```json
{
  "instructions": ["CONTRIBUTING.md", "docs/guidelines.md", ".cursor/rules/*.md"]
}
```

Remote instructions fetch with a 5-second timeout.

### Setup
Use `/init` command to auto-generate an `AGENTS.md` by scanning project structure. Should be committed to version control.

[Official] Source: https://opencode.ai/docs/rules/

## Commands

Custom prompts invoked via `/` prefix in the TUI (e.g., `/my-command`).

### Definition Methods
1. **Markdown files**: `.opencode/commands/` (project) or `~/.config/opencode/commands/` (global). Filename = command name.
2. **JSON config**: Under the `command` key in `opencode.json`.

### Configuration Options
| Option        | Type    | Purpose                                    |
|---------------|---------|--------------------------------------------|
| `template`    | string  | Prompt sent to LLM (required)              |
| `description` | string  | Shown in TUI                               |
| `agent`       | string  | Which agent executes the command            |
| `model`       | string  | Override default model                      |
| `subtask`     | boolean | Force subagent invocation                   |

### Template Placeholders
- `$ARGUMENTS` -- all passed arguments
- `$1`, `$2`, `$3` -- positional arguments
- `` !`command` `` -- injects bash command output
- `@filename` -- includes file content

### Built-in Commands
`/init`, `/undo`, `/redo`, `/share`, `/help`. Custom commands can override built-ins.

[Official] Source: https://opencode.ai/docs/commands/

## MCP Server Configuration

Defined in `opencode.json` under the `mcp` key.

### Local Servers

```json
{
  "mcp": {
    "my-local-mcp": {
      "type": "local",
      "command": ["npx", "-y", "my-mcp-command"],
      "environment": {
        "MY_ENV_VAR": "value"
      },
      "enabled": true,
      "timeout": 5000
    }
  }
}
```

### Remote Servers

```json
{
  "mcp": {
    "my-remote-mcp": {
      "type": "remote",
      "url": "https://my-mcp-server.com",
      "headers": {
        "Authorization": "Bearer API_KEY"
      },
      "oauth": {
        "clientId": "{env:CLIENT_ID}",
        "clientSecret": "{env:CLIENT_SECRET}",
        "scope": "tools:read tools:execute"
      }
    }
  }
}
```

### OAuth Support
Automatic Dynamic Client Registration (RFC 7591). Can pre-register credentials or disable with `oauth: false`.

Management commands: `opencode mcp auth`, `opencode mcp list`, `opencode mcp logout`, `opencode mcp debug`.

Token storage: `~/.local/share/opencode/mcp-auth.json`.

### Tool Availability Control
Disable MCP tools globally or per-agent using glob patterns in `tools` config.

[Official] Source: https://opencode.ai/docs/mcp-servers/

## Themes

Custom themes are stored in:
- `.opencode/themes/` (project)
- `~/.config/opencode/themes/` (global)

[Official] Source: https://opencode.ai/docs/ (navigation listing)

## Plugins

NPM packages or local TypeScript/JavaScript files that extend OpenCode with custom tools, hooks, and integrations.

### Locations
- `.opencode/plugins/` (project)
- `~/.config/opencode/plugins/` (global)
- NPM packages via `plugin` config array

### Dependencies
Add `package.json` to `.opencode/` directory; OpenCode runs `bun install` at startup.

[Official] Source: https://opencode.ai/docs/plugins/

## Directory Structure Summary

```
~/.config/opencode/
  opencode.json          # Global settings (JSONC)
  tui.json               # TUI-specific settings
  AGENTS.md              # Global instructions/rules
  agents/                # Global agent definitions (.md)
  commands/              # Global custom commands (.md)
  skills/                # Global skills (subdirs with SKILL.md)
  tools/                 # Global custom tools (.ts/.js)
  plugins/               # Global plugins (.ts/.js)
  themes/                # Global themes

.opencode/               # Project-level (same structure)
  opencode.json          # Project settings (JSONC)
  agents/
  commands/
  skills/
  tools/
  plugins/
  themes/

opencode.json            # Project root config (JSONC)
AGENTS.md                # Project root instructions
```

Note: Singular directory names (e.g., `agent/`) are supported for backwards compatibility. [Official]

## Content Type Comparison with Syllago

| Syllago Type | OpenCode Equivalent     | Format           | Notes                               |
|--------------|-------------------------|------------------|-------------------------------------|
| Rules        | AGENTS.md / instructions| Markdown         | Also reads CLAUDE.md as fallback    |
| Skills       | Skills (SKILL.md)       | Markdown + YAML  | Frontmatter with name, description  |
| Agents       | Agents                  | MD or JSON       | Primary agents + subagents          |
| Commands     | Commands                | MD or JSON       | Slash commands with templates       |
| Hooks        | Plugins (events)        | TypeScript/JS    | Event-based, not config-based       |
| MCP          | MCP servers             | JSONC in config  | Local + remote with OAuth           |
| Prompts      | Commands (templates)    | Markdown         | Closest equivalent                  |
