# Crush: Skills, Agents, and Rule Formats

Research date: 2026-03-30

---

## Table of Contents

- [Rules (AGENTS.md)](#rules-agentsmd)
- [Skills (SKILL.md)](#skills-skillmd)
- [Custom Agents](#custom-agents)
- [Crush-Specific Concepts](#crush-specific-concepts)

---

## Rules (AGENTS.md)

Crush reads `AGENTS.md` files for project-level instructions. This is the
cross-provider agent instructions format, not a Crush-specific format. [Official:
https://github.com/charmbracelet/crush]

### Format

**File format:** Standard Markdown. No frontmatter, no special schema.

**Location:** Project root (`AGENTS.md`).

**Behavior:** Contents are injected as system-level instructions for the agent.
[Unverified -- exact injection mechanics and subdirectory discovery not
confirmed]

### No Provider-Specific Rules

Crush does **not** have its own rules format. Unlike Cursor (`.mdc` files with
activation types), Gemini CLI (`GEMINI.md`), or Claude Code (`CLAUDE.md`), Crush
has no provider-specific rules system beyond `AGENTS.md`.

There is no:
- `.crushrules` file
- `.crush/rules/` directory
- Activation types (always-on, auto-attach, agent-requested)
- Frontmatter fields for rules
- Global user rules file

[Unverified -- based on absence from official docs and schema; no Crush-specific
rules format found]

---

## Skills (SKILL.md)

Crush supports the Agent Skills open standard. Skills are folders containing a
`SKILL.md` file that provide domain-specific instructions and optional
supporting resources. [Official: https://github.com/charmbracelet/crush]

### Discovery Paths

Skills are discovered from multiple directories in priority order:

| Path | Scope |
|------|-------|
| `$CRUSH_SKILLS_DIR` | Environment override |
| `~/.config/agents/skills/` | Cross-provider user skills |
| `~/.config/crush/skills/` | Crush-specific user skills |
| `.agents/skills/` | Project cross-provider |
| `.crush/skills/` | Project Crush-specific |
| `.claude/skills/` | Project (Claude Code compat) |
| `.cursor/skills/` | Project (Cursor compat) |

[Official: https://github.com/charmbracelet/crush]

### Skill Directory Structure

```
.crush/skills/my-skill/
  SKILL.md            # Required: instructions
  scripts/            # Optional: executable code
  references/         # Optional: supplementary docs
  assets/             # Optional: templates, data files
```

[Unverified -- directory layout assumed to follow Agent Skills standard; exact
Crush implementation not confirmed beyond SKILL.md requirement]

### SKILL.md Frontmatter Fields

Crush follows the Agent Skills standard. Exact frontmatter fields accepted by
Crush have not been confirmed from official sources.

Expected fields (from the Agent Skills standard):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Skill identifier |
| `description` | string | Yes | What the skill does; used for discovery |

[Unverified -- assumed from Agent Skills standard, not confirmed for Crush
specifically]

### Example

```markdown
---
name: deploy-staging
description: Deploys the application to the staging environment
---

# Deploy to Staging

## Steps
1. Run test suite: `go test ./...`
2. Build binary: `go build -o app .`
3. Deploy: `./scripts/deploy.sh staging`
```

---

## Custom Agents

**Not supported.** Crush does not support user-defined agent configuration files
as of 2026-03-30.

Crush has a two-model architecture:

| Role | Purpose |
|------|---------|
| **Large** ("Coder Agent") | Complex coding tasks |
| **Small** ("Task Agent") | Simpler operations |

Model roles are configured in `crush.json` under the `models` key:

```json
{
  "models": {
    "large": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    },
    "small": {
      "provider": "anthropic",
      "model": "claude-haiku-4-20250514"
    }
  }
}
```

Users can switch models mid-session while preserving context. [Official:
https://github.com/charmbracelet/crush]

There is no equivalent to:
- Claude Code's `.claude/agents/*.md`
- Cursor's `.cursor/agents/<name>.md` with `model`, `readonly`, `is_background`
  fields
- Custom agent personas or behavioral profiles

The `AgentTool` (sub-agent delegation) exists as a built-in tool but is not
user-configurable. [Community: https://deepwiki.com/charmbracelet/crush]

---

## Crush-Specific Concepts

### Content Type Comparison

| Concept | Crush | Cursor | Claude Code | Gemini CLI |
|---------|-------|--------|-------------|------------|
| Rules format | `AGENTS.md` only | `.mdc` + `AGENTS.md` | `CLAUDE.md` + `.claude/rules/` | `GEMINI.md` |
| Skills | `SKILL.md` (standard) | `SKILL.md` (extended) | `SKILL.md` (extended) | `SKILL.md` (standard) |
| Custom agents | No | Yes (`.cursor/agents/`) | Yes (`.claude/agents/`) | No |
| Commands | No | Yes (`.cursor/commands/`) | Yes (slash commands) | Yes (TOML commands) |
| Hooks | No | Yes (`hooks.json`) | Yes (settings.json) | Yes (settings.json) |

### Key Differences from Other Providers

- **No provider-specific rules**: Unlike every other major provider, Crush has
  no proprietary rules format. It relies solely on the cross-provider
  `AGENTS.md`.
- **Cross-provider skill discovery**: Crush scans `.claude/skills/` and
  `.cursor/skills/` in addition to its own `.crush/skills/`, enabling seamless
  skill sharing across providers.
- **No lifecycle hooks**: The most significant gap. See [hooks.md](hooks.md).
- **Two-model architecture**: Fixed large/small model roles rather than
  configurable agent profiles.

---

## Sources

- [Crush GitHub Repository](https://github.com/charmbracelet/crush) [Official]
- [Crush JSON Schema](https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json) [Official]
- [Crush Architecture -- DeepWiki](https://deepwiki.com/charmbracelet/crush) [Community]
- [Agent System -- DeepWiki](https://deepwiki.com/charmbracelet/crush/4-ai-and-llm-integration) [Community]
