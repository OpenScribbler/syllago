# Structure Templates

Templates for creating agents and skills with proper structure.

## Agent Structure Template

```markdown
---
name: agent-name
description: |
  When to use this agent. Include trigger phrases.

  <example>
  Context: Situation description
  user: "User message"
  assistant: "How assistant invokes this agent"
  <commentary>
  Why this triggers the agent.
  </commentary>
  </example>

model: inherit
color: blue
tools: [List only needed tools]
---

[Persona statement - WHO this agent is, with SPECIFIC expertise]

## Skills to Load

[Reference to skill loading protocol and which skills to load]

## Core Principles

[3-5 key behavioral principles]

## Workflow

[Step-by-step process with verification loops]

### Phase 1: Analysis
[Investigation before action]

### Phase 2: Plan
[Present plan for approval]

### Phase 3: Implement + Verify
[Make changes, test immediately, iterate]

## Context Awareness

[When to delegate or reduce context]

**Context thresholds:**
- <50K tokens: Full context OK
- 50-100K tokens: Reduce (focused files, search first)
- >100K tokens: Delegate (sub-agents, Architect-Editor)

## Confidence Indicator

[How to express certainty levels with action rules]

| Level | Meaning | Action |
|-------|---------|--------|
| HIGH | Verified evidence | Implement after approval |
| MEDIUM | Pattern match | Confirm before implementing |
| LOW | Heuristic | Discuss only, never implement |

## Tool Availability

[Fallback guidance if tools missing]

## What I DON'T Do

[Explicit boundaries and limitations - CRITICAL for specialization]
```

### Agent Template Field Guide

| Section | Purpose | Required |
|---------|---------|----------|
| **Frontmatter** | Metadata for agent selection | Yes |
| **Persona** | Specific expertise (not "helpful assistant") | Yes |
| **Skills to Load** | Which skills to read and when | If using skills |
| **Core Principles** | 3-5 key behavioral rules | Yes |
| **Workflow** | Phased process with verification | Yes |
| **Context Awareness** | When to delegate/reduce | Recommended |
| **Confidence Indicator** | Certainty levels + actions | Recommended |
| **Tool Availability** | Fallbacks if tools missing | Recommended |
| **What I DON'T Do** | Explicit boundaries | Yes |

### Specialization Checklist

Before finalizing an agent, verify:
- [ ] Persona has specific expertise (not generic)
- [ ] Single clear purpose (not kitchen-sink)
- [ ] "What I DON'T Do" excludes other agents' responsibilities
- [ ] Would splitting into multiple agents improve quality?

---

## Skill Structure Template

```markdown
---
name: skill-name
description: What this skill provides and when to load it.
---

# Skill Title

Brief description of what this skill provides.

## Quick Reference

[Checklist or table for fast lookup - covers 80% of needs]

## Decision Trees

[Visual decision guides for common choices]

## Commands (if applicable)

[Wrapper script documentation]

## References

| Task | Load |
|------|------|
| [Task description] | [reference-path.md](references/reference-path.md) |
```

### Skill Template Field Guide

| Section | Purpose | Required |
|---------|---------|----------|
| **Frontmatter** | Metadata for skill discovery | Yes |
| **Quick Reference** | Fast lookup for common needs | Yes |
| **Decision Trees** | Visual guides for choices | Recommended |
| **Commands** | Wrapper script docs | If applicable |
| **References** | Links to deep-dive content | If has references/ |

---

## Reference File Template

```markdown
# [Topic Name]

[Brief context - what this reference covers]

## Section 1

[Content with concrete examples]

### Subsection (if needed)

[Detailed content]

## Section 2

[Content with concrete examples]

---

## Quick Lookup

[Summary table or checklist for fast reference]
```

### Reference File Guidelines

- **Self-contained**: Works without loading other files
- **Grep-friendly headers**: Use consistent, searchable section names
- **Examples included**: Every pattern has a concrete example
- **Quick lookup**: End with summary table if content is long

---

## Workflow File Template

```markdown
# [Workflow Name] Workflow

> **Trigger:** "[trigger phrases]"

## Purpose

[Single sentence: what this workflow produces]

## Prerequisites

- [Prerequisite 1]
- [Prerequisite 2]

## Interactive Flow

### Phase 1: [Phase Name]

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | [Input name] | [type] | [default or required] | [validation rule] |

**Question Examples:**

Question 1: "[Question text]"
- Header: "[Short header]"
- Options: ["Option 1", "Option 2", "Custom"]

### Phase 2: [Phase Name]

[Logic and decision flow]

### Phase 3: [Phase Name]

[Generation or execution steps]

### Phase 4: Report Success

```
[Success message template with placeholders]

Next steps:
  1. [Action 1]
  2. [Action 2]
```

## Error Handling

| Error | Action |
|-------|--------|
| [Error condition] | [Recovery action] |
```

### Workflow Template Guidelines

- **One question at a time**: Never batch multiple inputs
- **Defaults provided**: Reduce user effort where sensible
- **Validation defined**: Every input has validation rules
- **Clear phases**: Each phase has a single purpose
- **Verification loops**: Test/validate after each change
- **Error recovery**: Every error has a defined action

### Workflow Quality Checklist

- [ ] Has Analysis/Investigation phase before changes
- [ ] Has Plan phase with user approval checkpoint
- [ ] Has verification step after implementation
- [ ] Defines when to ask user vs proceed
- [ ] Includes rollback/undo guidance for failures
