# EvaluateSkill Workflow

> **Trigger:** "evaluate skill", "review skill", "assess skill", "skill quality check"

## Purpose

Systematically evaluate a skill against quality rubrics, identify issues, and propose improvements.

> **Note:** For agent evaluation, use the [EvaluateAgent](EvaluateAgent.md) workflow instead.

## Prerequisites

- Path to skill directory (`skills/*/SKILL.md`)
- Access to skill's references and workflows
- Evaluation rubric loaded (`references/evaluation-rubric.md` — Skill Evaluation Framework section)

## Interactive Flow

### Phase 1: Identify Target

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | Skill path | path | (required) | Valid `skills/*/SKILL.md` path |
| 2 | Specific concern | text | (optional) | Helps focus evaluation |
| 3 | Evaluation depth | choice | "standard" | Quick/Standard/Deep |

**Question Examples:**

Question 1: "Which skill should we evaluate?"
- Header: "Skill"
- Options: [List discovered skills from `ls skills/*/SKILL.md`]

Question 2: "Is there a specific issue you've noticed? (optional)"
- Header: "Concern"
- Options: ["No specific issue - general review", "Structure problems", "Missing content", "Token waste / too large", "Other - describe"]

Question 3: "How deep should the evaluation be?"
- Header: "Depth"
- Options: [
    "Quick - checklist only",
    "Standard - checklist + red flags + scoring (Recommended)",
    "Deep - full rubric scoring + content audit"
  ]

### Phase 2: Load and Analyze

```
1. Load skill files:
   - Read SKILL.md entry point
   - List references/ directory contents
   - List Workflows/ directory contents (if exists)
   - Read each reference file
   - Read each workflow file

2. Collect metrics:
   - SKILL.md line count
   - Number of reference files
   - Total lines across all files
   - Number of workflows

3. Load evaluation materials:
   - Quick: Use Skill Quick Check from SKILL.md checklist
   - Standard: Also check red flags below
   - Deep: Also load references/evaluation-rubric.md (Skill Evaluation Framework)
```

### Phase 3: Evaluate Dimensions

Evaluate against **5 skill-specific dimensions**:

| Dimension | Focus | Key Question |
|-----------|-------|--------------|
| **Structure** | Organization and navigation | Is SKILL.md <100 lines with progressive detail? |
| **Actionability** | Practical usefulness | Are there copy-paste examples and BAD/GOOD pairs? |
| **Token Efficiency** | Load cost vs value | Does quick reference cover 80% of needs? |
| **Reusability** | Cross-agent value | Can multiple agents use this without assumptions? |
| **Content Quality** | Accuracy and completeness | Are patterns proven and current? |

**For each dimension, collect evidence:**

```
Dimension: [Name]
Score: [1-10]
Evidence:
  - [Direct observation or quote from skill files]
  - [Metric or structural fact]
Impact: [What goes wrong if this is weak]
```

**Structure checks:**
- [ ] SKILL.md exists as entry point
- [ ] SKILL.md < 100 lines
- [ ] References in separate files under `references/`
- [ ] Clear "When to Load" guidance for each reference
- [ ] Grep-friendly section headers

**Actionability checks:**
- [ ] BAD/GOOD example pairs or condensed rule statements for patterns
- [ ] Copy-paste ready commands or code snippets
- [ ] Decision trees for common choices
- [ ] Concrete patterns, not just abstract principles

**Token Efficiency checks:**
- [ ] Quick reference table satisfies common lookups
- [ ] Deep content deferred to `references/`
- [ ] References loadable independently (not all-or-nothing)
- [ ] No duplicate content across files

**Reusability checks:**
- [ ] Content is domain knowledge, not agent behavior
- [ ] No assumptions about specific agent workflow
- [ ] Loading instructions work for any agent
- [ ] No hardcoded paths to specific agents

**Content Quality checks:**
- [ ] Patterns are proven (not theoretical)
- [ ] Commands are tested and work
- [ ] Content is current (not outdated APIs or patterns)
- [ ] Edge cases and error scenarios covered

### Phase 4: Document Findings

For each finding, document:

```markdown
### Finding: [Title]

**Dimension:** [Which dimension this affects]
**Confidence:** [HIGH/MEDIUM/LOW]
**Evidence:** [Direct quote or specific observation]
**Impact:** [What goes wrong because of this]
**Recommendation:** [Specific fix]
```

**Categorize findings by confidence:**

- **HIGH Confidence**: Clear structural violation or missing content, direct evidence
- **MEDIUM Confidence**: Likely issue, pattern-based assessment
- **LOW Confidence**: Possible improvement, subjective assessment

### Phase 5: Present Evaluation Report

**For Quick Evaluation:**

```markdown
# Quick Evaluation: [skill name]

## Checklist Results

| Check | Status | Note |
|-------|--------|------|
| SKILL.md < 100 lines (Structure) | [Pass/Fail] | [Actual line count] |
| Quick reference table (Structure) | [Pass/Fail] | [Brief note] |
| Deep content in references/ (Token Efficiency) | [Pass/Fail] | [Brief note] |
| BAD/GOOD example pairs (Actionability) | [Pass/Fail] | [Brief note] |
| Decision trees (Actionability) | [Pass/Fail] | [Brief note] |
| Reusable by multiple agents (Reusability) | [Pass/Fail] | [Brief note] |

## Summary
[1-2 sentence overall assessment]

## Top Issues (if any)
1. [Issue] - [Quick fix suggestion]
```

**For Standard/Deep Evaluation:**

```markdown
# Skill Evaluation: [skill name]

## Summary
**Quality Index:** X/10
[2-3 sentence overall assessment]

## Dimension Scores

| Dimension | Score | Key Issue |
|-----------|-------|-----------|
| Structure | X/10 | [Issue or "Good"] |
| Actionability | X/10 | [Issue or "Good"] |
| Token Efficiency | X/10 | [Issue or "Good"] |
| Reusability | X/10 | [Issue or "Good"] |
| Content Quality | X/10 | [Issue or "Good"] |

## Metrics

| Metric | Value | Target |
|--------|-------|--------|
| SKILL.md lines | [N] | <100 |
| Reference files | [N] | 1+ |
| Total lines | [N] | — |
| Workflows | [N] | — |

## Findings

### HIGH Confidence
[Findings with direct evidence]

### MEDIUM Confidence
[Findings with circumstantial evidence]

### LOW Confidence
[Potential issues for discussion]

## Proposed Changes

### Will implement on approval
1. [Change]: [Rationale]

### Recommend with confirmation
1. [Change]: [Rationale]

### For discussion only
1. [Potential change]: [Why uncertain]

---

## Approval Required

Please respond with:
- **"Approved"** - Implement HIGH and MEDIUM confidence changes
- **"Approved with modifications"** - Specify changes to include/exclude
- **"Questions"** - Ask for clarification
```

### Phase 6: Implement Approved Changes

After approval:

1. **Implement HIGH confidence changes first**
2. **Implement approved MEDIUM confidence changes**
3. **Skip LOW confidence unless explicitly approved**
4. **Show before/after for significant changes**

```markdown
## Changes Implemented

### Change 1: [Title]
**Before:**
[snippet]

**After:**
[snippet]

### Change 2: [Title]
[...]

## Verification
- [ ] SKILL.md still < 100 lines
- [ ] All reference links valid
- [ ] No duplicate content introduced
- [ ] Grep-friendly headers preserved
```

**Completion report:**

```
Skill evaluation complete!

Skill: [skill path]
Quality Index: [before] -> [after]

Changes made:
  - [Change 1]
  - [Change 2]

Deferred (LOW confidence, not approved):
  - [Item 1]

Next steps:
  1. Test skill loading from an agent
  2. Verify references load independently
  3. Re-evaluate after updates to confirm improvement
```

## Error Handling

| Error | Action |
|-------|--------|
| Skill path not found | List available skills, re-prompt |
| Missing SKILL.md | Flag as critical Structure finding (score 1) |
| No references/ directory | Flag as Structure finding, may be intentional for small skills |
| Cannot read reference file | Note as "Unable to assess" for that file |
| No issues found | Report clean bill of health with score breakdown |
| Conflicting evidence | Note both pieces of evidence, lower confidence |

## Skill-Specific Red Flags

| Red Flag | Dimension | Severity |
|----------|-----------|----------|
| SKILL.md > 100 lines | Structure, Token Efficiency | HIGH |
| Monolithic (no references/) | Structure | HIGH |
| No BAD/GOOD examples or rule statements | Actionability | HIGH |
| Missing "When to Load" sections | Structure | MEDIUM |
| Duplicate content across files | Token Efficiency | MEDIUM |
| Agent-specific assumptions in content | Reusability | MEDIUM |
| No decision trees | Actionability | LOW |
| No quick reference table | Token Efficiency | MEDIUM |
| Outdated API references or patterns | Content Quality | HIGH |
| Theoretical patterns without examples | Content Quality | MEDIUM |

## Positive Signals

| Signal | Dimension |
|--------|-----------|
| Progressive detail (overview → deep dive) | Structure |
| Self-contained reference files | Structure, Reusability |
| Copy-paste ready snippets | Actionability |
| Cross-skill references | Reusability |
| Wrapper script integration | Token Efficiency |
| Version/date stamps on commands | Content Quality |
