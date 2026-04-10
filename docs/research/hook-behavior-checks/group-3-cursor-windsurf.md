# Hook Behavior Validation: Cursor + Windsurf

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Cursor | cursor.com/docs/hooks (fetched 2026-03-31), blog.gitbutler.com/cursor-hooks-deep-dive, johnlindquist/cursor-hooks TypeScript types, Cursor Community Forum |
| Windsurf | docs.windsurf.com/windsurf/cascade/hooks (fetched 2026-03-31) |

---

## Cursor

### Category A: Event Name Mapping Validation

| Check | Canonical | Spec Claim | Finding | Actual |
|-------|-----------|------------|---------|--------|
| CU-A1 | before_tool_execute | beforeShellExecution / beforeMCPExecution / beforeReadFile | CONFIRMED (+ preToolUse generic) | All three split events exist, plus newer generic `preToolUse` |
| CU-A2 | after_tool_execute | afterFileEdit (only) | **CORRECTED** | 4 events: afterShellExecution, afterMCPExecution, afterFileEdit, postToolUse |
| CU-A3 | session_start | -- (not supported) | **CORRECTED** | `sessionStart` exists (fixed in v2.4 after validation bug) |
| CU-A3b | session_end | -- (not supported) | **CORRECTED** | `sessionEnd` exists |
| CU-A4 | before_prompt | beforeSubmitPrompt | CONFIRMED | beforeSubmitPrompt |
| CU-A5 | agent_stop | stop | CONFIRMED | stop — input: `{ status, loop_count }`, output: `{ followup_message }` |
| CU-A6 | before_model | beforeAgentResponse | **CORRECTED** | `beforeAgentResponse` does NOT exist. Only `afterAgentResponse` exists. |
| CU-A7 | after_model | afterAgentResponse | CONFIRMED | afterAgentResponse — observational only |
| CU-A8 | before_tool_selection | beforeToolSelection | **NOT_FOUND** | Does not exist in Cursor docs or TypeScript types |
| CU-A9 | tool_use_failure | postToolUseFailure | CONFIRMED | postToolUseFailure — includes `failure_type` enum |
| CU-A10 | subagent_start | subagentStart | CONFIRMED | Newer event (post-January 2026) |
| CU-A10b | subagent_stop | subagentStop | CONFIRMED | Newer event |

#### CU-A1 Details: preToolUse is a newer addition

Cursor now has a generic `preToolUse` that fires before ANY tool use. This is important because:
- It supports `updated_input` for input rewriting (the split events do NOT)
- It supports `permission` output (allow/deny/ask)
- It coexists with the category-specific split events

The spec should map both the split events AND preToolUse.

#### CU-A2 Details: 4 after-tool events, not 1

The spec claims `afterFileEdit` is the only after-tool event. Actually:
- `afterShellExecution` — fires after shell command completes
- `afterMCPExecution` — fires after MCP tool completes  
- `afterFileEdit` — fires after file is edited
- `postToolUse` — generic, fires after any tool (supports `updated_mcp_tool_output`, `additional_context`)

#### CU-A6 Details: No beforeAgentResponse

The `before_model` → `beforeAgentResponse` mapping is broken. Only the post-response `afterAgentResponse` exists. If the spec maps `before_model` to Cursor, it is incorrect. Cursor also has `afterAgentThought` (not in spec).

#### Additional Cursor event: afterAgentThought

Not in spec. Fires after agent completes a thought/reasoning step.

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| CU-B1 | before_tool_execute | prevent | CONFIRMED | Both exit code 2 AND JSON `permission: "deny"` work |
| CU-B2 | after_tool_execute | observe | PARTIALLY CORRECTED | afterFileEdit/afterShellExecution/afterMCPExecution are observe-only, but postToolUse supports `additional_context` output |
| CU-B3 | before_prompt | observe | **CORRECTED: PREVENT** | `beforeSubmitPrompt` supports `continue: false` to block submission |
| CU-B4 | Timing shift (no beforeFileEdit) | Warning needed | CONFIRMED | No `beforeFileEdit` event exists — safety hooks for file writes must use `afterFileEdit` (timing inversion) |

#### CU-B1 Details: Blocking JSON format correction

The format convergence research cited `{"action": "block", "reason": "..."}` as Cursor's blocking format. This is WRONG. The correct format is:

```json
{"continue": false, "permission": "deny", "userMessage": "reason"}
```

Exit code 2 also works as a blocking mechanism.

#### CU-B3 Details: beforeSubmitPrompt CAN block

Official docs show `beforeSubmitPrompt` output: `{ continue: boolean, user_message?: string }`. Setting `continue: false` blocks the prompt. The spec's `observe` classification is wrong.

### Category C: Capability Validation

| Check | Capability | Spec Claim | Finding |
|-------|-----------|------------|---------|
| CU-C1 | structured_output | permission, userMessage, agentMessage | CONFIRMED — note both snake_case (`user_message`) and camelCase (`userMessage`) accepted |
| CU-C2 | input_rewrite | Not supported | **CORRECTED** — `preToolUse` supports `updated_input` field |
| CU-C3 | Split-event behavior | Separate independent events | CONFIRMED — each event is a separate key in hooks.json |
| CU-C4 | beforeReadFile read-gating | Unique capability | CONFIRMED — receives file `content` before it reaches LLM, can allow/deny |

#### CU-C2 Details: Cursor DOES support input rewriting

`preToolUse` (the generic pre-tool event) supports `updated_input` in its output. A Community Forum bug report (Feb 2026) confirms developers use it, though it has a known bug where it's silently ignored for the Task tool.

The category-specific events (`beforeShellExecution`, `beforeMCPExecution`) do NOT support `updated_input`. Only the generic `preToolUse` does.

---

## Windsurf

### Category A: Event Name Mapping Validation

| Check | Canonical | Spec Claim | Finding | Actual |
|-------|-----------|------------|---------|--------|
| WS-A1 | before_tool_execute | pre_read_code / pre_write_code / pre_run_command / pre_mcp_tool_use | CONFIRMED | All 4 pre-tool events confirmed |
| WS-A2 | after_tool_execute | post_read_code / post_write_code / post_run_command / post_mcp_tool_use | CONFIRMED | All 4 post-tool events confirmed |
| WS-A3 | session_start | -- | CONFIRMED | Not supported |
| WS-A3b | session_end | -- | CONFIRMED | Not supported |
| WS-A4 | before_prompt | pre_user_prompt | CONFIRMED | pre_user_prompt |
| WS-A5 | agent_stop | post_cascade_response | CONFIRMED | Fires after each agent response turn (imprecise: more "end of turn" than "agent stop") |

#### Complete 12-event list (from official docs)

1. pre_read_code
2. pre_write_code
3. pre_run_command
4. pre_mcp_tool_use
5. post_read_code
6. post_write_code
7. post_run_command
8. post_mcp_tool_use
9. pre_user_prompt
10. post_cascade_response
11. post_cascade_response_with_transcript
12. post_setup_worktree

Events 11-12 are NOT in the spec.

#### WS: post_cascade_response_with_transcript — NOT IN SPEC

Fires after agent response with full conversation transcript. Input: `{ transcript_path }` — JSONL file at `~/.windsurf/transcripts/{trajectory_id}.jsonl`. Files written with `0600` permissions; max 100 retained. Enterprise audit/compliance use case.

#### WS: post_setup_worktree — NOT IN SPEC

Fires after new git worktree created. Input: `{ worktree_path, root_workspace_path }`. Sets `$ROOT_WORKSPACE_PATH` env var.

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| WS-B1 | Blocking mechanism | JSON responses (from convergence research) | **CORRECTED** | Exit code 2 blocks, NOT JSON. Windsurf has NO structured JSON output format for hooks. |
| WS-B2 | before_tool_execute | prevent | CONFIRMED | All 5 pre-hooks block via exit code 2 |
| WS-B3 | after_tool_execute | observe | CONFIRMED | Post-hooks cannot block |
| WS-B4 | before_prompt | observe | **CORRECTED: PREVENT** | `pre_user_prompt` is listed as a blockable pre-hook (exit code 2) |
| WS-B5 | agent_stop | observe | CONFIRMED | post_cascade_response is async, cannot affect behavior |

#### WS-B1 Details: Windsurf uses exit code 2, NOT JSON

The format convergence research claiming Windsurf uses JSON responses for blocking is incorrect. Official docs are unambiguous:
> "For pre-hooks, your script can block the action by exiting with exit code 2."

Windsurf has no documented structured JSON output format at all. Communication is via exit codes and stderr only.

#### WS-B4 Details: pre_user_prompt CAN block

Official docs: "Only pre-hooks (pre_user_prompt, pre_read_code, pre_write_code, pre_run_command, pre_mcp_tool_use) can block actions using exit code 2."

### Category C: Capability Validation

| Check | Capability | Spec Claim | Finding |
|-------|-----------|------------|---------|
| WS-C1 | structured_output | Not supported | CONFIRMED — exit codes + stderr only |
| WS-C2 | input_rewrite | Not supported | CONFIRMED — binary block/allow, no modification |
| WS-C3 | Split-event behavior | Separate independent events | CONFIRMED |
| WS-C4 | Enterprise features | MDM, cloud dashboard, three-tier priority | CONFIRMED — actually FOUR tiers (Cloud Dashboard + System + User + Workspace) |
| WS-C5 | trajectory_id / execution_id | In hook payloads | CONFIRMED — common input fields for all events |

#### WS-C4 Details: Four tiers, not three

The spec says "three-tier" but official docs describe four levels:
1. **Cloud Dashboard** (Enterprise plan, `TEAM_SETTINGS_UPDATE` permission)
2. **System** (/Library/Application Support/Windsurf/hooks.json, /etc/windsurf/hooks.json)
3. **User** (~/.codeium/windsurf/hooks.json)
4. **Workspace** (.windsurf/hooks.json)

All tiers merge sequentially. System-level hooks bypass user modification without root access.

---

## Summary: High-Priority Corrections

1. **Cursor has sessionStart and sessionEnd** — spec says not supported. Fixed in Cursor v2.4.

2. **Cursor afterFileEdit is NOT the only after-tool event** — afterShellExecution, afterMCPExecution, postToolUse all exist.

3. **No beforeAgentResponse in Cursor** — before_model mapping is broken. Only afterAgentResponse exists.

4. **No beforeToolSelection in Cursor** — spec claim unsubstantiated.

5. **Cursor supports input_rewrite** via `preToolUse.updated_input` — spec says not supported.

6. **Both Cursor and Windsurf before_prompt CAN block** — spec says observe for both.

7. **Windsurf blocks via exit code 2, NOT JSON** — format convergence research was wrong.

8. **Windsurf has 4 config tiers**, not 3 (Cloud Dashboard is separate from System).

9. **Windsurf has 2 events not in spec** — post_cascade_response_with_transcript, post_setup_worktree.

10. **Cursor blocking JSON format** is `{permission: "deny"}`, NOT `{action: "block"}`.
