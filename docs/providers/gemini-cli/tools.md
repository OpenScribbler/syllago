# Gemini CLI Built-in Tools Reference

> Research report for syllago cross-provider mapping.
> Generated: 2026-03-21

## Overview

Gemini CLI ships a set of built-in tools in `packages/core/src/tools/`. This document catalogs each tool with its exact name, purpose, key parameters, cross-provider equivalents, and sensitivity classification.

**Source attribution tags:** `[Official]` = Google developer docs, `[Community]` = blog/forum posts, `[Inferred]` = derived from source code analysis, `[Unverified]` = not yet confirmed.

Sources:
- [Official tools index](https://google-gemini.github.io/gemini-cli/docs/tools/) `[Official]`
- [File system tools](https://google-gemini.github.io/gemini-cli/docs/tools/file-system.html) `[Official]`
- [Shell tool](https://google-gemini.github.io/gemini-cli/docs/tools/shell.html) `[Official]`
- [Memory tool](https://google-gemini.github.io/gemini-cli/docs/tools/memory.html) `[Official]`
- [Web fetch tool](https://google-gemini.github.io/gemini-cli/docs/tools/web-fetch.html) `[Official]`
- [Ask user tool](https://geminicli.com/docs/tools/ask-user/) `[Official]`
- [Plan mode](https://geminicli.com/docs/cli/plan-mode/) `[Official]`
- [Subagents](https://geminicli.com/docs/core/subagents/) `[Official]`
- [v0.34.0 release notes](https://github.com/google-gemini/gemini-cli/releases/tag/v0.34.0) `[Official]`
- [GitHub repo](https://github.com/google-gemini/gemini-cli) `[Official]`

---

## Sensitivity Classification

Tools that modify the filesystem or execute commands require user confirmation before running. These are the **sensitive tools** (require y/n prompt):

| Tool | Why Sensitive |
|------|--------------|
| `write_file` | Creates/modifies files |
| `replace` (edit) | Modifies file content |
| `run_shell_command` | Executes arbitrary commands |
| `web_fetch` | Network access to external URLs |
| `google_web_search` | Network access to external search |
| `activate_skill` | Injects instructions into session context |

**Non-sensitive tools** (no confirmation): `read_file`, `read_many_files`, `list_directory`, `glob`, `search_file_content`, `save_memory`, `ask_user`, `write_todos`.

`[Official]` — [tools overview](https://google-gemini.github.io/gemini-cli/docs/tools/), [plan mode safety](https://geminicli.com/docs/cli/plan-mode/)

The `--yolo` flag bypasses all confirmations. The `--sandbox` flag runs sensitive operations in a container. When `--yolo` is used, sandboxing is enabled by default.

---

## File System Tools

### `read_file` (ReadFile)

**Purpose:** Reads content from a single file. Supports text files, images (PNG, JPG, GIF, WEBP, SVG, BMP), and PDFs. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute or relative file path |
| `offset` | number | no | Line number to start reading from |
| `limit` | number | no | Max lines to read |

**Cross-provider equivalents:**
- Claude Code: `Read` (built-in tool)
- Cursor: `read_file`
- Windsurf: `read_file`

**Sensitivity:** None (read-only)

---

### `read_many_files` (MultiFileRead)

**Purpose:** Reads content from multiple files or directories in a single call. `[Inferred]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `paths` | string[] | yes | Array of file/directory paths to read |

**Cross-provider equivalents:**
- Claude Code: Multiple `Read` calls (no batch equivalent)
- Cursor: `read_file` (called per file)

**Sensitivity:** None (read-only)

---

### `write_file` (WriteFile)

**Purpose:** Creates or overwrites a file with provided content. Creates parent directories if they don't exist. Shows diff before writing. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file_path` | string | yes | Target file path |
| `content` | string | yes | Full file content to write |

**Cross-provider equivalents:**
- Claude Code: `Write`
- Cursor: `write_to_file`
- Windsurf: `write_to_file`

**Sensitivity:** SENSITIVE -- requires confirmation, shows diff

---

### `replace` (Edit)

**Purpose:** Performs targeted text replacement within a file. Uses multi-stage correction for accuracy. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file_path` | string | yes | File to edit |
| `old_string` | string | yes | Exact text to find |
| `new_string` | string | yes | Replacement text |
| `expected_replacements` | number | no | Expected number of matches (safety check) |

**Cross-provider equivalents:**
- Claude Code: `Edit` (similar search-and-replace model)
- Cursor: `edit_file` (uses diff-based edits)

**Sensitivity:** SENSITIVE -- requires confirmation, shows diff

---

### `list_directory` (ReadFolder)

**Purpose:** Lists files and subdirectories in a specified directory. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Directory path |
| `ignore` | string[] | no | Glob patterns to exclude |
| `respect_git_ignore` | boolean | no | Honor .gitignore rules |

**Cross-provider equivalents:**
- Claude Code: `Bash` with `ls`
- Cursor: `list_directory`

**Sensitivity:** None (read-only)

---

### `glob` (FindFiles)

**Purpose:** Locates files matching glob patterns. Returns absolute paths sorted by modification time. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `pattern` | string | yes | Glob pattern (e.g. `**/*.ts`) |
| `path` | string | no | Root directory for search |
| `case_sensitive` | boolean | no | Case-sensitive matching |
| `respect_git_ignore` | boolean | no | Honor .gitignore rules |

**Cross-provider equivalents:**
- Claude Code: `Glob`
- Cursor: `file_search`

**Sensitivity:** None (read-only)

---

### `search_file_content` (SearchText)

**Purpose:** Regex search across file contents. Returns matches with line numbers. Uses ripgrep when available. `[Official]`

Note: The user-provided list calls this `grep` / `grep_search`, but the official function name is `search_file_content`. The `grep_search` name appears in some config contexts (e.g., `tools.core` allowlists) as an alias.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `pattern` | string | yes | Regex search pattern |
| `path` | string | no | Root directory |
| `include` | string | no | File glob filter (e.g. `*.ts`) |

**Cross-provider equivalents:**
- Claude Code: `Grep`
- Cursor: `grep_search`

**Sensitivity:** None (read-only)

---

## Shell Tool

### `run_shell_command` (Shell)

**Purpose:** Executes shell commands as subprocesses. Primary mechanism for the agent to interact with the environment beyond file edits. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | Shell command to execute |
| `description` | string | no | Brief explanation of command purpose |
| `directory` | string | no | Working directory (relative to project root) |

**Platform behavior:**
- Linux/macOS: `bash -c`
- Windows: `powershell.exe -NoProfile -Command`

**Configuration options:**
- `tools.core`: Allowlist specific command prefixes (e.g., `git`, `npm`)
- `tools.exclude`: Blocklist commands (takes precedence over allowlist)
- `tools.shell.enableInteractiveShell`: Enable PTY for `vim`, `nano`, `git rebase -i`
- `tools.shell.showColor`: Enable colored output (requires interactive shell)
- `tools.shell.pager`: Custom pager (default: `cat`)

**Security note:** Command-specific restrictions are based on simple string matching and can be easily bypassed. This is NOT a security mechanism. `[Official]`

**Cross-provider equivalents:**
- Claude Code: `Bash`
- Cursor: `run_terminal_command`

**Sensitivity:** SENSITIVE -- requires confirmation, shows exact command

---

## Web Tools

### `google_web_search` (GoogleSearch)

**Purpose:** Performs web searches using Google Search grounding. Returns search results for the model to synthesize. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Search query |

**Cross-provider equivalents:**
- Claude Code: `WebSearch`
- Cursor: No built-in equivalent (uses MCP)

**Sensitivity:** SENSITIVE -- network access, requires confirmation

---

### `web_fetch` (WebFetch)

**Purpose:** Fetches and processes content from one or more URLs (up to 20). Uses Gemini API's `urlContext` for retrieval; falls back to direct fetch if API fails. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` | string | yes | Prompt containing URL(s) and processing instructions |

Note: URLs are embedded directly in the prompt string rather than as separate parameters. The prompt must contain at least one URL starting with `http://` or `https://`.

**Cross-provider equivalents:**
- Claude Code: `WebFetch`
- Cursor: No built-in equivalent

**Sensitivity:** SENSITIVE -- network access, requires confirmation

---

## Memory and Planning Tools

### `save_memory` (SaveMemory)

**Purpose:** Saves facts to persistent memory file (`~/.gemini/GEMINI.md`) for recall across sessions. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `fact` | string | yes | Clear, self-contained statement to remember |

Facts are appended under a `## Gemini Added Memories` section. Users can manually view/edit the file.

**Cross-provider equivalents:**
- Claude Code: No direct equivalent (uses `CLAUDE.md` but it's user-managed, not tool-writable)
- Cursor: `.cursorrules` (manual only)

**Sensitivity:** None

---

### `write_todos` (WriteTodos)

**Purpose:** Manages a list of subtasks for complex plans. Renders as a todo tray in the CLI UI. `[Community]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `todos` | object[] | yes | Array of todo items with text and status |

**Status:** Historically used replace-all semantics (rewrite entire list on each update). Was temporarily disabled for Gemini 3 models as a "net-negative" (see [issue #17035](https://github.com/google-gemini/gemini-cli/issues/17035)). Being reworked toward granular push/pop/update operations. `[Community]`

**Cross-provider equivalents:**
- Claude Code: `TodoWrite` (similar concept)
- Cursor: No equivalent

**Sensitivity:** None

---

### `enter_plan_mode` (EnterPlanMode)

**Purpose:** Switches to read-only research mode. Routes requests to a high-reasoning Pro model. Restricts write operations to `.md` files in plan directories only. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (none documented) | - | - | Triggered by model or `/plan` command |

**Plan mode restricts tools to:**
- Read-only: `read_file`, `list_directory`, `glob`, `search_file_content`
- Search: `google_web_search`, `get_internal_docs`
- Research subagents: `codebase_investigator`, `cli_help`
- Limited writes: `write_file`/`replace` for `.md` files in plan dirs only
- `save_memory`, `ask_user`, `activate_skill` (read-only)

**Cross-provider equivalents:**
- Claude Code: No equivalent (no modal planning mode)
- Cursor: No equivalent

---

### `exit_plan_mode` (ExitPlanMode)

**Purpose:** Transitions from planning to implementation. Automatically switches to a high-speed Flash model. In non-interactive environments, engages YOLO mode. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (none documented) | - | - | Triggered after plan approval |

**Cross-provider equivalents:** None

---

## Interaction Tools

### `ask_user` (AskUser)

**Purpose:** Asks the user 1-4 structured questions to gather preferences, clarify requirements, or make decisions. Supports multiple question types. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `questions` | object[] | yes | Array of 1-4 question objects |

**Question object properties:**

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `question` | string | yes | Full question text |
| `header` | string | yes | Short label, max 16 chars (e.g. "Auth", "Database") |
| `type` | string | no | `'choice'`, `'text'`, or `'yesno'` (default: `'choice'`) |
| `options` | object[] | conditional | 2-4 selectable options (for `choice` type) |
| `multiSelect` | boolean | no | Allow multiple selections |
| `placeholder` | string | no | Hint text for input fields |

**Output:** JSON string with user responses indexed by question position.

**Cross-provider equivalents:**
- Claude Code: No structured equivalent (uses plain text)
- Cursor: No equivalent

**Sensitivity:** Inherently interactive (always requires user input by definition)

---

## Knowledge and Skill Tools

### `get_internal_docs`

**Purpose:** Accesses Gemini CLI's own internal documentation to help answer questions about the CLI itself. Allowed in plan mode. `[Inferred]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (not documented) | - | - | Likely accepts a query or topic |

**Cross-provider equivalents:** None (most tools don't have self-documentation tools)

**Sensitivity:** None (read-only, internal)

---

### `activate_skill`

**Purpose:** Loads specialized procedural expertise (SKILL.md files) into the session context. When a user's request matches a skill's description, the model triggers this tool to inject the skill's full instructions and bundled assets. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (not fully documented) | - | - | Likely accepts skill name/identifier |

**Behavior:**
- Reads the full body of the corresponding SKILL.md
- Returns content to the model as injected context
- Registered in security policy engine
- Built-in skills may bypass confirmation; custom skills require ASK_USER confirmation

**Cross-provider equivalents:**
- Claude Code: Skills (`.claude/skills/`) -- loaded automatically, no tool call needed
- Cursor: No equivalent

**Sensitivity:** SENSITIVE -- structural instruction injection, requires confirmation for custom skills

---

## Subagent Tools

### `codebase_investigator`

**Purpose:** A specialized subagent (not a simple tool) that analyzes codebases, reverse-engineers dependencies, and maps complex relationships. Operates in its own context window with a restricted toolset. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (invoked via `@codebase_investigator` or automatically by main agent) | - | - | Accepts natural language task description |

**Key characteristics:**
- Independent context window (saves tokens in main conversation)
- Non-interactive: executes task and reports back
- Enabled by default in plan mode
- Has its own system prompt and persona

**Cross-provider equivalents:**
- Claude Code: No equivalent (agent uses tools directly)
- Cursor: No equivalent

**Sensitivity:** None (read-only research agent)

---

## Tracker Tools (v0.34.0+)

Added in [v0.34.0](https://github.com/google-gemini/gemini-cli/releases/tag/v0.34.0) via PR #19489. These provide CRUD operations for task tracking with visualization. `[Community]`

**Note:** PR #21355 ("fix: logic for task tracker strategy and remove tracker tools") suggests these tools were reorganized or partially removed. The exact current state is unclear. The tool names below are from the original PR and release notes.

### `create_task`

**Purpose:** Creates a new task in the tracker. `[Unverified]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (not publicly documented) | - | - | Likely: title, description, status |

### `update_task`

**Purpose:** Updates an existing task's status or details. `[Unverified]`

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (not publicly documented) | - | - | Likely: task ID, new status/fields |

### `get_task`

**Purpose:** Retrieves details of a specific task. `[Unverified]`

### `list_tasks`

**Purpose:** Lists all tasks in the tracker. `[Unverified]`

### `add_dependency`

**Purpose:** Adds a dependency relationship between tasks. `[Unverified]`

### `visualize`

**Purpose:** Renders a graph visualization of tasks and their dependencies. Confirmed as `tracker_visualize` in PR #21455 fix. `[Community]`

**Cross-provider equivalents (all tracker tools):**
- Claude Code: `TodoWrite` (flat list only, no dependencies or visualization)
- Cursor: No equivalent

**Status:** Experimental/unstable. The tracker strategy logic had bugs (issue #21357) and the tools may have been removed or renamed in subsequent releases.

---

## Tools NOT in the User-Provided List

The following tools were discovered during research but were NOT in the original list provided:

| Tool | Type | Notes |
|------|------|-------|
| `codebase_investigator` | Subagent | Research subagent, not a simple tool |
| `cli_help` | Subagent | Built-in help subagent for plan mode |
| `exit_plan_mode` | Planning | Counterpart to `enter_plan_mode` |

---

## Cross-Provider Mapping Summary

| Gemini CLI | Claude Code | Cursor | Category |
|------------|-------------|--------|----------|
| `read_file` | `Read` | `read_file` | File read |
| `read_many_files` | Multiple `Read` | Multiple `read_file` | File read (batch) |
| `write_file` | `Write` | `write_to_file` | File write |
| `replace` | `Edit` | `edit_file` | File edit |
| `list_directory` | `Bash` (ls) | `list_directory` | Directory listing |
| `glob` | `Glob` | `file_search` | File search |
| `search_file_content` | `Grep` | `grep_search` | Content search |
| `run_shell_command` | `Bash` | `run_terminal_command` | Shell execution |
| `google_web_search` | `WebSearch` | -- | Web search |
| `web_fetch` | `WebFetch` | -- | Web fetch |
| `save_memory` | -- (manual CLAUDE.md) | -- (manual .cursorrules) | Persistent memory |
| `write_todos` | `TodoWrite` | -- | Task tracking |
| `ask_user` | -- | -- | User interaction |
| `enter_plan_mode` | -- | -- | Planning mode |
| `exit_plan_mode` | -- | -- | Planning mode |
| `activate_skill` | -- (auto-loaded) | -- | Skill injection |
| `get_internal_docs` | -- | -- | Self-documentation |
| `codebase_investigator` | -- | -- | Research subagent |
| `create_task` | -- | -- | Task tracker |
| `update_task` | -- | -- | Task tracker |
| `get_task` | -- | -- | Task tracker |
| `list_tasks` | -- | -- | Task tracker |
| `add_dependency` | -- | -- | Task tracker |
| `visualize` | -- | -- | Task tracker |
