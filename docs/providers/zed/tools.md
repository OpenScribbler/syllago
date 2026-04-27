# Zed AI Agent Tools

Last updated: 2026-03-21

Zed's built-in agent exposes tools through the Agent Panel. Tools are grouped
into **profiles** (Write, Ask, Minimal, or custom) that control which tools are
available. Custom tools can be added via MCP servers.

## Complete Tool Inventory

### Read & Search Tools

| Tool | Description | Parameters | Notes |
|------|-------------|------------|-------|
| `read_file` | Reads the content of a file in the project | `path` (string) | Read-only |
| `find_path` | Finds files by matching glob patterns | `pattern` (glob string) | Similar to Claude Code's `Glob` |
| `grep` | Searches file contents using regular expressions | `regex` (string), `path` (string, optional) | Scoped to project; optional path narrows search |
| `list_directory` | Lists files and directories at a path | `path` (string) | Like `ls` |
| `diagnostics` | Gets errors/warnings from language server | `path` (string, optional) | No path = project-wide summary |
| `web_search` | Searches the web, returns snippets and links | `query` (string) | Real-time web access |
| `fetch` | Fetches a URL and returns content as Markdown | `url` (string) | Converts HTML to markdown |
| `now` | Returns current date and time | (none) | Useful for time-aware prompts |
| `thinking` | Internal reasoning without side effects | `content` (string) | No external action; planning/brainstorming |

### Write & Edit Tools

| Tool | Description | Parameters | Notes |
|------|-------------|------------|-------|
| `edit_file` | Replaces specific text in a file with new content | `path` (string), `old_text` (string), `new_text` (string) | Search-and-replace style; can create new files |
| `copy_path` | Copies a file or directory recursively | `source` (path), `destination` (path) | More efficient than read+write for duplication |
| `create_directory` | Creates a directory (including parents) | `path` (string) | Like `mkdir -p` |
| `delete_path` | Deletes a file or directory recursively | `path` (string) | Irreversible |
| `move_path` | Moves or renames a file or directory | `source` (path), `destination` (path) | Rename if only filename differs |
| `terminal` | Executes a shell command, returns output | `command` (string) | New shell process per invocation |
| `open` | Opens a file or URL with the OS default app | `path_or_url` (string) | Launches external application |
| `save_file` | Saves unsaved changes in an open buffer | `path` (string) | Triggers formatters/linters |
| `restore_file_from_disk` | Discards unsaved changes, reloads from disk | `path` (string) | Labeled "revert_file" in some UI contexts |

### Agent Management Tools

| Tool | Description | Parameters | Notes |
|------|-------------|------------|-------|
| `spawn_agent` | Spawns a subagent with its own context window | `task` (string), `context` (string, optional) | Subagent has same tools as parent; labeled "subagent" in some docs |

**Total: 19 built-in tools**

## Cross-Provider Equivalents

| Zed Tool | Claude Code | Cursor | Windsurf | Notes |
|----------|-------------|--------|----------|-------|
| `read_file` | `Read` | `read_file` | `read_file` | Universal |
| `edit_file` | `Edit` | `edit_file` | `edit_file` | Zed uses old_text/new_text; Claude uses line-targeted diffs |
| `find_path` | `Glob` | `list_dir` (partial) | `find_file` | Glob-pattern file search |
| `grep` | `Grep` | `grep_search` | `grep` | Regex content search |
| `list_directory` | `Bash(ls)` | `list_dir` | `list_dir` | Directory listing |
| `terminal` | `Bash` | `run_terminal_command` | `run_command` | Shell execution |
| `web_search` | `WebSearch` | N/A | N/A | Web search |
| `fetch` | `WebFetch` | N/A | N/A | URL content fetch |
| `spawn_agent` | `Agent`/`Task` | N/A | N/A | Subagent delegation |
| `diagnostics` | N/A | `diagnostics` | N/A | Language server errors |
| `thinking` | Extended thinking | N/A | N/A | Internal reasoning |
| `copy_path` | `Bash(cp)` | N/A | N/A | File/dir copy |
| `create_directory` | `Bash(mkdir)` | N/A | N/A | Directory creation |
| `delete_path` | `Bash(rm)` | N/A | N/A | File/dir deletion |
| `move_path` | `Bash(mv)` | N/A | N/A | File/dir move/rename |
| `now` | N/A | N/A | N/A | Current timestamp |
| `open` | N/A | N/A | N/A | Open with OS default app |
| `save_file` | N/A | N/A | N/A | Save buffer to disk |
| `restore_file_from_disk` | N/A | N/A | N/A | Discard unsaved changes |

## Tool Permissions

Permissions are configured in `settings.json` under `agent.tool_permissions`:

```json
{
  "agent": {
    "tool_permissions": {
      "default": "confirm",
      "tools": {
        "edit_file": {
          "always_allow": ["src/**"],
          "always_deny": [".env*", "credentials*"],
          "always_confirm": ["*.toml"]
        },
        "terminal": {
          "always_confirm": ["*"]
        },
        "mcp:github:create_issue": {
          "default": "confirm"
        }
      }
    }
  }
}
```

Priority order: built-in security rules > `always_deny` > `always_confirm` > `always_allow`.

MCP tools use the key format `mcp:<server>:<tool_name>`.

## Agent Profiles

Profiles group tools into permission sets:

| Profile | Tools Enabled | Use Case |
|---------|--------------|----------|
| **Write** | All built-in tools | Full agentic coding |
| **Ask** | Read-only tools only | Code questions without modification |
| **Minimal** | No tools | General conversation |
| **Custom** | User-defined | Via `agent.profiles` in settings or UI |

Access: `agent: manage profiles` or `cmd-alt-p` / `ctrl-alt-p`.

## Sources

- [Official] [Tools](https://zed.dev/docs/ai/tools) — Primary tool reference
- [Official] [Agent Panel](https://zed.dev/docs/ai/agent-panel) — Profiles and agent UI
- [Official] [Agent Settings](https://zed.dev/docs/ai/agent-settings) — Tool permissions config
- [Official] [Tool Permissions](https://zed.dev/docs/ai/tool-permissions) — Permission rules and priority
