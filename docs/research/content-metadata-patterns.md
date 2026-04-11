# Content Type Metadata: Native Storage Patterns

Where do providers store the metadata that agents need to decide **when and how** to invoke each content type?

This is a cross-provider survey of native storage patterns for each content type. The focus is on invocation metadata — the fields that control *whether* and *when* a piece of content gets activated, not exhaustive field documentation.

---

## Quick Reference

| Content Type | Invocation Metadata Location | Style | Key Fields |
|---|---|---|---|
| [Rules](#rules--instructions) | In the content file | Frontmatter (or none) | `description`, `alwaysApply`/`trigger`, `globs` |
| [Skills](#skills) | In the content file | Frontmatter | `name`, `description` |
| [Agents](#agents) | In the content file or config entry | Frontmatter or JSON | `name`, `description` |
| [Commands](#commands--slash-commands) | In the content file | Frontmatter | `name`, `description` |
| [Hooks](#hooks) | In provider settings | Embedded in binding | event name + `matcher` |
| [MCP Servers](#mcp-servers) | In provider settings | Config entry key | name only — no invocation metadata |

---

## Rules / Instructions

Rules tell the agent how to behave. Invocation metadata controls *when* a rule is active.

| Provider | Location | Format | Invocation Metadata |
|---|---|---|---|
| Claude Code | `CLAUDE.md` (root) or `.claude/rules/*.md` | Frontmatter in the `.md` file | `description`, `alwaysApply`, `globs` |
| Cursor | `.cursor/rules/*.mdc` | Frontmatter in the `.mdc` file | `description`, `alwaysApply`, `globs` |
| Windsurf | `.windsurf/rules/*.md` | Frontmatter in the `.md` file | `trigger`, `description`, `globs` |
| Kiro | `.kiro/steering/*.md` | Frontmatter in the `.md` file | `inclusion`, `fileMatchPattern` |
| Roo Code | `.roo/rules/*.md` | None — directory structure only | Directory path implies mode scope (e.g. `rules-code/`) |
| Gemini CLI | `GEMINI.md` (root) | None — plain markdown | No frontmatter; always active |
| Amp | `AGENTS.md` | Frontmatter (optional) | `globs` |

**Pattern:** All providers that support conditional activation embed the activation metadata directly in the file via frontmatter. Providers that only support always-on rules use plain markdown with no metadata at all.

**No-frontmatter convention:** Several providers also accept a bare `AGENTS.md` or `CLAUDE.md` with no frontmatter — implicitly always active. Many providers will read each other's root instruction files as a fallback.

---

## Skills

Skills are loaded on demand based on relevance. The agent uses the `description` field to decide whether to invoke the skill.

| Provider | Location | Format | Invocation Metadata |
|---|---|---|---|
| Claude Code | `.claude/skills/<name>/SKILL.md` | Frontmatter in `SKILL.md` | `name`, `description` (drives auto-invocation) |
| Gemini CLI | `.gemini/skills/<name>/SKILL.md` | Frontmatter in `SKILL.md` | `name`, `description` (drives auto-activation) |
| Amp | `.agents/skills/<name>/SKILL.md` | Frontmatter in `SKILL.md` | `name`, `description` |
| Kiro | Powers (`POWER.md`) | Frontmatter in `POWER.md` | `name`, `description`; keyword-based activation |

**Pattern:** All skills implementations use frontmatter embedded in the primary content file (`SKILL.md` / `POWER.md`). The `description` field is the universal activation signal — the agent evaluates it to decide relevance.

**Key observation:** Skills are the cleanest case for frontmatter. The content file is a markdown document that *is* the skill definition, and frontmatter is the natural home for metadata. There's no configuration file or external registry involved.

---

## Agents

Agents (subagents, custom modes) have name+description that tell the orchestrating agent when to delegate.

| Provider | Location | Format | Invocation Metadata |
|---|---|---|---|
| Claude Code | `.claude/agents/<name>.md` | Frontmatter in `.md` file | `name`, `description` |
| Gemini CLI | `.gemini/agents/<name>.md` | Frontmatter in `.md` file | `name`, `description`, `kind` |
| Kiro | `.kiro/agents/<name>.json` | JSON config (no content file) | `name`, `description` in JSON |
| Roo Code | Custom modes (YAML or JSON) | YAML/JSON config | `slug`, `name`, `whenToUse` |

**Pattern:** Most providers use frontmatter in a markdown file. Kiro is the exception — agents are pure JSON configs that reference a separate `.md` file via `file://` for the system prompt content. Roo Code's custom modes are a behavioral profile system (YAML/JSON), not a markdown-first format.

---

## Commands / Slash Commands

Commands are user-invokable actions. Metadata controls discoverability and invocation behavior.

| Provider | Location | Format | Invocation Metadata |
|---|---|---|---|
| Claude Code | `.claude/commands/<name>.md` | Frontmatter in `.md` file | `name`, `description`, `argument-hint` |
| Gemini CLI | `.gemini/commands/<name>.toml` | TOML (no markdown file) | `description` field in TOML |

**Pattern:** Claude Code uses frontmatter in a markdown file. Gemini CLI uses TOML as the config format — the description is just a field rather than frontmatter, but the principle is the same: metadata is in the content file.

---

## Hooks

Hooks are event-driven automations. The "invocation metadata" is the event binding itself — there's no separate description field used for AI-driven activation.

| Provider | Location | Format | Invocation Metadata |
|---|---|---|---|
| Claude Code | `.claude/settings.json` under `hooks` key | JSON, embedded in settings | Event name + `matcher` (tool name regex) |
| Gemini CLI | `.gemini/settings.json` under `hooks` key | JSON, embedded in settings | Event name + `matcher` |
| Kiro | `.kiro/agents/<name>.json` hooks section | JSON, embedded in agent config | Event name + `matcher` |

**Pattern:** No provider has a standalone hook file with frontmatter. Hooks are always embedded as structured JSON entries in a shared settings/config file. There is no content file to put frontmatter in.

**Key observation:** The invocation condition for a hook is structural — it fires on a specific lifecycle event, optionally filtered by a tool name regex. This is different from rules/skills/agents where a language description is evaluated by the AI. The binding *is* the metadata.

---

## MCP Servers

MCP servers expose tools to the agent. Unlike other content types, there is no invocation metadata — the agent discovers available tools by querying the server at runtime.

| Provider | Location | Format | Invocation Metadata |
|---|---|---|---|
| Claude Code | `.mcp.json` or `~/.claude.json` under `mcpServers` | JSON, embedded in config | None — server name is the entry key |
| Gemini CLI | `.gemini/settings.json` under `mcpServers` | JSON, embedded in settings | None — server name is the entry key |
| Kiro | `.kiro/settings/mcp.json` | JSON, dedicated MCP config file | None — server name is the entry key |
| Roo Code | `.roo/mcp.json` | JSON | None — server name is the entry key |
| Amp | `.amp/settings.json` under `amp.mcpServers` | JSON, embedded in settings | None — server name is the entry key |

**Pattern:** All providers configure MCP servers as named entries in a JSON config file. The server name is the only identifier — there is no `description` field, no activation condition, and no frontmatter to speak of. The tools the server exposes are discovered at connection time via the MCP protocol itself.

**Key observation:** MCP is the hardest case for any metadata standard. There is no "content file" — the entry is just connection plumbing (command/url/args/env). Any human-readable description of what the server does would have to live either in a sidecar file or be inferred from the tool names returned by the server at runtime.

---

## Summary: Metadata Location by Type

| Type | Has a content file? | Frontmatter viable? | Natural metadata home |
|---|---|---|---|
| Rules | Yes (`.md`) | Yes | Frontmatter in the rule file |
| Skills | Yes (`SKILL.md`) | Yes | Frontmatter in `SKILL.md` |
| Agents | Usually yes (`.md`) | Yes | Frontmatter in the agent file |
| Commands | Yes (`.md` or `.toml`) | Yes | Frontmatter / top-level field |
| Hooks | No (JSON entry in settings) | No | The event binding IS the metadata |
| MCP Servers | No (JSON entry in settings) | No | Nowhere — no invocation metadata exists |

---

## The Sidecar Question

The above survey shows a natural split:

- **File-based types** (rules, skills, agents, commands): all providers use frontmatter embedded in the content file. The file and its metadata travel together.
- **Config-entry types** (hooks, MCP): there is no content file. Metadata is either the config structure itself (hooks) or absent entirely (MCP).

**The case for sidecar as the universal standard:**

A sidecar file (e.g. `<name>.meta.yaml` or `.syllago.yaml`) would provide a consistent metadata location regardless of content type. For MCP servers especially, it's the only way to attach a human-readable description without modifying a shared config file. It also sidesteps the problem of frontmatter being provider-specific (Claude Code's `alwaysApply` vs Windsurf's `trigger` vs Kiro's `inclusion`).

**The tension with skills:**

For skills, the `description` field in `SKILL.md` frontmatter is both the canonical metadata and the AI activation signal. If a sidecar is required, you now have two places where the description could live — and they need to stay in sync. The frontmatter is the spec-native location; the sidecar would be a platform-native addition. Publishers would reasonably ask: which one is authoritative?

**One possible resolution:**

Sidecar as optional supplement, not replacement. Skills keep their frontmatter as the authoritative activation signal. The sidecar, if present, holds platform-specific metadata (identifiers, provenance, registries) that doesn't belong in the content file. MCP and hooks, which have no content file, can use a sidecar exclusively. This preserves the clean frontmatter-in-file pattern for content types that have files, while giving a standard home for types that don't.
