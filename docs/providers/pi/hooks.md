# Pi Extensions (Hooks) System

> **Identity note:** "Pi" refers to Mario Zechner's `pi` coding agent
> (`github.com/badlogic/pi-mono`). Not to be confused with Raspberry Pi hardware
> or other "Pi" named projects.

> Research date: 2026-03-30
> Status: Draft -- compiled from official repository source code and docs.

Pi uses a programmatic TypeScript extension system rather than a declarative hook
configuration format. Extensions subscribe to lifecycle events via the
`ExtensionAPI.on()` method, giving them full control over agent behavior at each
event point.

**Key difference from other providers:** There is no JSON/YAML hook config file.
Extensions are TypeScript modules that register event handlers programmatically.
This makes Pi's system more powerful but less portable -- there is no declarative
format to convert to/from.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md]

## Extension File Locations

Extensions are auto-discovered from these locations:

| Scope | Path |
|---|---|
| Project | `.pi/extensions/*.ts` |
| Global | `~/.pi/agent/extensions/*.ts` |
| Subdirectory | `.pi/extensions/<name>/index.ts` |
| Settings | `extensions` array in `settings.json` |
| CLI | `--extension <path>` flag |

Discovery is one level deep. Subdirectories must have an `index.ts` or
`index.js` entry point, or a `package.json` with a `pi` manifest field declaring
extension entry points.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/loader.ts]

## Extension Structure

A minimal extension exports a default function receiving `ExtensionAPI`:

```typescript
import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("session_start", async (_event, ctx) => {
    ctx.ui.notify("Extension loaded!", "info");
  });
}
```

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md]

## Supported Events

Pi defines 25+ lifecycle events across several categories. All event names are
snake_case strings.

### Session Events

| Event | When it fires | Can cancel? |
|---|---|---|
| `session_directory` | Custom session directory resolution | No (returns path) |
| `session_start` | Session started, loaded, or reloaded | No |
| `session_before_switch` | Before switching sessions | Yes (`{ cancel: true }`) |
| `session_before_fork` | Before forking a session | Yes (`{ cancel: true }`) |
| `session_before_compact` | Before context compaction | Yes (`{ cancel: true }`) or custom compaction |
| `session_compact` | After compaction completes | No |
| `session_shutdown` | Process exit | No |
| `session_before_tree` | Before tree navigation | Yes (`{ cancel: true }`) |
| `session_tree` | After tree navigation | No |

### Agent Events

| Event | When it fires | Can cancel? |
|---|---|---|
| `before_agent_start` | Before agent loop begins | No (can inject message/systemPrompt) |
| `agent_start` | Agent loop starts | No |
| `agent_end` | Agent loop ends | No |
| `turn_start` | Turn begins | No |
| `turn_end` | Turn ends | No |
| `context` | Before LLM call, messages modifiable | No (returns modified messages) |
| `before_provider_request` | Before provider API request | No |

### Message Events

| Event | When it fires | Can cancel? |
|---|---|---|
| `message_start` | Assistant message begins | No |
| `message_update` | Token-by-token streaming update | No |
| `message_end` | Assistant message completes | No |

### Tool Events

| Event | When it fires | Can cancel? |
|---|---|---|
| `tool_call` | Before tool executes | Yes (`{ block: true, reason: "..." }`) |
| `tool_result` | After tool executes | No (result modifiable: content, details, isError) |
| `tool_execution_start` | Tool execution begins | No |
| `tool_execution_update` | Partial/streaming tool output | No |
| `tool_execution_end` | Tool execution finishes | No |

### User/Input Events

| Event | When it fires | Can cancel? |
|---|---|---|
| `input` | User input received, before processing | Yes (transform or handle) |
| `user_bash` | User runs bash via `!` or `!!` prefix | No (can override operations/result) |

### Resource Events

| Event | When it fires | Can cancel? |
|---|---|---|
| `resources_discover` | Extension provides additional resource paths | No |

### Model Events

| Event | When it fires | Can cancel? |
|---|---|---|
| `model_select` | New model selected | No |

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/types.ts]

## Blocking and Cancellation

Pi does **not** use `throw` for blocking. Instead, handlers return structured
result objects:

- **Tool blocking:** Return `{ block: true, reason: "explanation" }` from
  `tool_call` handlers.
- **Session cancellation:** Return `{ cancel: true }` from `session_before_*`
  handlers.
- **Input transformation:** Return `{ action: "transform", text: "new text" }`
  or `{ action: "handled" }` from `input` handlers.

Errors thrown by handlers are caught, logged as error events, and do **not**
block execution. This is error isolation, not a blocking mechanism.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/runner.ts]

## Execution Model

- Handlers execute **sequentially** across all registered extensions
- A fresh context is created for each emit cycle
- Errors in one handler do not prevent subsequent handlers from running (error
  isolation)
- Some events accumulate results across handlers (e.g., `before_agent_start`
  gathers messages and system prompt modifications from all extensions)
- Input events support chaining: a `transform` result modifies data for
  subsequent handlers; `handled` short-circuits

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/runner.ts]

## Runtime

Extensions are loaded via **Jiti** (a TypeScript-to-JS transpiler/loader):

- **Bun binary mode:** Uses `virtualModules` with pre-bundled packages; Jiti
  handles all imports (no native resolution)
- **Node.js/development:** Uses aliases mapping to workspace paths or
  `node_modules`

Pre-bundled packages available to extensions: `@sinclair/typebox`,
`@mariozechner/pi-agent-core`, `@mariozechner/pi-tui`, `@mariozechner/pi-ai`,
and the coding agent itself.

Registration methods (tools, commands, flags) are available immediately. Action
methods (sendMessage, exec) are stubbed until the core binds, then become
functional.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/loader.ts]

## Typed Tool Call Events

Pi provides typed event variants for each built-in tool:

| Tool | Call Event Type | Result Event Type |
|---|---|---|
| bash | `BashToolCallEvent` | `BashToolResultEvent` |
| read | `ReadToolCallEvent` | `ReadToolResultEvent` |
| edit | `EditToolCallEvent` | `EditToolResultEvent` |
| write | `WriteToolCallEvent` | `WriteToolResultEvent` |
| grep | `GrepToolCallEvent` | `GrepToolResultEvent` |
| find | `FindToolCallEvent` | `FindToolResultEvent` |
| ls | `LsToolCallEvent` | `LsToolResultEvent` |
| (custom) | `CustomToolCallEvent` | `CustomToolResultEvent` |

Type guards (`isToolCallEventType()`, `isBashToolResult()`, etc.) enable safe
narrowing in event handlers.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/types.ts]

## Comparison with Other Providers

| Feature | Pi | Claude Code | Cursor |
|---|---|---|---|
| Hook format | Programmatic TypeScript | Declarative JSON (settings.json) | Declarative JSON (hooks.json) |
| Config location | `.pi/extensions/*.ts` | `.claude/settings.json` `hooks` key | `.cursor/hooks.json` |
| Events supported | 25+ | 4 | 6 |
| Blocking mechanism | Return `{ block: true }` | Exit code (0=allow, 2=deny) | JSON `{ permission: "deny" }` |
| Custom tools | Yes (full tool registration) | No | No |
| UI interaction | Yes (dialogs, widgets, overlays) | No | No |
| Runtime | Jiti (TypeScript) | Shell processes | Shell processes |

## Sources

- [Pi Extensions Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md) [Official]
- [Extension Types (types.ts)](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/types.ts) [Official]
- [Extension Loader (loader.ts)](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/loader.ts) [Official]
- [Extension Runner (runner.ts)](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/runner.ts) [Official]
- [Extension Examples](https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent/examples/extensions) [Official]
