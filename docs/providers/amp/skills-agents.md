<!-- provider-audit-meta
provider: amp
provider_version: "unknown (auth-gated)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/blog
-->

# Amp — Skills & Agents

## Skills

**Status:** Supported (syllago can install/discover skills for Amp)

### Format
- Markdown-based skill definitions [Inferred]
- Likely SKILL.md files with frontmatter, similar to Claude Code [Inferred]
- Skills replaced custom commands in Amp [Community]

### Directory Structure
- Project: `.agents/skills/<skill-name>/SKILL.md` [Inferred from provider definition]
- Project (compat fallback): `.claude/skills/<skill-name>/SKILL.md` [Inferred from provider definition]
- Global: `~/.config/agents/skills/<skill-name>/SKILL.md` [Inferred from provider definition]

### Invocation
- Skills are composable instructions that agents can invoke on demand [Community]
- Specific invocation syntax (slash command, natural language, etc.) is not documented publicly [Unverified]

### Unique Features
- Multi-model routing may allow skills to specify preferred models [Unverified]
- Skills can potentially leverage Amp-specific tools (Oracle, Librarian, Code Review) [Unverified]

## Agents

**Status:** Not supported by syllago for Amp

### Built-in Agent Types
Amp has several built-in specialized agents [Community]:

| Agent | Purpose |
|-------|---------|
| Oracle | Architectural review, second-opinion reasoning |
| Librarian | External repository exploration and documentation loading |
| Code Review | Automated code review with customizable checks |
| Course Correct | Parallel correction agent for quality monitoring |

### Custom Agent Definitions
- No publicly documented format for user-defined agent definitions [Unverified]
- Amp's agent system appears to be internal/built-in rather than user-extensible via files [Inferred]
- The `.agents/` directory is used for skills and checks, not agent definitions [Inferred]

### Custom Checks (Agent Extension)
Amp supports custom code review checks via `.agents/checks/` [Community]:
- Files use YAML frontmatter with `name` and `description` fields
- Acts as domain-specific review criteria for the Code Review agent
- This is the closest equivalent to user-defined agent customization

## Model Selection

Amp uses multi-model routing rather than single-model selection [Community]:

| Mode | Models |
|------|--------|
| Smart | Claude Opus 4.6, GPT-5 variants |
| Deep | GPT-5.3-Codex (specialized) |
| Rush | Lighter-weight models |

Model selection is handled at the platform level, not per-skill or per-agent. [Inferred]

## SDK Integration

Amp provides TypeScript (`@sourcegraph/amp-sdk`) and Python SDKs for programmatic embedding [Community]:
- Custom tools can be defined via a toolbox directory parameter
- This is an API-level integration, not a content-type that syllago would manage

## Documentation Gaps

- Full skill format specification and supported frontmatter fields
- Whether skills support `is_mode`, `model`, or tool permission fields
- How skills are triggered (slash commands? natural language? both?)
- Whether user-defined agent files are supported (vs only built-in agents)
- Whether the `.claude/skills/` fallback path is an intentional compatibility feature
- Whether Amp's skills format diverges from Claude Code's SKILL.md format
