# Good vs Bad Recommendations

Side-by-side comparisons illustrating specificity, framework application, and actionability.

## Pattern 1: Adding Content

**Bad**: "Add more code examples to help developers."
- No specificity (how many? what type? where?), no framework justification, no persona reference

**Good**: "Add 3 code examples (basic auth, token-based, mTLS) in 'Authentication Methods' section after line 145. Three personas flagged missing code as blocking Use action. Tutorial (Diataxis) requires executable steps. Include working JavaScript with error handling, 15-25 lines each."

**Key difference**: Good specifies count, content, location, personas, framework justification, format, and scope.

## Pattern 2: Restructuring

**Bad**: "Reorganize the document to flow better and make more sense."
- No current vs desired structure, no framework basis, no specific moves

**Good**: "Split into two documents: (1) 'Workload Identity for Security Leaders' (Explanation, Onboard action) with sections 1,2,4,6 and (2) 'Developer Guide' (How-To, Use action) with sections 3,5,7. Cyrus (CISO) frustrated by implementation detail; Devon (Developer) frustrated by strategic content. Diataxis: mixing Explanation+How-To violates type purity. 7-Action: Onboard+Use are incompatible in one doc."

**Key difference**: Good names specific sections to move, identifies personas, cites both frameworks, defines new document types.

## Pattern 3: Fixing Clarity

**Bad**: "The authentication section is confusing and needs to be clearer."
- Subjective, no specific terms, no analysis of root cause

**Good**: "Define mTLS, OIDC, SPIFFE, RBAC on first use (lines 52-55). New User: 'Completely lost -- what do these acronyms mean?' Replace one-sentence mention with inline definitions + parenthetical expansions. Add link to concepts page. Diataxis How-To principle: provide enough context for informed choices."

**Key difference**: Good identifies specific terms, exact lines, persona quote, replacement approach, framework backing.

## Pattern 4: Adding Wayfinding

**Bad**: "Add more links to related content."
- No specific elements, no analysis of where users get lost

**Good**: "Add developer exit callout after intro (line 15): 'Developers: Ready to implement? Beginners: [Tutorial], Experienced: [Production Guide]'. Add Next Steps section at end with audience-specific paths. Devon arrived via search expecting Use action, page provides Onboard. Page Type Mapping: Use Case pages need clear paths to How-To pages."

**Key difference**: Good specifies exact placement, exact text, persona context, action mismatch analysis.

## Pattern 5: Handling Conflicts

**Bad**: "Find a middle ground that works for everyone."
- No conflict analysis, "middle ground" satisfies no one, no framework decision

**Good**: "Devon (Developer) wants implementation depth (Use action, How-To type). Cyrus (CISO) wants strategic overview (Onboard action, Explanation type). Decision: Split. Keep current doc as Explanation for Onboard. Create new How-To for Use. Diataxis: mixing types violates purity. Sacrifice: users navigate between docs. Mitigation: prominent bidirectional wayfinding."

**Key difference**: Good names the conflict, cites frameworks, makes clear decision, states sacrifice and mitigation.

## Summary

| Quality | Characteristics |
|---------|----------------|
| **Bad** | Vague, no framework, no personas, no location, needs clarification to implement |
| **Good** | Specific, framework-justified, persona-tied, exact locations, implementable without questions |

**The Handoff Test**: Can someone unfamiliar with the document implement this recommendation without asking questions?
