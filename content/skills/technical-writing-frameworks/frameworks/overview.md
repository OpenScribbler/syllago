# Frameworks Quick Reference

Combined quick-reference for 7-Action Model and Diataxis. Load for fast lookup during active review/synthesis.

## 7-Action Model (User Intent)

| Action | User Intent | Typical Audiences |
|--------|-------------|-------------------|
| **Onboard** | "What is this? Why should I care?" | CISO, New Users, Evaluators |
| **Adopt** | "How do I integrate this?" | DevOps, AppSec, Architects |
| **Use** | "How do I do this specific task?" | Developers, DevOps, Everyone |
| **Administer** | "How do I manage at scale?" | Platform Teams, Security Admins |
| **Optimize** | "How do I make this better?" | DevOps, Platform Engineers |
| **Troubleshoot** | "How do I fix this?" | Everyone (when problems occur) |
| **Offboard** | "How do I migrate away?" | Platform Teams, Architects |

- Documents should address 1-2 specific actions, not all seven
- Actions progress: Onboard -> Adopt -> Use -> Administer -> Optimize (Troubleshoot at any stage)

## Diataxis Framework (Document Structure)

| Type | Purpose | User Mode | Content Focus |
|------|---------|-----------|---------------|
| **Tutorial** | Learning-oriented | Acquiring knowledge | Step-by-step, one path, guaranteed success |
| **How-To Guide** | Task-oriented | Applying knowledge | Goal-driven, production-ready, options shown |
| **Reference** | Information-oriented | Looking up facts | Complete, structured, austere facts |
| **Explanation** | Understanding-oriented | Deep knowledge | Concepts, "why", trade-offs, mental models |

- Don't mix document types. Each document should be clearly one type
- Decision matrix: Learning+Doing=Tutorial, Working+Doing=How-To, Learning+Knowing=Explanation, Working+Knowing=Reference

## Framework Alignment

| 7-Action | Best Diataxis Type(s) |
|----------|----------------------|
| Onboard | Explanation |
| Adopt | How-To + Explanation |
| Use | How-To (advanced), Tutorial (basic) |
| Administer | How-To + Reference |
| Optimize | How-To + Explanation |
| Troubleshoot | How-To + Reference |
| Offboard | How-To |

## Persona Quick Map

| Persona | Primary Actions | Diataxis Types |
|---------|----------------|----------------|
| **CISO** | Onboard, Adopt | Explanation |
| **DevOps** | Adopt, Administer, Optimize, Troubleshoot | How-To + Reference |
| **Developer** | Use, Adopt, Troubleshoot | How-To + Tutorial + Reference |
| **AppSec** | Adopt, Administer, Troubleshoot | How-To + Explanation |
| **New User** | Onboard, Use (basic) | Tutorial + Explanation |

## Quick Diagnosis

| Symptom | Likely Issue | Framework Lens |
|---------|-------------|----------------|
| "Too overwhelming" | Tutorial mixing with Optimize/Administer | 7-Action + Diataxis (Tutorial purity) |
| "Where's the code?" | Use action missing, doc is all Onboard/Explanation | 7-Action (missing Use) |
| "Too basic for me" | Advanced user in Tutorial | Diataxis (Tutorial for beginners only) |
| "I'm lost" | Wrong action or wrong type for user intent | Both frameworks |
| "Too much theory" | How-To mixed with Explanation | Diataxis (type mixing) |
| "Missing troubleshooting" | Troubleshoot action not addressed | 7-Action (missing action) |

## Common Issues and Solutions

- **Tutorial overwhelms beginners**: Mixing Use with Optimize/Administer. Keep Tutorial minimal (one path, no options), link to How-To for advanced topics
- **Developer lands in strategic overview**: User needs Use action, doc provides Onboard. Add wayfinding: "Developers: See [Integration Guide]"
- **Reference doc has step-by-step**: Reference mixed with How-To. Keep Reference austere, move instructions to How-To
- **Single doc serves CISO and Developer**: Mixing Onboard and Use. Split into audience-specific documents with cross-links
- **Missing troubleshooting entirely**: Create Troubleshoot How-To + Error Reference, link from Use/Adopt docs

## Trade-off Template (Quick)

```
CONFLICT: Persona A wants [X], Persona B wants [Y]
FRAMEWORK: A needs [action/type], B needs [action/type]
DECISION: [Clear choice]
RATIONALE: [Framework backing]
GAIN: [Benefits] | SACRIFICE: [Trade-offs] | MITIGATION: [How to address]
```

## Confidence Levels (Quick)

| Level | Criteria |
|-------|----------|
| HIGH | 3+ personas agree, clear framework violation, objective/measurable |
| MEDIUM | 2 personas agree, some ambiguity, best-practices based |
| LOW | 1 persona, speculative, context-dependent, needs validation |
