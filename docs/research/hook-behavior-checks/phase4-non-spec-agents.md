# Hook System Research: Non-Spec AI Coding Agents

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Amazon Q Developer | github.com/aws/amazon-q-developer-cli — docs/agent-format.md, docs/hooks.md; docs.aws.amazon.com/amazonq/ |
| Cline | cline.bot/blog/cline-v3-36-hooks, deepwiki.com/cline/cline/7.3-hooks-system |
| Augment Code | docs.augmentcode.com/cli/hooks |
| Roo Code | docs.roocode.com/, github.com/RooCodeInc/Roo-Code CHANGELOG.md |
| gptme | github.com/gptme/gptme — gptme/hooks/types.py, gptme/hooks/__init__.py, gptme/hooks/confirm.py |
| Pi Agent | github.com/badlogic/pi-mono — packages/coding-agent/docs/extensions.md, docs/hooks.md |

---

## Amazon Q Developer

### Event Model (5 events)

| Event | Description | Blocking |
|-------|-------------|----------|
| `agentSpawn` | Agent initialized; output persists for entire session | No |
| `userPromptSubmit` | User submits message; output ephemeral (current prompt only) | No |
| `preToolUse` | Before tool execution | Yes (exit code 2) |
| `postToolUse` | After tool execution | No |
| `stop` | Agent finishes responding | No |

### Configuration

Hooks defined in JSON agent config files (`~/.aws/amazonq/agents/`). Agents activated via `/agent` command.

```json
{
  "hooks": {
    "agentSpawn": [{ "command": "git branch --show-current" }],
    "userPromptSubmit": [{ "command": "git status --porcelain", "timeout_ms": 5000, "cache_ttl_seconds": 30 }],
    "preToolUse": [{ "matcher": "execute_bash", "command": "audit_command" }]
  }
}
```

### Key Details

- **Blocking:** Only `preToolUse` — exit code 2 blocks, stderr surfaced to LLM
- **Timeout:** 30s default, configurable via `timeout_ms`
- **stdin:** JSON with `hook_event_name`, `cwd`, tool fields for tool events
- **stdout:** Raw text injected as conversation context (not structured JSON)
- **Matcher patterns:** Exact names, wildcards (`fs_*`), MCP server refs (`@git`), catch-all (`*`)

### Unique Capabilities

1. **Output caching** (`cache_ttl_seconds`) — hook output reused across prompts within TTL window. No spec provider has this.
2. **`agentSpawn` persistent injection** — output persists for entire session, distinct from per-prompt events.
3. **MCP server namespace matchers** (`@git`) — target all tools from an MCP server, not individual tools.

---

## Cline

### Event Model (8 events)

| Event | Description | Can Cancel |
|-------|-------------|-----------|
| `TaskStart` | New task begins | Yes |
| `TaskResume` | Paused task resumes | Yes |
| `TaskCancel` | Task cancelled | No (observation) |
| `TaskComplete` | `attempt_completion` succeeds | No (observation) |
| `PreToolUse` | Before tool execution | Yes |
| `PostToolUse` | After tool execution | **Yes** (unusual) |
| `UserPromptSubmit` | User submits prompt | Yes |
| `PreCompact` | Before context compaction | Yes |

### Configuration

**Directory-based auto-discovery** (no JSON manifest):
- Global: `~/Documents/Cline/Hooks/` (macOS/Linux) — **VALIDATED: original path `~/Documents/Cline/Rules/Hooks/` was incorrect per official docs**
- Project: `<workspace>/.clinerules/hooks/`

Scripts are executable files named after the event type. Any interpreter via shebang.

### Key Details

- **Blocking:** JSON output with `"cancel": true`. Raises `PreToolUseHookCancellationError`.
- **Context injection:** `"contextModification"` field — text injected as XML into LLM context, affects next API request.
- **Timeout:** 30s; SIGTERM then SIGKILL
- **stdin:** JSON with `clineVersion`, `hookName`, `timestamp`, `taskId`, `workspaceRoots`, `userId`, event-specific data
- **Content limit:** ~50KB truncation on hook output
- **Platform:** macOS/Linux only; no Windows support

### Unique Capabilities

1. **`contextModification` field** — write to LLM context as structured XML without blocking.
2. **`PostToolUse` can cancel** — only agent that supports blocking further processing after tool execution.
3. **`PreCompact` event** — hooks fire before context compaction, enabling context reinforcement.
4. **Directory-based auto-discovery** — no config file; script presence = registration.

---

## Augment Code

### Event Model (5 events)

| Event | Description | Blocking |
|-------|-------------|----------|
| `PreToolUse` | Before tool execution | Yes (exit 2 or `permissionDecision: "deny"`) |
| `PostToolUse` | After tool execution | No |
| `Stop` | Agent stops responding | Yes (`decision: "block"` keeps agent working) |
| `SessionStart` | Session begins; stdout → agent context | No (context injection) |
| `SessionEnd` | Session concludes | No |

### Configuration

`settings.json` at three precedence levels:
- System: `/etc/augment/settings.json` (enterprise; **immutable**, cannot be overridden)
- User: `~/.augment/settings.json`
- Project: workspace-level settings

### Key Details

- **Blocking:** Exit code 2 or `permissionDecision: "deny"` for PreToolUse. `decision: "block"` for Stop.
- **Timeout:** 60 seconds (longer than most agents' 30s)
- **stdin:** JSON with `hook_event_name`, `conversation_id`, `workspace_roots`, tool fields
- **Environment vars:** `AUGMENT_PROJECT_DIR`, `AUGMENT_CONVERSATION_ID`, `AUGMENT_HOOK_EVENT`, `AUGMENT_TOOL_NAME`
- **Privacy opt-in flags:** `includeUserContext`, `includeMCPMetadata`, `includeConversationData` — hooks receive minimal data by default

### Unique Capabilities

1. **System-level immutable hooks** — enterprise IT/security can enforce hooks that users cannot override. Strongest enterprise deployment model.
2. **Privacy-first opt-in data model** — hooks receive minimal data by default; sensitive data requires explicit opt-in flags.
3. **`Stop` event blocking** — prevents agent from completing response (keeps agent working).

---

## Roo Code

**NO DEDICATED HOOK SYSTEM FOUND.**

Roo Code (Cline fork, v3.51.1) has no hooks. The `.roo/hooks/` directory appears only in community feature request discussions.

Has experimental Custom Tools (`.roo/tools/`) — TypeScript/JS files registered as LLM-callable tools. Not a lifecycle hook system.

---

## gptme

### Event Model (20+ events, Python-native)

**Step/Turn:** `step.pre`, `step.post`, `turn.pre`, `turn.post`
**Message:** `message.transform`
**Tool:** `tool.execute.pre`, `tool.execute.post`, `tool.transform`, `tool.confirm`
**File:** `file.save.pre`, `file.save.post`, `file.patch.pre`, `file.patch.post`
**Session:** `session.start`, `session.end`
**Generation:** `generation.pre`, `generation.post`, `generation.interrupt`
**Control:** `loop.continue`, `cwd.changed`, `cache.invalidated`, `elicit`

### Configuration

Python functions registered programmatically:
```python
register_hook(name, hook_type, func, priority, enabled)
```

Plugin discovery via `gptme.toml`:
```toml
[plugins]
paths = ["~/.config/gptme/plugins", "./plugins"]
enabled = ["my_plugin"]
```

Environment variables: `GPTME_HOOKS_DISABLED="hook1,hook2"`, `GPTME_HOOK_PRIORITY_HOOKNAME=20`

### Key Details

- **Runtime:** In-process Python (not shell subprocesses)
- **Blocking:** `StopPropagation` sentinel halts lower-priority hooks. `tool.confirm` returns `ConfirmAction.CONFIRM/SKIP/EDIT`.
- **Output:** Yielded `Message` objects injected into conversation
- **Priority system:** Higher priority runs first; `StopPropagation` short-circuits

### Unique Capabilities

1. **`tool.confirm` with EDIT option** — hook can modify tool input before execution (not just block/allow)
2. **`loop.continue`** — hooks control whether the agentic loop continues at all
3. **`message.transform` / `tool.transform`** — persistent content transformation (rewrites stored content)
4. **`cwd.changed`** — working directory change tracking
5. **`cache.invalidated`** — react to prompt cache invalidation
6. **`elicit`** — agent requests structured user input; hooks can intercept

---

## Pi Agent

### Event Model (20+ events, TypeScript-native)

**Session:** `session_start` (with reason: startup/reload/new/resume/fork), `session_directory`, `resources_discover`, `session_before_switch/fork/compact`, `session_shutdown`
**Agent:** `before_agent_start`, `agent_start`, `agent_end`, `turn_start`, `turn_end`
**Message:** `message_start`, `message_update`, `message_end`, `context`
**Tool:** `tool_call` (can block), `tool_execution_start/update/end`, `tool_result` (can modify)
**LLM:** `before_provider_request` (can replace entire payload)
**Interaction:** `model_select`, `input`, `user_bash`

### Configuration

TypeScript files loaded via jiti (no compilation):
- Global: `~/.pi/agent/extensions/` or `~/.pi/agent/hooks/`
- Project: `.pi/extensions/` or `.pi/hooks/`

Extensions support `package.json` with npm dependencies. Hot-reload via `/reload`.

### Key Details

- **Blocking:** `tool_call` returns `{ block: true, reason: "..." }`. Can show UI confirmation dialogs.
- **Runtime:** In-process TypeScript via jiti
- **Two layers:** Hooks (lightweight, one file) vs Extensions (full API access, custom tools, custom commands)

### Unique Capabilities

1. **`before_provider_request`** — inspect or REPLACE the entire LLM payload. No other agent exposes this.
2. **`tool_result` modification** — alter what the LLM sees as tool output (redact, normalize).
3. **Interactive UI from hooks** — `ctx.ui.confirm()`, `ctx.ui.notify()`, `ctx.ui.input()`, `ctx.ui.select()`, custom TUI widgets.
4. **Custom tool registration** — extensions register first-class LLM-callable tools.
5. **`session_start` reason field** — conditional behavior per session type (startup vs resume vs fork).
6. **npm dependency support** — extensions support `package.json` with `node_modules`.

### Trust Lifecycle Note

The research plan mentioned Pi's "pending → acknowledged → trusted → killed" trust lifecycle. This pattern was NOT found in current Pi Agent documentation. Trust is managed implicitly through `tool_call` blocking and `ctx.ui.confirm()` dialogs, not an explicit named state machine.

---

## Recommendation Summary

| Agent | Has Hooks | Add to Spec? | Priority | Notes |
|-------|----------|-------------|----------|-------|
| Amazon Q Developer | Yes (5 events) | **Yes** | High | Shell-based, close to spec model. Output caching is novel. |
| Cline | Yes (8 events) | **Yes** | High | Directory-based discovery, PostToolUse blocking, contextModification. |
| Augment Code | Yes (5 events) | **Yes** | High | Most enterprise-ready (immutable system hooks, privacy opt-in). |
| Roo Code | No | No | — | Custom Tools only, no lifecycle hooks. |
| gptme | Yes (20+ events) | Yes (with caveat) | Medium | Python-native, not shell subprocess. Needs "plugin API" category. |
| Pi Agent | Yes (20+ events) | Yes (with caveat) | Medium | TypeScript-native, not shell subprocess. Needs "plugin API" category. |

## Unique Capabilities for Spec Consideration

1. **Output caching** (Amazon Q) — `cache_ttl_seconds`
2. **Persistent session injection** (Amazon Q `agentSpawn`, Augment `SessionStart`)
3. **`contextModification`** (Cline) — structured context injection without blocking
4. **`PostToolUse` blocking** (Cline) — cancel after tool execution
5. **System-level immutable hooks** (Augment) — enterprise enforcement
6. **Privacy opt-in data model** (Augment) — minimal data by default
7. **`tool_result` modification** (Pi) — alter tool output before LLM sees it
8. **`before_provider_request` interception** (Pi) — full LLM payload replacement
9. **Interactive UI from hooks** (Pi) — TUI confirmation dialogs
10. **`tool.confirm` EDIT option** (gptme) — modify tool input (not just block/allow)
11. **`loop.continue` control** (gptme) — hooks decide if agentic loop continues
12. **MCP server namespace matchers** (Amazon Q) — `@git` syntax for tool groups
13. **In-process language-native hooks** (gptme Python, Pi TypeScript, OpenCode TypeScript) — fundamentally different integration model
