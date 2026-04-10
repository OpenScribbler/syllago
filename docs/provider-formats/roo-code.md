# Roo Code — Format Reference

Provider slug: `roo-code`

Supports: Rules, MCP, Agents (via custom modes)

Sources: [Roo Code custom instructions](https://docs.roocode.com/features/custom-instructions), [Roo Code MCP](https://docs.roocode.com/features/mcp/using-mcp-in-roo), [Roo Code custom modes](https://docs.roocode.com/features/custom-modes)

---

## Rules

**Location:** `<project>/.roo/rules/` (all modes) and `<project>/.roo/rules-{modeSlug}/` (mode-specific)

**Global location:** `~/.roo/rules/` (all modes) and `~/.roo/rules-{modeSlug}/` (mode-specific)

**Legacy:** `<project>/.roorules` (single file) and `<project>/.roorules-{modeSlug}` (mode-specific single file)

**Format:** Markdown or plain text

**Discovery:** Files are scanned recursively and processed in alphabetical order (case-insensitive). Numeric prefixes control ordering.

**Built-in modes:**
- `code` — coding mode
- `architect` — architecture/design mode
- `ask` — question-answering mode
- `debug` — debugging mode
- `orchestrator` — multi-step task coordination

**Precedence:** Directory method (`.roo/rules-{mode}/`) overrides legacy single file (`.roorules-{mode}`). Workspace rules override global rules on conflict.

**Cross-tool detection:** Roo Code detects `.clinerules` as a fallback (Cline fork heritage). Also detects `AGENTS.md`.

**Example directory structure:**

```
.roo/
  rules/                    # Apply to all modes
    01-general.md
    02-coding-style.md
  rules-code/               # Apply only in "code" mode
    01-js-conventions.md
    02-testing-standards.md
  rules-architect/          # Apply only in "architect" mode
    01-design-principles.md
```

---

## MCP Servers

**Location:** `<project>/.roo/mcp.json` (project) or VS Code globalStorage `mcp_settings.json` (global)

**Format:** JSON

**Server entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Conditional | Executable for stdio transport |
| `args` | string[] | No | Arguments for command |
| `env` | object | No | Environment variables |
| `disabled` | bool | No | Disable without removing |
| `type` | string | No | `"sse"` for SSE transport |
| `url` | string | Conditional | Endpoint for SSE transport |

MCP server packages can be installed to `~/.roo/mcps/` (global) or `./.roo/mcps/` (project).

**Example:**

```json
{
  "mcpServers": {
    "server-name": {
      "command": "npx",
      "args": ["-y", "package-name"],
      "env": {},
      "disabled": false
    },
    "sse-server": {
      "type": "sse",
      "url": "https://mcp.example.com/sse",
      "disabled": false
    }
  }
}
```

---

## Agents (Custom Modes)

**Format:** YAML (preferred) or JSON

Custom modes define behavioral profiles with tool access control.

**Mode config fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `slug` | string | Yes | Unique identifier (kebab-case) |
| `name` | string | Yes | Display name |
| `roleDefinition` | string | Yes | System prompt defining the mode's behavior |
| `whenToUse` | string | No | Description of when this mode is appropriate |
| `customInstructions` | string | No | Additional instructions |
| `groups` | string[] | No | Tool access groups: `read`, `browser`, `edit`, `command` |

**Example:**

```yaml
slug: reviewer
name: Code Reviewer
roleDefinition: "You are a code reviewer focused on security, performance, and maintainability."
whenToUse: "Use when reviewing pull requests or code changes"
customInstructions: "Always check for OWASP Top 10 vulnerabilities"
groups:
  - read
  - browser
```

---

## Unsupported Content Types

| Type | Notes |
|------|-------|
| Commands | Not supported |
| Skills | Not supported |
| Hooks | Not supported (Cline fork, no native hooks) |

---

## Detection

Detection signals:
- `.roo/` directory exists in project root
- VS Code extension for Roo Code installed

---

## Syllago Provider Mapping

| syllago ContentType | Roo Code Equivalent | Path |
|-------------------|---------------------|------|
| `Rules` | `.roo/rules/` and `.roo/rules-{mode}/` | Recursive markdown/text files |
| `MCP` | `.roo/mcp.json` | JSON merge |
| `Agents` | Custom modes (YAML/JSON) | Mode definitions |
| `Commands` | Not supported | — |
| `Skills` | Not supported | — |
| `Hooks` | Not supported | — |

---

## Key Differences from Cline

| Feature | Cline | Roo Code |
|---------|-------|----------|
| Config directory | `.clinerules/` | `.roo/` |
| Mode-specific rules | No | Yes (`.roo/rules-{mode}/`) |
| Custom modes | No | Yes (YAML-defined behavioral modes) |
| Project-level MCP | No (global only) | Yes (`.roo/mcp.json`) |
| Legacy Cline compat | N/A | Reads `.clinerules` as fallback |
| Rule file discovery | Flat directory | Recursive with alphabetical ordering |
| Tool access control | Global | Per-mode groups |
- **Symbolic link support**: 5-level cycle detection in rules directories
- **AGENTS.md support**: Reads workspace-root `AGENTS.md` as toggleable rule source
