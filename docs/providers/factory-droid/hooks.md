# Factory Droid Hooks System

Factory Droid (Factory AI's coding agent CLI, `factory-droid`) supports lifecycle
hooks using the same JSON schema as Claude Code -- event-to-matcher-group
structure with command entries. Hooks run shell commands at defined points in the
agent lifecycle.

**Identity note:** "Factory Droid" refers to Factory AI's coding agent CLI
(`factory-droid`). No known naming conflicts with other tools as of 2026-03-30.

**Status:** Active feature in the `droid` CLI.
[Official: https://docs.factory.ai/cli/configuration/hooks-guide]

## Supported Hook Events

Nine lifecycle events, using PascalCase names:

| Event | When it fires | Can block? |
|---|---|---|
| `PreToolUse` | After tool parameters are created, before execution | Yes (exit 2) |
| `PostToolUse` | After tool call completes | Yes (exit 2 sends feedback to Droid) |
| `UserPromptSubmit` | User submits prompt, before processing | Yes (exit 2 blocks and erases prompt) |
| `Stop` | Droid finishes responding (excludes user interrupts) | Yes (exit 2 blocks stoppage) |
| `SessionStart` | Session begins or resumes | No |
| `SessionEnd` | Session terminates | No |
| `PreCompact` | Before context compaction | No |
| `SubagentStart` | Sub-droid task starts | No [Unverified] |
| `SubagentStop` | Sub-droid task completes | Yes (exit 2 blocks stoppage) |

[Official: https://docs.factory.ai/reference/hooks-reference]

Note: The hooks guide page lists `Notification` as an event, but the reference
page groups it with the other eight. The syllago adapter maps 9 events
(excluding `Notification`). Treat `Notification` support as [Unverified].

## Configuration Format

Hooks are configured in `settings.json` using the same structure as Claude Code:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Execute",
        "hooks": [
          {
            "type": "command",
            "command": "/absolute/path/to/script.sh"
          }
        ]
      }
    ]
  }
}
```

Each event maps to an array of matcher groups. Each matcher group contains a
`matcher` string and an array of hook entries.

[Official: https://docs.factory.ai/cli/configuration/hooks-guide]

### File Locations

Settings are loaded in priority order (higher overrides lower):

| Scope | Path |
|---|---|
| Project local (uncommitted) | `.factory/settings.local.json` |
| Project | `.factory/settings.json` |
| User | `~/.factory/settings.json` |
| Enterprise | Managed policies |

[Official: https://docs.factory.ai/cli/configuration/settings]

### Matcher Patterns

Matchers apply to `PreToolUse` and `PostToolUse` only:

| Pattern | Example | Behavior |
|---|---|---|
| Exact match | `"Create"` | Matches one tool |
| Regex / alternation | `"Edit\|Create"` | Matches multiple tools |
| Wildcard | `"*"` | Matches all tools |
| Empty / omitted | `""` | Matches all tools |

Common tool names: `Task`, `Execute`, `Glob`, `Grep`, `Read`, `Edit`, `Create`,
`FetchUrl`, `WebSearch`, and MCP tools (`mcp__<server>__<tool>`).

[Official: https://docs.factory.ai/reference/hooks-reference]

## Execution Model

- Hooks receive **JSON on stdin** with event-specific fields
- **Timeout:** 60 seconds default per command (configurable)
- **Parallelization:** Matching hooks execute concurrently
- **Deduplication:** Identical commands are deduplicated automatically
- **Paths:** Must be absolute; use `$FACTORY_PROJECT_DIR` for project-relative scripts

[Official: https://docs.factory.ai/reference/hooks-reference]

### Hook Input (stdin)

All hooks receive JSON with common fields:

```json
{
  "session_id": "...",
  "transcript_path": "...",
  "cwd": "...",
  "permission_mode": "...",
  "hook_event_name": "PreToolUse"
}
```

Event-specific additions:

| Event | Extra fields |
|---|---|
| `PreToolUse` / `PostToolUse` | `tool_name`, `tool_input`, `tool_response` (PostToolUse only) |
| `UserPromptSubmit` | `prompt` |
| `Stop` / `SubagentStop` | `stop_hook_active` (boolean) |
| `PreCompact` | `trigger` ("manual"/"auto"), `custom_instructions` |
| `SessionStart` | `source` ("startup"/"resume"/"clear"/"compact") |
| `SessionEnd` | `reason` ("clear"/"logout"/"prompt_input_exit"/"other") |

[Official: https://docs.factory.ai/reference/hooks-reference]

### Exit Codes

| Code | Behavior |
|---|---|
| **0** | Success; stdout shown to user (or becomes context for UserPromptSubmit/SessionStart) |
| **2** | Blocking error; stderr fed to Droid for processing |
| **Other** | Non-blocking error; stderr displayed, execution continues |

[Official: https://docs.factory.ai/reference/hooks-reference]

### Advanced JSON Output (stdout)

Hooks can return structured JSON for finer control:

```json
{
  "continue": true,
  "stopReason": "string",
  "suppressOutput": false,
  "systemMessage": "string"
}
```

**PreToolUse-specific fields:**
- `permissionDecision`: `"allow"` / `"deny"` / `"ask"`
- `permissionDecisionReason`: Displayed to user/Droid
- `updatedInput`: Modifies tool parameters before execution

**PostToolUse-specific fields:**
- `decision`: `"block"` or undefined
- `additionalContext`: Information for Droid

**Stop/SubagentStop-specific fields:**
- `decision`: `"block"` or undefined
- `reason`: Required when blocking

[Official: https://docs.factory.ai/reference/hooks-reference]

## Comparison with Claude Code

| Feature | Factory Droid | Claude Code |
|---|---|---|
| Config file | `.factory/settings.json` | `.claude/settings.json` |
| JSON schema | Same structure | Same structure |
| Events | 9 (no Notification) | 9 + Notification + ErrorOccurred |
| Tool: shell | `Execute` | `Bash` |
| Tool: create file | `Create` | `Write` |
| Tool: fetch URL | `FetchUrl` | `WebFetch` |
| Tool: subagent | `Task` | `Agent` |
| Blocking mechanism | Exit code 2 | Exit code 2 |
| MCP tool format | `mcp__server__tool` | `mcp__server__tool` |
| Timeout default | 60s | 120s [Unverified] |
| Parallel execution | Yes | No (sequential) [Unverified] |

## Environment Variables

| Variable | Description |
|---|---|
| `FACTORY_PROJECT_DIR` | Project root directory |
| `DROID_PLUGIN_ROOT` | Plugin directory |

[Official: https://docs.factory.ai/reference/hooks-reference]

## Security Note

Hooks run automatically with your environment's credentials. Review hook
implementations before registering them -- they can access any file, environment
variable, or network endpoint your shell can reach.

[Official: https://docs.factory.ai/cli/configuration/hooks-guide]

## Sources

- [Factory Droid Hooks Guide](https://docs.factory.ai/cli/configuration/hooks-guide) [Official]
- [Factory Droid Hooks Reference](https://docs.factory.ai/reference/hooks-reference) [Official]
- [Factory Droid Settings](https://docs.factory.ai/cli/configuration/settings) [Official]
