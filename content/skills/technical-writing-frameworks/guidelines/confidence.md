# Confidence Level Assessment

Help users understand recommendation certainty.

## Three Levels

### HIGH Confidence
- 3+ personas independently identified the issue
- Clear Diataxis or 7-Action violation
- Objective and measurable (missing content, broken link, undefined term)
- Strong precedent in documentation best practices

### MEDIUM Confidence
- 2 personas identified the issue
- Framework suggests an issue but some ambiguity exists
- Based on best practices but valid alternatives may exist
- Somewhat subjective ("needs more detail" -- how much?)

### LOW Confidence
- Single persona mentioned the issue
- Speculative or limited evidence
- Multiple valid approaches, choice depends on unknown context
- Experimental or outside standard practices

## Decision Tree

| Factor | Points |
|--------|--------|
| 3+ personas | +2 |
| 2 personas | +1 |
| 1 persona | +0 |
| Clear framework violation | +2 |
| Possible violation, ambiguity | +1 |
| Judgment call | +0 |
| Objectively measurable | +2 |
| Partially measurable | +1 |
| Highly subjective | +0 |
| Externally verified | +1 |

**5-7 points = HIGH, 3-4 = MEDIUM, 0-2 = LOW**

## When Confidence is LOW

Always include one of:
- **User validation**: Interview 2-3 users to confirm need
- **Pilot/A-B test**: Test with small group before full rollout
- **Reversible implementation**: Add as collapsible section, easy to remove
- **Request context**: Ask specific clarifying question

## Priority x Confidence Matrix

| Combination | Action |
|-------------|--------|
| High Priority + High Confidence | Implement with conviction |
| High Priority + Low Confidence | Important if true, verify first |
| Low Priority + High Confidence | Nice-to-have, not urgent |
| Low Priority + Low Confidence | Future consideration, needs investigation |

## Justification Format

```
**Confidence**: HIGH/MEDIUM/LOW
**Justification**: [Number] personas flagged [issue]. [Framework violation or ambiguity]. [Objective or subjective nature]. [Verification status if applicable].
```
