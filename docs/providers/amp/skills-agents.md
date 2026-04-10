<!-- provider-audit-meta
provider: amp
provider_version: "current (rolling release)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/manual
-->

# Amp — Skills & Agents

## Skills

**Status:** Supported (syllago can install/discover skills for Amp)

### SKILL.md Format

Each skill is a directory containing `SKILL.md` with YAML frontmatter:

```yaml
---
name: my-skill
description: A description of what this skill does
---

# My Skill

Instructions for the agent...
```

**Required fields:**
- `name` — unique identifier
- `description` — visible to model, determines when skill invokes

**Optional components:**
- Bundled resources (scripts, templates, etc.) in the same directory
- `mcp.json` — bundled MCP servers that load with the skill

[Official] https://ampcode.com/manual

### Directory Structure and Precedence

Skills are searched in this order (highest priority first):

1. `~/.config/agents/skills/<name>/SKILL.md` — user-wide (primary)
2. `~/.config/amp/skills/<name>/SKILL.md` — user-wide (Amp-specific)
3. `.agents/skills/<name>/SKILL.md` — project-level
4. `.claude/skills/<name>/SKILL.md` — project-level (compatibility)
5. `~/.claude/skills/<name>/SKILL.md` — user-wide (compatibility)
6. Plugins, toolbox directories, built-in skills

**Additional paths:** Configurable via `amp.skills.path` setting in `settings.json`.

**Override behavior:** Project skills override user-wide skills, which override built-in skills, matched by `name`. [Official]

### Bundled MCP Servers

Skills can include an `mcp.json` file to declare MCP servers that load when the skill is active:

```json
{
  "my-server": {
    "command": "node",
    "args": ["server.js"],
    "includeTools": ["tool_*"]
  }
}
```

Same schema as the main MCP configuration. [Official]

### Compatibility with Claude Code

Amp searches `.claude/skills/` and `~/.claude/skills/` as fallback paths, providing compatibility with Claude Code skill directories. The SKILL.md format (name + description frontmatter + markdown body) appears identical to Claude Code's format. [Official]

## Agents

**Status:** Not supported as user-definable content files

### Built-in Agent Types

Amp has several built-in specialized agents that are invoked by the system, not defined by users:

| Agent | Purpose | Model |
|-------|---------|-------|
| Oracle | Second-opinion reasoning, debugging | GPT-5.4 (high reasoning) |
| Librarian | Cross-repo code search | Sourcegraph integration |
| Code Review | Automated review with custom checks | Configurable |
| Course Correct | Parallel correction monitoring | Background |
| Subagents (Task) | Parallel work on multi-step tasks | Same as parent mode |

[Official] https://ampcode.com/manual

### Subagents

Spawned via the Task tool for parallel work. Key constraints:
- Work in isolation — cannot communicate with each other
- Cannot be guided mid-task by the user
- Useful for parallel work across different code areas

[Official]

### Custom Checks (Agent Extension)

The closest equivalent to user-defined agents. Checks live in `.agents/checks/` with YAML frontmatter:

```yaml
---
name: security-review
description: Check for common security vulnerabilities
severity-default: warning
tools:
  - Grep
  - Read
---

Review criteria and instructions...
```

**Frontmatter fields:**
- `name` — check identifier
- `description` — what the check reviews
- `severity-default` — default severity level
- `tools` — tools the check is permitted to use

[Official]

## Model Selection

Amp uses mode-based model routing rather than per-skill model selection:

| Mode | Model | Use Case |
|------|-------|----------|
| Smart | Claude Opus 4.6 | Standard development, unconstrained |
| Deep | GPT-5.3 Codex | Complex problems, extended reasoning |
| Rush | Lighter models | Fast, well-defined tasks |

Deep reasoning effort is configurable: `amp.agent.deepReasoningEffort` = `"medium"` | `"high"` | `"xhigh"` [Official]

## Syllago Provider Definition Accuracy

Based on the official manual, the current syllago provider definition is **accurate**:

| Aspect | Provider Definition | Official Docs | Match? |
|--------|-------------------|---------------|--------|
| Rules path | `AGENTS.md` in project root | `AGENTS.md` (+ parents, fallbacks) | Yes |
| Skills project path | `.agents/skills/` | `.agents/skills/` | Yes |
| Skills compat path | `.claude/skills/` | `.claude/skills/` | Yes |
| Skills global path | `~/.config/agents/skills/` | `~/.config/agents/skills/` | Yes |
| MCP path | `.amp/settings.json` | `amp.mcpServers` in settings | Partial — MCP is in `~/.config/amp/settings.json`, not `.amp/settings.json` |
| Supports hooks | No | No | Yes |
| Supports commands | No | No (replaced by skills) | Yes |
| Supports agents | No | No (built-in only) | Yes |

### Potential Issue

The MCP discovery path in the provider definition uses `.amp/settings.json` (project-level), but the official docs describe MCP configuration in the main settings file (`~/.config/amp/settings.json`) under `amp.mcpServers`. It's unclear if `.amp/settings.json` is a valid project-level override or an incorrect path. This should be verified by testing with a local Amp installation.
