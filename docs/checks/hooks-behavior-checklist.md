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
| 1. Event Binding | 6 | HB-01-06 |
| 2. Blocking Behavior | 4 | HB-07-10 |
| 3. Capability Support | 5 | HB-11-15 |
| 4. Runtime Discovery | 2 | HB-16-17 |
| 5. Runtime Execution | 4 | HB-18-21 |
| 6. Security Posture | 3 | HB-22-24 |
| 7. Input Rewrite Safety | 1 | HB-25 |
| 8. Prompt Blocking | 2 | HB-26-27 |
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
