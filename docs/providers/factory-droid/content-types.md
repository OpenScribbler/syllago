# Factory Droid Content Types

Factory Droid (Factory AI's coding agent CLI, `factory-droid`) supports multiple
content types for configuring agent behavior, extending capabilities, and
integrating external tools.

**Identity note:** "Factory Droid" refers to Factory AI's coding agent CLI
(`factory-droid`). No known naming conflicts with other tools as of 2026-03-30.

## Overview

| Content Type | Config Location | Format | Merge Strategy |
|---|---|---|---|
| Rules | `AGENTS.md` (project root) | Markdown | Filesystem |
| Skills | `.factory/skills/<name>/SKILL.md` | Markdown + YAML frontmatter | Filesystem |
| Custom Droids | `.factory/droids/<name>.md` | Markdown + YAML frontmatter | Filesystem |
| Hooks | `.factory/settings.json` `hooks` key | JSON | JSON merge |
| MCP | `.factory/mcp.json` | JSON | JSON merge |
| Commands | `.factory/commands/<name>.md` | Markdown (or shebang script) | Filesystem |
| Plugins | Via plugins marketplace | Varies | [Unverified] |

## Rules (AGENTS.md)

Project-level instructions are provided through `AGENTS.md` in the project root.
This is a plain Markdown file that acts as a briefing for the agent -- covering
build commands, architecture, security, git workflows, and coding conventions.

`AGENTS.md` is a cross-provider convention also supported by Cursor, Windsurf,
and other agents.

[Official: https://docs.factory.ai/cli/configuration]

Factory Droid does not appear to support `.cursor/rules/` style MDC rule files
with frontmatter activation modes. [Unverified]

## Skills

Reusable capabilities stored as Markdown files with YAML frontmatter.

**Locations:**

| Scope | Path |
|---|---|
| Workspace | `.factory/skills/<skill-name>/SKILL.md` |
| Personal | `~/.factory/skills/<skill-name>/SKILL.md` |
| Legacy | `.agent/skills/<skill-name>/SKILL.md` |

**Frontmatter fields:**

| Field | Required | Default | Description |
|---|---|---|---|
| `name` | No | Directory name | Identifier (lowercase, alphanumeric, hyphens) |
| `description` | Recommended | -- | Usage context; guides agent invocation |
| `user-invocable` | No | `true` | `false` = agent-only, hidden from slash commands |
| `disable-model-invocation` | No | `false` | `true` = user-only via slash command |

Skills can include supporting files (`references.md`, `schemas/`, `checklists.md`).

Invoked via `/skill-name` or automatically by the agent when relevant to the task.

[Official: https://docs.factory.ai/cli/configuration/skills]

## Custom Droids (Subagents)

Specialized subagents defined as Markdown files. See
[skills-agents.md](skills-agents.md) for full details.

**Locations:**

| Scope | Path |
|---|---|
| Project | `.factory/droids/<name>.md` |
| Personal | `~/.factory/droids/<name>.md` |

[Official: https://docs.factory.ai/cli/configuration/custom-droids]

## Hooks

Lifecycle hooks configured in the `hooks` key of `settings.json`. Same JSON
schema as Claude Code. See [hooks.md](hooks.md) for full details.

**Config file:** `.factory/settings.json`

[Official: https://docs.factory.ai/cli/configuration/hooks-guide]

## MCP (Model Context Protocol)

MCP server configuration in a dedicated JSON file.

**Locations:**

| Scope | Path |
|---|---|
| User | `~/.factory/mcp.json` |
| Project | `.factory/mcp.json` |

User-level settings take priority when both levels define the same server.

**Format:**

```json
{
  "mcpServers": {
    "server-name": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@example/server"],
      "env": { "API_KEY": "..." }
    }
  }
}
```

**Transports:** `stdio` (local processes) and `http` (remote endpoints with
`url` and `headers` fields).

**Per-server controls:** `disabled` (boolean) and `disabledTools` (string array)
for selective toggling.

[Official: https://docs.factory.ai/cli/configuration/mcp] (URL returned 404;
data sourced from llms.txt index page which rendered MCP content)

## Custom Slash Commands

Reusable prompts or scripts invoked from chat.

**Locations:**

| Scope | Path |
|---|---|
| Workspace | `.factory/commands/<name>.md` |
| Personal | `~/.factory/commands/<name>.md` |

Workspace commands override personal ones with identical names. Only top-level
files register (no nested directories).

**Markdown commands** use optional YAML frontmatter with `description`,
`argument-hint`, and `allowed-tools` keys.

**Executable commands** require a shebang line and receive arguments as
positional parameters. Output (up to 64 KB) is posted to the chat.

[Official: https://docs.factory.ai/cli/configuration/custom-slash-commands]

## Sources

- [Factory Droid Configuration](https://docs.factory.ai/cli/configuration) [Official]
- [Factory Droid Skills](https://docs.factory.ai/cli/configuration/skills) [Official]
- [Factory Droid Custom Droids](https://docs.factory.ai/cli/configuration/custom-droids) [Official]
- [Factory Droid Hooks Guide](https://docs.factory.ai/cli/configuration/hooks-guide) [Official]
- [Factory Droid Custom Slash Commands](https://docs.factory.ai/cli/configuration/custom-slash-commands) [Official]
- [Factory Droid llms.txt](https://docs.factory.ai/llms.txt) [Official]
