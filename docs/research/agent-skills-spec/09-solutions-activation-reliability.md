# Solutions: Activation Reliability

## Solution 1: Directive Description Template (100% activation)

**Source:** Ivan Seleznov, 650-trial empirical study

**The winning template:**
```yaml
description: <Domain> expert. ALWAYS invoke this skill when the user asks about <trigger topics>. Do not <alternative action> directly -- use this skill first.
```

**Key insight:** Combination of **positive routing** ("ALWAYS invoke") plus **negative constraint** ("Do not X directly") blocks the escape path. Either alone is insufficient.

**Results:**

| Variant | Bare | +CLAUDE.md | +Hook | +Both |
|---------|------|------------|-------|-------|
| Passive | 87.5% | 81.5% | 37.0% | 100% |
| Expanded keywords | 85.2% | 81.5% | 100% | 100% |
| Directive | **100%** | 94.4% | **100%** | **100%** |

**Stats:** Directive vs Passive: OR = 20.6, p < 0.0001 (20x more likely to activate)

**Limitation:** Only Claude Opus 4.5 tested, only 3 skills, directive saturation risk untested.

## Solution 2: Structured Activation Field (RFC in agentskills#57)

**Proposed addition to SKILL.md frontmatter:**
```yaml
activation:
  triggers:
    keywords: ["library", "framework", "documentation"]
    file_patterns: ["package.json", "requirements.txt", "go.mod"]
    user_intent: ["research", "lookup", "find"]
  priority:
    level: high
    supersedes: ["WebSearch"]
  auto_load: true
```

**Counter-proposals in thread:**
- **venikman:** Separate Skill/Context/Activation; use `activation: auto | suggest | manual` modes
- **EricGT:** Activation should be a separate protocol, not embedded in skills
- **marswong:** Forced activation for predefined workflows via `<available_skills>` XML injection

**Maintainer position (@klazuka):** "I'm reluctant to introduce more structured activation. The hope is that natural language should suffice as LLMs get more intelligent."

## Solution 3: `files` / `paths` Glob Patterns (agentskills#64, shipping in Claude Code)

**Proposed in spec (agentskills#64):**
```yaml
files:
  include:
    - "*.sql"
    - "**/migrations/*.sql"
  exclude:
    - "*_test.sql"
```

**Already shipping in Claude Code** as `paths` field:
```yaml
---
name: sql-migrations
paths:
  - "**/migrations/*.sql"
  - "*.sql"
---
```

**Also in Cursor** as `globs` field in `.cursor/rules/*.mdc` files.

Provides deterministic activation without relying on LLM description matching.

## Solution 4: Cursor's Four Activation Modes

1. **alwaysApply: true** — injected into every chat (100% reliable)
2. **Apply Intelligently** (default) — LLM evaluates description (same unreliability as skills)
3. **globs** — deterministic file pattern matching
4. **Manual @-mention** — explicit user invocation

Precedence: Team Rules > Project Rules > User Rules.

## Solution 5: Forced Eval Hook (Scott Spence)

Three-step commitment mechanism:
1. EVALUATE: State YES/NO for each skill with reasoning
2. ACTIVATE: Use `Skill()` tool immediately
3. IMPLEMENT: Only proceed after activation

**Results across 50 prompts:**

| Hook Type | Success Rate |
|-----------|-------------|
| Forced Eval | 84% |
| LLM Eval | 80% |
| Simple Instruction | 20% |

## Solution 6: Agent RuleZ (Deterministic Policy Engine)

**Author:** Rick Hightower

Intercepts agent events and injects skill content via policy rules:
```yaml
- name: activate-react-component-skill
  matchers:
    tools: ["Edit", "Write"]
    extensions: [".tsx", ".jsx"]
    directories: ["src/components/**"]
  actions:
    inject: ".claude/skills/react-component/SKILL.md"
  governance:
    created_by: "react-skill@2.1.0"
```

**Key innovation:** PreCompact preservation — captures active skill state before context compaction and re-injects after, preventing "skill amnesia."

Three policy modes: audit, warn, enforce.

## Solution 7: The Layering Pattern

Use CLAUDE.md/AGENTS.md to explicitly direct the agent to use skills:
- "When working on X, use /skill-name"
- Works because CLAUDE.md is system-prompt level (higher priority)
- Fragile: treated as "suggestions, not constraints" per issue #7777

## Solution 8: Addressing Execution Failure (Marc Bara)

Distinct from activation failure. Skills load but steps get skipped.

**Fix:** Require visible output at each step.

**Before:** "Before delivering, verify that every milestone aligns with scope."
**After:** "Do not deliver the final charter until you have output a verification block listing each milestone and the in-scope deliverable it maps to."

## Effectiveness Ranking

| Mechanism | Activation Rate | Source |
|-----------|----------------|--------|
| Directive description (no hooks) | **100%** | Seleznov, 650 trials |
| CLAUDE.md + Hook + any description | **100%** | Seleznov, C4 condition |
| Forced eval hook | 84% | Spence, 50 prompts |
| LLM eval hook | 80% | Spence |
| Passive description, bare | 77-87% | Seleznov |
| Passive description + hook (!) | **37%** | Seleznov (hooks hurt!) |
| Simple instruction hook | 20% | Spence |
