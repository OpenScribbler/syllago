# Crush Agent Tools

> Research date: 2026-03-30
> Status: Draft -- compiled from official repo, DeepWiki, and community sources.

## Overview

Crush is a terminal-based AI coding agent built by Charmbracelet in Go. It
provides a set of built-in tools that the LLM can invoke during sessions. Tools
are assembled by a Coordinator that filters the full tool list based on
`permissions.allowed_tools` and appends MCP-exposed tools, then sorts by name.

There is no documented limit on tool calls per session. [Unverified]

Tool names are lowercase identifiers used in configuration (`allowed_tools`,
`disabled_tools`).

---

## Core Tools

The following tools are always registered. [Official:
https://github.com/charmbracelet/crush]

### 1. bash (BashTool)

Executes shell commands. Uses POSIX-compliant shell emulation via `mvdan.cc/sh`.
Requires user approval by default unless the tool is in `allowed_tools` or
`--yolo` is set.

Cross-provider equivalent: Claude Code `Bash`, Cursor `run_terminal_cmd`

### 2. view (ViewTool)

Reads file contents from the filesystem.

Cross-provider equivalent: Claude Code `Read`, Cursor `read_file`

### 3. edit (EditTool)

Modifies existing files with targeted edits.

Cross-provider equivalent: Claude Code `Edit`, Cursor `edit_file`

### 4. multi_edit (MultiEditTool)

Applies multiple edits to one or more files in a single operation. [Unverified
-- name inferred from source references]

Cross-provider equivalent: No direct equivalent in Claude Code or Cursor.

### 5. write (WriteTool)

Creates new files or overwrites existing files entirely.

Cross-provider equivalent: Claude Code `Write`, Cursor `edit_file` (create mode)

### 6. grep (GrepTool)

Text-based regex search. Configurable timeout via `tools.grep.timeout` in
`crush.json`. [Official: schema.json]

Cross-provider equivalent: Claude Code `Grep`, Cursor `grep_search`

### 7. glob (GlobTool)

Finds files by glob pattern matching.

Cross-provider equivalent: Claude Code `Glob`, Cursor `file_search`

### 8. ls (LsTool)

Lists directory contents. Configurable via `tools.ls.max_depth` (default 0) and
`tools.ls.max_items` (default 1000). [Official: schema.json]

Cross-provider equivalent: Claude Code `Bash` (`ls`), Cursor `list_dir`

### 9. fetch (FetchTool)

Fetches content from URLs.

Cross-provider equivalent: Claude Code `WebFetch`

### 10. download (DownloadTool)

Downloads files from URLs to the local filesystem.

Cross-provider equivalent: Claude Code `Bash` (`curl`/`wget`)

### 11. sourcegraph (SourcegraphTool)

Code search via Sourcegraph. Can be disabled via `options.disabled_tools`.

Cross-provider equivalent: Claude Code `Grep` (semantic mode), Cursor
`codebase_search`

### 12. todos (TodosTool)

Task management for tracking multi-step work.

Cross-provider equivalent: Claude Code `TodoWrite`, Cursor `todo_write`

---

## Agent Tools

### 13. agent (AgentTool)

Delegates tasks to a sub-agent. Crush supports two model roles: a "large" model
for complex coding tasks and a "small" model for simpler operations.

Cross-provider equivalent: Claude Code subagent spawning, Cursor subagents

---

## Job Management Tools

### 14. job_output (JobOutputTool)

Retrieves output from background jobs.

### 15. job_kill (JobKillTool)

Terminates running background jobs.

Cross-provider equivalent: Claude Code `Bash` (background process management)

---

## LSP Tools (Conditional)

Registered when LSP servers are configured or `options.auto_lsp` is not
explicitly disabled. [Official: https://github.com/charmbracelet/crush]

### 16. diagnostics (DiagnosticsTool)

Retrieves linter/diagnostic errors from configured language servers.

Cross-provider equivalent: Cursor `read_lints`

### 17. references (ReferencesTool)

Finds symbol references via LSP.

Cross-provider equivalent: No direct equivalent (Cursor/Claude Code use
semantic search)

### 18. lsp_restart (LSPRestartTool)

Restarts a language server.

Cross-provider equivalent: No equivalent in other providers.

---

## MCP Tools (Dynamic)

When MCP servers are configured in `crush.json`, their exposed tools are added
to the agent's tool set. MCP tools can be individually disabled via
`mcp.<server>.disabled_tools`. [Official: schema.json]

---

## Tool Configuration

### Permission Control

Tools require user approval by default. Configure pre-approved tools in
`crush.json`:

```json
{
  "permissions": {
    "allowed_tools": ["view", "ls", "grep", "glob", "edit"]
  }
}
```

Or skip all prompts: `crush --yolo` [Official:
https://github.com/charmbracelet/crush]

### Disabling Tools

Hide tools entirely from the agent:

```json
{
  "options": {
    "disabled_tools": ["bash", "sourcegraph"]
  }
}
```

[Official: https://github.com/charmbracelet/crush]

---

## Cross-Provider Tool Mapping

| Crush Tool | Claude Code | Cursor |
|------------|-------------|--------|
| `bash` | `Bash` | `run_terminal_cmd` |
| `view` | `Read` | `read_file` |
| `edit` | `Edit` | `edit_file` |
| `write` | `Write` | `edit_file` (create) |
| `grep` | `Grep` | `grep_search` |
| `glob` | `Glob` | `file_search` |
| `ls` | `Bash` (`ls`) | `list_dir` |
| `fetch` | `WebFetch` | `web_search` |
| `sourcegraph` | `Grep` (semantic) | `codebase_search` |
| `todos` | `TodoWrite` | `todo_write` |
| `agent` | Subagent | Subagents |
| `diagnostics` | `Bash` (linter) | `read_lints` |

---

## Sources

- [Crush GitHub Repository](https://github.com/charmbracelet/crush) [Official]
- [Crush JSON Schema](https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json) [Official]
- [Crush Architecture -- DeepWiki](https://deepwiki.com/charmbracelet/crush) [Community]
- [Agent System -- DeepWiki](https://deepwiki.com/charmbracelet/crush/4-ai-and-llm-integration) [Community]
