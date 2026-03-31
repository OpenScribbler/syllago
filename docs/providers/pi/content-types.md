# Pi Content Types Reference

> **Identity note:** "Pi" refers to Mario Zechner's `pi` coding agent
> (`github.com/badlogic/pi-mono`). Not to be confused with Raspberry Pi hardware
> or other "Pi" named projects.

Comprehensive documentation of all content types supported by Pi, including file
formats, directory structures, configuration, and loading behavior.

**Last updated:** 2026-03-30

**Sources:**
- [Official] https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md
- [Official] https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/settings.md
- [Official] https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md
- [Official] https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/prompt-templates.md

---

## Table of Contents

1. [Extensions (.pi/extensions/)](#1-extensions)
2. [Settings (.pi/settings.json)](#2-settings)
3. [Skills (.pi/skills/)](#3-skills)
4. [Prompt Templates (.pi/prompts/)](#4-prompt-templates)
5. [Themes](#5-themes)
6. [Packages](#6-packages)
7. [Content Types NOT Supported](#7-content-types-not-supported)

---

## 1. Extensions

Extensions are TypeScript modules that hook into Pi's lifecycle, register custom
tools, add commands, and modify agent behavior. They are Pi's equivalent of hooks
in other providers, but far more powerful -- full programmatic control rather
than declarative config.

See [hooks.md](hooks.md) for complete event documentation.

### File Locations

| Scope | Path |
|---|---|
| Project | `.pi/extensions/*.ts` |
| Global | `~/.pi/agent/extensions/*.ts` |
| Subdirectory (with entry) | `.pi/extensions/<name>/index.ts` |
| Subdirectory (with manifest) | `.pi/extensions/<name>/package.json` with `pi` field |
| Settings | `extensions` array in `settings.json` |
| CLI | `-e, --extension <path>` |

### File Format

TypeScript (`.ts`) or JavaScript (`.js`). Must export a default function:

```typescript
import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  // Register event handlers, tools, commands, etc.
}
```

Extensions with npm dependencies use a subdirectory with `package.json`. Pi
pre-bundles `@sinclair/typebox`, `@mariozechner/pi-agent-core`,
`@mariozechner/pi-tui`, and `@mariozechner/pi-ai` -- these do not need separate
installation.

### Capabilities

Extensions can:
- Subscribe to 25+ lifecycle events
- Register custom tools (with TypeBox schemas and TUI rendering)
- Register slash commands and keyboard shortcuts
- Register CLI flags
- Register custom message renderers
- Register and unregister LLM providers
- Control the TUI (widgets, overlays, header, footer, status line, theme)
- Send messages, fork sessions, navigate branches

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md]

---

## 2. Settings

Settings control Pi's behavior: model selection, UI preferences, compaction
behavior, and resource paths.

### File Locations

| Scope | Path | Priority |
|---|---|---|
| Global | `~/.pi/agent/settings.json` | Lower |
| Project | `.pi/settings.json` | Higher (overrides global) |

Project settings override global settings. Nested objects are merged (not
replaced).

### File Format

JSON. No comments or trailing commas.

### Key Settings Categories

**Model & Thinking:**

| Setting | Type | Default | Description |
|---|---|---|---|
| `defaultProvider` | string | -- | Default LLM provider |
| `defaultModel` | string | -- | Default model ID |
| `defaultThinkingLevel` | string | -- | One of: off, minimal, low, medium, high, xhigh |
| `hideThinkingBlock` | boolean | false | Hide reasoning blocks in output |
| `thinkingBudgets` | object | -- | Per-level token budgets |
| `enabledModels` | string[] | -- | Models available for cycling |

**UI & Display:**

| Setting | Type | Default | Description |
|---|---|---|---|
| `theme` | string | "dark" | Visual theme name |
| `quietStartup` | boolean | false | Suppress startup banner |
| `doubleEscapeAction` | string | "tree" | What double-Esc does |
| `showHardwareCursor` | boolean | false | Show terminal cursor |

**Compaction:**

| Setting | Type | Default | Description |
|---|---|---|---|
| `compaction.enabled` | boolean | true | Enable auto-compaction |
| `compaction.reserveTokens` | number | 16384 | Tokens reserved for output |
| `compaction.keepRecentTokens` | number | 20000 | Recent tokens to preserve |

**Retry:**

| Setting | Type | Default | Description |
|---|---|---|---|
| `retry.enabled` | boolean | true | Auto-retry on failure |
| `retry.maxRetries` | number | 3 | Maximum retry attempts |
| `retry.baseDelayMs` | number | 2000 | Initial backoff delay |
| `retry.maxDelayMs` | number | 60000 | Maximum backoff delay |

**Resources:**

| Setting | Type | Default | Description |
|---|---|---|---|
| `packages` | array | [] | npm/git package references |
| `extensions` | string[] | [] | Additional extension paths |
| `skills` | string[] | [] | Additional skill paths |
| `prompts` | string[] | [] | Additional prompt template paths |
| `themes` | string[] | [] | Additional theme paths |
| `enableSkillCommands` | boolean | true | Expose skills as slash commands |

**Shell:**

| Setting | Type | Default | Description |
|---|---|---|---|
| `shellPath` | string | -- | Custom shell binary path |
| `shellCommandPrefix` | string | -- | Prefix for shell commands |
| `npmCommand` | string[] | -- | Custom npm command |

**Sessions:**

| Setting | Type | Default | Description |
|---|---|---|---|
| `sessionDir` | string | -- | Custom session storage directory |

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/settings.md]

---

## 3. Skills

Skills are reusable capability packages that agents load dynamically. Pi
implements the Agent Skills standard (SKILL.md format with YAML frontmatter).

See [skills-agents.md](skills-agents.md) for complete format documentation.

### File Locations

| Scope | Path |
|---|---|
| Project | `.pi/skills/<name>/SKILL.md` |
| Global | `~/.pi/agent/skills/<name>/SKILL.md` |
| Packages | `skills/` directories or `pi.skills` in `package.json` |
| Settings | `skills` array in `settings.json` |
| CLI | `--skill <path>` |

### Loading Behavior

- Only skill names and descriptions are kept in context initially (progressive
  disclosure)
- Full skill content is loaded when the agent determines it is relevant
- Missing descriptions cause a skill to not load at all
- Name collisions: first discovered skill wins, with a warning
- Discovery can be disabled with `--no-skills`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md]

---

## 4. Prompt Templates

Prompt templates are reusable markdown snippets invoked via slash commands in the
interactive TUI. They support positional argument substitution.

### File Locations

| Scope | Path |
|---|---|
| Project | `.pi/prompts/*.md` |
| Global | `~/.pi/agent/prompts/*.md` |
| Packages | `prompts/` directories or `pi.prompts` in `package.json` |
| Settings | `prompts` array in `settings.json` |
| CLI | `--prompt-template <path>` |

### File Format

Markdown with optional YAML frontmatter. The filename (minus `.md`) becomes the
slash command name.

```markdown
---
description: Review staged git changes
---
Review the staged changes and provide feedback on:
- Code quality
- Potential bugs
- Style consistency

Focus on: $1
```

### Frontmatter Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `description` | string | No | Shown in autocomplete; first non-empty line used if omitted |

### Argument Substitution

| Syntax | Meaning |
|---|---|
| `$1`, `$2`, ... | Positional arguments |
| `$@` or `$ARGUMENTS` | All arguments joined |
| `${@:N}` | Arguments from position N onward |
| `${@:N:L}` | L arguments starting at position N |

### Invocation

Type `/` followed by the template name in the interactive editor:
- `/review` invokes `review.md`
- `/component Button` passes "Button" as `$1`

Discovery is non-recursive. Subdirectory templates require explicit settings
configuration.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/prompt-templates.md]

---

## 5. Themes

Themes customize Pi's visual appearance in the TUI. They are loaded from:

| Scope | Path |
|---|---|
| Global | `~/.pi/agent/themes/` |
| Settings | `themes` array in `settings.json` |

The active theme is set via the `theme` setting (default: `"dark"`). Extensions
can also switch themes programmatically via `ctx.ui.setTheme()`.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/settings.md]

---

## 6. Packages

Pi supports distributing bundles of extensions, skills, prompts, and themes via
npm or git packages. Packages are declared in the `packages` array in
`settings.json`.

A package's `package.json` can declare a `pi` field with entry points for each
resource type. [Unverified -- exact manifest format not confirmed from source]

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/packages.md]

---

## 7. Content Types NOT Supported

Pi deliberately omits several content types that other providers include as
built-in features. The design philosophy is that users build these via extensions
rather than having them imposed:

| Content Type | Pi Equivalent |
|---|---|
| MCP servers | Build via extensions (custom provider/tool registration) |
| Sub-agents | Build via extensions (see `examples/extensions/subagent/`) |
| Permission popups | Build via extensions (see `examples/extensions/permission-gate.ts`) |
| Plan mode | Build via extensions (see `examples/extensions/plan-mode/`) |
| Rules/instructions files | Use skills or AGENTS.md (cross-provider standard) |
| Hooks (declarative) | Use extensions (programmatic TypeScript) |

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/README.md]

---

## Comparison with Other Providers

| Content Type | Pi | Claude Code | Cursor |
|---|---|---|---|
| Instructions/Rules | Skills, AGENTS.md | CLAUDE.md, .claude/rules/ | .cursor/rules/*.mdc |
| Hooks/Extensions | `.pi/extensions/*.ts` (programmatic) | settings.json hooks (declarative) | hooks.json (declarative) |
| Settings | `.pi/settings.json` | `.claude/settings.json` | VS Code settings |
| Skills | `.pi/skills/SKILL.md` | `.claude/skills/SKILL.md` | `.cursor/skills/SKILL.md` |
| Commands | Extensions + prompt templates | `.claude/commands/*.md` | `.cursor/commands/*.md` |
| MCP | Not built-in (via extensions) | settings.json mcpServers | .cursor/mcp.json |
| Packages | npm/git bundles | Not supported | Not supported |
