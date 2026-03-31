# Factory Droid — Format Reference

Provider slug: `factory-droid`

Supports: Rules, Skills, Custom Droids (agents), Hooks, MCP, Commands

Sources: [Factory Droid docs](https://docs.factory.ai/cli/configuration),
[llms.txt](https://docs.factory.ai/llms.txt)

**Identity note:** "Factory Droid" refers to Factory AI's coding agent CLI
(`factory-droid`). No known naming conflicts with other tools as of 2026-03-30.

---

## Rules

**Location:** `AGENTS.md` in project root

**Format:** Plain Markdown (no frontmatter, no activation modes)

Cross-provider convention shared with Cursor, Windsurf, and others. Acts as a
briefing document covering build commands, architecture, coding conventions, and
security guidelines.

[Official: https://docs.factory.ai/cli/configuration]

---

## Skills

**Location:** `.factory/skills/<skill-name>/SKILL.md` (project) or
`~/.factory/skills/<skill-name>/SKILL.md` (personal)

**Format:** Markdown with YAML frontmatter

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Identifier; defaults to directory name |
| `description` | string | Recommended | Guides agent invocation decisions |
| `user-invocable` | bool | No | `false` = agent-only (default: `true`) |
| `disable-model-invocation` | bool | No | `true` = user-only (default: `false`) |

[Official: https://docs.factory.ai/cli/configuration/skills]

---

## Custom Droids (Agents)

**Location:** `.factory/droids/<name>.md` (project) or
`~/.factory/droids/<name>.md` (personal)

**Format:** Markdown with YAML frontmatter

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Lowercase alphanumeric + hyphens/underscores |
| `description` | string | No | Max 500 characters |
| `model` | string | No | `"inherit"` (default) or specific model ID |
| `tools` | string[] | No | Tool names or categories; omit for all tools |
| `reasoningEffort` | string | No | For compatible models |

Project droids override personal droids with the same name.

[Official: https://docs.factory.ai/cli/configuration/custom-droids]

---

## Hooks

**Location:** `.factory/settings.json` (under `hooks` key)

**Format:** JSON -- same schema as Claude Code (event to matcher-group arrays)

**Events:** `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Stop`,
`SessionStart`, `SessionEnd`, `PreCompact`, `SubagentStart`, `SubagentStop`

**Blocking:** Exit code 2 blocks on pre-hooks. Advanced JSON output supports
`permissionDecision` (`allow`/`deny`/`ask`).

[Official: https://docs.factory.ai/cli/configuration/hooks-guide]

---

## MCP

**Location:** `.factory/mcp.json` (project) or `~/.factory/mcp.json` (user)

**Format:** JSON with `mcpServers` top-level key

**Transports:** `stdio` (local processes) and `http` (remote endpoints)

**Per-server fields:** `type`, `command`/`url`, `args`, `env`, `headers`,
`disabled`, `disabledTools`

[Official: sourced from llms.txt index render; direct URL returned 404]

---

## Commands

**Location:** `.factory/commands/<name>.md` (project) or
`~/.factory/commands/<name>.md` (personal)

**Format:** Markdown (with optional YAML frontmatter) or shebang scripts

**Frontmatter fields:** `description`, `argument-hint`, `allowed-tools`

Filenames become slash command names. Only top-level files register; nested
directories are ignored.

[Official: https://docs.factory.ai/cli/configuration/custom-slash-commands]
