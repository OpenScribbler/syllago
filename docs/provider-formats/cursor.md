# Cursor — Format Reference

Provider slug: `cursor`

Supports: Rules

Sources: [Cursor docs](https://cursor.com/docs/context/rules)

---

## Rules

**Location:** `.cursor/rules/*.md` or `.cursor/rules/*.mdc` (supports subdirectories)

**Format:** MDC (Markdown with metadata control) — YAML-like frontmatter + markdown body

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | Conditional | Rule purpose; required for "Apply Intelligently" type |
| `alwaysApply` | bool | No | `true` = always active (default: `false`) |
| `globs` | string[] | No | File patterns for scoped activation |

**Activation model:**

| Mode | `alwaysApply` | `globs` | `description` | Behavior |
|------|---------------|---------|---------------|----------|
| Always Apply | `true` | — | optional | Active in every chat session |
| Apply Intelligently | `false` | — | required | Agent evaluates description for relevance |
| File-Scoped | `false` | required | optional | Triggered when file patterns match |
| Manual | `false` | — | — | User invokes via `@rule-name` in chat |

**Precedence:** Team Rules > Project Rules > User Rules

**Alternative:** `AGENTS.md` in project root — plain markdown, no frontmatter, simpler but less capable.

**Example:**

```markdown
---
description: React component patterns and TypeScript conventions
alwaysApply: false
globs:
  - "src/components/**/*.tsx"
  - "src/components/**/*.ts"
---

# React Components

1. Use functional components with hooks
2. Type props explicitly with TypeScript interfaces
3. Export components as named exports
4. Place tests in adjacent `__tests__/` directory
```

---

## Unsupported Content Types

Cursor does not support Commands, Skills, Agents, Hooks, or MCP configuration through file-based formats. These are managed through the Cursor IDE UI.
