# Provider Hook Systems - Current State (March 2026)

Research date: 2026-03-21

This document captures the current state of hook/lifecycle systems across major AI coding tool providers, noting changes since prior research and implications for syllago's canonical hook model.

---

## 1. Claude Code

**Source:** https://code.claude.com/docs/en/hooks

### Event Types (25 events - most extensive in the ecosystem)

| Category | Events |
|----------|--------|
| Session | `SessionStart`, `SessionEnd` |
| Instructions | `InstructionsLoaded` |
| User Input | `UserPromptSubmit` |
| Tool Execution | `PreToolUse`, `PermissionRequest`, `PostToolUse`, `PostToolUseFailure` |
| Notifications | `Notification` |
| Subagent | `SubagentStart`, `SubagentStop` |
| Completion | `Stop`, `StopFailure`, `TeammateIdle`, `TaskCompleted` |
| Config | `ConfigChange` |
| Context | `PreCompact`, `PostCompact` |
| Worktree | `WorktreeCreate`, `WorktreeRemove` |
| MCP Elicitation | `Elicitation`, `ElicitationResult` |

### Handler Types (4 - unique in ecosystem)

1. **`command`** - Shell script via stdin/stdout (standard across providers)
2. **`http`** - POST JSON to URL, response body as output. Supports header env var interpolation. Default timeout 30s.
3. **`prompt`** - Single-turn LLM evaluation (yes/no gate). Unique to Claude Code.
4. **`agent`** - Spawns subagent with tool access for verification. Unique to Claude Code.

### Input Contract (common fields)

```json
{
  "session_id": "string",
  "transcript_path": "string",
  "cwd": "string",
  "permission_mode": "default|plan|acceptEdits|dontAsk|bypassPermissions",
  "hook_event_name": "string",
  "agent_id": "string (if subagent)",
  "agent_type": "string (if subagent)"
}
```

### Output Contract

```json
{
  "continue": true,
  "stopReason": "string",
  "suppressOutput": false,
  "systemMessage": "string",
  "decision": "block (for some events)",
  "reason": "string",
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow|deny|ask",
    "permissionDecisionReason": "string",
    "updatedInput": {},
    "additionalContext": "string"
  }
}
```

### Exit Codes

- **0**: Success, parse stdout JSON
- **2**: Blocking error (stderr fed to model/user)
- **Other**: Non-blocking warning

### Configuration Locations

- `~/.claude/settings.json` (user)
- `.claude/settings.json` (project, committable)
- `.claude/settings.local.json` (project, gitignored)
- Managed policy settings (org-wide)
- Plugin `hooks/hooks.json`
- Skill/Agent YAML frontmatter

### Matcher System

Regex patterns matched against tool names, event reasons, notification types, MCP server names, etc. MCP tools follow `mcp__<server>__<tool>` naming.

### Notable Changes / Unique Features

- **Deprecated** top-level `decision`/`reason` for PreToolUse in favor of `hookSpecificOutput.permissionDecision`
- `PermissionRequest` output can programmatically update permission rules (`updatedPermissions` array with `addRules|replaceRules|removeRules|setMode`)
- `async: true` for fire-and-forget command hooks
- `once: true` for single-fire-per-session hooks (skills only)
- `WorktreeCreate` returns path via stdout (not JSON)
- `SessionStart` can set env vars via `CLAUDE_ENV_FILE`
- HTTP hooks with env var interpolation in headers

### Portability Implications

Claude Code is the most feature-rich hook system. It sets the upper bound for canonical model expressiveness. Key features with no equivalent elsewhere: `http` handler type, `prompt`/`agent` handler types, `PermissionRequest` event, `updatedPermissions` output, `InstructionsLoaded` event, `ConfigChange` event, `Elicitation`/`ElicitationResult` events.

---

## 2. Gemini CLI

**Source:** https://geminicli.com/docs/hooks/, https://geminicli.com/docs/hooks/reference/

### Event Types (11 events)

| Event | Trigger | Can Block? |
|-------|---------|------------|
| `SessionStart` | Startup, resume, /clear | No (context injection only) |
| `SessionEnd` | Exit, clear | No (advisory, best-effort) |
| `BeforeAgent` | After prompt, before planning | Yes |
| `AfterAgent` | Agent loop ends | Yes (can retry) |
| `BeforeModel` | Before LLM request | Yes (can mock response) |
| `AfterModel` | After LLM response | Yes (can redact) |
| `BeforeToolSelection` | Before tool selection | Yes (filter tool list) |
| `BeforeTool` | Before tool executes | Yes |
| `AfterTool` | After tool executes | Yes |
| `PreCompress` | Before context compression | No (advisory) |
| `Notification` | System alerts (e.g. ToolPermission) | No (observability) |

### Handler Type

Only `command` type (shell scripts via stdin/stdout).

### Input Contract (common fields)

```json
{
  "session_id": "string",
  "transcript_path": "string",
  "cwd": "string",
  "hook_event_name": "string",
  "timestamp": "string (ISO 8601)"
}
```

Event-specific additions:
- `BeforeTool`/`AfterTool`: `tool_input` object with tool parameters
- `BeforeToolSelection`: `llm_request.messages` array
- `AfterAgent`: `prompt_response` field
- `AfterModel`: `llm_request` and `llm_response` objects
- `SessionStart`: `source` ("startup"|"resume"|"clear")

### Output Contract

```json
{
  "systemMessage": "string",
  "suppressOutput": false,
  "continue": true,
  "stopReason": "string",
  "decision": "allow|deny|block",
  "reason": "string",
  "hookSpecificOutput": {}
}
```

`BeforeToolSelection` output supports `hookSpecificOutput.toolConfig`:
```json
{
  "mode": "ANY|NONE|AUTO",
  "allowedFunctionNames": ["tool1", "tool2"]
}
```

### Exit Codes

- **0**: Success, parse JSON
- **2**: System block (stderr as rejection reason)
- **Other**: Non-fatal warning

### Configuration

```json
// .gemini/settings.json, ~/.gemini/settings.json, /etc/gemini-cli/settings.json
{
  "hooks": {
    "BeforeTool": [
      {
        "type": "command",
        "command": "./scripts/check.sh",
        "matcher": "write_file",
        "name": "friendly-name",
        "timeout": 60000,
        "description": "purpose",
        "sequential": true
      }
    ]
  }
}
```

Timeout in **milliseconds** (default 60000). Note: Claude Code uses **seconds**.

### Matcher System

- Tool hooks: regex against tool name
- Lifecycle hooks: exact string matching
- MCP tools: `mcp_<server>_<tool>` (underscore, not double-underscore like Claude Code)

### Unique Features

- **`BeforeModel`**: Can intercept/mock LLM requests - no other provider has this
- **`AfterModel`**: Real-time response redaction/PII filtering
- **`BeforeToolSelection`**: Dynamic tool filtering with union aggregation across hooks
- **`AfterAgent`**: Can trigger retries
- **`sequential`** field for controlling parallel vs serial execution
- Extension-bundled hooks
- Hook fingerprinting for untrusted project detection

### Portability Implications

Gemini CLI has the deepest LLM-interaction hooks (`BeforeModel`, `AfterModel`, `BeforeToolSelection`) that no other provider matches. The `BeforeAgent`/`AfterAgent` split is more granular than most. MCP tool naming differs (`_` vs `__`). Timeout units differ (ms vs s).

---

## 3. Cursor

**Source:** https://blog.gitbutler.com/cursor-hooks-deep-dive

### Event Types (6 events - most minimal)

| Event | Can Block? | Notes |
|-------|------------|-------|
| `beforeSubmitPrompt` | No | Informational only in beta |
| `beforeShellExecution` | Yes | Command blocking |
| `beforeMCPExecution` | Yes | MCP tool blocking |
| `beforeReadFile` | Yes | File access control |
| `afterFileEdit` | No | Informational only |
| `stop` | No | Informational only |

### Handler Type

Only `command` type.

### Configuration

```json
// .cursor/hooks.json, ~/.cursor/hooks.json, /etc/cursor/hooks.json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "command": "./scripts/check.sh"
      }
    ]
  }
}
```

All hooks from all locations execute (no override/merge precedence).

### Input Contract

Event-specific via stdin:
- `beforeShellExecution`: command, cwd, conversation/generation IDs, workspace roots
- `beforeMCPExecution`: server name, tool name, tool parameters, command, workspace roots
- `beforeReadFile`: file path, file content, conversation/generation IDs
- `afterFileEdit`: file path, old string, new string, edit details
- `stop`: status ("completed"|"aborted"|"error")

### Output Contract (blocking hooks only)

```json
{
  "continue": true,
  "permission": "allow|deny|ask",
  "userMessage": "string",
  "agentMessage": "string"
}
```

### Key Observations

- **Beta status** - APIs explicitly unstable
- No timeout/failClosed configuration documented
- No `version` field enforcement yet
- `beforeSubmitPrompt` and `afterFileEdit` ignore output JSON
- No PostToolUse equivalent (only afterFileEdit)
- Separate `beforeShellExecution` and `beforeMCPExecution` rather than a unified PreToolUse with matchers

### Portability Implications

Cursor's model is the simplest. The separate `beforeShellExecution`/`beforeMCPExecution`/`beforeReadFile` split means adapters need to map a single canonical PreToolUse into multiple Cursor-specific hooks. No post-hook blocking capability. Still beta - expect changes.

---

## 4. Windsurf (Cascade)

**Source:** https://docs.windsurf.com/windsurf/cascade/hooks

### Event Types (12 events)

| Category | Pre-hooks (can block) | Post-hooks (cannot block) |
|----------|----------------------|--------------------------|
| Code | `pre_read_code`, `post_read_code` | |
| Code | `pre_write_code`, `post_write_code` | |
| Commands | `pre_run_command`, `post_run_command` | |
| MCP | `pre_mcp_tool_use`, `post_mcp_tool_use` | |
| User | `pre_user_prompt` | |
| Response | | `post_cascade_response`, `post_cascade_response_with_transcript` |
| Worktree | | `post_setup_worktree` |

### Handler Type

Only `command` type.

### Input Contract (common fields)

```json
{
  "agent_action_name": "string",
  "trajectory_id": "string",
  "execution_id": "string",
  "timestamp": "string (ISO 8601)",
  "tool_info": {}
}
```

Tool-specific `tool_info` structures:
- `pre_read_code`: `{file_path}`
- `pre_write_code`: `{file_path, edits: [{old_string, new_string}]}`
- `pre_run_command`: `{command_line, cwd}`
- `pre_mcp_tool_use`: `{mcp_server_name, mcp_tool_name, mcp_tool_arguments, mcp_result (post only)}`
- `pre_user_prompt`: `{user_prompt}`
- `post_cascade_response`: `{response}`
- `post_cascade_response_with_transcript`: `{transcript_path}`
- `post_setup_worktree`: `{worktree_path, root_workspace_path}`

### Exit Codes

- **0**: Success
- **2**: Block action (pre-hooks only)
- **Other**: Error, but proceeds

### Configuration

```json
// .windsurf/hooks.json (workspace)
// ~/.codeium/windsurf/hooks.json (user)
// /etc/windsurf/hooks.json (system)
{
  "hooks": {
    "pre_run_command": [
      {
        "command": "./check.sh",
        "show_output": true,
        "working_directory": "."
      }
    ]
  }
}
```

Priority: System > User > Workspace (highest to lowest). Cloud dashboard hooks load first.

### Enterprise Features

- Cloud dashboard hook management for teams
- MDM deployment support (Jamf, Intune, Ansible, etc.)
- Immutable system-level hooks via file permissions
- Transcript auto-pruning (100 most recent, 0600 permissions)

### Unique Features

- `post_cascade_response_with_transcript` - Full conversation JSONL file access
- `post_setup_worktree` - Git worktree creation hook
- `show_output` - Explicit UI visibility control
- `working_directory` - Per-hook working directory
- Enterprise cloud dashboard distribution

### Portability Implications

Windsurf uses `snake_case` naming (all others use PascalCase or camelCase). Tool categories are split into separate events (`pre_read_code`, `pre_write_code`, `pre_run_command`, `pre_mcp_tool_use`) rather than a single PreToolUse with matchers. No output JSON contract - blocking is exit-code-only. No matcher/regex system. The `tool_info` wrapper differs from flat input schemas used by others. Enterprise features (cloud dashboard, MDM) are unique.

---

## 5. VS Code Copilot

**Source:** https://code.visualstudio.com/docs/copilot/customization/hooks

### Status: Still in Preview

### Event Types (8 events)

| Event | Can Block? | Notes |
|-------|------------|-------|
| `SessionStart` | No | Initialize resources |
| `UserPromptSubmit` | Yes | Audit/inject context |
| `PreToolUse` | Yes | Block/modify inputs |
| `PostToolUse` | Yes | Block further processing |
| `PreCompact` | No | Export state |
| `SubagentStart` | No | Track nested agents |
| `SubagentStop` | Yes | Block (force continue) |
| `Stop` | Yes | Force continuation |

### Handler Type

Only `command` type, with OS-specific overrides.

### Input Contract (common fields)

```json
{
  "timestamp": "ISO-8601",
  "cwd": "string",
  "sessionId": "string",
  "hookEventName": "string",
  "transcript_path": "string"
}
```

Event-specific: `tool_name`, `tool_input`, `tool_use_id`, `tool_response`, `prompt`, `source`, `agent_id`, `agent_type`, `stop_hook_active`, `trigger`.

### Output Contract

```json
{
  "continue": true,
  "stopReason": "string",
  "systemMessage": "string",
  "hookSpecificOutput": {
    "permissionDecision": "allow|deny|ask",
    "permissionDecisionReason": "string",
    "updatedInput": {},
    "additionalContext": "string"
  }
}
```

### Exit Codes

- **0**: Success, parse JSON
- **2**: Blocking error, stderr to model
- **Other**: Non-blocking warning

### Configuration

```json
// .github/hooks/*.json, .claude/settings.json, ~/.copilot/hooks, ~/.claude/settings.json
{
  "hooks": {
    "PreToolUse": [
      {
        "type": "command",
        "command": "./hook.sh",
        "timeout": 30,
        "cwd": "relative/path",
        "env": {"KEY": "value"}
      }
    ]
  }
}
```

Also supports:
- `windows`/`linux`/`osx` OS-specific command overrides
- Agent-scoped hooks via `.agent.md` frontmatter
- `/hooks` chat command and `/create-hook` AI-assisted hook generation
- `chat.hookFilesLocations` setting for custom paths
- Parent repository discovery for monorepos

### Key Observations

- **Heavy convergence with Claude Code**: Same event names (PascalCase), same exit code semantics, same output JSON shape, same `hookSpecificOutput` pattern, same `permissionDecision` field
- Reads from `.claude/settings.json` - direct Claude Code compatibility
- `env` field for custom environment variables (unique)
- OS-specific command overrides (unique)
- `/create-hook` AI generation (unique UX feature)
- Still Preview - may change

### Portability Implications

VS Code Copilot appears to have intentionally aligned with Claude Code's hook contract. The output JSON is nearly identical. This means a single adapter could serve both. The `.github/hooks/*.json` path also overlaps with GitHub Copilot CLI. Fewer events than Claude Code (8 vs 25) but the core set is compatible.

---

## 6. GitHub Copilot CLI

**Source:** https://docs.github.com/en/copilot/reference/hooks-configuration

### Event Types (6 events)

| Event | Can Block? |
|-------|------------|
| `sessionStart` | No |
| `sessionEnd` | No |
| `userPromptSubmitted` | No |
| `preToolUse` | Yes (deny only) |
| `postToolUse` | No |
| `errorOccurred` | No |

### Configuration

```json
// .github/hooks/*.json (repo, must be on default branch)
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "bash": "./scripts/hook.sh",
        "powershell": "./scripts/hook.ps1",
        "cwd": ".",
        "timeoutSec": 30,
        "comment": "description"
      }
    ]
  }
}
```

### Input Contract

```json
{
  "timestamp": "unix milliseconds (number)",
  "cwd": "string",
  "toolName": "string",
  "toolArgs": "JSON string",
  "source": "new|resume|startup (sessionStart)",
  "initialPrompt": "string (sessionStart)",
  "reason": "complete|error|abort|timeout|user_exit (sessionEnd)",
  "prompt": "string (userPromptSubmitted)",
  "toolResult": {"resultType": "success|failure|denied", "textResultForLlm": "string"}
}
```

### Output Contract (preToolUse only)

```json
{
  "permissionDecision": "allow|deny|ask",
  "permissionDecisionReason": "string"
}
```

Only `deny` is currently processed.

### Key Differences from Other Providers

- **`version: 1`** field required (shared with Cursor)
- **`bash`/`powershell`** separate fields instead of single `command`
- **`timeoutSec`** naming (not `timeout`)
- **`toolArgs`** is a JSON string, not a parsed object
- **camelCase** event names (not PascalCase)
- **`timestamp`** is unix milliseconds (not ISO 8601)
- Hook failures are non-blocking by design ("a broken hook script shouldn't take down your workflow")
- `errorOccurred` event is unique to this provider
- No exit code 2 blocking - only JSON output controls decisions

### Portability Implications

Copilot CLI is deliberately conservative: most hooks are informational-only, only `preToolUse` can block, and even then only via `deny`. The `bash`/`powershell` split and `version` field add adapter complexity. The `toolArgs` as JSON string (not object) requires parsing. `errorOccurred` is a useful event no other provider has.

---

## 7. Kiro

**Source:** https://kiro.dev/docs/hooks/, https://kiro.dev/docs/cli/custom-agents/configuration-reference/

### IDE Event Types (10 events)

| Event | Category |
|-------|----------|
| `Prompt Submit` | User input |
| `Agent Stop` | Completion |
| `Pre Tool Use` | Tool execution |
| `Post Tool Use` | Tool execution |
| `File Create` | File system |
| `File Save` | File system |
| `File Delete` | File system |
| `Pre Task Execution` | Spec tasks |
| `Post Task Execution` | Spec tasks |
| `Manual Trigger` | On-demand |

### CLI Event Types (5 events)

| Event | Can Block? |
|-------|------------|
| `agentSpawn` | No |
| `userPromptSubmit` | No |
| `preToolUse` | Yes (exit code 2) |
| `postToolUse` | No |
| `stop` | No |

### Handler Types (2)

1. **Agent Prompt** ("Ask Kiro") - Natural language instruction to agent. Consumes credits.
2. **Shell Command** - Deterministic script execution. Default timeout 60s.

### Configuration

```json
// .kiro/hooks/ directory (IDE, form-based or AI-generated)
// agent.json hooks field (CLI)
{
  "hooks": {
    "preToolUse": [
      {
        "command": "./check.sh",
        "matcher": "execute_bash"
      }
    ]
  }
}
```

### Matcher System

CLI uses internal tool names: `fs_read`, `fs_write`, `execute_bash`, `use_aws`.
IDE uses categories: `read`, `write`, `shell`, `web`, `spec`, `*`.
MCP prefix filters: `@mcp`, `@powers`, `@builtin` with regex support.

### Unique Features

- **File system hooks** (`File Create`, `File Save`, `File Delete`) with glob patterns - unique across all providers
- **Spec task hooks** (`Pre Task Execution`, `Post Task Execution`) - tied to Kiro's spec system
- **Agent Prompt** handler type - natural language actions (similar concept to Claude Code's `prompt`/`agent` types)
- **Manual Trigger** - explicitly on-demand hooks
- AI-assisted hook creation via natural language description

### Portability Implications

Kiro's file system hooks and spec task hooks have no equivalent in other providers. The IDE vs CLI event sets differ significantly. The `Agent Prompt` handler is similar to Claude Code's `prompt` type but uses credits. Internal tool names (`fs_read` vs `Read`) require tool name mapping in adapters.

---

## 8. OpenCode

**Source:** https://opencode.ai/docs/plugins/

### Architecture: Plugin System (not hooks)

OpenCode uses a JavaScript/TypeScript plugin model rather than JSON-configured shell hooks. Plugins are Bun-based modules.

### Event Types (~30+ granular events)

| Category | Events |
|----------|--------|
| Command | `command.executed` |
| File | `file.edited`, `file.watcher.updated` |
| Installation | `installation.updated` |
| LSP | `lsp.client.diagnostics`, `lsp.updated` |
| Message | `message.part.removed`, `message.part.updated`, `message.removed`, `message.updated` |
| Permission | `permission.asked`, `permission.replied` |
| Server | `server.connected` |
| Session | `session.created`, `session.compacted`, `session.deleted`, `session.diff`, `session.error`, `session.idle`, `session.status`, `session.updated` |
| Todo | `todo.updated` |
| Shell | `shell.env` |
| Tool | `tool.execute.before`, `tool.execute.after` |
| TUI | `tui.prompt.append`, `tui.command.execute`, `tui.toast.show` |
| Experimental | `experimental.session.compacting` |

### Plugin Definition

```typescript
import type { Plugin } from "@opencode-ai/plugin"

export const MyPlugin: Plugin = async ({ project, client, $, directory, worktree }) => {
  return {
    "tool.execute.before": async (input, output) => {
      if (input.tool === "bash") {
        output.args.command = "modified command"
      }
    },
    "shell.env": async (input, output) => {
      output.env.MY_KEY = "value"
    }
  }
}
```

### Configuration

```json
// opencode.json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": ["opencode-helicone-session", "@my-org/custom-plugin"]
}
```

Local plugins: `.opencode/plugins/` (project) or `~/.config/opencode/plugins/` (global).
Dependencies via `.opencode/package.json`, auto-installed with `bun install`.

### Custom Tools

Plugins can define custom tools that override built-ins:
```typescript
tool({
  description: "string",
  args: { foo: tool.schema.string() },
  async execute(args, context) { return "result" }
})
```

### Input/Output Contract

Programmatic (not JSON stdin/stdout):
- `input`: Event-specific data object
- `output`: Mutable object for modifications (e.g., `output.args`, `output.env`, `output.context`)
- No exit code semantics - in-process execution

### Unique Features

- **Programmatic plugin model** - TypeScript/JavaScript, not shell scripts
- **In-process execution** - no stdin/stdout JSON serialization
- **Mutable output objects** - direct mutation rather than return values
- **Custom tool definition** - plugins can add/override tools
- **LSP events** - language server integration hooks
- **TUI events** - terminal UI interaction hooks
- **Message-level events** - granular conversation state changes
- **npm package distribution** - plugins installable via package managers

### Portability Implications

OpenCode is fundamentally different from all other providers. It uses a programmatic plugin model (TypeScript) rather than JSON-configured shell commands. There is no JSON config equivalent for hook definitions. Adapting to/from OpenCode requires generating TypeScript code or providing a bridge layer that wraps shell commands in a plugin. The event granularity is the highest of any provider (~30+ events vs 6-25 for others), but many events are UI/implementation-specific with no cross-provider equivalent.

---

## Cross-Provider Comparison Matrix

### Event Coverage

| Canonical Event | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro | OpenCode |
|----------------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| Session Start | SessionStart | SessionStart | - | - | SessionStart | sessionStart | agentSpawn | session.created |
| Session End | SessionEnd | SessionEnd | - | - | Stop | sessionEnd | stop | - |
| User Prompt | UserPromptSubmit | BeforeAgent | beforeSubmitPrompt | pre_user_prompt | UserPromptSubmit | userPromptSubmitted | userPromptSubmit | - |
| Pre Tool Use | PreToolUse | BeforeTool | beforeShell/MCP/Read | pre_read/write/run/mcp | PreToolUse | preToolUse | preToolUse | tool.execute.before |
| Post Tool Use | PostToolUse | AfterTool | afterFileEdit | post_read/write/run/mcp | PostToolUse | postToolUse | postToolUse | tool.execute.after |
| Pre Compact | PreCompact | PreCompress | - | - | PreCompact | - | - | experimental.session.compacting |
| Agent Stop | Stop | AfterAgent | stop | post_cascade_response | Stop | - | stop | session.idle |
| Subagent Start | SubagentStart | - | - | - | SubagentStart | - | - | - |
| Subagent Stop | SubagentStop | - | - | - | SubagentStop | - | - | - |
| Error | StopFailure | - | - | - | - | errorOccurred | - | session.error |
| Notification | Notification | Notification | - | - | - | - | - | - |
| Permission | PermissionRequest | - | - | - | - | - | - | permission.asked |
| File Events | - | - | - | - | - | - | File Create/Save/Delete | file.edited |
| Config Change | ConfigChange | - | - | - | - | - | - | - |
| LLM Intercept | - | BeforeModel/AfterModel | - | - | - | - | - | - |
| Tool Selection | - | BeforeToolSelection | - | - | - | - | - | - |
| Transcript | - | - | - | post_cascade_response_with_transcript | - | - | - | - |

### Handler Type Support

| Type | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro | OpenCode |
|------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| Shell command | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No |
| HTTP endpoint | Yes | No | No | No | No | No | No | No |
| LLM prompt | Yes | No | No | No | No | No | Yes* | No |
| Agent | Yes | No | No | No | No | No | Yes* | No |
| TypeScript plugin | No | No | No | No | No | No | No | Yes |

*Kiro's "Agent Prompt" is a natural-language handler but via IDE, not CLI.

### Output Contract Convergence

| Feature | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro CLI | OpenCode |
|---------|-------------|------------|--------|----------|-----------------|-------------|----------|----------|
| JSON stdout | Yes | Yes | Yes | No | Yes | Yes | Undocumented | N/A (in-process) |
| Exit code 2 blocks | Yes | Yes | No | Yes | Yes | No | Yes | N/A |
| `continue` field | Yes | Yes | Yes | No | Yes | No | No | N/A |
| `decision` field | Yes | Yes | No | No | No | No | No | N/A |
| `permissionDecision` | Yes | No | Yes | No | Yes | Yes | No | N/A |
| `updatedInput` | Yes | No | No | No | Yes | No | No | Yes (mutable) |
| `additionalContext` | Yes | No | No | No | Yes | No | No | Yes (mutable) |
| `systemMessage` | Yes | Yes | No | No | Yes | No | No | N/A |

### Configuration Format

| Provider | Format | Version Field | Naming | Timeout Unit | Timeout Default |
|----------|--------|---------------|--------|-------------|-----------------|
| Claude Code | JSON (settings.json) | No | PascalCase | seconds | 600s (cmd), 30s (http) |
| Gemini CLI | JSON (settings.json) | No | PascalCase | milliseconds | 60000ms |
| Cursor | JSON (hooks.json) | Yes (1) | camelCase | Not documented | Not documented |
| Windsurf | JSON (hooks.json) | No | snake_case | Not documented | Not documented |
| VS Code Copilot | JSON (multiple) | No | PascalCase | seconds | 30s |
| Copilot CLI | JSON (hooks.json) | Yes (1) | camelCase | seconds | 30s |
| Kiro CLI | JSON (agent.json) | No | camelCase | seconds | 60s |
| OpenCode | JSON + TypeScript | No | dot.notation | N/A | N/A |

---

## Key Findings for Canonical Model Design

### 1. Strong Convergence on Core Events (7 providers agree)

The following events have near-universal support and should form the canonical core:
- **Session start/end** (7/8 providers)
- **Pre/Post tool use** (8/8 providers - universal)
- **User prompt submit** (7/8 providers)
- **Agent stop** (7/8 providers)

### 2. Exit Code 2 = Block is De Facto Standard

Six of seven shell-based providers use exit code 2 for blocking. Copilot CLI is the outlier (only JSON output). This should be the canonical blocking mechanism.

### 3. Two Output Contract Families

**Family A (Claude Code / VS Code Copilot / Gemini CLI):** Rich JSON with `continue`, `decision`, `hookSpecificOutput`, `permissionDecision`, `updatedInput`, `additionalContext`, `systemMessage`. These three are converging.

**Family B (Windsurf / Copilot CLI / Cursor):** Simpler - exit codes for blocking, minimal or no JSON output processing.

The canonical model should support Family A's richness while degrading gracefully to Family B.

### 4. Tool Name Granularity Challenge

Providers split tool events differently:
- **Unified + matcher**: Claude Code, VS Code Copilot, Gemini CLI, Kiro, OpenCode (single PreToolUse with patterns)
- **Split by category**: Cursor (beforeShell, beforeMCP, beforeRead), Windsurf (pre_read_code, pre_write_code, pre_run_command, pre_mcp_tool_use)

The canonical model should use unified events with matchers, and adapters for split-category providers should map matchers to the correct category-specific event.

### 5. Provider-Exclusive Features (low portability)

These features exist in only one provider and should be marked as non-portable:
- LLM interception (Gemini: BeforeModel/AfterModel)
- Tool selection filtering (Gemini: BeforeToolSelection)
- Permission rule management (Claude Code: PermissionRequest/updatedPermissions)
- File system hooks (Kiro: File Create/Save/Delete)
- Config change hooks (Claude Code: ConfigChange)
- HTTP handler type (Claude Code only)
- TypeScript plugin model (OpenCode only)
- Spec task hooks (Kiro only)

### 6. Naming Convention Divergence

Four different conventions across providers: PascalCase, camelCase, snake_case, dot.notation. The canonical model must pick one and adapters must translate.

### 7. OpenCode is Architecturally Different

OpenCode requires a fundamentally different adapter strategy (code generation or bridge plugin) compared to all other providers (JSON config + shell scripts). It may warrant a separate portability tier.

### 8. Enterprise Distribution Varies

- **Windsurf**: Cloud dashboard + MDM + system paths
- **Claude Code**: Managed policy settings
- **Copilot CLI**: Repository-bound (.github/hooks/)
- **Others**: File-system only

### 9. Timestamp Format Divergence

- ISO 8601 string: Claude Code, Gemini CLI, Windsurf, VS Code Copilot
- Unix milliseconds: Copilot CLI

### 10. Input Wrapping Varies

- Flat fields: Claude Code, VS Code Copilot, Gemini CLI, Copilot CLI
- `tool_info` wrapper: Windsurf
- Programmatic args: OpenCode
