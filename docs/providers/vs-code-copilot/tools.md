# VS Code Copilot Agent Tools

> Research date: 2026-03-31
> Status: Draft -- compiled from official docs and VS Code documentation.

## Overview

VS Code Copilot agent mode provides built-in tools that the AI can invoke
autonomously during conversations. Unlike Claude Code (which exposes tools by
exact function name), VS Code Copilot uses a mix of tool aliases and internal
tool names. The public-facing tool names are used in custom agent configs and
the `#` tool picker in chat.

VS Code supports three categories of tools: **built-in tools**, **MCP tools**
(from Model Context Protocol servers), and **extension tools** (from VS Code
extensions). This document covers the built-in tools only.

A chat request can have a maximum of **128 tools** enabled at a time. [Official]

**Identity note:** "VS Code Copilot" refers to GitHub Copilot's agent mode
within VS Code (`vs-code-copilot`). Distinct from `copilot-cli` (GitHub Copilot
in the terminal, `gh copilot`).

---

## Built-in Tool Aliases

These are the tool names/aliases used in custom agent `tools` arrays and the
chat tool picker. VS Code does not publicly document exact function signatures
for its built-in tools -- these aliases are the public interface.

### File Operations

| Tool Alias | Description | CC Equivalent |
|---|---|---|
| `read` | Read file contents from the workspace | `Read` |
| `edit` | Edit or create files in the workspace | `Edit` / `Write` |

[Official: https://code.visualstudio.com/docs/copilot/agents/agent-tools]

### Search Tools

| Tool Alias | Description | CC Equivalent |
|---|---|---|
| `search` | General search across the codebase | `Grep` |
| `search/codebase` | Semantic codebase search (meaning-based) | `Grep` (semantic) |
| `search/changes` | Search recent changes (git diff) | `Bash` (`git diff`) |
| `search/usages` | Find usages of symbols | `Grep` (targeted) |

[Official: https://code.visualstudio.com/docs/copilot/agents/agent-tools]

### Diagnostics

| Tool Alias | Description | CC Equivalent |
|---|---|---|
| `read/problems` | Get editor diagnostics (compile errors, lint warnings) | `Bash` (run linter) |

[Official: https://code.visualstudio.com/docs/copilot/agents/agent-tools]

### Terminal

| Tool Alias | Description | CC Equivalent |
|---|---|---|
| `run_in_terminal` | Execute shell/bash commands in integrated terminal | `Bash` |

The terminal tool displays commands inline in chat. VS Code uses bash and
PowerShell tree-sitter grammars to extract sub-commands for display.

Terminal sandboxing restricts filesystem and network access when enabled. With
sandboxing on, terminal commands are auto-approved without user confirmation.

[Official: https://code.visualstudio.com/docs/copilot/chat/chat-agent-mode]

### Web

| Tool Alias | Description | CC Equivalent |
|---|---|---|
| `web/fetch` | Fetch content from a URL | `WebFetch` |

[Official: https://code.visualstudio.com/docs/copilot/agents/agent-tools]

---

## Internal Tool Names

These names appear in hook input (`tool_name` field) and are the actual
identifiers used by the agent internally. The exact mapping between aliases
and internal names is not fully documented.

| Internal Name | Likely Alias | Notes |
|---|---|---|
| `read_file` | `read` | [Unverified] Inferred from hook input examples |
| `edit_file` | `edit` | [Unverified] Inferred from hook input examples |
| `create_file` | `edit` | [Unverified] May be separate or merged with edit |
| `replace_string_in_file` | `edit` | [Unverified] Mentioned in VS Code hooks docs as differing from CC |
| `run_in_terminal` | `run_in_terminal` | [Official] Same name in both contexts |

**Important note from hooks docs:** VS Code tool names differ from Claude Code
tool names. Claude Code uses `Write`/`Edit`; VS Code uses `create_file`/
`replace_string_in_file`. Claude Code uses snake_case input fields
(`tool_input.file_path`); VS Code uses camelCase (`tool_input.filePath`).
[Official: https://code.visualstudio.com/docs/copilot/customization/hooks]

---

## Tool Sets

Tool sets group multiple tools under a single alias for use in custom agent
configs. They are defined in the VS Code settings or custom agent frontmatter.

Example tool set definition:

```json
{
  "reader": {
    "tools": ["search/changes", "search/codebase", "read/problems", "search/usages"]
  }
}
```

When configuring a custom agent, you reference tool sets by name:

```yaml
tools: ["reader", "edit", "run_in_terminal"]
```

If the `tools` field is omitted from a custom agent, it has access to **all**
available tools.

[Official: https://code.visualstudio.com/docs/copilot/customization/custom-agents]

---

## Context Variables (Not Tools)

These are context providers invoked with `#` in chat, distinct from tools:

| Variable | Description |
|---|---|
| `#codebase` | Include relevant codebase context |
| `#file` | Reference a specific file |
| `#terminalSelection` | Include selected terminal output |
| `#fetch` | Fetch a URL's content |

[Official: https://code.visualstudio.com/docs/copilot/chat/chat-agent-mode]

---

## Cross-Provider Tool Mapping

| VS Code Copilot | Claude Code | Cursor | Windsurf |
|---|---|---|---|
| `read` | `Read` | `read_file` | `read_file` |
| `edit` | `Edit` / `Write` | `edit_file` | `edit_file` |
| `run_in_terminal` | `Bash` | `run_terminal_cmd` | `run_command` |
| `search` | `Grep` | `grep_search` / `grep` | `grep_search` |
| `search/codebase` | `Grep` (semantic) | `codebase_search` | `codebase_search` |
| `web/fetch` | `WebFetch` | `web_search` | `web_search` |
| `read/problems` | `Bash` (linter) | `read_lints` | -- |

---

## Tools NOT Present (vs Claude Code)

The following Claude Code tools have **no documented built-in equivalent** in
VS Code Copilot:

| CC Tool | Status in VS Code Copilot |
|---|---|
| `Glob` | No dedicated glob tool; use `search` or terminal [Unverified] |
| `WebSearch` | No dedicated search tool; use `web/fetch` or MCP [Unverified] |
| `Agent` | Subagents exist but invoked differently (custom agents) [Official] |
| `NotebookEdit` | No dedicated notebook tool documented [Unverified] |
| `MultiEdit` | No multi-edit tool documented [Unverified] |
| `LS` | No dedicated directory listing; use terminal [Unverified] |
| `NotebookRead` | No dedicated notebook read tool [Unverified] |
| `KillBash` | No terminal kill tool documented [Unverified] |
| `Skill` | Skills invoked via `/skill-name`, not a tool call [Official] |
| `AskUserQuestion` | No dedicated tool; agent asks via chat [Unverified] |

---

## Security and Approvals

VS Code provides a permissions picker in the Chat view to control agent
autonomy for tool invocation:

- **Ask** -- User must approve each tool use
- **Auto-approve with sandboxing** -- Terminal commands run sandboxed
- **Auto-approve all** -- Full autonomy

The `chat.tools.edits.autoApprove` setting controls whether file edits require
manual approval.

[Official: https://code.visualstudio.com/docs/copilot/chat/chat-agent-mode]

---

## Sources

- [Use tools with agents - VS Code](https://code.visualstudio.com/docs/copilot/agents/agent-tools) [Official]
- [Use agent mode in VS Code](https://code.visualstudio.com/docs/copilot/chat/chat-agent-mode) [Official]
- [Custom agents in VS Code](https://code.visualstudio.com/docs/copilot/customization/custom-agents) [Official]
- [Agent hooks in VS Code](https://code.visualstudio.com/docs/copilot/customization/hooks) [Official]
- [Using agents in VS Code](https://code.visualstudio.com/docs/copilot/agents/overview) [Official]
