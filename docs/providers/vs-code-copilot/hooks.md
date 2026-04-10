# VS Code Copilot Hooks System

VS Code Copilot (GitHub Copilot's agent mode in VS Code) introduced hooks as a
preview feature in the February 2026 release (v1.110). Hooks let you run custom
shell commands at defined points in the agent lifecycle -- before/after specific
agent actions. They are deterministic programs (shell scripts, executables), not
LLM-driven.

**Identity note:** "VS Code Copilot" refers to GitHub Copilot's agent mode
within VS Code (`vs-code-copilot`). Distinct from `copilot-cli` (GitHub Copilot
in the terminal, `gh copilot`). Both are GitHub Copilot products but with
separate hook systems and configs.

**Status:** Preview feature as of March 2026. Configuration format and behavior
may change. [Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

## Supported Hook Events

Eight lifecycle events, each firing at a specific point in the agent loop:

| Event | When it fires | Can block? |
|---|---|---|
| `SessionStart` | User submits the first prompt of a new session | No |
| `UserPromptSubmit` | User submits a prompt | No |
| `PreToolUse` | Before agent invokes any tool | Yes |
| `PostToolUse` | After tool completes successfully | No (informational) |
| `PreCompact` | Before conversation context is compacted | No |
| `SubagentStart` | Subagent is spawned | No |
| `SubagentStop` | Subagent completes | No |
| `Stop` | Agent session ends | No |

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

**Note on event naming:** VS Code uses PascalCase event names (`PreToolUse`,
`PostToolUse`). When loading Copilot CLI hook configs, VS Code auto-converts
lowerCamelCase (`preToolUse`) to PascalCase. [Official]

### Comparison with Copilot CLI (Coding Agent) Events

The Copilot CLI / coding agent hook system (configured via `.github/hooks/`)
uses a different set of six events with lowerCamelCase naming:

| Copilot CLI Event | VS Code Equivalent |
|---|---|
| `sessionStart` | `SessionStart` |
| `sessionEnd` | `Stop` |
| `userPromptSubmitted` | `UserPromptSubmit` |
| `preToolUse` | `PreToolUse` |
| `postToolUse` | `PostToolUse` |
| `errorOccurred` | No direct equivalent |

[Official: https://docs.github.com/en/copilot/reference/hooks-configuration]

## Configuration Format

Hooks are defined in JSON files. The structure follows the same pattern as
Claude Code hooks but with platform-specific command fields:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "type": "command",
        "command": "./scripts/check-safety.sh",
        "windows": "powershell -File scripts\\check-safety.ps1",
        "linux": "./scripts/check-safety.sh",
        "osx": "./scripts/check-safety-mac.sh",
        "cwd": "relative/path",
        "env": { "MY_VAR": "value" },
        "timeout": 30
      }
    ]
  }
}
```

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

### Command Properties

| Property | Type | Required | Default | Description |
|---|---|---|---|---|
| `type` | string | Yes | — | Must be `"command"` |
| `command` | string | No | — | Cross-platform default command |
| `windows` | string | No | — | Windows-specific command override |
| `linux` | string | No | — | Linux-specific command override |
| `osx` | string | No | — | macOS-specific command override |
| `cwd` | string | No | Repo root | Working directory (relative to repo root) |
| `env` | object | No | `{}` | Environment variables injected into the command |
| `timeout` | number | No | 30 | Max seconds before timeout |

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

**Copilot CLI format compatibility:** When VS Code reads Copilot CLI hook
configs, `bash` maps to `osx` + `linux`, and `powershell` maps to `windows`.
[Official]

### File Locations

VS Code discovers hooks from multiple locations:

| Scope | Path |
|---|---|
| Workspace | `.github/hooks/*.json` |
| Workspace | `.claude/settings.json`, `.claude/settings.local.json` |
| User (global) | `~/.copilot/hooks/` |
| User (global) | `~/.claude/settings.json` |
| Custom agent | `hooks` field in `.agent.md` YAML frontmatter |
| Agent plugin | `hooks.json` or `hooks/hooks.json` in plugin directory |

The `chat.hookFilesLocations` VS Code setting can customize discovery paths.
[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

## Execution Model

- Hooks receive input as **JSON on stdin**
- Commands run as **separate processes** with VS Code's permissions
- Multiple hooks for the same event execute **sequentially**
- `PreToolUse` is the only event that can **block** agent actions
- All other events are **informational** -- their output does not affect agent
  behavior (aside from `systemMessage` injection)

### Hook Input (stdin)

All hooks receive a JSON object with common fields:

```json
{
  "timestamp": "2026-03-15T10:30:00Z",
  "cwd": "/workspace/path",
  "sessionId": "unique-identifier",
  "hookEventName": "PreToolUse",
  "transcript_path": "/path/to/transcript.json"
}
```

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

Plus event-specific fields:

| Event | Extra fields |
|---|---|
| `SessionStart` | `source` |
| `UserPromptSubmit` | `prompt` |
| `PreToolUse` | `tool_name`, `tool_input`, `tool_use_id` |
| `PostToolUse` | `tool_name`, `tool_input`, `tool_use_id`, `tool_response` |
| `PreCompact` | `trigger` |
| `SubagentStart` | `agent_id`, `agent_type` |
| `SubagentStop` | `stop_hook_active` |
| `Stop` | `stop_hook_active` |

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

### Hook Output (stdout)

Common output fields available to all hooks:

```json
{
  "continue": true,
  "stopReason": "Policy violation",
  "systemMessage": "Warning text injected into context"
}
```

[Official]

#### Exit Codes

| Exit code | Meaning |
|---|---|
| `0` | Success -- parse JSON output |
| `2` | Block the action (deny permission) |
| Other | Hook failed -- action proceeds (warning logged) |

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

### PreToolUse Output (Blocking)

`PreToolUse` is the only hook that can programmatically approve or deny tool
execution:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "permissionDecisionReason": "Command is safe",
    "updatedInput": {},
    "additionalContext": "Extra context for the agent"
  }
}
```

`permissionDecision` values: `"allow"`, `"deny"`, `"ask"`

When multiple hooks return different decisions, the **most restrictive wins**:
`deny` > `ask` > `allow`. [Official]

### Control Flow Priority

When multiple control mechanisms are used together, the most restrictive wins.
For example, if a hook returns `continue: false` and `permissionDecision:
"allow"`, the session still stops. [Official]

- `continue: false` stops the **entire agent session**
- `permissionDecision: "deny"` blocks a **single tool call**
- Exit code `2` is equivalent to `permissionDecision: "deny"`

## Matchers

**Current limitation:** Hook matchers (tool name filters like `"Edit|Write"`)
are parsed but **not applied**. All hooks run on every matching event regardless
of the tool name in the matcher. This is a known limitation of the preview
release. [Official]

This differs from Claude Code, where matchers filter by tool name.

## Environment Variable Injection

The `env` field on each hook entry injects variables into the hook command's
environment. Additionally, `SessionStart` hooks can return `env` in their
output to set session-scoped variables that propagate to all subsequent hooks:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "Project uses monorepo layout"
  },
  "env": { "PROJECT_TYPE": "monorepo" }
}
```

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

## Examples

### Auto-Format After File Edits

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "type": "command",
        "command": "npx prettier --write $TOOL_INPUT_FILE",
        "timeout": 10
      }
    ]
  }
}
```

### Block Dangerous Shell Commands

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "type": "command",
        "command": "./scripts/safety-check.sh",
        "env": { "BLOCKED_PATTERNS": "rm -rf|DROP TABLE|format c:" },
        "timeout": 5
      }
    ]
  }
}
```

## Debugging

- Right-click in the Chat view and select **Diagnostics** to see loaded hooks
  and validation errors
- Open the Output panel and select **"GitHub Copilot Chat Hooks"** from the
  channel list to see execution logs
- JSON parse errors and command failures are logged to this output channel

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

## Security Considerations

Hooks execute with the same permissions as VS Code. Recommended safeguards:

- Review all hook scripts before enabling, especially in shared repositories
- Validate and sanitize all input received from the agent
- Use the `chat.tools.edits.autoApprove` setting to prevent the agent from
  editing hook scripts during its own run
- Use environment variables for secrets -- never hardcode them in hook configs
- Enterprise organizations can disable hooks entirely via policy

[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

## Comparison with Other Providers

| Feature | VS Code Copilot | Claude Code | Cursor |
|---|---|---|---|
| Config format | JSON (`.github/hooks/*.json`) | JSONC (`.claude/settings.json`) | JSON (`.cursor/hooks.json`) |
| Events | 8 | 4 | 6 (agent) + 18 (extended) |
| Event naming | PascalCase | PascalCase | camelCase |
| Input mechanism | JSON on stdin | Environment variables | JSON on stdin |
| Output mechanism | JSON on stdout + exit code | Exit code (0=allow, 2=deny) | JSON on stdout + exit code |
| Platform commands | `windows`/`linux`/`osx` fields | Single `command` | Single `command` |
| Blocking support | Yes (`PreToolUse` only) | Yes (`PreToolUse`) | Yes (multiple events) |
| Matchers | Parsed but not applied (preview) | Tool name matching | Tool/command pattern matching |
| Agent-scoped hooks | Yes (`.agent.md` frontmatter) | No | No |
| Enterprise governance | Policy-disableable | N/A | Dashboard (Enterprise) |

## Sources

- [Agent hooks in VS Code (Preview)](https://code.visualstudio.com/docs/copilot/customization/hooks) [Official]
- [Making agents practical for real-world development](https://code.visualstudio.com/blogs/2026/03/05/making-agents-practical-for-real-world-development) [Official]
- [Hooks configuration reference - GitHub Docs](https://docs.github.com/en/copilot/reference/hooks-configuration) [Official]
- [About hooks - GitHub Docs](https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks) [Official]
- [Using hooks with GitHub Copilot agents](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/use-hooks) [Official]
- [VS Code Copilot settings reference](https://code.visualstudio.com/docs/copilot/reference/copilot-settings) [Official]
- [GitHub Copilot Hooks Complete Guide - SmartScope](https://smartscope.blog/en/generative-ai/github-copilot/github-copilot-hooks-guide/) [Community]
