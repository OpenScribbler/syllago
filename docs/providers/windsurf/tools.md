# Windsurf Cascade: Built-in Tools

> **Last updated:** 2026-03-21
>
> Windsurf (formerly Codeium) exposes these tools to its AI agent, Cascade.
> Tool definitions sourced from leaked system prompts (Feb 2025, April 2025)
> and official documentation. The April 2025 prompt is the most complete source.

## Sources

| Tag | Meaning |
|-----|---------|
| `[Official]` | From docs.windsurf.com or windsurf.com |
| `[Community]` | From leaked/extracted system prompts with multiple independent confirmations |
| `[Inferred]` | Deduced from behavior, changelogs, or indirect references |

Primary sources:
- [Windsurf Cascade docs](https://docs.windsurf.com/windsurf/cascade/cascade) `[Official]`
- [Windsurf llms-full.txt](https://docs.windsurf.com/llms-full.txt) `[Official]`
- [System prompt (Apr 2025)](https://github.com/dontriskit/awesome-ai-system-prompts/blob/main/windsurf/system-2025-04-20.md) `[Community]`
- [System prompt (Feb 2025)](https://gist.github.com/cedrickchee/98e0350697424fc611366dd3b75dce6c) `[Community]`

---

## Tool Summary

| # | Tool | Purpose | Claude Code Equivalent |
|---|------|---------|----------------------|
| 1 | `codebase_search` | Semantic code search | `Grep` / `mcp` semantic search |
| 2 | `grep_search` | Ripgrep pattern matching | `Grep` |
| 3 | `find_by_name` | File/dir search by glob | `Glob` |
| 4 | `list_dir` | List directory contents | `Bash` (`ls`) |
| 5 | `view_line_range` | View file lines (0-indexed) | `Read` |
| 6 | `view_file_outline` | View file structure/outline | No direct equivalent |
| 7 | `view_code_item` | View specific symbol definition | No direct equivalent |
| 8 | `search_in_file` | Search within a single file | `Grep` (single file) |
| 9 | `related_files` | Find related/adjacent files | No direct equivalent |
| 10 | `edit_file` | Edit existing file (diff-style) | `Edit` |
| 11 | `write_to_file` | Create new file | `Write` |
| 12 | `run_command` | Execute shell command | `Bash` |
| 13 | `command_status` | Check async command status | `Bash` (background task) |
| 14 | `read_url_content` | Fetch URL content | `WebFetch` |
| 15 | `view_web_document_content_chunk` | Paginate fetched web content | `WebFetch` (implicit) |
| 16 | `search_web` | Web search | `WebSearch` |
| 17 | `create_memory` | Persistent memory CRUD | No direct equivalent |
| 18 | `suggested_responses` | Suggest user reply options | No direct equivalent |
| 19 | `browser_preview` | Launch browser preview | No direct equivalent |
| 20 | `deploy_web_app` | Deploy web app (Netlify) | No direct equivalent |
| 21 | `read_deployment_config` | Check deploy readiness | No direct equivalent |
| 22 | `check_deploy_status` | Poll deployment status | No direct equivalent |
| 23 | `parallel` | Execute multiple tools at once | Native (parallel tool calls) |

---

## Tool Details

### 1. `codebase_search` `[Community]`

Semantic search across the codebase. Uses embeddings, not text matching. Best for
finding code by purpose/function rather than exact text.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Query` | string | Yes | Search query describing the code's purpose |
| `TargetDirectories` | string[] | Yes | Absolute paths to directories to search |

**Constraints:** Results capped at 50 matches. Only shows full content for top results; others may be truncated. Degrades with 500+ files. Not suitable for broad/vague queries.

**Cross-provider:** Claude Code has no built-in semantic search but can approximate with `Grep` or MCP-provided semantic search tools.

---

### 2. `grep_search` `[Community]`

Text-based pattern matching using ripgrep. Returns results in JSON format.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `SearchPath` | string | Yes | Directory or file path to search |
| `Query` | string | Yes | Search term or regex pattern |
| `CaseInsensitive` | boolean | No | Case-insensitive matching |
| `Includes` | string[] | No | File patterns or paths to include |
| `MatchPerLine` | boolean | No | Return matching lines with context |

**Constraints:** Results capped at 50 matches.

**Note:** The Feb 2025 prompt uses `SearchDirectory` instead of `SearchPath` and marks more parameters as required. The April 2025 version relaxes this.

**Cross-provider:** Direct equivalent of Claude Code's `Grep` tool.

---

### 3. `find_by_name` `[Community]`

File and directory search using fd (a fast `find` alternative). Supports glob patterns, respects `.gitignore`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `SearchDirectory` | string | Yes | Directory to search within |
| `Pattern` | string | No | Glob pattern to match |
| `Excludes` | string[] | No | Glob patterns to exclude |
| `Extensions` | string[] | No | File extensions to filter |
| `FullPath` | boolean | No | Match entire path vs filename only |
| `MaxDepth` | integer | No | Maximum search depth |
| `Type` | string | No | `"file"` or `"directory"` or `"any"` |

**Constraints:** Results capped at 50 matches.

**Cross-provider:** Claude Code's `Glob` tool serves a similar purpose.

---

### 4. `list_dir` `[Community]`

Lists directory contents with metadata (file sizes, child directory counts).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `DirectoryPath` | string | Yes | Absolute path to directory |

**Cross-provider:** Claude Code uses `Bash` with `ls` for this.

---

### 5. `view_line_range` `[Community]`

View specific line ranges from a file. Lines are 0-indexed.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `AbsolutePath` | string | Yes | Absolute file path |
| `StartLine` | integer | Yes | Starting line (0-indexed) |
| `EndLine` | integer | Yes | Ending line (inclusive) |

**Constraints:** Maximum 200 lines per call.

**Note:** The Feb 2025 prompt calls this `view_file` with the same parameters.

**Cross-provider:** Claude Code's `Read` tool (with `offset`/`limit` parameters, 1-indexed).

---

### 6. `view_file_outline` `[Community]`

Displays the structural outline of a file (classes, functions, imports). Described as "the preferred first-step tool for file exploration."

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `AbsolutePath` | string | Yes | Absolute file path |
| `ItemOffset` | integer | Yes | Pagination offset (start at 0) |

**Cross-provider:** No direct Claude Code equivalent. Can be approximated with `Bash` running tree-sitter or ctags.

---

### 7. `view_code_item` `[Community]`

Displays the full content of a specific code symbol (class, function) by its qualified name.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `File` | string | No | Absolute file path |
| `NodePath` | string | Yes | Hierarchical name (e.g., `ClassName.methodName`) |

**Note:** The Feb 2025 prompt uses `AbsolutePath` and `NodeName` instead.

**Constraints:** Should not be called for items already shown by `codebase_search`.

**Cross-provider:** No direct Claude Code equivalent. `Read` with line range serves a similar purpose.

---

### 8. `search_in_file` `[Community]`

Searches for code snippets within a single file matching a query.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `AbsolutePath` | string | Yes | File path to search |
| `Query` | string | Yes | Search query |

**Cross-provider:** Claude Code's `Grep` can target a single file.

---

### 9. `related_files` `[Community]`

Finds files related to or commonly used with a given file (e.g., test files, imports, co-edited files).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `absolutepath` | string | Yes | Input file's absolute path |

**Note:** Only confirmed in the Feb 2025 prompt. May have been renamed or merged in later versions.

**Cross-provider:** No direct Claude Code equivalent.

---

### 10. `edit_file` `[Community]`

Modifies an existing file using a diff-style format. Unchanged code is represented with `{{ ... }}` placeholders.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `TargetFile` | string | Yes | File to modify |
| `CodeEdit` | string | Yes | Edit content with `{{ ... }}` for unchanged lines |
| `CodeMarkdownLanguage` | string | Yes | Language identifier (e.g., `python`) |
| `Instruction` | string | Yes | Description of changes |
| `TargetLintErrorIds` | string[] | No | Related lint error IDs to resolve |

**Constraints:** Cannot parallel-edit the same file. Cannot edit `.ipynb` files.

**Cross-provider:** Claude Code's `Edit` tool (uses old_string/new_string matching).

---

### 11. `write_to_file` `[Community]`

Creates a new file. Parent directories are created automatically.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `TargetFile` | string | Yes | File path to create |
| `CodeContent` | string | No | Content to write |
| `EmptyFile` | boolean | No | Create an empty file |

**Constraints:** Must NOT be used on existing files. Confirm non-existence first.

**Cross-provider:** Claude Code's `Write` tool (which also handles overwrites).

---

### 12. `run_command` `[Community]`

Executes a shell command. User must approve before execution (unless `SafeToAutoRun` is true).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `CommandLine` | string | Yes | Exact command to execute |
| `Cwd` | string | Yes | Working directory |
| `Blocking` | boolean | Yes | Wait for completion |
| `SafeToAutoRun` | boolean | Yes | Skip user approval for safe commands |
| `WaitMsBeforeAsync` | integer | No | Ms to wait before going async |

**Note:** The Feb 2025 prompt uses `Command` + `ArgsList` (separate args array) instead of a single `CommandLine` string. The April 2025 version consolidated this.

**Constraints:** Never use `cd` -- use `Cwd` parameter instead.

**Cross-provider:** Claude Code's `Bash` tool.

---

### 13. `command_status` `[Community]`

Checks the status of a previously executed async/background command.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `CommandId` | string | Yes | ID of the command to check |
| `OutputCharacterCount` | integer | Yes | Character limit for output |
| `OutputPriority` | enum | Yes | `"top"`, `"bottom"`, or `"split"` |
| `WaitDurationSeconds` | integer | Yes | Wait before returning status |

**Cross-provider:** Claude Code handles this via `Bash` background tasks with automatic notification on completion.

---

### 14. `read_url_content` `[Community]`

Fetches and reads content from a URL.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Url` | string | Yes | HTTP(S) URL to fetch |

**Cross-provider:** Claude Code's `WebFetch` tool.

---

### 15. `view_web_document_content_chunk` `[Community]`

Accesses specific chunks of a previously fetched web document (pagination for large pages).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL previously read by `read_url_content` |
| `position` | integer | Yes | Chunk position index |

**Cross-provider:** Claude Code's `WebFetch` handles this implicitly via content summarization.

---

### 16. `search_web` `[Community]`

Performs a web search and returns relevant document listings.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query |
| `domain` | string | No | Domain to prioritize |

**Cross-provider:** Claude Code's `WebSearch` tool.

---

### 17. `create_memory` `[Community]`

Manages persistent memories -- context that survives across sessions.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Action` | enum | Yes | `"create"`, `"update"`, or `"delete"` |
| `Title` | string | Yes | Descriptive title |
| `UserTriggered` | boolean | Yes | Whether user explicitly requested this |
| `Content` | string | No | Memory content (blank for delete) |
| `CorpusNames` | string[] | No | Workspace corpus names (create only) |
| `Id` | string | No | Existing memory ID (update/delete) |
| `Tags` | string[] | No | Tags in snake_case (create only) |

**Cross-provider:** No direct Claude Code equivalent. Claude Code uses `CLAUDE.md` files and the memory system in `~/.claude/`.

---

### 18. `suggested_responses` `[Community]`

Presents multiple-choice options when asking the user a question.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Suggestions` | string[] | Yes | List of options (max 3, short phrases) |

**Cross-provider:** No direct Claude Code equivalent.

---

### 19. `browser_preview` `[Community]`

Opens a browser preview pane for a running web server inside the IDE.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Name` | string | Yes | Short title (3-5 words, title-cased) |
| `Url` | string | Yes | URL including scheme, domain, port |

**Cross-provider:** No direct Claude Code equivalent (Claude Code has no IDE integration).

---

### 20. `deploy_web_app` `[Community]`

Deploys JavaScript web applications to Netlify.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Framework` | enum | Yes | One of: nextjs, react, svelte, astro, etc. |
| `ProjectPath` | string | Yes | Absolute project path |
| `Subdomain` | string | Yes | Subdomain for new sites |
| `ProjectId` | string | Yes | Project ID for redeployments |

**Constraints:** Must run `read_deployment_config` first.

**Cross-provider:** No direct Claude Code equivalent.

---

### 21. `read_deployment_config` `[Community]`

Verifies web app deployment readiness and configuration.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `ProjectPath` | string | Yes | Absolute project path |

**Cross-provider:** No direct Claude Code equivalent.

---

### 22. `check_deploy_status` `[Community]`

Polls the status of a deployment initiated by `deploy_web_app`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `WindsurfDeploymentId` | string | Yes | Deployment ID (not project ID) |

**Cross-provider:** No direct Claude Code equivalent.

---

### 23. `parallel` `[Community]`

Meta-tool for executing multiple compatible tools simultaneously.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tool_uses` | array | Yes | Array of tool calls with names and parameters |

**Cross-provider:** Claude Code supports parallel tool calls natively (multiple tool calls in one response).

---

## Subagent: SWE-grep (Fast Context) `[Official]`

Windsurf uses a specialized subagent called **Fast Context** powered by `SWE-grep` and `SWE-grep-mini` models. These are custom models trained for rapid code retrieval. `[Official]`

- Executes up to **8 parallel tool calls per turn** over a **maximum of 4 turns**
- Uses a restricted, cross-platform tool set: `grep`, `read`, `glob`
- Throughput: >2,800 tokens/second
- Up to 20x faster than standard agent-based context retrieval

Source: [Windsurf Cascade docs](https://docs.windsurf.com/windsurf/cascade/cascade)

---

## Tool Behavior Rules `[Community]`

Key behavioral constraints from the system prompt:

1. **Explain before calling** -- Cascade must explain why it is calling a tool before invoking it
2. **Never disclose tool names** -- Tool names and descriptions must not be shared with the user
3. **Never call unlisted tools** -- Only explicitly provided tools may be used
4. **Tool call limit** -- Up to 20 tool calls per prompt `[Official]`
5. **Auto-fix linting** -- After `edit_file` or `write_to_file`, Cascade may automatically run linting and apply fixes `[Official]`
6. **Safe auto-run** -- Only commands marked `SafeToAutoRun: true` execute without user approval

---

## Version Differences

The tool set evolved between the Feb 2025 and April 2025 system prompts:

| Change | Feb 2025 | April 2025 |
|--------|----------|------------|
| File viewing | `view_file` (StartLine/EndLine) | `view_line_range` (same params) |
| Code item viewing | `AbsolutePath` + `NodeName` | `File` + `NodePath` |
| Command execution | `Command` + `ArgsList` (split) | `CommandLine` (single string) |
| Related files | `related_files` present | Not confirmed |
| File outline | Not present | `view_file_outline` added |
| Search in file | Not present | `search_in_file` added |
| Memory | Not present | `create_memory` added |
| Deployment tools | Not present | `deploy_web_app` + helpers added |
| Browser preview | `browser_preview` present | Still present |
| Suggested responses | Not present | `suggested_responses` added |
