# Zed Content Types

Last updated: 2026-03-21

## Summary

| Content Type | Supported | Format | Location |
|-------------|-----------|--------|----------|
| Rules | Yes | Plain text / Markdown | `.rules` file or Rules Library |
| MCP | Yes | JSON in `settings.json` | `context_servers` key |
| Agent Profiles | Yes | JSON in `settings.json` | `agent.profiles` key |
| External Agents | Yes | JSON in `settings.json` | `agent_servers` key |
| Slash Commands | Partial | TOML + code (extensions) | Extension `extension.toml` |
| Prompts | Deprecated | Replaced by Rules | Rules Library |

---

## Rules

### Overview

Rules are project-level instructions included in Agent Panel interactions. They
replaced the older "Prompt Library" system. [Official]

### File Names (priority order)

Zed checks for these files at the project root. The **first match wins**:

1. `.rules`
2. `.cursorrules`
3. `.windsurfrules`
4. `.clinerules`
5. `.github/copilot-instructions.md`
6. `AGENT.md`
7. `AGENTS.md`
8. `CLAUDE.md`
9. `GEMINI.md`

### Location

- **Project rules**: Root of the project worktree (auto-included in all Agent
  Panel interactions)
- **Library rules**: Stored internally by Zed's Rules Library (not filesystem
  files in the project)

### Format

Plain text or Markdown. No special schema required. Content is injected as-is
into the LLM context.

### Rules Library

- Full editor with syntax highlighting
- Rules can be set as **default** (auto-included in every interaction) via the
  paperclip icon
- Rules can be **@-mentioned** on demand: `@rule <name>`
- Rules can nest other rules via slash commands (text threads only)
- Access: `agent: open rules library` or `cmd-alt-l` / `ctrl-alt-l`

### Syllago Mapping

Rules map directly to syllago's `rules` content type. The canonical format is
plain text. Conversion from other providers' rule formats (e.g., Claude Code's
`CLAUDE.md`) is straightforward since Zed natively recognizes those filenames.

---

## MCP (Model Context Protocol)

### Overview

MCP servers extend the agent with custom tools. Configured in `settings.json`
under `context_servers`. [Official]

### Configuration Format

**Stdio (local command):**

```json
{
  "context_servers": {
    "my-server": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"],
      "env": {
        "HOME": "/Users/me"
      }
    }
  }
}
```

**Remote (URL-based, SSE or Streamable HTTP):**

```json
{
  "context_servers": {
    "remote-server": {
      "url": "https://mcp.example.com/sse",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

### Installation Methods

1. **Extensions**: Pre-packaged MCP servers from the Zed marketplace
   (`zed: extensions` command)
2. **Custom**: Manual JSON entries in `settings.json`
3. **Agent Panel**: "Add Custom Server" button in the Agent Panel menu

### Features

- Multiple servers can run simultaneously
- Tool permissions: `mcp:<server>:<tool_name>` format
- Runtime updates: auto-reloads when server sends
  `notifications/tools/list_changed`
- MCP prompts are also supported (server-provided prompt templates)

### Syllago Mapping

MCP configs map to syllago's `mcp` content type. The JSON structure merges into
the provider's settings file (`settings.json`). Key differences from other
providers: Zed uses `context_servers` (not `mcpServers` like Claude Code or
Cursor).

---

## Agent Profiles

### Overview

Profiles group tools into named sets. Three built-in profiles exist; custom
profiles can be created. [Official]

### Built-in Profiles

| Profile | Description |
|---------|-------------|
| **Write** | All tools enabled — full agentic coding |
| **Ask** | Read-only tools — questions without modifications |
| **Minimal** | No tools — general LLM conversation |

### Configuration

Configured in `settings.json` under `agent.profiles`. Can also be managed via
UI (`agent: manage profiles` or `cmd-alt-p` / `ctrl-alt-p`).

Custom profiles specify which tools are enabled/disabled, allowing fine-grained
control over agent capabilities.

### Syllago Mapping

No direct equivalent in most other providers. Could potentially map to a
combination of tool permissions and rules. Not a primary syllago content type.

---

## External Agents (ACP)

### Overview

External agents integrate via the Agent Client Protocol (ACP). They run as
separate processes and appear in Zed's Agent Panel alongside the built-in
agent. [Official]

### Supported Agents

- **Gemini CLI** (Google) — reference ACP implementation
- **Claude Agent** (Anthropic) — via Claude Agent SDK
- **Codex** (OpenAI)
- **GitHub Copilot**
- Community agents via ACP Registry

### Configuration Format

```json
{
  "agent_servers": {
    "My Custom Agent": {
      "type": "custom",
      "command": "node",
      "args": ["~/projects/agent/index.js", "--acp"],
      "env": {}
    }
  }
}
```

### Installation Methods

1. **ACP Registry** (recommended): `zed: acp registry` command
2. **Custom config**: Manual `agent_servers` entries in `settings.json`
3. **Extensions**: Marketplace (being deprecated in favor of registry)

### Syllago Mapping

No direct equivalent in other providers. This is Zed-specific infrastructure
for running third-party agents. Not a primary syllago content type.

---

## Slash Commands

### Overview

Slash commands insert dynamic content into conversations. Available in text
threads and the Rules Library editor. [Official]

### Built-in Slash Commands

| Command | Description |
|---------|-------------|
| `/file` | Insert file/directory contents (supports globs) |
| `/now` | Insert current date and time |
| `/diagnostics` | Insert language server errors/warnings |
| `/rule` (was `/prompt`) | Insert a rule from the Rules Library |
| `/tab` | Insert content of active/all open tabs |
| `/symbols` | Insert symbols (functions, classes) from current tab |

### Extension Slash Commands

Extensions can provide custom slash commands. Registered in `extension.toml`:

```toml
[slash_commands.my-command]
description = "Does something useful"
requires_argument = true
```

### Limitations

- Slash commands work in **text threads** and the **Rules Library**, but NOT in
  the main Agent Panel chat (which uses @-mentions instead)
- In the Agent Panel, the equivalent is `@rule <name>`, `@file <path>`, etc.

### Syllago Mapping

Slash commands are Zed-specific UI affordances, not portable content. Extension
slash commands could theoretically map to syllago's `commands` type but would
require significant adaptation.

---

## Deprecated: Prompt Library

The Prompt Library was renamed to the **Rules Library**. Prompts are now called
rules. The `/prompt` slash command became `/rule`. The `@prompt` mention became
`@rule`. All functionality is preserved under the new naming. [Official]

---

## Sources

- [Official] [Rules](https://zed.dev/docs/ai/rules) — Rules format, file names, library
- [Official] [MCP](https://zed.dev/docs/ai/mcp) — MCP server configuration
- [Official] [Agent Panel](https://zed.dev/docs/ai/agent-panel) — Profiles, slash commands
- [Official] [Agent Settings](https://zed.dev/docs/ai/agent-settings) — Settings JSON format
- [Official] [External Agents](https://zed.dev/docs/ai/external-agents) — ACP and external agent config
- [Official] [Slash Command Extensions](https://zed.dev/docs/extensions/slash-commands) — Custom slash commands
- [Community] [Zed Agents Blog Post](https://constantin.glez.de/notes/2025-05-24-zed-does-agents-now/) — Slash commands and @-mentions transition
