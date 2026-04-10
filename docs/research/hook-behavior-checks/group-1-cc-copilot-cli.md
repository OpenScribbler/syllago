# Hook Behavior Validation: Claude Code + GitHub Copilot CLI

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Claude Code | https://code.claude.com/docs/en/hooks (fetched 2026-03-31), cli/internal/converter/toolmap.go, cli/internal/converter/adapter_copilot.go |
| Copilot CLI | https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks (fetched 2026-03-31) |

---

## Claude Code

### Category A: Event Name Mapping Validation

| Check | Canonical | Spec Claim | Finding | Actual |
|-------|-----------|------------|---------|--------|
| CC-A1 | before_tool_execute | PreToolUse | CONFIRMED | PreToolUse |
| CC-A2 | after_tool_execute | PostToolUse | CONFIRMED | PostToolUse |
| CC-A3 | session_start | SessionStart | CONFIRMED | SessionStart |
| CC-A4 | session_end | SessionEnd | CONFIRMED | SessionEnd |
| CC-A5 | before_prompt | UserPromptSubmit | CONFIRMED | UserPromptSubmit |
| CC-A6 | agent_stop | Stop | CONFIRMED | Stop |
| CC-A7 | before_compact | PreCompact | CONFIRMED | PreCompact |
| CC-A8 | notification | Notification | CONFIRMED | Notification |
| CC-A9 | error_occurred | StopFailure | **CORRECTED** | ErrorOccurred (StopFailure is a separate event mapped to `stop_failure`) |
| CC-A10 | tool_use_failure | PostToolUseFailure | CONFIRMED | PostToolUseFailure |
| CC-A11 | file_changed | FileChanged | CONFIRMED | FileChanged |
| CC-A12 | subagent_start | SubagentStart | CONFIRMED | SubagentStart |
| CC-A13 | subagent_stop | SubagentStop | CONFIRMED | SubagentStop |
| CC-A14 | permission_request | PermissionRequest | CONFIRMED | PermissionRequest |
| CC-A15 | config_change | ConfigChange | CONFIRMED | ConfigChange |

#### CC-A9 Details: error_occurred mapping

The spec maps `error_occurred` to `StopFailure` for Claude Code. This is incorrect. In `toolmap.go`:
- `error_occurred` maps to `ErrorOccurred` (CC native)
- `stop_failure` maps to `StopFailure` (CC native, separate canonical event)

The spec conflates two distinct CC events into one canonical entry. `StopFailure` fires when a turn ends due to an API error; `ErrorOccurred` is a broader error event.

**VALIDATED (Round 1):** Validator confirmed the toolmap.go mapping is correct but noted that `ErrorOccurred` does NOT appear in CC's official docs (only `StopFailure` is listed). `ErrorOccurred` may be an undocumented or internal-only event. The codebase maps it but it may not be a real user-facing CC event. Needs further investigation before the spec documents it.

#### CC-A16: Additional Events NOT in Spec

The spec lists 15 CC events in Section 7.4. The actual CC event list has at least 25+. Missing from spec:

| CC Native | Canonical (from toolmap.go) |
|-----------|---------------------------|
| TaskCreated | task_created |
| TaskCompleted | task_completed |
| TeammateIdle | teammate_idle |
| InstructionsLoaded | instructions_loaded |
| CwdChanged | (not in toolmap) |
| WorktreeCreate | worktree_create |
| WorktreeRemove | worktree_remove |
| PostCompact | after_compact |
| Elicitation | elicitation |
| ElicitationResult | elicitation_result |
| StopFailure | stop_failure (distinct from error_occurred) |

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| CC-B1 | before_tool_execute | prevent | CONFIRMED | Exit code 2 or `permissionDecision: "deny"` blocks |
| CC-B2 | after_tool_execute | observe | CONFIRMED | PostToolUse runs after execution; can provide feedback but cannot prevent the tool from running |
| CC-B3 | session_start | observe | CONFIRMED | Non-blocking |
| CC-B4 | session_end | observe | CONFIRMED | Non-blocking |
| CC-B5 | before_prompt | prevent | CONFIRMED | `decision: "block"` blocks prompt processing |
| CC-B6 | agent_stop | retry | **CORRECTED** | "Force continue", not "retry". `Stop` hook with `decision: "block"` prevents Claude from stopping and forces continuation. No automatic retry mechanism — the blocking merely keeps the agent loop running. "retry" label is misleading. |

### Category C: Capability Validation

| Check | Capability | Spec Claim | Finding |
|-------|-----------|------------|---------|
| CC-C1 | structured_output | hookSpecificOutput with rich fields | CONFIRMED — event-specific sub-fields including permissionDecision, updatedInput, additionalContext |
| CC-C2 | input_rewrite | hookSpecificOutput.updatedInput | CONFIRMED — only PreToolUse and PermissionRequest hooks can rewrite input |
| CC-C3 | llm_evaluated | type "prompt" + "agent" | CONFIRMED — four handler types: command, http, prompt, agent |
| CC-C4 | http_handler | type "http" with url/headers/allowedEnvVars | CONFIRMED |
| CC-C5 | async_execution | async: true | CONFIRMED |
| CC-C6 | platform_commands | Not supported | CONFIRMED — CC has `shell` selector (bash/powershell) but no platform object |
| CC-C7 | custom_env | Not supported | CONFIRMED — system-injected vars only ($CLAUDE_PROJECT_DIR, etc.), no user-configurable env |
| CC-C8 | configurable_cwd | Not supported | CONFIRMED — no cwd field in hook config |

---

## GitHub Copilot CLI

### Category A: Event Name Mapping Validation

| Check | Canonical | Spec Claim | Finding | Actual |
|-------|-----------|------------|---------|--------|
| COP-A1 | before_tool_execute | preToolUse | CONFIRMED | preToolUse |
| COP-A2 | after_tool_execute | postToolUse | CONFIRMED | postToolUse |
| COP-A3 | session_start | sessionStart | CONFIRMED | sessionStart |
| COP-A4 | session_end | sessionEnd | CONFIRMED | sessionEnd |
| COP-A5 | before_prompt | userPromptSubmitted | CONFIRMED | userPromptSubmitted |
| COP-A6 | error_occurred | errorOccurred | CONFIRMED | errorOccurred |
| COP-A7 | tool_use_failure | errorOccurred | CONFIRMED | errorOccurred (collision — both map to same event) |
| COP-A8 | agent_stop | -- (not supported) | **CORRECTED** | agentStop IS supported. Both toolmap.go and official docs confirm it. |

#### COP-A8 Details: agent_stop IS supported

The spec shows `--` (unsupported) for `agent_stop` on Copilot CLI. This is wrong:
- `toolmap.go`: `"agent_stop": {..., "copilot-cli": "agentStop"}`
- `adapter_copilot.go` Capabilities() lists `"agent_stop"` in supported events
- Official docs list `agentStop` as a supported event

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| COP-B1 | before_tool_execute | prevent | CONFIRMED | preToolUse can approve or deny tool executions |
| COP-B2 | after_tool_execute | observe | CONFIRMED | No blocking mechanism for post-tool |
| COP-B3 | session_start | observe | CONFIRMED | No blocking documentation |
| COP-B4 | session_end | observe | CONFIRMED | No blocking documentation |
| COP-B5 | before_prompt | observe | NOT_FOUND | Cannot confirm or deny — docs don't specify blocking semantics for userPromptSubmitted |
| COP-B6 | agent_stop | -- (not supported) | **CORRECTED** | Should show `observe` (event exists but likely non-blocking) |

### Category C: Capability Validation

| Check | Capability | Spec Claim | Finding |
|-------|-----------|------------|---------|
| COP-C1 | structured_output | permissionDecision only | NOT_FOUND — docs don't document output fields; claim unverifiable |
| COP-C2 | input_rewrite | Not supported | CONFIRMED |
| COP-C3 | llm_evaluated | Not supported | CONFIRMED — command type only |
| COP-C4 | http_handler | Not supported | CONFIRMED |
| COP-C5 | async_execution | Not supported | CONFIRMED |
| COP-C6 | platform_commands | bash/powershell fields | CONFIRMED |
| COP-C7 | custom_env | Not supported | **CORRECTED** — Copilot CLI DOES support `env` field (key-value object). Official docs document it; adapter sets `SupportsEnv: true`. |
| COP-C8 | configurable_cwd | cwd field | CONFIRMED |

---

## Summary: High-Priority Corrections

1. **CC error_occurred → ErrorOccurred, not StopFailure** — StopFailure is a separate canonical event (`stop_failure`). Spec conflates two distinct CC events.

2. **Copilot CLI DOES support agentStop** — spec shows `--` in both mapping table and blocking matrix. Wrong in both places.

3. **CC agent_stop "retry" is misleading** — actual behavior is "force continue" (prevent stop), not automatic retry.

4. **Copilot CLI DOES support custom_env** — spec says only VS Code Copilot supports it. Wrong.

5. **11 CC events missing from spec** — TaskCreated, TaskCompleted, TeammateIdle, InstructionsLoaded, WorktreeCreate, WorktreeRemove, PostCompact, Elicitation, ElicitationResult, CwdChanged, and StopFailure as distinct canonical.
