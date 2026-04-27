# Cursor: Skills, Agents, and Rule Formats

Research date: 2026-03-21

---

## Table of Contents

- [Rules (.mdc format)](#rules-mdc-format)
- [Skills (SKILL.md)](#skills-skillmd)
- [Subagents (Custom Agents)](#subagents-custom-agents)
- [Cursor-Specific Concepts](#cursor-specific-concepts)

---

## Rules (.mdc format)

Rules are markdown files with YAML frontmatter stored in `.cursor/rules/`. The `.mdc` extension is Cursor-specific. As of Cursor 2.2, new rules can also be created as folders containing a `RULE.md` file, though flat `.mdc` files remain functional.

### Frontmatter Fields (Complete)

Cursor rules support exactly **three** frontmatter fields. There are no additional hidden or undocumented fields for tool permissions, model selection, or other metadata.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `description` | `string` | No | `""` | Shown in UI; used by agent to decide relevance. Critical for "Agent Requested" rules. |
| `globs` | `string \| string[]` | No | `""` | File path patterns for auto-attachment. Comma-separated string or YAML list. |
| `alwaysApply` | `boolean` | No | `false` | When `true`, rule is injected into every conversation regardless of context. |

[Official] https://cursor.com/docs/context/rules

### Globs Format

Globs accept standard glob syntax. Can be specified as:

```yaml
# Single glob as string
globs: "**/*.py"

# Comma-separated string
globs: "src/**/*.ts, tests/**/*.test.ts"

# YAML list
globs:
  - "**/*.py"
  - "tests/**/*.py"
```

[Community] https://forum.cursor.com/t/my-best-practices-for-mdc-rules-and-troubleshooting/50526

### Rule Types (Inferred from Frontmatter)

Rule "type" is not an explicit field -- it is inferred from the combination of the three frontmatter fields:

| Rule Type | Frontmatter Configuration | Behavior |
|-----------|--------------------------|----------|
| **Always** | `alwaysApply: true` | Injected into every conversation |
| **Auto-Attach** | `globs` set, `alwaysApply: false` | Attached when matching files are in context |
| **Agent Requested** | `description` set, no globs, `alwaysApply: false` | Agent reads description and decides whether to apply |
| **Manual** | No description, no globs, `alwaysApply: false` | Only applied when explicitly referenced via `@rule-name` |

[Official] https://cursor.com/docs/context/rules

### Full .mdc Example

```
---
description: Enforce PEP 484 type annotations on all Python files
globs: "**/*.py"
alwaysApply: false
---

# Python Type Hints

All functions must have full type annotations following PEP 484.

- Use `from __future__ import annotations` at the top of every file
- Prefer `X | Y` union syntax over `Union[X, Y]`
- All public functions require return type annotations
```

### Tool Permissions in Rules

**Not supported.** Cursor `.mdc` rules have no frontmatter field for tool permissions (`allowedTools`, `disallowedTools`, etc.). Tool restriction is not a concept in the Cursor rules system. The `allowedTools` pattern exists in Claude Code (CLI flag) and Kiro (`allowed-tools` in SKILL.md frontmatter), but not in Cursor.

[Inferred] Based on exhaustive search of official docs and community resources finding no such field.

### Model Selection in Rules

**Not supported.** There is no frontmatter field to specify which model a rule should use. Model selection is controlled through the Cursor UI, not through rule configuration.

[Inferred] Based on exhaustive search of official docs; NVIDIA's Cursor Rules Developer Guide confirms model selection is a UI concern, not a rule concern.

### File Locations

| Scope | Path |
|-------|------|
| Project | `.cursor/rules/*.mdc` |
| Project (2.2+ folder format) | `.cursor/rules/<name>/RULE.md` |

User/global rules are configured through Cursor Settings > General > Rules for AI (plain text, not `.mdc`).

[Official] https://cursor.com/docs/context/rules

---

## Skills (SKILL.md)

Skills are portable, version-controlled packages that teach agents domain-specific tasks. Each skill is a folder containing a `SKILL.md` file with optional supporting resources.

### SKILL.md Frontmatter Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | — | Lowercase identifier matching parent folder. Letters, numbers, hyphens only. |
| `description` | `string` | Yes | — | Explains functionality; used by agent for routing/discovery. First line becomes the skill's summary. |
| `license` | `string` | No | — | License name or file reference |
| `compatibility` | `string` | No | — | Environment/dependency requirements |
| `metadata` | `object` | No | — | Custom key-value pairs |
| `disable-model-invocation` | `boolean` | No | `false` | When `true`, prevents automatic activation; skill becomes explicit slash-command only. |

[Official] https://cursor.com/docs/context/skills

### Skill Directory Structure

```
.cursor/skills/my-skill/
  SKILL.md            # Required: frontmatter + instructions
  scripts/            # Optional: executable code (bash, python, JS, etc.)
  references/         # Optional: supplementary documentation
  assets/             # Optional: templates, images, config files
```

### File Locations

| Scope | Path |
|-------|------|
| Project | `.cursor/skills/<skill-name>/SKILL.md` |
| Project (alt) | `.agents/skills/<skill-name>/SKILL.md` |
| User/global | `~/.cursor/skills/<skill-name>/SKILL.md` |

[Official] https://cursor.com/docs/context/skills

### Discovery and Invocation

- **Automatic**: Cursor scans skill directories on startup and presents them contextually based on the `description` field.
- **Manual**: Type `/skill-name` or `@skill-name` in agent chat.
- **Progressive loading**: Supporting materials (scripts, references) are loaded on-demand, not preloaded.

Setting `disable-model-invocation: true` converts a skill to explicit-only (slash command), preventing automatic contextual application.

[Official] https://cursor.com/docs/context/skills

### SKILL.md Example

```
---
name: api-testing
description: Generate and run API integration tests using pytest and httpx
compatibility: Python 3.10+, pytest, httpx
disable-model-invocation: false
---

# API Testing Skill

## When to Use
Use this skill when creating or updating API integration tests.

## Steps
1. Create test file in `tests/integration/`
2. Use httpx.AsyncClient for all HTTP calls
3. Follow AAA pattern (Arrange, Act, Assert)

## Conventions
- Test files: `test_<endpoint_name>.py`
- Fixtures in `conftest.py`
```

---

## Subagents (Custom Agents)

Custom subagents are markdown files with YAML frontmatter that define specialized agents for task delegation. The main agent reads the description to decide when to delegate work.

### Subagent Frontmatter Fields (Complete)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | No | Derived from filename | Display name and identifier. Lowercase letters and hyphens. |
| `description` | `string` | No | — | Short description shown in Task tool hints. Agent reads this to decide delegation. |
| `model` | `string` | No | `inherit` | Which model the subagent uses (see Model Selection below). |
| `readonly` | `boolean` | No | `false` | When `true`, subagent cannot edit files or run state-changing shell commands. |
| `is_background` | `boolean` | No | `false` | When `true`, subagent runs in background without blocking the parent. |

[Official] https://cursor.com/docs/context/subagents

### Model Selection Values

The `model` field on subagents accepts three types of values:

| Value | Behavior |
|-------|----------|
| `inherit` | Uses the same model as the parent agent (default) |
| `fast` | Uses a smaller, faster model optimized for speed and cost |
| `<model-id>` | Specific model ID, e.g. `claude-4-sonnet` |

[Official] https://cursor.com/docs/context/subagents

### Tool Permissions

- Subagents **inherit all tools** from the parent agent, including MCP tools.
- Setting `readonly: true` restricts the subagent: no file edits, no state-changing shell commands.
- There is no fine-grained `allowedTools` or `disallowedTools` field -- it is binary (full access or readonly).

[Official] https://cursor.com/docs/context/subagents

### File Locations

| Scope | Path |
|-------|------|
| Project | `.cursor/agents/<name>.md` |
| Project (alt) | `.claude/agents/<name>.md`, `.codex/agents/<name>.md` |
| User/global | `~/.cursor/agents/<name>.md` |
| User/global (alt) | `~/.claude/agents/<name>.md`, `~/.codex/agents/<name>.md` |

Project subagents take precedence when names conflict with user-level ones.

[Official] https://cursor.com/docs/context/subagents

### Built-in Subagents

Cursor ships three built-in subagents:

| Name | Purpose |
|------|---------|
| **Explore** | Navigates large repos, finds relevant files, builds context |
| **Bash** | Runs shell commands in isolation |
| **Browser** | Handles browser interactions, filters DOM snapshots |

[Community] https://medium.com/@codeandbird/cursor-subagents-complete-guide-5853e8d39176

### Subagent Example

```
---
name: security-auditor
description: Reviews code changes for security vulnerabilities including injection, auth bypasses, and secret exposure.
model: inherit
readonly: true
is_background: false
---

# Security Auditor

You are a security-focused code reviewer. Analyze code changes for:

- SQL injection and XSS vulnerabilities
- Authentication and authorization bypasses
- Hardcoded secrets or credentials
- Insecure cryptographic practices

Report findings with severity levels (Critical, High, Medium, Low).
Do not modify any files.
```

---

## Cursor-Specific Concepts

### Content Type Comparison

| Concept | Format | Auto-Discovery | User Invocation | Agent Decides |
|---------|--------|---------------|-----------------|---------------|
| Rule | `.mdc` / `RULE.md` | Via globs/alwaysApply | `@rule-name` | Yes (Agent Requested type) |
| Skill | `SKILL.md` folder | On startup | `/skill-name` or `@skill-name` | Yes (unless `disable-model-invocation`) |
| Subagent | `<name>.md` | On startup | Via Task tool | Yes (reads description) |

### Key Differences from Other Providers

- **No tool permissions in rules**: Unlike Claude Code (which has `allowed_tools` in CLAUDE.md) or Kiro (`allowed-tools` in SKILL.md), Cursor rules have no tool permission fields. Tool restriction only exists on subagents via the binary `readonly` flag.
- **No model selection in rules**: Model selection exists only on subagents (`model` field), not on rules or skills.
- **Rule type is implicit**: Determined by frontmatter field combinations rather than an explicit `type` field.
- **Shared agent directories**: Cursor reads `.claude/agents/` and `.codex/agents/` in addition to `.cursor/agents/`, enabling cross-provider subagent sharing.
- **Skills are Cursor's equivalent of Claude Code skills**: Both use `SKILL.md` files with similar structure but different frontmatter schemas.

### Automations (March 2026)

Cursor introduced "Automations" for always-on agents that run on schedules or event triggers (Slack messages, Linear issues, GitHub PRs, PagerDuty incidents, webhooks). These run in cloud sandboxes with configured MCPs and models. Automations include a memory tool for learning from past runs.

[Official] https://cursor.com/blog/automations

---

## Sources

- [Cursor Rules Docs](https://cursor.com/docs/context/rules) [Official]
- [Cursor Skills Docs](https://cursor.com/docs/context/skills) [Official]
- [Cursor Subagents Docs](https://cursor.com/docs/context/subagents) [Official]
- [Cursor Automations Blog](https://cursor.com/blog/automations) [Official]
- [Cursor Skills Help Page](https://cursor.com/help/customization/skills) [Official]
- [Cursor Forum: Best Practices for MDC Rules](https://forum.cursor.com/t/my-best-practices-for-mdc-rules-and-troubleshooting/50526) [Community]
- [Cursor Forum: Deep Dive into Cursor Rules](https://forum.cursor.com/t/a-deep-dive-into-cursor-rules-0-45/60721) [Community]
- [Medium: Cursor Subagents Complete Guide](https://medium.com/@codeandbird/cursor-subagents-complete-guide-5853e8d39176) [Community]
- [NVIDIA Cursor Rules Developer Guide](https://docs.nvidia.com/nemo/agent-toolkit/1.2/extend/cursor-rules-developer-guide.html) [Community]
- [Builder.io: Agent Skills vs Rules vs Commands](https://www.builder.io/blog/agent-skills-rules-commands) [Community]
