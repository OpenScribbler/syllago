---
name: review-team
description: Multi-agent code review workflow. Spawns parallel reviewers, triages findings, presents remediation plan. Requires user approval at each phase.
invocation: /review-team-team [path]
---

# Multi-Agent Code Review Skill

This skill orchestrates a structured, user-supervised code review process using parallel specialist agents.

## Overview

The `/review-team` command runs a multi-phase code review:
1. **Parallel Review** — Spawn specialist agents for different quality dimensions
2. **Triage** — Collect findings, deduplicate, classify by severity
3. **Remediation Planning** — Present fix plan for user approval
4. **Implementation** — Delegate fixes to senior-engineer (if approved)

**Key Principle:** User remains in control. Each phase requires explicit approval before proceeding.

## Usage

```
/review-team [path]           # Full review (all phases, user approval gates)
/review-team --quick [path]   # Quick review (findings only, no remediation/implementation)
/review-team --security-only  # Focus on security dimension
```

### Modes

- **Full** (default) — All 4 phases with user approval gates. Use for pre-merge or pre-deploy reviews.
- **Quick** (`--quick`) — Phase 1 + Phase 2 only. Presents findings and stops. No remediation plan, no implementation. Use for fast feedback during development.

## Workflow

### Phase 1: Parallel Review

Spawn 3 Task agents in parallel to review from different perspectives:

| Agent | Focus | Skill to Load |
|-------|-------|---------------|
| **Language Patterns** | Idioms, best practices, error handling | `go-patterns`, `python-patterns`, etc. |
| **Security** | Credentials, injection, auth, data handling | `security-audit` |
| **Dead Code & Complexity** | Unused imports/functions, unnecessary abstractions, over-engineering, duplicated logic | `code-review-standards` |

Each agent outputs findings in this format:

```markdown
## [Agent Name] Findings

### Finding 1
- **File**: path/to/file.go:42
- **Severity**: Critical | High | Medium | Low
- **Issue**: Description of the problem
- **Impact**: What could go wrong
- **Suggested Fix**: How to address it
- **Confidence**: HIGH | MEDIUM | LOW
```

### Phase 2: Triage (User Checkpoint)

After all agents complete:

1. **Collect** all findings from the three agents
2. **Deduplicate** — Same issue found by multiple agents counts once
3. **Classify** by severity:
   - **Critical** — Must fix before merge/deploy
   - **High** — Should fix, significant risk
   - **Medium** — Consider fixing, minor risk
   - **Low** — Nice to have, style/preference
   - **False Positive** — Explain why not a real issue

4. **Present to user**:

```markdown
## Review Summary

**Files Reviewed**: X
**Total Findings**: Y (after deduplication)

### Critical (must fix)
| # | File:Line | Issue | Confidence |
|---|-----------|-------|------------|

### High (should fix)
[Same format]

### Medium (consider)
[Same format]

### False Positives
| # | File:Line | Why Not an Issue |

---
**Awaiting your decision.** Which findings should we address?
Options:
- "Fix all critical and high"
- "Fix critical only"
- "Fix findings 1, 3, 5"
- "Skip fixes, just wanted the review"
```

**⏸️ WAIT for user to approve which findings to address.**

**If `--quick` mode: STOP HERE.** Present findings and end. Do not proceed to Phase 3 or 4.

### Phase 3: Remediation Plan

For user-approved findings, create an implementation plan:

```markdown
## Remediation Plan

### Finding 1: [Title]
**Approach**: [How we'll fix it]
**Files to Modify**: [List]
**Risk**: [Could this break anything?]
**Tests**: [How we'll verify the fix]

### Finding 3: [Title]
[Same format]

---
**Awaiting approval to implement.** Proceed with this plan?
```

**⏸️ WAIT for user approval before implementing.**

### Phase 4: Implementation (If Approved)

Delegate implementation to `senior-engineer` agent:

1. Fix one finding at a time
2. Run tests after each fix using wrappers (`go-dev test`, `py-dev test`)
3. If tests fail, report immediately — do not proceed to next fix
4. After all fixes, run full test suite
5. Present summary of changes made

```markdown
## Implementation Complete

### Changes Made
| Finding | File | Change | Tests |
|---------|------|--------|-------|
| #1 | foo.go | Added input validation | ✅ Pass |
| #3 | bar.go | Fixed SQL injection | ✅ Pass |

### Full Test Suite: ✅ All passing

### Files Modified
- foo.go
- bar.go
```

## Constraints

### Mandatory Rules

1. **NEVER implement fixes without explicit user approval** — present findings first
2. **ALWAYS use wrapper scripts** — `go-dev`, `py-dev`, `sec-scan`, not raw commands
3. **STOP on test failures** — do not continue to next fix if current fix breaks tests
4. **Report regressions immediately** — if a fix breaks something, tell the user

### Agent Delegation

When spawning Task agents for Phase 1:

```
Task: senior-code-reviewer for language patterns review
Task: senior-security-engineer for security review
Task: senior-code-reviewer for dead code & complexity review
```

When implementing fixes in Phase 4:

```
Task: senior-engineer for implementing approved fixes
```

### Confidence Levels

All findings include confidence:

| Level | Meaning | User Action |
|-------|---------|-------------|
| **HIGH** | Clear issue, well-understood | Include in fix plan |
| **MEDIUM** | Likely issue, some uncertainty | Discuss before fixing |
| **LOW** | Possible issue, needs investigation | Investigate before deciding |

## Quick Reference

### Quick Review

```
User: /review-team --quick src/

Claude:
1. Spawns 3 parallel Task agents
2. Waits for all to complete
3. Presents deduplicated, classified findings
4. Done — no remediation or implementation
```

### Full Review

```
User: /review-team src/

Claude:
1. Spawns 3 parallel Task agents
2. Waits for all to complete
3. Presents deduplicated, classified findings
4. Asks which to fix

User: Fix critical and high
Claude: [Presents remediation plan]

User: Looks good, proceed
Claude: [Implements fixes, runs tests, reports results]
```

### Aborting

At any checkpoint, user can say:
- "Stop here, I just wanted the review"
- "Skip the rest"
- "I'll fix these myself"

## Related Skills

- `code-review-standards` — Detailed checklists and patterns
- `security-audit` — Deep security analysis
- `go-patterns`, `python-patterns` — Language-specific patterns
