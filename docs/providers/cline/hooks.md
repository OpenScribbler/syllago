# Cline Hooks

Cline supports lifecycle hooks as of v3.36. Hooks are executable scripts that run at
deterministic points in the task execution lifecycle, receiving JSON via stdin and
returning JSON via stdout.

## Supported Hook Events

Cline supports **8 hook events**:

| Hook Event | When It Fires | Cancellable | Notes |
|---|---|---|---|
| `TaskStart` | New task initiated | Yes | First hook in lifecycle |
| `TaskResume` | Paused task resumes | Yes | Fires on task re-activation |
| `TaskCancel` | User cancels task | No | Observation-only; cleanup |
| `TaskComplete` | Task finishes successfully | No | Post-completion actions |
| `PreToolUse` | Before each tool executes | Yes | Can block tool execution |
| `PostToolUse` | After each tool completes | Yes | Skipped for `attempt_completion` |
| `UserPromptSubmit` | User sends a message | Yes | Fires on initial task, resume, and feedback |
| `PreCompact` | Before context window compaction | Yes | Archive/summarize before truncation |

[Official] https://docs.cline.bot/features/hooks
[Official] https://deepwiki.com/cline/cline/7.3-hooks-system

## File Placement

### Workspace hooks (version-controlled, project-specific)

```
<project-root>/.clinerules/hooks/
├── PreToolUse       # extensionless executable (macOS/Linux)
├── PostToolUse
├── TaskStart
└── ...
```

### Global hooks (user-wide)

```
~/.cline/hooks/          # per DeepWiki source
~/Documents/Cline/Hooks/ # per official docs
```

When both global and workspace hooks exist for the same event, global hooks execute
first, then workspace hooks. Either returning `cancel: true` stops execution.

[Official] https://docs.cline.bot/features/hooks
[Official] https://deepwiki.com/cline/cline/7.3-hooks-system

### Platform-specific naming

| Platform | File Format |
|---|---|
| macOS / Linux | Extensionless executable (e.g., `PreToolUse`) |
| Windows | PowerShell script (e.g., `PreToolUse.ps1`) |

### Enable/disable toggle

- **Unix**: Enabled via `chmod +x`, disabled via `chmod -x`
- **Windows**: Toggle not yet supported; hook executes if file exists
- **UI**: Toggle switches in Cline's Hooks settings panel
- **CLI**: `cline config set hooks-enabled=true` or `cline "prompt" -s hooks_enabled=true`

[Official] https://docs.cline.bot/features/hooks

## Input Schema (stdin JSON)

All hooks receive a base payload via stdin:

```json
{
  "taskId": "string",
  "hookName": "PreToolUse",
  "clineVersion": "3.36.0",
  "timestamp": "1234567890",
  "workspaceRoots": ["/path/to/project"],
  "userId": "string",
  "model": {
    "provider": "anthropic",
    "slug": "claude-sonnet-4-20250514"
  }
}
```

### Hook-specific fields

| Hook | Additional Fields |
|---|---|
| `TaskStart` | `{ "task": "description string" }` |
| `TaskResume` | `{ "task": "description string" }` |
| `TaskCancel` | `{ "task": "description string" }` |
| `TaskComplete` | `{ "task": "description string" }` |
| `PreToolUse` | `{ "tool": "read_file", "parameters": { "path": "...", ... } }` |
| `PostToolUse` | Same as PreToolUse plus `"result"`, `"success"`, `"durationMs"` |
| `UserPromptSubmit` | `{ "prompt": "user message text" }` |
| `PreCompact` | `{ "conversationLength": 150, "estimatedTokens": 45000 }` |

The `pendingToolInfo` object in PreToolUse/PostToolUse can contain:
- `path`, `content` (first 200 chars), `diff` (first 200 chars) -- file operations
- `command` -- terminal execution
- `url` -- web operations
- `regex` -- search operations
- `mcpTool`, `mcpServer`, `resourceUri` -- MCP tool calls

[Official] https://deepwiki.com/cline/cline/7.3-hooks-system

## Output Schema (stdout JSON)

```json
{
  "cancel": false,
  "contextModification": "optional text injected into conversation",
  "errorMessage": "shown to user if cancel is true",
  "log": "optional logging message"
}
```

### Context injection

If `contextModification` starts with a `TYPE:` prefix (e.g., `SECURITY_CHECK: ...`),
it becomes an XML type attribute in the injected context:

```xml
<hook_context source="PreToolUse" type="SECURITY_CHECK">
<content>
Additional context from hook
</content>
</hook_context>
```

Otherwise the type defaults to `"general"`.

[Official] https://deepwiki.com/cline/cline/7.3-hooks-system

## Execution Details

- Hook scripts run as child processes via Node.js `child_process.spawn()`
- Working directory is set to the workspace root
- Use stderr (`>&2`) for debug logging (stdout is reserved for JSON output)
- Thread-safe: mutex-protected state management prevents race conditions
- Non-cancellable hooks (TaskComplete, TaskCancel) suppress cancellation errors
- Execution failures are logged but don't block agent continuation
- Real-time output streaming via `hook_output_stream` messages in the UI

[Official] https://deepwiki.com/cline/cline/7.3-hooks-system

## Comparison with Claude Code Hooks

| Feature | Cline | Claude Code |
|---|---|---|
| Hook discovery | File-based (named executables) | JSON config in `CLAUDE.md` or settings |
| Event count | 8 events | 4 events (PreToolUse, PostToolUse, Notification, Stop) |
| Context injection | Yes (`contextModification` field) | Yes (stdout becomes user-visible) |
| Cancellation | `cancel: true` in JSON output | Non-zero exit code blocks |
| Platform | macOS, Linux (Windows partial) | macOS, Linux, Windows |
| Scope | Global + workspace | Global + project |

[Inferred] Comparison assembled from both providers' documentation.
