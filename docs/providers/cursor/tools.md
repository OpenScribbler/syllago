# Cursor Agent Tools

> Research date: 2026-03-21
> Status: Draft — compiled from official docs, community sources, and leaked system prompts.

## Overview

Cursor is a VS Code fork with integrated AI agent capabilities. Its agent mode provides a set of built-in tools that the AI can invoke during conversations. Cursor recently added an [official tools page](https://cursor.com/docs/agent/tools) that describes tools at a high level, but does not expose exact function signatures. The parameter-level detail below comes from leaked system prompts (March 2025, and "Agent Prompt 2.0" circa late 2025/early 2026).

There is **no limit** on the number of tool calls an agent can make during a task. [Official]

Tool names evolve between prompt versions. Where names differ across sources, both are noted.

---

## Core Tools

### 1. codebase_search

Semantic search over the indexed codebase. Finds code by meaning, not exact text matches. Uses Cursor's custom embedding model.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Natural-language question about the code |
| `target_directories` | string[] | yes (can be empty) | Directory glob patterns to scope the search |
| `explanation` | string | yes | One-sentence rationale for the search |

**When to use:** Exploring unfamiliar codebases, finding code by intent ("where is auth handled?").
**When NOT to use:** Exact text matches (use grep), reading known files, file-name lookups.

Cross-provider equivalent: Claude Code `Grep` (semantic mode) / `mcp__codebase__search`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0, jujumilk3 leak]

---

### 2. read_file

Reads file contents from the filesystem. Supports text files and images (JPEG, PNG, GIF, WebP, SVG).

**v1 parameters** (March 2025 prompt):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_file` | string | yes | Absolute or relative file path |
| `start_line_one_indexed` | integer | yes | Starting line number |
| `end_line_one_indexed_inclusive` | integer | yes | Ending line number |
| `should_read_entire_file` | boolean | yes | Whether to read full file |
| `explanation` | string | yes | Rationale |

**v2 parameters** (Agent Prompt 2.0):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_file` | string | yes | Absolute or relative file path |
| `offset` | integer | no | Starting line number |
| `limit` | integer | no | Number of lines to read |

v1 caps reads at 250 lines per call. v2 does not document a cap.

Cross-provider equivalent: Claude Code `Read`, Windsurf `read_file`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0]

---

### 3. edit_file

Proposes edits to existing files or creates new files. Uses `// ... existing code ...` context markers to indicate unchanged regions. A secondary ("weaker") model applies the diff to the actual file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_file` | string | yes | File path |
| `instructions` | string | yes | One-sentence description of the edit |
| `code_edit` | string | yes | Only changed lines, with context markers for unchanged code |

**Behavioral rules from system prompt:**
- Use at most once per turn.
- Never output full unchanged code — only the changed portions.
- Minimize duplication of surrounding context.
- If the diff was applied incorrectly, use `reapply` rather than re-editing.

Cross-provider equivalent: Claude Code `Edit`, Windsurf `edit_file`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0, jujumilk3 leak]

---

### 4. run_terminal_cmd

Proposes a terminal command for execution. Requires user approval by default (can be configured).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | Shell command to execute |
| `is_background` | boolean | yes | Run in background |
| `require_user_approval` | boolean | yes | Whether to wait for user approval |
| `explanation` | string | no | Rationale for the command |

**Behavioral rules:**
- Assumes user is unavailable for interactive input — must use non-interactive flags.
- Append `| cat` for commands that use pagers.
- Environment persists between calls within a session.

Cross-provider equivalent: Claude Code `Bash`, Windsurf `run_command`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0, jujumilk3 leak]

---

### 5. grep_search / grep

Fast text-based regex search using ripgrep under the hood. Capped at 50 results.

**v1 parameters** (grep_search):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Regex pattern |
| `case_sensitive` | boolean | no | Case sensitivity |
| `include_pattern` | string | no | File glob to include (e.g. `*.ts`) |
| `exclude_pattern` | string | no | File glob to exclude |
| `explanation` | string | yes | Rationale |

**v2 parameters** (grep — Agent Prompt 2.0):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `pattern` | string | yes | Regex pattern |
| `path` | string | no | File/directory to search |
| `glob` | string | no | Glob filter |
| `output_mode` | string | no | `content`, `files_with_matches`, or `count` |
| `-B`, `-A`, `-C` | number | no | Context lines before/after/both |
| `-i` | boolean | no | Case insensitive |
| `type` | string | no | File type (js, py, rust, etc.) |
| `head_limit` | number | no | Limit output to N lines |
| `multiline` | boolean | no | Enable multiline mode |

Cross-provider equivalent: Claude Code `Grep`, Windsurf `grep_search`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0, jujumilk3 leak]

---

### 6. file_search / glob_file_search

Finds files by name. Two variants exist across prompt versions:

**file_search** (v1) — fuzzy matching against file paths, capped at 10 results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Fuzzy filename query |
| `explanation` | string | yes | Rationale |

**glob_file_search** (v2) — glob-pattern matching, sorted by modification time.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `glob_pattern` | string | yes | Glob pattern (e.g. `**/*.test.ts`) |
| `target_directory` | string | no | Directory to scope search |

Cross-provider equivalent: Claude Code `Glob`, Windsurf `find_file`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0, jujumilk3 leak]

---

### 7. list_dir

Lists directory contents. Described as "the quick tool for discovery" before using more targeted tools.

**v1:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `relative_workspace_path` | string | yes | Path relative to workspace root |
| `explanation` | string | yes | Rationale |

**v2:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_directory` | string | yes | Directory path |
| `ignore_globs` | string[] | no | Glob patterns to exclude |

Cross-provider equivalent: Claude Code `Bash` (`ls`), Windsurf `list_dir`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0, jujumilk3 leak]

---

### 8. delete_file

Deletes a file. Fails gracefully if the file does not exist or cannot be deleted.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_file` | string | yes | File path relative to workspace root |
| `explanation` | string | no | Rationale |

Cross-provider equivalent: Claude Code `Bash` (`rm`), Windsurf `delete_file`

[Community] [Source: sshh12 gist, x1xhlol Agent Prompt 2.0]

---

### 9. web_search

Searches the web for real-time information beyond the model's training data.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `search_term` | string | yes | Search query |
| `explanation` | string | no | Rationale |

Cross-provider equivalent: Claude Code `WebSearch`, Windsurf `web_search`

[Official] [Community] [Source: Cursor docs, sshh12 gist, x1xhlol Agent Prompt 2.0]

---

### 10. reapply

Invokes a smarter model to re-apply the last `edit_file` diff if the previous application produced unexpected results. Only available in v1 prompts.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_file` | string | yes | File to re-apply the edit to |

This is a Cursor-specific recovery mechanism with no direct cross-provider equivalent.

[Community] [Source: sshh12 gist, jujumilk3 leak]

---

### 11. diff_history

Retrieves recent file modifications in the workspace, including change counts and timestamps. Provides context about what has changed recently.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `explanation` | string | yes | Rationale for checking history |

Cross-provider equivalent: Claude Code `Bash` (`git diff` / `git log`)

[Community] [Source: jujumilk3 leak]

---

### 12. fetch_rules

Retrieves user-defined rules (`.cursorrules`, project rules) that provide codebase-specific guidance.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `rule_names` | string[] | yes | Rule identifiers to fetch |

This is a Cursor-specific mechanism. Cross-provider equivalent concept: Claude Code `CLAUDE.md` auto-loading, Windsurf `.windsurfrules`.

[Official] [Community] [Source: Cursor docs, sshh12 gist]

---

## Extended Tools (Agent Prompt 2.0 / Cursor 2.4+)

These tools appear in the newer "Agent Prompt 2.0" and may not be available in all configurations.

### 13. update_memory

Creates, updates, or deletes persistent knowledge items that survive across sessions (Cursor's "Memories" feature).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `title` | string | no | Memory title for lookup |
| `knowledge_to_store` | string | no | One paragraph of content |
| `action` | string | no | `create`, `update`, or `delete` (default: `create`) |
| `existing_knowledge_id` | string | no | Required for update/delete |

Cross-provider equivalent: Claude Code memory via `CLAUDE.md`, Windsurf Memories

[Community] [Source: x1xhlol Agent Prompt 2.0]

---

### 14. read_lints

Displays linter/diagnostic errors from the workspace. Can target specific files or scan all.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `paths` | string[] | no | Files/directories to check; omit for all |

Cross-provider equivalent: No direct equivalent in Claude Code (uses `Bash` to run linters).

[Community] [Source: x1xhlol Agent Prompt 2.0]

---

### 15. edit_notebook

Modifies or creates Jupyter notebook cells.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_notebook` | string | yes | Notebook file path |
| `cell_idx` | number | yes | Cell index (0-based) |
| `is_new_cell` | boolean | yes | True to insert, false to edit |
| `cell_language` | string | yes | `python`, `markdown`, `javascript`, `typescript`, `r`, `sql`, `shell`, `raw`, or `other` |
| `old_string` | string | yes | Text to replace (with 3-5 lines context) |
| `new_string` | string | yes | Replacement text or full new cell content |

Cross-provider equivalent: Claude Code `NotebookEdit`

[Community] [Source: x1xhlol Agent Prompt 2.0]

---

### 16. todo_write

Creates and manages task checklists for tracking multi-step coding work.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `merge` | boolean | yes | True to merge with existing todos, false to replace |
| `todos` | array | yes | Array of `{content, status, id}` objects |

Status values: `pending`, `in_progress`, `completed`, `cancelled`.

Cross-provider equivalent: Claude Code `TodoWrite` (similar concept)

[Community] [Source: x1xhlol Agent Prompt 2.0]

---

### 17. parallel (multi_tool_use)

Meta-tool that wraps multiple independent tool calls for simultaneous execution.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tool_uses` | array | yes | Array of `{recipient_name, parameters}` objects |

This is an OpenAI-style parallel tool-calling wrapper. Claude Code achieves parallelism natively through the Anthropic API's multi-tool response format.

[Community] [Source: x1xhlol Agent Prompt 2.0]

---

### 18. Browser

Controls a browser for taking screenshots, navigating pages, clicking elements, and capturing page state. Used for testing web applications and reviewing visual output. [Official]

Exact parameters are not documented in leaked prompts — this tool appears to be injected separately from the main agent prompt.

Cross-provider equivalent: Claude Code `mcp__chrome-devtools__*` (via MCP), Windsurf browser preview

[Official] [Source: Cursor docs — cursor.com/docs/agent/tools]

---

### 19. Image Generation

Generates images from text descriptions or reference images using an underlying model (Google Nano Banana Pro, per changelog). Saves output to the project's `assets/` folder by default. Introduced in Cursor 2.4 (January 2026). [Official]

Exact parameters are not documented in leaked prompts.

Cross-provider equivalent: No direct equivalent in Claude Code or Windsurf.

[Official] [Source: Cursor docs, Cursor changelog 2.4]

---

### 20. Ask Questions

Enables the agent to request clarification from the user during tasks. The agent can continue other work while waiting for a response. [Official]

Cross-provider equivalent: Claude Code `AskUserQuestion` (similar concept)

[Official] [Source: Cursor docs — cursor.com/docs/agent/tools]

---

## Tool Evolution Summary

The tool set has grown across prompt versions:

| Version | Approx. Date | Tool Count | Notable Additions |
|---------|-------------|------------|-------------------|
| v1 (Claude 3.7 Sonnet prompt) | March 2025 | 11-12 | Core set: codebase_search, read/edit/delete_file, grep_search, file_search, list_dir, run_terminal_cmd, web_search, reapply, diff_history, fetch_rules |
| v2 (Agent Prompt 2.0) | Late 2025 / Early 2026 | 14+ | Added: update_memory, read_lints, edit_notebook, todo_write, glob_file_search; grep expanded significantly |
| Cursor 2.4+ | January 2026 | 17-20 | Added: browser, image generation, ask questions, subagent spawning |

---

## Cross-Provider Tool Mapping

| Cursor Tool | Claude Code | Windsurf |
|------------|-------------|----------|
| `codebase_search` | `Grep` (semantic) | `codebase_search` |
| `read_file` | `Read` | `read_file` |
| `edit_file` | `Edit` | `edit_file` |
| `run_terminal_cmd` | `Bash` | `run_command` |
| `grep_search` / `grep` | `Grep` | `grep_search` |
| `file_search` / `glob_file_search` | `Glob` | `find_file` |
| `list_dir` | `Bash` (`ls`) | `list_dir` |
| `delete_file` | `Bash` (`rm`) | `delete_file` |
| `web_search` | `WebSearch` | `web_search` |
| `fetch_rules` | Auto-loaded `CLAUDE.md` | `.windsurfrules` auto-load |
| `update_memory` | `CLAUDE.md` edits | Memories |
| `read_lints` | `Bash` (run linter) | — |
| `edit_notebook` | `NotebookEdit` | — |
| `todo_write` | `TodoWrite` | — |
| `reapply` | — (not needed) | — |
| `diff_history` | `Bash` (`git log/diff`) | — |
| Browser | `mcp__chrome-devtools__*` | Browser preview |
| Image Generation | — | — |

---

## Key Behavioral Constraints

From the system prompts, Cursor enforces these rules on tool usage:

1. **Never disclose tool names** to the user — describe actions in natural language.
2. **Don't call tools unnecessarily** — if the answer is already known, respond directly.
3. **Never output code to the user** unless explicitly asked — use `edit_file` instead.
4. **Respect the `explanation` field** — most tools require a one-sentence rationale, used for internal logging/audit.
5. **Checkpoint before changes** — agent mode creates restore points before file modifications.
6. **User approval gate** — terminal commands require approval by default (configurable via `require_user_approval`).

---

## Sources

- [Cursor Official Tools Docs](https://cursor.com/docs/agent/tools) [Official]
- [Cursor Agent Overview](https://docs.cursor.com/chat/agent) [Official]
- [Cursor 2.4 Changelog — Subagents, Skills, Image Generation](https://cursor.com/changelog/2-4) [Official]
- [Cursor Agent System Prompt, March 2025 (sshh12 gist)](https://gist.github.com/sshh12/25ad2e40529b269a88b80e7cf1c38084) [Community]
- [Agent Prompt 2.0 (x1xhlol repo)](https://github.com/x1xhlol/system-prompts-and-models-of-ai-tools/blob/main/Cursor%20Prompts/Agent%20Prompt%202.0.txt) [Community]
- [Cursor Claude Sonnet 3.7 Prompt, March 2025 (jujumilk3)](https://github.com/jujumilk3/leaked-system-prompts/blob/main/cursor-ide-agent-claude-sonnet-3.7_20250309.md) [Community]
- [Cursor Forum — Agent Tools List](https://forum.cursor.com/t/agent-tools-list/31197) [Community]
