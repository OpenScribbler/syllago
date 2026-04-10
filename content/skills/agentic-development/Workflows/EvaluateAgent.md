# EvaluateAgent Workflow

> **Trigger:** "evaluate agent", "review agent", "assess agent", "agent quality check", "why is my agent [failing/inconsistent/slow]"

## Purpose

Systematically evaluate an agent against quality rubrics, identify issues, and propose improvements.

> **Note:** For skill evaluation, use the [EvaluateSkill](EvaluateSkill.md) workflow instead.

## Prerequisites

- Path to agent file (`agents/*.md`)
- Access to referenced skills and workflows
- Evaluation rubric loaded (`references/evaluation-rubric.md` — Agent Evaluation Framework section)

## Interactive Flow

### Phase 1: Identify Target

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | Agent path | path | (required) | Valid `agents/*.md` path |
| 2 | Specific concern | text | (optional) | Helps focus evaluation |
| 3 | Evaluation depth | choice | "standard" | Quick/Standard/Deep |

**Question Examples:**

Question 1: "Which agent should we evaluate?"
- Header: "Agent"
- Options: [List discovered agents from `ls agents/*.md`]

Question 2: "Is there a specific issue you've noticed? (optional)"
- Header: "Concern"
- Options: ["No specific issue - general review", "Inconsistent output", "Missing cases", "Too slow/verbose", "Other - describe"]

Question 3: "How deep should the evaluation be?"
- Header: "Depth"
- Options: [
    "Quick - checklist only (2 min)",
    "Standard - checklist + anti-patterns (5 min)",
    "Deep - full rubric scoring (10 min)"
  ]

### Phase 2: Load and Analyze

```
1. Load target file(s):
   - Read the agent/skill file
   - Read any referenced skills
   - Read any Workflows/ files

2. Load evaluation materials:
   - Quick: Use checklist from SKILL.md
   - Standard: Also load references/anti-patterns.md
   - Deep: Also load references/evaluation-rubric.md

3. Run evaluation:
   - Check each dimension
   - Note evidence for each finding
   - Assign confidence level (HIGH/MEDIUM/LOW)
```

**Evaluation Dimensions (6):**

| Dimension | What to Check |
|-----------|---------------|
| **Clarity** | Unambiguous instructions? No vague terms? |
| **Specialization** | Single focused purpose? Not kitchen-sink? |
| **Workflow Quality** | Plan-first? Verification loops? Investigation before action? |
| **Context Awareness** | Knows when to delegate? File loading guidance? |
| **Safety** | Boundaries defined? Confidence levels with action rules? |
| **Efficiency** | <200 lines? Knowledge in skills? Token-optimized commands? |

### Phase 3: Document Findings

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

- **HIGH Confidence**: Clear pattern/anti-pattern violation, direct quote as evidence
- **MEDIUM Confidence**: Likely issue, circumstantial evidence
- **LOW Confidence**: Possible issue, heuristic-based

### Phase 4: Present Evaluation Report

**For Quick Evaluation:**

```markdown
# Quick Evaluation: [target name]

## Checklist Results

| Check | Status | Note |
|-------|--------|------|
| Clear instructions (Clarity) | [Pass/Fail] | [Brief note] |
| Single focused purpose (Specialization) | [Pass/Fail] | [Brief note] |
| Plan-first + verification (Workflow) | [Pass/Fail] | [Brief note] |
| Context/delegation guidance (Context) | [Pass/Fail] | [Brief note] |
| Boundaries + confidence (Safety) | [Pass/Fail] | [Brief note] |
| <200 lines, skills loaded (Efficiency) | [Pass/Fail] | [Brief note] |

## Summary
[1-2 sentence overall assessment]

## Top Issues (if any)
1. [Issue] - [Quick fix suggestion]
```

**For Standard/Deep Evaluation:**

```markdown
# Evaluation: [target name]

## Summary
**Trustworthiness Index:** X/10
[2-3 sentence overall assessment]

## Dimension Scores

| Dimension | Score | Key Issue |
|-----------|-------|-----------|
| Clarity | X/10 | [Issue or "Good"] |
| Specialization | X/10 | [Issue or "Good"] |
| Workflow Quality | X/10 | [Issue or "Good"] |
| Context Awareness | X/10 | [Issue or "Good"] |
| Safety | X/10 | [Issue or "Good"] |
| Efficiency | X/10 | [Issue or "Good"] |

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

### Phase 5: Implement Approved Changes

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
- [How to verify improvement worked]
```

### Phase 6: Report Completion

```
Evaluation complete!

Target: [target path]
Trustworthiness Index: [before] -> [after]

Changes made:
  - [Change 1]
  - [Change 2]

Deferred (LOW confidence, not approved):
  - [Item 1]

Next steps:
  1. Test the agent/skill with sample inputs
  2. Monitor for the specific concern if one was raised
  3. Re-evaluate after usage to confirm improvement
```

## Error Handling

| Error | Action |
|-------|--------|
| File not found | List available agents/skills, re-prompt |
| Cannot read referenced skill | Note as "Unable to assess - skill not accessible" |
| No issues found | Report clean bill of health with score breakdown |
| Conflicting evidence | Note both pieces of evidence, lower confidence |
| User cancels mid-evaluation | Save partial findings for future reference |

## Evaluation Heuristics

### Red Flags (Usually HIGH Confidence Issues)

| Pattern | Why It's a Problem |
|---------|-------------------|
| "Be helpful" or "Use best judgment" | Vague, leads to inconsistency |
| No "What I DON'T Do" section | Missing boundaries |
| Raw CLI commands without filtering | Token waste |
| >200 lines in agent prompt | Probably needs skill extraction |
| No examples in description | Unclear when to use |
| Multiple domains in one agent | Scope creep |

### Positive Signals

| Pattern | Why It's Good |
|---------|---------------|
| Confidence levels defined | Consistent output quality |
| Skills loaded, not embedded | Separation of concerns |
| Wrapper scripts used | Token efficiency |
| Clear phase-based workflow | Predictable behavior |
| Specific error handling | Robust operation |

## Anti-Pattern Quick Check

Before deep evaluation, check for these anti-patterns:

1. **Prompt Stuffing**: Agent has >150 lines of static content
2. **Wishful Tooling**: References tools it doesn't have access to
3. **Confidence Theater**: Uses confidence words without evidence requirements
4. **Kitchen Sink Agent**: Tries to do everything
5. **Implicit Boundaries**: Says what to do but not what to avoid
