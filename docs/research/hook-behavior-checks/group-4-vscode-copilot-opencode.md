# Hook Behavior Validation: VS Code Copilot + OpenCode

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| VS Code Copilot | code.visualstudio.com/docs/copilot/customization/hooks (fetched 2026-03-31), github.com/microsoft/vscode-docs/blob/main/docs/copilot/customization/hooks.md |
| OpenCode | opencode.ai/docs/plugins/ (fetched 2026-03-31), github.com/sst/opencode — packages/plugin/src/index.ts, packages/opencode/src/permission/index.ts, packages/opencode/src/file/index.ts |

**Important distinction:** VS Code Copilot hooks and GitHub Copilot Coding Agent hooks are DIFFERENT products:
- **VS Code Copilot hooks**: 8 events, PascalCase (what this spec validates)
- **GitHub Copilot Coding Agent hooks**: 6 events, camelCase (cloud-hosted PR agents)

---

## VS Code Copilot

### Category A: Event Name Mapping Validation

| Check | Canonical | Spec Claim | Finding | Actual |
|-------|-----------|------------|---------|--------|
| VSC-A1 | before_tool_execute | PreToolUse | CONFIRMED | PreToolUse (event #3) |
| VSC-A2 | after_tool_execute | PostToolUse | CONFIRMED | PostToolUse (event #4) |
| VSC-A3 | session_start | SessionStart | CONFIRMED | SessionStart (event #1) — "first prompt of a new session" |
| VSC-A4 | session_end | -- (not supported) | CONFIRMED | Not in 8-event list |
| VSC-A5 | before_prompt | UserPromptSubmit | CONFIRMED | UserPromptSubmit (event #2) |
| VSC-A6 | agent_stop | Stop | CONFIRMED | Stop (event #8) — includes `stop_hook_active` boolean |
| VSC-A7 | before_compact | PreCompact | CONFIRMED | PreCompact (event #5) |
| VSC-A8 | subagent_start | SubagentStart | CONFIRMED | SubagentStart (event #6) |
| VSC-A9 | subagent_stop | SubagentStop | CONFIRMED | SubagentStop (event #7) — supports `decision: "block"` |

Complete VS Code Copilot event list (8 total):
1. SessionStart
2. UserPromptSubmit
3. PreToolUse
4. PostToolUse
5. PreCompact
6. SubagentStart
7. SubagentStop
8. Stop

Note: VS Code parses Copilot CLI configurations and converts lowerCamelCase (`preToolUse`) to PascalCase (`PreToolUse`).

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| VSC-B1 | before_tool_execute | prevent | CONFIRMED | `permissionDecision: "deny"` blocks; most restrictive decision wins across multiple hooks |
| VSC-B2 | after_tool_execute | observe | **CORRECTED** | PostToolUse supports `decision: "block"` to prevent further actions |
| VSC-B3 | before_prompt | prevent | **CORRECTED: OBSERVE** | UserPromptSubmit uses "common output format only" — lacks hook-specific blocking. Can inject context but cannot block. |
| VSC-B4 | session_start | observe | CONFIRMED | SessionStart only has `additionalContext` output |
| VSC-B5 | agent_stop | retry | **CORRECTED** | `decision: "block"` prevents termination, agent continues (consuming premium requests). Not "retry". |

#### VSC-B2 Details: PostToolUse is NOT purely observe

From VS Code docs: PostToolUse can output `decision: "block"` with a `reason`, preventing further actions. This is not purely observational.

#### VSC-B3 Details: UserPromptSubmit CANNOT block (critical)

From raw vscode-docs source:
> "UserPromptSubmit uses 'the common output format only' — it lacks hook-specific blocking mechanisms like permissionDecision."

Common output fields (`continue`, `stopReason`, `systemMessage`) are present, but the docs explicitly note absence of hook-specific blocking. It can inject `systemMessage` context but cannot deny the prompt.

**Security implication:** Any policy relying on blocking prompts via VS Code Copilot UserPromptSubmit will silently fail.

#### VSC-B5 Details: Stop is "prevent termination", not "retry"

From docs: "When a Stop hook blocks the agent from stopping, the agent continues running and the additional turns consume premium requests."

The mechanism is `decision: "block"` which forces continuation. Not automatic retry. Has cost implications.

### Category C: Capability Validation

| Check | Capability | Spec Claim | Finding |
|-------|-----------|------------|---------|
| VSC-C1 | structured_output | Same as Claude Code (aligned) | PARTIALLY CONFIRMED — similar pattern but property naming differs (VS Code camelCase vs CC snake_case) |
| VSC-C2 | input_rewrite | hookSpecificOutput.updatedInput | CONFIRMED — PreToolUse supports updatedInput |
| VSC-C3 | async_execution | Not supported | CONFIRMED — shell-based, synchronous with timeout |
| VSC-C4 | platform_commands | windows/linux/osx + command fallback | CONFIRMED — uses `osx` not `darwin` |
| VSC-C5 | custom_env | env field (key-value object) | CONFIRMED |
| VSC-C6 | configurable_cwd | cwd field | CONFIRMED |
| VSC-C7 | CC convergence | Intentionally aligned | PARTIALLY CONFIRMED — conceptually aligned but not drop-in compatible due to naming differences |
| VSC-C8 | /create-hook | AI-assisted generation | CONFIRMED — "Users can invoke /create-hook in chat to describe desired automation" |

---

## OpenCode

### Category A: Event Name Mapping Validation

| Check | Canonical | Spec Claim | Finding | Actual |
|-------|-----------|------------|---------|--------|
| OC-A1 | before_tool_execute | tool.execute.before | CONFIRMED | `"tool.execute.before"` in Hooks interface |
| OC-A2 | after_tool_execute | tool.execute.after | CONFIRMED | `"tool.execute.after"` in Hooks interface |
| OC-A3 | session_start | session.created | CONFIRMED | `Session.Event.Created` → `"session.created"` |
| OC-A4 | session_end | -- | CONFIRMED | No session.end; closest is session.deleted (cleanup) |
| OC-A5 | before_prompt | -- | CONFIRMED | No before_prompt hook; closest are `chat.message` and `chat.params` |
| OC-A6 | agent_stop | session.idle | CONFIRMED | session.idle is a bus event, accessed via generic `event` hook |
| OC-A7 | error_occurred | session.error | CONFIRMED | `Session.Event.Error` → `"session.error"` |
| OC-A8 | file_changed | file.edited | CONFIRMED | `File.Event.Edited` → `"file.edited"` |
| OC-A9 | permission_request | permission.asked | **CORRECTED** | Bus event = `permission.asked`, plugin hook = `permission.ask` (different names). **CRITICAL: permission.ask hook is never triggered at runtime (bug #7006)** |

#### OC-A6 Details: session.idle access pattern

`session.idle` is NOT a direct typed hook. Plugins must subscribe via the generic `event` handler and check `event.type === "session.idle"`. The `event` handler is fire-and-forget (not awaited — issue #16879).

#### OC-A9 Details: permission.ask is a dead hook

- Bus event published: `permission.asked`
- Plugin hook defined: `permission.ask`
- Bug: `Plugin.trigger("permission.ask", ...)` is never called. The system publishes directly to the UI bus event, bypassing the plugin hook entirely.
- Impact: Plugins cannot intercept permission requests despite the hook being defined in the types.

#### Additional OpenCode hooks not in spec

From the Hooks interface in packages/plugin/src/index.ts:
- `chat.message` — called when new message received
- `chat.params` — modify LLM sampling parameters
- `shell.env` — inject env vars into shell executions
- `tool.definition` — modify tool descriptions/parameters sent to LLM
- `tool.*` (custom tool registration)

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| OC-B1 | before_tool_execute | prevent | CONFIRMED | `throw new Error(msg)` blocks execution |
| OC-B2 | after_tool_execute | observe | CONFIRMED | Tool already ran; no blocking mechanism |
| OC-B3 | session_start | observe | CONFIRMED | event hook is fire-and-forget |
| OC-B4 | agent_stop | observe | CONFIRMED | session.idle via fire-and-forget event handler |

#### OC-B1 Known limitation

Issue #5894: `tool.execute.before` does NOT intercept tool calls from subagents spawned via the `task` tool. Only primary agent tool calls are blocked. This is a security gap.

### Category C: Capability Validation

| Check | Capability | Spec Claim | Finding |
|-------|-----------|------------|---------|
| OC-C1 | structured_output | N/A (in-process) | CONFIRMED — programmatic model, not JSON stdout |
| OC-C2 | input_rewrite | Mutable output.args | CONFIRMED — `output` passed by reference, mutation modifies tool args |
| OC-C3 | async_execution | Not supported | **CORRECTED** — named hooks ARE async (Promise<void>, awaited). Only `event` subscribe-all is fire-and-forget. |
| OC-C4 | platform_commands | Not supported | CONFIRMED — TS plugins, no JSON config with OS fields |
| OC-C5 | custom_env | Not supported | PARTIALLY CONFIRMED — no declarative `env` field, but `shell.env` hook provides programmatic env injection |
| OC-C6 | configurable_cwd | Not supported | CONFIRMED |
| OC-C7 | Plugin system | Bun, npm, custom tools, throw-blocks | ALL CONFIRMED |

#### OC-C3 Details: OpenCode IS async

Every hook in the Hooks interface returns `Promise<void>`:
```typescript
"tool.execute.before"?: (...) => Promise<void>
"tool.execute.after"?: (...) => Promise<void>
"permission.ask"?: (...) => Promise<void>
```

Direct named hooks are awaited. Only the generic `event` subscriber is fire-and-forget. The spec's blanket "not supported" is wrong.

---

## Summary: High-Priority Corrections

1. **VS Code Copilot UserPromptSubmit CANNOT block** — spec says prevent, docs say observe. Security-critical error.

2. **VS Code Copilot PostToolUse CAN block** — spec says observe, but `decision: "block"` is supported.

3. **VS Code Copilot Stop is "prevent termination", not "retry"** — has premium request cost implications.

4. **OpenCode DOES support async hooks** — named hooks are async/awaited, only event subscribe-all is fire-and-forget.

5. **OpenCode permission.ask hook is never triggered** — documented but dead code (bug #7006).

6. **OpenCode tool.execute.before doesn't intercept subagent calls** — security gap (bug #5894).

7. **VS Code Copilot ↔ Claude Code alignment is directional, not identical** — property naming conventions differ (camelCase vs snake_case).
