# Pi: Skills and Agent Capabilities

> **Identity note:** "Pi" refers to Mario Zechner's `pi` coding agent
> (`github.com/badlogic/pi-mono`). Not to be confused with Raspberry Pi hardware
> or other "Pi" named projects.

Research date: 2026-03-30

---

## Table of Contents

- [Skills (SKILL.md)](#skills-skillmd)
- [Sub-agents (via Extensions)](#sub-agents-via-extensions)
- [Prompt Templates as Lightweight Commands](#prompt-templates-as-lightweight-commands)

---

## Skills (SKILL.md)

Pi implements the Agent Skills standard. Skills are self-contained capability
packages that agents load dynamically using progressive disclosure -- only names
and descriptions remain in context until the agent determines a skill is
relevant.

### Discovery Locations

Skills load from multiple sources, checked in order:

| Source | Path / Mechanism |
|---|---|
| Project | `.pi/skills/<name>/SKILL.md` |
| Global | `~/.pi/agent/skills/<name>/SKILL.md` |
| Packages | `skills/` directory or `pi.skills` in `package.json` |
| Settings | `skills` array in `settings.json` |
| CLI | `--skill <path>` |

Discovery can be disabled entirely with `--no-skills`.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md]

### File Structure

Each skill is a directory containing a `SKILL.md` file with YAML frontmatter and
freeform markdown instructions. Additional files (helper scripts, reference docs,
assets) are allowed alongside `SKILL.md`.

```
.pi/skills/
  code-review/
    SKILL.md
    review-checklist.md   # optional supporting file
  deploy/
    SKILL.md
    deploy.sh             # optional helper script
```

### SKILL.md Format

```markdown
---
name: code-review
description: >
  Reviews code changes for bugs, style issues, and security concerns.
  Use when asked to review a PR, diff, or set of changes.
---

## Instructions

When reviewing code, follow these steps:
1. Read the diff or changed files
2. Check for common issues...
```

### Frontmatter Fields

| Field | Type | Required | Validation |
|---|---|---|---|
| `name` | string | Yes | 1-64 chars, lowercase alphanumeric + hyphens, no leading/trailing/consecutive hyphens |
| `description` | string | Yes (skill won't load without it) | Should clearly state what the skill does and when to use it |
| `license` | string | No | License identifier |
| `compatibility` | string | No | Compatibility notes |
| `metadata` | object | No | Arbitrary key-value pairs |
| `disable-model-invocation` | boolean | No | When true, hides skill from system prompt |

### Naming Rules

- Must match the parent directory name exactly
- Lowercase alphanumeric characters and hyphens only
- No leading or trailing hyphens
- No consecutive hyphens
- 1 to 64 characters

### Validation Behavior

Pi is lenient on most validation violations (warnings only), with one exception:
**missing descriptions cause the skill to not load at all**. This is because the
description drives progressive disclosure -- without it, the agent cannot decide
when to load the skill.

Name collisions (duplicate names from different sources) retain the first
discovered skill and issue a warning.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md]

### Loading Model (Progressive Disclosure)

1. At startup, Pi scans all skill sources and collects `name` + `description`
2. These summaries are included in the system prompt
3. When the agent determines a skill is relevant to the current task, the full
   `SKILL.md` content is loaded into context
4. Additional files in the skill directory can be referenced by the skill
   instructions

This keeps the base context small while making capabilities discoverable.

### Skill Commands

When `enableSkillCommands` is true (default), each skill is also available as a
slash command: `/code-review` triggers the `code-review` skill directly. This
can be disabled in settings.

---

## Sub-agents (via Extensions)

Pi does not have a built-in sub-agent system. Instead, sub-agents are
implemented via the extension API, which provides all the primitives needed:

- `ctx.sendMessage()` -- send messages programmatically
- `ctx.fork()` -- fork the session for parallel work
- `ctx.newSession()` -- create a fresh session
- Custom tools with full TypeScript logic

The official examples include a complete sub-agent implementation at
`examples/extensions/subagent/` that demonstrates:

- Agent definitions (planner, reviewer, scout, worker) as markdown files
- Prompt templates for multi-step workflows (scout-and-plan, implement,
  implement-and-review)
- An orchestrator extension that coordinates agents

```
examples/extensions/subagent/
  index.ts              # Orchestrator extension
  agents.ts             # Agent registration
  agents/
    planner.md          # Planner agent definition
    reviewer.md         # Review agent definition
    scout.md            # Scout agent definition
    worker.md           # Worker agent definition
  prompts/
    implement-and-review.md
    implement.md
    scout-and-plan.md
```

[Official: https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent/examples/extensions/subagent]

---

## Prompt Templates as Lightweight Commands

Prompt templates (`.pi/prompts/*.md`) serve as lightweight reusable commands.
They are not skills -- they do not use progressive disclosure and their full
content is expanded immediately on invocation.

See [content-types.md](content-types.md#4-prompt-templates) for format details.

**When to use skills vs. prompt templates:**

| Use Case | Mechanism |
|---|---|
| Complex, multi-step capability with supporting files | Skill |
| Simple reusable prompt snippet | Prompt template |
| Capability the agent should discover on its own | Skill |
| Explicit user-invoked command | Prompt template |
| Needs to be hidden from model | Skill (with `disable-model-invocation`) |

---

## Comparison with Other Providers

| Feature | Pi | Claude Code | Cursor |
|---|---|---|---|
| Skills format | SKILL.md with YAML frontmatter | SKILL.md with YAML frontmatter | SKILL.md with YAML frontmatter |
| Skills location | `.pi/skills/` | `.claude/skills/` | `.cursor/skills/` |
| Progressive disclosure | Yes | Yes | Yes |
| Sub-agents | Via extensions (programmatic) | Not user-definable | Not user-definable |
| Prompt templates | `.pi/prompts/*.md` with `$1` args | `.claude/commands/*.md` with `$ARGUMENTS` | `.cursor/commands/*.md` |
| Package distribution | npm/git packages | Not supported | Not supported |

## Sources

- [Skills Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md) [Official]
- [Extensions Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md) [Official]
- [Sub-agent Example](https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent/examples/extensions/subagent) [Official]
- [Prompt Templates Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/prompt-templates.md) [Official]
