<!-- provider-audit-meta
provider: amp
provider_version: "unknown (auth-gated)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/blog
-->

# Amp — Content Types

## Rules

**Format:** Markdown (`AGENTS.md`)
**Paths:**
- Project: `AGENTS.md` in project root [Inferred from provider definition]
- Global: `~/.config/amp/AGENTS.md` [Inferred from provider definition]

Amp uses `AGENTS.md` files for team-wide AI agent guidance, similar to Codex. This includes codebase conventions, build commands, and context instructions. [Community]

**Relationship to CLAUDE.md:** Amp's `AGENTS.md` is functionally equivalent to Claude Code's `CLAUDE.md` — both provide system-level instructions to the AI agent. The naming reflects Amp's multi-model, multi-agent philosophy. [Inferred]

## Skills

**Format:** Markdown (SKILL.md with frontmatter)
**Paths:**
- Project: `.agents/skills/` [Inferred from provider definition]
- Project (compat): `.claude/skills/` (fallback) [Inferred from provider definition]
- Global: `~/.config/agents/skills/` [Inferred from provider definition]

Skills are user-defined composable instructions that agents can invoke. Amp introduced skills as a replacement for custom commands. [Community]

**Directory structure:** Each skill is a directory containing a `SKILL.md` file, similar to Claude Code's skill format. [Inferred]

## MCP (Model Context Protocol)

**Format:** JSON (settings file merge)
**Path:** `.amp/settings.json` in project root [Inferred from provider definition]

MCP server configurations are stored as JSON and merged into the provider's settings file, following the same pattern as Claude Code and other providers. [Inferred]

## Agents

**Not currently supported by syllago for Amp.** Amp has a rich agent system internally (subagents, Oracle, Code Review agents), but the file format for user-defined agent definitions is not documented in public sources. [Unverified]

## Hooks

**Not supported.** No evidence of an event-based hooks system in Amp. Amp's automation model is agent-based (parallel course correction agents, background processes) rather than event-triggered scripts. [Inferred]

## Commands

**Not supported.** Amp replaced custom commands with the skills system. [Community]

## Symlink Support

| Content Type | Symlinks |
|-------------|----------|
| Rules | Yes |
| Skills | Yes |
| MCP | No (JSON merge) |

[Inferred from provider definition]

## Key Directories

| Directory | Purpose |
|-----------|---------|
| `.config/amp/` | Global Amp configuration (home directory) |
| `.agents/skills/` | Project-level skills |
| `.agents/checks/` | Custom code review checks (YAML frontmatter) |
| `.amp/` | Project-level Amp settings |
| `AGENTS.md` | Project-level rules |

## Documentation Gaps

- Exact AGENTS.md format specification (behind auth wall)
- MCP settings.json schema and supported fields
- Whether Amp supports agent definitions as content files
- Full skill format specification and frontmatter fields
- Whether `.claude/skills/` compatibility is intentional or a Claude Code heritage artifact
