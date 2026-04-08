# Tool Vocabulary

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines the canonical tool names that abstract over provider-specific
> naming, referenced by the core spec's matcher system.

**Registry Version:** 2026.04
**Last Modified:** 2026-04-08
**Status:** Initial Development

The tool vocabulary defines canonical tool names that abstract over provider-specific naming. Bare string matchers (Section 6.1) resolve against this vocabulary.

## §1 Canonical Tool Names

| Canonical Name | Description | claude-code | gemini-cli | cursor | windsurf | copilot-cli | kiro | opencode | factory-droid | codex |
|----------------|-------------|-------------|------------|--------|----------|-------------|------|----------|---------------|-------|
| `shell` | Shell command execution | Bash | run_shell_command | run_terminal_cmd | (event: pre_run_command) | bash | execute_bash | bash | Bash | Bash |
| `file_read` | Read file contents | Read | read_file | read_file | (event: pre_read_code) | view | fs_read | read | Read | -- |
| `file_write` | Create or overwrite file | Write | write_file | edit_file | (event: pre_write_code) | create | fs_write | write | Write | -- |
| `file_edit` | Modify existing file | Edit | replace | edit_file | (event: pre_write_code) | edit | fs_write | edit | Edit | -- |
| `search` | Search file contents | Grep | grep_search | grep_search | -- | grep | grep | grep | Grep | -- |
| `find` | Find files by pattern | Glob | glob | file_search | -- | glob | glob | glob | Glob | -- |
| `web_search` | Search the web | WebSearch | google_web_search | web_search | -- | -- | web_search | -- | WebSearch | -- |
| `web_fetch` | Fetch URL content | WebFetch | web_fetch | -- | -- | web_fetch | web_fetch | -- | WebFetch | -- |
| `agent` | Spawn sub-agent | Agent | -- | -- | -- | task | use_subagent | -- | Agent | -- |

A `--` indicates the provider does not have an equivalent tool or the tool vocabulary is not enumerated in hook documentation.

For split-event providers (Cursor, Windsurf), certain tool vocabulary entries map to native events rather than tool name matchers. For example, encoding `matcher: "shell"` for Windsurf produces a hook bound to the `pre_run_command` event rather than a matcher on a tool name.

**Matcher field support note:** VS Code Copilot and Copilot CLI do not honor the `matcher` field on hook entries. VS Code Copilot accepts but ignores matcher syntax — hooks fire for all tool invocations on the matching event regardless of the tool name. Copilot CLI has no matcher system at all. Hooks targeting tool-specific behavior on these providers must use separate hook definitions per native event, or accept that the hook will fire for all tools. Adapters encoding to these providers MUST emit a warning when a hook has a non-wildcard matcher.

**Cline tool vocabulary:** Cline hooks receive a `tool` field in their stdin payload containing the tool name string. The tool name vocabulary is not publicly enumerated in cline hook documentation. The `matcher` field in canonical format maps to filtering logic in the hook script itself, not a config-level matcher.

## §2 MCP Tool Names

MCP tools use structured objects in the canonical format. The provider-specific combined string formats and encoding rules are defined in Section 6.3. When decoding from a provider, adapters MUST parse the combined string format back into the structured `{"mcp": {"server": "...", "tool": "..."}}` representation.

| Provider | Combined Format | Example |
|----------|----------------|---------|
| claude-code, kiro, factory-droid | `mcp__<server>__<tool>` | `mcp__github__create_issue` |
| gemini-cli | `mcp_<server>_<tool>` | `mcp_github_create_issue` |
| copilot-cli | `<server>/<tool>` | `github/create_issue` |
| cursor, windsurf | `<server>__<tool>` | `github__create_issue` |
| codex | Not applicable — codex uses MCP as a tool provider, not as a hook matcher target | — |
| cline | Not applicable — cline hook scripts receive tool names but have no MCP matcher format | — |
