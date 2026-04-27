# Claude Code Built-in Tools Reference

> Comprehensive reference for all tools available to the Claude Code AI agent.
> Last updated: 2026-03-20. Based on Claude Code v2.1.80.

## Sources

All claims are tagged with attribution:

- `[Official]` -- from Anthropic docs or official repo
- `[Community]` -- from the Piebald-AI/claude-code-system-prompts repo (tracks every CC release)
- `[Inferred]` -- derived from tool schema introspection in a live Claude Code session

Primary sources:
- [Official Tools Reference](https://code.claude.com/docs/en/tools-reference) `[Official]`
- [Official Permissions Docs](https://code.claude.com/docs/en/permissions) `[Official]`
- [Official Sub-agents Docs](https://code.claude.com/docs/en/sub-agents) `[Official]`
- [Piebald-AI/claude-code-system-prompts](https://github.com/Piebald-AI/claude-code-system-prompts) `[Community]`
- [Internal tools implementation gist](https://gist.github.com/bgauryy/0cdb9aa337d01ae5bd0c803943aa36bd) `[Community]`

---

## Quick Reference Table

All 28 tools listed on the official tools reference page, plus additional tools discovered from system prompt analysis.

| Tool | Permission Required | Category | Available In |
|------|:-------------------:|----------|-------------|
| `Agent` | No | Agent Management | Interactive, SDK |
| `AskUserQuestion` | No | User Interaction | Interactive only |
| `Bash` | Yes | Execution | All modes |
| `CronCreate` | No | Scheduling | Interactive only |
| `CronDelete` | No | Scheduling | Interactive only |
| `CronList` | No | Scheduling | Interactive only |
| `Edit` | Yes | File Operations | All modes |
| `EnterPlanMode` | No | Planning | Interactive only |
| `EnterWorktree` | No | Workspace | Interactive only |
| `ExitPlanMode` | Yes | Planning | Interactive only |
| `ExitWorktree` | No | Workspace | Interactive only |
| `Glob` | No | Search | All modes |
| `Grep` | No | Search | All modes |
| `ListMcpResourcesTool` | No | MCP Integration | All modes |
| `LSP` | No | Code Intelligence | All modes (requires plugin) |
| `NotebookEdit` | Yes | File Operations | All modes |
| `Read` | No | File Operations | All modes |
| `ReadMcpResourceTool` | No | MCP Integration | All modes |
| `Skill` | Yes | Extensibility | Interactive, SDK |
| `TaskCreate` | No | Task Management | Interactive only |
| `TaskGet` | No | Task Management | Interactive only |
| `TaskList` | No | Task Management | Interactive only |
| `TaskOutput` | No | Agent Management | All modes |
| `TaskStop` | No | Agent Management | All modes |
| `TaskUpdate` | No | Task Management | Interactive only |
| `TodoWrite` | No | Task Management | Non-interactive, SDK |
| `ToolSearch` | No | MCP Integration | All modes (when tool search enabled) |
| `WebFetch` | Yes | Web | All modes |
| `WebSearch` | Yes | Web | All modes |
| `Write` | Yes | File Operations | All modes |

### Additional Tools (not on official reference page)

These tools appear in system prompt analysis but are not listed on the official tools reference:

| Tool | Category | Notes |
|------|----------|-------|
| `SendMessageTool` | Agent Teams | For inter-agent communication in teams `[Community]` |
| `TeammateTool` | Agent Teams | Create/manage agent teams `[Community]` |
| `TeamDelete` | Agent Teams | Delete agent teams `[Community]` |
| `Sleep` | Execution | Pause with early-wake capability `[Community]` |
| `Computer` | Screen Interaction | Screen/mouse/keyboard control `[Community]` |

---

## Detailed Tool Documentation

### File Operations

#### `Read`

**Purpose:** Read file contents from the local filesystem. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `file_path` | string | Yes | Absolute path to the file |
| `offset` | number | No | Line number to start reading from (1-based) |
| `limit` | number | No | Number of lines to read (default: 2000) |
| `pages` | string | No | Page range for PDFs (e.g., "1-5"). Max 20 pages per request |

**Behavior notes:**
- Returns content with `cat -n` format (line numbers starting at 1) `[Inferred]`
- Can read images (PNG, JPG, etc.) -- content presented visually via multimodal `[Inferred]`
- Can read PDFs. Large PDFs (10+ pages) require the `pages` parameter `[Inferred]`
- Can read Jupyter notebooks (.ipynb) -- returns all cells with outputs `[Inferred]`
- Cannot read directories -- use `ls` via Bash for that `[Inferred]`
- No permission required (read-only operation) `[Official]`

**Permission rule syntax:** `Read(./.env)`, `Read(/src/**)`, `Read(~/.zshrc)` `[Official]`

**Cross-provider equivalents:** Cursor has `read_file`, Windsurf has `read_file`, Copilot has file reading capabilities. All AI coding tools have a file read primitive. `[Inferred]`

---

#### `Write`

**Purpose:** Create new files or completely overwrite existing files. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `file_path` | string | Yes | Absolute path to the file to write |
| `content` | string | Yes | The content to write |

**Behavior notes:**
- Overwrites existing files entirely `[Inferred]`
- Requires reading the file first (via `Read`) if the file already exists -- will fail otherwise `[Inferred]`
- For modifying existing files, prefer `Edit` (sends only the diff) `[Inferred]`
- Permission required for every use `[Official]`

**Permission rule syntax:** Follows `Edit` rules (same path patterns). `[Official]`

**Cross-provider equivalents:** Cursor `write_to_file`, Windsurf `write_to_file`. Universal primitive. `[Inferred]`

---

#### `Edit`

**Purpose:** Perform exact string replacements in existing files. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `file_path` | string | Yes | Absolute path to the file |
| `old_string` | string | Yes | The text to replace (must be unique in file) |
| `new_string` | string | Yes | The replacement text (must differ from old_string) |
| `replace_all` | boolean | No | Replace all occurrences (default: false) |

**Behavior notes:**
- Fails if `old_string` is not unique in the file (unless `replace_all` is true) `[Inferred]`
- Must read the file first in the conversation before editing `[Inferred]`
- `replace_all` is useful for variable renaming across a file `[Inferred]`
- Edit rules also apply to `Write` and `NotebookEdit` in the permission system `[Official]`

**Permission rule syntax:** `Edit(/src/**/*.ts)`, `Edit(/docs/**)` `[Official]`

**Cross-provider equivalents:** Cursor uses `edit_file` with targeted edits, Windsurf uses `write_to_file` for all modifications. Claude Code's search-and-replace approach is distinctive. `[Inferred]`

---

#### `NotebookEdit`

**Purpose:** Modify Jupyter notebook (.ipynb) cells. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `notebook_path` | string | Yes | Absolute path to the notebook |
| `new_source` | string | Yes | New content for the cell |
| `cell_id` | string | No | ID of the cell to edit. For insert mode, new cell goes after this |
| `cell_type` | enum | No | `"code"` or `"markdown"`. Required for insert mode |
| `edit_mode` | enum | No | `"replace"` (default), `"insert"`, or `"delete"` |

**Behavior notes:**
- Cell numbering is 0-indexed `[Inferred]`
- Permission required `[Official]`

**Cross-provider equivalents:** Cursor and Windsurf handle notebooks via their file edit tools. Claude Code has a dedicated notebook tool. `[Inferred]`

---

### Search Tools

#### `Glob`

**Purpose:** Fast file pattern matching -- find files by name pattern. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `pattern` | string | Yes | Glob pattern (e.g., `**/*.js`, `src/**/*.ts`) |
| `path` | string | No | Directory to search in (defaults to cwd) |

**Behavior notes:**
- Returns matching file paths sorted by modification time `[Inferred]`
- Preferred over `find` or `ls` via Bash `[Inferred]`
- No permission required `[Official]`

**Cross-provider equivalents:** Cursor has `list_files`, Windsurf has `list_files`. All tools have file discovery. `[Inferred]`

---

#### `Grep`

**Purpose:** Search file contents using ripgrep. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `pattern` | string | Yes | Regex pattern to search for |
| `path` | string | No | File or directory to search (defaults to cwd) |
| `output_mode` | enum | No | `"files_with_matches"` (default), `"content"`, or `"count"` |
| `glob` | string | No | Glob to filter files (e.g., `"*.js"`) |
| `type` | string | No | File type filter (e.g., `"js"`, `"py"`, `"go"`) |
| `-i` | boolean | No | Case-insensitive search |
| `-n` | boolean | No | Show line numbers (default: true, content mode only) |
| `-A` | number | No | Lines after match (content mode only) |
| `-B` | number | No | Lines before match (content mode only) |
| `-C` / `context` | number | No | Lines of context around match (content mode only) |
| `multiline` | boolean | No | Enable multiline matching (default: false) |
| `head_limit` | number | No | Limit output to first N entries |
| `offset` | number | No | Skip first N entries before applying head_limit |

**Behavior notes:**
- Built on ripgrep, NOT grep -- literal braces need escaping (e.g., `interface\{\}`) `[Inferred]`
- Preferred over running `grep` or `rg` via Bash `[Inferred]`
- No permission required `[Official]`

**Cross-provider equivalents:** Cursor has `grep_search` / `codebase_search`, Windsurf has `search`. All tools have content search. `[Inferred]`

---

#### `ToolSearch`

**Purpose:** Search for and load deferred MCP tools when tool search is enabled. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `query` | string | Yes | Search query. Supports `"select:ToolA,ToolB"` for exact selection or keywords |
| `max_results` | number | No | Maximum results to return (default: 5) |

**Behavior notes:**
- Only available when MCP tool search is enabled `[Official]`
- Used to lazily load MCP tool schemas -- tools cannot be called until their schema is fetched via this tool `[Inferred]`
- No permission required `[Official]`

**Cross-provider equivalents:** Claude Code-specific. Other tools load all MCP tools eagerly. `[Inferred]`

---

### Execution

#### `Bash`

**Purpose:** Execute shell commands in the user's environment. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `command` | string | Yes | The command to execute |
| `description` | string | No | Human-readable description of what the command does |
| `timeout` | number | No | Timeout in milliseconds (max 600,000 = 10 min, default 120,000 = 2 min) |
| `run_in_background` | boolean | No | Run in background; output retrieved later via TaskOutput |
| `dangerouslyDisableSandbox` | boolean | No | Override sandbox mode |

**Behavior notes:**
- Working directory persists between commands; shell state (env vars, aliases) does NOT `[Official]`
- Shell environment initialized from user's profile (bash or zsh) `[Inferred]`
- Permission required for every command execution `[Official]`
- Compound commands (with `&&`) generate separate permission rules per subcommand `[Official]`
- Sandboxing provides OS-level filesystem/network isolation `[Official]`
- Set `CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR=1` to reset cwd after each command `[Official]`

**Permission rule syntax:** `Bash(npm run build)`, `Bash(npm run *)`, `Bash(git commit *)`, `Bash(* --version)` `[Official]`

**Cross-provider equivalents:** Cursor has `run_terminal_command`, Windsurf has `run_command`. Universal primitive, but Claude Code's permission/sandbox system is more granular. `[Inferred]`

---

### Agent Management

#### `Agent`

**Purpose:** Spawn a subagent with its own context window to handle a task. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `prompt` | string | Yes | The task description/instructions for the subagent |
| `description` | string | Yes | Brief description of what the subagent will do |
| `subagent_type` | string | Yes | Which subagent to use (e.g., `"Explore"`, `"Plan"`, custom name) |

**Behavior notes:**
- Renamed from `Task` in v2.1.63; `Task(...)` references still work as aliases `[Official]`
- Built-in subagent types: `Explore` (Haiku, read-only), `Plan` (inherits model, read-only), general-purpose (all tools) `[Official]`
- Subagents cannot spawn other subagents `[Official]`
- Can run in foreground (blocking) or background (concurrent) `[Official]`
- Use `Agent(worker, researcher)` in permission rules to restrict which subagents can be spawned `[Official]`
- No permission required to spawn `[Official]`

**Permission rule syntax:** `Agent(Explore)`, `Agent(my-custom-agent)` `[Official]`

**Cross-provider equivalents:** Claude Code-specific. No other AI coding tool has built-in subagent spawning. Cursor/Windsurf lack this capability. `[Inferred]`

---

#### `TaskOutput`

**Purpose:** Retrieve output from a background task (Bash or Agent). `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `bash_id` / `task_id` | string | Yes | ID of the background task |
| `filter` | string | No | Filter/extract specific output |

**Behavior notes:**
- Used to read incremental output from background Bash commands or subagents `[Community]`
- No permission required `[Official]`

**Cross-provider equivalents:** Claude Code-specific (tied to background execution model). `[Inferred]`

---

#### `TaskStop`

**Purpose:** Kill a running background task by ID. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `task_id` | string | Yes | ID of the task to stop |

**Behavior notes:**
- No permission required `[Official]`

---

### Task Management

Claude Code has two task management systems that serve different contexts:

- **Interactive mode:** Uses `TaskCreate`, `TaskGet`, `TaskList`, `TaskUpdate` `[Official]`
- **Non-interactive / SDK mode:** Uses `TodoWrite` `[Official]`

#### `TaskCreate`

**Purpose:** Create a new task in the session task list. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `subject` | string | Yes | Brief, actionable imperative title |
| `description` | string | Yes | Detailed explanation with context and acceptance criteria |
| `activeForm` | string | No | Present continuous form (e.g., "Running tests"). Defaults to subject |
| `status` | string | Yes | Always `"pending"` for new tasks |

**Behavior notes:**
- Use for tasks with 3+ distinct steps `[Community]`
- Check `TaskList` first to prevent duplicates `[Community]`
- No permission required `[Official]`

---

#### `TaskGet`

**Purpose:** Retrieve full details for a specific task. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `task_id` | string | Yes | ID of the task |

---

#### `TaskList`

**Purpose:** List all tasks with their current status. `[Official]`

Takes no parameters. Returns all tasks in the session. `[Community]`

---

#### `TaskUpdate`

**Purpose:** Update task status, dependencies, details, or delete tasks. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `task_id` | string | Yes | ID of the task to update |
| `status` | string | No | New status: `"pending"`, `"in_progress"`, `"completed"` |
| `description` | string | No | Updated description |
| `dependencies` | array | No | Task IDs this task depends on |

**Behavior notes:**
- Maintain exactly one `in_progress` task at a time `[Community]`
- Mark tasks complete immediately upon finishing `[Community]`

---

#### `TodoWrite`

**Purpose:** Manage session task checklist (non-interactive and SDK mode). `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `todos` | array | Yes | Array of todo items |

Each todo item has:
| Field | Type | Description |
|-------|------|-------------|
| `content` | string | Imperative form of what needs to be done |
| `status` | string | `"pending"`, `"in_progress"`, or `"completed"` |
| `activeForm` | string | Present continuous: what is being done |

**Behavior notes:**
- Replaces the entire todo list each call (not additive) `[Community]`
- Available in non-interactive mode and Agent SDK; interactive sessions use TaskCreate/TaskUpdate instead `[Official]`
- No permission required `[Official]`

**Cross-provider equivalents:** Claude Code-specific. Other tools do not have built-in task tracking. `[Inferred]`

---

### User Interaction

#### `AskUserQuestion`

**Purpose:** Ask interactive multiple-choice questions to gather requirements or clarify ambiguity. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `questions` | array | Yes | Array of 1-4 question objects |

Each question object:
| Field | Type | Description |
|-------|------|-------------|
| `question` | string | The question text |
| `header` | string | Short header (max 12 chars) |
| `multiSelect` | boolean | Allow multiple selections |
| `options` | array | 2-4 option objects |

Each option:
| Field | Type | Description |
|-------|------|-------------|
| `label` | string | Option text (1-5 words) |
| `description` | string | Longer description |

**Behavior notes:**
- Users can always select "Other" for custom text responses `[Community]`
- Place recommended options first, append "(Recommended)" to label `[Community]`
- In plan mode, do not ask confirmation questions -- use ExitPlanMode instead `[Community]`
- Foreground subagents pass questions through to user; background subagents cannot ask questions `[Official]`
- No permission required `[Official]`

**Cross-provider equivalents:** Claude Code-specific. Other AI coding tools ask questions as plain text in the chat. The structured multiple-choice UI is unique to Claude Code. `[Inferred]`

---

### Planning

#### `EnterPlanMode`

**Purpose:** Switch to plan mode for safe code analysis before implementation. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| (none documented) | | | |

**Behavior notes:**
- In plan mode, Claude can analyze but not modify files or execute commands `[Official]`
- No permission required `[Official]`

---

#### `ExitPlanMode`

**Purpose:** Present a plan for user approval and exit plan mode. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `plan` | string | Yes | The implementation plan to present |

**Behavior notes:**
- Permission required (because it transitions to execution mode) `[Official]`

**Cross-provider equivalents:** Claude Code-specific. Cursor has "plan mode" but it is conversational, not tool-based. `[Inferred]`

---

### Workspace Management

#### `EnterWorktree`

**Purpose:** Create an isolated git worktree and switch the session into it. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `name` | string | No | Name for the worktree (letters, digits, dots, underscores, dashes; max 64 chars). Random if omitted |

**Behavior notes:**
- Creates worktree inside `.claude/worktrees/` with a new branch based on HEAD `[Inferred]`
- Only use when the user explicitly says "worktree" `[Inferred]`
- Must be in a git repo (or have WorktreeCreate/WorktreeRemove hooks configured) `[Inferred]`
- Cannot already be in a worktree `[Inferred]`
- No permission required `[Official]`

---

#### `ExitWorktree`

**Purpose:** Exit a worktree session and return to the original working directory. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `action` | enum | Yes | `"keep"` (preserve worktree) or `"remove"` (delete it) |
| `discard_changes` | boolean | No | Must be true to force-remove a worktree with uncommitted changes |

**Behavior notes:**
- Only operates on worktrees created by EnterWorktree in the current session `[Inferred]`
- No permission required `[Official]`

**Cross-provider equivalents:** Claude Code-specific. No other AI coding tool has worktree management. `[Inferred]`

---

### Web Tools

#### `WebFetch`

**Purpose:** Fetch content from a URL and process it with an AI model. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `url` | string (URI) | Yes | The URL to fetch |
| `prompt` | string | Yes | Instructions for what to extract from the page |

**Behavior notes:**
- HTML is converted to markdown, then processed by a small/fast model `[Inferred]`
- Will fail for authenticated or private URLs (Google Docs, Confluence, etc.) `[Inferred]`
- HTTP auto-upgraded to HTTPS `[Inferred]`
- 15-minute self-cleaning cache `[Inferred]`
- Redirects to different hosts are reported, requiring a new fetch `[Inferred]`
- For GitHub URLs, prefer `gh` CLI via Bash `[Inferred]`
- Permission required `[Official]`

**Permission rule syntax:** `WebFetch(domain:example.com)` `[Official]`

**Cross-provider equivalents:** Cursor lacks built-in web fetching. Most tools rely on MCP or browser tools. `[Inferred]`

---

#### `WebSearch`

**Purpose:** Search the web for current information. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `query` | string | Yes | Search query (min 2 chars) |
| `allowed_domains` | string[] | No | Only include results from these domains |
| `blocked_domains` | string[] | No | Exclude results from these domains |

**Behavior notes:**
- Only available in the US `[Inferred]`
- Returns search result blocks with markdown hyperlinks `[Inferred]`
- Permission required `[Official]`

**Cross-provider equivalents:** Cursor lacks built-in web search. Most tools use MCP-based search. `[Inferred]`

---

### MCP Integration

#### `ListMcpResourcesTool`

**Purpose:** List resources exposed by connected MCP servers. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `server` | string | No | Specific MCP server to query |

**Behavior notes:**
- No permission required `[Official]`

---

#### `ReadMcpResourceTool`

**Purpose:** Read a specific MCP resource by URI. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `server` | string | Yes | MCP server name |
| `uri` | string | Yes | Resource URI |

**Behavior notes:**
- No permission required `[Official]`

---

### Code Intelligence

#### `LSP`

**Purpose:** Code intelligence via Language Server Protocol -- type errors, navigation, and symbol information. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `filePath` | string | Yes | Target file path |
| `line` | number | Yes | Line number (1-based) |
| `character` | number | Yes | Character offset (1-based) |
| `operation` | string | Yes | One of the supported operations (see below) |

**Supported operations:** `[Community]`
| Operation | Description |
|-----------|-------------|
| `goToDefinition` | Locate symbol definitions |
| `findReferences` | Find all references to a symbol |
| `hover` | Get documentation and type info |
| `documentSymbol` | List all symbols in a file |
| `workspaceSymbol` | Search symbols across workspace |
| `goToImplementation` | Find interface/abstract implementations |
| `prepareCallHierarchy` | Get call hierarchy items |
| `incomingCalls` | Find callers of a function |
| `outgoingCalls` | Find functions called by a function |

**Behavior notes:**
- Requires a code intelligence plugin and its language server binary `[Official]`
- Automatically reports type errors/warnings after file edits `[Official]`
- LSP servers must be configured for the file type `[Community]`
- No permission required `[Official]`

**Cross-provider equivalents:** Cursor and Windsurf have native IDE LSP integration (not exposed as an agent tool). Claude Code uniquely exposes LSP as an explicit tool the agent can call. `[Inferred]`

---

### Extensibility

#### `Skill`

**Purpose:** Execute a user-defined skill (slash command) within the main conversation. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `skill` | string | Yes | Skill name (e.g., `"commit"`, `"review-pr"`, `"plugin:skill-name"`) |
| `args` | string | No | Optional arguments for the skill |

**Behavior notes:**
- Skills are referenced by users as slash commands (e.g., `/commit`) `[Inferred]`
- Invoked BEFORE generating any other response when a matching skill is detected `[Inferred]`
- Available skills listed in system-reminder messages `[Inferred]`
- Permission required `[Official]`

**Cross-provider equivalents:** Cursor has custom instructions but not invocable skills. Claude Code's skill system is unique. `[Inferred]`

---

### Scheduling

#### `CronCreate`

**Purpose:** Schedule a recurring or one-shot prompt within the current session. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `schedule` | string | Yes | 5-field cron expression in user's local timezone |
| `prompt` | string | Yes | The prompt to enqueue when the schedule fires |
| `recurring` | boolean | No | Whether it repeats (default: true) |

**Behavior notes:**
- Jobs only fire during REPL idle periods `[Community]`
- One-shot tasks (recurring: false) auto-delete after firing `[Community]`
- Deterministic jitter applied: recurring tasks may fire up to 10% late (max 15 min) `[Community]`
- Avoid :00 and :30 minute marks for better load distribution `[Community]`
- Gone when Claude exits (session-scoped) `[Official]`
- No permission required `[Official]`

**Cross-provider equivalents:** Claude Code-specific. No other AI coding tool has scheduling. `[Inferred]`

---

#### `CronDelete`

**Purpose:** Cancel a scheduled task by ID. `[Official]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `job_id` | string | Yes | ID returned by CronCreate |

---

#### `CronList`

**Purpose:** List all scheduled tasks in the session. `[Official]`

Takes no parameters.

---

### Agent Teams (Advanced)

These tools are available for multi-agent team workflows and are not on the standard tools reference page.

#### `SendMessageTool`

**Purpose:** Send messages between agents in a team configuration. `[Community]`

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `to` | string | Yes | Recipient name or `"*"` for broadcast |
| `message` | string or object | Yes | Plain text or structured protocol message |
| `summary` | string | Yes | 5-10 word preview for UI display |

**Structured message types:** `[Community]`
- `shutdown_request` / `shutdown_response` -- graceful agent shutdown
- `plan_approval_response` -- approve/reject teammate's plan

---

#### `TeammateTool`

**Purpose:** Create and manage agent teams for parallel work. `[Community]`

---

#### `TeamDelete`

**Purpose:** Delete an agent team. `[Community]`

---

#### `Sleep`

**Purpose:** Pause execution with early-wake capability. `[Community]`

---

#### `Computer`

**Purpose:** Screen interaction -- mouse, keyboard, screenshots. `[Community]`

**Behavior notes:**
- Used for GUI automation scenarios `[Community]`
- Likely tied to specific deployment contexts (desktop app) `[Inferred]`

---

## Permission Rule Syntax Summary

Permission rules are used in `settings.json` under `permissions.allow`, `permissions.deny`, or `permissions.ask`, and via CLI flags `--allowedTools` / `--disallowedTools`. `[Official]`

```json
{
  "permissions": {
    "allow": [
      "Bash(npm run *)",
      "Bash(git commit *)",
      "Edit(/src/**/*.ts)",
      "Read",
      "WebFetch(domain:example.com)"
    ],
    "deny": [
      "Bash(git push *)",
      "Read(./.env)",
      "Agent(Explore)"
    ]
  }
}
```

**Evaluation order:** deny -> ask -> allow. First match wins. `[Official]`

### Tool-specific specifier patterns

| Tool | Specifier Syntax | Example |
|------|-----------------|---------|
| `Bash` | Command with glob wildcards | `Bash(npm run *)` |
| `Read` | gitignore-style path patterns | `Read(/src/**)` |
| `Edit` | gitignore-style path patterns | `Edit(~/.config/**)` |
| `Write` | Same as Edit | `Write(//tmp/scratch.txt)` |
| `WebFetch` | `domain:` prefix | `WebFetch(domain:github.com)` |
| `Agent` | Subagent name | `Agent(Explore)` |
| MCP tools | `mcp__server__tool` format | `mcp__puppeteer__puppeteer_navigate` |

### Path pattern types `[Official]`

| Pattern | Meaning | Example |
|---------|---------|---------|
| `//path` | Absolute from filesystem root | `Read(//Users/alice/secrets/**)` |
| `~/path` | Relative to home directory | `Read(~/Documents/*.pdf)` |
| `/path` | Relative to project root | `Edit(/src/**/*.ts)` |
| `path` or `./path` | Relative to current directory | `Read(*.env)` |

---

## Subagent Tool Access

When a subagent is spawned via `Agent`, its tool access can be controlled: `[Official]`

- **Default:** Inherits all tools from parent conversation
- **`tools` field:** Allowlist -- only specified tools available
- **`disallowedTools` field:** Denylist -- specified tools removed from inherited set
- If both set, `disallowedTools` applied first, then `tools` resolved against remainder

Built-in subagent tool restrictions: `[Official]`

| Subagent | Model | Tool Access |
|----------|-------|-------------|
| Explore | Haiku | Read-only (no Write, Edit) |
| Plan | Inherits | Read-only (no Write, Edit) |
| General-purpose | Inherits | All tools |

---

## Version History

| Version | Change |
|---------|--------|
| v2.1.63 | `Task` tool renamed to `Agent`; `Task(...)` still works as alias `[Official]` |
| v2.1.80 | Current version with 28+ tools on official reference `[Official]` |

---

## Notes for Cross-Provider Mapping

Tools that are **unique to Claude Code** (no direct equivalent in Cursor, Windsurf, or Copilot):
- `Agent` (subagent spawning)
- `AskUserQuestion` (structured multiple-choice UI)
- `EnterPlanMode` / `ExitPlanMode` (tool-based plan mode)
- `EnterWorktree` / `ExitWorktree` (git worktree management)
- `TaskCreate` / `TaskGet` / `TaskList` / `TaskUpdate` / `TodoWrite` (built-in task tracking)
- `CronCreate` / `CronDelete` / `CronList` (scheduling)
- `ToolSearch` (lazy MCP tool loading)
- `LSP` (explicit agent-callable LSP)
- `SendMessageTool` / `TeammateTool` / `TeamDelete` (agent teams)
- `ListMcpResourcesTool` / `ReadMcpResourceTool` (MCP resource access)
- `Skill` (invocable skill system)

Tools with **direct cross-provider equivalents:**
- `Read` -- universal
- `Write` -- universal
- `Edit` -- universal (implementations differ)
- `Bash` -- universal
- `Glob` -- universal (file discovery)
- `Grep` -- universal (content search)
- `WebFetch` -- some providers have this; many rely on MCP
- `WebSearch` -- some providers have this; many rely on MCP
- `NotebookEdit` -- some providers support notebooks
