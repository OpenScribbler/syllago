# Hook Behavior Validation: Gemini CLI + Kiro

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Gemini CLI | GitHub source: google-gemini/gemini-cli — packages/core/src/hooks/types.ts, hookRunner.ts, hookAggregator.ts, coreToolHookTriggers.ts, geminiChat.ts, hookEventHandler.ts |
| Kiro | kiro.dev/docs/hooks/, kiro.dev/docs/cli/hooks/, mikeartee/kiro-hooks-docs community schema |

---

## Gemini CLI

### Category A: Event Name Mapping Validation

| Check | Canonical | Spec Claim | Finding | Evidence |
|-------|-----------|------------|---------|----------|
| GEM-A1 | before_tool_execute | BeforeTool | CONFIRMED | `HookEventName.BeforeTool = 'BeforeTool'` in types.ts |
| GEM-A2 | after_tool_execute | AfterTool | CONFIRMED | `HookEventName.AfterTool = 'AfterTool'` |
| GEM-A3 | session_start | SessionStart | CONFIRMED | `HookEventName.SessionStart = 'SessionStart'` — source enum includes `startup`, `resume`, `clear` |
| GEM-A4 | session_end | SessionEnd | CONFIRMED | `HookEventName.SessionEnd = 'SessionEnd'` — reasons: `exit`, `clear`, `logout`, `prompt_input_exit`, `other` |
| GEM-A5 | before_prompt | BeforeAgent | CONFIRMED | `HookEventName.BeforeAgent = 'BeforeAgent'` |
| GEM-A6 | agent_stop | AfterAgent | CONFIRMED | `HookEventName.AfterAgent = 'AfterAgent'` — includes `stop_hook_active: boolean` |
| GEM-A7 | before_compact | PreCompress | CONFIRMED | `HookEventName.PreCompress = 'PreCompress'` — trigger: `manual` or `auto` |
| GEM-A8 | notification | Notification | CONFIRMED | `HookEventName.Notification = 'Notification'` — type includes `ToolPermission` |
| GEM-A9 | before_model | BeforeModel | CONFIRMED | `HookEventName.BeforeModel = 'BeforeModel'` — receives full `llm_request` object |
| GEM-A10 | after_model | AfterModel | CONFIRMED | `HookEventName.AfterModel = 'AfterModel'` — receives `llm_request` + `llm_response` |
| GEM-A11 | before_tool_selection | BeforeToolSelection | CONFIRMED | `HookEventName.BeforeToolSelection = 'BeforeToolSelection'` — receives `llm_request` |

No additional events found. The `HookEventName` enum is exhaustive with exactly 11 entries matching the spec.

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| GEM-B1 | before_tool_execute | prevent | CONFIRMED | Exit code semantics in hookRunner.ts: 0=allow, 1=allow+warning, >=2=deny. Note: ANY exit code >=2 blocks, not just exit code 2 specifically. |
| GEM-B2 | after_tool_execute | observe | CONFIRMED | AfterTool result used for additionalContext and tailToolCallRequest, not blocking |
| GEM-B3 | session_start | observe | CONFIRMED | Fires and continues; no blocking check |
| GEM-B4 | session_end | observe | CONFIRMED | Falls through to mergeSimple; session already ending |
| GEM-B5 | before_prompt | prevent | CONFIRMED | BeforeAgent uses mergeWithOrDecision; blocking decision prevents agent turn |
| GEM-B6 | agent_stop | retry | CONFIRMED | When AfterAgent blocks, agent sends continuation message with blocking reason as content. `stop_hook_active: true` set on subsequent AfterAgent fire. More precisely "trigger continuation turn" than "retry". |

### Category C: Capability Validation

#### C1: structured_output — CONFIRMED and EXPANDED

Gemini CLI `HookOutput` interface includes fields the spec omits:

| Field | In Spec? | Details |
|-------|----------|---------|
| decision | Yes | Values: 'ask', 'block', 'deny', 'approve', 'allow' |
| systemMessage | Yes | String injected into system prompt |
| hookSpecificOutput | Yes | Event-specific sub-fields (see below) |
| continue | **No** | Boolean — stop agent if false |
| stopReason | **No** | String reason for stopping |
| suppressOutput | **No** | Boolean — suppress tool output |
| reason | **No** | Human-readable decision explanation |

Event-specific hookSpecificOutput sub-fields:
- BeforeTool: `tool_input` (input rewriting)
- AfterTool: `additionalContext`, `tailToolCallRequest`
- BeforeAgent: `additionalContext`
- AfterAgent: `clearContext`
- SessionStart: `additionalContext`
- BeforeModel: `llm_request` (modify), `llm_response` (synthetic/mock)
- AfterModel: `llm_response` (modify/redact)
- BeforeToolSelection: `toolConfig` (filter tool list)

#### C2: input_rewrite — CORRECTED: Gemini CLI DOES support it

**Spec claim:** Gemini CLI does NOT support input rewriting.

**Finding:** CORRECTED. Gemini CLI supports input rewriting via `hookSpecificOutput.tool_input` on BeforeTool hooks.

**Evidence from source:**
```typescript
// types.ts — BeforeToolHookOutput class
getModifiedToolInput(): Record<string, unknown> | undefined {
  if (this.hookSpecificOutput && 'tool_input' in this.hookSpecificOutput) { ... }
}

// coreToolHookTriggers.ts
const modifiedInput = beforeOutput.getModifiedToolInput();
if (modifiedInput) {
  Object.assign(invocation.params, modifiedInput);
}
```

This is a significant spec error. The input_rewrite capability matrix (Section 9.2) needs to add Gemini CLI.

#### C3: llm_evaluated — CONFIRMED not supported

Two hook types exist: `HookType.Command` (shell) and `HookType.Runtime` (in-process JS). Neither invokes an LLM.

#### C4: BeforeModel/AfterModel/BeforeToolSelection capabilities — ALL CONFIRMED

**BeforeModel (geminiChat.ts):**
- Returning `hookSpecificOutput: { llm_response: ... }` creates a synthetic response, skipping the actual model call entirely. Can mock/intercept LLM requests.
- Returning `hookSpecificOutput: { llm_request: ... }` modifies the request before sending.
- `decision: 'block'` or `continue: false` stops the model call.

**BeforeToolSelection:**
- `hookSpecificOutput: { toolConfig: { mode: 'NONE' | 'ANY', allowedFunctionNames: [...] } }` modifies the tool list.
- Caveat from hookAggregator.ts: hooks can only add/enable tools, not filter individually. NONE mode is most restrictive and wins.

**AfterModel (geminiChat.ts):**
- `hookSpecificOutput: { llm_response: { candidates: [...] } }` replaces the response via getModifiedResponse(). Can redact/modify.

---

## Kiro

### Category A: Event Name Mapping Validation

**CRITICAL FINDING:** Kiro has dual systems (IDE and CLI) but both use **camelCase** event names. The spec's display-name mappings ("File Save", "Pre Task Execution") are incorrect — actual config keys are camelCase identifiers.

**VALIDATED (Round 1):** Validator confirmed CLI docs show camelCase (`agentSpawn`, `userPromptSubmit`, `preToolUse`, `postToolUse`, `stop`), matching IDE. Original claim of PascalCase CLI names was incorrect.

| Check | Canonical | Spec Claim | Finding | IDE Name | CLI Name |
|-------|-----------|------------|---------|----------|----------|
| KIRO-A1 | before_tool_execute | preToolUse | CONFIRMED | preToolUse | preToolUse |
| KIRO-A2 | after_tool_execute | postToolUse | CONFIRMED | postToolUse | postToolUse |
| KIRO-A3 | session_start | agentSpawn | CONFIRMED (CLI) / NOT_FOUND (IDE) | (no equivalent) | agentSpawn |
| KIRO-A4 | session_end | stop | **CORRECTED** | agentStop (per-turn) | stop (per-turn) |
| KIRO-A5 | before_prompt | userPromptSubmit | CORRECTED (IDE) / CONFIRMED (CLI) | promptSubmit | userPromptSubmit |
| KIRO-A6 | agent_stop | stop | PARTIALLY CONFIRMED | agentStop | stop |
| KIRO-A7 | file_changed | File Save | **CORRECTED** | fileEdited | (not documented) |
| KIRO-A8 | file_created | File Create | **CORRECTED** | fileCreated | (not documented) |
| KIRO-A9 | file_deleted | File Delete | **CORRECTED** | fileDeleted | (not documented) |
| KIRO-A10 | before_task | Pre Task Execution | **CORRECTED** | preTaskExecution | (not documented) |
| KIRO-A11 | after_task | Post Task Execution | **CORRECTED** | postTaskExecution | (not documented) |

#### KIRO-A4 Details: session_end mapping is wrong

`stop`/`agentStop` fires at end of each agent TURN, not at session termination. It is semantically equivalent to `agent_stop`, not `session_end`. Kiro has NO session-end event.

#### Additional Kiro event: userTriggered (Manual Trigger)

The IDE has a `userTriggered` event type for manually-invoked hooks. Not in spec.

### Category B: Blocking Behavior Validation

| Check | Event | Spec Claim | Finding | Details |
|-------|-------|------------|---------|---------|
| KIRO-B1 | before_tool_execute | prevent | CONFIRMED | CLI: exit code 2 specifically blocks. IDE: any non-zero blocks. |
| KIRO-B2 | after_tool_execute | observe | CONFIRMED | Cannot prevent tool operations |
| KIRO-B3 | session_start | observe | CONFIRMED | AgentSpawn: non-zero shows warning but allows continuation |
| KIRO-B4 | session_end | observe | CONFIRMED (but event semantics wrong — see A4) | |
| KIRO-B5 | before_prompt | observe | **CORRECTED: IDE=PREVENT, CLI=OBSERVE** | IDE blocks on non-zero exit. CLI shows warning only (does not block). |
| KIRO-B6 | agent_stop | observe | CONFIRMED | No retry mechanism; response already delivered |

#### KIRO-B5 Details: before_prompt blocking differs by system

**Spec claim:** `before_prompt` (Kiro) = observe

**VALIDATED (Round 1):** Validator found the original research incorrectly generalized IDE behavior to CLI.

Evidence from kiro.dev/docs/hooks/actions/ (IDE):
> "Non-zero exit code: the user prompt submission is blocked." → **PREVENT**

Evidence from kiro.dev/docs/cli/hooks/ (CLI):
> For `userPromptSubmit`, non-zero exit: "Show STDERR warning to user." Only `preToolUse` supports exit code 2 blocking. → **OBSERVE**

The spec is wrong for the IDE (should be PREVENT) but correct for the CLI (is OBSERVE).

#### KIRO-B1 Note: Blocking mechanism differs between IDE and CLI

- **CLI**: Exit code 2 specifically blocks. Exit code 1 is a warning.
- **IDE**: Any non-zero exit code blocks for preToolUse and promptSubmit.

This internal inconsistency within Kiro should be noted in the spec.

### Category C: Capability Validation

| Check | Capability | Spec Claim | Finding |
|-------|-----------|------------|---------|
| KIRO-C1 | structured_output | "Undocumented" | **CORRECTED** — CLI uses exit codes + stderr/stdout (documented). No JSON output schema. |
| KIRO-C2 | input_rewrite | Not supported | CONFIRMED — no input modification field |
| KIRO-C3 | llm_evaluated | "Ask Kiro" (IDE only, credits) | CONFIRMED — `then.type: "askAgent"` action, consumes credits |
| KIRO-C4 | File lifecycle events + globs | Exist with glob patterns | CONFIRMED — `"patterns": ["**/*.ts"]` in when clause |
| KIRO-C5 | Spec task hooks | Tied to Kiro Specs | CONFIRMED — fire on spec task status transitions (in_progress, completed) |

---

## Summary: High-Priority Corrections

1. **Gemini CLI DOES support input rewriting** via `hookSpecificOutput.tool_input` on BeforeTool. Spec Section 9.2 capability matrix must add Gemini CLI.

2. **Kiro IDE before_prompt is PREVENT, CLI is OBSERVE** — IDE blocks prompt submission on non-zero exit. CLI only shows a warning. Spec Section 8.2 blocking matrix should show PREVENT for IDE, OBSERVE for CLI. **VALIDATED: original claim that both block was overcorrected; CLI is observe-only.**

3. **Kiro IDE and CLI both use camelCase** — both systems use camelCase event identifiers (`preToolUse`, `agentSpawn`, `stop`). **VALIDATED: original claim of PascalCase CLI names was incorrect.**

4. **Kiro has no session_end event** — `stop`/`agentStop` fires per-turn, not at session termination. Mapping `session_end` → `stop` is semantically incorrect.

5. **Kiro event config keys are camelCase identifiers**, not display names — `fileEdited` not "File Save", `fileCreated` not "File Create", `preTaskExecution` not "Pre Task Execution", etc.

6. **Gemini CLI HookOutput has additional fields** spec omits: `continue`, `stopReason`, `suppressOutput`, `reason`.

7. **Kiro CLI structured output IS documented** (exit codes + stderr/stdout), not "Undocumented".
