# GitHub Copilot CLI Hooks Reference

Comprehensive documentation of GitHub Copilot CLI's hook/lifecycle system, researched from official GitHub documentation and community sources. Last updated: 2026-03-21.

## Source Attribution Key

- `[Official]` -- from docs.github.com official documentation
- `[Community]` -- from community blogs, tutorials, or GitHub examples
- `[Inferred]` -- logically derived from official docs but not explicitly stated
- `[Unverified]` -- mentioned in search results but not confirmed in primary sources

---

## Overview

Hooks are user-defined shell commands that execute at specific lifecycle points during a Copilot CLI session. They provide **deterministic** control -- guaranteed to execute when their event fires, unlike custom instructions which guide but don't enforce behavior. `[Official]`

Hooks receive structured JSON input via stdin describing the current event (tool being called, arguments, session context) and can optionally respond with JSON output to influence Copilot's behavior (e.g., deny a tool call). `[Official]`

**Key properties:**
- Hooks run **synchronously** and block agent execution `[Official]`
- Keep execution under **5 seconds** for optimal responsiveness `[Official]`
- Multiple hooks of the same type execute **sequentially** in definition order `[Official]`
- For `preToolUse`, if **any** hook returns "deny", the tool is blocked `[Official]`
- Hook failures (non-zero exit, timeout) are **logged and skipped** -- they never block agent execution `[Official]`

**Shared format with Coding Agent:** Commit one configuration file to your repository and it works in both Copilot Coding Agent and Copilot CLI. Coding Agent requires hooks on the default branch; CLI reads from `.github/hooks/*.json` in the current working directory. `[Official]`

---

## Hook Events (Complete List)

8 events total. `[Official]`

| Event | When It Fires | Can Block |
|-------|---------------|-----------|
| `sessionStart` | New session begins, existing session resumes, or startup | No |
| `sessionEnd` | Session completes or is terminated | No |
| `userPromptSubmitted` | User submits a prompt to the agent | No |
| `preToolUse` | Before a tool call executes (bash, edit, view, etc.) | Yes |
| `postToolUse` | After a tool call completes (success or failure) | No |
| `subagentStop` | Subagent finishes execution | No |
| `agentStop` | Main agent stops execution | No |
| `errorOccurred` | Error occurs during agent execution | No |

The `subagentStop` event was added in CLI version 1.0.9 alongside a `subagentStart` hook for injecting context into spawned subagents. `[Official]`

---

## Configuration Format

Hooks are JSON files stored in `.github/hooks/*.json` in the repository. You can name the file anything (e.g., `security.json`, `logging.json`). `[Official]`

**Personal hooks** can also be loaded from `~/.copilot/hooks/` in addition to repo-level hooks. `[Community]`

### Top-Level Structure

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [ ... ],
    "postToolUse": [ ... ],
    "sessionStart": [ ... ],
    "sessionEnd": [ ... ],
    "userPromptSubmitted": [ ... ],
    "errorOccurred": [ ... ],
    "subagentStop": [ ... ],
    "agentStop": [ ... ]
  }
}
```

`[Official]`

### Hook Entry Properties

| Property | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `type` | string | Yes | -- | Must be `"command"` |
| `bash` | string | OS-dependent | -- | Shell command or script path (macOS/Linux) |
| `powershell` | string | OS-dependent | -- | PowerShell command or script path (Windows) |
| `cwd` | string | No | repo root | Working directory (relative to repo root) |
| `env` | object | No | `{}` | Custom environment variables merged with existing |
| `timeoutSec` | number | No | `30` | Timeout in seconds |
| `comment` | string | No | -- | Optional description of the hook's purpose |

`[Official]`

### Example: Complete Hook File

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "bash": "./scripts/block-dangerous-commands.sh",
        "powershell": "./scripts/block-dangerous-commands.ps1",
        "cwd": "scripts",
        "timeoutSec": 10,
        "comment": "Block rm -rf and other dangerous bash commands"
      }
    ],
    "sessionStart": [
      {
        "type": "command",
        "bash": "./scripts/log-session.sh",
        "env": { "AUDIT_LOG": "/var/log/copilot-audit.log" },
        "comment": "Log session start for auditing"
      }
    ]
  }
}
```

---

## Matcher Syntax

### Native Copilot Format

Copilot's native hook format does **not** have a built-in matcher field. Filtering by tool name is done **inside the hook script** by reading the `toolName` field from the JSON stdin input. `[Official]`

```bash
#!/bin/bash
INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.toolName')

# Only validate bash commands -- allow everything else
if [ "$TOOL_NAME" != "bash" ]; then
  exit 0
fi

# ... validation logic for bash commands ...
```

### Claude Code Nested Matcher Format (Supported)

Copilot CLI also supports Claude Code's nested `matcher`/`hooks` structure with regex-based tool name filtering. This was added for cross-provider compatibility. `[Official]`

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "matcher": "bash|execute",
        "hooks": [
          {
            "type": "command",
            "bash": "./scripts/validate-bash.sh"
          }
        ]
      }
    ]
  }
}
```

The `matcher` value is a regex pattern matched against the tool name. Only matching tool calls trigger the nested hooks. `[Inferred]`

**Note:** VS Code currently parses but ignores matcher values -- all hooks run on every matching event regardless of matcher. CLI and Coding Agent respect matchers. `[Official]`

---

## Input JSON Structure

Each hook receives event-specific JSON via stdin. `[Official]`

### Common Fields (All Events)

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | number | Unix timestamp in milliseconds |
| `cwd` | string | Current working directory |

### sessionStart

| Field | Type | Values |
|-------|------|--------|
| `source` | string | `"new"`, `"resume"`, or `"startup"` |
| `initialPrompt` | string | The initial prompt text (if any) |

### sessionEnd

| Field | Type | Values |
|-------|------|--------|
| `reason` | string | `"complete"`, `"error"`, `"abort"`, `"timeout"`, or `"user_exit"` |

### userPromptSubmitted

| Field | Type | Description |
|-------|------|-------------|
| `prompt` | string | The submitted prompt text |

### preToolUse

| Field | Type | Description |
|-------|------|-------------|
| `toolName` | string | Name of the tool being called (e.g., `"bash"`, `"edit"`, `"view"`) |
| `toolArgs` | string | JSON-encoded string of tool arguments |

### postToolUse

| Field | Type | Description |
|-------|------|-------------|
| `toolName` | string | Name of the tool that was called |
| `toolArgs` | string | JSON-encoded string of tool arguments |
| `toolResult` | object | Contains `resultType` and `textResultForLlm` |

### errorOccurred

| Field | Type | Description |
|-------|------|-------------|
| `error` | object | Contains `message`, `name`, `stack` |

---

## Output JSON Structure

Only `preToolUse` hooks produce meaningful output. All other hook types ignore stdout. `[Official]`

### preToolUse Response

```json
{
  "permissionDecision": "allow" | "deny" | "ask",
  "permissionDecisionReason": "Human-readable explanation"
}
```

| Decision | Effect |
|----------|--------|
| `"allow"` | Tool call proceeds (same as no output) |
| `"deny"` | Tool call is blocked; reason shown to agent |
| `"ask"` | Prompt user for confirmation before proceeding |

The `"ask"` decision was added in a later release. `[Official]`

**Exit codes:** `0` for success, non-zero for errors. A non-zero exit code does not block execution -- it is logged and the hook is treated as a no-op. `[Official]`

---

## Execution Model

### Timing and Ordering

- Hooks run **synchronously** -- the agent waits for each hook to complete `[Official]`
- Multiple hooks of the same event type run **sequentially in array order** `[Official]`
- For `preToolUse`: first "deny" short-circuits -- remaining hooks are skipped `[Official]`

### Failure Handling

- Hook failures (non-zero exit, timeout, crash) are **logged and skipped** `[Official]`
- A broken hook script never blocks the agent workflow `[Official]`
- This is intentional design -- reliability of the agent takes priority over hook enforcement `[Official]`

### Timeout

- Default: **30 seconds** per hook `[Official]`
- Configurable via `timeoutSec` property `[Official]`
- Recommended: keep hooks under **5 seconds** for responsiveness `[Official]`

### Environment Variables

Custom environment variables can be set via the `env` property in the hook configuration. These are merged with the existing process environment. `[Official]`

No special Copilot-injected environment variables are documented beyond what's available in the standard process environment. `[Inferred]`

---

## VS Code Compatibility

VS Code parses Copilot CLI hook configurations and maps between naming conventions: `[Official]`

| Copilot CLI (lowerCamelCase) | VS Code (PascalCase) |
|------------------------------|----------------------|
| `preToolUse` | `PreToolUse` |
| `postToolUse` | `PostToolUse` |
| `sessionStart` | `SessionStart` |
| `sessionEnd` | `SessionEnd` |
| `userPromptSubmitted` | `UserPromptSubmitted` |

The `bash` property maps to macOS/Linux and `powershell` maps to Windows. `[Official]`

---

## Comparison with Claude Code Hooks

| Feature | Copilot CLI | Claude Code |
|---------|-------------|-------------|
| Event count | 8 | 22 |
| Event naming | lowerCamelCase | PascalCase |
| Configuration file | `.github/hooks/*.json` | `.claude/settings.json` or `.claude/settings.local.json` |
| Execution model | Sequential, blocking | Parallel |
| Matcher syntax | In-script filtering (native) or nested `matcher` (Claude Code compat) | Built-in `matcher` field with regex |
| Hook types | `command` only | `command`, `http`, `prompt`, `agent` |
| Failure behavior | Log and skip | Configurable |
| Deny mechanism | JSON output `permissionDecision` | JSON output + exit codes |
| Timeout default | 30 seconds | 120 seconds |
| Personal hooks dir | `~/.copilot/hooks/` | N/A (user-level settings) |

---

## Source Index

1. [Hooks configuration reference](https://docs.github.com/en/copilot/reference/hooks-configuration) `[Official]`
2. [About hooks](https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks) `[Official]`
3. [Using hooks with Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/use-hooks) `[Official]`
4. [Using hooks with coding agent](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/use-hooks) `[Official]`
5. [Copilot CLI hooks tutorial](https://docs.github.com/en/copilot/tutorials/copilot-cli-hooks) `[Official]`
6. [GitHub Copilot Hooks Complete Guide](https://smartscope.blog/en/generative-ai/github-copilot/github-copilot-hooks-guide/) `[Community]`
7. [Copilot CLI Tips & Tricks: Hooks](https://bartwullems.blogspot.com/2026/03/github-copilot-cli-tips-tricks-part-4.html) `[Community]`
8. [awesome-copilot hooks docs](https://github.com/github/awesome-copilot/blob/main/docs/README.hooks.md) `[Community]`
9. [VS Code hooks documentation](https://code.visualstudio.com/docs/copilot/customization/hooks) `[Official]`
10. [Copilot CLI changelog](https://github.com/github/copilot-cli/blob/main/changelog.md) `[Official]`
