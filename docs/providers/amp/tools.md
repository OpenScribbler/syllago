<!-- provider-audit-meta
provider: amp
provider_version: "current (rolling release)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/manual
-->

# Amp — Built-in Tools

Amp is a CLI-based AI coding agent built by **Sourcegraph**. It uses Claude and other models (GPT-5 variants, Gemini) in a multi-model routing architecture.

Run `amp tools list` for the complete tool list. Run `amp permissions list --builtin` for built-in permission rules.

## Core Tools

| Tool | Purpose | Cross-provider equivalent |
|------|---------|--------------------------|
| Bash | Execute terminal commands | Claude Code: Bash |
| Read | Read file contents | Claude Code: Read |
| Edit File | Modify files with old_str/new_str replacements | Claude Code: Edit |
| Grep | Search file contents with regex | Claude Code: Grep |
| Task | Spawn subagents for parallel work | Claude Code: Agent |
| Oracle | Second-opinion model (GPT-5.4, high reasoning) | No equivalent |
| Painter | Image generation/editing (Gemini 3 Pro) | No equivalent |
| Librarian | Search external GitHub/Bitbucket repositories | No equivalent |

[Official] https://ampcode.com/manual

## Agent Modes

| Mode | Purpose | Model |
|------|---------|-------|
| Smart | Standard agentic development, unconstrained | Claude Opus 4.6 |
| Deep | Extended reasoning for complex problems | GPT-5.3 Codex |
| Rush | Faster, cheaper for well-defined tasks | Lighter-weight models |

[Official] https://ampcode.com/manual

## Tool Configuration

### Disabling Tools

```json
"amp.tools.disable": ["Painter", "mcp__*"]
```

### Tool Timeout

```json
"amp.tools.stopTimeout": 300
```

Default 300 seconds before canceling a tool execution. [Official]

## Permissions System

Tool usage controlled through permission rules in `settings.json`:

```json
"amp.permissions": [
  {
    "tool": "Bash",
    "matches": { "cmd": "*git commit*" },
    "action": "ask"
  },
  {
    "tool": "mcp__playwright_*",
    "action": "allow"
  }
]
```

**Actions:** `allow`, `reject`, `ask`, `delegate`

**Delegate** runs an external program — receives `AGENT_TOOL_NAME` env var and tool arguments on stdin. Exit codes: 0=allow, 1=ask, 2=reject.

**CLI commands:**
- `amp permissions edit` — edit rules interactively
- `amp permissions test Bash --cmd 'git diff*'` — evaluate without executing
- `amp permissions list` — display configured rules

[Official] https://ampcode.com/manual

## Toolbox System

External tools via `AMP_TOOLBOX` environment variable. On start, Amp invokes executables in the toolbox directory with `TOOLBOX_ACTION=describe`. Tools output key-value pairs:

```
name: run-tests
description: use this tool instead of Bash
dir: string the workspace directory
```

When invoked: `TOOLBOX_ACTION=execute` with parameters on stdin.

[Official] https://ampcode.com/manual

## Unique Capabilities

- Multi-model routing — uses different models for different tasks [Official]
- Parallel subagent execution via Task tool [Official]
- Sourcegraph Librarian for cross-repo code search [Official]
- Thread-based conversation with cross-thread references [Official]
- Custom code review checks (`.agents/checks/` with YAML frontmatter) [Official]
- Oracle for second-opinion reasoning at high effort levels [Official]
- IDE integrations: VS Code, JetBrains, Neovim, Zed [Official]
