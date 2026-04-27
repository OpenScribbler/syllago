# Cline Tools Reference

Cline exposes built-in tools via XML-style invocation in its system prompt. Tools are
grouped by category below.

## Tool Invocation Format

```xml
<tool_name>
  <parameter_name>value</parameter_name>
</tool_name>
```

One tool per message. Each tool use is informed by the result of the previous one.
All tools require human-in-the-loop approval (configurable per-tool via `alwaysAllow`
in MCP settings).

---

## File Operations

### `read_file`

Read the contents of a file.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path`    | Yes      | File path relative to cwd |

[Official] https://docs.cline.bot/exploring-clines-tools/cline-tools-guide

### `write_to_file`

Create a new file or completely overwrite an existing file. Displays a streaming diff
in the editor for approval.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path`    | Yes      | File path relative to cwd |
| `content` | Yes      | Complete intended content of the file |

[Official] https://docs.cline.bot/exploring-clines-tools/cline-tools-guide

### `replace_in_file`

Make targeted edits to an existing file using SEARCH/REPLACE blocks. Preferred over
`write_to_file` for surgical changes to avoid rewriting entire files.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path`    | Yes      | File path relative to cwd |
| `diff`    | Yes      | One or more SEARCH/REPLACE blocks (exact match required) |

Diff block format:

```
<<<<<<< SEARCH
exact lines to find
=======
replacement lines
>>>>>>> REPLACE
```

**Note:** Earlier Cline versions used a tool called `apply_diff`. The current tool is
`replace_in_file`. Some forks (e.g., Roo Code) may still use `apply_diff`.

[Official] https://docs.cline.bot/exploring-clines-tools/cline-tools-guide
[Official] https://cline.bot/blog/improving-diff-edits-by-10

### `search_files`

Regex-based content search across files using ripgrep (Rust regex syntax).

| Parameter      | Required | Description |
|----------------|----------|-------------|
| `path`         | Yes      | Directory path to search recursively |
| `regex`        | Yes      | Regular expression pattern (Rust syntax) |
| `file_pattern` | No       | Glob pattern to filter searched files (e.g., `*.ts`) |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

### `list_files`

List files and directories at a given path.

| Parameter   | Required | Description |
|-------------|----------|-------------|
| `path`      | Yes      | Directory path to list |
| `recursive` | No       | Whether to list recursively (boolean, default: false) |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

### `list_code_definition_names`

List definition names (classes, functions, methods, etc.) in source code files at a
given directory. Uses tree-sitter for parsing.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path`    | Yes      | Directory path to analyze |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

---

## Terminal

### `execute_command`

Run a CLI command in the integrated terminal.

| Parameter           | Required | Description |
|---------------------|----------|-------------|
| `command`           | Yes      | CLI command to execute |
| `requires_approval` | Yes      | Boolean — whether the command needs explicit user approval before running |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

---

## Browser Automation

### `browser_action`

Interact with a Puppeteer-controlled browser. Each action (except `close`) returns a
screenshot and console logs. Only one browser instance at a time; no other tools can
be used while the browser is open.

| Parameter    | Required | Description |
|--------------|----------|-------------|
| `action`     | Yes      | One of: `launch`, `click`, `type`, `scroll_down`, `scroll_up`, `screenshot`, `close` |
| `url`        | Conditional | URL to navigate to (required with `launch`) |
| `text`       | Conditional | Text string to type (required with `type`) |
| `coordinate` | Conditional | `x,y` pixel coordinates (required with `click`). Within 1280x800 resolution |

**Constraints:**
- Browser viewport is 900x600 pixels (screenshot resolution is 1280x800)
- Sequence must start with `launch` and end with `close`
- While browser is active, no other tools can be called
- To visit a new unrelated URL, close first then re-launch

[Official] https://docs.cline.bot/tools-reference/browser-automation
[Community] https://deepwiki.com/cline/cline/4.3-browser-integration

---

## MCP Integration

### `use_mcp_tool`

Invoke a tool exposed by a connected MCP server.

| Parameter     | Required | Description |
|---------------|----------|-------------|
| `server_name` | Yes      | Name of the MCP server providing the tool |
| `tool_name`   | Yes      | Name of the tool to execute |
| `arguments`   | Yes      | JSON object of the tool's input parameters |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

### `access_mcp_resource`

Read a resource exposed by a connected MCP server (files, API responses, system info).

| Parameter     | Required | Description |
|---------------|----------|-------------|
| `server_name` | Yes      | Name of the MCP server providing the resource |
| `uri`         | Yes      | URI identifying the specific resource |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

---

## Interaction / Control Flow

### `ask_followup_question`

Ask the user a clarifying question. Used when encountering ambiguity or needing
additional information.

| Parameter  | Required | Description |
|------------|----------|-------------|
| `question` | Yes      | Clear, specific question for the user |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

### `attempt_completion`

Present the final result of a task to the user. Should only be used when the task is
complete and no further input is needed.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `result`  | Yes      | Final result description (no questions or further engagement) |
| `command` | No       | CLI command to demonstrate the result (e.g., `open index.html`) |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

### `plan_mode_response`

Respond to user in PLAN MODE to discuss solution strategy before executing.

| Parameter  | Required | Description |
|------------|----------|-------------|
| `response` | Yes      | Response text for the planning discussion |

[Official] https://gist.github.com/maoxiaoke/cd960ac88e11b08cbb4fa697439ebc68

### `new_task`

End the current session and start a new one with preloaded context. Used for context
window management and task handoffs.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `context` | Yes      | Structured context block to carry into the new task (completed work, current state, next steps, references) |

[Official] https://docs.cline.bot/exploring-clines-tools/new-task-tool
[Official] https://cline.bot/blog/unlocking-persistent-memory-how-clines-new_task-tool-eliminates-context-window-limitations

---

## Complete Tool List (Summary)

| # | Tool | Category | Purpose |
|---|------|----------|---------|
| 1 | `read_file` | File | Read file contents |
| 2 | `write_to_file` | File | Create or overwrite a file |
| 3 | `replace_in_file` | File | Surgical SEARCH/REPLACE edits |
| 4 | `search_files` | File | Regex search across files |
| 5 | `list_files` | File | List directory contents |
| 6 | `list_code_definition_names` | File | List code definitions (classes, functions, etc.) |
| 7 | `execute_command` | Terminal | Run CLI commands |
| 8 | `browser_action` | Browser | Puppeteer browser interaction |
| 9 | `use_mcp_tool` | MCP | Call an MCP server tool |
| 10 | `access_mcp_resource` | MCP | Read an MCP server resource |
| 11 | `ask_followup_question` | Interaction | Ask user for clarification |
| 12 | `attempt_completion` | Interaction | Present final result |
| 13 | `plan_mode_response` | Interaction | Respond in plan mode |
| 14 | `new_task` | Interaction | Start new task with context handoff |
