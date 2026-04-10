# Blocking Behavior Matrix

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines the expected blocking behavior per event-provider combination
> when a blocking hook returns exit code 2 or `decision: "deny"`.

**Last Modified:** 2026-04-08
**Status:** Initial Development

When a blocking hook returns exit code 2 (or `decision: "deny"`), the resulting behavior depends on both the event and the target provider. This matrix defines the expected behavior per combination.

## §1 Behavior Vocabulary

| Behavior | Meaning |
|----------|---------|
| **prevent** | The triggering action is stopped. The tool does not execute, the prompt is not submitted, etc. |
| **retry** | The agent is re-engaged to attempt an alternative approach. |
| **observe** | The hook runs but cannot affect the outcome. Blocking signals are ignored. |

## §2 Matrix

| Event | claude-code | gemini-cli | cursor | windsurf | vs-code-copilot | copilot-cli | kiro | opencode | factory-droid | codex | cline |
|-------|-------------|------------|--------|----------|-----------------|-------------|------|----------|---------------|-------|-------|
| `before_tool_execute` | prevent | prevent | prevent | prevent | prevent | prevent | prevent | prevent | prevent | prevent | prevent |
| `after_tool_execute` | observe | prevent | observe | observe | observe | observe | observe | observe | observe | observe | observe |
| `session_start` | observe | observe | prevent† | -- | observe | observe | observe | observe | observe | observe | prevent |
| `session_end` | observe | observe | observe | -- | -- | observe | observe | -- | observe | -- | observe |
| `before_prompt` | prevent | prevent | prevent | prevent | prevent | observe | observe | -- | prevent | prevent | prevent |
| `agent_stop` | retry | retry | observe | observe | retry | observe | observe | observe | retry | retry | observe |
| `permission_request` | prevent | -- | -- | -- | -- | -- | -- | prevent | -- | -- | -- |

A `--` indicates the provider does not support the event (same as the Event Name Mapping table).

**Footnotes:**

- gemini-cli `after_tool_execute`: `decision: "deny"` hides the tool result from the agent's context (turn continues). This is not true action prevention since the tool has already run, but semantically closer to block than observe — the agent cannot act on a result it cannot see.
- opencode `before_tool_execute`: blocking works via thrown JavaScript exceptions, not exit codes or structured output fields. The `prevent` label is functionally correct but the mechanism differs from all other providers.
- cline `session_start` (TaskStart/TaskResume): `cancel: true` blocks task initiation or resumption.
- †cursor `session_start` (sessionStart): `{"continue": false}` is supposed to prevent session creation but is silently ignored as of at least v2.4.21. The event is classified as `prevent` per spec but blocking is currently broken in Cursor. See also: events.md §4 sessionStart footnote.

When encoding a hook with `"blocking": true` for a provider where the behavior is `observe`, adapters MUST emit a warning indicating that the blocking intent cannot be honored on the target provider. If the hook defines a `degradation` strategy for the relevant capability, that strategy applies; otherwise the hook is encoded with the blocking field preserved and the warning emitted.

## §3 Timing Shift Warning

For split-event providers, some canonical `before_tool_execute` matchers may map to a provider-native event that fires **after** the action rather than before it. For example, Cursor has no `beforeFileEdit` event — the closest match is `afterFileEdit`, which fires after the file is written.

This timing inversion is more severe than a capability gap: a blocking safety hook intended to prevent an action will instead observe it after the fact. Adapters MUST emit a prominent warning when a before-event hook is mapped to an after-event on the target provider. The warning MUST indicate that the hook's blocking intent cannot prevent the action, only observe it.

## Provider Event Support

The following table is auto-generated from `docs/provider-capabilities/*.yaml`. Do not edit by hand — run `syllago capmon generate` to refresh.

<!-- GENERATED FROM provider-capabilities/*.yaml -->
| Canonical Event | amp | claude-code | cline | codex | copilot-cli | crush | cursor | factory-droid | gemini-cli | kiro | opencode | pi | roo-code | windsurf | zed |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

<!-- END GENERATED -->
