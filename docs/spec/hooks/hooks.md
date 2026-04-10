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
7. [Conversion Pipeline](#7-conversion-pipeline)
8. [Conformance Levels](#8-conformance-levels)
9. [Versioning](#9-versioning)
- [Appendix A: JSON Schema Reference](#appendix-a-json-schema-reference)

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
- A registry of canonical event names with provider mappings (see [Event Registry](events.md))
- A capability model describing optional features with provider support matrices (see [Capability Registry](capabilities.md))
- A tool vocabulary mapping canonical tool names to provider-native names (see [Tool Vocabulary](tools.md))
- A [blocking behavior matrix](blocking-matrix.md) per event/provider combination
- Conformance levels for implementations

### 1.2 What This Spec Does Not Define

- How providers implement their native hook systems
- Hook distribution, packaging, or registry publishing mechanisms
- Cryptographic signing, provenance, or trust policies
- Enterprise policy enforcement (see [policy-interface.md](policy-interface.md) for the interface contract)
- Hook authoring workflows or IDE integrations

### 1.3 Document Set

This specification is organized as a set of focused documents. Readers who need only the
format definition can read §1-6 of this document. Implementers building a converter need
the [Event Registry](events.md) and [Tool Vocabulary](tools.md). The
[Blocking Behavior Matrix](blocking-matrix.md) and [Capability Registry](capabilities.md)
are used during the validation stage of the conversion pipeline.

See the [directory README](README.md) for the full document index.

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
| `event` | string | REQUIRED | Canonical event name from the [Event Registry](events.md). |
| `matcher` | string, object, or array | OPTIONAL | Tool matcher expression (§6). When omitted, the hook matches all tools for the bound event. |
| `handler` | object | REQUIRED | Handler definition (§3.5). |
| `blocking` | boolean | OPTIONAL | Whether this hook can prevent the triggering action. Default: `false`. |
| `degradation` | object | OPTIONAL | Per-capability fallback strategies (see [Capability Registry](capabilities.md)). Keys are capability identifiers, values are strategy strings. |
| `provider_data` | object | OPTIONAL | Opaque provider-specific data, keyed by provider slug. |
| `capabilities` | array | OPTIONAL | Informational list of capability identifier strings. Implementations MUST NOT make conformance or behavioral decisions based on this field. Implementations MUST infer capabilities from manifest fields and handler output patterns. Capability inference rules are defined in the [Capability Registry](capabilities.md). |

### 3.5 Handler Definition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | REQUIRED | Handler type. MUST be `"command"` for shell-based handlers. Other types are defined as capabilities in the [Capability Registry](capabilities.md). |
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
| 0 | Success | Action proceeds. Stdout is parsed as JSON if present (§5). Empty stdout is treated as `{}`. |
| 1 | Hook Error | Non-blocking warning. Action proceeds. Stderr SHOULD be logged. |
| 2 | Block | Action is prevented. Stderr SHOULD be presented to the user or agent as the blocking reason. Only meaningful when `blocking` is `true` for the hook; when `blocking` is `false`, exit code 2 MUST be treated as exit code 1 (non-blocking warning). |
| Other | Hook Error | Same behavior as exit code 1. |

Implementations MUST NOT treat exit code 1 or other non-zero codes (besides 2) as blocking, regardless of the `blocking` field.

When a hook exceeds its timeout, behavior is determined by the `timeout_action` field on
the handler definition (§3.5). The `blocking: false` downgrade applies before
`timeout_action` evaluation — when `blocking` is `false`, timeout always degrades to a
warning regardless of `timeout_action`.

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
2. **Exit code / decision resolution.** After the downgrade, apply the precedence rules from §5.2.

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

---

## 6. Matcher Types

The `matcher` field on a hook definition controls which tools trigger the hook for tool-related events (e.g., `before_tool_execute`, `after_tool_execute`). For non-tool events (e.g., `session_start`), the `matcher` field is not applicable and MUST be ignored by implementations.

### 6.1 Bare String

A bare string value is a **tool vocabulary lookup**. It MUST match against the canonical tool names defined in the [Tool Vocabulary](tools.md). Implementations resolve the canonical name to the provider-native tool name during encoding.

When a bare string does not match any canonical tool name in the Tool Vocabulary, implementations MUST pass it through as a literal string and SHOULD emit a warning. This ensures forward compatibility when new tool names are added to the vocabulary: manifests using the new name will work with older implementations that do not yet recognize it.

```json
"matcher": "shell"
```

### 6.2 Pattern Object

An object with a `pattern` key specifies a regular expression matched against the provider-native tool name. The regex flavor MUST be RE2 for cross-language compatibility. RE2 guarantees linear-time matching and is available in Go, Rust, Python, JavaScript (via libraries), Java, and other languages. Implementations MUST NOT accept patterns that require features beyond RE2 (e.g., backreferences, lookahead).

```json
"matcher": {"pattern": "file_(read|write|edit)"}
```

Pattern matchers are **not portable** across providers. The pattern is passed through to the target provider and matched at runtime against that provider's native tool names. Hook authors who need cross-provider portability SHOULD use bare string matchers (§6.1), which are translated through the Tool Vocabulary. Pattern matchers are an escape hatch for provider-specific tool names that the vocabulary does not cover.

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

## Registries

The following documents define the registries and matrices referenced by this specification.
Consult them when implementing adapters or verifying provider support.

| Document | Contents |
|----------|----------|
| [events.md](events.md) | Canonical event names, provider-native mappings, event name mapping table |
| [tools.md](tools.md) | Canonical tool names and provider-native equivalents |
| [capabilities.md](capabilities.md) | Optional capability features, support matrices, and degradation strategies |
| [blocking-matrix.md](blocking-matrix.md) | Blocking behavior per event and provider |
| [provider-strengths.md](provider-strengths.md) | Non-normative provider highlights |

---

## 7. Conversion Pipeline

Converting a hook between providers follows a four-stage pipeline: decode, validate, encode, verify.

### 7.1 Decode

The source adapter reads a provider-native hook configuration and produces a canonical hook manifest.

During decode, the adapter MUST:
1. Translate provider-native event names to canonical names (see [Event Registry](events.md)).
2. Translate provider-native tool names in matchers to canonical names (see [Tool Vocabulary](tools.md)).
3. Convert timeout values to seconds (the canonical unit).
4. Preserve provider-specific fields with no canonical equivalent in `provider_data`.

For split-event providers: the adapter MUST merge category-specific events into unified canonical events with appropriate matchers. For example, Windsurf's `pre_run_command` becomes `before_tool_execute` with `matcher: "shell"`.

### 7.2 Validate

The implementation checks the canonical manifest against the target provider's capabilities.

During validation, the implementation MUST:
1. Infer capabilities from manifest fields (see [Capability Registry](capabilities.md) for inference rules).
2. For each inferred capability, check whether the target provider supports it.
3. Apply degradation strategies (author-specified or defaults from the [Capability Registry](capabilities.md)).
4. For each event, check whether the target provider supports it (see [Event Registry](events.md)).
5. Check blocking behavior: if the hook is blocking and the target maps to `observe`, emit a warning (see [Blocking Behavior Matrix](blocking-matrix.md)).
6. Verify the manifest contains at least one hook with a supported event on the target.

Validation MUST produce a list of warnings. Validation MUST NOT silently discard information.

### 7.3 Encode

The target adapter writes a provider-native configuration from the canonical manifest.

During encode, the adapter MUST:
1. Translate canonical event names to provider-native names.
2. Translate canonical tool names in matchers to provider-native names.
3. Convert timeout values from seconds to the provider's unit (e.g., milliseconds for Gemini CLI).
4. Render `provider_data` for the target provider if present.
5. For split-event providers: map unified `before_tool_execute` + matcher into the correct category-specific event.
6. Map `platform` overrides to provider-specific fields (e.g., `platform.windows` to Copilot CLI's `powershell` field).

### 7.4 Verify

After encoding, the implementation SHOULD re-decode the output with the target adapter to verify structural fidelity.

During verification:
1. Parse the encoded output using the target adapter's decode function.
2. Compare the decoded result against the canonical manifest: event count, handler types, command strings, matcher presence.
3. On verification failure, report an encoding error. Implementations MUST NOT silently produce corrupt output.

---

## 8. Conformance Levels

This specification defines three conformance levels. An implementation declares which level it targets.

### 8.1 Core

A Core-conformant implementation MUST:
- Parse and produce valid canonical hook manifests (§3).
- Support all core events (see [Event Registry](events.md) §1).
- Implement the exit code contract (§4).
- Support bare string and omitted matchers (§6.1, §6.5).
- Translate event names between canonical and at least one provider format.
- Translate tool names between canonical and at least one provider format.
- Ignore unknown fields at all levels of the manifest.

#### Core Conformance Checklist

**Core event requirements.** A Core-conformant implementation MUST support these six events
by name: `before_tool_execute`, `after_tool_execute`, `session_start`, `session_end`,
`before_prompt`, `agent_stop`. Full event names and provider mappings are in
[events.md](events.md).

**Core tool vocabulary requirements.** A Core-conformant implementation MUST support bare
string matchers for these canonical tool names: `shell`, `file_read`, `file_write`,
`file_edit`, `search`, `find`, `web_search`, `web_fetch`, `agent`. Full vocabulary with
provider mappings is in [tools.md](tools.md).

**Core capability requirements.** Core conformance does not require capability support.
Extended conformance requires at minimum: `structured_output`, `input_rewrite`,
`platform_commands`. Full capability definitions are in [capabilities.md](capabilities.md).

### 8.2 Extended

An Extended-conformant implementation MUST satisfy all Core requirements and additionally:
- Support all matcher types: bare string, pattern, MCP, and array (§6).
- Support the canonical output schema (§5), including interaction with exit codes.
- Implement capability inference (see [Capability Registry](capabilities.md)) for at least `structured_output`, `input_rewrite`, and `platform_commands`.
- Implement degradation strategies with safe defaults (see [Capability Registry](capabilities.md)).
- Support extended events (see [Event Registry](events.md) §2) when the target provider supports them.

**Extended capabilities** (implementations MUST infer at minimum):
`structured_output`, `input_rewrite`, `platform_commands`

### 8.3 Full

A Full-conformant implementation MUST satisfy all Extended requirements and additionally:
- Support `provider_data` round-tripping: data decoded from provider P, encoded back to provider P, MUST be structurally equivalent.
- Implement the verification stage (§7.4).
- Support all capabilities in the [Capability Registry](capabilities.md).
- Support provider-exclusive events (see [Event Registry](events.md) §3) for lossless round-tripping.

---

## 9. Versioning

This specification manages three independently versioned artifacts.

### 9.1 Specification Version

The specification itself follows [Semantic Versioning 2.0](https://semver.org/). The version is declared in the `spec` field of hook manifests as `"hooks/<major>.<minor>"`.

- **Major** version increments indicate breaking changes (field removal, semantic changes to existing fields).
- **Minor** version increments indicate additive changes (new optional fields, clarifications).
- **Patch** version increments indicate editorial corrections (typos, examples).

The `spec` field in manifests includes only major and minor versions (e.g., `"hooks/0.1"`). The specification document itself uses semver with a patch component (e.g., `0.1.0`), but this patch version is not represented in manifests. Implementations MUST accept any manifest whose `spec` field matches the major and minor version they support.

### 9.2 Registries

The [Event Registry](events.md), [Capability Registry](capabilities.md), and [Tool Vocabulary](tools.md) are date-stamped and grow independently of the specification version. Adding a new event, capability, or tool name does not require a spec version increment.

Registry versions use the format `YYYY.MM` (e.g., `2026.03`).

### 9.3 Provider Support Matrix

The [Blocking Behavior Matrix](blocking-matrix.md) and per-capability support matrices are living documents that update whenever a provider ships changes to its hook system. These do not have a formal version; they are updated in place with a changelog.

---

## Appendix A: JSON Schema Reference

The canonical hook format is formally defined by a JSON Schema at [schema/hook.schema.json](schema/hook.schema.json).

The schema validates:
- Top-level structure (`spec`, `hooks`)
- Hook definition fields (`event`, `matcher`, `handler`, `blocking`, `degradation`, `provider_data`)
- Handler fields (`type`, `command`, `platform`, `cwd`, `env`, `timeout`, `async`)
- Matcher types (bare string, pattern object, MCP object, array)
- Output schema fields (`decision`, `reason`, `continue`, `context`)

Implementations MAY use this schema for validation but are not required to. The normative text in this specification takes precedence over the JSON Schema in case of conflict.
