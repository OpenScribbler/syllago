# Hook Behavior Matrix

Research date: 2026-03-31
Method: Source code inspection + official docs, validated by sonnet subagents (4 research groups, 4 validators, 1 round until clean)

---

## 1. Event Name Mapping (Corrected)

This is the validated version of the spec's Section 7.4 mapping table. Changes from the spec are marked with `*`.

| Canonical | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro (IDE) | Kiro (CLI) | OpenCode |
|-----------|-------------|------------|--------|----------|-----------------|-------------|------------|------------|----------|
| before_tool_execute | PreToolUse | BeforeTool | preToolUse / beforeShellExecution / beforeMCPExecution / beforeReadFile | pre_read_code / pre_write_code / pre_run_command / pre_mcp_tool_use | PreToolUse | preToolUse | preToolUse | preToolUse | tool.execute.before |
| after_tool_execute | PostToolUse | AfterTool | postToolUse* / afterShellExecution* / afterMCPExecution* / afterFileEdit | post_read_code / post_write_code / post_run_command / post_mcp_tool_use | PostToolUse | postToolUse | postToolUse | postToolUse | tool.execute.after |
| session_start | SessionStart | SessionStart | sessionStart* | -- | SessionStart | sessionStart | -- | agentSpawn | session.created |
| session_end | SessionEnd | SessionEnd | sessionEnd* | -- | -- | sessionEnd | -- | -- (no real session-end)* | -- |
| before_prompt | UserPromptSubmit | BeforeAgent | beforeSubmitPrompt | pre_user_prompt | UserPromptSubmit | userPromptSubmitted | promptSubmit | userPromptSubmit | -- |
| agent_stop | Stop | AfterAgent | stop | post_cascade_response | Stop | agentStop* | agentStop | stop | session.idle |
| before_compact | PreCompact | PreCompress | -- | -- | PreCompact | -- | -- | -- | -- |
| notification | Notification | Notification | -- | -- | -- | -- | -- | -- | -- |
| error_occurred | ErrorOccurred* | -- | -- | -- | -- | errorOccurred | -- | -- | session.error |
| stop_failure* | StopFailure* | -- | -- | -- | -- | -- | -- | -- | -- |
| tool_use_failure | PostToolUseFailure | -- | postToolUseFailure | -- | -- | errorOccurred | -- | -- | -- |
| file_changed | FileChanged | -- | afterFileEdit | -- | -- | -- | fileEdited* | -- | file.edited |
| subagent_start | SubagentStart | -- | subagentStart | -- | SubagentStart | -- | -- | -- | -- |
| subagent_stop | SubagentStop | -- | subagentStop | -- | SubagentStop | subagentStop | -- | -- | -- |
| permission_request | PermissionRequest | -- | -- | -- | -- | -- | -- | -- | permission.asked / permission.ask (bug: never triggered) |
| before_model | -- | BeforeModel | -- (no beforeAgentResponse)* | -- | -- | -- | -- | -- | -- |
| after_model | -- | AfterModel | afterAgentResponse | -- | -- | -- | -- | -- | -- |
| before_tool_selection | -- | BeforeToolSelection | -- (does not exist)* | -- | -- | -- | -- | -- | -- |
| config_change | ConfigChange | -- | -- | -- | -- | -- | -- | -- | -- |
| file_created | -- | -- | -- | -- | -- | -- | fileCreated* | -- | -- |
| file_deleted | -- | -- | -- | -- | -- | -- | fileDeleted* | -- | -- |
| before_task | -- | -- | -- | -- | -- | -- | preTaskExecution* | -- | -- |
| after_task | -- | -- | -- | -- | -- | -- | postTaskExecution* | -- | -- |

### Key corrections from spec v0.1.0

1. **`*` Cursor has sessionStart/sessionEnd** — spec said not supported
2. **`*` Cursor has 4 after-tool events**, not just afterFileEdit — added afterShellExecution, afterMCPExecution, postToolUse
3. **`*` Cursor has NO beforeAgentResponse** — before_model mapping was broken
4. **`*` Cursor has NO beforeToolSelection** — claim was unsubstantiated
5. **`*` Copilot CLI supports agentStop** — spec said not supported
6. **`*` CC error_occurred → ErrorOccurred**, not StopFailure — these are separate canonical events. Note: ErrorOccurred may be undocumented in CC official docs.
7. **`*` Kiro event names are camelCase identifiers** — spec used display names ("File Save" → `fileEdited`, "File Create" → `fileCreated`, etc.)
8. **`*` Kiro has no session_end event** — `stop`/`agentStop` fires per-turn
9. **Kiro split into IDE and CLI columns** — different event sets and different behavior

### CC Events Missing from Spec (11)

| CC Native | Proposed Canonical |
|-----------|-------------------|
| TaskCreated | task_created |
| TaskCompleted | task_completed |
| TeammateIdle | teammate_idle |
| InstructionsLoaded | instructions_loaded |
| CwdChanged | cwd_changed |
| WorktreeCreate | worktree_create |
| WorktreeRemove | worktree_remove |
| PostCompact | after_compact |
| Elicitation | elicitation |
| ElicitationResult | elicitation_result |
| StopFailure | stop_failure |

### Additional Events Not in Spec

| Agent | Event | Description |
|-------|-------|-------------|
| Cursor | afterAgentThought | After agent completes reasoning step |
| Cursor | preToolUse (generic) | Generic pre-tool, supports updated_input |
| Windsurf | post_cascade_response_with_transcript | After response with full conversation transcript |
| Windsurf | post_setup_worktree | After git worktree creation |
| Kiro IDE | userTriggered | Manually-invoked hooks |
| OpenCode | chat.message | New message received |
| OpenCode | chat.params | Modify LLM sampling parameters |
| OpenCode | shell.env | Inject env vars into shell executions |
| OpenCode | tool.definition | Modify tool descriptions/parameters |

---

## 2. Blocking Behavior Matrix (Corrected)

| Event | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro IDE | Kiro CLI | OpenCode |
|-------|-------------|------------|--------|----------|-----------------|-------------|----------|----------|----------|
| before_tool_execute | prevent | prevent | prevent | prevent | prevent | prevent | prevent | prevent | prevent |
| after_tool_execute | observe | observe | observe | observe | observe+block* | observe | observe | observe | observe |
| session_start | observe | observe | observe* | -- | observe | observe | -- | observe | observe |
| session_end | observe | observe | observe* | -- | -- | observe | -- | -- | -- |
| before_prompt | prevent | prevent | prevent* | prevent* | observe* | observe? | prevent | observe | -- |
| agent_stop | continue* | continue* | observe | observe | continue* | observe | observe | observe | observe |

### Key corrections from spec v0.1.0

1. **`*` VS Code Copilot PostToolUse can block** via `decision: "block"` — not purely observe
2. **`*` VS Code Copilot UserPromptSubmit is OBSERVE, not prevent** — CRITICAL security finding. Cannot block prompts.
3. **`*` Cursor beforeSubmitPrompt CAN block** via `continue: false` — spec said observe
4. **`*` Windsurf pre_user_prompt CAN block** via exit code 2 — spec said observe
5. **`*` Cursor has session_start/session_end** — spec said not supported
6. **`*` agent_stop "retry" relabeled to "continue"** — the mechanism prevents termination and forces the agent to continue, not a retry. Applies to CC, Gemini CLI, and VS Code Copilot.
7. **`*` Kiro before_prompt differs by system** — IDE is PREVENT, CLI is OBSERVE

### Blocking Mechanisms by Agent

| Agent | Primary Mechanism | JSON Output Blocking | Notes |
|-------|------------------|---------------------|-------|
| Claude Code | Exit code 2 | `permissionDecision: "deny"`, `decision: "block"` | Both mechanisms work |
| Gemini CLI | Exit code >=2 (any non-0/non-1) | `decision: "deny"/"block"` | Note: not just exit 2, any >=2 |
| Cursor | Exit code 2 | `permission: "deny"`, `continue: false` | NOT `action: "block"` as convergence research claimed |
| Windsurf | Exit code 2 | None — exit codes only | NOT JSON as convergence research claimed |
| VS Code Copilot | Exit code 2 | `permissionDecision: "deny"`, `decision: "block"` | Aligned with CC |
| Copilot CLI | Exit code 2 | Unknown | |
| Kiro IDE | Any non-zero | None documented | |
| Kiro CLI | Exit code 2 | None documented | Exit 1 is warning |
| OpenCode | throw Error() | N/A (in-process) | Exception-based |

---

## 3. Capability Support Matrix (Corrected)

| Capability | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro | OpenCode |
|-----------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| structured_output | Yes (richest) | Yes (+ continue, stopReason, suppressOutput, reason)* | Yes (permission, userMessage, agentMessage) | No | Yes (aligned with CC, camelCase naming) | Unknown | No (exit codes + stderr) | N/A (in-process) |
| input_rewrite | Yes (updatedInput) | Yes (tool_input)* | Yes (preToolUse.updated_input)* | No | Yes (updatedInput) | No | No | Yes (mutable output.args) |
| llm_evaluated | Yes (prompt + agent types) | No | No | No | No | No | Yes (Ask Kiro, IDE only) | No |
| http_handler | Yes (type: http) | No | No | No | No | No | No | No |
| async_execution | Yes | No | No | No | No | No | No | Yes (named hooks are async/awaited)* |
| platform_commands | No | No | No | No | Yes (windows/linux/osx) | Yes (bash/powershell) | No | No |
| custom_env | No | Yes (per-hook env field) | Yes (session-scoped) | No | Yes (env field) | Yes* | No | Partial (shell.env hook) |
| configurable_cwd | No | No | Per-tier CWD | Yes (working_directory) | Yes (cwd) | Yes (cwd) | No | No |

### Key corrections from spec v0.1.0

1. **`*` Gemini CLI DOES support input_rewrite** via `hookSpecificOutput.tool_input` — spec said not supported
2. **`*` Cursor DOES support input_rewrite** via `preToolUse.updated_input` — spec said not supported
3. **`*` Copilot CLI DOES support custom_env** via `env` field — spec said not supported
4. **`*` Gemini CLI structured_output has additional fields** not in spec: `continue`, `stopReason`, `suppressOutput`, `reason`
5. **`*` OpenCode named hooks ARE async** — spec said not supported. Only event subscribe-all is fire-and-forget.

---

## 4. Runtime Behavior Matrix

| Dimension | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro | OpenCode |
|-----------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| **Discovery** | settings.json (5 levels) | settings.json (4 levels) | hooks.json (4 tiers) | hooks.json (4 tiers) | .github/hooks/*.json | .github/hooks/*.json (project only) | IDE-managed | opencode.json + plugins dir |
| **Same-event order** | Parallel | Parallel (default) or sequential | Unknown | System > User > Workspace | Sequential | Sequential | Unknown | Sequential (shared output object) |
| **Output chaining** | No (parallel) | Yes (sequential mode) | Unknown | Unknown | No | Unknown | Unknown | Yes (mutable output) |
| **Default timeout** | 600s (command) / 30s (prompt/http) / 60s (agent) | 60s (ms unit) | Unknown | Unknown | 30s | 30s | 30s (ms unit) | None (in-process) |
| **Timeout signal** | Unknown | SIGTERM → 5s → SIGKILL | Unknown | Unknown | Unknown | Unknown | Unknown | N/A |
| **Error handling** | Non-blocking, fail-open | Non-fatal, fail-open | Fail-open (failClosed option) | Unknown | Unknown | Unknown | Unknown | Exception propagates |
| **Env vars injected** | CLAUDE_PROJECT_DIR + 4 more | GEMINI_PROJECT_DIR + CLAUDE_PROJECT_DIR (compat) + sanitized parent | CURSOR_PROJECT_DIR + 5 more | ROOT_WORKSPACE_PATH (worktree only) | Inherited + per-hook env | Per-hook env only | Unknown | N/A (in-process context) |
| **Shell** | bash (configurable) | bash -c / powershell (explicit, shell:false) | Unknown | Any executable | System default | bash / powershell | Unknown | Bun import() |

---

## 5. Security Posture Summary

| Agent | Provenance Controls | Sandbox | Env Protection | Auto-Execute from Repo |
|-------|-------------------|---------|---------------|----------------------|
| Claude Code | Source labels (informational) | None | HTTP hooks: allowedEnvVars | Yes |
| Gemini CLI | TrustedHooksManager (opt-in) | Yes (5 methods) | Deny-list (opt-in locally) | Yes (default), No (with folderTrust) |
| Cursor | Workspace trust (coarse) | None | Session-scoped injection | Yes (after trust granted) |
| Windsurf | None | None | None | Yes |
| VS Code Copilot | None | None | Per-hook env field | Yes |
| Copilot CLI | None | None (ephemeral runner provides run isolation, not in-run sandboxing) | Per-hook env field | Yes (mitigated by runner ephemerality) |
| Kiro | None documented | None | Unknown | Unknown |
| OpenCode | Tool permission rules | None (in-process) | shell.env hook | N/A (no hook injection) |

---

## 6. Non-Spec Agents Summary

| Agent | Events | Config Format | Blocking | Unique Capabilities |
|-------|--------|--------------|----------|-------------------|
| Amazon Q Developer | 5 | JSON in agent config | Exit code 2 (preToolUse only) | Output caching (cache_ttl_seconds), MCP namespace matchers (@git), persistent agentSpawn injection |
| Cline | 8 | Directory-based auto-discovery | JSON `cancel: true` | PostToolUse blocking, contextModification, PreCompact event, directory auto-discovery |
| Augment Code | 5 | settings.json (3 tiers) | Exit code 2 + JSON decisions | System-level immutable hooks, privacy opt-in data model, Stop blocking |
| Roo Code | 0 | N/A | N/A | No hook system |
| gptme | 22 | Python functions (in-process) | StopPropagation / ConfirmAction | tool.confirm EDIT option, loop.continue, message.transform, cwd.changed |
| Pi Agent | 20+ | TypeScript via jiti (in-process) | return {block: true} | before_provider_request (LLM payload replacement), tool_result modification, interactive UI from hooks |
