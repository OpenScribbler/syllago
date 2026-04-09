# ExtractSkill Workflow

> **Trigger:** "extract skill", "create skill from", "agent is too big", "refactor into skill", "skill extraction"

## Purpose

Extract domain knowledge from a bloated agent prompt into a reusable skill with proper structure.

## Prerequisites

- Agent prompt with extractable content (>50 lines of static content)
- Understanding of skill structure (SKILL.md + references/)
- Access to write files in `skills/` directory

## Interactive Flow

### Phase 1: Identify Source

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | Source agent | path | (required) | Valid agent file |
| 2 | Extraction reason | choice | (required) | Helps determine scope |
| 3 | Skill name | string | (derived) | Lowercase, hyphenated |
| 4 | Reuse expectation | choice | "single" | Single agent or multiple |

**Question Examples:**

Question 1: "Which agent needs content extracted?"
- Header: "Source Agent"
- Options: [List from `agents/*.md`]

Question 2: "Why does this agent need extraction?"
- Header: "Reason"
- Options: [
    "Agent prompt too long (>150 lines)",
    "Content could be reused by other agents",
    "Domain knowledge mixed with behavior",
    "Static content making agent slow",
    "Other"
  ]

Question 3: "What should we name the new skill?"
- Header: "Skill Name"
- Options: ["[suggested-name] (Recommended)", "Custom name"]
- Note: Suggest based on content type (e.g., security checklist -> "security-standards")

Question 4: "Will other agents use this skill?"
- Header: "Reuse"
- Options: ["Just this agent", "Multiple agents will share"]

### Phase 2: Analyze Content

Read the source agent and categorize all content:

```
1. Read agent file completely
2. Identify each section/block of content
3. Categorize each block:

   KEEP IN AGENT (behavioral):
   - Persona statement
   - Workflow steps
   - Confidence rules
   - Boundary definitions
   - Skill loading instructions

   EXTRACT TO SKILL (domain knowledge):
   - Checklists (>10 items)
   - Reference tables
   - Command documentation
   - Pattern catalogs
   - Code examples (>20 lines)
   - Best practices lists

   UNCERTAIN (discuss):
   - Medium-sized content (25-50 lines)
   - Content that straddles behavior/knowledge
```

**Present extraction plan for approval:**

```markdown
## Extraction Analysis: [agent-name]

**Current prompt size:** [X] lines
**After extraction:** ~[Y] lines (estimated)

### Content to KEEP in Agent

| Section | Lines | Reason |
|---------|-------|--------|
| [Section name] | [N] | [Why it stays] |

### Content to EXTRACT to Skill

| Section | Lines | Destination |
|---------|-------|-------------|
| [Section name] | [N] | SKILL.md quick reference |
| [Section name] | [N] | references/[name].md |

### Uncertain (Need Input)

| Section | Lines | Question |
|---------|-------|----------|
| [Section name] | [N] | [Should this stay or go?] |

---
Proceed with extraction? [Yes/Modify/Cancel]
```

### Phase 3: Design Skill Structure

After approval, design the skill:

```
1. Create skill directory structure:
   skills/[skill-name]/
     SKILL.md            # Entry point
     references/         # Deep-dive content

2. Design SKILL.md (must be <100 lines):
   - Frontmatter (name, description)
   - Quick Reference section (most common lookups)
   - Decision Tree (if applicable)
   - References table (links to deep content)

3. Design reference files:
   - One file per major topic
   - Self-contained (works without SKILL.md)
   - Grep-friendly headers
```

**Skill Structure Preview:**

```markdown
## Proposed Skill Structure

skills/[skill-name]/
  SKILL.md (~[N] lines)
    - Quick Reference: [content summary]
    - Decision Tree: [if applicable]
    - References: [list of reference files]

  references/
    [topic-1].md (~[N] lines)
      - [Section list]
    [topic-2].md (~[N] lines)
      - [Section list]
```

### Phase 4: Extract and Create Files

Execute extraction in this order:

1. **Create skill directory**
   ```bash
   mkdir -p skills/[skill-name]/references
   ```

2. **Create SKILL.md** (entry point)
   - Extract quick reference content
   - Add decision trees if applicable
   - Add references table

3. **Create reference files**
   - One file per major topic
   - Include all detailed content
   - Ensure self-contained

4. **Update source agent**
   - Remove extracted content
   - Add skill loading instructions
   - Keep behavioral content intact

### Phase 5: Validate Extraction

Run validation checks:

```markdown
## Extraction Validation

### SKILL.md Quality
- [ ] Entry point exists: skills/[skill-name]/SKILL.md
- [ ] Under 100 lines: [actual] lines
- [ ] Has Quick Reference section
- [ ] References table links are valid

### Agent Quality
- [ ] Agent prompt is shorter: [before] -> [after] lines
- [ ] Skill loading instructions added
- [ ] No duplicate content between agent and skill
- [ ] Agent workflow still makes sense

### Reference Quality
- [ ] Each reference is self-contained
- [ ] Grep-friendly headers used
- [ ] Examples included

### Issues Found
- [Issue 1] - [Fix]
- [None if clean]
```

### Phase 6: Report Success

```
Skill extraction complete!

New skill: skills/[skill-name]/
  - SKILL.md ([N] lines)
  - references/[file1].md ([N] lines)
  - references/[file2].md ([N] lines)

Updated agent: agents/[agent-name].md
  - Before: [N] lines
  - After: [N] lines
  - Reduction: [X]%

Skill loading added to agent:
  Primary: skills/[skill-name]/SKILL.md
  On-demand: [list of references]

Next steps:
  1. Test agent to ensure it still works correctly
  2. Verify skill loading instructions are clear
  3. Consider if other agents should use this skill
  4. Add to skill as new patterns emerge
```

## Error Handling

| Error | Action |
|-------|--------|
| Agent file not found | List available agents, re-prompt |
| Skill name conflict | Suggest alternative or confirm merge |
| Cannot create directory | Report error, check permissions |
| Content unclear (behavioral vs knowledge) | Ask user to categorize |
| Extraction would leave agent empty | Warn, suggest keeping core workflow |
| Reference file too large (>300 lines) | Split into multiple references |

## Extraction Decision Tree

Use this to categorize each content block:

```
Content Block
    |
    v
Is it a persona/identity statement?
    |-- Yes --> KEEP IN AGENT
    |-- No
        v
Is it workflow/process steps?
    |-- Yes --> KEEP IN AGENT
    |-- No
        v
Is it confidence/boundary rules?
    |-- Yes --> KEEP IN AGENT
    |-- No
        v
Is it >50 lines of static content?
    |-- Yes --> EXTRACT TO SKILL
    |-- No
        v
Could multiple agents use this?
    |-- Yes --> EXTRACT TO SKILL
    |-- No
        v
Is it domain knowledge (not behavior)?
    |-- Yes --> EXTRACT TO SKILL
    |-- No --> KEEP IN AGENT
```

## Content Placement Guide

### Goes in SKILL.md (Quick Reference)

- Condensed checklist (top 10 items)
- Command quick reference (1-2 lines each)
- Decision tree diagrams
- Links to references

### Goes in references/

- Full checklists (>10 items)
- Detailed command documentation
- Pattern catalogs with examples
- Code snippets (>10 lines)
- Configuration references

### Stays in Agent

- "You are..." persona statement
- "## Workflow" section
- "## Confidence Indicator" rules
- "## What I DON'T Do" section
- Skill loading instructions

## Quality Checks

Before completing extraction, verify:

1. **80/20 Rule**: Quick reference in SKILL.md handles 80% of lookups
2. **Independence**: Each reference file works without others
3. **No Duplication**: Content exists in exactly one place
4. **Clear Loading**: Agent specifies when to load each reference
5. **Behavior Intact**: Agent's workflow and boundaries unchanged
