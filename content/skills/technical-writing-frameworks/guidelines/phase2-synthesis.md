# Phase 2: Synthesis

Transform Phase 1 analysis into a comprehensive, actionable plan.

## Required Plan Sections

### A. Executive Summary (2-4 sentences)
Answer: Current state, 1-2 primary problems, recommended approach (fix, restructure, split, supplement).

### B. Framework Analysis

**B1: 7-Action Model** -- List actions the doc SHOULD address, actions currently addressed (well/poorly/missing), gaps, and mismatches.

**B2: Diataxis Classification** -- Current type with evidence, user expectation with evidence, match assessment, structural issues, recommendation (keep type, reclassify, split, supplement).

### C. Verification (Before Action Items)
- Any claims about features/platforms verified?
- Any recommended links verified to exist?
- Link format correct?

### D. Prioritized Action Items

For each item use this structure:

```
### [P0/P1/P2/P3]: [Actionable Title]

**Issue**: [What's wrong, reference persona feedback]
**Affected Users**: [Personas, access contexts]
**Impact**: [Why it matters, what happens if unfixed]
**Recommendation**: [Specific fix]
**Confidence**: HIGH/MEDIUM/LOW with justification
**Framework Justification**: 7-Action: [which action] | Diataxis: [which principle]
**Trade-offs**: [If applicable]
**Effort**: Small/Medium/Large with justification
```

**Priority definitions**:
- P0: Blocking user success, must fix
- P1: Significantly degrades experience, should fix soon
- P2: Quality improvement, fix when time allows
- P3: Future enhancement

### E. Trade-offs (if personas conflict)
Use trade-off structure from tradeoffs.md.

### F. Document Splitting (if applicable)
For each proposed document: type, audience, actions, content from current doc, new content needed. Include wayfinding between documents.

## Plan Closing

```
Please review and provide feedback on:
1. Priority assignments
2. Framework decisions and trade-offs
3. Specific recommendations
4. Splitting recommendation (if proposed)

Type "APPROVE" to proceed, or provide feedback.
```

## Quality Checklist

- Every P0 clearly blocks user success
- All conflicts have documented trade-offs
- Framework justifications present and correct
- Confidence levels assigned with reasoning
- Recommendations specific and actionable
- Effort estimates realistic
- Empty sections explicitly stated (see empty-phases.md)
