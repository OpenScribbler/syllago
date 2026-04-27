# Kiro: Built-in Tools

> Research date: 2026-03-21
> Sources tagged: `[Official]` = kiro.dev docs, `[Syllago]` = existing codebase mappings, `[Inferred]` = derived from patterns

## Overview

Kiro has two interfaces (IDE and CLI) with overlapping but not identical tool sets. The CLI documents tools more explicitly; the IDE exposes them through the agent system. Tools are referenced by name in agent configs, hook matchers, and `allowedTools` arrays.

## Tool Reference Syntax

Tools can be referenced in several ways [Official](https://kiro.dev/docs/cli/custom-agents/configuration-reference/):

| Syntax | Meaning |
|--------|---------|
| `"read"` | Single built-in tool by name |
| `"@builtin"` | All built-in tools |
| `"*"` | All tools (built-in + MCP) |
| `"@server_name"` | All tools from an MCP server |
| `"@server_name/tool_name"` | Specific MCP server tool |
| `"@powers"` | All Powers MCP tools |
| `"@mcp"` | All MCP tools (hook matchers) |

Wildcard patterns are supported in `allowedTools`: `"@server/read_*"`, `"@*-mcp/status"`, `"?ead"` [Official](https://kiro.dev/docs/cli/custom-agents/configuration-reference/).

## Core File Tools

| Tool Name | Aliases | Description | Key Parameters |
|-----------|---------|-------------|----------------|
| `read` | `fs_read` | Reads files, folders, and images | `allowedPaths`, `deniedPaths` |
| `write` | `fs_write` | Creates and edits files | `allowedPaths`, `deniedPaths`, custom diff tool |
| `glob` | — | Fast file discovery using glob patterns, respects `.gitignore` | `allowedPaths`, `deniedPaths`, `allowReadOnly` |
| `grep` | — | Fast content search using regex, respects `.gitignore` | `allowedPaths`, `deniedPaths` |

[Official: CLI Built-in Tools](https://kiro.dev/docs/cli/reference/built-in-tools/)

### Syllago Canonical Mappings [Syllago]

| Syllago Canonical | Kiro Name |
|-------------------|-----------|
| `Read` | `read` |
| `Write` | `fs_write` |
| `Edit` | `fs_write` |
| `Glob` | `read` |
| `Grep` | `read` |
| `Bash` | `shell` |

Note: Syllago maps Glob and Grep to `read` for Kiro. The CLI docs show `glob` and `grep` as separate tools. This may reflect CLI vs IDE differences, or a syllago simplification.

## Shell Tool

| Tool Name | Aliases | Description |
|-----------|---------|-------------|
| `shell` | `execute_bash` | Executes bash commands |

Configuration options [Official](https://kiro.dev/docs/cli/reference/built-in-tools/):
- `allowedCommands` — whitelist of allowed commands
- `deniedCommands` — blacklist of denied commands
- `autoAllowReadonly` — auto-approve read-only commands
- `denyByDefault` — block all commands unless explicitly allowed

## Web Tools

| Tool Name | Description |
|-----------|-------------|
| `web_search` | Real-time internet search |
| `web_fetch` | Fetch content from URLs (selective, truncated, or full modes) |

Web tools support `trustedPatterns` and `blockedPatterns` for URL filtering. Enterprise admins can disable web tools via IAM Identity Center [Official](https://kiro.dev/changelog/ide/0-8/).

## AWS Tool

| Tool Name | Aliases | Description |
|-----------|---------|-------------|
| `aws` | `use_aws` | Makes AWS CLI calls with service, operation, and parameters |

Supports `allowedServices` and `allowedOperations` filtering [Official](https://kiro.dev/docs/cli/reference/built-in-tools/).

## Code Intelligence

| Tool Name | Description |
|-----------|-------------|
| `code` | Symbol search, LSP integration, pattern-based code search |

[Official](https://kiro.dev/docs/cli/reference/built-in-tools/)

## Agent & Task Tools

| Tool Name | Aliases | Description |
|-----------|---------|-------------|
| `delegate` | — | Delegates tasks to background agents (async) |
| `use_subagent` | — | Spawns up to 4 specialized subagents simultaneously with isolated context |

[Official](https://kiro.dev/docs/cli/reference/built-in-tools/)

## Utility Tools

| Tool Name | Description | Status |
|-----------|-------------|--------|
| `introspect` | Self-awareness — answers questions about Kiro's features and commands | Stable |
| `session` | Temporarily override CLI settings for current session | Stable |
| `report` | Opens browser for pre-filled GitHub issue templates | Stable |

[Official](https://kiro.dev/docs/cli/reference/built-in-tools/)

## Experimental Tools

| Tool Name | Description |
|-----------|-------------|
| `knowledge` | Store and retrieve information in a knowledge base across sessions |
| `thinking` | Internal reasoning mechanism for improved task quality |
| `todo` | Task tracking for multi-step workflows |

[Official](https://kiro.dev/docs/cli/reference/built-in-tools/)

## Spec Tool

| Tool Name | Description |
|-----------|-------------|
| `spec` | Interacts with Kiro's spec-driven development system |

Referenced in tool categories (`read`, `write`, `shell`, `web`, `spec`) [Official](https://kiro.dev/docs/chat/subagents/).

## Hook Matcher Tool Names

When configuring hook matchers, both canonical and alias names work [Official](https://kiro.dev/docs/cli/hooks/):

| Canonical | Alias |
|-----------|-------|
| `fs_read` | `read` |
| `fs_write` | `write` |
| `execute_bash` | `shell` |
| `use_aws` | `aws` |

Special matchers:
- `*` — match all tools
- `@builtin` — match all built-in tools
- `@mcp` — match all MCP tools
- `@powers` — match all Powers tools
- `@mcp.*sql.*` — regex pattern matching [Official](https://kiro.dev/docs/hooks/types/)

## MCP Tool Name Format

Kiro uses the `mcp__server__tool` prefix format for MCP tools, same as Claude Code [Syllago](https://kiro.dev/docs/cli/custom-agents/configuration-reference/).

## Complete Tool Summary (18 tools)

**Stable:** `read`, `write`, `glob`, `grep`, `shell`, `aws`, `web_search`, `web_fetch`, `code`, `delegate`, `use_subagent`, `introspect`, `session`, `report`, `spec`
**Experimental:** `knowledge`, `thinking`, `todo`
