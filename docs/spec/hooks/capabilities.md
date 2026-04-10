# Capability Registry

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines optional semantic features (capabilities) that hooks may use
> beyond the core event/handler/blocking model, and the degradation strategies for
> handling capability gaps during cross-provider conversion.

**Registry Version:** 2026.04
**Last Modified:** 2026-04-08
**Status:** Initial Development

---

## 1. Capability Registry

Capabilities describe optional semantic features that a hook uses beyond the core event/handler/blocking model. Each capability has a support matrix, an inference rule, and a default degradation strategy.

Capabilities are **inferred by tooling** from manifest fields and handler output patterns. Hook authors do not need to declare capabilities manually. The optional `capabilities` array on a hook definition is informational and MAY be used by tooling for display purposes.

### 1.1 `structured_output`

Hook produces JSON output with fields beyond simple exit codes.

**Inference rule:** Tooling infers this capability when the hook manifest includes `blocking: true` (implying the hook returns structured decisions), when the hook's `degradation` field references output-dependent capabilities, or when the hook author has explicitly listed `"structured_output"` in the `capabilities` array.

**Advanced output fields** defined by this capability:

| Field | Type | Description |
|-------|------|-------------|
| `suppress_output` | boolean | Suppress the tool's output from being shown to the user. |
| `system_message` | string | Message injected into the system prompt. |

**Support matrix:**

| Provider | Mechanism | Notes |
|----------|-----------|-------|
| claude-code | `hookSpecificOutput` with rich fields | Most expressive |
| vs-code-copilot | Same as claude-code | Aligned implementation |
| gemini-cli | `decision`, `systemMessage`, `hookSpecificOutput` | |
| copilot-cli | `permissionDecision` only | Minimal |
| cursor | `permission`, `userMessage`, `agentMessage` | |
| windsurf | Not supported | Exit codes only |
| kiro | Undocumented | |
| opencode | N/A (in-process) | Programmatic model; blocking works via thrown JavaScript exceptions rather than exit codes or structured output fields |
| factory-droid | Same schema as codex: `continue`, `decision`, `suppressOutput`, `systemMessage`, `hookSpecificOutput`, `permissionDecision`, `additionalContext` | Closely aligned with claude-code |
| codex | Rich output: `continue`, `decision`, `reason`, `stopReason`, `suppressOutput`, `systemMessage`, `hookSpecificOutput` (with `permissionDecision`, `additionalContext`, `updatedInput`/`updatedMCPToolOutput`). `permissionDecision` supports three-way: `"allow"` / `"deny"` / `"ask"` | More granular than claude-code's binary allow/deny |
| cline | `{ cancel: boolean, contextModification?: string, errorMessage?: string }` | Simpler schema; `cancel` is the blocking mechanism; `contextModification` injects text (max 50KB) |

**Kiro-specific capability note:** Kiro supports a `cache_ttl_seconds` field on hook entries that caches hook execution results for the specified duration. This is a Kiro-unique capability with no canonical equivalent. Conversion tools should preserve this value in `provider_data` during decode and drop it with a warning during encode to other providers.

**Default degradation:** `warn`. The hook executes; JSON output fields that cannot be mapped are silently dropped with a warning.

### 1.2 `input_rewrite`

Hook modifies tool arguments before execution. **This is a safety-critical capability.** A hook that sanitizes inputs provides no protection if the rewrite is silently dropped on the target provider.

**Inference rule:** Tooling infers this capability when the hook output includes an `updated_input` field.

**Advanced output field** defined by this capability:

| Field | Type | Description |
|-------|------|-------------|
| `updated_input` | object | Modified tool arguments that replace the original input. |

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| claude-code | `hookSpecificOutput.updatedInput` |
| vs-code-copilot | `hookSpecificOutput.updatedInput` |
| gemini-cli | `hookSpecificOutput.tool_input` — note: gemini-cli uses `tool_input`, not `updatedInput`. Claude Code and gemini-cli diverge here. |
| codex | `hookSpecificOutput.updatedInput` |
| opencode | Mutable `output.args` in plugin |
| All others | Not supported |

**Default degradation:** `block`. When the target provider does not support input rewriting, the adapter MUST generate a hook that blocks the action entirely (exit code 2) rather than allowing unmodified input through. This prevents a false sense of security. Hook authors MAY override to `warn` or `exclude` via the `degradation` field when the rewrite is cosmetic rather than safety-critical.

### 1.3 `llm_evaluated`

Hook logic is evaluated by an LLM rather than a deterministic script.

**Inference rule:** Tooling infers this capability when `handler.type` is `"prompt"` or `"agent"`.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| claude-code | `type: "prompt"` (single-turn) and `type: "agent"` (multi-turn with tools) |
| codex | `type: "prompt"` (single-turn) and `type: "agent"` (multi-turn) — same handler types as claude-code |
| kiro | "Ask Kiro" agent prompt actions (IDE only, consumes credits) |
| All others | Not supported |

**Default degradation:** `exclude`. LLM evaluation IS the hook -- wrapping it in a CLI shim is fragile and requires the source provider's infrastructure. Hook authors MAY override to `warn` if partial functionality is acceptable.

### 1.4 `http_handler`

Hook executes via HTTP POST to an endpoint rather than a local process.

**Inference rule:** Tooling infers this capability when `handler.type` is `"http"`.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| claude-code | `type: "http"` with `url`, `headers`, `allowedEnvVars` |
| All others | Not supported |

**Default degradation:** `warn`. An adapter MAY approximate HTTP handlers by generating a shell script that invokes `curl` or an equivalent HTTP client. The generated script SHOULD preserve the URL, method, headers, and body contract.

### 1.5 `async_execution`

Hook runs without blocking the agent (fire-and-forget).

**Inference rule:** Tooling infers this capability when `handler.async` is `true`.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| claude-code | `async: true` |
| codex | `async: true` on command handlers; also supports `scope: "thread"` or `"turn"` execution scope |
| All others | Hooks execute synchronously |

**Default degradation:** `warn`. The hook executes synchronously. A warning is emitted that the hook may block the agent.

### 1.6 `platform_commands`

Per-operating-system command overrides.

**Inference rule:** Tooling infers this capability when the `handler.platform` field is present.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| vs-code-copilot | `windows`, `linux`, `osx` fields with `command` fallback (note: native format uses `osx`; the canonical format uses `darwin`) |
| copilot-cli | `bash`, `powershell` fields |
| claude-code | `shell: "bash" \| "powershell"` — selects the shell interpreter rather than providing per-OS command overrides. Not equivalent to vs-code-copilot's per-OS pattern but is a platform-related command field. |
| cline | Windows: `{HookName}.ps1`; Unix: extensionless `{HookName}` executable scripts. Platform selection is implicit via filename extension. |
| All others | Single `command` field only |

**Default degradation:** `warn`. The default `command` is used. Platform-specific overrides are dropped with a warning.

### 1.7 `custom_env`

Custom environment variables passed to the hook process.

**Inference rule:** Tooling infers this capability when the `handler.env` field is present.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| vs-code-copilot | `env` field (key-value object) |
| gemini-cli | `env` field (`Record<string, string>`) in `CommandHookConfig` — present in source code type definitions; not documented in official docs but confirmed in types.ts |
| All others | Not supported |

**Default degradation:** `warn`. Environment variables are dropped with a warning.

### 1.8 `configurable_cwd`

Explicit working directory for hook execution.

**Inference rule:** Tooling infers this capability when the `handler.cwd` field is present.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| windsurf | `working_directory` field |
| copilot-cli | `cwd` field |
| vs-code-copilot | `cwd` field |
| All others | Not configurable (implementation-defined default) |

**Default degradation:** `warn`. The `cwd` value is ignored with a warning that the hook may execute from an unexpected directory.

---

## 2. Degradation Strategies

When converting a hook to a target provider that lacks support for a capability the hook uses, a **degradation strategy** determines the behavior.

### 2.1 Strategy Values

| Strategy | Behavior |
|----------|----------|
| `block` | The converted hook blocks the action entirely (exit code 2) instead of running with reduced functionality. Use for safety-critical capabilities where silent degradation would be dangerous. |
| `warn` | The converted hook runs with reduced functionality. The adapter emits a warning describing what was lost. Use for convenience capabilities where partial operation is acceptable. |
| `exclude` | The hook is excluded entirely from the target provider's configuration. The adapter emits a warning that the hook was omitted. Use for capabilities that are the entirety of the hook's purpose. |

### 2.2 Author-Specified Degradation

Hook authors MAY specify per-capability degradation strategies in the `degradation` field:

```json
{
  "event": "before_tool_execute",
  "handler": {
    "type": "command",
    "command": "./sanitize-input.sh"
  },
  "blocking": true,
  "degradation": {
    "input_rewrite": "block",
    "custom_env": "warn"
  }
}
```

### 2.3 Safe Defaults

When the `degradation` field is absent or does not specify a strategy for a given capability, the following defaults apply:

| Capability | Default Strategy | Rationale |
|------------|-----------------|-----------|
| `structured_output` | `warn` | Partial output is better than no output. |
| `input_rewrite` | `block` | Silent drop of input sanitization creates a false sense of security. |
| `llm_evaluated` | `exclude` | LLM evaluation is the hook's entire purpose. |
| `http_handler` | `warn` | Can be approximated with generated `curl` script. |
| `async_execution` | `warn` | Synchronous execution is a safe fallback. |
| `platform_commands` | `warn` | Default command is available. |
| `custom_env` | `warn` | Missing env vars may cause hook errors, but not safety issues. |
| `configurable_cwd` | `warn` | Wrong directory is detectable and not a security risk. |

## Provider Capability Support

The following table is auto-generated from `docs/provider-capabilities/*.yaml`. Do not edit by hand — run `syllago capmon generate` to refresh.

<!-- GENERATED FROM provider-capabilities/*.yaml -->
| Canonical Event | amp | claude-code | cline | codex | copilot-cli | crush | cursor | factory-droid | gemini-cli | kiro | opencode | pi | roo-code | windsurf | zed |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

<!-- END GENERATED -->
