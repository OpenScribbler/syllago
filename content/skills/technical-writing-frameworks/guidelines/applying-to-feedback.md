# Applying Frameworks to Feedback

Every recommendation must cite framework requirements, not just address feedback symptoms.

## Core Technique

**Wrong (symptom-based)**: "Add prerequisites -- addresses Theme 1 feedback about missing prerequisites"

**Right (framework-grounded)**: "Add prerequisites -- Diataxis Tutorial type requires explicit prerequisites (learning-oriented docs must state what learner needs). 7-Action Onboard intent requires clear readiness criteria (users must assess if they can proceed)."

## Recommendation Format

```
### Recommendation [N]: [Action Verb] [What]

**Framework basis**:
- Diataxis: [Document type requirement + why it exists]
- 7-Action: [User intent need + why it serves the user]

**What**: [Specific change with bulleted details]
**Why**: [Feedback theme + how it serves user intent]
**Where**: [Exact location in document]
**Verification**:
- [Diataxis requirement check]
- [7-Action Model check]
- [Link verification if applicable]
```

## Implementation Task Format

```
## Task [N]: [Action Verb] [What]

**Framework alignment**: Diataxis: [requirement] | 7-Action: [intent]
**Steps**: [Numbered implementation steps]
**Expected output**: [What should exist when complete]
**Verification**: [Framework compliance checks, not just "exists"]
```

## Common Failures

1. **Not loading this skill**: Agent substitutes generic frameworks (Bloom's Taxonomy, Cognitive Load Theory) instead of Diataxis/7-Action
2. **Recommendations without framework grounding**: Says "addresses Theme X" without citing which Diataxis type requirement or 7-Action intent is served
3. **No documentation verification**: Recommending links without checking they exist
4. **Generic verification**: "Section exists" instead of framework compliance checks like "Prerequisites align with tutorial scope (Diataxis)"

## Checklist

**Before analysis**: Load technical-writing-frameworks, this guide, and relevant doc skills

**During analysis**: Identify Diataxis type, list type requirements, map feedback to 7-Action intents, verify related docs exist

**Creating recommendations**: Each cites framework basis, each explains which requirement it satisfies, all links verified

**Creating implementation plan**: Each task states framework alignment, expected outputs include framework requirements, verification checks framework compliance
