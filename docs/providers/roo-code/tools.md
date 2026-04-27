# Roo Code Tools Reference

Roo Code (fork of Cline) exposes tools to AI models via XML-style tool use in the
system prompt. Tools use **snake_case** names in the AI-facing interface while the
source code uses **PascalCase** class names (e.g., `ReadFileTool`).

## Relationship to Cline

Roo Code forked from Cline and shares the same foundational tools. Key divergences:

- Roo Code added **multiple diff/edit strategies** (`apply_diff`, `edit_file`,
  `search_and_replace`, `search_replace`, `insert_content`) where Cline has fewer
  editing tools. [Official]
- Roo Code added **mode-related tools** (`switch_mode`, `fetch_instructions`) that
  Cline lacks (Cline uses Plan/Act instead of modes). [Official]
- Roo Code added **`codebase_search`** (semantic search), **`generate_image`**,
  **`update_todo_list`**, **`skill`**, **`apply_patch`**, and
  **`read_command_output`** — none present in upstream Cline. [Official]
- Both share the same core tool names for: `read_file`, `write_to_file`,
  `list_files`, `search_files`, `list_code_definition_names`, `execute_command`,
  `browser_action`, `use_mcp_tool`, `access_mcp_resource`, `ask_followup_question`,
  `attempt_completion`, `new_task`. [Official]

---

## File Read Tools

### read_file

Reads file contents. Supports text files, PDFs, Word docs, and images.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | File path relative to cwd |
| `mode` | string | no | `"slice"` (default) or `"indentation"` |
| `offset` | integer | no | 1-based start line (slice mode, default: 1) |
| `limit` | integer | no | Max lines to return (slice mode, default: 2000) |
| `anchor_line` | integer | indentation mode | Line number to anchor extraction |
| `max_levels` | integer | no | Max indentation levels above anchor |
| `include_siblings` | boolean | no | Include sibling blocks |
| `include_header` | boolean | no | Include file header content |
| `max_lines` | integer | no | Hard cap on lines returned |

Enhanced multi-file format uses an `args` object with nested `file` entries
supporting `path` and `line_range` (e.g., `"1-50"`).

Source: `src/core/tools/ReadFileTool.ts` [Official]

### read_command_output

Retrieves full output from previously truncated command executions.

Source: Tool Use Overview docs [Official]

### list_files

Lists files and directories within a specified path.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Directory path relative to cwd |
| `recursive` | boolean | no | If true, lists recursively |

Source: `src/core/tools/ListFilesTool.ts` [Official]

### list_code_definition_names

Lists top-level definition names (classes, functions, methods) from source files.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | File or directory path relative to cwd |

Uses tree-sitter for parsing. Shared with Cline. [Official]

### search_files

Regex search across files in a directory with context-rich results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Directory to search in |
| `regex` | string | yes | Regex pattern to match |
| `file_pattern` | string | no | Glob to filter files (e.g., `"*.ts"`) |

Source: `src/core/tools/SearchFilesTool.ts` [Official]

### codebase_search

Semantic search across the indexed codebase. Requires indexing to be enabled.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Natural language search query |
| `path` | string | no | Directory to scope the search |

Roo Code only (not in Cline). [Official]

---

## File Write/Edit Tools

### write_to_file

Creates new files or completely replaces existing file content.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | File path relative to cwd |
| `content` | string | yes | Complete file content to write |
| `line_count` | integer | yes | Number of lines in content (including empty) |

Source: `src/core/tools/WriteToFileTool.ts` [Official]

### apply_diff

Applies targeted changes using search/replace blocks with fuzzy matching.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | File path relative to cwd |
| `diff` | string | yes | Search/replace block with `<<<<<<< SEARCH` / `=======` / `>>>>>>> REPLACE` markers |
| `start_line` | number | no | Hint for where search content begins |
| `end_line` | number | no | Hint for where search content ends |

Line hints (`:start_line:`, `:end_line:`) are embedded within each diff block.
Uses Levenshtein distance for fuzzy matching when exact match fails. [Official]

### apply_patch

Applies multi-file unified diff patches in a single operation.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `patch` | string | yes | Unified diff patch content |

Roo Code only (not in Cline). [Official]

### edit_file

Search-and-replace editing with occurrence count validation. Replaces all
occurrences by default.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | File path relative to cwd |
| `old_string` | string | yes | Text to find |
| `new_string` | string | yes | Replacement text |
| `expected_replacements` | integer | no | Expected match count (default: 1) |

Uses three fallback strategies: exact match, whitespace-tolerant regex, token-based
matching. Source: `src/core/tools/EditFileTool.ts` [Official]

### search_and_replace / search_replace

Deprecated aliases for edit-style operations. Simple search-and-replace for all
occurrences. Being superseded by `edit_file`.

Source: `src/core/tools/SearchAndReplaceTool.ts`, `SearchReplaceTool.ts` [Official]

### insert_content

Inserts content at a specific line position within an existing file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | File path relative to cwd |
| `position` | integer | yes | Line number to insert at |
| `content` | string | yes | Content to insert |

Source: Tool Use Overview docs [Official]

---

## Command Execution Tools

### execute_command

Runs CLI commands on the user's system.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | CLI command to execute |
| `cwd` | string | no | Working directory (defaults to current) |

Source: `src/core/tools/ExecuteCommandTool.ts` [Official]

### run_slash_command

Executes predefined slash commands for templated instructions. Experimental.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command_name` | string | yes | Name of the slash command to run |

[Official]

---

## Browser Tool

### browser_action

Browser automation via Puppeteer. While the browser is active, no other tools can
be used.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | One of: `launch`, `click`, `type`, `scroll_down`, `scroll_up`, `screenshot`, `close` |
| `url` | string | launch only | URL to navigate to |
| `coordinate` | string | click only | `"x,y"` click coordinates |
| `text` | string | type only | Text to type |

Requires browser prerequisites (Puppeteer/Chromium). Shared with Cline. [Official]

---

## Image Tool

### generate_image

Generates AI-powered images from text prompts via OpenRouter. Experimental.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` | string | yes | Image generation prompt |
| `path` | string | yes | Output file path in workspace |
| `image` | string | no | Path to existing image (for editing) |

Supports PNG, JPG, JPEG, GIF, WEBP input. Roo Code only. [Official]

---

## MCP Tools

### use_mcp_tool

Executes a tool provided by a connected MCP server.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_name` | string | yes | Name of the MCP server |
| `tool_name` | string | yes | Name of the tool to call |
| `arguments` | object | yes | Tool arguments as JSON |

Shared with Cline. [Official]

### access_mcp_resource

Accesses a resource provided by a connected MCP server.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_name` | string | yes | Name of the MCP server |
| `uri` | string | yes | Resource URI |

Shared with Cline. [Official]

---

## Workflow & Interaction Tools

### ask_followup_question

Asks the user a question to gather additional information.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `question` | string | yes | The question to ask |
| `follow_up` | array | no | Suggested answer options |

Shared with Cline. [Official]

### attempt_completion

Presents the final result of a task to the user.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `result` | string | yes | Final result description |
| `command` | string | no | CLI command to demonstrate the result |

Shared with Cline. [Official]

### switch_mode

Requests switching to a different operational mode.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode_slug` | string | yes | Target mode slug (e.g., `"code"`, `"architect"`) |
| `reason` | string | no | Why the switch is needed |

Roo Code only (Cline uses Plan/Act instead). [Official]

### new_task

Creates a new task instance in a chosen mode with an initial message.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | string | yes | Mode slug for the new task |
| `message` | string | yes | Initial task message |

Shared concept with Cline, but Roo adds mode routing. [Official]

### fetch_instructions

Fetches detailed instructions for performing specific tasks (e.g., creating an MCP
server or a new mode).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task` | string | yes | Type of task to get instructions for |

Roo Code only. [Official]

### update_todo_list

Creates and manages an interactive todo list UI component in the chat.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | string | yes | Markdown checklist content |

Automatically created for complex/multi-step tasks. Roo Code only. [Official]

### skill

Loads and executes predefined skill instructions from SKILL.md files.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `skill_name` | string | yes | Name of the skill to load |

Injects skill instructions into conversation context on demand. Roo Code only. [Official]

---

## Tool Groups (Mode Access Control)

Tools are organized into groups that control which modes can access them:

| Group | Tools |
|-------|-------|
| **read** | `read_file`, `list_files`, `list_code_definition_names`, `search_files`, `codebase_search`, `read_command_output` |
| **edit** | `write_to_file`, `apply_diff`, `apply_patch`, `edit_file`, `search_and_replace`, `search_replace`, `insert_content` |
| **command** | `execute_command`, `run_slash_command` |
| **browser** | `browser_action` |
| **mcp** | `use_mcp_tool`, `access_mcp_resource` |

Workflow tools (`ask_followup_question`, `attempt_completion`, `switch_mode`,
`new_task`) are available in all modes. [Official]

---

## Custom Tools (Experimental)

User-defined tools via TypeScript/JavaScript files. See content-types.md for
details on the `.roo/tools/` format.

---

## Sources

- [Tool Use Overview](https://docs.roocode.com/advanced-usage/available-tools/tool-use-overview) [Official]
- [read_file docs](https://docs.roocode.com/advanced-usage/available-tools/read-file) [Official]
- [write_to_file docs](https://docs.roocode.com/advanced-usage/available-tools/write-to-file) [Official]
- [apply_diff docs](https://docs.roocode.com/advanced-usage/available-tools/apply-diff) [Official]
- [execute_command docs](https://docs.roocode.com/advanced-usage/available-tools/execute-command) [Official]
- [Custom Tools docs](https://docs.roocode.com/features/experimental/custom-tools) [Official]
- [How Tools Work](https://docs.roocode.com/basic-usage/how-tools-work) [Official]
- [File Operation Tools (DeepWiki)](https://deepwiki.com/RooCodeInc/Roo-Code/6.4-file-operation-tools) [Community]
- [GitHub: RooCodeInc/Roo-Code](https://github.com/RooCodeInc/Roo-Code) [Official]
