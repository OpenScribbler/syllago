# Hooks Spec — Conformance Test Vectors

This directory contains conformance test vectors for the hooks canonical format (spec version `hooks/0.1`). Each vector is a JSON file that defines either an expected input/output pair for a conversion operation, or a case that a compliant parser MUST reject.

Conformance test vectors are provider-neutral. They document the contract between the canonical format and each supported provider's native format, including how capabilities degrade when a provider lacks support for a feature.

---

## Format Contract

### Canonical vectors (`canonical/`)

Canonical vectors are valid hook manifests in the canonical format. Every canonical vector MUST parse without error.

Top-level fields:

| Field | Required | Description |
|-------|----------|-------------|
| `spec` | yes | Format version string. Currently `"hooks/0.1"`. |
| `hooks` | yes | Non-empty array of hook definitions. |
| `_comment` | no | Human-readable description of what the vector tests. Ignored by parsers. |

Each hook definition in the `hooks` array:

| Field | Required | Description |
|-------|----------|-------------|
| `event` | yes | Canonical event name (snake_case). E.g. `before_tool_execute`, `session_start`. |
| `handler` | yes | Handler object with `type`, `command`, and optional `timeout`, `platform`, `cwd`, `env`. |
| `matcher` | no | Tool filter. String (single tool), array of strings (OR semantics), or object (`{"mcp": {"server": "...", "tool": "..."}}`) for MCP tools. Omitting `matcher` on a tool event means wildcard (all tools). |
| `blocking` | no | Boolean. Whether the hook can block the action via exit code 2. Defaults to `false`. |
| `capabilities` | no | Array of capability strings the hook requires (e.g. `input_rewrite`, `structured_output`). |
| `degradation` | no | Object mapping capability names to strategies (`"block"`, `"warn"`, `"exclude"`). Governs converter behavior when the target provider lacks the capability. |
| `provider_data` | no | Object keyed by provider slug. Passthrough data rendered only when converting to that provider. |

### Provider vectors (`claude-code/`, `gemini-cli/`, `cursor/`, `windsurf/`)

Provider vectors are the expected output of converting the paired canonical vector to that provider's native format. They are in the provider's native JSON structure, not canonical.

Provider vectors may include these metadata fields (ignored by the converter itself):

| Field | Description |
|-------|-------------|
| `_comment` | What this vector tests and what conversions are applied. |
| `_warnings` | Array of strings describing capability degradations, dropped fields, or semantic approximations that a conforming converter MUST emit. |

### Invalid vectors (`invalid/`)

Invalid vectors are canonical-shaped documents that a conforming parser MUST reject. Each file's `_comment` states the specific violation.

---

## Pairing Rules

Canonical and provider vectors are paired by filename. The file `canonical/X.json` is the input; `<provider>/X.json` is the expected output after converting to that provider.

```
canonical/simple-blocking.json   <-- input
claude-code/simple-blocking.json <-- expected output for Claude Code
gemini-cli/simple-blocking.json  <-- expected output for Gemini CLI
cursor/simple-blocking.json      <-- expected output for Cursor
windsurf/simple-blocking.json    <-- expected output for Windsurf
```

A provider vector with no canonical pair is a standalone test. The two round-trip vectors in `claude-code/` follow a different pairing rule: `roundtrip-source.json` is the native input, and `roundtrip-canonical.json` is the expected canonical form after decoding it. Re-encoding that canonical form back to Claude Code MUST produce output structurally equivalent to `roundtrip-source.json`.

Not every canonical vector has a provider output for every provider. The index table below records which provider outputs exist.

---

## Directory Structure

```
test-vectors/
  canonical/              Canonical format inputs. All are valid hook manifests.
  claude-code/            Expected outputs for Claude Code (unified-event, .claude/settings.json)
  gemini-cli/             Expected outputs for Gemini CLI (flat array, .gemini/settings.json)
  cursor/                 Expected outputs for Cursor (split-event, .cursor/hooks/ JSON)
  windsurf/               Expected outputs for Windsurf (split-event, .windsurf/settings.json)
  invalid/                Documents that a conforming parser MUST reject.
```

Provider classifications:

- **Unified-event** (Claude Code): hooks are grouped under a single event key; the matcher filters within the event.
- **Flat-array** (Gemini CLI): hooks are a flat JSON array; each entry has a `trigger` field for the event and a `toolMatcher` field.
- **Split-event** (Cursor, Windsurf): the event name encodes both timing (before/after) and tool category. No separate matcher field is needed for the tool — it is implied by the event name. Array matchers in canonical form must be expanded into one entry per matched tool/event.

---

## Index

### Canonical vectors

| File | What it tests |
|------|---------------|
| `canonical/simple-blocking.json` | Minimal blocking hook: `before_tool_execute` + shell matcher + command handler. Core layer only — no capabilities, degradation, or provider_data. |
| `canonical/full-featured.json` | Most canonical fields at once: blocking hook with platform overrides, cwd, env, degradation, provider_data passthrough, MCP matcher, and a `session_start` lifecycle hook. |
| `canonical/multi-event.json` | Diverse events and matcher styles: array matcher (OR semantics), wildcard matcher (omitted), `before_prompt`, `agent_stop`. |
| `canonical/degradation-input-rewrite.json` | Safety-critical `input_rewrite` degradation. Hook sanitizes shell arguments via `updated_input`. Providers that lack `input_rewrite` MUST block unconditionally rather than pass unmodified input through. |

### Provider vectors

| Canonical input | Provider | File | Key conversions and notes |
|-----------------|----------|------|--------------------------|
| `simple-blocking.json` | Claude Code | `claude-code/simple-blocking.json` | `before_tool_execute` -> `PreToolUse`; `shell` -> `Bash`; timeout stays in seconds. |
| `simple-blocking.json` | Gemini CLI | `gemini-cli/simple-blocking.json` | `before_tool_execute` -> `BeforeTool`; `shell` -> `run_shell_command`; timeout seconds -> milliseconds. |
| `simple-blocking.json` | Cursor | `cursor/simple-blocking.json` | Split-event: `before_tool_execute + shell` -> `beforeShellExecution`; no matcher field needed; timeout stays. |
| `simple-blocking.json` | Windsurf | `windsurf/simple-blocking.json` | Split-event: `before_tool_execute + shell` -> `pre_run_command`; timeout dropped; blocking implicit via exit code. |
| `full-featured.json` | Claude Code | `claude-code/full-featured.json` | MCP matcher -> `mcp__github__create_issue` (double underscore); `session_start` -> `SessionStart`; platform/cwd/env dropped with warn. |
| `full-featured.json` | Gemini CLI | `gemini-cli/full-featured.json` | MCP matcher -> `mcp_github_create_issue` (single underscore); `session_start` -> `SessionStart`; platform/cwd/env dropped. |
| `full-featured.json` | Cursor | `cursor/full-featured.json` | MCP matcher -> `beforeMCPExecution` with `matcher: "github__create_issue"`; `session_start` dropped (unsupported). |
| `full-featured.json` | Windsurf | `windsurf/full-featured.json` | MCP matcher -> `pre_mcp_tool_use` (granularity lost — fires for all MCP tools); `provider_data.windsurf` rendered; `session_start` dropped. |
| `multi-event.json` | Claude Code | `claude-code/multi-event.json` | Array matcher `['shell','file_write']` -> regex alternation `Bash\|Write`; `before_prompt` -> `UserPromptSubmit`; `agent_stop` -> `Stop`. |
| `multi-event.json` | Gemini CLI | `gemini-cli/multi-event.json` | Array matcher expands to two separate entries (Gemini has no regex alternation); `before_prompt` -> `BeforeAgent`; `agent_stop` -> `AfterAgent`. |
| `multi-event.json` | Cursor | `cursor/multi-event.json` | Array matcher splits into `beforeShellExecution` + `afterFileEdit` (timing shift warning: Cursor has no `beforeFileEdit`); `before_prompt` -> `beforeSubmitPrompt`. |
| `multi-event.json` | Windsurf | `windsurf/multi-event.json` | Array matcher splits into `pre_run_command` + `pre_write_code`; wildcard `after_tool_execute` expands to all four post-events; `agent_stop` -> `post_cascade_response` (semantic approximation). |
| `degradation-input-rewrite.json` | Gemini CLI | `gemini-cli/degradation-input-rewrite.json` | `input_rewrite` unsupported; strategy `block` applied; converted hook unconditionally exits 2 with explanation message. |
| `degradation-input-rewrite.json` | Cursor | `cursor/degradation-input-rewrite.json` | `input_rewrite` unsupported; strategy `block` applied; `beforeShellExecution` wraps original command name in block message. |
| `degradation-input-rewrite.json` | Windsurf | `windsurf/degradation-input-rewrite.json` | `input_rewrite` unsupported; strategy `block` applied; `pre_run_command` unconditionally exits 2. Original command NOT executed. |

### Round-trip vectors (Claude Code)

| File | Role |
|------|------|
| `claude-code/roundtrip-source.json` | Native Claude Code format (starting point). Decode this to canonical. |
| `claude-code/roundtrip-canonical.json` | Expected canonical form after decoding `roundtrip-source.json`. Re-encoding this MUST reproduce `roundtrip-source.json`. |

### Invalid vectors

| File | Violation |
|------|-----------|
| `invalid/missing-spec.json` | Missing required `spec` field. |
| `invalid/missing-hooks.json` | Missing required `hooks` field. |
| `invalid/empty-hooks-array.json` | `hooks` array is empty; spec requires non-empty. |
| `invalid/missing-event.json` | Hook entry missing required `event` field. |
| `invalid/missing-handler.json` | Hook entry missing required `handler` field. |
| `invalid/invalid-degradation-strategy.json` | `degradation` strategy value `"ignore"` is not valid; must be `"block"`, `"warn"`, or `"exclude"`. |

---

## How to Use

### Validation tests (invalid/)

For each file in `invalid/`, your parser MUST return an error. The `_comment` field states the expected violation. A conforming parser MUST NOT accept any of these inputs.

### Conversion tests (canonical/ + provider/)

For each paired canonical/provider file:

1. Parse the canonical input file.
2. Run your converter targeting the named provider.
3. Compare the converter output to the provider vector.

Structural equivalence is required: field names, values, and nesting must match. Key ordering within objects does not matter. The `_comment` and `_warnings` fields in the provider vector are metadata — do not include them in the comparison.

If the provider vector contains a `_warnings` field, your converter MUST emit a warning for each listed item. The exact warning text need not match, but the semantic content must (which capability was dropped, which strategy was applied).

### Round-trip tests (claude-code/roundtrip-*)

1. Parse `roundtrip-source.json` as native Claude Code format.
2. Decode to canonical. Compare against `roundtrip-canonical.json`.
3. Re-encode the canonical form back to Claude Code. Compare against `roundtrip-source.json`.

Both comparisons must pass for the round-trip to be considered lossless.

### Degradation tests

Degradation vectors (`degradation-input-rewrite.json` in each provider directory) verify safety-critical behavior. A converter that silently drops a hook requiring `input_rewrite` and allows the action to proceed is non-conforming. The correct behavior is to replace the hook with one that unconditionally blocks (exits 2) and emits a message explaining why.
