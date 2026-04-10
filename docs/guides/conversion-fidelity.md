# Conversion Fidelity Guide

What happens when syllago converts content between providers? This document describes the field mappings, what's preserved, and what's lost.

## How Conversion Works

Syllago uses a **hub-and-spoke model**: content goes from the source provider format to a canonical intermediate format, then from canonical to the target provider format. This means every conversion involves two steps, even for direct provider-to-provider operations.

```
Source (Cursor .mdc) → Canonical (YAML frontmatter + markdown) → Target (Windsurf .md)
```

## Rules Conversion

Rules are the most common conversion target. Each provider has a different way of expressing the same concepts: "always apply this rule" vs "apply only to specific files."

### Canonical Format

```yaml
---
description: What this rule does
alwaysApply: true          # or false
globs:                     # file patterns (when alwaysApply is false)
    - "*.ts"
    - "*.tsx"
---

Rule content here (markdown).
```

### Field Mapping by Provider

| Canonical Field | Claude Code | Cursor | Windsurf | Copilot | Kiro | Cline | Zed |
|----------------|-------------|--------|----------|---------|------|-------|-----|
| `alwaysApply: true` | No frontmatter (plain md) | `alwaysApply: true` | `trigger: always_on` | No frontmatter | `inclusion: always` | No frontmatter | Plain text |
| `alwaysApply: false` + globs | `paths:` array | `globs:` string | `trigger: glob` + `globs:` | `applyTo:` string | `inclusion: fileMatch` + `fileMatchPattern:` | `paths:` array | Dropped (warning) |
| `alwaysApply: false` + description | Scope embedded as prose | `description:` | `trigger: model_decision` | Scope embedded as prose | `inclusion: auto` | No equivalent | Dropped (warning) |
| `description` | Dropped (not used) | Preserved | Preserved | Dropped | Preserved | Dropped | HTML comment |

### What's Preserved

- **Body content**: Always passes through unchanged.
- **File scoping (globs)**: Preserved across all providers that support it. Format differs (array vs string vs `applyTo`) but semantics are identical.
- **Description**: Preserved where the target provider has a description field.

### What's Lost or Changed

- **Zed**: Cannot express conditional activation. Glob-scoped rules become unconditional (with a warning).
- **Claude Code**: `description` is dropped. Scope for non-glob rules is embedded as prose text with a `syllago:converted` marker.
- **Copilot**: `description` is dropped for glob-scoped rules (only `applyTo` is used).
- **Windsurf `manual` trigger**: No direct equivalent in other providers. Maps to non-alwaysApply without globs.

## Hooks Conversion

Hooks convert between provider-specific event names and the canonical event system.

### Event Name Mapping

| Canonical Event | Claude Code | Copilot | Gemini CLI |
|----------------|-------------|---------|------------|
| `before_tool_execute` | `PreToolUse` | `onBeforeToolExecution` | `BeforeTool` |
| `after_tool_execute` | `PostToolUse` | `onAfterToolExecution` | `AfterTool` |
| `session_start` | `Stop` (subagent) | `onSessionStart` | `SessionStart` |
| `session_end` | `Stop` | `onSessionEnd` | `SessionEnd` |

### What's Preserved

- **Command**: The shell command to run.
- **Matcher**: Tool-specific matchers (e.g., `Bash`, `Edit`) are translated to canonical names and back.
- **Timeout**: Preserved where supported.

### What's Lost or Changed

- **LLM-evaluated hooks**: Claude Code supports `type: "llm"` hooks that use AI evaluation. These have no equivalent in other providers and are either dropped (with warning) or wrapped in generated scripts.

## MCP Configs Conversion

MCP server configurations are relatively uniform across providers.

### What's Preserved

- **Server name, command, args, env**: All preserved as-is.
- **Multiple servers**: Each server converts independently.

### What's Changed

- **File location**: Different providers store MCP configs in different files (`settings.json`, `.claude.json`, `mcp-config.json`). Syllago handles the routing.

## Skills, Agents, Commands

These content types are primarily markdown-based and convert with high fidelity:

- **Skills**: SKILL.md files pass through with minimal changes. Provider-specific metadata (if any) is adjusted.
- **Agents**: AGENT.md files similarly pass through. Provider-specific isolation settings are mapped where possible.
- **Commands**: Format varies more widely (TOML for Gemini, markdown for others). The command body is preserved; metadata mappings vary.

## Checking Conversion Fidelity

Use `syllago convert --diff` to see exactly what changes:

```bash
syllago convert ./my-rule.mdc --from cursor --to windsurf --diff
```

Use `syllago compat <item>` to see which providers support a content item and what warnings apply.
