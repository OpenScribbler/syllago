# Skill Extraction Guide

When and how to extract content from agents into reusable skills.

---

## When to Extract

### Definitely Extract
- Domain knowledge (security checklists, language idioms, K8s patterns)
- Command documentation (wrapper scripts, CLI reference)
- Reference tables (error codes, API endpoints, config options)
- Content reusable by 2+ agents
- >50 lines of static content in agent prompt

### Consider Extracting
- 25-50 lines of content that's updated separately from agent behavior
- Detailed examples (keep 1-2 in agent, rest in skill)

### Keep in Agent
- Persona definition, workflow steps, confidence rules, boundaries, skill loading instructions. These are behavior, not knowledge.

---

## Extraction Process

### 1. Identify Candidates
Look for embedded checklists (30+ lines), command docs (15+ lines), pattern catalogs, and reference tables in agent prompts.

### 2. Design Structure
```
skills/<skill-name>/
  SKILL.md           # Entry point: quick reference + routing table (<100 lines)
  references/
    <topic>.md       # Deep content, loadable independently
```

Naming: `domain-focus` (e.g., `code-review-standards`, `go-patterns`).

### 3. Create SKILL.md
Entry point should: provide quick reference (satisfies 80% of lookups), include decision trees for common choices, link to references with "when to load" guidance.

```markdown
---
name: skill-name
description: What and when.
---
# Skill Title

## Quick Reference
[Table covering common needs]

## References
| Task | Load |
|------|------|
| Deep dive on X | [x.md](references/x.md) |
```

### 4. Create Reference Files
Each reference: self-contained (works without other files), grep-friendly headers, concrete examples.

### 5. Update Agent
Replace embedded content with skill reference:

```markdown
# BEFORE (in agent): 50 lines of security checks
# AFTER (in agent):
## Skills to Load
Load `skills/code-review-standards/SKILL.md` for review checklists.
```

---

## Design Principles

1. **Quick reference first**: SKILL.md satisfies most needs without loading references.
2. **Progressive detail**: SKILL.md (always) → references (on-demand).
3. **Grep-friendly headers**: Consistent, searchable section names.
4. **Self-contained references**: Each file works independently.

## Tool Documentation in Skills

| Tool Complexity | Where to Document |
|-----------------|-------------------|
| Simple (1-2 subcommands) | SKILL.md Commands section |
| Medium (3-5 subcommands) | SKILL.md with link to reference |
| Complex (>5 subcommands) | Separate reference file |

Include: one-line purpose, quick reference table, usage example, output format, when to use.

## Validation Checklist

- [ ] SKILL.md <100 lines
- [ ] Quick reference covers 80% of lookups
- [ ] Each reference independently useful
- [ ] Agent prompt shorter than before
- [ ] Agent includes skill loading instructions with when-to-load guidance
- [ ] No duplicate content between agent and skill
