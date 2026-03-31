# Hooks Empirical Testing Framework — Implementation Plan

**Design doc:** `docs/plans/2026-03-31-hooks-empirical-testing-design.md`
**Research sources:** `docs/research/hook-behavior-checks/`
**Created:** 2026-03-31

---

## Overview

Three artifacts:

1. `docs/checks/hooks-behavior-checklist.md` — 28 checks with per-agent expected results
2. `content/hooks/benchmark/hb-XX-*/` — 22 executable benchmark hooks (one per automatable check)
3. `docs/plans/2026-03-31-hooks-dogfood-plan.md` — dogfood execution playbook
4. `docs/checks/results/` — per-agent result collection templates

Tasks are grouped by artifact. Checklist tasks come first (by category), then benchmark hooks, then dogfood plan, then result templates.

---

## Task Index

| # | Task | Artifact | Est. |
|---|------|----------|------|
| T-01 | Scaffold checklist file + header | Checklist | 2 min |
| T-02 | Write Category 1: Event Binding (6 checks, HB-01–06) | Checklist | 5 min |
| T-03 | Write Category 2: Blocking Behavior (4 checks, HB-07–10) | Checklist | 4 min |
| T-04 | Write Category 3: Capability Support (5 checks, HB-11–15) | Checklist | 5 min |
| T-05 | Write Category 4: Runtime Discovery (2 checks, HB-16–17) | Checklist | 3 min |
| T-06 | Write Category 5: Runtime Execution (4 checks, HB-18–21) | Checklist | 4 min |
| T-07 | Write Category 6: Security Posture (3 checks, HB-22–24) | Checklist | 3 min |
| T-08 | Write Category 7: Input Rewrite Safety (1 check, HB-25) | Checklist | 2 min |
| T-09 | Write Category 8: Prompt Blocking (2 checks, HB-26–27) | Checklist | 3 min |
| T-10 | Write Category 9: Non-Spec Agent Coverage (1 check, HB-28) | Checklist | 2 min |
| T-11 | Write benchmark hooks HB-01–06 (event binding) | Benchmark | 5 min |
| T-12 | Write benchmark hooks HB-07–10 (blocking behavior) | Benchmark | 4 min |
| T-13 | Write benchmark hooks HB-11–15 (capability support) | Benchmark | 5 min |
| T-14 | Write benchmark hooks HB-16–21 (runtime: discovery + execution) | Benchmark | 5 min |
| T-15 | Write benchmark hooks HB-25–27 (input rewrite + prompt blocking) | Benchmark | 3 min |
| T-16 | Write dogfood plan | Dogfood | 5 min |
| T-17 | Write result collection templates | Results | 3 min |

---

## T-01: Scaffold checklist file + header

**Depends on:** nothing
**Creates:** `docs/checks/hooks-behavior-checklist.md`

```markdown
# Hooks Behavior Checklist

**Version:** 0.1.0
**Research date:** 2026-03-31
**Source research:** `docs/research/hook-behavior-checks/`
**Benchmark hooks:** `content/hooks/benchmark/`

This checklist documents expected hook behavior across 12 AI coding agents. Each check has an ID, category, description, rationale, automatable flag, and per-agent expected result table.

Checks are derived from empirical source code research (see research files for sources and validation methodology). Expected results reflect what the code *says* it does — the benchmark hooks in `content/hooks/benchmark/` verify what it *actually* does.

## Agent Coverage

| # | Agent | Tier | Automatable |
|---|-------|------|-------------|
| 1 | claude-code | Paid | Yes |
| 2 | gemini-cli | Free | Yes |
| 3 | cursor | Paid | Yes |
| 4 | windsurf | Free tier | Yes |
| 5 | vscode-copilot | Paid | Yes |
| 6 | copilot-cli | Paid | Yes |
| 7 | kiro-ide | Free | Partial (IDE required) |
| 8 | kiro-cli | Free | Yes |
| 9 | opencode | Free | Yes |
| 10 | amazon-q | Free | Yes |
| 11 | cline | Free | Yes |
| 12 | augment-code | Free tier | Yes |

Agents marked Partial require an IDE to be open. Manual checks are indicated per-check.

## Check Categories

| Category | Checks | IDs |
|----------|--------|-----|
| 1. Event Binding | 6 | HB-01–06 |
| 2. Blocking Behavior | 4 | HB-07–10 |
| 3. Capability Support | 5 | HB-11–15 |
| 4. Runtime Discovery | 2 | HB-16–17 |
| 5. Runtime Execution | 4 | HB-18–21 |
| 6. Security Posture | 3 | HB-22–24 |
| 7. Input Rewrite Safety | 1 | HB-25 |
| 8. Prompt Blocking | 2 | HB-26–27 |
| 9. Non-Spec Agent Coverage | 1 | HB-28 |

## Result Legend

| Symbol | Meaning |
|--------|---------|
| PASS | Behavior confirmed working |
| FAIL | Behavior confirmed broken or absent |
| SKIP | Agent does not support this event/capability |
| PARTIAL | Works but with caveats (noted in check) |
| UNVERIFIED | Not yet tested empirically |
| MANUAL | Requires manual testing (not automatable) |
```

**Verification:** File exists at `docs/checks/hooks-behavior-checklist.md` with the above content.

---

## T-02: Write Category 1 — Event Binding (HB-01–06)

**Depends on:** T-01
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append the following after the Result Legend table:

```markdown
---

## Category 1: Event Binding

Event binding checks verify that syllago's canonical event names map to the correct native event names in each agent. These are the most fundamental checks — a wrong event name means the hook silently binds to nothing.

---

### HB-01: before_tool_execute event binding

**Category:** Event Binding
**What it checks:** The agent fires a native event when a tool is about to execute, and syllago's canonical `before_tool_execute` maps to the correct native name.
**Why it matters:** This is the most-used hook event. If the mapping is wrong, blocking hooks silently bind to nothing and safety checks are bypassed.
**Automatable:** Yes

| Agent | Expected | Native Event(s) |
|-------|----------|----------------|
| claude-code | PASS | `PreToolUse` |
| gemini-cli | PASS | `BeforeTool` |
| cursor | PASS | `preToolUse` / `beforeShellExecution` / `beforeMCPExecution` / `beforeReadFile` |
| windsurf | PASS | `pre_read_code` / `pre_write_code` / `pre_run_command` / `pre_mcp_tool_use` |
| vscode-copilot | PASS | `PreToolUse` |
| copilot-cli | PASS | `preToolUse` |
| kiro-ide | PASS | `preToolUse` |
| kiro-cli | PASS | `preToolUse` |
| opencode | PASS | `tool.execute.before` |
| amazon-q | PASS | `preToolUse` |
| cline | PASS | `PreToolUse` |
| augment-code | PASS | `PreToolUse` |

---

### HB-02: after_tool_execute event binding

**Category:** Event Binding
**What it checks:** The agent fires a native event after tool execution completes, and syllago's canonical `after_tool_execute` maps to the correct native name(s).
**Why it matters:** Post-execution hooks enable audit logging, output sanitization, and prompt injection defenses. Missing mappings mean these hooks never fire.
**Automatable:** Yes

| Agent | Expected | Native Event(s) |
|-------|----------|----------------|
| claude-code | PASS | `PostToolUse` |
| gemini-cli | PASS | `AfterTool` |
| cursor | PASS | `postToolUse` / `afterShellExecution` / `afterMCPExecution` / `afterFileEdit` |
| windsurf | PASS | `post_read_code` / `post_write_code` / `post_run_command` / `post_mcp_tool_use` |
| vscode-copilot | PASS | `PostToolUse` |
| copilot-cli | PASS | `postToolUse` |
| kiro-ide | PASS | `postToolUse` |
| kiro-cli | PASS | `postToolUse` |
| opencode | PASS | `tool.execute.after` |
| amazon-q | PASS | `postToolUse` |
| cline | PASS | `PostToolUse` |
| augment-code | PASS | `PostToolUse` |

**Note for cursor:** The spec originally listed only `afterFileEdit`. The full set is 4 events. A converted `after_tool_execute` hook should bind to all four (or the generic `postToolUse`).

---

### HB-03: session_start event binding

**Category:** Event Binding
**What it checks:** The agent fires a native event when a session begins, and syllago's canonical `session_start` maps to the correct native name.
**Why it matters:** Session-start hooks inject context into the agent's initial prompt window. Wrong mappings mean context never loads.
**Automatable:** Yes (trigger: open a new session)

| Agent | Expected | Native Event |
|-------|----------|-------------|
| claude-code | PASS | `SessionStart` |
| gemini-cli | PASS | `SessionStart` |
| cursor | PASS | `sessionStart` |
| windsurf | SKIP | No session-start event |
| vscode-copilot | PASS | `SessionStart` |
| copilot-cli | PASS | `sessionStart` |
| kiro-ide | SKIP | No session-start event |
| kiro-cli | PASS | `agentSpawn` |
| opencode | PASS | `session.created` |
| amazon-q | PASS | `agentSpawn` |
| cline | PASS | `TaskStart` |
| augment-code | PASS | `SessionStart` |

**Note:** Cursor's `session_start` support was missing from spec v0.1.0 — it does exist.

---

### HB-04: session_end event binding

**Category:** Event Binding
**What it checks:** The agent fires a native event when a session ends, and syllago's canonical `session_end` maps to the correct native name.
**Why it matters:** Session-end hooks enable cleanup, audit finalization, and resource release. Spurious bindings (e.g., per-turn events incorrectly mapped as session-end) cause hooks to fire too frequently.
**Automatable:** Yes (trigger: close/exit a session)

| Agent | Expected | Native Event |
|-------|----------|-------------|
| claude-code | PASS | `SessionEnd` |
| gemini-cli | PASS | `SessionEnd` |
| cursor | PASS | `sessionEnd` |
| windsurf | SKIP | No session-end event |
| vscode-copilot | SKIP | No session-end event |
| copilot-cli | PASS | `sessionEnd` |
| kiro-ide | SKIP | No session-end event (stop/agentStop fires per-turn, not at session end) |
| kiro-cli | SKIP | No session-end event |
| opencode | SKIP | No session-end event |
| amazon-q | SKIP | No session-end event |
| cline | PASS | `TaskComplete` |
| augment-code | PASS | `SessionEnd` |

**Note:** Kiro's spec mapping to `stop`/`agentStop` was wrong — those fire per-turn, not at session end.

---

### HB-05: before_prompt event binding

**Category:** Event Binding
**What it checks:** The agent fires a native event when the user submits a prompt, and syllago's canonical `before_prompt` maps to the correct native name.
**Why it matters:** Pre-prompt hooks enable prompt safety checks, PII scrubbing, and context injection. Wrong mappings bypass these controls.
**Automatable:** Yes (trigger: submit a chat message)

| Agent | Expected | Native Event |
|-------|----------|-------------|
| claude-code | PASS | `UserPromptSubmit` |
| gemini-cli | PASS | `BeforeAgent` |
| cursor | PASS | `beforeSubmitPrompt` |
| windsurf | PASS | `pre_user_prompt` |
| vscode-copilot | PASS | `UserPromptSubmit` |
| copilot-cli | PASS | `userPromptSubmitted` |
| kiro-ide | PASS | `promptSubmit` |
| kiro-cli | PASS | `userPromptSubmit` |
| opencode | SKIP | No before-prompt event |
| amazon-q | PASS | `userPromptSubmit` |
| cline | PASS | `UserPromptSubmit` |
| augment-code | SKIP | No before-prompt event (only `SessionStart` for context injection) |

---

### HB-06: agent_stop event binding

**Category:** Event Binding
**What it checks:** The agent fires a native event when the agent finishes responding, and syllago's canonical `agent_stop` maps to the correct native name.
**Why it matters:** Stop hooks enable response post-processing, audit trail completion, and "continue working" patterns.
**Automatable:** Yes (trigger: let agent complete a response)

| Agent | Expected | Native Event |
|-------|----------|-------------|
| claude-code | PASS | `Stop` |
| gemini-cli | PASS | `AfterAgent` |
| cursor | PASS | `stop` |
| windsurf | PASS | `post_cascade_response` |
| vscode-copilot | PASS | `Stop` |
| copilot-cli | PASS | `agentStop` |
| kiro-ide | PASS | `agentStop` |
| kiro-cli | PASS | `stop` |
| opencode | PASS | `session.idle` |
| amazon-q | PASS | `stop` |
| cline | PASS | `TaskComplete` |
| augment-code | PASS | `Stop` |

**Note:** Copilot CLI's `agentStop` was missing from spec v0.1.0 — it does exist.
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-01 through HB-06 with complete tables.

---

## T-03: Write Category 2 — Blocking Behavior (HB-07–10)

**Depends on:** T-02
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-06:

```markdown
---

## Category 2: Blocking Behavior

Blocking behavior checks verify that the agent actually prevents tool execution or prompt processing when a hook signals a block. These are the most safety-critical checks — a hook that silently fails to block means a bypass.

---

### HB-07: before_tool_execute blocking via exit code 2

**Category:** Blocking Behavior
**What it checks:** A hook script that exits with code 2 bound to `before_tool_execute` actually prevents the tool from running.
**Why it matters:** Exit code 2 is the dominant blocking convention. If it doesn't work, every blocking safety hook is silently bypassed.
**Automatable:** Yes (hook exits 2, verify tool does not execute)

| Agent | Expected | Mechanism |
|-------|----------|-----------|
| claude-code | PASS | Exit code 2; stderr surfaced to LLM |
| gemini-cli | PASS | Exit code >=2 (any non-0/non-1) |
| cursor | PASS | Exit code 2 |
| windsurf | PASS | Exit code 2 |
| vscode-copilot | PASS | Exit code 2 |
| copilot-cli | PASS | Exit code 2 |
| kiro-ide | PASS | Any non-zero exit |
| kiro-cli | PASS | Exit code 2 (exit 1 = warning only) |
| opencode | SKIP | In-process; throws Error() instead |
| amazon-q | PASS | Exit code 2 |
| cline | PASS | Exit code 2 or JSON `cancel: true` |
| augment-code | PASS | Exit code 2 |

**Note for gemini-cli:** Any exit code >= 2 blocks. Exit code 1 is non-fatal warning. Unlike most agents, exit code 3, 4, etc. also block.

---

### HB-08: before_tool_execute blocking via JSON decision field

**Category:** Blocking Behavior
**What it checks:** A hook that outputs a JSON object with `decision: "block"` or `permissionDecision: "deny"` actually prevents the tool from running.
**Why it matters:** JSON-based blocking is needed for hooks that want to include a reason/message alongside the block decision.
**Automatable:** Yes

| Agent | Expected | JSON Format |
|-------|----------|-------------|
| claude-code | PASS | `{"decision": "block"}` or `{"permissionDecision": "deny"}` |
| gemini-cli | PASS | `{"decision": "deny"}` or `{"decision": "block"}` |
| cursor | PASS | `{"permission": "deny"}` or `{"continue": false}` |
| windsurf | SKIP | Exit codes only — no structured JSON output |
| vscode-copilot | PASS | `{"permissionDecision": "deny"}` or `{"decision": "block"}` |
| copilot-cli | UNVERIFIED | Unknown — needs empirical testing |
| kiro-ide | SKIP | Exit codes only — no documented JSON format |
| kiro-cli | SKIP | Exit codes only |
| opencode | SKIP | In-process exception model |
| amazon-q | SKIP | Raw text output, not JSON decisions |
| cline | PASS | `{"cancel": true}` |
| augment-code | PASS | `{"permissionDecision": "deny"}` |

**Critical note for cursor:** The format convergence research claimed `{"action": "block"}` — this is WRONG. The correct formats are `{"permission": "deny"}` or `{"continue": false}`.
**Critical note for windsurf:** The format convergence research claimed JSON output works — this is WRONG. Windsurf uses exit codes only.

---

### HB-09: before_prompt blocking (agent prevents message from being submitted)

**Category:** Blocking Behavior
**What it checks:** A hook bound to `before_prompt` that signals a block actually prevents the user's message from reaching the agent.
**Why it matters:** Prompt safety hooks (PII scrubbing, policy enforcement) depend on this. Agents classified as observe-only cannot enforce prompt policies.
**Automatable:** Semi-automatable (requires human to observe whether message is processed)

| Agent | Expected | Mechanism | Notes |
|-------|----------|-----------|-------|
| claude-code | PASS | Exit code 2 | Blocks message processing |
| gemini-cli | PASS | Exit code >=2 | Blocks agent from starting |
| cursor | PASS | `continue: false` in JSON output | Spec v0.1.0 incorrectly classified as observe-only |
| windsurf | PASS | Exit code 2 | Spec v0.1.0 incorrectly classified as observe-only |
| vscode-copilot | FAIL | Observe-only | **Critical:** Cannot block prompts. Spec v0.1.0 incorrectly classified as prevent. |
| copilot-cli | UNVERIFIED | Unknown | Needs empirical testing |
| kiro-ide | PASS | Non-zero exit | IDE system — prevent |
| kiro-cli | UNVERIFIED | Likely observe | CLI system — needs verification |
| opencode | SKIP | No before-prompt event | |
| amazon-q | UNVERIFIED | Unknown | Needs empirical testing |
| cline | PASS | `cancel: true` | |
| augment-code | SKIP | No before-prompt event | |

---

### HB-10: agent_stop "continue" behavior (hook prevents agent from stopping)

**Category:** Blocking Behavior
**What it checks:** A hook bound to `agent_stop` that signals "continue" actually keeps the agent working rather than letting it stop.
**Why it matters:** This pattern is used to implement multi-step workflows where the agent shouldn't stop until certain conditions are met.
**Automatable:** Yes (hook signals continue, verify agent does not stop)

| Agent | Expected | Mechanism |
|-------|----------|-----------|
| claude-code | PASS | `decision: "block"` in JSON output — forces agent to continue |
| gemini-cli | PASS | `decision: "block"` or `continue: true` in JSON output |
| cursor | SKIP | observe-only for stop event |
| windsurf | SKIP | observe-only for stop event |
| vscode-copilot | PASS | `decision: "block"` — forces continuation |
| copilot-cli | SKIP | observe-only |
| kiro-ide | SKIP | observe-only |
| kiro-cli | SKIP | observe-only |
| opencode | SKIP | observe-only |
| amazon-q | SKIP | observe-only |
| cline | SKIP | observe-only |
| augment-code | PASS | `decision: "block"` — keeps agent working |

**Note:** The spec used the term "retry" for this behavior. The correct term is "continue" — the mechanism prevents termination and forces the agent to keep working; it's not an automatic retry of the last action.
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-07 through HB-10.

---

## T-04: Write Category 3 — Capability Support (HB-11–15)

**Depends on:** T-03
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-10:

```markdown
---

## Category 3: Capability Support

Capability checks verify that agent-specific capabilities (structured output, input rewriting, LLM-evaluated hooks, etc.) work as documented. These checks matter for syllago's degradation strategy — the converter needs accurate capability matrices to know when to apply fallbacks.

---

### HB-11: structured_output — hook stdout parsed as JSON

**Category:** Capability Support
**What it checks:** The agent actually parses hook stdout as JSON when the hook returns a decision object, rather than treating it as plain text.
**Why it matters:** Blocking via JSON decisions only works if the agent parses the output. Agents that ignore JSON output are implicitly exit-code-only.
**Automatable:** Yes (output JSON decision object, verify blocking or field behavior)

| Agent | Expected | Notes |
|-------|----------|-------|
| claude-code | PASS | Richest support: `decision`, `permissionDecision`, `updatedInput`, `reason`, etc. |
| gemini-cli | PASS | Supports: `decision`, `continue`, `stopReason`, `suppressOutput`, `reason`, `hookSpecificOutput` |
| cursor | PASS | Supports: `permission`, `continue`, `userMessage`, `agentMessage`, `updated_input` |
| windsurf | FAIL | No structured output — exit codes only |
| vscode-copilot | PASS | Aligned with CC; camelCase naming |
| copilot-cli | UNVERIFIED | Unknown — needs empirical testing |
| kiro-ide | FAIL | Exit codes + stderr only |
| kiro-cli | FAIL | Exit codes + stderr only |
| opencode | SKIP | In-process; return value from plugin function |
| amazon-q | FAIL | Raw text stdout injected as context; no JSON decisions |
| cline | PASS | JSON with `cancel`, `contextModification` |
| augment-code | PASS | JSON with `permissionDecision`, `decision` |

---

### HB-12: input_rewrite — hook can modify tool input before execution

**Category:** Capability Support
**What it checks:** A hook bound to `before_tool_execute` can modify the tool's input arguments before the tool executes.
**Why it matters:** Input rewriting is used for secrets scrubbing, path normalization, and safety guardrails. The spec's degradation strategy for agents without this capability defaults to blocking — which incorrectly fires for agents that DO support it.
**Automatable:** Yes (hook rewrites input, verify tool receives modified input)

| Agent | Expected | JSON Field |
|-------|----------|-----------|
| claude-code | PASS | `updatedInput` in response JSON |
| gemini-cli | PASS | `hookSpecificOutput.tool_input` — **spec v0.1.0 said not supported; this is wrong** |
| cursor | PASS | `preToolUse.updated_input` — **spec v0.1.0 said not supported; this is wrong** |
| windsurf | SKIP | No structured output; no input rewrite |
| vscode-copilot | PASS | `updatedInput` in response JSON |
| copilot-cli | SKIP | No documented input rewrite |
| kiro-ide | SKIP | Exit codes only |
| kiro-cli | SKIP | Exit codes only |
| opencode | PASS | Mutable `output.args` from plugin function |
| amazon-q | SKIP | No structured output |
| cline | SKIP | No input rewrite (has contextModification for context injection) |
| augment-code | SKIP | No documented input rewrite |

**Critical note:** Spec v0.1.0 listed Gemini CLI and Cursor as not supporting `input_rewrite`. Both actually do. The degradation strategy must be updated — syllago should NOT apply blocking fallback for these agents.

---

### HB-13: llm_evaluated — hook uses LLM to make decisions

**Category:** Capability Support
**What it checks:** The agent supports hooks that invoke an LLM (not just a shell script) to evaluate and decide on tool use.
**Why it matters:** LLM-evaluated hooks enable richer policy decisions (semantic analysis, context-aware filtering) but cost more and are slower. Knowing which agents support them affects syllago's capability negotiation.
**Automatable:** No — requires configuring an LLM hook type and observing behavior

| Agent | Expected | Notes |
|-------|----------|-------|
| claude-code | PASS | Supports `type: prompt` and `type: agent` hooks |
| gemini-cli | SKIP | Script-only |
| cursor | SKIP | Script-only |
| windsurf | SKIP | Script-only |
| vscode-copilot | SKIP | Script-only |
| copilot-cli | SKIP | Script-only |
| kiro-ide | PASS | "Ask Kiro" feature — IDE only |
| kiro-cli | SKIP | Script-only |
| opencode | SKIP | Plugin functions only (no LLM hook type) |
| amazon-q | SKIP | Script-only |
| cline | SKIP | Script-only |
| augment-code | SKIP | Script-only |

---

### HB-14: async_execution — hook runs asynchronously

**Category:** Capability Support
**What it checks:** The agent supports fire-and-forget asynchronous hook execution (hook does not block the main flow while running).
**Why it matters:** Async hooks are needed for logging and side-effect-only hooks where blocking would add unacceptable latency.
**Automatable:** Yes (time hook execution to detect blocking)

| Agent | Expected | Notes |
|-------|----------|-------|
| claude-code | PASS | Async by default for non-blocking hooks |
| gemini-cli | UNVERIFIED | Appears to be parallel for independent hooks, sequential when configured |
| cursor | UNVERIFIED | Unknown execution model |
| windsurf | UNVERIFIED | Unknown |
| vscode-copilot | UNVERIFIED | Unknown |
| copilot-cli | UNVERIFIED | Unknown |
| kiro-ide | UNVERIFIED | Unknown |
| kiro-cli | UNVERIFIED | Unknown |
| opencode | PASS | Named hooks are async (Promise<void>, awaited per sequential hook chain) — **spec said not supported; this is wrong** |
| amazon-q | UNVERIFIED | Unknown |
| cline | UNVERIFIED | Unknown |
| augment-code | UNVERIFIED | Unknown |

---

### HB-15: custom_env — hook receives per-hook environment variables

**Category:** Capability Support
**What it checks:** The agent supports defining per-hook environment variables that are passed to the hook script.
**Why it matters:** Custom env vars are used to pass secrets, configuration, and context to hooks without requiring the hook to read from disk.
**Automatable:** Yes (define custom env in hook config, verify hook script can read it)

| Agent | Expected | Notes |
|-------|----------|-------|
| claude-code | SKIP | No per-hook env (HTTP hooks have `allowedEnvVars` allowlist, not injection) |
| gemini-cli | PASS | `env` field per hook in config |
| cursor | PASS | Session-scoped env injection |
| windsurf | SKIP | No per-hook env field |
| vscode-copilot | PASS | `env` field per hook |
| copilot-cli | PASS | `env` field per hook — **spec said not supported; this is wrong** |
| kiro-ide | SKIP | No documented env field |
| kiro-cli | SKIP | No documented env field |
| opencode | SKIP | No per-hook env (shell.env hook is a separate mechanism) |
| amazon-q | SKIP | No documented per-hook env |
| cline | SKIP | No documented per-hook env |
| augment-code | SKIP | No documented per-hook env |
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-11 through HB-15.

---

## T-05: Write Category 4 — Runtime Discovery (HB-16–17)

**Depends on:** T-04
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-15:

```markdown
---

## Category 4: Runtime Discovery

Runtime discovery checks verify that hooks are found and loaded from the expected file paths. If syllago's install command places hooks in the wrong location, they'll never fire.

---

### HB-16: hook config location — hooks loaded from expected path

**Category:** Runtime Discovery
**What it checks:** After `syllago install` places a hook in the expected path for the target agent, the agent actually discovers and loads it without requiring manual configuration.
**Why it matters:** If the install path is wrong, the hook is silently ignored. This validates syllago's per-agent install locations.
**Automatable:** Yes (install hook, trigger event, check if it fires)

| Agent | Expected Path | Config Format |
|-------|--------------|---------------|
| claude-code | `~/.claude/settings.json` (user) or `.claude/settings.json` (project) | JSON merge into `hooks` array |
| gemini-cli | `~/.gemini/settings.json` (user) or `.gemini/settings.json` (project) | JSON merge into `hooks` array |
| cursor | `~/.cursor/hooks.json` (user) or `.cursor/hooks.json` (workspace) | JSON (4-tier hierarchy) |
| windsurf | `~/.codeium/windsurf/hooks.json` (user) or `.windsurf/hooks.json` (project) | JSON |
| vscode-copilot | `.github/hooks/*.json` (project) or `~/.vscode/hooks/*.json` (user) | JSON files, 1 file per hook |
| copilot-cli | `.github/hooks/` (project only) | JSON files, 1 file per hook |
| kiro-ide | IDE-managed settings | IDE-managed (no direct file edit) |
| kiro-cli | `.kiro/hooks/` | JSON |
| opencode | `opencode.json` `hooks` field or `plugins/` directory | JSON or TypeScript |
| amazon-q | `~/.aws/amazonq/agents/<name>.json` | JSON agent config `hooks` field |
| cline | `~/Documents/Cline/Hooks/` (global) or `<workspace>/.clinerules/hooks/` | Executable files named after event |
| augment-code | `~/.augment/settings.json` (user) or workspace settings | JSON `hooks` field |

---

### HB-17: hook config priority — user-level vs project-level precedence

**Category:** Runtime Discovery
**What it checks:** When the same event has hooks at multiple config levels (user vs project), the higher-priority level wins and lower-priority hooks are either merged or overridden as documented.
**Why it matters:** Enterprise deployments use user-level hooks for policy enforcement. Project-level hooks shouldn't be able to override or disable user-level safety hooks.
**Automatable:** Semi-automatable (requires setting up hooks at two levels)

| Agent | Expected | Notes |
|-------|----------|-------|
| claude-code | Merge (all fire) | User + project hooks both run; enterprise can set `allowManagedHooksOnly` |
| gemini-cli | Merge with sequential option | Multiple levels merge; parallel or sequential per config |
| cursor | Workspace overrides user (4-tier: user < workspace < project < folder) | Project-level can silence user hooks |
| windsurf | System > User > Workspace (most restrictive wins for security hooks) | |
| vscode-copilot | User + project merge; "most restrictive wins" for permission | |
| copilot-cli | Project only | No user-level config |
| kiro-ide | IDE-managed | Unknown merge behavior |
| kiro-cli | Unknown | Needs empirical testing |
| opencode | Sequential merge | `opencode.json` + `plugins/` directory |
| amazon-q | Agent-level only | No multi-level config |
| cline | Global + project merge | Both directories loaded |
| augment-code | System > User > Project | System hooks immutable (enterprise enforcement) |
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-16 and HB-17.

---

## T-06: Write Category 5 — Runtime Execution (HB-18–21)

**Depends on:** T-05
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-17:

```markdown
---

## Category 5: Runtime Execution

Runtime execution checks verify timeout behavior, environment variable injection, error handling, and execution ordering for concurrent hooks. These affect reliability and debuggability.

---

### HB-18: timeout — hook killed after configured timeout

**Category:** Runtime Execution
**What it checks:** A hook that runs longer than the configured timeout is actually killed, and the agent handles the timeout gracefully (fail-open by default).
**Why it matters:** A hung hook can freeze the agent indefinitely. Timeout behavior is critical for production safety hooks.
**Automatable:** Yes (slow — hook sleeps 90s, verify agent kills it after timeout)

| Agent | Expected | Default Timeout | Kill Signal |
|-------|----------|----------------|-------------|
| claude-code | PASS | 600s (command) / 30s (prompt) / 60s (agent) | Unknown |
| gemini-cli | PASS | 60s (ms unit) | SIGTERM → 5s → SIGKILL |
| cursor | UNVERIFIED | Unknown | Unknown |
| windsurf | UNVERIFIED | Unknown | Unknown |
| vscode-copilot | PASS | 30s | Unknown |
| copilot-cli | PASS | 30s | Unknown |
| kiro-ide | UNVERIFIED | 30s (ms unit) | Unknown |
| kiro-cli | UNVERIFIED | Unknown | Unknown |
| opencode | SKIP | None (in-process) | N/A |
| amazon-q | PASS | 30s (configurable via `timeout_ms`) | Unknown |
| cline | PASS | 30s | SIGTERM → SIGKILL |
| augment-code | PASS | 60s | Unknown |

---

### HB-19: env vars — agent-standard env vars injected into hook process

**Category:** Runtime Execution
**What it checks:** Standard agent-specific environment variables (project dir, session ID, etc.) are available inside the hook script without any extra configuration.
**Why it matters:** Hooks commonly need to know the project root. If the env var isn't injected, hooks must use brittle alternatives (e.g., hardcoded paths or parsing args).
**Automatable:** Yes (print env vars from hook script, verify values are set)

| Agent | Expected | Primary Env Vars |
|-------|----------|-----------------|
| claude-code | PASS | `CLAUDE_PROJECT_DIR`, `CLAUDE_SESSION_ID`, `CLAUDE_HOOK_EVENT`, `CLAUDE_TOOL_NAME`, `CLAUDE_HOOK_TYPE` |
| gemini-cli | PASS | `GEMINI_PROJECT_DIR`, `CLAUDE_PROJECT_DIR` (compat), sanitized parent env |
| cursor | PASS | `CURSOR_PROJECT_DIR`, `CURSOR_SESSION_ID`, plus 5 more |
| windsurf | PARTIAL | `ROOT_WORKSPACE_PATH` (worktree events only) |
| vscode-copilot | PASS | Standard process env + per-hook env field |
| copilot-cli | PASS | Per-hook env field only |
| kiro-ide | UNVERIFIED | Unknown |
| kiro-cli | UNVERIFIED | Unknown |
| opencode | SKIP | In-process context object |
| amazon-q | PASS | Sends JSON stdin with hook_event_name, cwd, tool fields |
| cline | PASS | JSON stdin with clineVersion, hookName, timestamp, taskId, workspaceRoots |
| augment-code | PASS | `AUGMENT_PROJECT_DIR`, `AUGMENT_CONVERSATION_ID`, `AUGMENT_HOOK_EVENT`, `AUGMENT_TOOL_NAME` |

---

### HB-20: stdin — agent sends hook event data as JSON via stdin

**Category:** Runtime Execution
**What it checks:** The agent sends a JSON object to the hook script's stdin describing the current event (tool name, input args, context, etc.).
**Why it matters:** Hooks that need to inspect tool inputs or context (e.g., blocking dangerous shell commands) depend entirely on stdin JSON. If the format differs from spec, all input-parsing hooks need agent-specific code.
**Automatable:** Yes (read and log stdin from hook script, compare to expected format)

| Agent | Expected | Stdin Format |
|-------|----------|-------------|
| claude-code | PASS | JSON with `hookEventName`, `toolName`, `toolInput`, `sessionId`, etc. |
| gemini-cli | PASS | JSON — similar structure, may include additional fields |
| cursor | UNVERIFIED | JSON expected but format needs verification |
| windsurf | UNVERIFIED | JSON expected but format needs verification |
| vscode-copilot | PASS | JSON aligned with CC format |
| copilot-cli | UNVERIFIED | JSON expected |
| kiro-ide | UNVERIFIED | Unknown |
| kiro-cli | UNVERIFIED | Unknown |
| opencode | SKIP | In-process object |
| amazon-q | PASS | JSON with `hook_event_name`, `cwd`, tool-specific fields |
| cline | PASS | JSON with `clineVersion`, `hookName`, `timestamp`, `taskId`, `workspaceRoots` |
| augment-code | PASS | JSON with `hook_event_name`, `conversation_id`, `workspace_roots`, tool fields |

---

### HB-21: error handling — failed hook does not crash the agent (fail-open)

**Category:** Runtime Execution
**What it checks:** When a hook script exits with a non-zero code (for a non-blocking event), or crashes, the agent continues normally rather than halting.
**Why it matters:** Non-blocking hooks (audit loggers, context injectors) that crash should never stop the agent from working. Fail-open is the expected default.
**Automatable:** Yes (hook exits 1 or throws, verify agent proceeds normally)

| Agent | Expected | Default Behavior |
|-------|----------|-----------------|
| claude-code | PASS | Non-blocking; fail-open on error |
| gemini-cli | PASS | Non-fatal; fail-open on error |
| cursor | PASS | Fail-open default; `failClosed` option available for blocking hooks |
| windsurf | UNVERIFIED | Likely fail-open |
| vscode-copilot | UNVERIFIED | Likely fail-open |
| copilot-cli | UNVERIFIED | Likely fail-open |
| kiro-ide | UNVERIFIED | Unknown |
| kiro-cli | UNVERIFIED | Unknown |
| opencode | PASS | Exception propagates within plugin; other hooks unaffected |
| amazon-q | UNVERIFIED | Unknown |
| cline | PASS | Error logged; hook failure does not cancel task |
| augment-code | UNVERIFIED | Unknown |
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-18 through HB-21.

---

## T-07: Write Category 6 — Security Posture (HB-22–24)

**Depends on:** T-06
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-21:

```markdown
---

## Category 6: Security Posture

Security posture checks are MANUAL assessments — they require reading documentation and configuration options, not running benchmark hooks. Each check is a structured audit of the agent's security model.

---

### HB-22: supply chain protection — project hooks require approval before auto-executing

**Category:** Security Posture
**What it checks:** When a repository contains hook configuration files, the agent requires some form of user approval before executing those hooks (rather than auto-executing on clone/open).
**Why it matters:** Auto-executing project hooks on clone is a supply chain attack vector. An attacker who controls a repository can run arbitrary code on every developer's machine who clones it.
**Automatable:** No — Manual security assessment

| Agent | Expected | Mechanism | Notes |
|-------|----------|-----------|-------|
| claude-code | FAIL | None — project hooks auto-execute | Enterprise `allowManagedHooksOnly` mitigates for managed deployments |
| gemini-cli | PARTIAL | `folderTrust` approval gate — opt-in, off by default | When enabled: strong. Default: auto-executes. |
| cursor | PARTIAL | Workspace trust gate | All-or-nothing; users are nudged to grant trust broadly |
| windsurf | FAIL | None — auto-executes; explicit warning "you are entirely responsible" | No mitigations |
| vscode-copilot | FAIL | None — advisory only | No approval gate |
| copilot-cli | PARTIAL | Ephemeral runner mitigates blast radius | No provenance controls; within-run access is unrestricted |
| kiro-ide | UNVERIFIED | Unknown | No security documentation found |
| kiro-cli | UNVERIFIED | Unknown | |
| opencode | PASS | Structurally safer — no hook-scripts-from-repos model | Plugins are opt-in installed, not auto-discovered from project files |
| amazon-q | FAIL | Hooks auto-execute in agent config | Agents must be explicitly activated via `/agent` (partial mitigation) |
| cline | UNVERIFIED | Unknown | |
| augment-code | PARTIAL | System-level immutable hooks (enterprise) | User/project hooks: unclear approval gate |

---

### HB-23: env var protection — hook scripts cannot trivially read agent API keys

**Category:** Security Posture
**What it checks:** The agent restricts which environment variables are passed to hook scripts, preventing hooks from reading API keys and credentials from the process environment.
**Why it matters:** A malicious hook can trivially exfiltrate API keys and cloud credentials via `curl` if the full process environment is passed through.
**Automatable:** No — Manual security assessment

| Agent | Expected | Mechanism |
|-------|----------|-----------|
| claude-code | PARTIAL | HTTP hooks: `allowedEnvVars` allowlist. Command hooks: full `process.env` |
| gemini-cli | PARTIAL | `sanitizeEnvironment()` deny-list (disabled locally by default, auto-enabled in CI) |
| cursor | FAIL | Full process env passed to hook scripts |
| windsurf | FAIL | Full process env |
| vscode-copilot | FAIL | Full process env (per-hook `env` field adds, does not restrict) |
| copilot-cli | FAIL | Per-hook `env` adds, does not restrict |
| kiro-ide | UNVERIFIED | Unknown |
| kiro-cli | UNVERIFIED | Unknown |
| opencode | PARTIAL | Permission rules gate some access; no full env isolation |
| amazon-q | FAIL | Full process env in hook shell |
| cline | FAIL | Full process env |
| augment-code | PARTIAL | Privacy opt-in flags (`includeUserContext`, `includeMCPMetadata`) — hooks receive minimal data by default |

---

### HB-24: sandboxing — hook scripts run in a restricted execution environment

**Category:** Security Posture
**What it checks:** The agent runs hook scripts in a sandboxed environment that restricts network access, filesystem writes, or process spawning.
**Why it matters:** A sandbox limits the blast radius of a malicious or compromised hook. Without sandboxing, a hook can exfiltrate data, modify files, or spawn persistent processes.
**Automatable:** No — Manual security assessment

| Agent | Expected | Mechanism |
|-------|----------|-----------|
| claude-code | FAIL | No sandbox; hook scripts have full process access |
| gemini-cli | PASS | 5 sandbox methods: macOS Seatbelt, Docker/Podman, Windows Sandbox, gVisor, LXC/LXD. Opt-in. |
| cursor | FAIL | Hook scripts not sandboxed (agent has network restrictions; hooks bypass them) |
| windsurf | FAIL | No sandbox |
| vscode-copilot | FAIL | No sandbox |
| copilot-cli | PARTIAL | Ephemeral Actions runner provides run-to-run isolation; no in-run sandboxing |
| kiro-ide | UNVERIFIED | Unknown |
| kiro-cli | UNVERIFIED | Unknown |
| opencode | PARTIAL | In-process; tool permission rules provide some access control |
| amazon-q | FAIL | No sandbox |
| cline | FAIL | No sandbox |
| augment-code | FAIL | No documented sandbox |
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-22 through HB-24.

---

## T-08: Write Category 7 — Input Rewrite Safety (HB-25)

**Depends on:** T-07
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-24:

```markdown
---

## Category 7: Input Rewrite Safety

This category contains one check covering the critical safety finding from the spec validation report: the spec's degradation strategy for `input_rewrite` is wrong for Gemini CLI and Cursor.

---

### HB-25: input_rewrite capability detection — converter does not incorrectly downgrade

**Category:** Input Rewrite Safety
**What it checks:** When syllago converts a hook with `input_rewrite` capability for Gemini CLI or Cursor, it does NOT apply the blocking-fallback degradation strategy, because both agents actually support input rewriting.
**Why it matters:** The spec v0.1.0 incorrectly classified Gemini CLI and Cursor as not supporting `input_rewrite`. If syllago's converter trusts the spec, it will apply blocking-by-default for these agents — meaning hooks designed to scrub secrets from tool inputs would instead block all tool executions on these agents.
**Automatable:** Yes (install an input-rewriting hook via syllago converter, verify tool receives modified input not a block)

| Agent | Expected Converter Behavior | Input Rewrite Field |
|-------|---------------------------|---------------------|
| claude-code | PASS — passes through `updatedInput` | `updatedInput` |
| gemini-cli | PASS — uses `hookSpecificOutput.tool_input` (not blocking fallback) | `hookSpecificOutput.tool_input` |
| cursor | PASS — uses `preToolUse.updated_input` (not blocking fallback) | `preToolUse.updated_input` |
| windsurf | SKIP — no input rewrite; blocking fallback appropriate | N/A |
| vscode-copilot | PASS — passes through `updatedInput` | `updatedInput` |
| copilot-cli | SKIP — no documented input rewrite | N/A |
| kiro-ide | SKIP | N/A |
| kiro-cli | SKIP | N/A |
| opencode | PASS — mutable `output.args` | `output.args` |
| amazon-q | SKIP — no structured output | N/A |
| cline | SKIP | N/A |
| augment-code | SKIP | N/A |

**Note:** This check validates syllago's converter correctness, not just the agents. It should be re-run after any converter update to the `input_rewrite` capability matrix.
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-25.

---

## T-09: Write Category 8 — Prompt Blocking (HB-26–27)

**Depends on:** T-08
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-25:

```markdown
---

## Category 8: Prompt Blocking

This category contains two checks covering the critical VS Code Copilot finding: its `UserPromptSubmit` event is observe-only, not blocking — a fundamental difference from the spec's claim.

---

### HB-26: vscode-copilot UserPromptSubmit is observe-only (not blocking)

**Category:** Prompt Blocking
**What it checks:** A hook bound to `UserPromptSubmit` on VS Code Copilot that exits with code 2 does NOT prevent the user's message from reaching the agent — it fires after the message is already submitted.
**Why it matters:** Critical security finding. Enterprise teams relying on VS Code Copilot hooks for prompt policy enforcement (PII scrubbing, content filtering) cannot use `before_prompt` as a blocking mechanism. Policies must be implemented on the server side or via other controls.
**Automatable:** Semi-automatable (requires human to observe whether message is processed after hook blocks)

| Agent | Expected Result | Notes |
|-------|----------------|-------|
| vscode-copilot | FAIL to block | Hook fires but message is NOT blocked. Exit code 2 does not prevent processing. |

**Expected behavior:** The hook fires, logs PASS, but the agent proceeds with the user's message regardless of exit code. This is the correct (if confusing) behavior — `UserPromptSubmit` is observe-only.

---

### HB-27: prompt blocking alternatives for observe-only agents

**Category:** Prompt Blocking
**What it checks:** For agents where `before_prompt` is observe-only, the alternative blocking mechanisms (tool blocking, server-side, admin policies) are documented and verified.
**Why it matters:** Users converting a blocking `before_prompt` hook to an observe-only agent need to know the correct alternative — otherwise their safety hook silently becomes a no-op.
**Automatable:** No — Documentation and integration check

| Agent | before_prompt Blocking | Alternative for Blocking |
|-------|----------------------|------------------------|
| claude-code | PASS (can block) | N/A |
| gemini-cli | PASS (can block) | N/A |
| cursor | PASS (can block via `continue: false`) | N/A |
| windsurf | PASS (can block via exit 2) | N/A |
| vscode-copilot | FAIL (observe-only) | Server-side content filtering; admin-managed extensions |
| copilot-cli | UNVERIFIED | Unknown |
| kiro-ide | PASS (can block) | N/A |
| kiro-cli | UNVERIFIED | Unknown |
| opencode | SKIP (no before-prompt event) | Alternative event needed |
| amazon-q | UNVERIFIED | Unknown |
| cline | PASS (can block) | N/A |
| augment-code | SKIP (no before-prompt event) | Alternative event needed |
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-26 and HB-27.

---

## T-10: Write Category 9 — Non-Spec Agent Coverage (HB-28)

**Depends on:** T-09
**Modifies:** `docs/checks/hooks-behavior-checklist.md`

Append after HB-27:

```markdown
---

## Category 9: Non-Spec Agent Coverage

This category contains one check summarizing the non-spec agents (Amazon Q, Cline, Augment Code, gptme, Pi) and their baseline hook compatibility.

---

### HB-28: non-spec agent hook compatibility baseline

**Category:** Non-Spec Agent Coverage
**What it checks:** Agents not covered by the spec v0.1.0 have a documented baseline of hook events, blocking behavior, and config format. This check establishes the minimum required to add a non-spec agent to syllago's converter.
**Why it matters:** Syllago's community wants to share hooks across all agents, including popular ones not in the initial spec. This check defines what "supported" means for a non-spec agent.
**Automatable:** No — Documentation review and basic event fire test

| Agent | Core Events | Blocking | Config Format | Syllago Convertible? |
|-------|-------------|----------|--------------|---------------------|
| amazon-q | 5 (agentSpawn, userPromptSubmit, preToolUse, postToolUse, stop) | Yes (preToolUse, exit 2) | JSON in `~/.aws/amazonq/agents/` | Yes |
| cline | 8 (TaskStart, TaskResume, TaskCancel, TaskComplete, PreToolUse, PostToolUse, UserPromptSubmit, PreCompact) | Yes (PreToolUse + PostToolUse, `cancel: true`) | Directory-based (no JSON manifest) | Partial (directory layout, not JSON merge) |
| augment-code | 5 (PreToolUse, PostToolUse, Stop, SessionStart, SessionEnd) | Yes (PreToolUse via exit 2 or JSON, Stop via `decision: "block"`) | JSON in `~/.augment/settings.json` | Yes |
| roo-code | 0 | N/A | N/A | No (no hook system) |
| gptme | 20+ (Python in-process) | StopPropagation / ConfirmAction | Python functions registered via API | No (language-native; not shell subprocess) |
| pi-agent | 20+ (TypeScript in-process) | `{block: true}` return | TypeScript via jiti | No (language-native; not shell subprocess) |

**Unique capabilities to track for future spec versions:**
- Amazon Q: `cache_ttl_seconds` (output caching), MCP server namespace matchers (`@git`)
- Cline: `contextModification` (structured context injection), PostToolUse blocking, directory-based discovery
- Augment: System-level immutable hooks (enterprise), privacy opt-in data model (`includeUserContext`)
- gptme: `tool.confirm` EDIT option (modify tool input), `before_provider_request` via Pi analog
- Pi: `before_provider_request` (full LLM payload replacement), `tool_result` modification, interactive UI from hooks
```

**Verification:** `docs/checks/hooks-behavior-checklist.md` contains HB-28. The complete checklist file now has all 28 checks.

---

## T-11: Write benchmark hooks HB-01–06 (event binding)

**Depends on:** T-02
**Creates:**
- `content/hooks/benchmark/hb-01-before-tool-execute/.syllago.yaml`
- `content/hooks/benchmark/hb-01-before-tool-execute/hb-01.sh`
- `content/hooks/benchmark/hb-02-after-tool-execute/.syllago.yaml`
- `content/hooks/benchmark/hb-02-after-tool-execute/hb-02.sh`
- `content/hooks/benchmark/hb-03-session-start/.syllago.yaml`
- `content/hooks/benchmark/hb-03-session-start/hb-03.sh`
- `content/hooks/benchmark/hb-04-session-end/.syllago.yaml`
- `content/hooks/benchmark/hb-04-session-end/hb-04.sh`
- `content/hooks/benchmark/hb-05-before-prompt/.syllago.yaml`
- `content/hooks/benchmark/hb-05-before-prompt/hb-05.sh`
- `content/hooks/benchmark/hb-06-agent-stop/.syllago.yaml`
- `content/hooks/benchmark/hb-06-agent-stop/hb-06.sh`

Each `.syllago.yaml` follows this format (values vary per hook):

```yaml
# content/hooks/benchmark/hb-01-before-tool-execute/.syllago.yaml
format_version: 1
name: hb-01-before-tool-execute
type: hooks
description: "Benchmark: validates before_tool_execute event fires"
hook:
  event: before_tool_execute
  blocking: false
  command: ./hb-01.sh
```

Note: The design doc's example uses `type: hook` (singular) and omits `format_version`. The canonical format for this project uses `type: hooks` (plural) and `format_version: 1` — the plan's format above is correct. No `id` field for authored benchmark content (ids are assigned on install).

Each `hb-XX.sh` follows this pattern:

```bash
#!/bin/bash
# HB-01: before_tool_execute event binding
# Logs PASS when the event fires.
# Requires $SYLLAGO_BENCHMARK_LOG to be set before running benchmarks.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-01|before_tool_execute|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
```

Full file list with per-hook values:

| Hook | Event | Script ID | Script event string |
|------|-------|-----------|---------------------|
| hb-01 | before_tool_execute | HB-01 | before_tool_execute |
| hb-02 | after_tool_execute | HB-02 | after_tool_execute |
| hb-03 | session_start | HB-03 | session_start |
| hb-04 | session_end | HB-04 | session_end |
| hb-05 | before_prompt | HB-05 | before_prompt |
| hb-06 | agent_stop | HB-06 | agent_stop |

**Verification:** `ls content/hooks/benchmark/` shows 6 directories, each with `.syllago.yaml` and `hb-XX.sh`. Each script has `chmod +x` applied.

---

## T-12: Write benchmark hooks HB-07–10 (blocking behavior)

**Depends on:** T-03
**Creates:**
- `content/hooks/benchmark/hb-07-block-exit2/.syllago.yaml`
- `content/hooks/benchmark/hb-07-block-exit2/hb-07.sh`
- `content/hooks/benchmark/hb-08-block-json/.syllago.yaml`
- `content/hooks/benchmark/hb-08-block-json/hb-08.sh`
- `content/hooks/benchmark/hb-09-prompt-block/.syllago.yaml`
- `content/hooks/benchmark/hb-09-prompt-block/hb-09.sh`
- `content/hooks/benchmark/hb-10-stop-continue/.syllago.yaml`
- `content/hooks/benchmark/hb-10-stop-continue/hb-10.sh`

HB-07 uses `blocking: true` and exits 2 (so it can be tested for blocking behavior, but logs PASS first):

```yaml
# content/hooks/benchmark/hb-07-block-exit2/.syllago.yaml
format_version: 1
name: hb-07-block-exit2
type: hooks
description: "Benchmark: validates before_tool_execute exit-2 blocking"
hook:
  event: before_tool_execute
  blocking: true
  command: ./hb-07.sh
```

```bash
#!/bin/bash
# HB-07: before_tool_execute blocking via exit code 2
# This hook logs PASS then exits 2 to verify blocking behavior.
# DO NOT install permanently — it will block all tool use.
# Use only for point-in-time blocking verification.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-07|before_tool_execute_block_exit2|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 2
```

HB-08 logs PASS and outputs a JSON block decision:

```bash
#!/bin/bash
# HB-08: before_tool_execute blocking via JSON decision field
# Outputs JSON decision and exits 0. Verify the agent honors the JSON block.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-08|before_tool_execute_block_json|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Output varies by agent during testing — use agent-specific format
echo '{"decision": "block", "reason": "HB-08 benchmark test"}'
```

HB-09 targets `before_prompt` and exits 2 (semi-automatable, observe whether message fires):

```bash
#!/bin/bash
# HB-09: before_prompt blocking
# Logs PASS and exits 2. Observer verifies whether the agent blocks the prompt.
# Semi-automatable: requires human observation of agent behavior.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-09|before_prompt_block|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 2
```

HB-10 targets `agent_stop` and outputs a "continue" decision:

```bash
#!/bin/bash
# HB-10: agent_stop continue behavior
# Logs PASS and outputs a block/continue decision. Verify agent keeps working.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-10|agent_stop_continue|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
echo '{"decision": "block"}'
```

**Verification:** 4 additional directories in `content/hooks/benchmark/` with `.syllago.yaml` and `hb-XX.sh`.

---

## T-13: Write benchmark hooks HB-11–15 (capability support)

**Depends on:** T-04
**Creates:**
- `content/hooks/benchmark/hb-11-structured-output/.syllago.yaml`
- `content/hooks/benchmark/hb-11-structured-output/hb-11.sh`
- `content/hooks/benchmark/hb-12-input-rewrite/.syllago.yaml`
- `content/hooks/benchmark/hb-12-input-rewrite/hb-12.sh`
- `content/hooks/benchmark/hb-14-async-execution/.syllago.yaml`
- `content/hooks/benchmark/hb-14-async-execution/hb-14.sh`
- `content/hooks/benchmark/hb-15-custom-env/.syllago.yaml`
- `content/hooks/benchmark/hb-15-custom-env/hb-15.sh`

Note: HB-13 (llm_evaluated) is non-automatable — no benchmark hook created.

HB-11 outputs a JSON decision object and logs whether the agent parsed it:

```bash
#!/bin/bash
# HB-11: structured output — verify agent parses JSON stdout
# Logs PASS and returns a JSON observation. Tester verifies agent did not
# treat JSON as plain text or inject it verbatim into context.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-11|structured_output|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
echo '{"decision": "allow", "_benchmark": "hb-11"}'
```

HB-12 rewrites the tool input (replaces command arg with a sentinel):

```bash
#!/bin/bash
# HB-12: input_rewrite — verify agent uses rewritten tool input
# Logs PASS and outputs updatedInput. Tester verifies tool received the rewritten
# value (echo "HB-12-REWRITTEN") rather than the original.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-12|input_rewrite|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Claude Code / VS Code Copilot format:
echo '{"updatedInput": {"command": "echo HB-12-REWRITTEN"}}'
```

HB-14 sleeps briefly to test async vs sync execution:

```bash
#!/bin/bash
# HB-14: async execution — verify hook does not block main flow
# Sleeps 3 seconds. If the agent is async, it will not delay tool execution.
# If sync, tool execution will be delayed by 3 seconds.
# Tester times the tool execution latency.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
sleep 3
echo "PASS|HB-14|async_execution|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
```

HB-15 reads and logs the custom env var:

```bash
#!/bin/bash
# HB-15: custom env — verify per-hook env var is injected
# Logs PASS and whether HB15_TEST_VAR is set.
# Configure the hook with HB15_TEST_VAR=benchmark in the agent's hook config.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
if [ -n "$HB15_TEST_VAR" ]; then
  echo "PASS|HB-15|custom_env|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
else
  echo "FAIL|HB-15|custom_env|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
fi
```

**Verification:** 4 additional directories (hb-11, hb-12, hb-14, hb-15) in `content/hooks/benchmark/`.

---

## T-14: Write benchmark hooks HB-16–21 (runtime: discovery + execution)

**Depends on:** T-05, T-06
**Creates:**
- `content/hooks/benchmark/hb-16-discovery/.syllago.yaml`
- `content/hooks/benchmark/hb-16-discovery/hb-16.sh`
- `content/hooks/benchmark/hb-18-timeout/.syllago.yaml`
- `content/hooks/benchmark/hb-18-timeout/hb-18.sh`
- `content/hooks/benchmark/hb-19-env-vars/.syllago.yaml`
- `content/hooks/benchmark/hb-19-env-vars/hb-19.sh`
- `content/hooks/benchmark/hb-20-stdin/.syllago.yaml`
- `content/hooks/benchmark/hb-20-stdin/hb-20.sh`
- `content/hooks/benchmark/hb-21-error-handling/.syllago.yaml`
- `content/hooks/benchmark/hb-21-error-handling/hb-21.sh`

Note: HB-17 (priority/merge behavior) is semi-automatable with no single-script benchmark — skip. HB-18 (timeout) requires a long-running script.

HB-16 (discovery) simply logs PASS — if it fires, the install path was correct:

```bash
#!/bin/bash
# HB-16: hook config location — validates syllago installed to the right path
# This hook fires if and only if the agent discovered it from the correct path.
# A PASS entry in the log means discovery worked.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-16|discovery|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
```

HB-18 (timeout) sleeps 90 seconds — the tester watches for agent kill:

```bash
#!/bin/bash
# HB-18: timeout — hangs for 90 seconds to trigger agent timeout kill
# DO NOT install permanently. Use only for point-in-time timeout verification.
# Expected: agent kills this process after its configured timeout (30-60s typical).

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "START|HB-18|timeout|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
sleep 90
# If this line is reached, the agent did NOT kill the hook — unexpected.
echo "FAIL|HB-18|timeout_not_killed|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
```

HB-19 dumps env vars to the log:

```bash
#!/bin/bash
# HB-19: env vars — logs all injected project/session env vars
# Tester checks log for expected CLAUDE_PROJECT_DIR, GEMINI_PROJECT_DIR, etc.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-19|env_vars|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Log all known agent env vars (will be empty on unsupported agents)
echo "HB-19|CLAUDE_PROJECT_DIR=${CLAUDE_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|GEMINI_PROJECT_DIR=${GEMINI_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|CURSOR_PROJECT_DIR=${CURSOR_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|AUGMENT_PROJECT_DIR=${AUGMENT_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|ROOT_WORKSPACE_PATH=${ROOT_WORKSPACE_PATH:-UNSET}" >> "$LOG"
```

HB-20 dumps stdin to the log:

```bash
#!/bin/bash
# HB-20: stdin — logs the full stdin JSON sent by the agent
# Tester inspects the log to verify format matches spec (hookEventName, toolName, etc.)

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
STDIN_DATA=$(cat)
echo "PASS|HB-20|stdin|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
echo "HB-20|STDIN=${STDIN_DATA}" >> "$LOG"
```

HB-21 exits 1 to test fail-open behavior:

```bash
#!/bin/bash
# HB-21: error handling — exits 1 to test fail-open behavior
# Expected: agent continues normally despite hook failure.
# If agent halts or shows error to user, fail-open is not working.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-21|error_handling_exit1|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 1
```

**Verification:** 5 additional directories (hb-16, hb-18, hb-19, hb-20, hb-21) in `content/hooks/benchmark/`.

---

## T-15: Write benchmark hooks HB-25–27 (input rewrite safety + prompt blocking)

**Depends on:** T-08, T-09
**Creates:**
- `content/hooks/benchmark/hb-25-input-rewrite-no-downgrade/.syllago.yaml`
- `content/hooks/benchmark/hb-25-input-rewrite-no-downgrade/hb-25.sh`
- `content/hooks/benchmark/hb-26-vscode-copilot-prompt-observe/.syllago.yaml`
- `content/hooks/benchmark/hb-26-vscode-copilot-prompt-observe/hb-26.sh`

Note: HB-27 (prompt blocking alternatives) is a documentation check — no benchmark hook.
Note: HB-22–24 (security posture) are manual assessments — no benchmark hooks.
Note: HB-28 (non-spec agents) is a documentation check — no benchmark hook.

HB-25 (input rewrite no-downgrade) — tests that converter output for Gemini/Cursor uses rewrite, not block:

```bash
#!/bin/bash
# HB-25: input_rewrite capability detection
# When converted for gemini-cli or cursor, this hook should rewrite input, NOT block.
# If the tool is blocked, the converter incorrectly applied blocking fallback.
# Tester verifies: (1) tool executes (not blocked), (2) input is rewritten.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-25|input_rewrite_no_downgrade|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Gemini CLI format:
echo '{"hookSpecificOutput": {"tool_input": {"command": "echo HB-25-REWRITTEN"}}}'
# Note: When testing cursor, use {"preToolUse": {"updated_input": {"command": "echo HB-25-REWRITTEN"}}}
```

HB-26 (VS Code Copilot prompt observe) — logs PASS and exits 2, tester observes whether message is blocked:

```bash
#!/bin/bash
# HB-26: vscode-copilot UserPromptSubmit is observe-only
# This hook fires on before_prompt, logs PASS, then exits 2.
# Expected result for vscode-copilot: agent DOES process the message (hook didn't block).
# Expected result for blocking agents (claude-code, gemini-cli, etc.): message IS blocked.
# Tester observes whether their message is processed after the hook fires.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-26|prompt_block_test|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 2
```

**Verification:** 2 additional directories (hb-25, hb-26) in `content/hooks/benchmark/`. Total benchmark hook count: 22.

---

## T-16: Write dogfood plan

**Depends on:** T-11 through T-15 (benchmark hooks must exist before the plan references them)
**Creates:** `docs/plans/2026-03-31-hooks-dogfood-plan.md`

Full content:

```markdown
# Hooks Benchmark Dogfood Plan

**Created:** 2026-03-31
**Benchmark hooks:** `content/hooks/benchmark/`
**Checklist:** `docs/checks/hooks-behavior-checklist.md`
**Results:** `docs/checks/results/<agent-slug>.md`

## Purpose

This plan walks through installing and running the hooks behavior benchmark suite across AI coding agents using syllago itself. It validates two things simultaneously:

1. **Agent behavior** — does the agent fire the expected events and honor blocking signals?
2. **Syllago converter** — does `syllago install` produce hook configs that work on each agent?

## Setup

Before any testing, set the benchmark log location:

```bash
export SYLLAGO_BENCHMARK_LOG=/tmp/syllago-benchmark.log
touch "$SYLLAGO_BENCHMARK_LOG"
```

To watch results live:

```bash
tail -f /tmp/syllago-benchmark.log
```

## Test Execution Per Agent

For each agent:

**Step 1: Install benchmark hooks via syllago**

```bash
syllago install hb-01-before-tool-execute --provider <agent-slug>
syllago install hb-02-after-tool-execute --provider <agent-slug>
syllago install hb-03-session-start --provider <agent-slug>
# ... for each applicable check
```

Or, once registry support lands:
```bash
syllago install content/hooks/benchmark --provider <agent-slug> --all
```

**Step 2: Verify hook file placement**

Inspect the agent's config file to confirm:
- Hook entries are present at the correct path
- Event names are converted to the agent's native names
- Shell command is pointing to the installed script
- No YAML/JSON syntax errors

**Step 3: Trigger events**

| Event | How to trigger |
|-------|---------------|
| before_tool_execute | Ask agent to run a shell command or read a file |
| after_tool_execute | Same — fires after step above |
| session_start | Open a new session / restart the agent |
| session_end | Close the session |
| before_prompt | Submit any chat message |
| agent_stop | Ask agent a question, wait for it to finish |

**Step 4: Check log**

```bash
cat /tmp/syllago-benchmark.log
```

Expected format:
```
PASS|HB-01|before_tool_execute|2026-03-31T15:30:00Z
PASS|HB-02|after_tool_execute|2026-03-31T15:30:01Z
PASS|HB-03|session_start|2026-03-31T15:30:02Z
```

**Step 5: Record results**

Copy `docs/checks/results/_template.md` to `docs/checks/results/<agent-slug>.md` and fill in each check's status.

---

## Phase 1: Free Agents

Target agents with free access. Run all applicable checks from HB-01–28.

**Note on ordering:** The design doc lists gptme and Pi as Phase 1 (free) and Amazon Q / Augment as Phase 3 (stretch). This plan inverts that: Amazon Q and Augment are mainstream CLI agents with straightforward shell hook models — they're better Phase 1 targets. gptme and Pi use language-native in-process hooks (Python and TypeScript respectively) — benchmark shell scripts won't run on them, so they're stretch. Kiro is split by mode: kiro-cli (Phase 1) vs kiro-ide (Phase 3, requires IDE environment).

### Gemini CLI

**Prerequisites:** Gemini CLI installed, `gemini` in PATH

```bash
# Install hooks
syllago install hb-01-before-tool-execute --provider gemini-cli
syllago install hb-02-after-tool-execute --provider gemini-cli
syllago install hb-03-session-start --provider gemini-cli
syllago install hb-04-session-end --provider gemini-cli
syllago install hb-05-before-prompt --provider gemini-cli
syllago install hb-06-agent-stop --provider gemini-cli
syllago install hb-11-structured-output --provider gemini-cli
syllago install hb-12-input-rewrite --provider gemini-cli
syllago install hb-16-discovery --provider gemini-cli
syllago install hb-19-env-vars --provider gemini-cli
syllago install hb-20-stdin --provider gemini-cli

# Verify placement
cat ~/.gemini/settings.json | jq '.hooks'

# Run session
gemini
# Trigger: open session (HB-03), submit prompt (HB-05), run shell tool (HB-01, HB-02), exit (HB-04, HB-06)

# Check results
cat /tmp/syllago-benchmark.log
```

**Expected checks to PASS:** HB-01, HB-02, HB-03, HB-04, HB-05, HB-06, HB-07, HB-11, HB-12 (uses `hookSpecificOutput.tool_input`)
**Expected checks to SKIP:** HB-22, HB-24 (not automatable)

**Format Note:** After installation, inspect `~/.gemini/settings.json`. Verify hook uses `hookSpecificOutput.tool_input` (not generic `updatedInput`). The benchmark hook logs in generic format for cross-agent testing, but the Gemini CLI converter should produce the agent-specific field name. If the converter emits `updatedInput`, that's a converter bug — Gemini CLI won't recognize it.

### Windsurf

**Prerequisites:** Windsurf installed

Note: Windsurf has no session_start/session_end events. Skip HB-03, HB-04.
Note: Windsurf uses exit codes only — no JSON structured output. Expect HB-08, HB-11 to FAIL.
Note: Windsurf `before_prompt` CAN block (exit 2). Expect HB-09 to PASS.

```bash
syllago install hb-01-before-tool-execute --provider windsurf
syllago install hb-02-after-tool-execute --provider windsurf
syllago install hb-05-before-prompt --provider windsurf
syllago install hb-06-agent-stop --provider windsurf
syllago install hb-16-discovery --provider windsurf
syllago install hb-19-env-vars --provider windsurf

cat ~/.codeium/windsurf/hooks.json

# Trigger events in Windsurf
# Check results
cat /tmp/syllago-benchmark.log
```

### Cursor

**Prerequisites:** Cursor installed

Note: Cursor has sessionStart/sessionEnd. Expect HB-03, HB-04 to PASS.
Note: Cursor supports input_rewrite via `preToolUse.updated_input`. Expect HB-25 to PASS (no downgrade).

```bash
syllago install hb-01-before-tool-execute --provider cursor
syllago install hb-03-session-start --provider cursor
syllago install hb-04-session-end --provider cursor
syllago install hb-12-input-rewrite --provider cursor
syllago install hb-25-input-rewrite-no-downgrade --provider cursor
syllago install hb-16-discovery --provider cursor
syllago install hb-19-env-vars --provider cursor
syllago install hb-20-stdin --provider cursor

cat ~/.cursor/hooks.json

# Trigger events in Cursor
cat /tmp/syllago-benchmark.log
```

### OpenCode

**Prerequisites:** OpenCode CLI installed

Note: OpenCode is in-process TypeScript. Script-based hooks use a different mechanism (shell.env, tool plugins). Most checks will SKIP.

```bash
# OpenCode uses opencode.json plugins dir — verify converter output
syllago install hb-01-before-tool-execute --provider opencode
syllago install hb-16-discovery --provider opencode

cat opencode.json

# Trigger tool use
opencode
cat /tmp/syllago-benchmark.log
```

### Amazon Q Developer

**Prerequisites:** AWS CLI installed, `q` CLI available, `~/.aws/amazonq/agents/` accessible

```bash
syllago install hb-01-before-tool-execute --provider amazon-q
syllago install hb-05-before-prompt --provider amazon-q
syllago install hb-03-session-start --provider amazon-q
syllago install hb-16-discovery --provider amazon-q

cat ~/.aws/amazonq/agents/benchmark.json

q chat
# Use /agent benchmark to activate
# Trigger events
cat /tmp/syllago-benchmark.log
```

### Cline

**Prerequisites:** Cline installed in VS Code

Note: Cline uses directory-based auto-discovery. Syllago install should create script files in the hooks directory, not JSON config.

```bash
syllago install hb-01-before-tool-execute --provider cline
# Verify: ls ~/Documents/Cline/Hooks/
# Trigger tool use in Cline
cat /tmp/syllago-benchmark.log
```

### Augment Code

**Prerequisites:** Augment Code CLI installed

```bash
syllago install hb-01-before-tool-execute --provider augment-code
syllago install hb-03-session-start --provider augment-code
syllago install hb-04-session-end --provider augment-code
syllago install hb-05-before-prompt --provider augment-code
syllago install hb-06-agent-stop --provider augment-code
syllago install hb-16-discovery --provider augment-code
syllago install hb-19-env-vars --provider augment-code

cat ~/.augment/settings.json | jq '.hooks'
# Trigger events
cat /tmp/syllago-benchmark.log
```

### Kiro CLI

**Prerequisites:** Kiro CLI installed

```bash
syllago install hb-01-before-tool-execute --provider kiro-cli
syllago install hb-05-before-prompt --provider kiro-cli
syllago install hb-16-discovery --provider kiro-cli

ls .kiro/hooks/
# Trigger events
cat /tmp/syllago-benchmark.log
```

---

## Phase 2: Paid Agents

Run after Phase 1 validation confirms the framework is working. All applicable checks HB-01–28.

### Claude Code

**Prerequisites:** Claude Code subscription active

```bash
syllago install hb-01-before-tool-execute --provider claude-code
syllago install hb-02-after-tool-execute --provider claude-code
syllago install hb-03-session-start --provider claude-code
syllago install hb-04-session-end --provider claude-code
syllago install hb-05-before-prompt --provider claude-code
syllago install hb-06-agent-stop --provider claude-code
syllago install hb-07-block-exit2 --provider claude-code
syllago install hb-08-block-json --provider claude-code
syllago install hb-10-stop-continue --provider claude-code
syllago install hb-11-structured-output --provider claude-code
syllago install hb-12-input-rewrite --provider claude-code
syllago install hb-16-discovery --provider claude-code
syllago install hb-19-env-vars --provider claude-code
syllago install hb-20-stdin --provider claude-code
syllago install hb-21-error-handling --provider claude-code

cat ~/.claude/settings.json | jq '.hooks'

claude
# Trigger all event types
cat /tmp/syllago-benchmark.log
```

**Expected checks to PASS:** HB-01–12, HB-16, HB-18, HB-19, HB-20, HB-21

### VS Code Copilot

**Prerequisites:** VS Code Copilot subscription

Note: Critical check — HB-26 (before_prompt is observe-only). Expect this to demonstrate that exit code 2 does NOT block prompts.

```bash
syllago install hb-01-before-tool-execute --provider vscode-copilot
syllago install hb-26-vscode-copilot-prompt-observe --provider vscode-copilot
syllago install hb-16-discovery --provider vscode-copilot

ls .github/hooks/
# Open VS Code with Copilot
# Submit a prompt — observe whether it is processed despite HB-26 hook exiting 2
cat /tmp/syllago-benchmark.log
```

**Expected outcome for HB-26:** Log shows `PASS|HB-26|prompt_block_test|...` (hook fired), but your message IS processed (proving observe-only).

### Copilot CLI

**Prerequisites:** GitHub Copilot CLI installed (`gh extension install github/gh-copilot` or standalone), GitHub Copilot subscription

Note: Copilot CLI uses project-only hooks (no user-level config). Config path is `.github/hooks/`.
Note: Many checks are UNVERIFIED — this is the highest-priority empirical target.

```bash
syllago install hb-01-before-tool-execute --provider copilot-cli
syllago install hb-02-after-tool-execute --provider copilot-cli
syllago install hb-03-session-start --provider copilot-cli
syllago install hb-05-before-prompt --provider copilot-cli
syllago install hb-06-agent-stop --provider copilot-cli
syllago install hb-07-block-exit2 --provider copilot-cli
syllago install hb-16-discovery --provider copilot-cli
syllago install hb-19-env-vars --provider copilot-cli
syllago install hb-20-stdin --provider copilot-cli

ls .github/hooks/

# Trigger tool use via copilot CLI
# Check results
cat /tmp/syllago-benchmark.log
```

**Expected checks to PASS:** HB-01, HB-02, HB-03, HB-05, HB-06, HB-07, HB-16
**High-value UNVERIFIED targets:** HB-08 (JSON blocking), HB-09 (prompt blocking), HB-11 (structured output), HB-15 (custom env)

---

## Phase 3: Stretch

Run if agents are available. Record results even if only partial checks are possible.

### Kiro IDE

**Prerequisites:** Kiro IDE installed (requires IDE environment — cannot run headless)

Focus on: HB-01 (tool events), HB-16 (discovery), HB-22 (security posture manual assessment)

### gptme

**Prerequisites:** gptme installed

Note: gptme uses Python in-process hooks, not shell scripts. The benchmark hooks are bash scripts — they won't work as gptme plugins. Record SKIP for most checks. Manually document the event model.

### Pi Agent

**Prerequisites:** Pi Agent installed

Note: Pi uses TypeScript in-process hooks. Same situation as gptme. Record SKIP for automated checks. Manually document capabilities for HB-28.

---

## Manual Checks (HB-13, HB-17, HB-22–24, HB-27–28)

These checks cannot be tested with automated benchmark hooks. Verify them manually and record results directly in the agent's result file.

### HB-13: LLM-Evaluated Hooks
For agents supporting LLM hook types (claude-code `prompt`/`agent` types, kiro-ide "Ask Kiro"):
1. Configure an LLM-evaluated hook in the agent's settings
2. Trigger the bound event and observe whether the LLM handler runs
3. Record: PASS (LLM hook executes as expected) / SKIP (agent doesn't support this)

### HB-17: Config Precedence (Multi-Level Merge)
For agents with multiple config levels (claude-code has user + project, gemini-cli has 4 levels, cursor has 4 tiers):
1. Install a hook at the user/global level
2. Install the same hook at the project level with different behavior (e.g., different log message)
3. Verify: Which takes precedence? Both run? Does behavior match documentation?
4. Record: PASS (behavior matches documented precedence) / FAIL (unexpected merge behavior)

Best tested on Claude Code first (user: `~/.claude/settings.json`, project: `.claude/settings.json`).

### HB-22: Supply Chain Protection
1. Clone or open a repo that contains a hook config file (`.claude/settings.json`, `.gemini/settings.json`, etc.)
2. Observe: Does the agent prompt for approval before executing project-scoped hooks?
3. Record: PASS (approval required) / PARTIAL (warning shown but auto-executes) / FAIL (silent auto-execute)

### HB-23: Environment Variable Protection
1. Review agent documentation for env var filtering mechanisms
2. Check: Does agent restrict which env vars hook processes receive (allowlist, denylist, sanitization)?
3. Record: PASS (env vars restricted) / PARTIAL (some restriction) / FAIL (full parent env passed through)

### HB-24: Hook Sandboxing
1. Check: Does agent provide any hook execution sandbox (gVisor, Docker, namespace, capabilities)?
2. If yes: test network access and file system access from within a hook
3. Record: PASS (sandbox enforced) / PARTIAL (some isolation, e.g., network only) / FAIL (no sandbox)

### HB-27: Prompt Blocking Alternatives
For agents where `before_prompt` is observe-only (VS Code Copilot confirmed):
1. Document what alternative blocking mechanisms exist (server-side filtering, admin policies, tool-level blocking)
2. Record: the specific alternatives available, not pass/fail

### HB-28: Non-Spec Agent Baseline
For Amazon Q, Cline, Augment, gptme, Pi:
1. Verify syllago converter can handle the agent (or document that it can't yet)
2. Test basic hook installation and event firing
3. Record: agent supports hooks (Yes/No), converter status (Works/Partial/Not yet implemented)

---

## Result Collection

After each agent run:

1. Clear the log: `> /tmp/syllago-benchmark.log`
2. Run the agent session (trigger all applicable events)
3. Copy `docs/checks/results/_template.md` to `docs/checks/results/<agent-slug>.md`
4. Fill in each check row with status (PASS/FAIL/SKIP/PARTIAL/UNVERIFIED/MANUAL)
5. Paste contents of `/tmp/syllago-benchmark.log` into the "Raw Log" code block in the result file
6. Add notes for any converter issues, unexpected behavior, or format mismatches
7. Commit only `.md` files (no separate `.log` files — log content lives inside the result markdown)

## What a Complete Run Produces

- One `docs/checks/results/<agent-slug>.md` per tested agent (log embedded in markdown)
- Empirical confirmation (or correction) of expected results in the checklist
- Bug reports for syllago converter if installed hooks don't fire as expected
- Updated `docs/checks/hooks-behavior-checklist.md` with confirmed results replacing UNVERIFIED
- Manual check results for HB-13, HB-17, HB-22–24, HB-27–28 recorded in each agent's result file
```

**Verification:** `docs/plans/2026-03-31-hooks-dogfood-plan.md` exists with Phase 1, Phase 2, Phase 3, and result collection sections.

---

## T-17: Write result collection templates

**Depends on:** T-16
**Creates:**
- `docs/checks/results/_template.md`
- `docs/checks/results/README.md`

`_template.md`:

```markdown
# <AGENT_SLUG> Benchmark Results

**Agent:** <agent-slug>
**Test date:** <ISO date>
**Syllago version:** <version from `syllago version`>
**OS:** <platform>
**Agent version:** <version>

## Summary

| Stat | Count |
|------|-------|
| PASS | |
| FAIL | |
| SKIP | |
| PARTIAL | |
| UNVERIFIED | |

## Converter Validation

Syllago install command: `syllago install ... --provider <agent-slug>`
Hook config file inspected: `<path>`
All events mapped to native names correctly: Yes / No / Partial

Issues found:
- (none)

## Check Results

| Check | Status | Notes |
|-------|--------|-------|
| HB-01 | | |
| HB-02 | | |
| HB-03 | | |
| HB-04 | | |
| HB-05 | | |
| HB-06 | | |
| HB-07 | | |
| HB-08 | | |
| HB-09 | | |
| HB-10 | | |
| HB-11 | | |
| HB-12 | | |
| HB-13 | | |
| HB-14 | | |
| HB-15 | | |
| HB-16 | | |
| HB-17 | | |
| HB-18 | | |
| HB-19 | | |
| HB-20 | | |
| HB-21 | | |
| HB-22 | | |
| HB-23 | | |
| HB-24 | | |
| HB-25 | | |
| HB-26 | | |
| HB-27 | | |
| HB-28 | | |

## Raw Log

```
(paste /tmp/syllago-benchmark.log contents here)
```

## Notes

(observations, issues encountered, converter bugs found)
```

`README.md`:

```markdown
# Benchmark Results

One file per tested agent. Files are named `<agent-slug>.md`.

## Status

| Agent | File | Last Tested |
|-------|------|-------------|
| gemini-cli | gemini-cli.md | Not yet |
| windsurf | windsurf.md | Not yet |
| cursor | cursor.md | Not yet |
| opencode | opencode.md | Not yet |
| amazon-q | amazon-q.md | Not yet |
| cline | cline.md | Not yet |
| augment-code | augment-code.md | Not yet |
| kiro-cli | kiro-cli.md | Not yet |
| kiro-ide | kiro-ide.md | Not yet |
| claude-code | claude-code.md | Not yet |
| vscode-copilot | vscode-copilot.md | Not yet |
| copilot-cli | copilot-cli.md | Not yet |

## How to Add Results

1. Copy `_template.md` to `<agent-slug>.md`
2. Run the agent section from `docs/plans/2026-03-31-hooks-dogfood-plan.md`
3. Fill in statuses (PASS/FAIL/SKIP/PARTIAL/UNVERIFIED)
4. Paste raw log at the bottom
5. Update the table above with today's date
```

**Verification:** `docs/checks/results/_template.md` and `docs/checks/results/README.md` both exist.

---

## Dependency Graph

```
T-01 (scaffold checklist)
  └── T-02 (HB-01–06 event binding)
        └── T-03 (HB-07–10 blocking)
              └── T-04 (HB-11–15 capability)
                    └── T-05 (HB-16–17 discovery)
                          └── T-06 (HB-18–21 execution)
                                └── T-07 (HB-22–24 security)
                                      └── T-08 (HB-25 input rewrite safety)
                                            └── T-09 (HB-26–27 prompt blocking)
                                                  └── T-10 (HB-28 non-spec)

T-11 (benchmark hooks HB-01–06) ← depends T-02
T-12 (benchmark hooks HB-07–10) ← depends T-03
T-13 (benchmark hooks HB-11–15) ← depends T-04
T-14 (benchmark hooks HB-16–21) ← depends T-05, T-06
T-15 (benchmark hooks HB-25–27) ← depends T-08, T-09

T-16 (dogfood plan) ← depends T-11 through T-15
T-17 (result templates) ← depends T-16
```

## Success Criteria

When all tasks are complete:

1. `docs/checks/hooks-behavior-checklist.md` has 28 checks (HB-01–28) across 9 categories
2. `content/hooks/benchmark/` has 22 hook directories, each with `.syllago.yaml` and `hb-XX.sh`
3. `docs/plans/2026-03-31-hooks-dogfood-plan.md` covers Phase 1 (8 free agents), Phase 2 (2 paid), Phase 3 (2 stretch) with step-by-step execution instructions
4. `docs/checks/results/_template.md` and `docs/checks/results/README.md` exist
5. At least one agent run from the dogfood plan completed and results recorded
