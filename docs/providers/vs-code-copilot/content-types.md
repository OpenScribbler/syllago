# VS Code Copilot Content Types Reference

Documentation of all content types supported by VS Code Copilot (GitHub
Copilot's agent mode in VS Code), including file formats, directory structures,
configuration schemas, and loading behavior.

**Last updated:** 2026-03-31

**Identity note:** "VS Code Copilot" refers to GitHub Copilot's agent mode
within VS Code (`vs-code-copilot`). Distinct from `copilot-cli` (GitHub Copilot
in the terminal, `gh copilot`).

---

## Table of Contents

1. [Rules / Instructions](#1-rules--instructions)
2. [MCP Servers](#2-mcp-servers)
3. [Hooks](#3-hooks)
4. [Skills](#4-skills)
5. [Custom Agents](#5-custom-agents)
6. [Prompt Files](#6-prompt-files)
7. [Content Types NOT Supported](#7-content-types-not-supported)

---

## 1. Rules / Instructions

VS Code Copilot supports multiple instruction systems that shape agent behavior.
Instructions are injected into the model's context automatically or on demand.

### 1a. copilot-instructions.md (Repository-Level)

**File format:** Markdown. No frontmatter required. [Official]

**Location:** `.github/copilot-instructions.md` at workspace root.

**Behavior:** Automatically detected and applied to all chat requests within the
workspace. Single file for project-wide coding standards. [Official]

**VS Code setting:** `github.copilot.chat.codeGeneration.useInstructionFiles`
(default: `true`). [Official]

**Generate via:** Type `/init` in chat to auto-generate from codebase analysis.
[Official]

**Example:**

```markdown
# Project Guidelines

- Use TypeScript strict mode
- Prefer functional patterns over class-based approaches
- Write Vitest tests for all new functions
- Use date-fns instead of moment.js (moment is deprecated)
```

[Official: https://code.visualstudio.com/docs/copilot/customization/custom-instructions]

### 1b. File-Based Instructions (*.instructions.md)

**File format:** Markdown with YAML frontmatter. [Official]

**Locations:**
- `.github/instructions/*.instructions.md`
- `~/.copilot/instructions/*.instructions.md`

**Purpose:** Conditional instructions applied when files matching `applyTo`
patterns are in context. Use for language-specific or framework-specific rules.

**Frontmatter fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | No | Display name shown in UI |
| `description` | string | No | Short summary, shown on hover |
| `applyTo` | string | No | Glob pattern for file matching (e.g., `**/*.ts,**/*.tsx`) |

[Official: https://code.visualstudio.com/docs/copilot/customization/custom-instructions]

**VS Code setting:** `chat.instructionsFilesLocations` configures discovery
paths. `chat.includeApplyingInstructions` controls auto-attachment behavior.

**Example:**

```markdown
---
name: Python Testing
description: Standards for Python test files
applyTo: "tests/**/*.py"
---

# Python Test Standards

- Use pytest (not unittest)
- Follow AAA pattern (Arrange, Act, Assert)
- Use fixtures in conftest.py
```

### 1c. AGENTS.md

**File format:** Standard Markdown. No frontmatter. [Official]

**Location:** Workspace root and/or any subdirectory. Nested files discovered
when `chat.useNestedAgentsMdFiles` is enabled (experimental).

**VS Code setting:** `chat.useAgentsMdFile` (default: enabled). [Official]

**Cross-provider compatibility:** `AGENTS.md` is recognized by Claude Code,
Cursor, Gemini CLI, Windsurf, and other agents. [Official]

### 1d. CLAUDE.md

**File format:** Standard Markdown. No frontmatter. [Official]

**Locations:** Workspace root, `.claude/`, `~/.claude/`

**VS Code setting:** `chat.useClaudeMdFile` enables/disables support. [Official]

**Note:** VS Code Copilot reads `CLAUDE.md` as always-on instructions for
compatibility with Claude Code projects. [Official]

### 1e. Organization-Level Instructions

Shared across all repositories in a GitHub organization. Managed through GitHub
organization settings, not local files.

**VS Code setting:** `github.copilot.chat.organizationInstructions.enabled`
[Official]

### Priority Hierarchy

When instructions conflict:

1. Personal instructions (user-level) -- highest
2. Repository instructions (`.github/copilot-instructions.md`, `AGENTS.md`)
3. Organization instructions -- lowest

[Official: https://code.visualstudio.com/docs/copilot/customization/custom-instructions]

---

## 2. MCP Servers

MCP (Model Context Protocol) extends agent capabilities with external tools and
data sources. MCP tools only work in Agent mode (not Ask or Edit mode). [Official]

### File Locations

| Scope | Location |
|---|---|
| Workspace (per-project) | `.vscode/mcp.json` |
| User (global) | User profile `mcp.json` (open via `MCP: Open User Configuration` command) |

**Important:** VS Code uses the root key `"servers"` (not `"mcpServers"` like
Cursor and Claude Desktop). Copy-pasting configs without changing this key is
the most common setup mistake. [Official]

[Official: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration]

### JSON Schema

```json
{
  "servers": {
    "<server-name>": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  },
  "inputs": []
}
```

### Server Configuration Fields

#### STDIO Transport (Local)

| Field | Type | Required | Description |
|---|---|---|---|
| `command` | string | Yes | Executable name or path |
| `args` | string[] | No | Command-line arguments |
| `env` | object | No | Environment variables |
| `sandboxEnabled` | boolean | No | Restrict filesystem/network access (macOS/Linux only) |

#### Remote Transport (HTTP/SSE)

| Field | Type | Required | Description |
|---|---|---|---|
| `url` | string | Yes | MCP server endpoint URL |
| `headers` | object | No | Custom HTTP headers |
| `auth` | object | No | OAuth configuration |

### Variable Interpolation

| Variable | Description |
|---|---|
| `${env:NAME}` | Environment variable |
| `${userHome}` | User home directory |
| `${workspaceFolder}` | Workspace root path |
| `${workspaceFolderBasename}` | Workspace folder name |
| `${pathSeparator}` or `${/}` | OS path separator |

[Official: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration]

### Transport Types

1. **STDIO** -- Local processes, stdin/stdout communication
2. **SSE** (Server-Sent Events) -- Remote servers, legacy protocol
3. **Streamable HTTP** -- Remote servers, current standard

[Official]

---

## 3. Hooks

Hooks execute custom shell commands at agent lifecycle points. See
[hooks.md](hooks.md) for full documentation.

### File Locations (Summary)

| Scope | Location |
|---|---|
| Workspace | `.github/hooks/*.json` |
| User | `~/.copilot/hooks/` |
| Agent-scoped | `hooks` field in `.agent.md` frontmatter |

**Status:** Preview feature. [Official]

---

## 4. Skills

Skills are portable, version-controlled packages that extend agents with
domain-specific capabilities. They use the `SKILL.md` format shared across
multiple providers.

### SKILL.md Format

**File format:** Markdown with YAML frontmatter. [Official]

**Frontmatter fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Lowercase identifier, hyphens for spaces, must match parent directory. Max 64 chars. |
| `description` | string | Yes | What the skill does and when to use it. Max 1024 chars. |
| `showInMenu` | boolean | No | Show as `/` slash command in chat menu (default: `true`) |

[Official: https://code.visualstudio.com/docs/copilot/customization/agent-skills]

### Discovery

- Skills are discovered automatically by VS Code
- Users invoke via `/skill-name` in chat
- Agent can auto-invoke skills based on description relevance
- Create with `/create-skill` command in chat
- Extract from conversation: "create a skill from how we just debugged that"

[Official: https://code.visualstudio.com/docs/copilot/customization/agent-skills]

### Cross-Provider Portability

Skills created in VS Code work with multiple agents: GitHub Copilot in VS Code,
GitHub Copilot CLI, and GitHub Copilot coding agent. [Official]

---

## 5. Custom Agents

Custom agents define specialized AI personas with specific tools, models, and
behaviors. See [skills-agents.md](skills-agents.md) for full documentation.

### File Locations (Summary)

| Scope | Location |
|---|---|
| Workspace | `.github/agents/*.agent.md` |
| User | User profile agents directory |

[Official: https://code.visualstudio.com/docs/copilot/customization/custom-agents]

---

## 6. Prompt Files

Prompt files are reusable prompt templates that can reference instructions,
files, and other context. They provide structured, repeatable prompts.

**Generate via:** `/create-prompt` command in chat. [Official]

[Unverified] Exact file format and location details are sparse in current
documentation.

---

## 7. Content Types NOT Supported

The following content types have **no equivalent** in VS Code Copilot:

| Content Type | Notes |
|---|---|
| **Loadouts** | No concept of bundled content sets. [Inferred] |
| **Commands** (Cursor-style) | VS Code uses prompt files instead of Cursor's `.cursor/commands/*.md` pattern. [Inferred] |

---

## Summary: Content Type Locations

| Content Type | Workspace Location | Global Location | File Format |
|---|---|---|---|
| Instructions (project) | `.github/copilot-instructions.md` | -- | Markdown |
| Instructions (file-scoped) | `.github/instructions/*.instructions.md` | `~/.copilot/instructions/` | Markdown + YAML frontmatter |
| Instructions (cross-provider) | `AGENTS.md` (root or subdirs) | -- | Markdown |
| Instructions (CC compat) | `CLAUDE.md` | `~/.claude/` | Markdown |
| MCP servers | `.vscode/mcp.json` | User profile `mcp.json` | JSON (`"servers"` root key) |
| Hooks | `.github/hooks/*.json` | `~/.copilot/hooks/` | JSON |
| Skills | `SKILL.md` in skill directories | -- | Markdown + YAML frontmatter |
| Custom agents | `.github/agents/*.agent.md` | User profile | Markdown + YAML frontmatter |

---

## Sources

- [Custom instructions in VS Code](https://code.visualstudio.com/docs/copilot/customization/custom-instructions) [Official]
- [Customize AI in VS Code](https://code.visualstudio.com/docs/copilot/customization/overview) [Official]
- [MCP configuration reference](https://code.visualstudio.com/docs/copilot/reference/mcp-configuration) [Official]
- [Add and manage MCP servers in VS Code](https://code.visualstudio.com/docs/copilot/customization/mcp-servers) [Official]
- [Agent skills in VS Code](https://code.visualstudio.com/docs/copilot/customization/agent-skills) [Official]
- [Custom agents in VS Code](https://code.visualstudio.com/docs/copilot/customization/custom-agents) [Official]
- [Agent hooks in VS Code](https://code.visualstudio.com/docs/copilot/customization/hooks) [Official]
- [VS Code Copilot settings reference](https://code.visualstudio.com/docs/copilot/reference/copilot-settings) [Official]
