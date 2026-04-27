# Codex CLI Built-in Tools

Reference for all built-in tools in OpenAI's Codex CLI coding agent.
Source: [`codex-rs/core/src/tools/spec.rs`](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs) [Official]

Last updated: 2026-03-21

---

## Tool Inventory

| # | Tool Name | Category | Feature-Gated | Cross-Provider Equivalent |
|---|-----------|----------|---------------|--------------------------|
| 1 | `shell` | Execution | Default shell mode | Claude Code `Bash`, Cursor `terminal` |
| 2 | `shell_command` | Execution | ShellCommand mode | Claude Code `Bash` |
| 3 | `exec_command` | Execution | UnifiedExec mode | Claude Code `Bash` |
| 4 | `write_stdin` | Execution | UnifiedExec mode | (none) |
| 5 | `apply_patch` | File Editing | ApplyPatchFreeform | Claude Code `Edit`/`Write` |
| 6 | `read_file` | File Reading | Experimental | Claude Code `Read` |
| 7 | `list_dir` | File System | Experimental | Claude Code `Glob` (partial) |
| 8 | `grep_files` | Search | Experimental | Claude Code `Grep` |
| 9 | `view_image` | File Reading | Always on | Claude Code `Read` (image mode) |
| 10 | `update_plan` | Planning | Always on | (none -- Claude Code has no plan tool) |
| 11 | `web_search` | Search | WebSearch config | Claude Code `WebSearch` |
| 12 | `image_generation` | Media | ImageGeneration feature | (none) |
| 13 | `spawn_agent` | Multi-Agent | Collab feature | Claude Code `Agent` (Task tool) |
| 14 | `send_input` | Multi-Agent | Collab feature | (none) |
| 15 | `wait_agent` | Multi-Agent | Collab feature | (none) |
| 16 | `close_agent` | Multi-Agent | Collab feature | (none) |
| 17 | `resume_agent` | Multi-Agent | Collab (v1 only) | (none) |
| 18 | `spawn_agents_on_csv` | Batch Jobs | SpawnCsv feature | (none) |
| 19 | `report_agent_job_result` | Batch Jobs | SpawnCsv (worker) | (none) |
| 20 | `request_user_input` | Interaction | Not in subagents | Claude Code `AskUserQuestion` |
| 21 | `request_permissions` | Permissions | RequestPermissionsTool | (none) |
| 22 | `tool_search` | Discovery | Model supports it | Claude Code `ToolSearch` |
| 23 | `tool_suggest` | Discovery | ToolSuggest feature | (none) |
| 24 | `js_repl` | Code Execution | JsRepl feature | (none) |
| 25 | `js_repl_reset` | Code Execution | JsRepl feature | (none) |
| 26 | `artifacts` | Code Execution | Artifact feature | (none) |
| 27 | `codex` (code mode) | Code Execution | CodeMode feature | (none) |
| 28 | `wait` (code mode) | Code Execution | CodeMode feature | (none) |
| 29 | `list_mcp_resources` | MCP | MCP tools present | (none) |
| 30 | `list_mcp_resource_templates` | MCP | MCP tools present | (none) |
| 31 | `read_mcp_resource` | MCP | MCP tools present | (none) |
| 32 | `test_sync_tool` | Internal | Experimental | (none -- test-only) |

---

## Detailed Tool Definitions

### 1. `shell` [Official]

**Purpose:** Execute shell commands via `execvp()` (Unix) or `CreateProcessW()` (Windows).

**When active:** Default shell mode (`ConfigShellToolType::Default`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | `string[]` | Yes | The command to execute (e.g. `["bash", "-lc", "ls"]`) |
| `workdir` | `string` | No | Working directory for execution |
| `timeout_ms` | `number` | No | Timeout in milliseconds |

Also accepts optional approval parameters when `exec_permission_approvals_enabled`.

Source: [`spec.rs` L846-908](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L846) [Official]

---

### 2. `shell_command` [Official]

**Purpose:** Run a shell script string in the user's default shell. Simpler interface than `shell` -- takes a single command string instead of an argv array.

**When active:** ShellCommand or ZshFork shell mode.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | `string` | Yes | Shell script to execute |
| `workdir` | `string` | No | Working directory |
| `timeout_ms` | `number` | No | Timeout in milliseconds |
| `login` | `boolean` | No | Run with login shell semantics (default: true) |

Source: [`spec.rs` L909-985](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L909) [Official]

---

### 3. `exec_command` [Official]

**Purpose:** Run a command in a PTY, returning output or a session ID for ongoing interaction. The most capable shell variant -- supports TTY allocation, yield timing, and token limits.

**When active:** UnifiedExec shell mode.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cmd` | `string` | Yes | Shell command to execute |
| `workdir` | `string` | No | Working directory (defaults to turn cwd) |
| `shell` | `string` | No | Shell binary to launch |
| `tty` | `boolean` | No | Allocate a TTY (default: false) |
| `yield_time_ms` | `number` | No | Wait time before yielding output |
| `max_output_tokens` | `number` | No | Max tokens to return (excess truncated) |
| `login` | `boolean` | No | Login shell semantics (default: true) |

**Output schema:** `{ wall_time_seconds, output, exit_code?, session_id?, chunk_id?, original_token_count? }`

Source: [`spec.rs` L658-746](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L658) [Official]

---

### 4. `write_stdin` [Official]

**Purpose:** Write characters to an existing `exec_command` session and return recent output. Used for interactive processes.

**When active:** UnifiedExec shell mode (paired with `exec_command`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `session_id` | `number` | Yes | ID of the running exec session |
| `chars` | `string` | No | Bytes to write to stdin (empty to poll) |
| `yield_time_ms` | `number` | No | Wait time before yielding |
| `max_output_tokens` | `number` | No | Max tokens to return |

**Output schema:** Same as `exec_command`.

Source: [`spec.rs` L747-795](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L747) [Official]

---

### 5. `apply_patch` [Official]

**Purpose:** Modify or create files using a unified-diff-style patch format. Two variants exist: freeform (raw patch text) and JSON (structured input).

**When active:** Feature-gated by `ApplyPatchFreeform` or model config.

**Freeform variant:** Raw patch text sent directly (not JSON-wrapped). Format:
```
*** Begin Patch
*** Update File: path/to/file.py
@@ def example():
- pass
+ return 123
*** End Patch
```

**JSON variant:** `{ "input": "<patch text>" }`

Defined in: [`handlers/apply_patch.rs`](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/handlers/apply_patch.rs) [Official]

---

### 6. `read_file` [Official]

**Purpose:** Read a local file with 1-indexed line numbers. Supports simple slice mode and indentation-aware block mode.

**When active:** Experimental (model must list `"read_file"` in `experimental_supported_tools`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file_path` | `string` | Yes | Absolute path to file |
| `offset` | `number` | No | Line number to start from (>= 1) |
| `limit` | `number` | No | Max lines to return |
| `mode` | `string` | No | `"slice"` (default) or `"indentation"` |
| `indentation` | `object` | No | Indentation-mode options (see below) |

**Indentation sub-parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `anchor_line` | `number` | Center line for indentation lookup |
| `max_levels` | `number` | Parent indentation levels to include |
| `include_siblings` | `boolean` | Include blocks at same indent level |
| `include_header` | `boolean` | Include doc comments/attributes above block |
| `max_lines` | `number` | Hard cap on lines returned |

Source: [`spec.rs` L1931-2035](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1931) [Official]

---

### 7. `list_dir` [Official]

**Purpose:** List entries in a local directory with 1-indexed entry numbers and type labels.

**When active:** Experimental (`"list_dir"` in `experimental_supported_tools`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `dir_path` | `string` | Yes | Absolute path to directory |
| `offset` | `number` | No | Entry number to start from (>= 1) |
| `limit` | `number` | No | Max entries to return |
| `depth` | `number` | No | Max directory depth to traverse (>= 1) |

Source: [`spec.rs` L2036-2083](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2036) [Official]

---

### 8. `grep_files` [Official]

**Purpose:** Find files whose contents match a regex pattern, listed by modification time.

**When active:** Experimental (`"grep_files"` in `experimental_supported_tools`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `pattern` | `string` | Yes | Regex pattern to search for |
| `include` | `string` | No | Glob to limit searched files (e.g. `"*.rs"`) |
| `path` | `string` | No | Directory/file to search (defaults to cwd) |
| `limit` | `number` | No | Max file paths to return (default: 100) |

Source: [`spec.rs` L1671-1723](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1671) [Official]

---

### 9. `view_image` [Official]

**Purpose:** View a local image file from the filesystem. Returns a data URL of the loaded image.

**When active:** Always registered.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | `string` | Yes | Local filesystem path to image file |
| `detail` | `string` | No | Set to `"original"` for full resolution (when supported) |

**Output schema:** `{ image_url: string, detail: string | null }`

Source: [`spec.rs` L986-1033](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L986) [Official]

---

### 10. `update_plan` [Official]

**Purpose:** Create and track a step-by-step execution plan. Demonstrates task understanding and communicates approach to the user.

**When active:** Always registered.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `explanation` | `string` | No | Explanation when updating plan |
| `plan` | `array` | No | List of step objects |

**Plan item schema:** `{ step: string, status: "pending" | "in_progress" | "completed" }`

Rules: Exactly one step should be `in_progress` at a time. Steps should be 1-5-7 words each.

Source: [`handlers/plan.rs`](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/handlers/plan.rs) [Official]
System prompt: [`prompt.md`](https://github.com/openai/codex/blob/main/codex-rs/core/prompt.md) [Official]

---

### 11. `web_search` [Official]

**Purpose:** Search the web for up-to-date information. This is a first-party OpenAI tool type (not a function call), sent as `{ "type": "web_search_preview" }` in the API.

**When active:** Controlled by `web_search` config: `"cached"` (default for local), `"live"`, or `"disabled"`.

**Configuration parameters (not call-time):**

| Config | Description |
|--------|-------------|
| `external_web_access` | `true` for live, `false` for cached |
| `filters` | Domain include/exclude filters |
| `user_location` | Location hint for search |
| `search_context_size` | Token budget for search context |
| `search_content_types` | `["text"]` or `["text", "image"]` |

This tool is invoked implicitly by the model -- there are no explicit call-time parameters the agent provides. The model decides when to search based on the conversation.

Source: [`spec.rs` L2909-2953](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2932) [Official]
Docs: [Features - Codex CLI](https://developers.openai.com/codex/cli/features) [Official]

---

### 12. `image_generation` [Official]

**Purpose:** Generate images. Sent as `{ "type": "image_generation" }` in the API.

**When active:** `ImageGeneration` feature flag + model supports image input.

**Configuration:** `output_format: "png"`

Like `web_search`, this is an OpenAI-native tool type, not a function call.

Source: [`spec.rs` L2956](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2956) [Official]

---

### 13. `spawn_agent` [Official]

**Purpose:** Spawn a sub-agent for a well-scoped, parallelizable task. Returns the agent's task name and/or ID.

**When active:** `Collab` feature flag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `message` | `string` | No | Initial plain-text task (use `message` or `items`) |
| `items` | `array` | No | Structured input items (text, image, skill, mention) |
| `agent_type` | `string` | No | Agent role (from configured `agent_roles`) |
| `fork_context` | `boolean` | No | Fork current thread history into new agent |
| `model` | `string` | No | Model override for the sub-agent |
| `reasoning_effort` | `string` | No | Reasoning effort override |
| `task_name` | `string` | No | Canonical task name (lowercase, digits, underscores) |

**Output schema:** `{ agent_id: string?, task_name: string?, nickname: string? }`

Source: [`spec.rs` L1086-1224](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1086) [Official]

---

### 14. `send_input` [Official]

**Purpose:** Send a message to an existing agent. Supports interrupt mode to redirect work immediately.

**When active:** `Collab` feature flag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target` | `string` | Yes | Agent ID or canonical task name |
| `message` | `string` | No | Plain-text message (legacy; use `message` or `items`) |
| `items` | `array` | No | Structured input items |
| `interrupt` | `boolean` | No | Stop current task and handle immediately (default: false) |

**Output schema:** `{ submission_id: string }`

Source: [`spec.rs` L1354-1399](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1354) [Official]

---

### 15. `wait_agent` [Official]

**Purpose:** Wait for one or more agents to reach a final status. Returns completed statuses or times out.

**When active:** `Collab` feature flag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `targets` | `string[]` | Yes | Agent IDs or task names to wait on |
| `timeout_ms` | `number` | No | Timeout in ms (default/min/max configured server-side) |

**Output schema:** `{ status: { [id]: AgentStatus }, timed_out: boolean }`

Where `AgentStatus` is one of: `"pending_init"`, `"running"`, `"shutdown"`, `"not_found"`, `{ completed: string? }`, `{ errored: string }`.

Source: [`spec.rs` L1425-1460](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1425) [Official]

---

### 16. `close_agent` [Official]

**Purpose:** Close an agent and any open descendants, returning the agent's previous status.

**When active:** `Collab` feature flag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target` | `string` | Yes | Agent ID or canonical task name |

**Output schema:** `{ previous_status: AgentStatus }`

Source: [`spec.rs` L1577-1601](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1577) [Official]

---

### 17. `resume_agent` [Official]

**Purpose:** Resume a previously closed agent so it can receive `send_input` and `wait_agent` calls again.

**When active:** `Collab` feature flag, v1 multi-agent only (excluded in `MultiAgentV2`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Agent ID to resume |

**Output schema:** `{ status: AgentStatus }`

Source: [`spec.rs` L1400-1424](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1400) [Official]

---

### 18. `spawn_agents_on_csv` [Official]

**Purpose:** Batch-process a CSV file by spawning one worker sub-agent per row. Blocks until all rows finish and exports results.

**When active:** `SpawnCsv` feature flag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `csv_path` | `string` | Yes | Path to input CSV |
| `instruction` | `string` | Yes | Template with `{column_name}` placeholders |
| `id_column` | `string` | No | Column name for stable item ID |
| `output_csv_path` | `string` | No | Output CSV path for results |
| `max_concurrency` | `number` | No | Max concurrent workers (default: 16) |
| `max_workers` | `number` | No | Alias for max_concurrency |
| `max_runtime_seconds` | `number` | No | Max runtime per worker (default: 1800) |
| `output_schema` | `object` | No | JSON schema for worker results |

Source: [`spec.rs` L1225-1302](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1225) [Official]

---

### 19. `report_agent_job_result` [Official]

**Purpose:** Worker-only tool to report a result for a batch job item. Main agents should not call this.

**When active:** `SpawnCsv` feature flag, worker subagent context only.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job identifier |
| `item_id` | `string` | Yes | Job item identifier |
| `result` | `object` | Yes | Result data (matches output_schema) |
| `stop` | `boolean` | No | Cancel remaining items after recording |

Source: [`spec.rs` L1303-1353](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1303) [Official]

---

### 20. `request_user_input` [Official]

**Purpose:** Present structured multiple-choice questions to the user with labeled options.

**When active:** Enabled for main agents, disabled for subagents.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `questions` | `array` | Yes | Questions to show (prefer 1, max 3) |

**Question schema:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | Yes | Stable snake_case identifier |
| `header` | `string` | Yes | UI header (<= 12 chars) |
| `question` | `string` | Yes | Single-sentence prompt |
| `options` | `array` | Yes | 2-3 mutually exclusive choices |

**Option schema:** `{ label: string, description: string }`

Source: [`spec.rs` L1461-1547](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1461) [Official]

---

### 21. `request_permissions` [Official]

**Purpose:** Request elevated sandbox permissions at runtime (filesystem paths, network access).

**When active:** `RequestPermissionsTool` feature flag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `reason` | `string` | No | Short explanation for why permissions are needed |
| `permissions` | `object` | Yes | Permission request (see sub-schema) |

**Permissions sub-schema** includes `network` and `filesystem` objects for requesting specific access grants.

Source: [`spec.rs` L1548-1576](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1548) [Official]

---

### 22. `tool_search` [Official]

**Purpose:** Search for available tools from connected apps/MCP servers by keyword query.

**When active:** Model supports search tool + app tools configured.

**Type:** `ToolSpec::ToolSearch` (special spec type with `execution: "client"`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | `string` | Yes | Search query for app tools |
| `limit` | `number` | No | Max tools to return (default: configured limit) |

Source: [`spec.rs` L1724-1803](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1724) [Official]

---

### 23. `tool_suggest` [Official]

**Purpose:** Suggest a discoverable tool (connector or plugin) that could help with the current task.

**When active:** `ToolSuggest` feature flag + discoverable tools available.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tool_type` | `string` | Yes | `"connector"` or `"plugin"` |
| `action_type` | `string` | Yes | `"install"` or `"enable"` |
| `tool_id` | `string` | Yes | Connector/plugin ID from known list |
| `suggest_reason` | `string` | Yes | One-line user-facing reason |

Source: [`spec.rs` L1804-1930](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1804) [Official]

---

### 24. `js_repl` [Official]

**Purpose:** Run JavaScript in a persistent Node.js kernel with top-level await support.

**When active:** `JsRepl` feature flag.

**Type:** Freeform tool (not JSON -- send raw JS source text).

**Input format:** Raw JavaScript, optionally with a first-line pragma: `// codex-js-repl: timeout_ms=15000`

Do NOT send JSON, quoted strings, or markdown fences.

Source: [`spec.rs` L2084-2115](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2084) [Official]

---

### 25. `js_repl_reset` [Official]

**Purpose:** Restart the `js_repl` kernel and clear persisted top-level bindings.

**When active:** `JsRepl` feature flag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (none) | | | No parameters |

Source: [`spec.rs` L2143-2159](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2143) [Official]

---

### 26. `artifacts` [Official]

**Purpose:** Run JavaScript against the `@oai/artifact-tool` package for creating presentations (PPTX) or spreadsheets (XLSX).

**When active:** `Artifact` feature flag + artifact runtime available.

**Type:** Freeform tool (raw JS source, like `js_repl`).

**Input format:** Raw JavaScript with optional pragma: `// codex-artifact-tool: timeout_ms=15000`. The `@oai/artifact-tool` package is preloaded. Use `Presentation.create()`, `Workbook.create()`, etc.

Source: [`spec.rs` L2116-2142](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2116) [Official]

---

### 27. `codex` (code mode) [Official]

**Purpose:** Execute code in an isolated code-mode environment with access to nested tools. The public tool name is dynamically set (referenced as `PUBLIC_TOOL_NAME` in source).

**When active:** `CodeMode` feature flag.

**Type:** Freeform tool with `@exec:` pragma annotations for tool declarations.

When `CodeModeOnly` is also enabled, this becomes the sole execution interface -- all other tools are nested inside it.

Source: [`spec.rs` L2160-2187](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2160) [Official]

---

### 28. `wait` (code mode) [Official]

**Purpose:** Wait on a yielded code-mode cell and return new output or completion status.

**When active:** `CodeMode` feature flag (paired with code mode tool).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cell_id` | `string` | Yes | ID of the running exec cell |
| `yield_time_ms` | `number` | No | Wait time before yielding again |
| `max_tokens` | `number` | No | Max output tokens for this wait call |
| `terminate` | `boolean` | No | Whether to terminate the cell |

Source: [`spec.rs` L796-845](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L796) [Official]

---

### 29. `list_mcp_resources` [Official]

**Purpose:** List resources provided by connected MCP servers. Prefers MCP resources over web search.

**When active:** MCP tools are configured.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server` | `string` | No | MCP server name (omit for all servers) |
| `cursor` | `string` | No | Pagination cursor from previous call |

Source: [`spec.rs` L2188-2223](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2188) [Official]

---

### 30. `list_mcp_resource_templates` [Official]

**Purpose:** List parameterized resource templates from MCP servers.

**When active:** MCP tools are configured.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server` | `string` | No | MCP server name (omit for all) |
| `cursor` | `string` | No | Pagination cursor from previous call |

Source: [`spec.rs` L2224-2259](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2224) [Official]

---

### 31. `read_mcp_resource` [Official]

**Purpose:** Read a specific resource from an MCP server by server name and URI.

**When active:** MCP tools are configured.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server` | `string` | Yes | MCP server name (must match config) |
| `uri` | `string` | Yes | Resource URI from `list_mcp_resources` |

Source: [`spec.rs` L2260-2306](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L2260) [Official]

---

### 32. `test_sync_tool` [Official]

**Purpose:** Internal synchronization helper for Codex integration tests. Not user-facing.

**When active:** Experimental (`"test_sync_tool"` in `experimental_supported_tools`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `sleep_before_ms` | `number` | No | Delay before barrier |
| `sleep_after_ms` | `number` | No | Delay after barrier |
| `barrier` | `object` | No | `{ id, participants, timeout_ms }` |

Source: [`spec.rs` L1602-1670](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs#L1602) [Official]

---

## Shell Tool Variants

Codex has three mutually exclusive shell execution backends, selected by configuration:

| Shell Type | Tool Name | Input Format | PTY Support | Interactive |
|------------|-----------|--------------|-------------|-------------|
| `Default` | `shell` | `string[]` (argv) | No | No |
| `ShellCommand` / `ZshFork` | `shell_command` | `string` (script) | No | No |
| `UnifiedExec` | `exec_command` + `write_stdin` | `string` (cmd) | Yes | Yes (via write_stdin) |

Handler aliases registered regardless of mode: `shell`, `container.exec`, `local_shell`, `shell_command`.

---

## Tool Type Taxonomy

Codex uses three distinct tool specification types:

| Type | Format | Examples |
|------|--------|---------|
| `ToolSpec::Function` | Standard JSON function calling | `shell`, `read_file`, `spawn_agent`, etc. |
| `ToolSpec::Freeform` | Raw text with grammar validation | `js_repl`, `artifacts`, code mode |
| `ToolSpec::WebSearch` | OpenAI-native tool type | `web_search` |
| `ToolSpec::ImageGeneration` | OpenAI-native tool type | `image_generation` |
| `ToolSpec::LocalShell` | OpenAI-native local shell | `local_shell` (Default mode variant) |
| `ToolSpec::ToolSearch` | Client-executed search | `tool_search` |

---

## MCP Server Mode Tools

When Codex CLI runs as an MCP server itself (for integration with Agents SDK or other MCP clients), it exposes two tools:

| Tool | Purpose |
|------|---------|
| `codex()` | Start a new Codex conversation session |
| `codex-reply()` | Continue an existing session by conversation ID and prompt |

Source: [Agents SDK guide](https://developers.openai.com/codex/guides/agents-sdk) [Official]

---

## Sources

- [Codex CLI GitHub repository](https://github.com/openai/codex) [Official]
- [`codex-rs/core/src/tools/spec.rs`](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec.rs) -- Primary tool definitions [Official]
- [`codex-rs/core/src/tools/handlers/plan.rs`](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/handlers/plan.rs) -- Plan tool [Official]
- [`codex-rs/core/prompt.md`](https://github.com/openai/codex/blob/main/codex-rs/core/prompt.md) -- System prompt [Official]
- [Codex CLI Features](https://developers.openai.com/codex/cli/features) -- Official docs [Official]
- [Codex CLI Reference](https://developers.openai.com/codex/cli/reference) -- CLI flags [Official]
- [Codex Changelog](https://developers.openai.com/codex/changelog) -- Tool updates [Official]
- [Agents SDK Guide](https://developers.openai.com/codex/guides/agents-sdk) -- MCP server mode [Official]
