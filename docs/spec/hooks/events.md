# Event Registry

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines the canonical event names and their provider-native mappings
> referenced by the core spec's matcher and conversion pipeline sections.

**Registry Version:** 2026.04
**Last Modified:** 2026-04-08
**Status:** Initial Development

---

Canonical event names use `snake_case` and describe the lifecycle moment in provider-neutral terms.

## §1 Core Events

Core events have near-universal provider support. Conforming implementations at the Core level (Section 13) MUST support all core events.

| Event | Description | Blocking Semantic |
|-------|-------------|-------------------|
| `before_tool_execute` | Fires before any tool runs. The hook receives the tool name and input arguments. | Can prevent the tool from executing. |
| `after_tool_execute` | Fires after a tool completes. The hook receives the tool name, input, and result. | Observational only; the action has already occurred. Setting `blocking: true` on an observe-only event is not a validation error, but implementations SHOULD warn that the blocking intent has no effect. |
| `session_start` | Fires when a coding session begins, resumes, or resets. | Observational; may inject context. |
| `session_end` | Fires when a session terminates. Best-effort delivery; the process may exit before the hook completes. | Observational. |
| `before_prompt` | Fires after user input is submitted but before it reaches the agent. | Can modify or reject user input. |
| `agent_stop` | Fires when the agent's main loop ends. | Can trigger a retry (provider-dependent). |

## §2 Extended Events

Extended events have partial provider support. They appear in the event registry but are not required for Core conformance.

| Event | Description |
|-------|-------------|
| `before_compact` | Fires before context window compression. |
| `after_compact` | Fires after context window compression completes. Currently claude-code only; included in §2 for anticipated adoption. |
| `notification` | Non-blocking system notification (e.g., permission prompts, status updates). |
| `error_occurred` | Fires when the agent encounters an error. |
| `tool_use_failure` | Fires when a tool invocation fails. Distinct from `after_tool_execute` in that it signals an error, not a successful completion. |
| `file_changed` | Fires when a file is created, modified, or saved in the project. |
| `subagent_start` | Fires when a nested agent is spawned. |
| `subagent_stop` | Fires when a nested agent finishes. |
| `permission_request` | Fires when a sensitive action requires permission. |
| `permission_denied` | Fires when permission was automatically denied (as opposed to `permission_request` which fires to ask). Currently claude-code only; included in §2 as it represents a generic lifecycle moment. |
| `before_model` | Fires before an LLM API call. Hook can intercept or mock the request. |
| `after_model` | Fires after an LLM response is received. Hook can redact or modify. |
| `before_tool_selection` | Fires before the LLM chooses which tool to use. Hook can filter the tool list. |

## §3 Provider-Exclusive Events

Provider-exclusive events exist in only one provider. They are included in the registry for lossless round-tripping but are expected to be dropped or degraded during cross-provider conversion.

| Event | Description | Origin Provider(s) |
|-------|-------------|-------------------|
| `config_change` | Fires when configuration is modified. | Claude Code |
| `instructions_loaded` | Fires when memory or rules files are loaded into context. Has `file_path`, `memory_type`, `load_reason` fields. | Claude Code |
| `task_created` | Fires when a new task is created via TaskCreate tool. Blocking (exit 2 rolls back task creation). Multi-agent team feature. | Claude Code |
| `task_completed` | Fires when a task is marked complete. Blocking. Multi-agent team feature. | Claude Code |
| `teammate_idle` | Fires when a teammate agent in a multi-agent team goes idle. Blocking. | Claude Code |
| `cwd_changed` | Fires when the working directory changes. Observational; supports `CLAUDE_ENV_FILE` env persistence. | Claude Code |
| `elicitation` | Fires when an MCP server requests user input. Blocking; has `server_name`, `form_schema`, `form_values` fields. Hook can respond with `action` (accept/decline/cancel). | Claude Code |
| `elicitation_result` | Fires when the user responds to an MCP elicitation. Blocking; hook can override response values. | Claude Code |
| `file_created` | Fires when a new file is created in the project. | Kiro |
| `file_deleted` | Fires when a file is deleted from the project. | Kiro |
| `before_task` | Fires before a spec task executes. | Kiro |
| `after_task` | Fires after a spec task completes. | Kiro |
| `manual_trigger` | Fires when a hook is manually triggered by the user (not tied to an agent lifecycle event). | Kiro |
| `windsurf_transcript_response` | Post-response event that provides a `transcript_path` (JSONL file) with the full response. Enterprise compliance variant of `agent_stop`. | Windsurf |
| `windsurf_worktree_setup` | Fires after worktree creation; has `worktree_path` and `root_workspace_path` fields. | Windsurf |
| `opencode_command_before` | Fires before a shell command executes (OpenCode plugin system). | OpenCode |
| `opencode_command_after` | Fires after a shell command executes. | OpenCode |
| `opencode_chat_params` | Fires before an LLM API call; allows modifying LLM request parameters. | OpenCode |
| `opencode_chat_headers` | Fires to inject or modify HTTP headers on LLM API calls. | OpenCode |
| `opencode_shell_env` | Fires to inject environment variables for shell execution. | OpenCode |
| `opencode_tool_definition` | Fires to modify tool definitions sent to the LLM. | OpenCode |
| `opencode_tui_events` | TUI-specific events: `tui.prompt.append`, `tui.command.execute`. | OpenCode |
| `cursor_agent_thought` | Fires after the agent produces a thinking/reasoning block. Observe only. Requires a thinking model (does not fire with standard models). | Cursor |
| `cursor_tab_file_read` | Fires before a tab file is read (semantics unclear from available docs). | Cursor |
| `cursor_tab_file_edit` | Fires after a tab file is edited. | Cursor |

## §4 Event Name Mapping

The following table maps canonical event names to provider-native names. Adapters use this mapping during decode (native to canonical) and encode (canonical to native).

| Canonical | claude-code | gemini-cli | cursor | windsurf | vs-code-copilot | copilot-cli | kiro | opencode | factory-droid | codex | cline |
|-----------|-------------|------------|--------|----------|-----------------|-------------|------|----------|---------------|-------|-------|
| `before_tool_execute` | PreToolUse | BeforeTool | beforeShellExecution / beforeMCPExecution / beforeReadFile | pre_read_code / pre_write_code / pre_run_command / pre_mcp_tool_use | PreToolUse | preToolUse | preToolUse | tool.execute.before | PreToolUse | PreToolUse | PreToolUse |
| `after_tool_execute` | PostToolUse | AfterTool | afterShellExecution / afterMCPExecution / afterFileEdit | post_read_code / post_write_code / post_run_command / post_mcp_tool_use | PostToolUse | postToolUse | postToolUse | tool.execute.after | PostToolUse | PostToolUse | PostToolUse |
| `session_start` | SessionStart | SessionStart | sessionStart | -- | SessionStart | sessionStart | -- | session.created | SessionStart | SessionStart | TaskStart / TaskResume (merged) |
| `session_end` | SessionEnd | SessionEnd | sessionEnd | -- | -- | sessionEnd | -- | session.deleted | SessionEnd | -- | TaskCancel (partial) |
| `before_prompt` | UserPromptSubmit | BeforeAgent | beforeSubmitPrompt | pre_user_prompt | UserPromptSubmit | userPromptSubmitted | userPromptSubmit | -- | UserPromptSubmit | UserPromptSubmit | UserPromptSubmit |
| `agent_stop` | Stop | AfterAgent | stop | post_cascade_response | Stop | -- | Agent Stop | session.idle | Stop | Stop | TaskComplete |
| `before_compact` | PreCompact | PreCompress | -- | -- | PreCompact | -- | -- | experimental.session.compacting | PreCompact | -- | PreCompact |
| `notification` | Notification | Notification | -- | -- | -- | -- | -- | -- | -- | -- | Notification |
| `error_occurred` | StopFailure | -- | -- | -- | -- | errorOccurred | -- | session.error | -- | -- | -- |
| `tool_use_failure` | PostToolUseFailure | -- | postToolUseFailure | -- | -- | errorOccurred | -- | -- | -- | -- | -- |
| `file_changed` | FileChanged | -- | afterFileEdit | -- | -- | -- | File Save | file.edited | -- | -- | -- |
| `subagent_start` | SubagentStart | -- | subagentStart | -- | SubagentStart | -- | -- | -- | SubagentStart | -- | -- |
| `subagent_stop` | SubagentStop | -- | subagentStop | -- | SubagentStop | -- | -- | -- | SubagentStop | -- | -- |
| `permission_request` | PermissionRequest | -- | -- | -- | -- | -- | -- | permission.asked | -- | -- | -- |
| `before_model` | -- | BeforeModel | beforeAgentResponse | -- | -- | -- | -- | -- | -- | -- | -- |
| `after_model` | -- | AfterModel | afterAgentResponse | -- | -- | -- | -- | -- | -- | -- | -- |
| `before_tool_selection` | -- | BeforeToolSelection | beforeToolSelection | -- | -- | -- | -- | -- | -- | -- | -- |
| `config_change` | ConfigChange | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `file_created` | -- | -- | -- | -- | -- | -- | File Create | -- | -- | -- | -- |
| `file_deleted` | -- | -- | -- | -- | -- | -- | File Delete | -- | -- | -- | -- |
| `before_task` | -- | -- | -- | -- | -- | -- | Pre Task Execution | -- | -- | -- | -- |
| `after_task` | -- | -- | -- | -- | -- | -- | Post Task Execution | -- | -- | -- | -- |
| `instructions_loaded` | InstructionsLoaded | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `permission_denied` | PermissionDenied | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `task_created` | TaskCreated | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `task_completed` | TaskCompleted | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `teammate_idle` | TeammateIdle | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `cwd_changed` | CwdChanged | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `after_compact` | PostCompact | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `elicitation` | Elicitation | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `elicitation_result` | ElicitationResult | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `manual_trigger` | -- | -- | -- | -- | -- | -- | Manual Trigger | -- | -- | -- | -- |
| `windsurf_transcript_response` | -- | -- | -- | post_cascade_response_with_transcript | -- | -- | -- | -- | -- | -- | -- |
| `windsurf_worktree_setup` | -- | -- | -- | post_setup_worktree | -- | -- | -- | -- | -- | -- | -- |
| `opencode_command_before` | -- | -- | -- | -- | -- | -- | -- | command.execute.before | -- | -- | -- |
| `opencode_command_after` | -- | -- | -- | -- | -- | -- | -- | command.execute.after | -- | -- | -- |
| `opencode_chat_params` | -- | -- | -- | -- | -- | -- | -- | chat.params | -- | -- | -- |
| `opencode_chat_headers` | -- | -- | -- | -- | -- | -- | -- | chat.headers | -- | -- | -- |
| `opencode_shell_env` | -- | -- | -- | -- | -- | -- | -- | shell.env | -- | -- | -- |
| `opencode_tool_definition` | -- | -- | -- | -- | -- | -- | -- | tool.definition | -- | -- | -- |
| `opencode_tui_events` | -- | -- | -- | -- | -- | -- | -- | tui.prompt.append / tui.command.execute | -- | -- | -- |
| `cursor_agent_thought` | -- | -- | afterAgentThought | -- | -- | -- | -- | -- | -- | -- | -- |
| `cursor_tab_file_read` | -- | -- | beforeTabFileRead | -- | -- | -- | -- | -- | -- | -- | -- |
| `cursor_tab_file_edit` | -- | -- | afterTabFileEdit | -- | -- | -- | -- | -- | -- | -- | -- |

A `--` indicates the provider does not support that event. When encoding a hook for a provider that does not support its event, the adapter MUST apply the degradation strategy (Section 11).

**Split-event providers:** Cursor and Windsurf map a single `before_tool_execute` event to multiple provider-native events based on the matcher. When encoding for these providers, adapters MUST inspect the `matcher` field to select the correct native event. When decoding from these providers, adapters MUST merge split events into `before_tool_execute` with an appropriate matcher.

**Footnotes:**

- **kiro event name casing:** Kiro event name casing is unverified — official docs describe events in display form ("Pre Tool Use", "Agent Stop") and URL slugs use kebab-case (`pre-tool-use`), while the provider-formats reference uses camelCase (`preToolUse`). Implementations should accept both kebab-case and camelCase when decoding from Kiro.
- **copilot-cli dual-mapping:** Both `error_occurred` and `tool_use_failure` map to copilot-cli's `errorOccurred`. This is lossy but functional — copilot-cli has one error event covering both scenarios. Round-tripping via copilot-cli will merge these two distinct canonical events into one.
- **cline session mapping:** `TaskStart` and `TaskResume` both decode to `session_start`. On encode, a single `session_start` hook is written as `TaskStart` only (lossy — no resume variant generated). `TaskCancel` is the closest cline equivalent to `session_end` but fires specifically on user cancellation, not natural session termination.
- **cline architecture:** Cline uses script-file-based hooks with no JSON manifest. Hook scripts are discovered by filename matching event names (e.g., `PreToolUse` or `PreToolUse.ps1`). Cross-provider conversion to/from cline is lossy — task-lifecycle events (`TaskStart`, `TaskResume`, `TaskCancel`, `TaskComplete`) have no canonical equivalents and are merged into broader canonical events.
- **cursor `sessionStart` blocking bug:** `{"continue": false}` is silently ignored as of at least v2.4.21 — the session creation proceeds regardless. The event is classified as blocking per spec (see blocking-matrix.md) but blocking is currently broken in Cursor. Implementations targeting Cursor SHOULD warn when encoding a blocking `session_start` hook that the blocking intent may not be honored.
- **cursor `after_tool_execute` split model:** Cursor maps `after_tool_execute` to three native events: `afterShellExecution` (shell commands), `afterMCPExecution` (MCP tool calls), and `afterFileEdit` (file edits). This mirrors the split model used for `before_tool_execute`. Adapters MUST inspect the matcher field when encoding, and MUST merge all three when decoding.
- **cursor `beforeSubmitPrompt` blocking limits:** `beforeSubmitPrompt` supports only `{"continue": true/false}` — the `userMessage`, `agentMessage`, and `permission` response fields are not supported. A known bug causes blocked messages (when `continue: false`) to remain in conversation history sent to the LLM.

## Provider Support Matrix

The following table is auto-generated from `docs/provider-capabilities/*.yaml`. Do not edit by hand — run `syllago capmon generate` to refresh.

<!-- GENERATED FROM provider-capabilities/*.yaml -->
<!-- END GENERATED -->
