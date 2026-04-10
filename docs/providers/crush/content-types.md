# Crush Content Types Reference

Comprehensive documentation of all content types supported by Crush, including
file formats, directory structures, configuration schemas, and loading behavior.

**Last updated:** 2026-03-30

**Sources:**
- [Official] https://github.com/charmbracelet/crush
- [Official] https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json
- [Community] https://deepwiki.com/charmbracelet/crush

---

## Table of Contents

1. [Configuration (crush.json)](#1-configuration)
2. [Rules (AGENTS.md)](#2-rules)
3. [MCP Servers (crush.json mcp key)](#3-mcp-servers)
4. [Skills (SKILL.md)](#4-skills)
5. [Content Types NOT Supported](#5-content-types-not-supported)

---

## 1. Configuration

Crush uses JSON configuration files with hierarchical merging. [Official:
https://github.com/charmbracelet/crush]

### File Locations

Configuration is loaded from the first file found in this priority order:

| Priority | Path | Scope |
|----------|------|-------|
| 1 (highest) | `.crush.json` | Project-local (gitignored) |
| 2 | `crush.json` | Project (committed to VCS) |
| 3 | `$HOME/.config/crush/crush.json` | Global user config |

Data storage (sessions, etc.) uses a separate path:

| Platform | Path |
|----------|------|
| Unix | `$HOME/.local/share/crush/crush.json` |
| Windows | `%LOCALAPPDATA%\crush\crush.json` |

Environment overrides: `CRUSH_GLOBAL_CONFIG`, `CRUSH_GLOBAL_DATA` [Official:
https://github.com/charmbracelet/crush]

### JSON Schema

A machine-readable JSON Schema is published at:
- Repo: `https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json`
- CDN: `https://charm.land/crush.json`

[Official: https://github.com/charmbracelet/crush]

### Top-Level Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `$schema` | string | No | Schema reference URL |
| `models` | object | No | Model configurations keyed by role ("large", "small") |
| `providers` | object | No | AI provider configurations (API keys, endpoints) |
| `mcp` | object | No | MCP server configurations |
| `lsp` | object | No | Language Server Protocol configurations |
| `options` | object | No | General application options |
| `permissions` | object | No | Permission settings for tool usage |
| `tools` | object | Yes | Tool configurations (ls, grep settings) |

[Official: schema.json]

### Options Fields

| Field | Type | Description |
|-------|------|-------------|
| `disabled_tools` | string[] | Tools hidden from the agent entirely |
| `disable_notifications` | boolean | Disable desktop notifications |
| `auto_lsp` | boolean | Auto-detect and start LSP servers |

[Official: schema.json, README]

### Variable Interpolation

Configuration values support shell-style variable expansion: [Official:
https://github.com/charmbracelet/crush]

| Syntax | Description |
|--------|-------------|
| `$VAR` | Environment variable |
| `${VAR}` | Environment variable (braced) |
| `$(command)` | Command substitution |

### Example crush.json

```json
{
  "$schema": "https://charm.land/crush.json",
  "models": {
    "large": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    },
    "small": {
      "provider": "anthropic",
      "model": "claude-haiku-4-20250514"
    }
  },
  "permissions": {
    "allowed_tools": ["view", "ls", "grep", "glob"]
  },
  "tools": {
    "ls": {
      "max_depth": 3,
      "max_items": 500
    }
  }
}
```

### .crushignore

Crush respects `.gitignore` by default. For additional exclusions, create a
`.crushignore` file using gitignore syntax. [Official:
https://github.com/charmbracelet/crush]

---

## 2. Rules

Crush reads `AGENTS.md` files for project-level instructions. This is the
cross-provider agent instructions format supported by multiple tools. [Official:
https://github.com/charmbracelet/crush]

### File Format

**Format:** Standard Markdown. No frontmatter, no special schema.

**Location:** Project root. Crush's own repo contains an `AGENTS.md` with
development instructions for contributors. [Official:
https://github.com/charmbracelet/crush/blob/main/AGENTS.md]

**Behavior:** Crush reads `AGENTS.md` and applies its contents as system-level
instructions for the agent. [Unverified -- exact loading behavior and
subdirectory discovery not confirmed from official sources]

**Supported by:** Claude Code (`CLAUDE.md`), Cursor, GitHub Copilot, Gemini
CLI, Windsurf, Aider, Zed, Warp, RooCode, and others.

### Crush-Specific Rules

Crush does **not** have a provider-specific rules format (no `.crush/rules/`
directory, no `.crushrules` file). The only rules mechanism is `AGENTS.md`.
[Unverified -- no Crush-specific rules format found in official sources]

---

## 3. MCP Servers

MCP (Model Context Protocol) extends Crush's agent with external tools and data
sources. Configuration is part of `crush.json`. [Official:
https://github.com/charmbracelet/crush]

### JSON Structure

```json
{
  "mcp": {
    "<server-name>": {
      "type": "stdio",
      "command": "path/to/server",
      "args": ["--flag"],
      "env": { "KEY": "value" },
      "timeout": 15,
      "disabled": false,
      "disabled_tools": ["tool-to-hide"]
    }
  }
}
```

### MCP Server Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | Yes | — | Transport: `"stdio"`, `"http"`, or `"sse"` |
| `command` | string | For stdio | — | Executable path |
| `url` | string | For http/sse | — | Server endpoint URI |
| `args` | string[] | No | `[]` | Command-line arguments |
| `env` | object | No | `{}` | Environment variables |
| `headers` | object | No | `{}` | HTTP headers (http/sse only) |
| `timeout` | integer | No | `15` | Connection timeout in seconds |
| `disabled` | boolean | No | `false` | Disable the entire server |
| `disabled_tools` | string[] | No | `[]` | Hide specific tools from the server |

[Official: schema.json]

### Transport Types

1. **stdio** -- Local processes, communicates via stdin/stdout [Official]
2. **http** -- Remote HTTP endpoints [Official]
3. **sse** -- Server-Sent Events for streaming [Official]

---

## 4. Skills

Crush supports the Agent Skills open standard for extending agent capabilities
with reusable skill packages. [Official:
https://github.com/charmbracelet/crush]

### Skill Discovery Paths

Crush scans these directories for skills (in order): [Official:
https://github.com/charmbracelet/crush]

| Path | Scope |
|------|-------|
| `$CRUSH_SKILLS_DIR` | Environment override |
| `$XDG_CONFIG_HOME/agents/skills/` | Cross-provider user skills |
| `$XDG_CONFIG_HOME/crush/skills/` | Crush-specific user skills |
| `~/.config/agents/skills/` | Fallback user skills |
| `~/.config/crush/skills/` | Fallback Crush-specific |
| `.agents/skills/` | Project cross-provider |
| `.crush/skills/` | Project Crush-specific |
| `.claude/skills/` | Project (Claude Code compat) |
| `.cursor/skills/` | Project (Cursor compat) |

Windows also checks `%LOCALAPPDATA%\agents\skills` and
`%LOCALAPPDATA%\crush\skills`.

### Skill Format

Each skill is a directory containing a `SKILL.md` file with YAML frontmatter
and markdown instructions. [Unverified -- exact frontmatter fields for Crush not
confirmed; assumed to follow the Agent Skills standard]

```
.crush/skills/my-skill/
  SKILL.md            # Required
  scripts/            # Optional
  references/         # Optional
  assets/             # Optional
```

---

## 5. Content Types NOT Supported

| Content Type | Notes |
|-------------|-------|
| **Hooks** | No lifecycle hook system. See [hooks.md](hooks.md). |
| **Commands** | No slash-command system. No `.crush/commands/` equivalent. [Unverified] |
| **Custom Agents** | No user-definable agent configuration files. See [skills-agents.md](skills-agents.md). |
| **Provider-specific rules** | No `.crushrules` or `.crush/rules/` format. Only `AGENTS.md`. |

---

## Summary: Content Type Locations

| Content Type | Project Location | Global Location | File Format |
|-------------|-----------------|-----------------|-------------|
| Configuration | `.crush.json` or `crush.json` | `~/.config/crush/crush.json` | JSON |
| Rules | `AGENTS.md` | N/A | Markdown |
| MCP servers | `crush.json` `mcp` key | `~/.config/crush/crush.json` `mcp` key | JSON |
| Skills | `.crush/skills/*/SKILL.md` or `.agents/skills/*/SKILL.md` | `~/.config/crush/skills/*/SKILL.md` | Markdown + YAML frontmatter |
| Ignore patterns | `.crushignore` | N/A | Gitignore syntax |
