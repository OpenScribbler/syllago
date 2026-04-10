# Cline — Format Reference

Provider slug: `cline`

Supports: Rules, MCP

Sources: [Cline Rules docs](https://docs.cline.bot/features/cline-rules), [Cline MCP docs](https://docs.cline.bot/mcp/configuring-mcp-servers)

---

## Rules

**Location:** `<project>/.clinerules/` (directory with `.md`/`.txt` files)

**Global location:** `~/Documents/Cline/Rules/` (macOS/Linux)

**Legacy:** `<project>/.clinerules` (single file, still supported)

**Format:** Markdown with optional YAML frontmatter

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `paths` | string[] | No | Glob patterns for conditional activation (scoped to matching files) |

**Activation model:** Files in `.clinerules/` are read in alphabetical order. Numeric prefixes (e.g., `01-coding.md`, `02-testing.md`) control ordering. Rules without `paths:` frontmatter apply globally. Rules are injected verbatim into the system prompt.

**Cross-tool detection:** Cline auto-detects `.cursorrules`, `.windsurfrules`, and `AGENTS.md` in the same project and presents them as toggleable rule sources in the UI.

**Token guidance:** Recommended limit is under 1000 tokens per rule file.

**Example:**

```markdown
---
paths:
  - "src/components/**"
  - "src/hooks/**"
---

Use functional React components with TypeScript.
Always include PropTypes or TypeScript interfaces.
```

---

## MCP Servers

**Location:** `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` (Linux)

**Note:** Global only. No project-level MCP configuration is supported (open feature request).

**Format:** JSON

**Transport types:** Cline supports both stdio (`command` + `args`) and SSE (`url`) transports.

**Server entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Conditional | Executable command (stdio transport) |
| `args` | string[] | No | Arguments for command (stdio transport) |
| `url` | string | Conditional | SSE endpoint URL (SSE transport) |
| `env` | object | No | Environment variables |
| `alwaysAllow` | string[] | No | Tools to auto-approve (skip confirmation) |
| `disabled` | bool | No | Disable without removing |

**Example:**

```json
{
  "mcpServers": {
    "stdio-server": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-slack"],
      "env": { "SLACK_TOKEN": "xoxb-..." },
      "alwaysAllow": ["tool1", "tool2"],
      "disabled": false
    },
    "sse-server": {
      "url": "https://mcp.example.com/sse",
      "disabled": false
    }
  }
}
```

---

## Unsupported Content Types

| Type | Notes |
|------|-------|
| Commands | Not supported |
| Skills | Not supported (Memory Bank provides partial equivalent) |
| Agents | Not supported |
| Hooks | Not supported in rule files (VS Code extension feature only) |

---

## Detection

Cline is a VS Code extension (`saoudrizwan.claude-dev`). Detection signals:
- VS Code globalStorage directory exists for the extension
- `.clinerules/` directory exists in project root

---

## Syllago Provider Mapping

| syllago ContentType | Cline Equivalent | Path |
|-------------------|------------------|------|
| `Rules` | `.clinerules/` directory | `<project>/.clinerules/` |
| `MCP` | `cline_mcp_settings.json` | VS Code globalStorage (JSON merge) |

---

## Key Differences from Similar Tools

- **No project-level MCP**: MCP config is global only (unlike Roo Code fork)
- **Cross-tool rule detection**: Unique feature — reads other tools' rule files as toggleable sources
- **Simple format**: Closest to plain markdown of all providers
- **VS Code only**: Extension-based, not standalone CLI
