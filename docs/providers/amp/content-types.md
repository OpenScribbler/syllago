<!-- provider-audit-meta
provider: amp
provider_version: "current (rolling release)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/manual
-->

# Amp — Content Types

## Rules (AGENTS.md)

**Format:** Markdown
**Paths (searched in order):**
- Current working directory and all parent directories up to `$HOME`
- Subtree directories (included when agent reads files in that subtree)
- `$HOME/.config/amp/AGENTS.md`
- `$HOME/.config/AGENTS.md`

**Fallback file names:** `AGENT.md`, `CLAUDE.md` [Official]

**@-mention syntax:** Reference other files with `@path/to/file`. Supports:
- Relative paths (relative to the AGENTS.md file)
- Absolute paths
- Home directory: `@~/path`
- Glob patterns: `@doc/*.md`, `@.agent/**/*.md`

**Granular guidance via frontmatter:** Referenced files can include YAML frontmatter with `globs` to scope when they load:

```yaml
---
globs:
  - '**/*.ts'
  - '**/*.tsx'
---
```

Files with globs only load when Amp reads matching files. Globs are implicitly prefixed with `**/` unless starting with `../` or `./`. [Official]

## Skills

**Format:** Markdown (SKILL.md with YAML frontmatter)

**Directory structure:** Each skill is a directory containing `SKILL.md`.

**Required frontmatter fields:**
- `name` — unique identifier
- `description` — visible to model, determines when skill invokes

**Optional components:**
- Bundled resources (scripts, templates) in the same directory
- `mcp.json` file for bundled MCP servers

**Skill search precedence (highest to lowest):**
1. `~/.config/agents/skills/`
2. `~/.config/amp/skills/`
3. `.agents/skills/` (project)
4. `.claude/skills/` (project, compatibility)
5. `~/.claude/skills/` (user, compatibility)
6. Plugins, toolbox directories, built-in skills

**Additional paths:** Configurable via `amp.skills.path` setting. [Official]

**Override behavior:** Project skills override user-wide skills, which override built-in skills (by name). [Official]

## MCP (Model Context Protocol)

**Format:** JSON

**Configuration locations:**
- Settings file: `amp.mcpServers` in `settings.json`
- Skill-bundled: `mcp.json` in a skill directory
- CLI flags

**Loading precedence:** CLI flags > User/workspace config > Skills [Official]

**Local command-based server:**

```json
{
  "chrome-devtools": {
    "command": "npx",
    "args": ["-y", "chrome-devtools-mcp@latest"],
    "includeTools": ["navigate_*", "take_screenshot"],
    "env": {}
  }
}
```

Fields: `command` (required), `args` (optional), `env` (optional), `includeTools` (optional, recommended)

**Remote HTTP server:**

```json
{
  "linear": {
    "url": "https://mcp.linear.app/sse",
    "headers": {},
    "includeTools": ["list_issues", "create_issue"]
  }
}
```

Fields: `url` (required), `headers` (optional), `includeTools` (optional)

**Environment variable substitution:** Use `${VAR_NAME}` in values.

**MCP permissions:**

```json
"amp.mcpPermissions": [...]
```

Allow/block MCP servers. [Official]

## Agents

**Not supported as user-definable content files.** Amp has built-in agent types (Oracle, Librarian, Code Review, Course Correct) but no documented file format for user-defined agent definitions. [Official — not mentioned in manual]

## Hooks

**Not supported** as an event-based system. See `hooks.md` for details on the Toolbox system, which is the closest equivalent. [Official]

## Commands

**Not supported.** Skills replaced custom commands. [Official]

## Settings File

**Path:** `~/.config/amp/settings.json` (macOS/Linux), `%USERPROFILE%\.config\amp\settings.json` (Windows)

**All settings use `amp.` prefix.** Key settings:

| Setting | Type | Purpose |
|---------|------|---------|
| `amp.mcpServers` | object | MCP server definitions |
| `amp.permissions` | array | Tool use permission rules |
| `amp.skills.path` | string | Additional skill directories |
| `amp.tools.disable` | array | Disable tools by name/glob |
| `amp.tools.stopTimeout` | number | Seconds before canceling tool (default: 300) |
| `amp.defaultVisibility` | object | Per-repo thread visibility |
| `amp.mcpPermissions` | array | Allow/block MCP servers |
| `amp.anthropic.thinking.enabled` | boolean | Extended thinking (default: true) |
| `amp.showCosts` | boolean | Display thread costs (default: true) |
| `amp.agent.deepReasoningEffort` | string | GPT-5.3 effort: medium/high/xhigh |
| `amp.updates.mode` | string | Update checking behavior |
| `amp.terminal.theme` | string | CLI theme name |

**Enterprise managed settings:** Deploy to `/etc/ampcode/managed-settings.json` (Linux), `/Library/Application Support/ampcode/managed-settings.json` (macOS). [Official]

## Custom Checks

Code review checks in `.agents/checks/` with YAML frontmatter:
- `name` — check name
- `description` — what the check reviews
- `severity-default` — default severity level
- `tools` — permitted tools for the check

[Official]

## Symlink Support

| Content Type | Symlinks |
|-------------|----------|
| Rules | Yes |
| Skills | Yes |
| MCP | No (JSON merge) |

[Inferred from syllago provider definition]
