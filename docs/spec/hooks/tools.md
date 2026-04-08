# Tool Vocabulary

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines the canonical tool names that abstract over provider-specific
> naming, referenced by the core spec's matcher system.

**Registry Version:** 2026.04
**Last Modified:** 2026-04-08
**Status:** Initial Development

The tool vocabulary defines canonical tool names that abstract over provider-specific naming. Bare string matchers (Section 6.1) resolve against this vocabulary.

## §1 Canonical Tool Names

| Canonical Name | Description | claude-code | gemini-cli | cursor | windsurf | copilot-cli | kiro | opencode |
|----------------|-------------|-------------|------------|--------|----------|-------------|------|----------|
| `shell` | Shell command execution | Bash | run_shell_command | run_terminal_cmd | (event: pre_run_command) | bash | execute_bash | bash |
| `file_read` | Read file contents | Read | read_file | read_file | (event: pre_read_code) | view | fs_read | read |
| `file_write` | Create or overwrite file | Write | write_file | edit_file | (event: pre_write_code) | create | fs_write | write |
| `file_edit` | Modify existing file | Edit | replace | edit_file | (event: pre_write_code) | edit | fs_write | edit |
| `search` | Search file contents | Grep | grep_search | grep_search | -- | grep | grep | grep |
| `find` | Find files by pattern | Glob | glob | file_search | -- | glob | glob | glob |
| `web_search` | Search the web | WebSearch | google_web_search | web_search | -- | -- | web_search | -- |
| `web_fetch` | Fetch URL content | WebFetch | web_fetch | -- | -- | web_fetch | web_fetch | -- |
| `agent` | Spawn sub-agent | Agent | -- | -- | -- | task | use_subagent | -- |

A `--` indicates the provider does not have an equivalent tool.

For split-event providers (Cursor, Windsurf), certain tool vocabulary entries map to native events rather than tool name matchers. For example, encoding `matcher: "shell"` for Windsurf produces a hook bound to the `pre_run_command` event rather than a matcher on a tool name.

## §2 MCP Tool Names

MCP tools use structured objects in the canonical format. The provider-specific combined string formats and encoding rules are defined in Section 6.3. When decoding from a provider, adapters MUST parse the combined string format back into the structured `{"mcp": {"server": "...", "tool": "..."}}` representation.

| Provider | Combined Format | Example |
|----------|----------------|---------|
| claude-code, kiro | `mcp__<server>__<tool>` | `mcp__github__create_issue` |
| gemini-cli | `mcp_<server>_<tool>` | `mcp_github_create_issue` |
| copilot-cli | `<server>/<tool>` | `github/create_issue` |
| cursor, windsurf | `<server>__<tool>` | `github__create_issue` |
