# Kiro: Hooks

> Research date: 2026-03-21
> Sources tagged: `[Official]` = kiro.dev docs, `[Syllago]` = existing codebase mappings, `[Inferred]` = derived

## Overview

Kiro hooks are event-driven automations that execute agent prompts or shell commands when specific events occur. Available in both IDE (GUI-based) and CLI (JSON config in agent files).

## Hook Events (10 total)

### IDE Hook Events

[Official: Hook Types](https://kiro.dev/docs/hooks/types/)

| Event | Description | Blocking? | Extra Fields |
|-------|-------------|-----------|--------------|
| **Prompt Submit** | User submits a prompt | Yes (on error) | `USER_PROMPT` env var |
| **Agent Stop** | Agent finishes responding | No | — |
| **Pre Tool Use** | Agent about to invoke a tool | Yes (on error) | Tool name field |
| **Post Tool Use** | Agent finished invoking a tool | No | Tool name field |
| **File Create** | New file created matching pattern | No | File pattern |
| **File Save** | File saved matching pattern | No | File pattern |
| **File Delete** | File deleted matching pattern | No | File pattern |
| **Pre Task Execution** | Spec task status changes to `in_progress` | No | — |
| **Post Task Execution** | Spec task status changes to `completed` | No | — |
| **Manual Trigger** | On-demand execution | No | — |

### CLI Hook Events

[Official: CLI Hooks](https://kiro.dev/docs/cli/hooks/)

| Event Name | Kiro CLI Name | Description |
|------------|---------------|-------------|
| AgentSpawn | `agentSpawn` | Agent activates; no tool context |
| UserPromptSubmit | `userPromptSubmit` | User submits prompt; output added to conversation |
| PreToolUse | `preToolUse` | Before tool execution; can block |
| PostToolUse | `postToolUse` | After tool execution with full result access |
| Stop | `stop` | Agent finishes responding (end of turn) |

### Syllago Canonical Event Mapping [Syllago]

| Syllago Canonical | Kiro Name |
|-------------------|-----------|
| `PreToolUse` | `preToolUse` |
| `PostToolUse` | `postToolUse` |
| `UserPromptSubmit` | `userPromptSubmit` |
| `Stop` | `stop` |
| `SessionStart` | `agentSpawn` |

Note: File events (`FileCreate`, `FileSave`, `FileDelete`), task events (`PreTaskExecution`, `PostTaskExecution`), and `ManualTrigger` are IDE-only with no syllago canonical mapping.

## Hook Actions (2 types)

[Official: Hook Actions](https://kiro.dev/docs/hooks/actions/)

### Agent Prompt Action
- Natural language prompt sent to the agent
- For `PromptSubmit` triggers: prompt is appended to user prompt ("Add to prompt")
- Consumes credits (triggers new agent loop)

### Shell Command Action
- Executes a shell command
- **Exit code 0**: stdout added to agent context
- **Exit code 2** (PreToolUse only): blocks tool execution; stderr returned to LLM
- **Other exit codes**: stderr shown as warning; blocks action for PreToolUse and PromptSubmit
- Does NOT consume credits
- Default timeout: 60 seconds (IDE) / 30 seconds (CLI); configurable

## CLI Hook Configuration Format

[Official: CLI Hooks](https://kiro.dev/docs/cli/hooks/)

Hooks are defined within agent JSON configuration files:

```json
{
  "name": "my-agent",
  "hooks": {
    "preToolUse": [
      {
        "command": "./scripts/validate-tool.sh",
        "matcher": "fs_write",
        "timeout_ms": 5000
      }
    ],
    "stop": [
      {
        "command": "npm run lint"
      }
    ],
    "userPromptSubmit": [
      {
        "command": "echo 'Additional context here'"
      }
    ]
  }
}
```

### Hook Entry Fields (CLI)

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Shell command to execute |
| `matcher` | string | Tool name filter (PreToolUse/PostToolUse only) |
| `timeout_ms` | int | Timeout in milliseconds (default: 30000) |
| `cache_ttl_seconds` | int | Cache successful results (0 = disabled) |

### STDIN Input (CLI)

All hooks receive JSON via STDIN with at minimum:
```json
{
  "hook_event_name": "preToolUse",
  "cwd": "/path/to/project"
}
```

Tool-related hooks add:
```json
{
  "tool_name": "fs_write",
  "tool_input": { "path": "src/main.ts", "content": "..." },
  "tool_response": "..."  // PostToolUse only
}
```

## IDE Hook Configuration

IDE hooks are created through the Kiro UI (Command Palette or Hooks panel). They are stored in `.kiro/hooks/` directory.

### IDE Hook Fields

| Field | Description |
|-------|-------------|
| Title | Short name |
| Description | What the hook does |
| Event | Trigger type (dropdown) |
| Tool name | Tool filter (Pre/Post Tool Use only) |
| File pattern | Glob pattern (File Create/Save/Delete only) |
| Action | "Ask Kiro" (agent prompt) or "Run Command" (shell) |
| Instructions/Command | The prompt text or shell command |

## Tool Matchers

[Official: Hook Types](https://kiro.dev/docs/hooks/types/)

Used in Pre Tool Use and Post Tool Use hooks:

| Pattern | Matches |
|---------|---------|
| `read` or `fs_read` | File read tool |
| `write` or `fs_write` | File write tool |
| `shell` or `execute_bash` | Shell execution |
| `aws` or `use_aws` | AWS CLI tool |
| `*` | All tools |
| `@builtin` | All built-in tools |
| `@mcp` | All MCP tools |
| `@powers` | All Powers tools |
| `@git` | All tools from git MCP server |
| `@git/status` | Specific MCP tool |
| `@mcp.*sql.*` | Regex pattern matching |

## Syllago Hook Rendering [Syllago]

Syllago renders hooks for Kiro as agent JSON files (`syllago-hooks.json`):

```json
{
  "name": "syllago-hooks",
  "description": "Hooks installed by syllago",
  "prompt": "",
  "hooks": {
    "preToolUse": [
      {
        "command": "./my-script.sh",
        "matcher": "fs_write",
        "timeout_ms": 5000
      }
    ]
  }
}
```

LLM-evaluated hooks (type `prompt` or `agent`) are either dropped with warning or converted to wrapper scripts via `--llm-hooks=generate`.

## Hook Examples

[Official: Hook Examples](https://kiro.dev/docs/hooks/examples/)

| Example | Event | Action |
|---------|-------|--------|
| Security scanner | Agent Stop | Agent Prompt: review for leaked credentials |
| Prompt logging | Prompt Submit | Shell: curl to log endpoint |
| i18n helper | File Save (`src/locales/en/*.json`) | Agent Prompt: flag missing translations |
| Test coverage | File Save (`src/**/*.{js,ts,jsx,tsx}`) | Agent Prompt: generate missing tests |
| Doc generator | Manual Trigger | Agent Prompt: generate docs |
| Design validation | File Save (`*.css`, `*.html`) | Agent Prompt + Figma MCP |

## Key Differences: IDE vs CLI Hooks

| Aspect | IDE | CLI |
|--------|-----|-----|
| Storage | `.kiro/hooks/` (UI-managed) | Agent JSON files |
| File events | Yes (Create/Save/Delete) | No |
| Task events | Yes (Pre/Post Task) | No |
| Manual trigger | Yes | No |
| STDIN input | Not documented | JSON with event context |
| Exit code blocking | Via shell action | Exit code 2 blocks |
| Caching | Not documented | `cache_ttl_seconds` |
