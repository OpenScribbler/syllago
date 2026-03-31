# Hook Interchange Format Specification

**Version:** 0.1.0
**Status:** Initial Development
**License:** CC-BY-4.0

## Abstract

This specification defines a provider-neutral interchange format for AI coding tool hooks. It enables hook authors to write lifecycle hooks once and deploy them across multiple AI coding tool providers through a canonical representation, capability model, and conversion pipeline. The format covers event binding, handler execution, matcher syntax, exit code contracts, structured output, and degradation strategies for cross-provider portability.

## Status of This Document

This specification is in initial development (major version zero). Per [Semantic Versioning](https://semver.org/), anything MAY change at any time. The public interface should not be considered stable until version 1.0.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Terminology](#2-terminology)
3. [Canonical Format](#3-canonical-format)
4. [Exit Code Contract](#4-exit-code-contract)
5. [Canonical Output Schema](#5-canonical-output-schema)
6. [Matcher Types](#6-matcher-types)
7. [Event Registry](#7-event-registry)
8. [Blocking Behavior Matrix](#8-blocking-behavior-matrix)
9. [Capability Registry](#9-capability-registry)
10. [Tool Vocabulary](#10-tool-vocabulary)
11. [Degradation Strategies](#11-degradation-strategies)
12. [Conversion Pipeline](#12-conversion-pipeline)
13. [Conformance Levels](#13-conformance-levels)
14. [Versioning](#14-versioning)
- [Appendix A: Provider Strengths](#appendix-a-provider-strengths)
- [Appendix B: JSON Schema Reference](#appendix-b-json-schema-reference)

---

## 1. Introduction

AI coding tools provide hook systems that let developers run custom logic at lifecycle points: before a tool executes, when a session starts, after the agent stops. These hooks enforce policy, audit actions, inject context, and customize behavior.

Every provider's hook system is different. Event names, JSON schemas, output contracts, matcher syntax, timeout units, and configuration file layouts vary across providers. A hook written for one tool cannot run on another without manual adaptation.

This specification defines a **canonical hook format** that serves as an interchange representation between providers. It does not replace any provider's native format. Instead, it provides a neutral hub through which hooks are converted: decode from a source provider's format into canonical, then encode from canonical into a target provider's format.

### 1.1 What This Spec Defines

- A canonical JSON format for hook manifests
- An exit code contract for hook processes
- A canonical output schema for structured hook responses
- A typed matcher system for tool filtering
- A registry of canonical event names with provider mappings
- A capability model describing optional features with provider support matrices
- A tool vocabulary mapping canonical tool names to provider-native names
- Degradation strategies for handling capability gaps during conversion
- Conformance levels for implementations

### 1.2 What This Spec Does Not Define

- How providers implement their native hook systems
- Hook distribution, packaging, or registry publishing mechanisms
- Cryptographic signing, provenance, or trust policies
- Enterprise policy enforcement (see [policy-interface.md](policy-interface.md) for the interface contract)
- Hook authoring workflows or IDE integrations

---

## 2. Terminology

Terms used in this specification are defined in the [Glossary](glossary.md). The key words "MUST," "MUST NOT," "REQUIRED," "SHALL," "SHALL NOT," "SHOULD," "SHOULD NOT," "RECOMMENDED," "MAY," and "OPTIONAL" in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

---

## 3. Canonical Format

### 3.1 File Format

The canonical interchange format is JSON. Hook manifests MAY be authored in YAML format; conforming implementations MUST accept both JSON and YAML representations and MUST produce identical canonical structures from either.

### 3.2 Forward Compatibility

Implementations MUST ignore unknown fields at any level of the canonical format. This ensures that manifests written against a newer version of this specification can be processed by older implementations without error. Implementations MUST NOT reject a manifest solely because it contains unrecognized fields.

### 3.3 Top-Level Structure

A canonical hook manifest is a JSON object with the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec` | string | REQUIRED | Specification version identifier. MUST be `"hooks/0.1"` for this version. |
| `hooks` | array | REQUIRED | Non-empty array of hook definition objects. Implementations MUST reject a manifest with an empty `hooks` array as a validation error. |

When multiple hooks bind to the same event, implementations MUST execute them in the order they appear in the `hooks` array. Implementations MUST NOT reorder hooks.

### 3.4 Hook Definition

Each element of the `hooks` array is an object with the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | OPTIONAL | Human-readable identifier for this hook. Used in warnings, logs, and policy references. When omitted, implementations SHOULD refer to the hook by its position in the array (e.g., "Hook 1"). |
| `event` | string | REQUIRED | Canonical event name from the Event Registry (Section 7). |
| `matcher` | string, object, or array | OPTIONAL | Tool matcher expression (Section 6). When omitted, the hook matches all tools for the bound event. |
| `handler` | object | REQUIRED | Handler definition (Section 3.5). |
| `blocking` | boolean | OPTIONAL | Whether this hook can prevent the triggering action. Default: `false`. |
| `degradation` | object | OPTIONAL | Per-capability fallback strategies (Section 11). Keys are capability identifiers, values are strategy strings. |
| `provider_data` | object | OPTIONAL | Opaque provider-specific data, keyed by provider slug. |
| `capabilities` | array | OPTIONAL | Informational list of capability identifier strings. Implementations SHOULD infer capabilities from manifest fields and handler output patterns rather than relying on this field. See Section 9 for the inference rules. |

### 3.5 Handler Definition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | REQUIRED | Handler type. MUST be `"command"` for shell-based handlers. Other types are defined as capabilities in Section 9. |
| `command` | string | Conditional | Shell command or script path, relative to the hook directory. REQUIRED when `type` is `"command"`. |
| `platform` | object | OPTIONAL | Per-OS command overrides. Keys MUST be `"windows"`, `"linux"`, or `"darwin"`. Values are command strings. The `command` field serves as the default when no platform override matches. |
| `cwd` | string | OPTIONAL | Working directory for the hook process, relative to the project root. |
| `env` | object | OPTIONAL | Environment variables passed to the hook process. Keys are variable names, values are strings. |
| `timeout` | number | OPTIONAL | Maximum execution time in seconds. Implementations SHOULD apply a reasonable default (30 seconds is RECOMMENDED). A value of `0` means no timeout (the implementation's default applies). |
| `timeout_action` | string | OPTIONAL | Behavior when the hook exceeds its timeout. MUST be `"warn"` or `"block"`. Default: `"warn"`. When `"warn"`, timeout is treated as exit code 1 (non-blocking warning). When `"block"`, timeout is treated as exit code 2 (action prevented). Only meaningful when `blocking` is `true` on the parent hook definition; when `blocking` is `false`, timeout always degrades to a warning regardless of this field. |
| `async` | boolean | OPTIONAL | Whether the hook runs asynchronously (fire-and-forget). Default: `false`. When `true`, the triggering action does not wait for the hook to complete. |
| `status_message` | string | OPTIONAL | Human-readable status text displayed to the user while the hook executes (e.g., `"Running linter..."`). When omitted, no status is shown. Not all providers support this field; providers that do not support it silently ignore it. |

#### Script References vs. Inline Commands

The `command` field accepts two forms:

- **Script reference:** A relative path starting with `./` or containing a path separator (e.g., `"./check.sh"`, `"./scripts/audit.py"`). Resolved relative to the hook directory.
- **Inline command:** A shell command string (e.g., `"grep -r TODO . | wc -l"`). Passed directly to the system shell.

Hook authors SHOULD use script references for blocking hooks. Script references enable content integrity verification (SHA-256 hashing of script files), security scanning (static analysis of script contents), and explicit shell selection (via shebang line). Inline commands bypass all three and are passed to the system's default shell, which varies across platforms and providers.

### 3.6 Provider Data

The `provider_data` field holds opaque data keyed by canonical provider slug. Each value is a JSON object whose schema is defined by the respective provider, not by this specification.

During conversion:
- An adapter encoding for provider P MUST render `provider_data[P]` into the target format if present.
- An adapter encoding for provider P MUST silently ignore `provider_data[Q]` where Q != P.
- An adapter decoding from provider P SHOULD preserve provider-specific fields that have no canonical equivalent in `provider_data[P]`.

Canonical provider slugs:

| Slug | Provider |
|------|----------|
| `claude-code` | Claude Code |
| `gemini-cli` | Gemini CLI |
| `cursor` | Cursor |
| `windsurf` | Windsurf |
| `vs-code-copilot` | VS Code Copilot |
| `copilot-cli` | GitHub Copilot CLI |
| `kiro` | Kiro |
| `opencode` | OpenCode |

### 3.7 Minimal Example

```json
{
  "spec": "hooks/0.1",
  "hooks": [
    {
      "event": "before_tool_execute",
      "handler": {
        "type": "command",
        "command": "./check.sh"
      },
      "blocking": true
    }
  ]
}
```

### 3.8 Full Example

```json
{
  "spec": "hooks/0.1",
  "hooks": [
    {
      "name": "safety-check",
      "event": "before_tool_execute",
      "matcher": "shell",
      "handler": {
        "type": "command",
        "command": "./safety-check.sh",
        "platform": {
          "windows": "./safety-check.ps1"
        },
        "cwd": ".",
        "env": {
          "AUDIT_LOG": "./audit.log"
        },
        "timeout": 10
      },
      "blocking": true,
      "degradation": {
        "input_rewrite": "block"
      },
      "provider_data": {
        "windsurf": {
          "show_output": true,
          "working_directory": "/opt/hooks"
        }
      }
    },
    {
      "event": "before_model",
      "handler": {
        "type": "command",
        "command": "./redact-pii.sh",
        "timeout": 5
      },
      "blocking": true
    },
    {
      "event": "before_tool_execute",
      "matcher": {"mcp": {"server": "github", "tool": "create_issue"}},
      "handler": {
        "type": "command",
        "command": "./validate-issue.sh"
      },
      "blocking": true
    },
    {
      "event": "file_saved",
      "matcher": {"pattern": "\\.test\\.(ts|js)$"},
      "handler": {
        "type": "command",
        "command": "./run-tests.sh"
      },
      "blocking": false
    },
    {
      "event": "before_tool_execute",
      "matcher": ["file_write", "file_edit"],
      "handler": {
        "type": "command",
        "command": "./lint-on-write.sh"
      },
      "blocking": false
    },
    {
      "event": "session_start",
      "handler": {
        "type": "command",
        "command": "./init.sh"
      },
      "blocking": false
    }
  ]
}
```

---

## 4. Exit Code Contract

When a hook handler of type `"command"` terminates, the exit code determines how the calling tool processes the result.

| Exit Code | Name | Behavior |
|-----------|------|----------|
| 0 | Success | Action proceeds. Stdout is parsed as JSON if present (Section 5). Empty stdout is treated as `{}`. |
| 1 | Hook Error | Non-blocking warning. Action proceeds. Stderr SHOULD be logged. |
| 2 | Block | Action is prevented. Stderr SHOULD be presented to the user or agent as the blocking reason. Only meaningful when `blocking` is `true` for the hook; when `blocking` is `false`, exit code 2 MUST be treated as exit code 1 (non-blocking warning). |
| Other | Hook Error | Same behavior as exit code 1. |

Implementations MUST NOT treat exit code 1 or other non-zero codes (besides 2) as blocking, regardless of the `blocking` field.

When a hook exceeds its timeout, the `timeout_action` field on the handler definition determines the behavior (see Section 3.5). The default is `"warn"` (treat as exit code 1, action proceeds).

---

## 5. Canonical Output Schema

When a hook process exits with code 0 and produces JSON on stdout, the output is interpreted according to this schema. All fields are OPTIONAL. An empty JSON object (`{}`) or empty stdout is valid.

### 5.1 Core Fields

| Field | Type | Description |
|-------|------|-------------|
| `decision` | string | One of `"allow"`, `"deny"`, or `"ask"`. Controls whether the triggering action proceeds. `"allow"` permits the action. `"deny"` prevents it (equivalent to exit code 2). `"ask"` defers to the user for confirmation. |
| `reason` | string | Human-readable explanation of the decision. Implementations SHOULD present this to the user or agent. |
| `continue` | boolean | Whether the agent should continue its loop after this hook. Default: `true`. When `false`, the agent SHOULD stop. |
| `context` | string | Additional context injected into the agent's conversation or system prompt. |

### 5.2 Interaction with Exit Codes

When both exit code and JSON output are present:
- Exit code 2 takes precedence over `decision: "allow"` in the JSON output.
- `decision: "deny"` with exit code 0 MUST be treated as a block (equivalent to exit code 2).
- `decision: "allow"` with exit code 0 is the normal success path.
- `decision: "ask"` with exit code 0 defers to the user; if the provider does not support interactive confirmation, it MUST be treated as `"deny"`. This includes non-interactive environments such as CI/CD pipelines, headless sessions, and automated workflows where no user is available to respond.
- When the `decision` field is absent and exit code is 0, the implementation MUST treat the result as `decision: "allow"`.
- When a hook exits with code 0 and stdout is not valid JSON, implementations MUST treat the result as exit code 1 (hook error) and SHOULD log stderr and stdout for debugging.

### 5.3 Evaluation Order

When both exit code and JSON `decision` are present, the following evaluation order applies:

1. **Non-blocking downgrade first.** If `blocking` is `false`, exit code 2 MUST be treated as exit code 1 before any further evaluation.
2. **Exit code / decision resolution.** After the downgrade, apply the precedence rules from Section 5.2.

The combined truth table:

| `blocking` | Exit Code | `decision` | Result |
|------------|-----------|------------|--------|
| `true` | 0 | `"allow"` | Allow |
| `true` | 0 | `"deny"` | Block |
| `true` | 0 | `"ask"` | Ask user (deny if unsupported) |
| `true` | 0 | absent | Allow |
| `true` | 2 | `"allow"` | Block (exit code 2 overrides) |
| `true` | 2 | `"deny"` | Block |
| `true` | 1 | any | Warning, allow |
| `false` | 0 | `"allow"` | Allow |
| `false` | 0 | `"deny"` | Block |
| `false` | 0 | absent | Allow |
| `false` | 2 | any | Warning, allow (downgraded to exit 1) |
| `false` | 1 | any | Warning, allow |

### 5.4 Advanced Output Fields

The following fields are capability-specific. They are defined in the Capability Registry (Section 9) alongside their respective capabilities, not in the core output schema. Implementations MUST ignore these fields if the corresponding capability is not supported.

- `updated_input` -- defined by `input_rewrite` capability
- `suppress_output` -- defined by `structured_output` capability
- `system_message` -- defined by `structured_output` capability

---

## 6. Matcher Types

The `matcher` field on a hook definition controls which tools trigger the hook for tool-related events (e.g., `before_tool_execute`, `after_tool_execute`). For non-tool events (e.g., `session_start`), the `matcher` field is not applicable and MUST be ignored by implementations.

### 6.1 Bare String

A bare string value is a **tool vocabulary lookup**. It MUST match against the canonical tool names defined in the Tool Vocabulary (Section 10). Implementations resolve the canonical name to the provider-native tool name during encoding.

When a bare string does not match any canonical tool name in the Tool Vocabulary, implementations MUST pass it through as a literal string and SHOULD emit a warning. This ensures forward compatibility when new tool names are added to the vocabulary: manifests using the new name will work with older implementations that do not yet recognize it.

```json
"matcher": "shell"
```

### 6.2 Pattern Object

An object with a `pattern` key specifies a regular expression matched against the provider-native tool name. The regex flavor MUST be RE2 for cross-language compatibility. RE2 guarantees linear-time matching and is available in Go, Rust, Python, JavaScript (via libraries), Java, and other languages. Implementations MUST NOT accept patterns that require features beyond RE2 (e.g., backreferences, lookahead).

```json
"matcher": {"pattern": "file_(read|write|edit)"}
```

Pattern matchers are **not portable** across providers. The pattern is passed through to the target provider and matched at runtime against that provider's native tool names. Hook authors who need cross-provider portability SHOULD use bare string matchers (Section 6.1), which are translated through the Tool Vocabulary. Pattern matchers are an escape hatch for provider-specific tool names that the vocabulary does not cover.

### 6.3 MCP Object

An object with an `mcp` key matches MCP (Model Context Protocol) tools. The `mcp` value is an object with the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `server` | string | REQUIRED | MCP server name. |
| `tool` | string | OPTIONAL | MCP tool name. When omitted, matches all tools on the specified server. |

```json
"matcher": {"mcp": {"server": "github", "tool": "create_issue"}}
```

```json
"matcher": {"mcp": {"server": "github"}}
```

Implementations encode MCP matchers into provider-specific combined formats during conversion. The canonical format uses a structured object to avoid the parsing ambiguity of combined string formats, which vary across providers.

| Provider | Combined Format | Example |
|----------|----------------|---------|
| claude-code, kiro | `mcp__<server>__<tool>` | `mcp__github__create_issue` |
| gemini-cli | `mcp_<server>_<tool>` | `mcp_github_create_issue` |
| copilot-cli | `<server>/<tool>` | `github/create_issue` |
| cursor, windsurf | `<server>__<tool>` | `github__create_issue` |

### 6.4 Array (OR)

An array of matchers matches if **any** element matches. Array elements MAY be bare strings, pattern objects, or MCP objects.

```json
"matcher": ["shell", "file_write", {"mcp": {"server": "github"}}]
```

### 6.5 Omitted

When the `matcher` field is absent, the hook matches **all tools** for the bound event. This is equivalent to a wildcard.

---

## 7. Event Registry

Canonical event names use `snake_case` and describe the lifecycle moment in provider-neutral terms.

### 7.1 Core Events

Core events have near-universal provider support. Conforming implementations at the Core level (Section 13) MUST support all core events.

| Event | Description | Blocking Semantic |
|-------|-------------|-------------------|
| `before_tool_execute` | Fires before any tool runs. The hook receives the tool name and input arguments. | Can prevent the tool from executing. |
| `after_tool_execute` | Fires after a tool completes. The hook receives the tool name, input, and result. | Observational only; the action has already occurred. Setting `blocking: true` on an observe-only event is not a validation error, but implementations SHOULD warn that the blocking intent has no effect. |
| `session_start` | Fires when a coding session begins, resumes, or resets. | Observational; may inject context. |
| `session_end` | Fires when a session terminates. Best-effort delivery; the process may exit before the hook completes. | Observational. |
| `before_prompt` | Fires after user input is submitted but before it reaches the agent. | Can modify or reject user input. |
| `agent_stop` | Fires when the agent's main loop ends. | Can trigger a retry (provider-dependent). |

### 7.2 Extended Events

Extended events have partial provider support. They appear in the event registry but are not required for Core conformance.

| Event | Description |
|-------|-------------|
| `before_compact` | Fires before context window compression. |
| `notification` | Non-blocking system notification (e.g., permission prompts, status updates). |
| `error_occurred` | Fires when the agent encounters an error. |
| `tool_use_failure` | Fires when a tool invocation fails. Distinct from `after_tool_execute` in that it signals an error, not a successful completion. |
| `file_changed` | Fires when a file is created, modified, or saved in the project. |
| `subagent_start` | Fires when a nested agent is spawned. |
| `subagent_stop` | Fires when a nested agent finishes. |
| `permission_request` | Fires when a sensitive action requires permission. |
| `before_model` | Fires before an LLM API call. Hook can intercept or mock the request. |
| `after_model` | Fires after an LLM response is received. Hook can redact or modify. |
| `before_tool_selection` | Fires before the LLM chooses which tool to use. Hook can filter the tool list. |

### 7.3 Provider-Exclusive Events

Provider-exclusive events exist in only one provider. They are included in the registry for lossless round-tripping but are expected to be dropped or degraded during cross-provider conversion.

| Event | Description | Origin Provider(s) |
|-------|-------------|-------------------|
| `config_change` | Fires when configuration is modified. | Claude Code |
| `file_created` | Fires when a new file is created in the project. | Kiro |
| `file_deleted` | Fires when a file is deleted from the project. | Kiro |
| `before_task` | Fires before a spec task executes. | Kiro |
| `after_task` | Fires after a spec task completes. | Kiro |

### 7.4 Event Name Mapping

The following table maps canonical event names to provider-native names. Adapters use this mapping during decode (native to canonical) and encode (canonical to native).

| Canonical | claude-code | gemini-cli | cursor | windsurf | vs-code-copilot | copilot-cli | kiro | opencode |
|-----------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| `before_tool_execute` | PreToolUse | BeforeTool | beforeShellExecution / beforeMCPExecution / beforeReadFile | pre_read_code / pre_write_code / pre_run_command / pre_mcp_tool_use | PreToolUse | preToolUse | preToolUse | tool.execute.before |
| `after_tool_execute` | PostToolUse | AfterTool | afterFileEdit | post_read_code / post_write_code / post_run_command / post_mcp_tool_use | PostToolUse | postToolUse | postToolUse | tool.execute.after |
| `session_start` | SessionStart | SessionStart | -- | -- | SessionStart | sessionStart | agentSpawn | session.created |
| `session_end` | SessionEnd | SessionEnd | -- | -- | -- | sessionEnd | stop | -- |
| `before_prompt` | UserPromptSubmit | BeforeAgent | beforeSubmitPrompt | pre_user_prompt | UserPromptSubmit | userPromptSubmitted | userPromptSubmit | -- |
| `agent_stop` | Stop | AfterAgent | stop | post_cascade_response | Stop | -- | stop | session.idle |
| `before_compact` | PreCompact | PreCompress | -- | -- | PreCompact | -- | -- | -- |
| `notification` | Notification | Notification | -- | -- | -- | -- | -- | -- |
| `error_occurred` | StopFailure | -- | -- | -- | -- | errorOccurred | -- | session.error |
| `tool_use_failure` | PostToolUseFailure | -- | postToolUseFailure | -- | -- | errorOccurred | -- | -- |
| `file_changed` | FileChanged | -- | afterFileEdit | -- | -- | -- | File Save | file.edited |
| `subagent_start` | SubagentStart | -- | subagentStart | -- | SubagentStart | -- | -- | -- |
| `subagent_stop` | SubagentStop | -- | subagentStop | -- | SubagentStop | -- | -- | -- |
| `permission_request` | PermissionRequest | -- | -- | -- | -- | -- | -- | permission.asked |
| `before_model` | -- | BeforeModel | beforeAgentResponse | -- | -- | -- | -- | -- |
| `after_model` | -- | AfterModel | afterAgentResponse | -- | -- | -- | -- | -- |
| `before_tool_selection` | -- | BeforeToolSelection | beforeToolSelection | -- | -- | -- | -- | -- |
| `config_change` | ConfigChange | -- | -- | -- | -- | -- | -- | -- |
| `file_created` | -- | -- | -- | -- | -- | -- | File Create | -- |
| `file_deleted` | -- | -- | -- | -- | -- | -- | File Delete | -- |
| `before_task` | -- | -- | -- | -- | -- | -- | Pre Task Execution | -- |
| `after_task` | -- | -- | -- | -- | -- | -- | Post Task Execution | -- |

A `--` indicates the provider does not support that event. When encoding a hook for a provider that does not support its event, the adapter MUST apply the degradation strategy (Section 11).

**Split-event providers:** Cursor and Windsurf map a single `before_tool_execute` event to multiple provider-native events based on the matcher. When encoding for these providers, adapters MUST inspect the `matcher` field to select the correct native event. When decoding from these providers, adapters MUST merge split events into `before_tool_execute` with an appropriate matcher.

---

## 8. Blocking Behavior Matrix

When a blocking hook returns exit code 2 (or `decision: "deny"`), the resulting behavior depends on both the event and the target provider. This matrix defines the expected behavior per combination.

### 8.1 Behavior Vocabulary

| Behavior | Meaning |
|----------|---------|
| **prevent** | The triggering action is stopped. The tool does not execute, the prompt is not submitted, etc. |
| **retry** | The agent is re-engaged to attempt an alternative approach. |
| **observe** | The hook runs but cannot affect the outcome. Blocking signals are ignored. |

### 8.2 Matrix

| Event | claude-code | gemini-cli | cursor | windsurf | vs-code-copilot | copilot-cli | kiro | opencode |
|-------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| `before_tool_execute` | prevent | prevent | prevent | prevent | prevent | prevent | prevent | prevent |
| `after_tool_execute` | observe | observe | observe | observe | observe | observe | observe | observe |
| `session_start` | observe | observe | -- | -- | observe | observe | observe | observe |
| `session_end` | observe | observe | -- | -- | observe | observe | observe | -- |
| `before_prompt` | prevent | prevent | observe | observe | prevent | observe | observe | -- |
| `agent_stop` | retry | retry | observe | observe | retry | observe | observe | observe |

A `--` indicates the provider does not support the event (same as the Event Name Mapping table).

When encoding a hook with `"blocking": true` for a provider where the behavior is `observe`, adapters MUST emit a warning indicating that the blocking intent cannot be honored on the target provider. If the hook defines a `degradation` strategy for the relevant capability, that strategy applies; otherwise the hook is encoded with the blocking field preserved and the warning emitted.

### 8.3 Timing Shift Warning

For split-event providers, some canonical `before_tool_execute` matchers may map to a provider-native event that fires **after** the action rather than before it. For example, Cursor has no `beforeFileEdit` event — the closest match is `afterFileEdit`, which fires after the file is written.

This timing inversion is more severe than a capability gap: a blocking safety hook intended to prevent an action will instead observe it after the fact. Adapters MUST emit a prominent warning when a before-event hook is mapped to an after-event on the target provider. The warning MUST indicate that the hook's blocking intent cannot prevent the action, only observe it.

---

## 9. Capability Registry

Capabilities describe optional semantic features that a hook uses beyond the core event/handler/blocking model. Each capability has a support matrix, an inference rule, and a default degradation strategy.

Capabilities are **inferred by tooling** from manifest fields and handler output patterns. Hook authors do not need to declare capabilities manually. The optional `capabilities` array on a hook definition is informational and MAY be used by tooling for display purposes.

### 9.1 `structured_output`

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
| opencode | N/A (in-process) | Programmatic model |

**Default degradation:** `warn`. The hook executes; JSON output fields that cannot be mapped are silently dropped with a warning.

### 9.2 `input_rewrite`

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
| opencode | Mutable `output.args` in plugin |
| All others | Not supported |

**Default degradation:** `block`. When the target provider does not support input rewriting, the adapter MUST generate a hook that blocks the action entirely (exit code 2) rather than allowing unmodified input through. This prevents a false sense of security. Hook authors MAY override to `warn` or `exclude` via the `degradation` field when the rewrite is cosmetic rather than safety-critical.

### 9.3 `llm_evaluated`

Hook logic is evaluated by an LLM rather than a deterministic script.

**Inference rule:** Tooling infers this capability when `handler.type` is `"prompt"` or `"agent"`.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| claude-code | `type: "prompt"` (single-turn) and `type: "agent"` (multi-turn with tools) |
| kiro | "Ask Kiro" agent prompt actions (IDE only, consumes credits) |
| All others | Not supported |

**Default degradation:** `exclude`. LLM evaluation IS the hook -- wrapping it in a CLI shim is fragile and requires the source provider's infrastructure. Hook authors MAY override to `warn` if partial functionality is acceptable.

### 9.4 `http_handler`

Hook executes via HTTP POST to an endpoint rather than a local process.

**Inference rule:** Tooling infers this capability when `handler.type` is `"http"`.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| claude-code | `type: "http"` with `url`, `headers`, `allowedEnvVars` |
| All others | Not supported |

**Default degradation:** `warn`. An adapter MAY approximate HTTP handlers by generating a shell script that invokes `curl` or an equivalent HTTP client. The generated script SHOULD preserve the URL, method, headers, and body contract.

### 9.5 `async_execution`

Hook runs without blocking the agent (fire-and-forget).

**Inference rule:** Tooling infers this capability when `handler.async` is `true`.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| claude-code | `async: true` |
| All others | Hooks execute synchronously |

**Default degradation:** `warn`. The hook executes synchronously. A warning is emitted that the hook may block the agent.

### 9.6 `platform_commands`

Per-operating-system command overrides.

**Inference rule:** Tooling infers this capability when the `handler.platform` field is present.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| vs-code-copilot | `windows`, `linux`, `osx` fields with `command` fallback (note: native format uses `osx`; the canonical format uses `darwin`) |
| copilot-cli | `bash`, `powershell` fields |
| All others | Single `command` field only |

**Default degradation:** `warn`. The default `command` is used. Platform-specific overrides are dropped with a warning.

### 9.7 `custom_env`

Custom environment variables passed to the hook process.

**Inference rule:** Tooling infers this capability when the `handler.env` field is present.

**Support matrix:**

| Provider | Mechanism |
|----------|-----------|
| vs-code-copilot | `env` field (key-value object) |
| All others | Not supported |

**Default degradation:** `warn`. Environment variables are dropped with a warning.

### 9.8 `configurable_cwd`

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

## 10. Tool Vocabulary

The tool vocabulary defines canonical tool names that abstract over provider-specific naming. Bare string matchers (Section 6.1) resolve against this vocabulary.

### 10.1 Canonical Tool Names

| Canonical Name | Description | claude-code | gemini-cli | cursor | windsurf | copilot-cli | kiro | opencode |
|----------------|-------------|-------------|------------|--------|----------|-------------|------|----------|
| `shell` | Shell command execution | Bash | run_shell_command | run_terminal_cmd | (event: pre_run_command) | bash | execute_bash | bash |
| `file_read` | Read file contents | Read | read_file | read_file | (event: pre_read_code) | view | fs_read | read |
| `file_write` | Create or overwrite file | Write | write_file | edit_file | (event: pre_write_code) | create | fs_write | write |
| `file_edit` | Modify existing file | Edit | replace | edit_file | (event: pre_write_code) | edit | fs_write | edit |
| `search` | Search file contents | Grep | grep_search | grep_search | -- | grep | grep | grep |
| `find` | Find files by pattern | Glob | glob | file_search | -- | glob | glob | glob |
| `web_search` | Search the web | WebSearch | google_web_search | web_search | -- | -- | web_search | -- |
| `web_fetch` | Fetch URL content | WebFetch | web_fetch | -- | -- | web_fetch | web_fetch | -- |
| `agent` | Spawn sub-agent | Agent | -- | -- | -- | task | use_subagent | -- |

A `--` indicates the provider does not have an equivalent tool.

For split-event providers (Cursor, Windsurf), certain tool vocabulary entries map to native events rather than tool name matchers. For example, encoding `matcher: "shell"` for Windsurf produces a hook bound to the `pre_run_command` event rather than a matcher on a tool name.

### 10.2 MCP Tool Names

MCP tools use structured objects in the canonical format. The provider-specific combined string formats and encoding rules are defined in Section 6.3. When decoding from a provider, adapters MUST parse the combined string format back into the structured `{"mcp": {"server": "...", "tool": "..."}}` representation.

---

## 11. Degradation Strategies

When converting a hook to a target provider that lacks support for a capability the hook uses, a **degradation strategy** determines the behavior.

### 11.1 Strategy Values

| Strategy | Behavior |
|----------|----------|
| `block` | The converted hook blocks the action entirely (exit code 2) instead of running with reduced functionality. Use for safety-critical capabilities where silent degradation would be dangerous. |
| `warn` | The converted hook runs with reduced functionality. The adapter emits a warning describing what was lost. Use for convenience capabilities where partial operation is acceptable. |
| `exclude` | The hook is excluded entirely from the target provider's configuration. The adapter emits a warning that the hook was omitted. Use for capabilities that are the entirety of the hook's purpose. |

### 11.2 Author-Specified Degradation

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

### 11.3 Safe Defaults

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

---

## 12. Conversion Pipeline

Converting a hook between providers follows a four-stage pipeline: decode, validate, encode, verify.

### 12.1 Decode

The source adapter reads a provider-native hook configuration and produces a canonical hook manifest.

During decode, the adapter MUST:
1. Translate provider-native event names to canonical names (Section 7.4).
2. Translate provider-native tool names in matchers to canonical names (Section 10).
3. Convert timeout values to seconds (the canonical unit).
4. Preserve provider-specific fields with no canonical equivalent in `provider_data`.

For split-event providers: the adapter MUST merge category-specific events into unified canonical events with appropriate matchers. For example, Windsurf's `pre_run_command` becomes `before_tool_execute` with `matcher: "shell"`.

### 12.2 Validate

The implementation checks the canonical manifest against the target provider's capabilities.

During validation, the implementation MUST:
1. Infer capabilities from manifest fields (Section 9 inference rules).
2. For each inferred capability, check whether the target provider supports it.
3. Apply degradation strategies (author-specified or defaults from Section 11.3).
4. For each event, check whether the target provider supports it (Section 7.4).
5. Check blocking behavior: if the hook is blocking and the target maps to `observe`, emit a warning.
6. Verify the manifest contains at least one hook with a supported event on the target.

Validation MUST produce a list of warnings. Validation MUST NOT silently discard information.

### 12.3 Encode

The target adapter writes a provider-native configuration from the canonical manifest.

During encode, the adapter MUST:
1. Translate canonical event names to provider-native names.
2. Translate canonical tool names in matchers to provider-native names.
3. Convert timeout values from seconds to the provider's unit (e.g., milliseconds for Gemini CLI).
4. Render `provider_data` for the target provider if present.
5. For split-event providers: map unified `before_tool_execute` + matcher into the correct category-specific event.
6. Map `platform` overrides to provider-specific fields (e.g., `platform.windows` to Copilot CLI's `powershell` field).

### 12.4 Verify

After encoding, the implementation SHOULD re-decode the output with the target adapter to verify structural fidelity.

During verification:
1. Parse the encoded output using the target adapter's decode function.
2. Compare the decoded result against the canonical manifest: event count, handler types, command strings, matcher presence.
3. On verification failure, report an encoding error. Implementations MUST NOT silently produce corrupt output.

---

## 13. Conformance Levels

This specification defines three conformance levels. An implementation declares which level it targets.

### 13.1 Core

A Core-conformant implementation MUST:
- Parse and produce valid canonical hook manifests (Section 3).
- Support all core events (Section 7.1).
- Implement the exit code contract (Section 4).
- Support bare string and omitted matchers (Sections 6.1, 6.5).
- Translate event names between canonical and at least one provider format.
- Translate tool names between canonical and at least one provider format.
- Ignore unknown fields at all levels of the manifest.

### 13.2 Extended

An Extended-conformant implementation MUST satisfy all Core requirements and additionally:
- Support all matcher types: bare string, pattern, MCP, and array (Section 6).
- Support the canonical output schema (Section 5), including interaction with exit codes.
- Implement capability inference (Section 9) for at least `structured_output`, `input_rewrite`, and `platform_commands`.
- Implement degradation strategies (Section 11) with safe defaults.
- Support extended events (Section 7.2) when the target provider supports them.

### 13.3 Full

A Full-conformant implementation MUST satisfy all Extended requirements and additionally:
- Support `provider_data` round-tripping: data decoded from provider P, encoded back to provider P, MUST be structurally equivalent.
- Implement the verification stage (Section 12.4).
- Support all capabilities in the Capability Registry (Section 9).
- Support provider-exclusive events (Section 7.3) for lossless round-tripping.

---

## 14. Versioning

This specification manages three independently versioned artifacts.

### 14.1 Specification Version

The specification itself follows [Semantic Versioning 2.0](https://semver.org/). The version is declared in the `spec` field of hook manifests as `"hooks/<major>.<minor>"`.

- **Major** version increments indicate breaking changes (field removal, semantic changes to existing fields).
- **Minor** version increments indicate additive changes (new optional fields, clarifications).
- **Patch** version increments indicate editorial corrections (typos, examples).

The `spec` field in manifests includes only major and minor versions (e.g., `"hooks/0.1"`). The specification document itself uses semver with a patch component (e.g., `0.1.0`), but this patch version is not represented in manifests. Implementations MUST accept any manifest whose `spec` field matches the major and minor version they support.

### 14.2 Registries

The Event Registry (Section 7), Capability Registry (Section 9), and Tool Vocabulary (Section 10) are date-stamped and grow independently of the specification version. Adding a new event, capability, or tool name does not require a spec version increment.

Registry versions use the format `YYYY.MM` (e.g., `2026.03`).

### 14.3 Provider Support Matrix

The Blocking Behavior Matrix (Section 8) and per-capability support matrices are living documents that update whenever a provider ships changes to its hook system. These do not have a formal version; they are updated in place with a changelog.

---

## Appendix A: Provider Strengths

Each provider brings unique capabilities to the hook ecosystem. This appendix highlights what each provider does best, independent of how many canonical features it supports.

### Claude Code

The richest structured output contract. Hooks can return nuanced decisions (`allow`/`deny`/`ask`), rewrite tool inputs, inject system messages, suppress output, and add conversation context. Four handler types (command, HTTP, prompt, agent) cover deterministic, network, and LLM-evaluated hooks. The deepest event set (25+ events) provides fine-grained lifecycle control.

### Gemini CLI

Unique LLM-interaction hooks. `before_model` can intercept or mock LLM API calls before they reach the model. `after_model` enables real-time response redaction and PII filtering. `before_tool_selection` dynamically filters which tools the model can choose. No other provider offers this level of control over the model interaction layer.

### Kiro

File system lifecycle hooks. `file_created`, `file_saved`, and `file_deleted` events with glob pattern matching enable hooks that respond to file changes, not just tool invocations. Spec task hooks (`before_task`, `after_task`) integrate with Kiro's specification-driven development workflow.

### Cursor

Pre-read file access control. The `beforeReadFile` event lets hooks gate file reads, enabling data classification and access control patterns. The split-event model (`beforeShellExecution`, `beforeMCPExecution`, `beforeReadFile`) provides category-specific context without matcher overhead.

### Windsurf

Enterprise deployment infrastructure. Cloud dashboard hook management, MDM deployment (Jamf, Intune, Ansible), immutable system-level hooks, and a three-tier priority system (system > user > workspace) support organizational policy enforcement at scale. Transcript access hooks enable compliance auditing.

### OpenCode

Programmatic extensibility. TypeScript/JavaScript plugins with in-process execution, mutable argument objects, custom tool definitions, and npm package distribution. The highest event granularity (~30+ events) including LSP integration and TUI interaction hooks. Best suited for deep tool customization beyond what declarative hook configs can express.

### VS Code Copilot

Convergent implementation. Intentionally aligned with Claude Code's hook contract (same event names, same output schema, same `hookSpecificOutput` pattern). Adds OS-specific command overrides and custom environment variables. The `/create-hook` AI-assisted generation feature lowers the authoring barrier.

### Copilot CLI

Conservative safety design. Hook failures are explicitly non-blocking ("a broken hook script shouldn't take down your workflow"). Repository-bound hooks (`.github/hooks/`) ensure hooks travel with code. The `bash`/`powershell` split provides explicit cross-platform support without runtime detection.

---

## Appendix B: JSON Schema Reference

The canonical hook format is formally defined by a JSON Schema at [schema/hook.schema.json](schema/hook.schema.json).

The schema validates:
- Top-level structure (`spec`, `hooks`)
- Hook definition fields (`event`, `matcher`, `handler`, `blocking`, `degradation`, `provider_data`)
- Handler fields (`type`, `command`, `platform`, `cwd`, `env`, `timeout`, `async`)
- Matcher types (bare string, pattern object, MCP object, array)
- Output schema fields (`decision`, `reason`, `continue`, `context`)

Implementations MAY use this schema for validation but are not required to. The normative text in this specification takes precedence over the JSON Schema in case of conflict.
