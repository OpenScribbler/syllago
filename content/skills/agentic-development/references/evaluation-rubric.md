# Agent & Skill Evaluation Rubric

Structured framework for assessing quality. Six agent dimensions + five skill dimensions.

---

## Agent Evaluation (6 Dimensions)

### 1. Clarity (1-10)
Are instructions unambiguous? Would different interpretations lead to different behaviors?

| Score | Criteria |
|-------|----------|
| 9-10 | Every instruction has one clear interpretation. Concrete examples provided. |
| 7-8 | Most instructions clear. Minor ambiguities unlikely to cause issues. |
| 5-6 | Some vague instructions ("be thorough", "use judgment"). |
| 3-4 | Multiple vague instructions. Significant interpretation variance. |
| 1-2 | Mostly vague. Unpredictable behavior. |

**Red flags**: "Be helpful/thorough/careful", "Use your best judgment", "When appropriate".

### 2. Specialization (1-10)
Does the agent have a single, focused purpose?

| Score | Criteria |
|-------|----------|
| 9-10 | Single clear purpose. Domain-specific expertise. Ignores out-of-scope. |
| 7-8 | Focused with minor scope overlap. |
| 5-6 | Covers 2-3 related domains. Some dilution. |
| 3-4 | Kitchen-sink agent. Many unrelated areas. |
| 1-2 | Generic "helpful assistant" with no specialization. |

**Check**: Specific persona? Clear domain? Explicit out-of-scope? Would splitting improve quality?

### 3. Workflow Quality (1-10)
Does the agent follow patterns that lead to successful outcomes?

| Score | Criteria |
|-------|----------|
| 9-10 | Plan-first. Verification after changes. Investigation before action. Clear phases. |
| 7-8 | Good structure. Minor gaps in verification or planning. |
| 5-6 | Basic workflow. Missing verification loops or planning phase. |
| 3-4 | Jumps to implementation. No verification. |
| 1-2 | No workflow structure. Improvises everything. |

**Check**: Plan-first? Verification loops? Investigation before action? Escalation path?

### 4. Context Awareness (1-10)
Does the agent manage context effectively?

| Score | Criteria |
|-------|----------|
| 9-10 | Explicit context guidance. Knows when to load, delegate, or reduce. |
| 7-8 | Good management. Minor gaps. |
| 5-6 | Some awareness. May load too much or miss files. |
| 3-4 | Poor management. Tries to hold everything. |
| 1-2 | No awareness. Will fail on large codebases. |

**Check**: File loading guidance? Delegation thresholds (<50K/50-100K/>100K)? Skill loading? Progressive disclosure?

### 5. Safety (1-10)
Are appropriate guardrails in place?

| Score | Criteria |
|-------|----------|
| 9-10 | Clear boundaries, confidence requirements, escalation paths. |
| 7-8 | Good boundaries. Minor gaps. |
| 5-6 | Some boundaries. Missing confidence indicator or escalation. |
| 3-4 | Weak boundaries. May take inappropriate actions. |
| 1-2 | No guardrails. |

**Check**: "What I DON'T Do"? Confidence indicator with action rules? Tool restrictions appropriate to role?

### 6. Efficiency (1-10)
Is token usage optimized?

| Score | Criteria |
|-------|----------|
| 9-10 | <150 lines. Skills for knowledge. Wrappers for CLI. Progressive loading. |
| 7-8 | Good efficiency. Minor embedded content. |
| 5-6 | Some bloat. Embedded tables that should be skills. 150-200 lines. |
| 3-4 | Significant bloat. Raw CLI, large embedded content. 200-300 lines. |
| 1-2 | >300 lines. Entire reference docs in prompt. |

**Size thresholds**: <150 = excellent, 150-200 = acceptable, 200-250 = must extract, >250 = refactor.

---

## Trustworthiness Index

Average of 6 dimensions. **Critical weakness rule**: Any dimension <=4 is a blocker regardless of average.

| Index | Interpretation |
|-------|----------------|
| 9-10 | Production ready |
| 7-8 | Ready with minor improvements |
| 5-6 | Usable with caution |
| 3-4 | Significant revision needed |
| 1-2 | Fundamental redesign required |

**Priority order for fixes**: Context Awareness (80% of failures) > Workflow Quality > Specialization > Clarity > Safety > Efficiency.

---

## Skill Evaluation (5 Dimensions)

### 1. Structure (1-10)
Clear entry point (<100 lines), progressive detail, logical organization. SKILL.md with quick reference + separate references with load guidance.

### 2. Actionability (1-10)
Concrete examples, copy-paste commands, decision trees. BAD/GOOD pairs for patterns. Not just abstract principles.

### 3. Token Efficiency (1-10)
Quick reference covers 80% of needs. Deep content loadable separately. SKILL.md <100 lines.

### 4. Reusability (1-10)
Used by 3+ agents. Domain knowledge (not agent-specific behavior). Clear loading protocol. No agent assumptions.

### 5. Content Quality (1-10)
Accurate, current, comprehensive. Patterns are proven. Commands tested. Edge cases covered.

---

## Quick Evaluation Checklists

### Agent Quick Check
| Check | Pass/Fail |
|-------|-----------|
| Single clear purpose (not "helpful assistant")? | |
| "What I DON'T Do" section? | |
| Confidence indicator with action rules? | |
| Plan-first workflow? | |
| Verification step after changes? | |
| Context guidance (when to load/delegate)? | |
| <200 lines (knowledge in skills)? | |
| Skill loading instructions? | |

### Skill Quick Check
| Check | Pass/Fail |
|-------|-----------|
| SKILL.md <100 lines? | |
| Quick reference table? | |
| Deep content in references/? | |
| Concrete examples? | |
| Reusable by multiple agents? | |

---

## Evaluation Report Template

```markdown
# Evaluation: [Name]

## Summary
[2-3 sentence assessment]

## Scores
| Dimension | Score | Notes |
|-----------|-------|-------|
| [dim] | X/10 | [observation] |
| **Trustworthiness Index** | **X/10** | |

## Critical Issues (any dimension <=4)
- [Issue] | Dimension: [X] | Confidence: HIGH

## Recommended Improvements (by failure impact)
1. [Fix] | Impact: HIGH | Confidence: HIGH

## Strengths to Preserve
- [What's working]
```
