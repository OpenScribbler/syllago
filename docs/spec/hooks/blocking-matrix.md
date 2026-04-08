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

## §3 Timing Shift Warning

For split-event providers, some canonical `before_tool_execute` matchers may map to a provider-native event that fires **after** the action rather than before it. For example, Cursor has no `beforeFileEdit` event — the closest match is `afterFileEdit`, which fires after the file is written.

This timing inversion is more severe than a capability gap: a blocking safety hook intended to prevent an action will instead observe it after the fact. Adapters MUST emit a prominent warning when a before-event hook is mapped to an after-event on the target provider. The warning MUST indicate that the hook's blocking intent cannot prevent the action, only observe it.
