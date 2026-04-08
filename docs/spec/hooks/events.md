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
| `notification` | Non-blocking system notification (e.g., permission prompts, status updates). |
| `error_occurred` | Fires when the agent encounters an error. |
| `tool_use_failure` | Fires when a tool invocation fails. Distinct from `after_tool_execute` in that it signals an error, not a successful completion. |
| `file_changed` | Fires when a file is created, modified, or saved in the project. |
| `subagent_start` | Fires when a nested agent is spawned. |
| `subagent_stop` | Fires when a nested agent finishes. |
| `permission_request` | Fires when a sensitive action requires permission. |
| `before_model` | Fires before an LLM API call. Hook can intercept or mock the request. |
| `after_model` | Fires after an LLM response is received. Hook can redact or modify. |
| `before_tool_selection` | Fires before the LLM chooses which tool to use. Hook can filter the tool list. |

## §3 Provider-Exclusive Events

Provider-exclusive events exist in only one provider. They are included in the registry for lossless round-tripping but are expected to be dropped or degraded during cross-provider conversion.

| Event | Description | Origin Provider(s) |
|-------|-------------|-------------------|
| `config_change` | Fires when configuration is modified. | Claude Code |
| `file_created` | Fires when a new file is created in the project. | Kiro |
| `file_deleted` | Fires when a file is deleted from the project. | Kiro |
| `before_task` | Fires before a spec task executes. | Kiro |
| `after_task` | Fires after a spec task completes. | Kiro |

## §4 Event Name Mapping

The following table maps canonical event names to provider-native names. Adapters use this mapping during decode (native to canonical) and encode (canonical to native).

| Canonical | claude-code | gemini-cli | cursor | windsurf | vs-code-copilot | copilot-cli | kiro | opencode |
|-----------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| `before_tool_execute` | PreToolUse | BeforeTool | beforeShellExecution / beforeMCPExecution / beforeReadFile | pre_read_code / pre_write_code / pre_run_command / pre_mcp_tool_use | PreToolUse | preToolUse | preToolUse | tool.execute.before |
| `after_tool_execute` | PostToolUse | AfterTool | afterFileEdit | post_read_code / post_write_code / post_run_command / post_mcp_tool_use | PostToolUse | postToolUse | postToolUse | tool.execute.after |
| `session_start` | SessionStart | SessionStart | -- | -- | SessionStart | sessionStart | agentSpawn | session.created |
| `session_end` | SessionEnd | SessionEnd | -- | -- | -- | sessionEnd | stop | -- |
| `before_prompt` | UserPromptSubmit | BeforeAgent | beforeSubmitPrompt | pre_user_prompt | UserPromptSubmit | userPromptSubmitted | userPromptSubmit | -- |
| `agent_stop` | Stop | AfterAgent | stop | post_cascade_response | Stop | -- | stop | session.idle |
| `before_compact` | PreCompact | PreCompress | -- | -- | PreCompact | -- | -- | -- |
| `notification` | Notification | Notification | -- | -- | -- | -- | -- | -- |
| `error_occurred` | StopFailure | -- | -- | -- | -- | errorOccurred | -- | session.error |
| `tool_use_failure` | PostToolUseFailure | -- | postToolUseFailure | -- | -- | errorOccurred | -- | -- |
| `file_changed` | FileChanged | -- | afterFileEdit | -- | -- | -- | File Save | file.edited |
| `subagent_start` | SubagentStart | -- | subagentStart | -- | SubagentStart | -- | -- | -- |
| `subagent_stop` | SubagentStop | -- | subagentStop | -- | SubagentStop | -- | -- | -- |
| `permission_request` | PermissionRequest | -- | -- | -- | -- | -- | -- | permission.asked |
| `before_model` | -- | BeforeModel | beforeAgentResponse | -- | -- | -- | -- | -- |
| `after_model` | -- | AfterModel | afterAgentResponse | -- | -- | -- | -- | -- |
| `before_tool_selection` | -- | BeforeToolSelection | beforeToolSelection | -- | -- | -- | -- | -- |
| `config_change` | ConfigChange | -- | -- | -- | -- | -- | -- | -- |
| `file_created` | -- | -- | -- | -- | -- | -- | File Create | -- |
| `file_deleted` | -- | -- | -- | -- | -- | -- | File Delete | -- |
| `before_task` | -- | -- | -- | -- | -- | -- | Pre Task Execution | -- |
| `after_task` | -- | -- | -- | -- | -- | -- | Post Task Execution | -- |

A `--` indicates the provider does not support that event. When encoding a hook for a provider that does not support its event, the adapter MUST apply the degradation strategy (Section 11).

**Split-event providers:** Cursor and Windsurf map a single `before_tool_execute` event to multiple provider-native events based on the matcher. When encoding for these providers, adapters MUST inspect the `matcher` field to select the correct native event. When decoding from these providers, adapters MUST merge split events into `before_tool_execute` with an appropriate matcher.
