<!-- provider-audit-meta
provider: amp
provider_version: "current (rolling release)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/manual
-->

# Amp — Hooks

## Status: No Native Hook System

Amp does not have an event-based hooks system comparable to Claude Code, Gemini CLI, or Copilot CLI. There are no pre/post tool-use or pre/post message event hooks. [Official — hooks not documented in manual]

## Toolbox System (Closest Equivalent)

Amp has a **Toolbox** mechanism that registers external executables as tools:

**Setup:** Set `AMP_TOOLBOX` environment variable to a directory containing executables.

**Discovery:** On startup, Amp invokes each executable with `TOOLBOX_ACTION=describe`. The tool outputs key-value pairs:

```
name: run-tests
description: use this tool instead of Bash
dir: string the workspace directory
```

**Execution:** When invoked by the agent, the executable receives `TOOLBOX_ACTION=execute` with parameters on stdin.

**Key difference from hooks:** Toolbox tools are invoked by the agent as part of its reasoning (like MCP tools), not automatically triggered by lifecycle events. The agent decides when to use them based on the tool description. [Official]

## Permission Delegates (Another Partial Equivalent)

The permissions system supports `delegate` action, which runs an external program to decide tool permissions:

```json
{
  "action": "delegate",
  "to": "amp-permission-helper",
  "tool": "Bash"
}
```

The delegate receives `AGENT_TOOL_NAME` env var and tool arguments on stdin. This runs before tool execution but is a permission gate, not a general-purpose hook. [Official]

## Implications for Syllago

- No hook conversion path to/from Amp
- Amp is excluded from the hook interchange format (`docs/spec/hooks-v1.md`)
- The provider definition correctly returns `""` for `InstallDir(catalog.Hooks)` and `false` for `SupportsType(catalog.Hooks)`
- The Toolbox and permission delegate systems are structurally different from hooks and not convertible
