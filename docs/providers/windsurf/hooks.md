# Windsurf Hooks

## Overview

Windsurf supports a lifecycle hook system called **Cascade Hooks**, introduced in v1.12.41. Hooks execute custom scripts at specific points in the Cascade agent pipeline. They are designed for compliance automation, auditing, security policy enforcement, and workflow customization.

Hooks are configured in `hooks.json` files and receive JSON payloads via stdin.

## Hook Events (12 Total)

| Event | Type | Can Block? | Description |
|-------|------|------------|-------------|
| `pre_read_code` | Pre | Yes | Before Cascade reads a code file |
| `post_read_code` | Post | No | After successful file read |
| `pre_write_code` | Pre | Yes | Before Cascade writes/modifies a file |
| `post_write_code` | Post | No | After code changes complete |
| `pre_run_command` | Pre | Yes | Before terminal command execution |
| `post_run_command` | Post | No | After command execution |
| `pre_mcp_tool_use` | Pre | Yes | Before MCP tool invocation |
| `post_mcp_tool_use` | Post | No | After MCP tool usage |
| `pre_user_prompt` | Pre | Yes | Before prompt processing |
| `post_cascade_response` | Post (async) | No | After Cascade completes a response |
| `post_cascade_response_with_transcript` | Post (async) | No | Writes full conversation to JSONL file |
| `post_setup_worktree` | Post | No | After worktree creation (added v1.13.9) |

[Official] https://docs.windsurf.com/windsurf/cascade/hooks

## Configuration File Locations

Hooks merge in order: enterprise cloud -> system -> user -> workspace.

### System-Level (organization-wide, cannot be disabled without root)

| OS | Path |
|----|------|
| macOS | `/Library/Application Support/Windsurf/hooks.json` |
| Linux/WSL | `/etc/windsurf/hooks.json` |
| Windows | `C:\ProgramData\Windsurf\hooks.json` |

### User-Level (personal preferences)

| Product | Path |
|---------|------|
| Windsurf IDE | `~/.codeium/windsurf/hooks.json` |
| JetBrains Plugin | `~/.codeium/hooks.json` |

### Workspace-Level (project-specific)

`.windsurf/hooks.json` in workspace root.

### Enterprise Cloud Dashboard

Enterprise teams can configure hooks via the cloud dashboard (requires `TEAM_SETTINGS_UPDATE` permission). These load first, before all local hooks.

[Official] https://docs.windsurf.com/windsurf/cascade/hooks

## Configuration Schema

```json
{
  "hooks": {
    "<hook_event_name>": [
      {
        "command": "string (required) — shell command to execute",
        "show_output": "boolean — display stdout/stderr in Cascade UI",
        "working_directory": "string (optional) — execution directory; defaults to workspace root; supports relative/absolute paths; ~ expansion NOT supported"
      }
    ]
  }
}
```

Multiple hooks can be registered per event (array). Each hook entry has three fields:

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `command` | Yes | string | Shell command to execute |
| `show_output` | No | boolean | Whether to display stdout/stderr in Cascade UI |
| `working_directory` | No | string | Execution directory (defaults to workspace root) |

[Official] https://docs.windsurf.com/windsurf/cascade/hooks

## Common Input Payload (stdin JSON)

All hooks receive a JSON object via stdin with these base fields:

```json
{
  "agent_action_name": "string",
  "trajectory_id": "string",
  "execution_id": "string",
  "timestamp": "ISO 8601 string",
  "tool_info": {}
}
```

## Event-Specific Payloads (tool_info)

### pre_read_code / post_read_code

```json
{ "file_path": "/path/to/file" }
```

### pre_write_code / post_write_code

```json
{
  "file_path": "/path/to/file",
  "edits": [
    { "old_string": "string", "new_string": "string" }
  ]
}
```

### pre_run_command / post_run_command

```json
{
  "command_line": "string",
  "cwd": "/path"
}
```

### pre_mcp_tool_use / post_mcp_tool_use

```json
{
  "mcp_server_name": "string",
  "mcp_tool_name": "string",
  "mcp_tool_arguments": {},
  "mcp_result": "string (post-hook only)"
}
```

### pre_user_prompt

```json
{ "user_prompt": "string" }
```

### post_cascade_response

```json
{ "response": "markdown formatted string" }
```

### post_cascade_response_with_transcript

```json
{ "transcript_path": "/Users/name/.windsurf/transcripts/{trajectory_id}.jsonl" }
```

Transcript files contain JSONL entries with `type`, `status`, and event-specific data. File permissions are set to `0600`. Windsurf maintains a 100-file limit on transcripts.

### post_setup_worktree

```json
{
  "worktree_path": "/path/to/worktree",
  "root_workspace_path": "/path/to/root"
}
```

Environment variable `$ROOT_WORKSPACE_PATH` is also available.

[Official] https://docs.windsurf.com/windsurf/cascade/hooks

## Exit Codes

| Code | Meaning | Effect |
|------|---------|--------|
| `0` | Success | Action proceeds normally |
| `2` | Block | **Pre-hooks only**: blocks the action; user sees stderr message in Cascade UI |
| Other | Error | Action proceeds normally (non-blocking) |

Only pre-hooks can block actions. Post-hooks run after the action completes and cannot block.

[Official] https://docs.windsurf.com/windsurf/cascade/hooks

## Enterprise Distribution

System-level hooks can be deployed via:
- **MDM**: Jamf, Intune, Workspace ONE
- **Config management**: Ansible, Puppet, Chef, SaltStack
- **Cloud dashboard**: Enterprise plan with `TEAM_SETTINGS_UPDATE` permission

Enterprise cloud hooks auto-distribute to all team members and load first in the merge order.

[Official] https://docs.windsurf.com/windsurf/cascade/hooks

## Syllago Mapping Notes

- Hooks use JSON merge into a single `hooks.json` file per scope level
- Hook events map to syllago canonical hook events (pre/post pattern on tool actions)
- The `command` field is the hook body; no inline script support — must be a shell command
- No concept of "hook name" or "hook description" — hooks are anonymous entries in arrays keyed by event name
- Workspace hooks go in `.windsurf/hooks.json`, not alongside code
