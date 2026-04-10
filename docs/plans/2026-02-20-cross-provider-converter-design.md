# Cross-Provider Content Conversion Pipeline — Design

## Problem Statement

Syllago stores AI coding tool content in a repo and installs to provider-specific locations. Currently, installation just copies or symlinks files — no format transformation happens. When a Claude Code rule needs to go to Cursor (.mdc format) or a Claude agent needs to go to Codex (flat AGENTS.md), the content is copied as-is, producing broken or incompatible output.

## Solution: Hub-and-Spoke Converter Architecture

A **converter package** with a hub-and-spoke architecture: content is normalized to a canonical format on import (with the original preserved in `.source/`), and rendered to provider-specific formats on export.

### Source-Preserving Canonical Model

```
External source (Cursor .mdc)
    ↓ syllago add --type rules --provider cursor
    ↓   1. Copy original → .source/rule.mdc (verbatim preservation)
    ↓   2. Parse source → RuleMeta → canonical rule.md (with frontmatter)
    ↓   3. Record source_provider in .syllago.yaml
rules/cursor/my-rule/
    rule.md              ← canonical format (YAML frontmatter + markdown body)
    .source/rule.mdc     ← original file (preserved verbatim)
    .syllago.yaml          ← metadata (source_provider: cursor, source_format: mdc)

    ↓ syllago export --to windsurf
    ↓   Reads canonical rule.md → renders with Windsurf trigger: field
~/.codeium/windsurf/rules/my-rule/rule.md

    ↓ syllago export --to cursor
    ↓   Detects target == source_provider → copies .source/rule.mdc verbatim (lossless!)
~/.cursor/rules/my-rule/rule.mdc
```

### Why This Approach

- **No data loss ever** — original preserved in `.source/`
- **One parse format for export** — all renderers read from canonical, never from arbitrary provider formats
- **Lossless roundtrip to source provider** — use `.source/` original directly
- **Adding new providers** — only need a renderer, not a parser (parsers only needed for import)
- **Auditability** — diff canonical vs original to see what was normalized

### Canonical Rule Format

`rule.md` with YAML frontmatter containing the universal rule activation fields:

```yaml
---
description: "Rule description"
alwaysApply: true
globs:
  - "*.ts"
  - "*.tsx"
---

# Rule Title

Rule content in markdown...
```

This is the superset of all provider rule formats. The frontmatter fields map to every provider's activation semantics.

### Activation Semantics (Rosetta Stone)

| Semantic | Canonical | Cursor | Windsurf | Claude/Codex |
|----------|-----------|--------|----------|--------------|
| Always active | `alwaysApply: true` | `alwaysApply: true` | `trigger: always_on` | Included in single file |
| File-scoped | `globs: ["*.ts"]` | `globs: ["*.ts"]` | `trigger: glob` + `globs` | Excluded (no glob) |
| AI-decided | `alwaysApply: false` + `description` | same | `trigger: model_decision` | Excluded |
| Manual/user | `alwaysApply: false` (bare) | same | `trigger: manual` | Excluded |

### Data Loss Strategy

- Single-file providers (Claude, Codex, Gemini) can only express `alwaysApply: true` rules
- When a non-alwaysApply rule targets these providers, the converter returns `nil Content` + a warning
- Warnings are structured and printed to stderr (or included in JSON output)

## Scope

**Phase 1 (this design)**: Rules converter + infrastructure. Rules are the most complex and most valuable content type.

**Future phases**: Skills (trivial — same format), commands, agents, MCP env vars, hooks.

## Key Design Decisions

1. **Two-method interface** — `Canonicalize()` for import, `Render()` for export
2. **Source preservation** — `.source/` directory holds verbatim original
3. **Canonical = YAML frontmatter + markdown** — same structure as Cursor but `.md` extension
4. **nil Content = skip** — clean way to signal "can't express this in target format"
5. **Reuse existing parser** — export `ParseMDCFrontmatter` from `parse/cursor.go`
6. **Phase 1 is rules only** — focused scope, future phases are separate plans

## Research Basis

See `docs/cross-provider-conversion-reference.md` for comprehensive research from 6 open-source converter tools.
