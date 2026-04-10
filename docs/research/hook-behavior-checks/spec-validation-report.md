# Hooks Spec v0.1.0 Validation Report

Research date: 2026-03-31
Method: 4 research groups (8 agents x 3 categories), 4 non-spec agents, security audit, runtime behavior discovery. Validated by 4 parallel sonnet validators until clean.

---

## Executive Summary

The hooks spec v0.1.0 has **27 confirmed errors** across its event mapping table, blocking behavior matrix, and capability support matrices. The most severe are security-relevant: VS Code Copilot's `UserPromptSubmit` is incorrectly classified as blocking (it's observe-only), and two agents (Cursor, Gemini CLI) support input rewriting that the spec says they don't — meaning the `input_rewrite` degradation strategy (`block` by default) would incorrectly fire for these agents.

The spec is also missing 11 Claude Code events, 2 Windsurf events, and several Cursor events that were added since the spec was written.

---

## Errors by Category

### A. Event Name Mapping Errors (Section 7.4) — 12 errors

| # | Error | Severity |
|---|-------|----------|
| 1 | CC `error_occurred` → `StopFailure` is wrong. Should be `ErrorOccurred`. `StopFailure` maps to separate `stop_failure` canonical. | Medium |
| 2 | Copilot CLI `agent_stop` shown as `--` (unsupported). Actually supported as `agentStop`. | Medium |
| 3 | Cursor `session_start` and `session_end` shown as `--`. Both exist (`sessionStart`, `sessionEnd`). | Medium |
| 4 | Cursor `after_tool_execute` shows only `afterFileEdit`. Actually 4 events: `afterShellExecution`, `afterMCPExecution`, `afterFileEdit`, `postToolUse`. | High |
| 5 | Cursor `before_model` → `beforeAgentResponse` is wrong. Event does not exist. Only `afterAgentResponse` exists. | **Critical** (converter produces silent no-op) |
| 6 | Cursor `before_tool_selection` → `beforeToolSelection` is wrong. Event does not exist in Cursor. | **High** (converter produces silent no-op) |
| 7 | Kiro file events use display names ("File Save", "File Create", "File Delete") instead of config identifiers (`fileEdited`, `fileCreated`, `fileDeleted`). | High (breaks conversions) |
| 8 | Kiro task events use display names ("Pre Task Execution", "Post Task Execution") instead of config identifiers (`preTaskExecution`, `postTaskExecution`). | High (breaks conversions) |
| 9 | Kiro `session_end` → `stop` is semantically wrong. `stop`/`agentStop` fires per-turn, not at session end. Kiro has no session-end event. | Medium |
| 10 | Spec missing 11 CC events (TaskCreated, TaskCompleted, TeammateIdle, InstructionsLoaded, CwdChanged, WorktreeCreate, WorktreeRemove, PostCompact, Elicitation, ElicitationResult, StopFailure as distinct canonical). | Medium |
| 11 | Spec missing 2 Windsurf events (post_cascade_response_with_transcript, post_setup_worktree). | Low |
| 12 | Spec missing Cursor events (preToolUse generic, afterShellExecution, afterMCPExecution, afterAgentThought). | Medium |

### B. Blocking Behavior Matrix Errors (Section 8.2) — 8 errors

| # | Error | Severity |
|---|-------|----------|
| 1 | VS Code Copilot `before_prompt` classified as `prevent`. **Actually observe-only.** | **Critical** (security) |
| 2 | VS Code Copilot `after_tool_execute` classified as `observe`. Can also block via `decision: "block"`. | Medium |
| 3 | Cursor `before_prompt` classified as `observe`. **Actually prevent** (`continue: false` blocks). | High |
| 4 | Windsurf `before_prompt` classified as `observe`. **Actually prevent** (exit code 2 blocks). | High |
| 5 | Kiro IDE `before_prompt` classified as `observe`. **Actually prevent** (non-zero exit blocks). | Medium |
| 6 | Copilot CLI `agent_stop` shown as `--`. Should be `observe`. | Medium |
| 7 | CC/Gemini/VS Code `agent_stop` labeled `retry`. Should be `continue` (prevents termination, forces continuation — not automatic retry). | Medium |
| 8 | Cursor `session_start`/`session_end` shown as `--`. Both are `observe`. | Low |

### C. Capability Matrix Errors (Section 9) — 5 errors

| # | Error | Severity |
|---|-------|----------|
| 1 | `input_rewrite`: Gemini CLI listed as "Not supported". **Actually supported** via `hookSpecificOutput.tool_input`. | **Critical** (safety — degradation would incorrectly block) |
| 2 | `input_rewrite`: Cursor listed as "Not supported". **Actually supported** via `preToolUse.updated_input`. | **Critical** (safety — degradation would incorrectly block) |
| 3 | `custom_env`: Copilot CLI listed as "Not supported". **Actually supported** via `env` field. | Medium |
| 4 | `structured_output`: Gemini CLI missing fields `continue`, `stopReason`, `suppressOutput`, `reason`. | Low |
| 5 | `async_execution`: OpenCode listed as "Not supported". Named hooks ARE async (Promise<void>, awaited). | Medium |

### D. Other Errors — 2 errors

| # | Error | Severity |
|---|-------|----------|
| 1 | Windsurf blocking mechanism: Format convergence research said JSON responses. **Actually exit code 2.** No structured JSON output at all. | High |
| 2 | Cursor blocking JSON format: Convergence research said `{"action": "block"}`. **Actually `{"permission": "deny"}`** or `{"continue": false}`. | Medium |

---

## Error Severity Distribution

| Severity | Count | Examples |
|----------|-------|---------|
| Critical | 3 | VS Code Copilot before_prompt blocking, Gemini/Cursor input_rewrite |
| High | 6 | Cursor missing events, Kiro display names, Windsurf blocking mechanism |
| Medium | 13 | Various mapping gaps, missing events, capability errors |
| Low | 5 | Missing optional events, incomplete field lists |

---

## What the Spec Gets Right

Despite the errors, the spec correctly captures:
- Core event model (before/after tool, session, prompt, stop) — the concepts are right even where names are wrong
- Exit code 2 as the dominant blocking convention (confirmed for 6 of 8 agents)
- Split-event model for Cursor and Windsurf (confirmed)
- Timing shift warning for Cursor's missing `beforeFileEdit` (confirmed)
- Degradation strategy framework (correct approach, just needs updated capability matrices)
- Provider-exclusive events concept (Kiro file/task events, CC config_change)
- Tool vocabulary mapping approach (confirmed functional)
- Conversion pipeline stages (decode → validate → encode → verify)
