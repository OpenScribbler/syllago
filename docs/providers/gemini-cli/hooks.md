# Gemini CLI Hooks & Lifecycle System

Comprehensive reference for the Gemini CLI hook system. Hooks are scripts or programs
that execute at specific points in the agentic loop, allowing interception and
customization without modifying the CLI source.

Introduced in Gemini CLI v0.26.0. Enabled by default in Preview and Nightly channels;
Stable channel requires explicit opt-in via `settings.json`. [Official]

---

## Sources

| Label | URL |
|-------|-----|
| Official docs (overview) | https://geminicli.com/docs/hooks/ |
| Official docs (reference) | https://geminicli.com/docs/hooks/reference/ |
| Official docs (writing hooks) | https://geminicli.com/docs/hooks/writing-hooks/ |
| Official docs (best practices) | https://geminicli.com/docs/hooks/best-practices/ |
| Official docs (configuration) | https://geminicli.com/docs/reference/configuration/ |
| GitHub docs source | https://github.com/google-gemini/gemini-cli/tree/main/docs/hooks |
| Google Developers Blog | https://developers.googleblog.com/tailor-gemini-cli-to-your-workflow-with-hooks/ |
| Feature issue #9070 | https://github.com/google-gemini/gemini-cli/issues/9070 |

---

## Table of Contents

1. [Hook Events](#hook-events)
2. [Configuration Format](#configuration-format)
3. [Config File Locations](#config-file-locations)
4. [Hook Types](#hook-types)
5. [Matcher Syntax](#matcher-syntax)
6. [Execution Model](#execution-model)
7. [Communication Protocol](#communication-protocol)
8. [Base Input (All Hooks)](#base-input-all-hooks)
9. [Environment Variables](#environment-variables)
10. [Exit Codes](#exit-codes)
11. [Common Output Fields](#common-output-fields)
12. [Event-Specific I/O Schemas](#event-specific-io-schemas)
13. [Built-in Tool Names](#built-in-tool-names)
14. [Security Model](#security-model)
15. [Management Commands](#management-commands)
16. [Syllago Mapping](#syllago-mapping)

---

## Hook Events

Gemini CLI defines **11 lifecycle events** where hooks can fire. [Official]

| Event | Category | When It Fires | Typical Use |
|-------|----------|---------------|-------------|
| `SessionStart` | Lifecycle | App startup, session resume, or `/clear` command | Load initial context, inject project memories |
| `SessionEnd` | Lifecycle | CLI exit, session clear, or logout | Cleanup, final telemetry, memory consolidation |
| `BeforeAgent` | Agent | After user submits a prompt, before the agent begins planning | Prompt validation, dynamic context injection |
| `AfterAgent` | Agent | Once per turn after the model finishes its final response | Quality validation, retry logic, final logging |
| `BeforeModel` | Model | Before an LLM request is sent | Request modification, prompt engineering, mock responses |
| `AfterModel` | Model | After each LLM response chunk is received | Real-time redaction, PII filtering, streaming monitoring |
| `BeforeToolSelection` | Model | Before the LLM decides which tools to call | Filter available tools, force tool modes |
| `BeforeTool` | Tool | Before a tool executes | Validate arguments, block dangerous operations, arg rewriting |
| `AfterTool` | Tool | After a tool executes | Process results, log outputs, trigger follow-up tool calls |
| `PreCompress` | Lifecycle | Before chat history compression | Logging, state saving (advisory, async, cannot block) |
| `Notification` | Lifecycle | System alerts (errors, warnings, info) | Forward alerts, custom notification handling |

### Event Flow Diagram

```
SessionStart
    |
    v
BeforeAgent (user submits prompt)
    |
    v
BeforeModel (LLM request prepared)
    |
    v
BeforeToolSelection (filter available tools)
    |
    v
[LLM processes request]
    |
    v
AfterModel (response chunk received)
    |
    v
BeforeTool (tool about to execute)
    |
    v
[Tool executes]
    |
    v
AfterTool (tool result available)
    |
    v
[Loop continues: BeforeModel -> AfterModel -> BeforeTool -> AfterTool ...]
    |
    v
AfterAgent (final response complete)
    |
    v
SessionEnd (CLI exits)

PreCompress -----> fires async before context compression
Notification ----> fires on system alerts (any time)
```

---

## Configuration Format

Hooks are defined in `settings.json` inside the `hooks` object. Each event key maps
to an array of **matcher groups**, each containing an array of **hook definitions**. [Official]

### Full Schema

```json
{
  "hooks": {
    "<EventName>": [
      {
        "matcher": "<pattern>",
        "sequential": false,
        "hooks": [
          {
            "type": "command",
            "command": "<shell command or script path>",
            "name": "<friendly identifier>",
            "timeout": 60000,
            "description": "<purpose explanation>"
          }
        ]
      }
    ]
  }
}
```

### Matcher Group Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `matcher` | string | No | `""` (match all) | Pattern to filter when this group fires. Regex for tool events, exact string for lifecycle events. |
| `sequential` | boolean | No | `false` | When `true`, hooks in this group run sequentially instead of in parallel. [Official] |
| `hooks` | array | Yes | — | Array of hook definitions to execute when matched. |

### Hook Definition Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | Yes | — | Hook type. `"command"` for shell scripts. `"plugin"` for npm packages. [Official] |
| `command` | string | Yes (for `command` type) | — | Shell command to execute. Supports `$GEMINI_PROJECT_DIR` variable expansion. |
| `name` | string | No | — | Friendly identifier. Used in `/hooks panel` UI and fingerprinting. |
| `timeout` | number | No | `60000` | Timeout in milliseconds. The hook process is killed after this duration. |
| `description` | string | No | — | Human-readable purpose explanation. Shown in `/hooks panel`. |

### Example: Complete Configuration

```json
{
  "hooks": {
    "BeforeTool": [
      {
        "matcher": "write_file|replace",
        "hooks": [
          {
            "name": "security-check",
            "type": "command",
            "command": "$GEMINI_PROJECT_DIR/.gemini/hooks/security.sh",
            "timeout": 5000
          }
        ]
      },
      {
        "matcher": "run_shell_command",
        "hooks": [
          {
            "name": "shell-guard",
            "type": "command",
            "command": "bash .gemini/hooks/shell-guard.sh",
            "timeout": 3000
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "name": "load-context",
            "type": "command",
            "command": "bash .gemini/hooks/load-context.sh"
          }
        ]
      }
    ],
    "AfterAgent": [
      {
        "hooks": [
          {
            "name": "quality-check",
            "type": "command",
            "command": "node .gemini/hooks/validate-response.js",
            "timeout": 10000
          }
        ]
      }
    ]
  }
}
```

---

## Config File Locations

Configuration merges from multiple layers (highest precedence first): [Official]

| Level | Path | Scope |
|-------|------|-------|
| Project | `.gemini/settings.json` (in project root) | Per-project hooks |
| User | `~/.gemini/settings.json` | User-wide hooks |
| System | `/etc/gemini-cli/settings.json` | Machine-wide hooks |
| Extensions | Bundled within installed extensions | Extension-provided hooks |

### Hook System Controls

The `hooksConfig` object provides system-level management: [Official]

```json
{
  "hooksConfig": {
    "enabled": true,
    "disabled": ["hook-name-1", "hook-name-2"],
    "notifications": true
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | boolean | Master toggle for the entire hooks system |
| `disabled` | string[] | Array of specific hook names to disable |
| `notifications` | boolean | Toggle visual indicators during hook execution |

---

## Hook Types

### Command Hooks (type: `"command"`)

Execute arbitrary shell scripts or commands. Communication uses JSON-over-stdin/stdout,
mirroring Claude Code's contract. [Official]

This is the primary and most common hook type.

### Plugin Hooks (type: `"plugin"`)

Load external npm packages tagged with `geminicli-plugin`. Implement a stable TypeScript
interface with dependency injection to access core services (Logger, Config, HttpClient). [Official]

Plugin hooks use dependency injection to avoid global state and ensure loose coupling.
They are validated against their declared API version before loading.

This type is newer and less documented than command hooks. [Inferred]

---

## Matcher Syntax

Matchers determine which hooks fire for a given event. The syntax differs by event category. [Official]

### Tool Events (BeforeTool, AfterTool)

Matchers are **regular expressions** compared against the tool name being executed.

| Pattern | Matches |
|---------|---------|
| `"write_file"` | Exact match: `write_file` tool only |
| `"write_file\|replace"` | Either `write_file` or `replace` |
| `"write_.*"` | Any tool starting with `write_` |
| `"read_file\|grep_search"` | File reading tools |
| `"mcp_.*"` | All MCP tools |
| `"*"` or `""` | Wildcard: matches all tools |

### Lifecycle Events (SessionStart, SessionEnd, etc.)

Matchers are **exact strings** compared to event-specific values.

For `SessionStart`, the matcher is compared against the `source` field:
- `"startup"` — initial launch
- `"resume"` — resuming a previous session
- `"clear"` — after `/clear` command

For `SessionEnd`, the matcher is compared against the `reason` field:
- `"exit"` — normal exit
- `"clear"` — `/clear` command
- `"logout"` — user logout
- `"prompt_input_exit"` — exit from prompt
- `"other"` — other reasons

### Wildcard Matching

`"*"` or `""` (empty string) matches all occurrences for any event type. [Official]

### MCP Tool Name Format

MCP tools follow the pattern `mcp_<server_name>_<tool_name>` in Gemini CLI. [Official]

Note: This differs from Claude Code's `mcp__<server>__<tool>` format (double underscores
with `mcp__` prefix). Gemini CLI uses single underscores with `mcp_` prefix in the docs,
but syllago's toolmap uses `server__tool` (double underscores, no prefix) for the actual
matching format. [Inferred — verify against live behavior]

---

## Execution Model

### Synchronous Execution

Hooks run **synchronously** as part of the agent loop. When a hook event fires,
Gemini CLI waits for all matching hooks to complete before continuing. [Official]

This means slow hooks directly delay the agent's response time.

### Parallel vs Sequential

Within a matcher group:
- **Default (`sequential: false`)**: All hooks in the group run in parallel. [Official]
- **Sequential (`sequential: true`)**: Hooks run one after another in array order. [Official]

Multiple matcher groups matching the same event all fire. Their hooks execute according
to each group's `sequential` setting.

### Timeout

Default timeout is **60,000ms (1 minute)**. [Official]

Configurable per hook via the `timeout` field. The hook process is killed when the
timeout expires.

### Error Handling

- **Exit 0**: Success. stdout parsed as JSON for decisions.
- **Exit 2**: System block. Action rejected. stderr becomes the rejection reason.
- **Any other exit code**: Warning. Non-fatal — CLI continues with original parameters. [Official]

If stdout contains non-JSON text, parsing fails. The CLI defaults to "Allow" and
treats the entire stdout as a `systemMessage`. [Official]

---

## Communication Protocol

### The "Golden Rule"

> Your script **must not** print any plain text to `stdout` other than the final JSON object.
> Even a single `echo` or `print` call before the JSON will break parsing.
> Use `stderr` for **all** logging and debugging. [Official]

### Channels

| Channel | Direction | Purpose |
|---------|-----------|---------|
| `stdin` | CLI -> Hook | JSON input with event context |
| `stdout` | Hook -> CLI | JSON output with decisions/modifications |
| `stderr` | Hook -> CLI | Logging and debug output (captured but never parsed as JSON) |

---

## Base Input (All Hooks)

Every hook receives this base JSON structure on stdin: [Official]

```json
{
  "session_id": "string (unique session identifier)",
  "transcript_path": "string (absolute path to transcript file)",
  "cwd": "string (current working directory)",
  "hook_event_name": "string (e.g., 'BeforeTool', 'SessionStart')",
  "timestamp": "string (ISO 8601 format)"
}
```

Additional fields are added per event type (see [Event-Specific I/O Schemas](#event-specific-io-schemas)).

---

## Environment Variables

Hooks receive a sanitized environment with these variables: [Official]

| Variable | Description |
|----------|-------------|
| `GEMINI_PROJECT_DIR` | Absolute path to the project root |
| `GEMINI_SESSION_ID` | Unique ID for the current session |
| `GEMINI_CWD` | Current working directory |
| `CLAUDE_PROJECT_DIR` | Compatibility alias for `GEMINI_PROJECT_DIR` |

### Environment Variable Redaction

Sensitive environment variables can be redacted from hook environments: [Official]

```json
{
  "security": {
    "environmentVariableRedaction": {
      "enabled": true,
      "allowed": ["MY_REQUIRED_TOOL_KEY"],
      "blocked": ["SECRET_TOKEN"]
    }
  }
}
```

When enabled, variables matching patterns like `KEY`, `TOKEN`, `SECRET` are automatically
redacted unless explicitly listed in `allowed`. [Official]

---

## Exit Codes

| Code | Name | Behavior |
|------|------|----------|
| `0` | Success | `stdout` is parsed as JSON. Use this for all structured decisions, including intentional blocks. [Official] |
| `2` | System Block | Critical abort. `stderr` content becomes the rejection reason. The action is blocked. [Official] |
| Other | Warning | Non-fatal. CLI proceeds with original parameters. A warning is logged. [Official] |

### Strategy Guidance

- **Exit 0 with `"decision": "deny"`**: Preferred for intentional blocks with structured feedback. [Official]
- **Exit 2**: Emergency brake — simple denial using stderr for the reason string. [Official]

---

## Common Output Fields

Most hooks support these output fields (exceptions noted per event): [Official]

| Field | Type | Purpose |
|-------|------|---------|
| `systemMessage` | string | User-facing message displayed in the terminal |
| `suppressOutput` | boolean | Hide hook metadata from logs (terminal output still visible) |
| `continue` | boolean | `false` stops the entire agent loop immediately |
| `stopReason` | string | Message shown when `continue` is `false` |
| `decision` | string | `"allow"`, `"deny"`, or `"block"` |
| `reason` | string | Feedback sent to the agent when `decision` is `"deny"` |

---

## Event-Specific I/O Schemas

### SessionStart

**Input:**
```json
{
  "source": "startup | resume | clear"
}
```

**Output:**
```json
{
  "hookSpecificOutput": {
    "additionalContext": "string (injected as first turn or prepended to prompt)"
  },
  "systemMessage": "string (displayed at session start)"
}
```

**Notes:** Flow-control fields (`decision`, `continue`) are **ignored** — startup can never be blocked. [Official]

---

### SessionEnd

**Input:**
```json
{
  "reason": "exit | clear | logout | prompt_input_exit | other"
}
```

**Output:**
```json
{
  "systemMessage": "string (displayed during shutdown)"
}
```

**Notes:** **Best effort** — CLI won't wait for hooks to complete. All flow-control fields are ignored. [Official]

---

### BeforeAgent

**Input:**
```json
{
  "prompt": "string (user-submitted text)"
}
```

**Output:**
```json
{
  "decision": "allow | deny",
  "reason": "string (required if denied)",
  "continue": true,
  "hookSpecificOutput": {
    "additionalContext": "string (appended to prompt as context)"
  }
}
```

**Decision behavior:**
- `"deny"`: Blocks the turn and erases it from conversation history. [Official]
- `continue: false`: Blocks the turn but saves it to history. [Official]

---

### AfterAgent

**Input:**
```json
{
  "prompt": "string (original user request)",
  "prompt_response": "string (agent's final response text)",
  "stop_hook_active": false
}
```

**Output:**
```json
{
  "decision": "allow | deny",
  "reason": "string (sent to agent as correction prompt for retry)",
  "continue": true,
  "hookSpecificOutput": {
    "clearContext": false
  }
}
```

**Decision behavior:**
- `"deny"`: Rejects the response and forces a retry. The `reason` is sent to the agent as a correction prompt. [Official]
- `continue: false`: Stops the session without retry. [Official]
- `clearContext: true`: Clears LLM memory before retry. [Official]
- `stop_hook_active`: Boolean indicating this is a retry sequence (set by the system). [Official]

---

### BeforeModel

**Input:**
```json
{
  "llm_request": {
    "model": "string",
    "messages": [
      { "role": "user | model | system", "content": "string" }
    ],
    "config": { "temperature": 0.7 },
    "toolConfig": {
      "mode": "string",
      "allowedFunctionNames": ["string"]
    }
  }
}
```

**Output:**
```json
{
  "decision": "allow | deny",
  "reason": "string",
  "hookSpecificOutput": {
    "llm_request": { "...overrides..." },
    "llm_response": { "...synthetic response to skip LLM call..." }
  }
}
```

**Capabilities:**
- Override parts of the LLM request (model, messages, config). [Official]
- Provide a synthetic response to skip the actual LLM call entirely. [Official]
- Block the request with `decision: "deny"`. [Official]

---

### AfterModel

**Input:**
```json
{
  "llm_request": { "...original request..." },
  "llm_response": {
    "candidates": [
      {
        "content": { "role": "model", "parts": ["string"] },
        "finishReason": "string"
      }
    ],
    "usageMetadata": { "totalTokenCount": 0 }
  }
}
```

**Output:**
```json
{
  "decision": "allow | deny",
  "continue": true,
  "hookSpecificOutput": {
    "llm_response": { "...replacement response chunk..." }
  }
}
```

**Notes:**
- Fires after **every output chunk** (streaming), not just the final response. [Official]
- Use for real-time redaction or PII filtering. [Official]
- `decision: "deny"` discards the chunk and blocks the turn. [Official]

---

### BeforeToolSelection

**Input:**
```json
{
  "llm_request": { "...same format as BeforeModel..." }
}
```

**Output:**
```json
{
  "hookSpecificOutput": {
    "toolConfig": {
      "mode": "AUTO | ANY | NONE",
      "allowedFunctionNames": ["write_file", "read_file"]
    }
  }
}
```

**Mode values:**
- `"AUTO"` — default, LLM chooses tools freely. [Official]
- `"ANY"` — forces at least one tool call. [Official]
- `"NONE"` — disables all tools for this turn. [Official]

**Aggregation:** Multiple hooks' whitelists are combined via **union** — the agent receives all tools allowed by any hook. [Official]

**Limitations:** Does NOT support `decision`, `continue`, or `systemMessage` fields. [Official]

---

### BeforeTool

**Input:**
```json
{
  "tool_name": "string (e.g., 'write_file', 'run_shell_command')",
  "tool_input": { "...model-generated arguments..." },
  "mcp_context": { "...optional MCP metadata..." },
  "original_request_name": "string (original tool name if tail call)"
}
```

**Output:**
```json
{
  "decision": "allow | deny | block",
  "reason": "string (sent to agent as tool error if denied)",
  "continue": true,
  "hookSpecificOutput": {
    "tool_input": { "...merged/overridden arguments..." }
  }
}
```

**Capabilities:**
- **Block execution**: `decision: "deny"` or `"block"` prevents the tool from running. [Official]
- **Rewrite arguments**: `hookSpecificOutput.tool_input` merges with/overrides model arguments. [Official]
- **Kill agent**: `continue: false` stops the entire agent loop. [Official]

---

### AfterTool

**Input:**
```json
{
  "tool_name": "string",
  "tool_input": { "...arguments used..." },
  "tool_response": {
    "llmContent": "...",
    "returnDisplay": "...",
    "error": "...optional..."
  },
  "mcp_context": { "...optional..." },
  "original_request_name": "string"
}
```

**Output:**
```json
{
  "decision": "allow | deny",
  "reason": "string (replaces tool result for agent if denied)",
  "continue": true,
  "hookSpecificOutput": {
    "additionalContext": "string (appended to tool result)",
    "tailToolCallRequest": {
      "name": "string (tool to execute next)",
      "args": { "...arguments..." }
    }
  }
}
```

**Capabilities:**
- **Hide result**: `decision: "deny"` hides the real tool output from the agent. The `reason` replaces it. [Official]
- **Append context**: `additionalContext` adds information to the tool result. [Official]
- **Tail call**: `tailToolCallRequest` executes another tool immediately after, replacing the original result. [Official]
- **Exit 2 behavior**: Hides the tool result, uses stderr as replacement content. The turn continues. [Official]

---

### PreCompress

**Input:**
```json
{
  "trigger": "auto | manual"
}
```

**Output:**
```json
{
  "systemMessage": "string (shown before compression)"
}
```

**Notes:** Fired **asynchronously** — cannot block compression. Advisory only. [Official]

---

### Notification

**Input:**
```json
{
  "notification_type": "string (e.g., 'ToolPermission')",
  "message": "string (alert summary)",
  "details": { "...alert-specific metadata..." }
}
```

**Output:**
```json
{
  "systemMessage": "string (displayed alongside the alert)"
}
```

**Notes:** **Observability only** — cannot block actions, grant permissions, or control flow. [Official]

---

## Built-in Tool Names

These are the known built-in Gemini CLI tool names for use in BeforeTool/AfterTool matchers: [Official]

| Tool Name | Purpose |
|-----------|---------|
| `read_file` | Read file contents |
| `write_file` | Write/create files |
| `replace` | Edit/replace content in files |
| `run_shell_command` | Execute shell commands |
| `list_directory` | List directory contents |
| `grep_search` | Search file contents |
| `google_search` | Web search |
| `write_todos` | Write TODO items |
| `enter_plan_mode` | Enter plan mode |
| `exit_plan_mode` | Exit plan mode |

MCP tools follow the pattern: `mcp_<server_name>_<tool_name>` [Official]

---

## Security Model

### Threat Hierarchy

Four hook sources, from most trusted to least: [Official]

1. **System** (`/etc/gemini-cli/settings.json`) — safest
2. **User** (`~/.gemini/settings.json`)
3. **Extensions** — installed packages
4. **Project** (`.gemini/settings.json`) — untrusted by default

### Fingerprinting

Project hooks generate unique fingerprints. If a hook's `name` or `command` changes
(e.g., via `git pull`), it is treated as a new, untrusted hook and triggers a warning
before execution. [Official]

### Core Risks

1. **Arbitrary code execution** — hooks run with your user privileges. [Official]
2. **Data exfiltration** — hooks can access prompts, API keys from environment. [Official]
3. **Prompt injection** — malicious file content can manipulate hook behavior. [Official]

### Mitigation

- Environment variable redaction (see [Environment Variables](#environment-variables)). [Official]
- Review all project-level hook scripts before enabling. [Official]
- Use `jq empty` to validate JSON structure on input before processing. [Community]
- Verify the hook is not running as root. [Official]

---

## Management Commands

Gemini CLI provides built-in commands for hook management: [Official]

| Command | Description |
|---------|-------------|
| `/hooks panel` | View all hooks: execution counts, recent failures, error messages, timing |
| `/hooks enable-all` | Enable all hooks |
| `/hooks disable-all` | Disable all hooks |
| `/hooks enable <name>` | Enable a specific hook by name |
| `/hooks disable <name>` | Disable a specific hook by name |

---

## Syllago Mapping

Syllago uses Claude Code event names as canonical and translates to/from Gemini CLI.
This mapping is defined in `cli/internal/converter/toolmap.go`.

### Event Name Mapping

| Canonical (Claude Code) | Gemini CLI |
|-------------------------|------------|
| `PreToolUse` | `BeforeTool` |
| `PostToolUse` | `AfterTool` |
| `UserPromptSubmit` | `BeforeAgent` |
| `Stop` | `AfterAgent` |
| `SessionStart` | `SessionStart` |
| `SessionEnd` | `SessionEnd` |
| `PreCompact` | `PreCompress` |
| `Notification` | `Notification` |
| `SubagentStart` | *(not supported)* |
| `SubagentCompleted` | *(not supported)* |

### Tool Name Mapping

| Canonical (Claude Code) | Gemini CLI |
|-------------------------|------------|
| `Read` | `read_file` |
| `Write` | `write_file` |
| `Edit` | `replace` |
| `Bash` | `run_shell_command` |
| `Glob` | `list_directory` |
| `Grep` | `grep_search` |
| `WebSearch` | `google_search` |
| `Task` | *(not mapped)* |

### MCP Tool Name Format

| Provider | Format | Example |
|----------|--------|---------|
| Claude Code | `mcp__server__tool` | `mcp__github__get_issue` |
| Gemini CLI | `server__tool` | `github__get_issue` |

### Events Gemini CLI Has That Claude Code Lacks

| Gemini CLI Event | Description |
|------------------|-------------|
| `BeforeModel` | Modify/mock LLM requests before sending |
| `AfterModel` | Process/redact LLM response chunks in real time |
| `BeforeToolSelection` | Filter available tools before LLM selection |

These three events have no Claude Code equivalent and are dropped during conversion
with a warning. [Inferred from syllago toolmap — `BeforeModel`, `AfterModel`, and
`BeforeToolSelection` have no entries in the `HookEvents` map]

---

## Writing Hooks: Quick Reference

### Minimal Bash Hook

```bash
#!/usr/bin/env bash
input=$(cat)
tool_name=$(echo "$input" | jq -r '.tool_name')
echo "Processing: $tool_name" >&2  # stderr for logs
echo '{"decision": "allow"}'       # stdout for JSON only
exit 0
```

### Minimal Node.js Hook

```javascript
#!/usr/bin/env node
const fs = require('fs');
const input = JSON.parse(fs.readFileSync(0, 'utf-8'));
console.log(JSON.stringify({ decision: 'allow' }));
```

### Deny with Structured Feedback

```bash
#!/usr/bin/env bash
input=$(cat)
content=$(echo "$input" | jq -r '.tool_input.content // empty')
if echo "$content" | grep -qE 'api[_-]?key|password|secret'; then
  cat <<'EOF'
{
  "decision": "deny",
  "reason": "Security Policy: Potential secret detected in content",
  "systemMessage": "Security scanner blocked this operation"
}
EOF
  exit 0
fi
echo '{"decision": "allow"}'
exit 0
```

### Context Injection (BeforeAgent)

```bash
#!/usr/bin/env bash
context=$(git log -5 --oneline 2>/dev/null)
cat <<EOF
{
  "hookSpecificOutput": {
    "additionalContext": "Recent commits:\n$context"
  }
}
EOF
exit 0
```

### Tool Filtering (BeforeToolSelection, Node.js)

```javascript
#!/usr/bin/env node
const fs = require('fs');
const input = JSON.parse(fs.readFileSync(0, 'utf-8'));
const text = input.llm_request?.messages?.slice(-1)[0]?.content || '';

const allowed = ['write_todos'];
if (text.includes('read')) allowed.push('read_file');
if (text.includes('test')) allowed.push('run_shell_command');

console.log(JSON.stringify({
  hookSpecificOutput: {
    toolConfig: {
      mode: 'ANY',
      allowedFunctionNames: allowed
    }
  }
}));
```
