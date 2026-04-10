# Zed — Format Reference

Provider slug: `zed`

Supports: Rules, MCP

Sources: [Zed AI Rules](https://zed.dev/docs/ai/rules), [Zed AI Configuration](https://zed.dev/docs/ai/configuration), [Zed Agent Settings](https://zed.dev/docs/ai/agent-settings), [Zed MCP docs](https://zed.dev/docs/ai/mcp)

---

## Rules

**Location:** `<project>/.rules` (project root)

**Format:** Markdown (plain, no frontmatter)

**Discovery priority:** Zed checks for project-level rules in this order (first match wins, no merging):
1. `.rules`
2. `.cursorrules`
3. `.windsurfrules`
4. `.clinerules`
5. `.github/copilot-instructions.md`
6. `AGENT.md`
7. `AGENTS.md`
8. `CLAUDE.md`
9. `GEMINI.md`

**Important:** Only the first matching file is used. Multiple rule files are NOT merged.

**Rules Library:** Zed provides a UI-managed Rules Library for creating reusable rules that can be `@-mentioned` in agent conversations. These are managed through the Agent Panel, not filesystem files.

**Example:**

```markdown
# Project Rules

Use TypeScript for all new files.
Prefer functional components with hooks.
Always write tests alongside implementation code.
```

---

## MCP Servers

**Location:** `~/.config/zed/settings.json` under `"context_servers"` key

**Note:** Global only via settings.json. MCP servers can also be installed as Zed extensions.

**Format:** JSON (nested in settings.json)

**Server entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | Yes | Must be `"custom"` for user-defined servers |
| `command` | string | Yes | Executable command |
| `args` | string[] | No | Arguments for command |
| `env` | object | No | Environment variables |

**Transport:** Only stdio is currently supported. Streamable HTTP is an open feature request.

**Example:**

```json
{
  "context_servers": {
    "my-mcp-server": {
      "source": "custom",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"],
      "env": {}
    }
  }
}
```

---

## Tool Permissions

**Location:** `~/.config/zed/settings.json` under `"agent.tool_permissions"`

Zed has granular per-tool permissions with regex pattern matching:

```json
{
  "agent": {
    "tool_permissions": {
      "default": "confirm",
      "tools": {
        "terminal": {
          "default": "confirm",
          "always_allow": [{ "pattern": "^cargo\\s+(build|test)" }],
          "always_deny": [{ "pattern": "rm\\s+-rf\\s+(/|~)" }]
        },
        "mcp:github:create_issue": { "default": "confirm" }
      }
    }
  }
}
```

Not directly mappable to syllago hooks, but worth noting for future consideration.

---

## Agent/Model Configuration

**Location:** `~/.config/zed/settings.json` under `"agent"` key

```json
{
  "agent": {
    "default_model": { "provider": "zed.dev", "model": "claude-sonnet-4-5" },
    "inline_assistant_model": { "provider": "anthropic", "model": "claude-3-5-sonnet" },
    "commit_message_model": { "provider": "openai", "model": "gpt-4o-mini" }
  }
}
```

Not directly mappable to syllago agent definitions.

---

## Unsupported Content Types

| Type | Notes |
|------|-------|
| Commands | Not supported |
| Skills | Not supported |
| Agents | Not directly supported (model profiles only) |
| Hooks | Not supported (tool permissions are similar but not equivalent) |

---

## Detection

Detection signals:
- `~/.config/zed/` directory exists
- `zed` command available in PATH

---

## Syllago Provider Mapping

| syllago ContentType | Zed Equivalent | Path |
|-------------------|----------------|------|
| `Rules` | `.rules` file | `<project>/.rules` |
| `MCP` | `context_servers` in settings.json | JSON merge into `~/.config/zed/settings.json` |

---

## Key Differences from Similar Tools

- **Broad rule file compatibility**: Auto-detects 9 different rule file formats from other tools (but only uses the first match)
- **No rule merging**: Unlike most tools, only one rule file is used — first match wins
- **No project-level MCP**: MCP is configured globally in settings.json
- **Different MCP key**: Uses `context_servers` not `mcpServers`
- **Extension-based MCP**: MCP servers can also be installed as Zed extensions
- **Rules Library**: IDE-managed reusable rules with `@-mention` support (not filesystem-based)
- **Template overrides**: Advanced Handlebars template system for system prompt customization (`~/.config/zed/prompt_overrides/*.hbs`)
- **Inline alternatives**: Can send same prompt to multiple models simultaneously for A/B comparison
