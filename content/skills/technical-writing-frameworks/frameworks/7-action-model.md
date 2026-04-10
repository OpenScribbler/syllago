# The 7-Action Model Reference

Users have specific goals (actions) when reading documentation. Each document should intentionally address 1-2 actions for specific audiences.

## The Seven Actions

### 1. Onboard
**Intent**: "What is this? Why should I care?"
**Audiences**: Executives, evaluators, new users
**Content**: Value propositions, use cases, comparisons, architecture diagrams (conceptual)
**Diataxis**: Explanation
**Red flags**: Tutorial starts with "why use this?", reference docs include value props

### 2. Adopt
**Intent**: "How do I integrate this into my environment?"
**Audiences**: DevOps, architects, AppSec, team leads
**Content**: Deployment guides, integration patterns, prerequisites, governance setup
**Diataxis**: How-To + Explanation
**Red flags**: Deployment guide assumes already deployed, toy examples instead of production patterns

### 3. Use
**Intent**: "How do I accomplish this specific task?"
**Audiences**: Developers, DevOps, anyone task-focused
**Content**: Step-by-step instructions, code examples, CLI commands, config guides
**Diataxis**: How-To (advanced), Tutorial (basic)
**Red flags**: Conceptual steps instead of concrete ones, missing code examples, outdated examples

### 4. Administer
**Intent**: "How do I manage this at scale?"
**Audiences**: Platform teams, security admins, compliance
**Content**: Admin console guides, policy management, bulk operations, audit/reporting
**Diataxis**: How-To + Reference
**Red flags**: Only single-service examples, admin features buried in general docs

### 5. Optimize
**Intent**: "How do I make this faster/cheaper/more reliable?"
**Audiences**: DevOps, platform engineers
**Content**: Performance tuning, automation, cost optimization, monitoring setup
**Diataxis**: How-To + Explanation
**Red flags**: Optimization in getting-started docs, no baseline guidance before optimization

### 6. Troubleshoot
**Intent**: "This is broken. How do I fix it?"
**Audiences**: Everyone (when problems occur)
**Content**: Error explanations, diagnostic flowcharts, common issues/solutions, log analysis
**Diataxis**: How-To + Reference
**Red flags**: Error messages not documented, no path from error to solution

### 7. Offboard
**Intent**: "How do I migrate away or remove this?"
**Audiences**: Platform teams, architects
**Content**: Migration guides, removal procedures, data export, rollback strategies
**Diataxis**: How-To
**Red flags**: No migration docs, removal undocumented, unclear data export

## Review Process

1. **Identify intent**: Which action(s) should this doc address? (Based on title, audience, placement)
2. **Assess coverage**: For each relevant action: Addressed well / Partially / Missing / N/A
3. **Identify mismatches**:
   - Wrong action for document type (Tutorial spending half on "why" = Onboard in Use doc)
   - Missing expected action (API Reference lacking troubleshooting for common errors)
   - User arrived for action X, doc delivers action Y (DevOps wants Adopt, lands in Onboard)
   - Multiple conflicting actions (Onboard for execs + Use for devs in one doc)
4. **Recommend**: Add content, remove content, split document, or add wayfinding

## Persona-Action Mapping

| Persona | Primary Actions | Secondary Actions |
|---------|----------------|-------------------|
| **CISO** | Onboard, Adopt (governance) | Administer (policy oversight) |
| **DevOps** | Adopt, Administer, Optimize, Troubleshoot | Use (when implementing) |
| **Developer** | Use, Adopt (integration), Troubleshoot | Onboard (if new) |
| **AppSec** | Adopt (security), Administer, Troubleshoot | Use (when auditing) |
| **New User** | Onboard, Use (basic) | Adopt (when ready) |

## Common Pitfalls

- **Action creep**: Document tries to address all 7 actions. Focus on 1-2, link to others
- **Action-type mismatch**: Tutorial (learning) tries to address Optimize (advanced). Align with Diataxis
- **Missing Troubleshoot**: No troubleshooting anywhere. Add to relevant docs or create dedicated guide
- **Assuming progression**: Use docs assume Adopt is done. Add wayfinding for users arriving via search
