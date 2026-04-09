# Diataxis Framework Reference

Four documentation types based on user needs. Each document should have one clear type -- don't mix.

## The Four Types

### Tutorial (Learning-Oriented)
- **Purpose**: Help beginners achieve first success through guided experience
- **Structure**: Learning goal upfront, prerequisites listed, numbered steps (one path only), expected outcomes shown, success messaging, "what's next" links
- **Include**: Concrete tested steps, screenshots/expected outputs, minimal scope
- **Exclude**: Concept explanations (save for Explanation), multiple paths/options, production patterns, extensive troubleshooting, advanced config
- **Mistakes**: Explaining too much, offering choices, assuming knowledge, making it too long (target 10-15 min), production-grade examples

### How-To Guide (Task-Oriented)
- **Purpose**: Help users accomplish a specific real-world task. Assumes user knows basics
- **Structure**: Task/goal in title, prerequisites with links, steps, options/alternatives, troubleshooting section, success verification
- **Include**: "How to..." title, multiple approaches, production-ready examples, troubleshooting, related tasks
- **Exclude**: Concept explanations (save for Explanation), complete reference (save for Reference), beginner hand-holding (they need Tutorial)
- **Mistakes**: Teaching instead of doing, one-size-fits-all (no alternatives), toy examples, missing troubleshooting

### Reference (Information-Oriented)
- **Purpose**: Accurate, complete technical information in structured, scannable format
- **Structure**: Consistent formatting, logical organization, parameters with types/constraints, brief examples, no instructions
- **Include**: Complete option/parameter lists, data types, defaults, brief examples, return values
- **Exclude**: Tutorials or step-by-step, concept explanations, recommendations/opinions, long examples
- **Mistakes**: Narrative style, incomplete coverage, inconsistent format, teaching

### Explanation (Understanding-Oriented)
- **Purpose**: Deepen understanding of concepts, design decisions, and "why" questions
- **Structure**: Topic stated, background/context, diagrams, design rationale, trade-offs, connections to related concepts
- **Include**: "Why" and "how it works", architecture overviews, design decisions, comparisons, historical context
- **Exclude**: Step-by-step instructions (save for How-To/Tutorial), complete specs (save for Reference)
- **Mistakes**: Diving into implementation, assuming audience, no diagrams, being too abstract

## Decision Matrix

```
Learning + Doing = Tutorial
Working  + Doing = How-To Guide
Learning + Knowing = Explanation
Working  + Knowing = Reference
```

## Common Type Violations

- **Tutorial with too many options**: Causes decision paralysis. Pick one path, mention alternatives in How-To
- **Reference with instructions**: "First, initialize..." breaks scannable format. Move to How-To
- **How-To with lengthy explanations**: 3 paragraphs of theory before task. Link to Explanation instead
- **Explanation with step-by-step**: Deployment commands in conceptual doc. Move to How-To

## Acceptable Type Mixing

- Tutorial + basic troubleshooting ("If you see error X, do Y") -- keeps tutorial successful
- How-To + 1-2 sentence context before task -- motivates "how" without becoming Explanation
- Reference + brief code snippet -- illustrates info without becoming How-To

## Diataxis + 7-Action Model

| Diataxis Type | Primary 7-Actions Served |
|---------------|--------------------------|
| Tutorial | Use (basic), Onboard (hands-on intro) |
| How-To Guide | Use, Adopt, Administer, Optimize, Troubleshoot |
| Reference | Use (lookup), Administer (policy ref), Troubleshoot (error codes) |
| Explanation | Onboard, Adopt (architecture), Optimize (trade-offs) |

## Quality Checklists

**Tutorial**: Beginner can succeed? One path? Prerequisites explicit? Avoids "why"? Clear success moment? Links to next steps?

**How-To**: Task/goal clear from title? Prerequisites listed? Multiple approaches when valid? Troubleshooting section? Production-ready?

**Reference**: Coverage complete? Format consistent? Brief examples? Searchable/scannable? No instructions/opinions?

**Explanation**: Builds mental model? Grounded in examples? Trade-offs discussed? Diagrams? Avoids step-by-step? Answers "why"?

## When to Split

Split a document when: serving multiple Diataxis types, serving multiple audience levels, exceeding ~2000 words on distinct topics, or mixing incompatible 7-Actions. After splitting, add prominent wayfinding between related docs.
