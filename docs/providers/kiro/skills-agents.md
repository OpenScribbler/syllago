# Kiro: Skills & Agents (Subagents)

> Research date: 2026-03-21
> Sources tagged: `[Official]` = kiro.dev docs, `[Syllago]` = existing codebase mappings, `[Inferred]` = derived

## Custom Agents (Subagents)

[Official: Subagents](https://kiro.dev/docs/chat/subagents/) | [Official: Agent Config Reference](https://kiro.dev/docs/cli/custom-agents/configuration-reference/)

### Overview

Custom agents are specialized subagents with their own system prompts, tool permissions, model selection, and MCP access. They run with isolated context windows and can be invoked automatically or manually.

### File Locations

```
.kiro/agents/              # Workspace scope (project-specific)
~/.kiro/agents/            # Global scope (user-wide)
```

Local agents take precedence over global agents with the same name (warning shown).

### File Format

Markdown files with YAML frontmatter. The body contains the system prompt.

```yaml
---
name: code-reviewer
description: Expert code review assistant specializing in TypeScript
tools: ["read", "glob", "grep", "@context7"]
model: claude-sonnet-4
allowedTools: ["read", "glob"]
includeMcpJson: true
includePowers: false
keyboardShortcut: "ctrl+r"
welcomeMessage: "Code reviewer ready. Share the files you'd like reviewed."
---

You are an expert code reviewer. Focus on:
- Type safety and correct TypeScript usage
- Performance implications
- Security vulnerabilities
- Adherence to project conventions defined in steering files
```

### Configuration Fields

[Official: Agent Config Reference](https://kiro.dev/docs/cli/custom-agents/configuration-reference/)

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `name` | string | Agent identifier | Filename |
| `description` | string | Purpose (used for auto-selection) | None |
| `prompt` | string | System prompt; supports `file://` URIs | Markdown body |
| `model` | string | LLM model ID (e.g., `claude-sonnet-4`) | Current chat model |
| `tools` | string[] | Available tools | None |
| `allowedTools` | string[] | Auto-approved tools (no user prompt) | None |
| `toolAliases` | object | Remap tool names | None |
| `toolsSettings` | object | Per-tool config (paths, commands) | None |
| `mcpServers` | object | Inline MCP server definitions | None |
| `resources` | string[] | Context resources (`file://`, `skill://`) | None |
| `includeMcpJson` | boolean | Include workspace/global MCP configs | false |
| `includePowers` | boolean | Include Powers MCP tools | false |
| `keyboardShortcut` | string | Quick-switch trigger (e.g., `"ctrl+a"`) | None |
| `welcomeMessage` | string | Displayed when switching to agent | None |
| `hooks` | object | Lifecycle hook commands | None |

### Tool Access Syntax

```json
{
  "tools": [
    "read",                    // Single built-in tool
    "write",
    "shell",
    "@builtin",                // All built-in tools
    "*",                       // All tools (built-in + MCP)
    "@git",                    // All tools from git MCP server
    "@git/status",             // Specific MCP tool
    "@figma/*"                 // Wildcard MCP tool pattern
  ]
}
```

### Tools Settings Example

```json
{
  "toolsSettings": {
    "write": {
      "allowedPaths": ["src/", "tests/"]
    },
    "shell": {
      "allowedCommands": ["npm test", "npm run lint"],
      "denyByDefault": true
    }
  }
}
```

### Inline MCP Servers

```json
{
  "mcpServers": {
    "my-server": {
      "command": "npx",
      "args": ["-y", "@my-org/mcp-server"],
      "env": {
        "API_KEY": "${MY_API_KEY}"
      },
      "timeout": 120000
    }
  }
}
```

### Resources

```json
{
  "resources": [
    "file://README.md",
    "file://docs/architecture.md",
    "skill://my-skill"
  ]
}
```

`file://` resources are loaded directly into context. `skill://` resources use progressive loading with metadata frontmatter.

Knowledge base resources:
```json
{
  "resources": [
    {
      "type": "knowledge",
      "source": "./docs",
      "name": "project-docs",
      "description": "Project documentation",
      "indexType": "semantic",
      "autoUpdate": true
    }
  ]
}
```

### Agent Hooks

Hooks can be defined inline within agent configuration:

```json
{
  "hooks": {
    "agentSpawn": [{ "command": "./setup.sh" }],
    "userPromptSubmit": [{ "command": "./validate-prompt.sh" }],
    "preToolUse": [{ "command": "./check-tool.sh", "matcher": "fs_write" }],
    "postToolUse": [{ "command": "./log-tool.sh" }],
    "stop": [{ "command": "./cleanup.sh" }]
  }
}
```

### Invocation Methods

| Method | Description |
|--------|-------------|
| Automatic | Kiro selects based on `description` field matching |
| Chat mention | "Use the code-reviewer subagent to..." |
| Slash command | `/code-reviewer find issues...` |
| Keyboard shortcut | If `keyboardShortcut` is set |

### Built-in Subagents

Kiro includes two default subagents [Official](https://kiro.dev/docs/chat/subagents/):

1. **Context gathering subagent** — explores projects and collects relevant context
2. **General purpose subagent** — parallelizes other tasks

### Limitations

- Subagents cannot access Specs
- Subagents do not trigger Hooks
- Steering files and MCP servers work identically to main agent
- Up to 4 subagents can run simultaneously

## Agent Skills

[Official](https://kiro.dev/changelog/ide/0-9/)

### Overview

Skills follow the open [Agent Skills specification](https://agentskills.io). They bundle instructions, scripts, and templates that Kiro activates on-demand when relevant to a task.

### Usage

- Import skills from community repositories
- Create custom skills for project-specific workflows
- Share skills across projects
- Skills are loaded via `skill://` resource references in agent configs

### Syllago Mapping [Syllago]

Syllago maps `catalog.Skills` to Kiro steering files (`.kiro/steering/`) with `auto` inclusion mode, since Kiro's native skill support uses the same progressive loading mechanism.

## Model Selection

[Official](https://kiro.dev/docs/cli/custom-agents/configuration-reference/)

Available models (set via `model` field):
- `claude-sonnet-4` — reliable advanced coding and reasoning [Official](https://kiro.dev/)
- `auto` — mix of frontier models for intent detection and caching [Official](https://kiro.dev/)

The `model` field falls back to the default if the specified model is unavailable.

## Syllago Content Type Mappings [Syllago]

| Syllago Type | Kiro Location | Format | Install Strategy |
|-------------|---------------|--------|-----------------|
| `catalog.Agents` | `.kiro/agents/` | JSON | Symlink |
| `catalog.Rules` | `.kiro/steering/` | Markdown | Symlink (project scope) |
| `catalog.Skills` | `.kiro/steering/` | Markdown | Symlink (project scope) |
| `catalog.Hooks` | `.kiro/agents/` | JSON | JSON merge |
| `catalog.MCP` | `.kiro/settings/mcp.json` | JSON | JSON merge |
