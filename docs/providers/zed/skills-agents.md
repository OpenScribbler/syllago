# Zed: Skills, Agents, and AI Configuration

## Rules (Instructions / Prompts)

Zed's equivalent of "skills" or "agent instructions" is the **rules** system. Rules are
Markdown prompts injected into the agent's system prompt.

### Project-Level Rules

A single rules file at the project root, auto-included in all Agent Panel interactions.
Priority order (first match wins):

1. `.rules` -- Zed-native format
2. `.cursorrules` -- Cursor compatibility
3. `.windsurfrules` -- Windsurf compatibility

Format: plain Markdown, no frontmatter or special syntax. The entire file content is
treated as the system prompt.

[Official] https://zed.dev/docs/ai/rules

### Rules Library

A built-in UI for creating and managing reusable rule files stored locally:

- Open via `agent: open rules library` or `cmd-alt-l` / `ctrl-alt-l`
- Any rule can be set as **default** (auto-included in every interaction) via the
  paperclip icon
- Rules can be **@-mentioned** in conversations for on-demand inclusion
- In text threads, rules support `/file` and `/prompt` slash commands for nesting

[Official] https://zed.dev/docs/ai/rules

### Implications for Syllago

- `.rules` files map directly to syllago's rules content type
- Plain Markdown format -- no conversion needed beyond filename
- `.cursorrules` and `.windsurfrules` compatibility means Zed already handles
  cross-provider rule files natively

---

## Slash Commands

### Built-in Slash Commands

| Command | Description | Requires Argument |
|---------|-------------|-------------------|
| `/file` | Insert file or directory content (supports globs) | Yes |
| `/diagnostics` | Inject LSP errors/warnings into context | No |
| `/now` | Insert current date and time | No |
| `/prompt` | Insert a rule from the Rules Library | Yes |
| `/symbols` | Insert active symbols from current tab | No |
| `/tab` | Insert content of active/all open tabs | No |

[Official] https://zed.dev/docs/ai/agent-panel

### Extension Slash Commands

Extensions can define custom slash commands via `extension.toml`:

```toml
[slash_commands.my_command]
description = "What this command does"
requires_argument = true
```

Implementation requires two Rust trait methods:

- `run_slash_command` -- executes the command, returns `SlashCommandOutput` with `text`
  and optional `sections`
- `complete_slash_command_argument` -- provides argument completions (optional)

[Official] https://zed.dev/docs/extensions/slash-commands

### Implications for Syllago

- Slash commands are not a standalone content type -- they're part of extensions
- No file-based format to import/export; they live in compiled Rust extensions
- Built-in commands are hardcoded and not configurable

---

## Agent / Model Configuration

### Model Selection

Configured in `settings.json` under the `agent` key:

```json
{
  "agent": {
    "default_model": {
      "provider": "zed.dev",
      "model": "claude-sonnet-4-5"
    },
    "inline_assistant_model": {
      "provider": "anthropic",
      "model": "claude-3-5-sonnet"
    },
    "commit_message_model": {
      "provider": "openai",
      "model": "gpt-4o-mini"
    },
    "thread_summary_model": {
      "provider": "google",
      "model": "gemini-2.0-flash"
    }
  }
}
```

Feature-specific models allow different models for different tasks (agent panel, inline
assist, commit messages, thread summaries).

[Official] https://zed.dev/docs/ai/agent-settings

### Inline Alternatives

```json
{
  "agent": {
    "inline_alternatives": [
      { "provider": "zed.dev", "model": "gpt-5-mini" },
      { "provider": "zed.dev", "model": "gemini-3-flash" }
    ]
  }
}
```

### Model Temperature

```json
{
  "agent": {
    "model_parameters": [
      { "provider": "openai", "temperature": 0.5 },
      { "provider": "zed.dev", "model": "claude-sonnet-4-5", "temperature": 1.0 }
    ]
  }
}
```

---

## Tool Permissions

Granular control over what the agent can do, configured in `settings.json`:

```json
{
  "agent": {
    "tool_permissions": {
      "default": "allow",
      "tools": {
        "terminal": {
          "default": "confirm",
          "always_allow": [
            { "pattern": "^cargo\\s+(build|test|check)" },
            { "pattern": "^git\\s+(status|log|diff)" }
          ],
          "always_deny": [{ "pattern": "rm\\s+-rf\\s+(/|~)" }],
          "always_confirm": [{ "pattern": "sudo\\s" }]
        },
        "edit_file": {
          "always_deny": [
            { "pattern": "\\.env" },
            { "pattern": "\\.(pem|key)$" }
          ]
        }
      }
    }
  }
}
```

**Evaluation priority** (highest to lowest):
1. Built-in security rules
2. `always_deny` patterns
3. `always_confirm` patterns
4. `always_allow` patterns
5. Tool-specific `default`
6. Global `default`

**MCP tool permissions** use the format `mcp:<server_name>:<tool_name>`:

```json
{
  "agent": {
    "tool_permissions": {
      "tools": {
        "mcp:github:create_issue": { "default": "confirm" },
        "mcp:github:create_pull_request": { "default": "deny" }
      }
    }
  }
}
```

Patterns are case-insensitive by default; set `"case_sensitive": true` to override.

[Official] https://zed.dev/docs/ai/agent-settings

---

## Built-in Agent Tools

### Read & Search
- **diagnostics** -- LSP errors/warnings for files or project-wide
- **fetch** -- fetch URL content as Markdown
- **find_path** -- glob-based file search
- **grep** -- regex content search across project
- **list_directory** -- directory listing
- **now** -- current date/time
- **open** -- launch files/URLs with OS default app
- **read_file** -- read project file content
- **thinking** -- internal reasoning (no side effects)
- **web_search** -- web search with snippets

### Edit
- **copy_path** -- duplicate files/directories
- **create_directory** -- create directories (recursive)
- **delete_path** -- remove files/directories
- **edit_file** -- modify file content by text replacement
- **move_path** -- move/rename files
- **restore_file_from_disk** -- discard unsaved changes
- **save_file** -- persist unsaved modifications
- **terminal** -- execute shell commands

### Delegation
- **spawn_agent** -- delegate tasks to subagents with separate context windows

[Official] https://zed.dev/docs/ai/tools

---

## External Agents (ACP)

Zed supports external AI agents via the Agent Client Protocol (ACP).

### Configuration

**Via settings.json:**

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

**Via ACP Registry** (preferred as of v0.221.x): install agents from the registry UI
with automatic updates. Access via `zed: acp registry` command.

**Via extensions:** agents distributed as Zed extensions with `extension.toml`
configuration.

### Supported External Agents
- Claude Agent (via ACP adapter)
- Gemini CLI (Google's reference ACP implementation)
- Codex CLI (OpenAI)
- GitHub Copilot

### Feature Support for External Agents

| Feature | Claude Agent | Gemini CLI | Codex CLI |
|---------|-------------|------------|-----------|
| Subagents | Yes | -- | -- |
| Custom slash commands | Yes | -- | -- |
| Agent teams | No | No | No |
| Hooks | No | No | No |
| MCP servers | Yes | No | Yes |
| Edit past messages | No | No | No |
| Resume from history | No | No | No |
| Checkpointing | No | No | No |

[Official] https://zed.dev/docs/ai/external-agents

### Implications for Syllago

- External agent definitions (`agent_servers` in settings.json) are a potential content
  type -- they define command, args, and env for running an agent
- Tool permissions in settings.json could map to syllago's settings merge pattern
  (JSON merge into provider settings)
- Model selection settings similarly merge into settings.json
- ACP Registry agents are installed via UI, not file-based -- not directly
  importable/exportable

---

## Zed-Specific AI Concepts

### Text Threads vs Agent Panel
- **Agent Panel**: full agentic mode with tool use, file editing, terminal access
- **Text Threads**: lightweight chat-like interactions, support slash commands in rules,
  no tool execution

### Context Management
Zed uses **explicit context** (user provides files via @ mentions and slash commands)
rather than automatic codebase indexing. The agent can search via `grep` and `find_path`
tools, but explicit context improves quality.

### Subagents (spawn_agent)
The agent can spawn subagents with separate context windows for parallel investigation.
This is Zed's equivalent of agent delegation/teams, though formal "agent teams" are not
yet supported.

## Sources

- [Official] Zed Rules: https://zed.dev/docs/ai/rules
- [Official] Zed Agent Settings: https://zed.dev/docs/ai/agent-settings
- [Official] Zed Tools: https://zed.dev/docs/ai/tools
- [Official] Zed Agent Panel: https://zed.dev/docs/ai/agent-panel
- [Official] Zed External Agents: https://zed.dev/docs/ai/external-agents
- [Official] Zed Slash Command Extensions: https://zed.dev/docs/extensions/slash-commands
- [Official] Zed ACP: https://zed.dev/acp
