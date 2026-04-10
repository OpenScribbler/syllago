# Factory Droid Tools

Factory Droid (Factory AI's coding agent CLI, `factory-droid`) provides built-in
tools for file operations, search, web access, and subagent delegation. Tool
names use PascalCase and diverge from Claude Code in several cases.

**Identity note:** "Factory Droid" refers to Factory AI's coding agent CLI
(`factory-droid`). No known naming conflicts with other tools as of 2026-03-30.

## Built-in Tools

| Tool | Description | Claude Code equivalent |
|---|---|---|
| `Read` | Read file contents | `Read` (same) |
| `Edit` | Modify existing files with targeted edits | `Edit` (same) |
| `Create` | Create new files | `Write` |
| `Execute` | Run shell commands | `Bash` |
| `Glob` | Find files by glob pattern | `Glob` (same) |
| `Grep` | Search file contents with regex | `Grep` (same) |
| `WebSearch` | Search the web | `WebSearch` (same) |
| `FetchUrl` | Fetch and parse URL content | `WebFetch` |
| `Task` | Delegate work to a sub-droid | `Agent` |

[Official: https://docs.factory.ai/reference/hooks-reference] (tool names
referenced in matcher documentation)

## Tool Categories for Custom Droids

When configuring custom droids, tools can be specified individually or by
category shorthand:

| Category | Tools included |
|---|---|
| `read-only` | Read, LS, Grep, Glob |
| `edit` | Create, Edit, ApplyPatch |
| `execute` | Execute |
| `web` | WebSearch, FetchUrl |
| `mcp` | Dynamically populated from configured MCP servers |

[Official: https://docs.factory.ai/cli/configuration/custom-droids]

## Key Divergences from Claude Code

| Area | Factory Droid | Claude Code |
|---|---|---|
| File creation | `Create` | `Write` |
| Shell execution | `Execute` | `Bash` |
| URL fetching | `FetchUrl` | `WebFetch` |
| Subagent delegation | `Task` | `Agent` |
| Patch application | `ApplyPatch` | Not a separate tool [Unverified] |
| Directory listing | `LS` (in read-only category) | `LS` [Unverified] |

The identical tools (`Read`, `Edit`, `Glob`, `Grep`, `WebSearch`) share names
across both providers. MCP tool naming also uses the same
`mcp__<server>__<tool>` format.

## Sources

- [Factory Droid Hooks Reference](https://docs.factory.ai/reference/hooks-reference) [Official]
- [Factory Droid Custom Droids](https://docs.factory.ai/cli/configuration/custom-droids) [Official]
