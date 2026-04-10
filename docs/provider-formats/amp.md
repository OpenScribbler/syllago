# Amp — Format Reference

Provider slug: `amp`

Supports: Rules, Skills, MCP

Sources: [Amp Manual](https://ampcode.com/manual), [Amp Agent Skills](https://ampcode.com/news/agent-skills), [Amp MCP Setup Guide](https://github.com/sourcegraph/amp-examples-and-guides/blob/main/guides/mcp/amp-mcp-setup-guide.md)

---

## Rules (AGENTS.md)

**Location:** `AGENTS.md` at project root, subdirectories, or `~/.config/amp/AGENTS.md` (global)

**Format:** Markdown with optional YAML frontmatter

**Frontmatter:**

| Field | Type | Description |
|-------|------|-------------|
| `globs` | string[] | File patterns for conditional activation |

Rules without frontmatter are always-active. Amp also reads `CLAUDE.md` and `AGENT.md` as fallbacks.

**Example:**

```markdown
---
globs:
  - "**/*.ts"
  - "**/*.tsx"
---

# Frontend Conventions

- Use React functional components
- State management via Zustand
```

**Special features:**
- `@filepath` references in AGENTS.md (relative, absolute, or `@~/` paths)
- Subtree AGENTS.md files loaded when agent reads files in that directory
- Parent directory traversal up to `$HOME`

---

## Skills (SKILL.md)

**Location:** `.agents/skills/<name>/SKILL.md` (project), `~/.config/agents/skills/<name>/SKILL.md` (global)

**Compat paths:** `.claude/skills/`, `~/.claude/skills/`

**Format:** SKILL.md with YAML frontmatter (Agent Skills spec)

**Frontmatter:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill identifier |
| `description` | string | When to load this skill |

Amp follows the base Agent Skills spec. Skills lazily load instructions on demand based on description matching.

**Example:**

```markdown
---
name: code-review
description: Use when reviewing code for quality and security issues
---

# Code Review Skill

Review code for:
1. Security vulnerabilities
2. Performance issues
3. Code style violations
```

**Note:** Amp deprecated custom commands in favor of skills. Former `.agents/commands/` files should be migrated to `.agents/skills/`.

---

## MCP Servers

**Location:** `.amp/settings.json` (project), `~/.config/amp/settings.json` (global)

**Format:** JSON

**Top-level key:** `amp.mcpServers`

**Per-server fields:**

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `url` | string | Remote server URL |
| `headers` | object | HTTP headers |
| `includeTools` | string[] | Tool name glob patterns to filter |

**Example:**

```json
{
  "amp.mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"],
      "includeTools": ["read_*", "list_directory"]
    },
    "remote-api": {
      "url": "https://api.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${API_KEY}"
      }
    }
  }
}
```

**Key details:**
- `includeTools` supports glob patterns (e.g., `"navigate_*"`, `"fill*"`)
- Workspace servers require explicit approval before running
- Global/CLI-configured servers do not require approval
- Supports `${VAR_NAME}` env var expansion in values

---

## Unsupported Content Types

| Type | Status |
|------|--------|
| Agents | No structured agent format. Subagents are auto-spawned, not user-configurable. |
| Commands | Deprecated. Merged into Skills system. |
| Hooks | Not supported as separate content type |

---

## Detection

Amp is detected by the presence of `~/.config/amp/` directory.
