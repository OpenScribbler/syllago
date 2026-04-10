# Windsurf — Format Reference

Provider slug: `windsurf`

Supports: Rules

Sources: [Windsurf docs](https://docs.windsurf.com) (limited), community repos

---

## Rules

**Location:** `.windsurf/rules/*.md` (directory-based) or `.windsurfrules` (single-file legacy)

**Format:** YAML frontmatter + markdown body

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `trigger` | string | No | Activation mode (see below) |
| `description` | string | No | Rule purpose; used for `model_decision` activation |
| `globs` | string | No | Comma-separated file patterns (required when `trigger: glob`) |

Note: `globs` is a single comma-separated string, not an array.

**Trigger types:**

| Trigger | Behavior |
|---------|----------|
| `always_on` | Rule applies to every session |
| `model_decision` | Agent evaluates `description` for relevance |
| `glob` | Applies when `globs` patterns match active files |
| `manual` | User invokes via mention in chat |

**Default behavior:** If no frontmatter is present, rule is treated as `always_on`.

**Example:**

```markdown
---
trigger: glob
description: TypeScript API layer patterns
globs: "src/api/**/*.ts, src/services/**/*.ts"
---

# API Conventions

1. Validate all inputs at layer boundary
2. Use consistent error handling with custom error types
3. Return typed responses for all endpoints
```

---

## Unsupported Content Types

Windsurf does not support Commands, Skills, Agents, Hooks, or MCP configuration through file-based formats. These are managed through the Windsurf IDE UI.
