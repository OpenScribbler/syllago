# Claude Code Hooks Reference

Comprehensive documentation of Claude Code's hook/lifecycle system, researched from official Anthropic documentation and community sources. Last updated: 2026-03-20.

## Source Attribution Key

- `[Official]` -- from docs.anthropic.com / code.claude.com official documentation
- `[Community]` -- from community blogs, tutorials, or GitHub examples
- `[Inferred]` -- logically derived from official docs but not explicitly stated
- `[Unverified]` -- mentioned in search results but not confirmed in primary sources

---

## Overview

Hooks are user-defined shell commands, HTTP endpoints, LLM prompts, or agents that execute automatically at specific lifecycle points in Claude Code. They provide **deterministic** control over Claude Code's behavior -- guaranteed to run when their event fires, unlike CLAUDE.md instructions which are suggestions the LLM may deprioritize in long conversations. `[Official]`

Hooks communicate with Claude Code through stdin, stdout, stderr, and exit codes. When an event fires, Claude Code passes event-specific data as JSON to the hook. The hook processes the data and communicates its decision back via exit code and optional JSON output. `[Official]`

**Key properties:**
- All matching hooks for an event run **in parallel** `[Official]`
- Identical hook commands are **automatically deduplicated** (by command string for command hooks, by URL for HTTP hooks) `[Official]`
- Hooks fire for **subagent actions** too -- if Claude spawns a subagent, PreToolUse/PostToolUse hooks execute for every tool the subagent uses `[Official]`

---

## Hook Events (Complete List)

22 events total. Ordered by lifecycle phase: session setup, agentic loop, session end. `[Official]`

| Event | When It Fires | Supports Matchers | Can Block |
|-------|---------------|-------------------|-----------|
| `SessionStart` | Session begins or resumes | Yes (source: startup, resume, clear, compact) | No |
| `InstructionsLoaded` | CLAUDE.md or .claude/rules/*.md loaded into context | Yes (load reason) | No |
| `UserPromptSubmit` | User submits a prompt, before Claude processes it | No | Yes |
| `PreToolUse` | Before a tool call executes | Yes (tool name) | Yes |
| `PermissionRequest` | Permission dialog is about to appear | Yes (tool name) | Yes |
| `PostToolUse` | After a tool call succeeds | Yes (tool name) | No |
| `PostToolUseFailure` | After a tool call fails | Yes (tool name) | No |
| `Notification` | Claude Code sends a notification | Yes (notification type) | No |
| `SubagentStart` | Subagent is spawned | Yes (agent type) | No |
| `SubagentStop` | Subagent finishes | Yes (agent type) | Yes |
| `Stop` | Claude finishes responding | No | Yes |
| `StopFailure` | Turn ends due to API error | Yes (error type) | No |
| `TeammateIdle` | Agent team teammate about to go idle | No | Yes |
| `TaskCompleted` | Task being marked as completed | No | Yes |
| `ConfigChange` | Configuration file changes during session | Yes (config source) | Yes |
| `WorktreeCreate` | Git worktree being created (via --worktree or isolation: "worktree") | No | Yes |
| `WorktreeRemove` | Git worktree being removed (session exit or subagent finish) | No | No |
| `PreCompact` | Before context compaction | Yes (trigger type) | No |
| `PostCompact` | After context compaction completes | Yes (trigger type) | No |
| `Elicitation` | MCP server requests user input during a tool call | Yes (MCP server name) | Yes |
| `ElicitationResult` | User responds to MCP elicitation, before response sent to server | Yes (MCP server name) | Yes |
| `SessionEnd` | Session terminates | Yes (end reason) | No |

**Additionally:** A `Setup` event exists in the TypeScript SDK (`SetupHookInput` type) but is not documented in the main hooks reference page. It supports `additionalContext` output. `[Unverified]`

---

## Configuration Format

### Settings File Structure

Hooks are defined under the `"hooks"` key in a settings JSON file: `[Official]`

```json
{
  "hooks": {
    "EVENT_NAME": [
      {
        "matcher": "regex_pattern",
        "hooks": [
          {
            "type": "command",
            "command": "your-script.sh",
            "timeout": 600,
            "statusMessage": "Running validation...",
            "async": false,
            "once": false
          }
        ]
      }
    ]
  },
  "disableAllHooks": false
}
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `matcher` | string (regex) | No | Filter when hooks fire. Omit, use `""`, or `"*"` to match all. |
| `hooks` | array | Yes | Array of hook handler objects. |
| `type` | string | Yes | `"command"`, `"http"`, `"prompt"`, or `"agent"`. |
| `command` | string | Yes (command) | Shell command to execute. |
| `url` | string | Yes (http) | HTTP endpoint URL. |
| `prompt` | string | Yes (prompt/agent) | LLM prompt text. Supports `$ARGUMENTS` variable. |
| `model` | string | No (prompt/agent) | Model to use. Defaults to fast model (Haiku) for prompt, sonnet for agent. |
| `headers` | object | No (http) | HTTP headers. Values support `$VAR_NAME` interpolation. |
| `allowedEnvVars` | array | No (http) | Env vars allowed in header interpolation. |
| `timeout` | number | No | Timeout in seconds. Defaults vary by type. |
| `statusMessage` | string | No | Message shown while hook runs. |
| `async` | boolean | No (command) | Run in background without blocking. |
| `once` | boolean | No | Skills only -- run only once. |

`[Official]`

### Default Timeouts

| Hook Type | Default Timeout |
|-----------|----------------|
| Command | 600 seconds (10 minutes) |
| HTTP | 30 seconds |
| Prompt | 30 seconds |
| Agent | 60 seconds |
| SessionEnd (all types) | 1.5 seconds |

`[Official]`

The SessionEnd timeout can be overridden globally via the `CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS` environment variable. `[Official]`

---

## Configuration File Locations

Settings files are checked in order from most specific to least. Hooks from all applicable files are merged. `[Official]`

| Location | Scope | Shareable | Notes |
|----------|-------|-----------|-------|
| `.claude/settings.local.json` | Single project | No (gitignored) | Project-local overrides |
| `.claude/settings.json` | Single project | Yes (committable) | Shared project hooks |
| `~/.claude/settings.json` | All projects | No (local machine) | User global hooks |
| Managed policy settings | Organization-wide | Yes (admin-controlled) | Enterprise policies |
| Plugin `hooks/hooks.json` | When plugin enabled | Yes (bundled) | Plugin-scoped hooks |
| Skill/Agent frontmatter | While component active | Yes (in component file) | Component-scoped hooks |

`[Official]`

**Disabling hooks:** Set `"disableAllHooks": true` in a settings file. User settings cannot disable managed (enterprise) hooks. `[Official]`

**Live reloading:** If you edit settings files while Claude Code is running, the file watcher normally picks up hook changes automatically. `[Official]`

**The /hooks menu:** Type `/hooks` inside Claude Code to browse all configured hooks grouped by event. Read-only -- editing requires modifying JSON files directly. Source labels: `[User]`, `[Project]`, `[Local]`, `[Plugin]`, `[Session]`, `[Built-in]`. `[Official]`

---

## Hook Types

### 1. Command Hooks (`type: "command"`)

Run a shell command. The most common hook type. `[Official]`

```json
{
  "type": "command",
  "command": "bash $CLAUDE_PROJECT_DIR/.claude/hooks/my-hook.sh",
  "async": false
}
```

- Receives event JSON on **stdin**
- Returns decisions via **exit code** + **stdout** (JSON) + **stderr** (messages)
- Runs in the current directory with Claude Code's environment
- Shell profile is sourced (can cause issues -- see Troubleshooting)

### 2. HTTP Hooks (`type: "http"`)

POST event data to an HTTP endpoint. `[Official]`

```json
{
  "type": "http",
  "url": "http://localhost:8080/hooks/tool-use",
  "headers": {
    "Authorization": "Bearer $MY_TOKEN"
  },
  "allowedEnvVars": ["MY_TOKEN"]
}
```

- Receives same JSON as command hooks, sent as POST body
- Returns results through HTTP response body (same JSON format as command hooks)
- Header values support `$VAR_NAME` / `${VAR_NAME}` interpolation
- Only variables in `allowedEnvVars` are resolved
- HTTP status codes alone cannot block actions -- must use JSON response body
- Error handling: 2xx with empty body = success; 2xx with JSON = structured control; non-2xx/timeout/connection failure = non-blocking error

### 3. Prompt Hooks (`type: "prompt"`)

Single-turn LLM evaluation for judgment-based decisions. `[Official]`

```json
{
  "type": "prompt",
  "prompt": "Should this tool call be allowed? Input: $ARGUMENTS",
  "model": "fast_model"
}
```

- The model returns `{"ok": true}` or `{"ok": false, "reason": "explanation"}`
- `ok: false` blocks the action and feeds the reason back to Claude
- Default model: Haiku (fast model)
- Default timeout: 30 seconds

### 4. Agent Hooks (`type: "agent"`)

Multi-turn verification with tool access (Read, Grep, Glob, etc.). `[Official]`

```json
{
  "type": "agent",
  "prompt": "Verify that all unit tests pass. Run the test suite. $ARGUMENTS",
  "timeout": 120
}
```

- Spawns a subagent that can use tools to inspect codebase
- Same `{"ok": true/false, "reason": "..."}` response format as prompt hooks
- Up to 50 tool-use turns
- Default timeout: 60 seconds
- Use when verification requires inspecting files or running commands

---

## Matcher Syntax

Matchers are **regex patterns** that filter when hooks fire. Case-sensitive. `[Official]`

### Matcher Values by Event

| Event | Matches Against | Example Values |
|-------|-----------------|----------------|
| `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PermissionRequest` | Tool name | `Bash`, `Edit\|Write`, `mcp__.*`, `mcp__github__.*` |
| `SessionStart` | Session source | `startup`, `resume`, `clear`, `compact` |
| `SessionEnd` | End reason | `clear`, `resume`, `logout`, `prompt_input_exit`, `bypass_permissions_disabled`, `other` |
| `Notification` | Notification type | `permission_prompt`, `idle_prompt`, `auth_success`, `elicitation_dialog` |
| `SubagentStart`, `SubagentStop` | Agent type | `Bash`, `Explore`, `Plan`, or custom agent names |
| `PreCompact`, `PostCompact` | Trigger type | `manual`, `auto` |
| `ConfigChange` | Config source | `user_settings`, `project_settings`, `local_settings`, `policy_settings`, `skills` |
| `StopFailure` | Error type | `rate_limit`, `authentication_failed`, `billing_error`, `invalid_request`, `server_error`, `max_output_tokens`, `unknown` |
| `InstructionsLoaded` | Load reason | `session_start`, `nested_traversal`, `path_glob_match`, `include`, `compact` |
| `Elicitation`, `ElicitationResult` | MCP server name | Your configured MCP server names |
| `UserPromptSubmit`, `Stop`, `TeammateIdle`, `TaskCompleted`, `WorktreeCreate`, `WorktreeRemove` | N/A | No matcher support -- always fires |

`[Official]`

### MCP Tool Name Format

MCP tools follow the pattern `mcp__<server>__<tool>`. `[Official]`

Examples:
- `mcp__github__search_repositories`
- `mcp__filesystem__read_file`
- `mcp__memory__create_entities`

Matcher patterns:
- `mcp__github__.*` -- all tools from the GitHub server
- `mcp__.*__write.*` -- any write tool across all servers
- `mcp__.*` -- all MCP tools

### Built-in Tool Names

Tools that can appear in PreToolUse/PostToolUse matchers: `[Official]`

`Bash`, `Edit`, `Write`, `Read`, `Glob`, `Grep`, `Agent`, `WebFetch`, `WebSearch`, plus any MCP tools.

---

## Exit Codes

| Exit Code | JSON Processing | Effect |
|-----------|-----------------|--------|
| 0 | Yes -- stdout parsed as JSON | Action proceeds (unless JSON says otherwise) |
| 2 | No -- JSON ignored | Blocking error (event-dependent, see below) |
| Other non-zero | No | Non-blocking error; stderr logged in verbose mode |

`[Official]`

### Exit Code 2 Behavior by Event

| Event | Effect of Exit 2 |
|-------|------------------|
| `PreToolUse` | Blocks the tool call; stderr fed to Claude as feedback |
| `PermissionRequest` | Denies permission |
| `UserPromptSubmit` | Blocks prompt, erases from context |
| `Stop` | Prevents Claude from stopping (forces continuation) |
| `SubagentStop` | Prevents subagent from stopping |
| `TeammateIdle` | Teammate gets stderr feedback, continues working |
| `TaskCompleted` | Task not marked complete; stderr fed to model |
| `ConfigChange` | Blocks config change (except policy_settings) |
| `Elicitation` | Denies the elicitation |
| `ElicitationResult` | Blocks response, becomes decline |
| `WorktreeCreate` | Worktree creation fails |
| Non-blocking events | stderr shown/logged only; no blocking effect |

`[Official]`

---

## Environment Variables

Variables available to hook scripts: `[Official]`

| Variable | Available In | Description |
|----------|-------------|-------------|
| `$CLAUDE_PROJECT_DIR` | All hooks | Project root directory |
| `$CLAUDE_ENV_FILE` | SessionStart only | File path for persisting environment variables |
| `${CLAUDE_PLUGIN_ROOT}` | Plugin hooks | Plugin installation directory |
| `${CLAUDE_PLUGIN_DATA}` | Plugin hooks | Plugin persistent data directory |
| `$CLAUDE_CODE_REMOTE` | All hooks | Set to `"true"` in remote/web environments |

Additionally, hooks run with Claude Code's full environment, so all standard environment variables are available. `[Inferred]`

### Persisting Environment Variables (SessionStart)

In `SessionStart` hooks, write `KEY=VALUE` lines to the file at `$CLAUDE_ENV_FILE` to set environment variables that persist for the entire session. `[Official]`

---

## Input Data (stdin)

### Common Input Fields (All Events)

Every event includes these fields: `[Official]`

```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/current/working/directory",
  "permission_mode": "default|plan|acceptEdits|dontAsk|bypassPermissions",
  "hook_event_name": "EventName",
  "agent_id": "agent-abc123",
  "agent_type": "AgentName"
}
```

- `agent_id` and `agent_type` are present when running in subagent context or with `--agent`

### Event-Specific Input

#### SessionStart

```json
{
  "source": "startup|resume|clear|compact",
  "model": "claude-sonnet-4-6",
  "agent_type": "AgentName"
}
```

#### InstructionsLoaded

```json
{
  "file_path": "/path/to/CLAUDE.md",
  "memory_type": "User|Project|Local|Managed",
  "load_reason": "session_start|nested_traversal|path_glob_match|include|compact",
  "globs": ["glob_patterns"],
  "trigger_file_path": "/path",
  "parent_file_path": "/path"
}
```

#### UserPromptSubmit

```json
{
  "prompt": "User's submitted text"
}
```

#### PreToolUse

```json
{
  "tool_name": "Bash",
  "tool_input": { ... },
  "tool_use_id": "toolu_..."
}
```

Tool input varies by tool:

| Tool | Key Input Fields |
|------|-----------------|
| `Bash` | `command`, `description`, `timeout`, `run_in_background` |
| `Write` | `file_path`, `content` |
| `Edit` | `file_path`, `old_string`, `new_string`, `replace_all` |
| `Read` | `file_path`, `offset`, `limit` |
| `Glob` | `pattern`, `path` |
| `Grep` | `pattern`, `path`, `glob`, `output_mode`, `-i`, `multiline` |
| `WebFetch` | `url`, `prompt` |
| `WebSearch` | `query`, `allowed_domains`, `blocked_domains` |
| `Agent` | `prompt`, `description`, `subagent_type`, `model` |

#### PermissionRequest

```json
{
  "tool_name": "...",
  "tool_input": { ... },
  "permission_suggestions": [
    {
      "type": "addRules|replaceRules|removeRules|setMode|addDirectories|removeDirectories",
      "rules": [{"toolName": "...", "ruleContent": "..."}],
      "behavior": "allow|deny|ask",
      "mode": "default|acceptEdits|dontAsk|bypassPermissions|plan",
      "directories": ["/path"],
      "destination": "session|localSettings|projectSettings|userSettings"
    }
  ]
}
```

#### PostToolUse

```json
{
  "tool_name": "...",
  "tool_input": { ... },
  "tool_response": { ... },
  "tool_use_id": "toolu_..."
}
```

#### PostToolUseFailure

```json
{
  "tool_name": "...",
  "tool_input": { ... },
  "tool_use_id": "toolu_...",
  "error": "error message",
  "is_interrupt": false
}
```

#### Notification

```json
{
  "message": "notification text",
  "title": "optional title",
  "notification_type": "permission_prompt|idle_prompt|auth_success|elicitation_dialog"
}
```

#### SubagentStart

```json
{
  "agent_id": "agent-...",
  "agent_type": "Explore|Plan|Bash|..."
}
```

#### SubagentStop

```json
{
  "stop_hook_active": false,
  "agent_id": "agent-...",
  "agent_type": "...",
  "agent_transcript_path": "/path/to/transcript",
  "last_assistant_message": "subagent's final response"
}
```

#### Stop

```json
{
  "stop_hook_active": false,
  "last_assistant_message": "Claude's final response"
}
```

The `stop_hook_active` field is critical for preventing infinite loops -- check it and exit 0 if true.

#### StopFailure

```json
{
  "error": "rate_limit|authentication_failed|billing_error|invalid_request|server_error|max_output_tokens|unknown",
  "error_details": "additional info",
  "last_assistant_message": "API error string"
}
```

#### TeammateIdle

```json
{
  "teammate_name": "teammate-name",
  "team_name": "team-name"
}
```

#### TaskCompleted

```json
{
  "task_id": "task-001",
  "task_subject": "task title",
  "task_description": "description",
  "teammate_name": "teammate",
  "team_name": "team"
}
```

#### ConfigChange

```json
{
  "source": "user_settings|project_settings|local_settings|policy_settings|skills",
  "file_path": "/path/to/settings.json"
}
```

#### WorktreeCreate

```json
{
  "name": "worktree-slug"
}
```

#### WorktreeRemove

```json
{
  "worktree_path": "/path/to/worktree"
}
```

#### PreCompact

```json
{
  "trigger": "manual|auto",
  "custom_instructions": "user-provided text"
}
```

#### PostCompact

```json
{
  "trigger": "manual|auto",
  "compact_summary": "generated summary text"
}
```

#### Elicitation (Form Mode)

```json
{
  "mcp_server_name": "server-name",
  "message": "prompt text",
  "mode": "form",
  "requested_schema": {
    "type": "object",
    "properties": { ... }
  },
  "elicitation_id": "id"
}
```

#### Elicitation (URL Mode)

```json
{
  "mcp_server_name": "server-name",
  "message": "prompt text",
  "mode": "url",
  "url": "https://auth.example.com/login"
}
```

#### SessionEnd

```json
{
  "reason": "clear|resume|logout|prompt_input_exit|bypass_permissions_disabled|other"
}
```

`[Official]`

---

## Output Format (stdout JSON)

### Common Output Fields

All events support these top-level fields: `[Official]`

```json
{
  "continue": true,
  "stopReason": "message shown when continue=false",
  "suppressOutput": false,
  "systemMessage": "warning shown to user",
  "decision": "block",
  "reason": "explanation for block",
  "hookSpecificOutput": {
    "hookEventName": "EventName"
  }
}
```

### Event-Specific Output

#### SessionStart

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "context text injected into session"
  }
}
```

Plain text stdout also becomes context (no JSON wrapper needed).

#### UserPromptSubmit

- Plain text stdout is added as context
- JSON with `additionalContext` adds context discretely
- JSON with `"decision": "block"` and `"reason"` blocks the prompt

#### PreToolUse

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow|deny|ask",
    "permissionDecisionReason": "reason text",
    "updatedInput": {
      "command": "modified command"
    },
    "additionalContext": "context for Claude"
  }
}
```

**Permission decision semantics:**
- `"allow"` -- skip the interactive permission prompt. **Does NOT override deny rules.** If a deny rule matches, the call is still blocked. If an ask rule matches, the user is still prompted. Deny rules from any settings scope (including managed/enterprise) always take precedence.
- `"deny"` -- cancel the tool call; reason is fed back to Claude
- `"ask"` -- show the permission prompt to the user as normal

**Input modification** (added in v2.0.10): `[Official]`
The `updatedInput` field lets hooks modify tool inputs before execution. Modifications are invisible to Claude. Use cases: transparent sandboxing, automatic security enforcement, team convention adherence.

#### PermissionRequest

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PermissionRequest",
    "decision": {
      "behavior": "allow|deny",
      "updatedInput": { ... },
      "updatedPermissions": [
        {
          "type": "addRules|replaceRules|removeRules|setMode|addDirectories|removeDirectories",
          "rules": [{"toolName": "..."}],
          "behavior": "allow|deny|ask",
          "mode": "default|acceptEdits|dontAsk|bypassPermissions|plan",
          "destination": "session|localSettings|projectSettings|userSettings"
        }
      ],
      "message": "deny reason (deny only)"
    }
  }
}
```

**Note:** `PermissionRequest` hooks do NOT fire in non-interactive mode (`-p`). Use `PreToolUse` hooks for automated permission decisions in headless mode. `[Official]`

#### PostToolUse

```json
{
  "decision": "block",
  "reason": "explanation",
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "additionalContext": "context",
    "updatedMCPToolOutput": "value"
  }
}
```

`updatedMCPToolOutput` is available for MCP tools only.

#### PostToolUseFailure / Notification / SubagentStart

```json
{
  "hookSpecificOutput": {
    "hookEventName": "...",
    "additionalContext": "context"
  }
}
```

#### Stop / SubagentStop

```json
{
  "decision": "block",
  "reason": "explanation why Claude should continue"
}
```

**Infinite loop prevention:** Check `stop_hook_active` in the input. If true, exit 0 immediately to let Claude stop.

#### TeammateIdle / TaskCompleted

- Exit 2: feedback via stderr, teammate continues working (TeammateIdle) or task not marked complete (TaskCompleted)
- JSON with `"continue": false`: stops the teammate entirely

#### ConfigChange

```json
{
  "decision": "block",
  "reason": "explanation"
}
```

Cannot block `policy_settings` changes.

#### WorktreeCreate

- Print the **absolute worktree path** to stdout (required)
- Any other output goes to stderr
- Non-zero exit = creation fails

#### Elicitation / ElicitationResult

```json
{
  "hookSpecificOutput": {
    "hookEventName": "Elicitation",
    "action": "accept|decline|cancel",
    "content": {
      "field_name": "field_value"
    }
  }
}
```

`[Official]`

---

## Execution Model

### Parallel Execution

All matching hooks for a single event fire **in parallel**. `[Official]`

### Deduplication

Identical handlers are automatically deduplicated: `[Official]`
- Command hooks: deduplicated by command string
- HTTP hooks: deduplicated by URL

### Async Hooks

Command hooks support `"async": true` to run in the background without blocking. Not supported for all events. Useful for logging and notifications. `[Official]`

### Timeout Behavior

A timeout on an individual command does not affect other commands running for the same event. `[Official]`

### Shell Environment

- Hooks run in the **current directory** with Claude Code's environment `[Official]`
- Shell profile (`~/.zshrc` or `~/.bashrc`) is sourced -- this can cause issues if the profile prints output (see Troubleshooting) `[Official]`
- Hooks run in **non-interactive** shells `[Official]`

### Subagent Recursive Enforcement

Hooks fire for subagent actions. If Claude spawns a subagent via the Agent tool, PreToolUse and PostToolUse hooks execute for every tool the subagent uses. Without this, subagents could bypass safety gates. `[Official]`

---

## Limitations

- Command hooks communicate through stdout/stderr/exit codes only -- cannot trigger commands or tool calls directly `[Official]`
- `PostToolUse` hooks cannot undo actions (tool already executed) `[Official]`
- `PermissionRequest` hooks do not fire in non-interactive mode (`-p`) `[Official]`
- `Stop` hooks fire whenever Claude finishes responding, not only at task completion; they do not fire on user interrupts; API errors fire `StopFailure` instead `[Official]`
- `StopFailure` output and exit code are ignored `[Official]`

---

## Troubleshooting

### JSON Validation Failed

Shell profile (`~/.zshrc`, `~/.bashrc`) may print text that gets prepended to hook JSON output. Fix by wrapping echo statements in interactive-shell guards: `[Official]`

```bash
if [[ $- == *i* ]]; then
  echo "Shell ready"
fi
```

### Hook Not Firing

- Check `/hooks` menu for correct registration `[Official]`
- Matchers are **case-sensitive** `[Official]`
- Verify correct event type (PreToolUse = before, PostToolUse = after) `[Official]`
- `PermissionRequest` does not fire in `-p` mode `[Official]`

### Stop Hook Infinite Loop

Always check `stop_hook_active` and exit 0 if true: `[Official]`

```bash
INPUT=$(cat)
if [ "$(echo "$INPUT" | jq -r '.stop_hook_active')" = "true" ]; then
  exit 0
fi
```

### Debugging

- Toggle verbose mode with `Ctrl+O` to see hook output in transcript `[Official]`
- Run `claude --debug` for full execution details including which hooks matched and exit codes `[Official]`

---

## Common Patterns

### Auto-format After Edits

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '.tool_input.file_path' | xargs npx prettier --write"
          }
        ]
      }
    ]
  }
}
```

`[Official]`

### Block Protected Files

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/protect-files.sh"
          }
        ]
      }
    ]
  }
}
```

`[Official]`

### Re-inject Context After Compaction

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'Reminder: use Bun, not npm. Run bun test before committing.'"
          }
        ]
      }
    ]
  }
}
```

`[Official]`

### Desktop Notifications

```json
{
  "hooks": {
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "notify-send 'Claude Code' 'Claude Code needs your attention'"
          }
        ]
      }
    ]
  }
}
```

`[Official]`

### Auto-approve Specific Permissions

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": "ExitPlanMode",
        "hooks": [
          {
            "type": "command",
            "command": "echo '{\"hookSpecificOutput\": {\"hookEventName\": \"PermissionRequest\", \"decision\": {\"behavior\": \"allow\"}}}'"
          }
        ]
      }
    ]
  }
}
```

`[Official]`

### Audit Config Changes

```json
{
  "hooks": {
    "ConfigChange": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "jq -c '{timestamp: now | todate, source: .source, file: .file_path}' >> ~/claude-config-audit.log"
          }
        ]
      }
    ]
  }
}
```

`[Official]`

---

## Sources

- [Hooks Reference](https://code.claude.com/docs/en/hooks) -- Official Anthropic documentation (primary source for event schemas, input/output formats, matchers)
- [Automate Workflows with Hooks (Guide)](https://code.claude.com/docs/en/hooks-guide) -- Official Anthropic guide with examples and patterns
- [Claude Code Hooks Tutorial](https://blakecrosley.com/blog/claude-code-hooks-tutorial) -- Community tutorial
- [GitButler Claude Code Hooks](https://docs.gitbutler.com/features/ai-integration/claude-code-hooks) -- Community documentation
- [Claude Code Hooks Mastery (GitHub)](https://github.com/disler/claude-code-hooks-mastery) -- Community examples repository
- [TypeScript SDK Reference](https://docs.anthropic.com/en/docs/claude-code/sdk/sdk-typescript) -- SDK hook event types
