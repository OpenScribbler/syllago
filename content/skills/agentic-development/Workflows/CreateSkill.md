# CreateSkill Workflow

> **Trigger:** "create skill", "new skill", "build skill", "design skill"

## Purpose

Interactively design and create a new skill from scratch with proper structure, references, and loading protocol.

> **Note:** To extract a skill from an existing agent, use the [ExtractSkill](ExtractSkill.md) workflow instead.

## Prerequisites

- Clear understanding of the domain knowledge to capture
- Access to write files in `skills/` directory
- Knowledge of skill structure (see `skills/LOADING-PROTOCOL.md`)
- Templates reference available (`references/templates.md`)

## Interactive Flow

### Phase 1: Understand Requirements

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | Skill purpose | text | (required) | Non-empty, describes domain knowledge |
| 2 | Skill name | string | (derived from purpose) | Lowercase, hyphenated |
| 3 | Content sources | multi-select | (required) | At least one source |
| 4 | Target agents | multi-select | [] | Which agents will use this |
| 5 | Workflows needed | boolean | false | Yes/No |

**Question Examples:**

Question 1: "What domain knowledge should this skill capture? Describe the expertise area."
- Header: "Purpose"
- Options: [Free text input required]
- Note: Good purposes describe *knowledge*, not *behavior* (e.g., "Go testing patterns" not "Review Go tests")

Question 2: "What should we name this skill?"
- Header: "Skill Name"
- Options: ["[suggested-name] (Recommended)", "Custom name"]
- Note: Suggest based on domain (e.g., "Go testing patterns" -> "go-testing-patterns")

Question 3: "Where will the content come from?"
- Header: "Sources"
- Options: [
    "Existing documentation or references",
    "Code patterns from the codebase",
    "External documentation or standards",
    "Expert knowledge (you'll describe it)"
  ]
- multiSelect: true

Question 4: "Which agents will use this skill?"
- Header: "Agents"
- Options: [List from `agents/*.md`, plus "New agent (not yet created)"]
- multiSelect: true
- Note: Helps design loading instructions

Question 5: "Does this skill need interactive workflows?"
- Header: "Workflows"
- Options: ["No, just reference content (Recommended)", "Yes, needs interactive processes"]

### Phase 2: Research and Gather Content

Based on sources identified in Phase 1:

```
1. Collect content from sources:
   - Read existing docs/references
   - Identify code patterns in codebase
   - Gather external documentation
   - Document expert knowledge

2. Categorize each content item using decision tree:

   Content Item
       |
       v
   Quick lookup needed frequently?
       |-- Yes --> SKILL.md Quick Reference
       |-- No
           v
   Deep reference material?
       |-- Yes --> references/<topic>.md
       |-- No
           v
   Interactive multi-step process?
       |-- Yes --> Workflows/<name>.md
       |-- No --> Assess if it belongs in this skill

3. Group content into logical topics for reference files:
   - Each topic = one reference file
   - Each file should be self-contained
   - Target: 50-300 lines per reference file
```

**Content Categorization Summary:**

```markdown
## Content Inventory

### Quick Reference (SKILL.md)
- [Item]: [Why it's frequent lookup]

### Reference Files
- references/[topic-1].md: [Content summary]
- references/[topic-2].md: [Content summary]

### Workflows (if applicable)
- Workflows/[name].md: [Process description]

### Excluded
- [Item]: [Why it doesn't belong in this skill]
```

### Phase 3: Design Skill Structure

Design the skill and **present for approval before creating files:**

```markdown
## Proposed Skill Design

**Name:** [skill-name]
**Purpose:** [purpose statement]
**Target agents:** [agent list]

### Directory Structure

skills/[skill-name]/
  SKILL.md (~[N] lines)
    - Frontmatter (name, description)
    - Quick Reference: [content summary]
    - Decision Trees: [if applicable]
    - References table: [N] reference files
  references/
    [topic-1].md (~[N] lines) - [summary]
    [topic-2].md (~[N] lines) - [summary]
  Workflows/ (if applicable)
    [workflow].md (~[N] lines) - [summary]

### SKILL.md Outline

1. Frontmatter (name, description)
2. Quick Reference section:
   - [Table or checklist content]
3. Decision Trees:
   - [Decision tree description]
4. References table:
   - [topic-1]: [when to load]
   - [topic-2]: [when to load]

### Loading Instructions (for agents)

Primary: Load SKILL.md for quick reference
On-demand: Load references/[topic].md when [condition]

---
Proceed with skill creation? [Yes/Modify/Cancel]
```

**Validation before approval:**
- [ ] SKILL.md outline is under 100 lines
- [ ] Quick reference covers 80% of common lookups
- [ ] Each reference file is self-contained
- [ ] No content duplicated across files

### Phase 4: Create Files

Execute creation in this order:

**Step 1: Create directory structure**
```bash
mkdir -p skills/[skill-name]/references
# If workflows needed:
mkdir -p skills/[skill-name]/Workflows
```

**Step 2: Create SKILL.md** (entry point — always first)

Use the skill structure template from `references/templates.md`:
- Frontmatter with name and description
- Quick Reference section (most-used content)
- Decision trees if applicable
- References table with loading guidance

**Key constraint:** SKILL.md MUST be under 100 lines.

**Step 3: Create reference files**

For each reference file:
- Self-contained (works without loading SKILL.md)
- Grep-friendly section headers
- Concrete examples included
- 50-300 lines target

**Step 4: Create workflow files** (if applicable)

Use the workflow template from `references/templates.md`:
- 4-6 phases with clear purposes
- AskUserQuestion for inputs (one at a time)
- Approval gates before changes
- Error handling table

### Phase 5: Validate Structure

Run validation checks:

```markdown
## Skill Validation

### Structure
- [ ] Entry point exists: skills/[skill-name]/SKILL.md
- [ ] SKILL.md under 100 lines: [actual] lines
- [ ] Has Quick Reference section
- [ ] References table links are valid

### Token Efficiency
- [ ] Quick reference covers common lookups
- [ ] Deep content in references/ (not SKILL.md)
- [ ] No duplicate content across files

### Reusability
- [ ] No agent-specific assumptions
- [ ] Loading instructions work for any agent
- [ ] Content is domain knowledge (not behavior)

### Content Quality
- [ ] Each reference is self-contained
- [ ] Grep-friendly headers used
- [ ] Examples included in references
- [ ] BAD/GOOD pairs or condensed rule statements where applicable

### Issues Found
- [Issue 1] - [Fix]
- [None if clean]
```

If validation fails, fix issues before proceeding.

### Phase 6: Report Success

```
Skill created successfully!

New skill: skills/[skill-name]/
  - SKILL.md ([N] lines)
  - references/[file1].md ([N] lines)
  - references/[file2].md ([N] lines)
  - Workflows/[file].md ([N] lines) (if applicable)

Agent integration:
  Add to agent's "Skills to Load" section:

  **[skill-name]**: Load `~/.claude/skills/[skill-name]/SKILL.md` for:
  - [What the skill provides]

  References loaded on-demand:
  - references/[topic].md — when [condition]

Next steps:
  1. Review generated skill files
  2. Add skill loading to target agents
  3. Test skill loading from an agent
  4. Update LOADING-PROTOCOL.md if adding new trigger
  5. Iterate on content as patterns emerge
```

## Error Handling

| Error | Action |
|-------|--------|
| Skill name already exists | Prompt for different name or confirm merge |
| Cannot create directory | Report error, check permissions |
| Empty purpose | Re-prompt with examples of good purpose statements |
| Purpose describes behavior not knowledge | Explain difference, re-prompt |
| Content too large for SKILL.md | Move excess to references/ |
| No content sources identified | Suggest sources based on purpose |
| Reference file >300 lines | Split into multiple reference files |

## Validation Rules

### Skill Name
- Lowercase letters, numbers, hyphens only
- No spaces or underscores
- 3-40 characters
- Must not conflict with existing skill

### Purpose Statement
- Describes domain knowledge (not agent behavior)
- At least 10 words
- Includes target domain or technology

### Content Placement Rules

| Content Type | Placement | Size Target |
|--------------|-----------|-------------|
| Frequently looked up | SKILL.md Quick Reference | 5-20 lines |
| Decision guides | SKILL.md Decision Trees | 10-30 lines |
| Reference loading table | SKILL.md References | 5-15 lines |
| Detailed patterns | references/*.md | 50-300 lines each |
| Interactive processes | Workflows/*.md | 100-300 lines each |
| Full checklists (>10 items) | references/*.md | Variable |
| Code examples (>10 lines) | references/*.md | Variable |
| Command documentation | references/*.md or SKILL.md | Depends on length |

### Quality Thresholds

| Metric | Target | Blocker |
|--------|--------|---------|
| SKILL.md lines | <100 | >150 |
| Reference file lines | 50-300 | >400 |
| Quick reference coverage | 80% of lookups | <50% |
| Pattern examples | BAD/GOOD pairs or rule statements | No patterns documented |
