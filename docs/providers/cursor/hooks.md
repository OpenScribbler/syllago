# Cursor Hooks System

Cursor introduced hooks in version 1.7 (2025) as a beta feature. Hooks let you
run custom scripts at defined points in the agent lifecycle -- before/after
specific agent actions. They are deterministic programs (shell scripts,
executables), not LLM-driven.

**Status:** Beta feature, still present in Cursor 2.0+. Enterprise dashboard
distribution added in 2.0. [Official]

## Supported Hook Events

Six lifecycle events, each firing at a specific point in the agent loop:

| Event | When it fires | Can block? |
|---|---|---|
| `beforeSubmitPrompt` | User submits prompt, before model receives it | No (informational only) |
| `beforeShellExecution` | Before agent runs a shell command | Yes |
| `beforeMCPExecution` | Before agent calls an MCP tool | Yes |
| `beforeReadFile` | Before file contents are sent to LLM | Yes |
| `afterFileEdit` | After agent modifies a file | No (informational only) |
| `stop` | Agent task completes, aborts, or errors | No (informational only) |

[Official: https://cursor.com/docs] [Community: https://blog.gitbutler.com/cursor-hooks-deep-dive]

## Configuration Format

Hooks are defined in `hooks.json` files. The structure:

```json
{
  "version": 1,
  "hooks": {
    "<event_name>": [
      {
        "command": "path/to/script"
      }
    ]
  }
}
```

Each event maps to an array of command objects. Multiple commands per event are
supported and run sequentially.

### File Locations

Cursor searches three locations and executes hooks from ALL discovered files:

| Scope | Path |
|---|---|
| Project | `<project>/.cursor/hooks.json` |
| User (global) | `~/.cursor/hooks.json` |
| Enterprise | `/etc/cursor/hooks.json` |

Command paths are resolved relative to the directory containing their
`hooks.json` file.

[Community: https://blog.gitbutler.com/cursor-hooks-deep-dive]

## Execution Model

- Hooks receive input as **JSON on stdin**
- Commands run as **separate processes**
- Multiple hooks execute **sequentially** across all config locations
- Blocking hooks (`beforeShellExecution`, `beforeMCPExecution`, `beforeReadFile`)
  can deny the action via structured JSON output
- Non-blocking hooks (`beforeSubmitPrompt`, `afterFileEdit`, `stop`) are
  informational -- Cursor does not process their output

### Hook Input (stdin)

All hooks receive a JSON object with common fields:

```json
{
  "conversation_id": "unique-id",
  "generation_id": "unique-id",
  "hook_event_name": "beforeShellExecution",
  "workspace_roots": ["/path/to/project"]
}
```

Plus event-specific fields:

| Event | Extra fields |
|---|---|
| `beforeSubmitPrompt` | `prompt`, `attachments` |
| `beforeShellExecution` | `command`, `cwd` |
| `beforeMCPExecution` | `server`, `tool_name`, `tool_input`, `command` |
| `beforeReadFile` | `file_path`, `content` |
| `afterFileEdit` | `file_path`, `edits` (array of `{old_string, new_string}`) |
| `stop` | `status` (`"completed"`, `"aborted"`, `"error"`) |

### Hook Output (stdout, blocking hooks only)

Blocking hooks can return JSON to control behavior:

```json
{
  "continue": true,
  "permission": "allow",
  "userMessage": "Visible to the user",
  "agentMessage": "Visible to the LLM"
}
```

`permission` values: `"allow"`, `"deny"`, `"ask"`

## Debugging

Cursor provides a dedicated "Hooks" output channel (View > Output > select
"Hooks") that shows execution logs, JSON parse errors, and command results.

[Community: https://blog.gitbutler.com/cursor-hooks-deep-dive]

## Comparison with Other Providers

| Feature | Cursor | Claude Code |
|---|---|---|
| Hook config format | JSON (`hooks.json`) | JSONC (`.claude/settings.json`) |
| Hook config location | `.cursor/hooks.json` | `.claude/settings.json` `hooks` key |
| Events supported | 6 | 4 (`PreToolUse`, `PostToolUse`, `Notification`, `Stop`) |
| Input mechanism | JSON on stdin | Environment variables |
| Output mechanism | JSON on stdout | Exit code (0=allow, 2=deny) |
| Blocking support | Yes (allow/deny/ask) | Yes (exit code based) |
| Enterprise distribution | Dashboard (2.0+) | Not applicable |

## Sources

- [Cursor 1.7 Adds Hooks for Agent Lifecycle Control - InfoQ](https://www.infoq.com/news/2025/10/cursor-hooks/) [Community]
- [Deep Dive into Cursor Hooks - GitButler Blog](https://blog.gitbutler.com/cursor-hooks-deep-dive) [Community]
- [Cursor Docs](https://cursor.com/docs) [Official]
- [How to Use Cursor 1.7 Hooks - Skywork](https://skywork.ai/blog/how-to-cursor-1-7-hooks-guide/) [Community]
- [Cursor Rules, Commands, Skills, and Hooks Guide](https://theodoroskokosioulis.com/blog/cursor-rules-commands-skills-hooks-guide/) [Community]
