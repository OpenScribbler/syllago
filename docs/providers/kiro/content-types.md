# Kiro: Content Types

> Research date: 2026-03-21
> Sources tagged: `[Official]` = kiro.dev docs, `[Syllago]` = existing codebase mappings, `[Community]` = third-party, `[Inferred]` = derived

## Overview

Kiro organizes project content under the `.kiro/` directory. Content types include steering files, specs, hooks, agents (subagents), MCP configurations, and Powers. Global content lives under `~/.kiro/`.

## 1. Steering Files (Rules equivalent)

[Official: Steering docs](https://kiro.dev/docs/steering/)

### Purpose
Persistent project-specific knowledge in markdown files. Guides AI behavior according to project conventions, architecture, and business context. Analogous to Claude Code's rules or Cursor's rules.

### Directory Structure

```
.kiro/steering/           # Workspace scope (per-project)
~/.kiro/steering/         # Global scope (all projects)
```

Workspace steering overrides global when conflicting.

### File Format

Markdown with optional YAML frontmatter. Frontmatter must appear at the very beginning with no preceding content or blank lines.

### Inclusion Modes

| Mode | Frontmatter | Behavior |
|------|-------------|----------|
| **Always** (default) | `inclusion: always` | Loaded in every interaction |
| **File Match** | `inclusion: fileMatch` + `fileMatchPattern: "*.tsx"` | Included when working with matching files |
| **Manual** | `inclusion: manual` | On-demand via `#steering-file-name` in chat |
| **Auto** | `inclusion: auto` + `name` + `description` | Included when request matches description |

### Frontmatter Fields

```yaml
---
inclusion: fileMatch
fileMatchPattern: "components/**/*.tsx"
---
```

```yaml
---
inclusion: auto
name: api-design
description: REST API design patterns and conventions
---
```

`fileMatchPattern` supports single patterns (`"*.tsx"`) or arrays (`["app/api/**/*", "**/*.test.*"]`).

### Foundation Files

Kiro generates three core steering files by default:

| File | Purpose |
|------|---------|
| `product.md` | Product purpose, users, features, business objectives |
| `tech.md` | Frameworks, libraries, tools, constraints |
| `structure.md` | File organization, naming conventions, architecture |

These are always included in every interaction.

### File References

Steering files can reference live project files:
```markdown
#[[file:path/to/file.ext]]
```

### AGENTS.md Support

Kiro supports the `AGENTS.md` standard [Official](https://kiro.dev/docs/steering/). These files work like steering but don't support inclusion modes and are always included. Can be placed in `~/.kiro/steering/` or workspace root.

### Syllago Mapping [Syllago]

Syllago maps `catalog.Rules` to Kiro steering files and `catalog.Skills` also maps to steering (with `auto` inclusion mode). Both emit to `.kiro/steering/`.

## 2. Specs

[Official: Specs Best Practices](https://kiro.dev/docs/specs/best-practices/)

### Purpose
Spec-driven development system. Transforms high-level feature ideas into detailed implementation plans. Unique to Kiro — no direct equivalent in other providers.

### Directory Structure

```
.kiro/specs/
├── feature-name/
│   ├── requirements.md    # User stories + acceptance criteria (EARS notation)
│   ├── design.md          # Technical architecture + implementation approach
│   └── tasks.md           # Actionable development work items
```

### Workflow Approaches

| Approach | When to Use |
|----------|-------------|
| **Requirements-First** | System behavior known, architecture flexible |
| **Design-First** | Existing architecture or strict non-functional requirements |

Cannot switch workflow after creation.

### Spec Files

- **requirements.md** — Uses EARS (Easy Approach to Requirements Syntax) notation for acceptance criteria
- **design.md** — Technical architecture, system design, tech stack
- **tasks.md** — Checkable task list with implementation steps

All spec files are automatically included in conversation context.

### Syllago Mapping [Syllago]

Specs have no syllago content type mapping — Kiro-exclusive feature.

## 3. Hooks

[Official: Hooks docs](https://kiro.dev/docs/hooks/)

### Purpose
Event-driven automations that execute agent prompts or shell commands in response to IDE/CLI events.

### Directory Structure

```
.kiro/hooks/               # Workspace hooks
```

### Configuration

Hooks are created via the Kiro UI (natural language or form) in the IDE. In the CLI, hooks are defined within agent configuration JSON files.

See [hooks.md](./hooks.md) for complete hook documentation.

### Syllago Mapping [Syllago]

Hooks use JSON merge strategy. Discovery path is `.kiro/agents` (hooks embedded in agent JSON). Rendered as `syllago-hooks.json` agent files.

## 4. Agents (Custom Subagents)

[Official: Subagents](https://kiro.dev/docs/chat/subagents/)

### Purpose
Specialized agents with custom system prompts, tool access, and model selection. Can be invoked automatically or manually.

### Directory Structure

```
.kiro/agents/              # Workspace scope
~/.kiro/agents/            # Global scope
```

### File Format

Markdown with YAML frontmatter:

```yaml
---
name: code-reviewer
description: Expert code review assistant
tools: ["read", "@context7"]
model: claude-sonnet-4
includeMcpJson: true
includePowers: false
---

System prompt content goes here in the markdown body.
```

See [skills-agents.md](./skills-agents.md) for complete agent documentation.

### Syllago Mapping [Syllago]

`catalog.Agents` maps to `.kiro/agents/` with JSON format.

## 5. MCP Configuration

[Official: MCP Configuration](https://kiro.dev/docs/mcp/configuration/)

### Purpose
Model Context Protocol server definitions for extending Kiro with external tools.

### File Locations

```
.kiro/settings/mcp.json    # Workspace scope
~/.kiro/settings/mcp.json  # Global scope
```

### Format

```json
{
  "mcpServers": {
    "server-name": {
      "command": "npx",
      "args": ["-y", "@server/package"],
      "env": {
        "API_KEY": "${API_KEY}"
      },
      "disabled": false,
      "disabledTools": ["tool_name"]
    }
  }
}
```

Remote servers use `url` + `headers` instead of `command` + `args`.

### Priority Hierarchy

1. Agent config `mcpServers` field (highest)
2. Workspace `.kiro/settings/mcp.json`
3. Global `~/.kiro/settings/mcp.json` (lowest)

Same-name servers: highest priority wins (complete override). Different names: additive merge.

### Enterprise MCP Registry

Organizations can define an MCP registry for governance. Kiro fetches registry at startup and every 24 hours. Servers removed from registry are terminated automatically [Official](https://kiro.dev/docs/enterprise/governance/mcp/).

### Syllago Mapping [Syllago]

`catalog.MCP` uses JSON merge strategy. Discovery path: `.kiro/settings/mcp.json`.

## 6. Powers

[Official](https://kiro.dev/blog/introducing-kiro/)

### Purpose
Pre-packaged bundles of MCP servers, steering files, and hooks from Kiro partners. Installed in one click.

### Components

A Power can include:
- MCP servers (specialized tool access)
- Steering files (best practices)
- Hooks (automated actions)

Examples: AWS Observability Power (Feb 2026) [Official](https://aws.amazon.com/about-aws/whats-new/2026/02/aws-observability-kiro-power/).

### Syllago Mapping [Syllago]

No direct syllago content type. Powers are composite — their components (MCP, steering, hooks) map to existing types.

## 7. .kiroignore

[Official](https://kiro.dev/changelog/ide/0-8/)

Excludes files from agent access. Works like `.gitignore` syntax. Placed in workspace root.

## Content Type Summary

| Content Type | Directory | Format | Scope | Syllago Type |
|-------------|-----------|--------|-------|--------------|
| Steering | `.kiro/steering/` | Markdown + YAML frontmatter | Workspace + Global | Rules, Skills |
| Specs | `.kiro/specs/` | Markdown | Workspace only | — (Kiro-exclusive) |
| Hooks | `.kiro/hooks/` (IDE), agent JSON (CLI) | JSON / UI form | Workspace | Hooks |
| Agents | `.kiro/agents/` | Markdown + YAML frontmatter | Workspace + Global | Agents |
| MCP | `.kiro/settings/mcp.json` | JSON | Workspace + Global | MCP |
| Powers | Managed by Kiro | Composite | — | — |
| .kiroignore | Workspace root | Gitignore syntax | Workspace | — |
