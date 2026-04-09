# Trade-off Documentation

Use when personas want conflicting things or framework principles conflict with practical concerns.

## Template

```
## Trade-off: [Brief Title]

### The Conflict
**Persona A wants**: [Request] -- Why: [Rationale] -- Evidence: [Quote]
**Persona B wants**: [Conflicting request] -- Why: [Rationale] -- Evidence: [Quote]
**Nature**: [Why these are in tension]

### Framework Analysis
**7-Action**: A needs [action], B needs [action]. [Should these coexist or separate?]
**Diataxis**: A needs [type], B needs [type]. [Does mixing harm users?]

### Decision
**Chosen approach**: [Clear statement]
**Rationale**: [Framework-backed reasons]
**What we gain**: [Benefits]
**What we sacrifice**: [Honest trade-offs]
**Mitigation**: [How to address sacrifices]

### Implementation
**Changes to current doc**: [Specifics]
**New content needed**: [What to create]
**Wayfinding**: [Links/callouts between docs]
**Success criteria**: [How to know it worked]
```

## Common Trade-off Patterns

### Depth vs. Accessibility
- **Pattern**: Developer wants more technical detail, executive wants less
- **Typical resolution**: Split into Explanation (strategic) + How-To (implementation), add wayfinding between them
- **Framework basis**: Diataxis type purity, 7-Action separation of Onboard vs Use

### Completeness vs. Overwhelm
- **Pattern**: Expert wants all edge cases, beginner wants simple path
- **Typical resolution**: Tutorial (minimal, guaranteed success) + separate production/advanced How-To
- **Framework basis**: Diataxis Tutorial purity, progressive disclosure through action progression

### Single Doc vs. Split
- **Pattern**: User wants "everything in one place" vs. focused docs
- **Typical resolution**: Split with strong wayfinding. Single docs serving incompatible audiences serve neither well
- **Framework basis**: Document type integrity, action alignment

## Checklist

Before finalizing a trade-off:
- Conflicting needs clearly identified with persona quotes
- Both frameworks applied (7-Action + Diataxis)
- Clear decision made (not "try to do both")
- Sacrifices honestly stated
- Concrete mitigation strategy provided
- Implementation steps defined
- Success criteria specified
