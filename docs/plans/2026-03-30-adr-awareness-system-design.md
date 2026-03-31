# ADR Awareness System - Design Document

**Goal:** Ensure AI coding agents consistently create and reference Architecture Decision Records, preventing accidental reversals of important decisions.

**Decision Date:** 2026-03-30

---

## Problem Statement

Holden accidentally reverted a safety-critical decision (hook degradation enforcement) because the rationale wasn't surfaced during the AI session. The decision existed as an ADR, but nothing connected "you're editing converter files" to "ADR-0001 governs this area." The agent had no mechanism to discover or be reminded of architectural decisions during its work.

More broadly: decisions made in one session are invisible to future sessions unless they're encoded in CLAUDE.md rules. But CLAUDE.md rules capture the *what* (directive) without the *why* (rationale, alternatives, consequences). When an agent sees only the rule, it may judge the rule as outdated or unnecessary and override it — exactly what happened.

## Proposed Solution

A three-component system that lives in the global PAI infrastructure and auto-activates for any project with a `docs/adr/` directory:

1. **ADR Skill** (`/adr`) — Global skill for creating, consulting, and reviewing ADRs
2. **Awareness Hook** — PostToolUse hook that surfaces relevant ADRs when editing scoped files
3. **Commit Gate Hook** — PreToolUse hook that warns or blocks commits to ADR-scoped files (tiered enforcement)

## Architecture

### Component Locations

| Component | Location | Scope |
|-----------|----------|-------|
| ADR Skill | `~/.config/pai/skills/adr/` (symlinked to `~/.claude/skills/adr`) | Global — works in any project |
| Awareness Hook | `~/.config/pai/hooks/adr-awareness.sh` | Global — auto-detects `docs/adr/INDEX.md` |
| Commit Gate Hook | `~/.config/pai/hooks/adr-commit-gate.sh` | Global — auto-detects `docs/adr/INDEX.md` |
| Hook Registration | `~/.claude/settings.json` | Global hooks config |
| ADR Content | `docs/adr/` per project | Project-local content only |

### Auto-Detection Pattern

Both hooks check for `docs/adr/INDEX.md` in the current git repo root as their first action. If the file doesn't exist, they exit 0 silently. This means:
- **Non-ADR projects**: zero overhead, hooks are invisible
- **ADR projects**: no per-project hook configuration needed — just create the content

### Data Model: ADR Frontmatter

Every ADR uses YAML frontmatter that drives automation:

```yaml
---
id: "0001"
title: Hook Degradation Enforcement
status: accepted          # proposed | accepted | superseded
date: 2026-03-28
enforcement: strict       # strict = blocks commits, advisory = warns only
files: ["cli/internal/converter/*"]
tags: [hooks, conversion, safety, degradation]
---
```

| Field | Purpose | Used by |
|-------|---------|---------|
| `id` | Sequential number | Skill (auto-increment on create) |
| `title` | Human-readable name | Index, hook messages |
| `status` | Lifecycle state | Hooks (only `accepted` triggers), index display |
| `date` | When decided | Index, staleness review |
| `enforcement` | `strict` or `advisory` | Commit gate (strict=block, advisory=warn) |
| `files` | Glob patterns for scoped files | Both hooks (path matching) |
| `tags` | Topical keywords | Skill consult workflow (broader matching) |

### ADR Index (`docs/adr/INDEX.md`)

The lightweight, always-scannable index. Referenced from CLAUDE.md so agents load it at session start.

```markdown
# ADR Index

Architectural decisions for this project. Before modifying files in a listed scope, read the full ADR.

| ADR | Title | Status | Enforcement | Scope | Summary |
|-----|-------|--------|-------------|-------|---------|
| [0001](0001-hook-degradation-enforcement.md) | Hook Degradation Enforcement | accepted | strict | `cli/internal/converter/*` | block/warn/exclude strategies must be enforced during conversion, not silently dropped |
```

**Design decisions:**
- Single table with status column (not separate sections per status) — works well up to ~30 ADRs
- Scope in backticks — what the hooks parse
- Enforcement column — visible at a glance which ADRs block vs warn
- Relative links — index lives alongside ADR files
- Superseded ADRs get ~~strikethrough~~ on title

### CLAUDE.md Section

Added to each project's CLAUDE.md (after Key Conventions, before Testing Requirements):

```markdown
## Architectural Decisions

Active ADRs are indexed in `docs/adr/INDEX.md`. Before modifying files listed in an ADR's scope, read the full ADR to understand the rationale. Hooks will remind you when you touch scoped files — `strict` ADRs block commits, `advisory` ADRs warn.
```

Three sentences. The index does the heavy lifting.

## Component Details

### 1. ADR Skill (`~/.config/pai/skills/adr/`)

```
adr/
├── SKILL.md                    # Routing + conventions
├── workflows/
│   ├── create.md               # /adr create "title"
│   ├── consult.md              # /adr (no args)
│   └── review.md               # /adr review
└── references/
    └── template.md             # ADR template with frontmatter
```

#### Workflow: create

Trigger: `/adr create "Decision Title"`

1. Find `docs/adr/` in current repo (convention-based, error if not found)
2. Read INDEX.md to determine next sequential number
3. Scaffold new file from template: `docs/adr/NNNN-slugified-title.md`
4. Pre-fill frontmatter (id, title, date=today, status=proposed, enforcement=advisory)
5. Ask user for: scope (files patterns), tags, enforcement level
6. Write the ADR file
7. Add row to INDEX.md
8. Tell agent to fill in Context, Decision, and Consequences sections

#### Workflow: consult

Trigger: `/adr` or `/adr consult`

1. Read INDEX.md
2. Identify ADRs relevant to current work context (file being discussed, topic area)
3. Read full text of matching ADRs
4. Summarize key constraints and decisions the agent should be aware of

#### Workflow: review

Trigger: `/adr review`

1. Scan all ADR files in `docs/adr/`
2. Check for staleness:
   - Superseded ADRs still referenced in CLAUDE.md
   - Scope patterns that don't match any existing files (dead scopes)
   - ADRs older than 6 months with `proposed` status (stale proposals)
   - Accepted ADRs with low confidence (if we add that field later)
3. Report findings and suggest actions

### 2. Awareness Hook (`adr-awareness.sh`)

**Type:** PostToolUse on Edit|Write
**Behavior:** Non-blocking (always exit 0)

Logic:
1. Parse `file_path` from JSON stdin (python3 one-liner for JSON, consistent with existing hooks)
2. `git rev-parse --show-toplevel` → repo root
3. Check `$REPO_ROOT/docs/adr/INDEX.md` exists — if not, exit 0
4. Normalize file_path to repo-relative (strip repo root prefix if absolute)
5. Scan INDEX.md rows: skip non-data rows, skip non-`accepted` status
6. Extract scope patterns from Scope column (inside backticks)
7. Match file path against patterns (bash prefix matching for `dir/*`, recursive for `dir/**`)
8. On first match: print `"Note: This file is covered by ADR-XXXX: [title]. Read docs/adr/XXXX-slug.md for the rationale behind decisions affecting this code."`

**Performance:** Reads one small file, does string matching in bash. Single-digit milliseconds even with 50 ADRs.

### 3. Commit Gate Hook (`adr-commit-gate.sh`)

**Type:** PreToolUse on Bash
**Behavior:** Tiered — strict ADRs block (exit 2), advisory ADRs warn (exit 0 with message)

Logic:
1. Parse command from JSON stdin
2. Check if command matches `git commit` — if not, exit 0
3. Check `$REPO_ROOT/docs/adr/INDEX.md` exists — if not, exit 0
4. Run `git diff --cached --name-only` to get staged files
5. For each staged file, check against ADR scopes in INDEX.md
6. Collect matches, grouped by enforcement level
7. If any `strict` matches: exit 2 with message listing the ADRs and saying "Review these ADRs before committing. If your changes align with the decisions, re-run the commit."
8. If only `advisory` matches: exit 0 with warning listing the ADRs

**Why exit 2 for strict:** Claude Code interprets exit 2 from PreToolUse as "block this action." The agent sees the message and must explicitly decide to proceed (by re-attempting after reading the ADR). This is the same pattern as the existing `release-guard.py` hook.

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Skill location | Global (`~/.config/pai/skills/adr/`) | ADR management is a workflow, not project-specific code |
| Hook location | Global (`~/.config/pai/hooks/`) | Auto-detect pattern means zero per-project config |
| Auto-detection | Check for `docs/adr/INDEX.md` | Convention-based, no config files needed |
| Frontmatter format | YAML with `files:` and `enforcement:` | Archgate-inspired but lighter; enables automation |
| Hook parses INDEX.md | Not individual ADR frontmatter | One file read per invocation, fast |
| Enforcement tiers | `strict` (blocks) vs `advisory` (warns) | Safety-critical decisions get stronger enforcement |
| Default enforcement | `advisory` | Low friction by default; `strict` is opt-in for safety-critical ADRs |
| ADR status in frontmatter AND body | Intentional redundancy | Frontmatter drives automation, body drives readability |
| Hook language | Bash + python3 for JSON | Consistent with all existing syllago and PAI hooks |

## Data Flow

```
Agent edits cli/internal/converter/adapter.go
    → PostToolUse fires adr-awareness.sh
    → Hook reads docs/adr/INDEX.md
    → Matches cli/internal/converter/* → ADR-0001
    → Prints: "Note: This file is covered by ADR-0001..."
    → Agent reads ADR-0001 before proceeding

Agent runs git commit
    → PreToolUse fires adr-commit-gate.sh
    → Hook reads staged files via git diff --cached
    → Matches staged files against ADR scopes
    → ADR-0001 is strict → exit 2, blocks commit
    → Agent reads ADR, verifies alignment, re-commits
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No `docs/adr/INDEX.md` in repo | Hooks exit 0 silently — project doesn't use ADRs |
| INDEX.md has malformed rows | Hook skips unparseable rows, continues — advisory failures are silent |
| No matching ADR for edited file | No output — hook is invisible |
| Multiple ADRs match same file | Awareness hook shows first match; commit gate shows all matches |
| Python3 not available | JSON parse fails, hook exits 0 — graceful degradation |
| `git rev-parse` fails (not in repo) | Hook exits 0 — not in a git repo |

## Success Criteria

1. **Editing a scoped file shows the ADR advisory** — the agent sees the message and can read the full ADR
2. **Committing to strict-scoped files blocks** until the agent acknowledges the ADR
3. **Non-ADR projects see zero overhead** — hooks exit silently
4. **`/adr create` produces a correctly formatted ADR** with frontmatter and INDEX.md entry
5. **`/adr review` identifies stale ADRs** with actionable suggestions

## Open Questions

1. **Should we add a `confidence:` field to ADR frontmatter?** Microsoft recommends recording confidence level. Low-confidence decisions flagged for reconsideration. Deferred — can add later without breaking anything.
2. **Should INDEX.md be auto-generated?** Currently manual. A pre-commit git hook could regenerate it from ADR frontmatter. Deferred — manual is fine with <20 ADRs.
3. **Should the commit gate check require explicit acknowledgment?** Currently it blocks (exit 2) and the agent re-attempts after reading. A more sophisticated system could track "agent has read ADR-XXXX in this session." Deferred — the simple block-and-retry pattern works.

---

## Implementation Scope (Syllago First)

For the first project to use this system:

### Global Infrastructure (new)
- `~/.config/pai/skills/adr/SKILL.md` + workflows + references
- `~/.config/pai/hooks/adr-awareness.sh`
- `~/.config/pai/hooks/adr-commit-gate.sh`
- `~/.claude/settings.json` — register both hooks
- `~/.claude/skills/adr` — symlink to PAI skill

### Syllago Project Content (modify/new)
- `docs/adr/0001-hook-degradation-enforcement.md` — add YAML frontmatter
- `docs/adr/INDEX.md` — create with first entry
- `CLAUDE.md` — add Architectural Decisions section

---

## Next Steps

Ready for implementation planning with the `Plan` skill.
