# Windsurf Skills & Agents

## Overview

Windsurf supports two content types relevant to this category:

1. **Skills** — folders containing a `SKILL.md` file plus supporting resources, following the open Agent Skills standard (agentskills.io). Cascade loads skills on demand via progressive disclosure.
2. **AGENTS.md** — plain markdown files placed in project directories that provide location-scoped instructions to Cascade. These feed into the same Rules engine as `.windsurf/rules/`.

Windsurf does NOT have a separate "agent definition" format. Cascade is the single built-in agent; customization happens through rules, skills, and workflows.

## Skills

### File Structure

Each skill is a directory containing at minimum a `SKILL.md` file:

```
skill-name/
├── SKILL.md          # Required: YAML frontmatter + markdown instructions
├── scripts/          # Optional: executable code
├── references/       # Optional: additional documentation
├── assets/           # Optional: templates, resources
└── ...               # Any additional files
```

[Official] https://docs.windsurf.com/windsurf/cascade/skills
[Official] https://agentskills.io/specification

### SKILL.md Format

YAML frontmatter followed by markdown body content.

#### Frontmatter Fields

| Field | Required | Constraints | Description |
|-------|----------|-------------|-------------|
| `name` | Yes | Max 64 chars. Lowercase letters, numbers, hyphens only. Must match parent directory name. No leading/trailing/consecutive hyphens. | Unique identifier for the skill |
| `description` | Yes | Max 1024 chars. Non-empty. | What the skill does and when to use it. Drives automatic invocation. |
| `license` | No | Short string or reference to bundled LICENSE file | License applied to the skill |
| `compatibility` | No | Max 500 chars | Environment requirements (target product, system packages, network access) |
| `metadata` | No | Map of string keys to string values | Arbitrary key-value pairs (commonly `author`, `version`) |
| `allowed-tools` | No | Space-delimited list | Pre-approved tools the skill may use. **Experimental** — support varies by agent. |

[Official] https://agentskills.io/specification

#### Example SKILL.md

```markdown
---
name: deploy-staging
description: |
  Deploy to staging environment with pre-flight checks.
  Use when user mentions "deploy", "staging", or "release candidate".
license: MIT
metadata:
  author: platform-team
  version: "2.1"
allowed-tools: Bash(git:*) Bash(docker:*) Read
---

## Steps

1. Run pre-deploy checks via `scripts/pre-deploy-checks.sh`
2. Validate environment config against `environment-template.env`
3. Deploy using the project's deploy script
4. If deployment fails, follow `rollback-steps.md`
```

#### Body Content

The markdown body contains the actual skill instructions. No format restrictions. Recommended sections:
- Step-by-step instructions
- Input/output examples
- Edge cases

Keep under 500 lines. Move detailed reference material to separate files in `references/`.

### Storage Locations

| Scope | Path | Notes |
|-------|------|-------|
| Workspace | `.windsurf/skills/<skill-name>/` | Project-specific; typically committed to repo |
| Global | `~/.codeium/windsurf/skills/<skill-name>/` | All workspaces; not version-controlled |
| System (Enterprise) | OS-specific paths | Read-only; deployed via MDM |

### Cross-Agent Discovery Paths

Windsurf also discovers skills from these alternative locations for cross-tool compatibility:

| Path | Scope |
|------|-------|
| `.agents/skills/` | Workspace |
| `~/.agents/skills/` | Global |
| `.claude/skills/` | Workspace (if Claude Code config reading enabled) |
| `~/.claude/skills/` | Global (if Claude Code config reading enabled) |

[Official] https://docs.windsurf.com/windsurf/cascade/skills

### Progressive Disclosure

Skills use a three-tier loading strategy to conserve context window:

1. **Metadata** (~100 tokens): `name` and `description` loaded at startup for all skills
2. **Instructions** (<5000 tokens recommended): Full `SKILL.md` body loaded when skill is activated
3. **Resources** (as needed): Supporting files (`scripts/`, `references/`, `assets/`) loaded only when required

[Official] https://agentskills.io/specification

### Invocation

| Method | How | When to Use |
|--------|-----|-------------|
| Automatic | Cascade matches user request to skill descriptions | Default behavior; keep descriptions specific and action-oriented |
| Manual | Type `@skill-name` in Cascade input | When you want to guarantee a specific skill is used |

### Enterprise Skills

Enterprise organizations can deploy system-level skills that:
- Are available across all workspaces for all team members
- Cannot be modified by end users
- Are deployed via MDM or cloud dashboard

[Official] https://docs.windsurf.com/windsurf/cascade/skills

## AGENTS.md

### Format

Plain markdown. No frontmatter required. Supports headers, bullet points, code blocks, examples.

### File Recognition

Windsurf discovers files named `AGENTS.md` or `agents.md` (case-insensitive) throughout the workspace and parent directories up to the git root.

### Scoping Rules

Location-based automatic scoping:

| File Location | Behavior |
|---------------|----------|
| Project root | "Always-on" rule — included in every system prompt |
| Subdirectory | Glob rule with pattern `<directory>/**` — applies only when Cascade interacts with files in that directory |

[Official] https://docs.windsurf.com/windsurf/cascade/agents-md

### Relationship to Rules

AGENTS.md files feed into the same Rules engine as `.windsurf/rules/` files:

| Aspect | AGENTS.md | .windsurf/rules/ |
|--------|-----------|-------------------|
| Location | Anywhere in project tree | `.windsurf/rules/` directory |
| Scoping | Automatic (location-based) | Manual (glob, always-on, model decision) |
| Best use | Directory-specific conventions | Cross-cutting concerns, complex logic |

Use AGENTS.md for simple location-scoped instructions. Use Rules when you need more control over activation modes.

[Official] https://docs.windsurf.com/windsurf/cascade/agents-md

## What Windsurf Does NOT Have

- **Custom agent definitions**: No way to define multiple named agents with different models, system prompts, or tool permissions. Cascade is the single agent.
- **Model selection per skill**: The Agent Skills spec does not include a `model` field. Some community sources mention it, but it is NOT part of the official spec and Windsurf does not document it.
- **Workflow agents**: Windsurf has a separate "Workflows" feature (`.windsurf/workflows/`) for multi-step task templates, but these are prompt sequences, not agent definitions.

[Inferred] Based on absence from official docs at https://docs.windsurf.com

## Cascade Modes

Cascade operates in three modes (not configurable per-skill):

| Mode | Purpose |
|------|---------|
| **Code** | Full agentic mode — reads, writes, runs commands |
| **Ask** | Read-only analysis and explanation |
| **Plan** | Creates detailed implementation plans before coding |

[Official] https://windsurf.com/cascade

## AI Models

Windsurf offers these models for Cascade (user selects globally, not per-skill):

| Model | Description |
|-------|-------------|
| SWE-1.5 | Best agentic coding model; near Claude 4.5-level performance at 13x speed |
| SWE-1 | First agentic model; Claude 3.5-level performance at lower cost |
| SWE-1-mini | Powers passive suggestions in Windsurf Tab (autocomplete) |
| BYOK models | Users can bring their own API keys for select models |

[Official] https://docs.windsurf.com/windsurf/models

## Syllago Mapping Notes

- Skills map directly to syllago's skill content type (SKILL.md + supporting files in a directory)
- AGENTS.md maps to syllago's rules content type (plain markdown instructions)
- No agent content type mapping — Windsurf has no custom agent definitions
- Cross-agent discovery paths (`.agents/skills/`, `.claude/skills/`) are important for portability
- The Agent Skills spec is the open standard; Windsurf follows it with some extensions (enterprise deployment)
- `allowed-tools` field is experimental and may not be enforced
