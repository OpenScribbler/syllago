# CreateAgent Workflow

> **Trigger:** "create agent", "new agent", "build agent", "design agent"

## Purpose

Interactively design and create a new Claude Code agent with proper structure, skills, and patterns.

## Prerequisites

- Clear understanding of what the agent should do
- Access to write files in `agents/` directory
- Knowledge of existing skills that might be relevant

## Interactive Flow

### Phase 1: Understand Requirements

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | Agent purpose | text | (required) | Non-empty, describes a task |
| 2 | Agent name | string | (derived from purpose) | Lowercase, hyphenated |
| 3 | Primary expertise | choice | (required) | Select from common domains |
| 4 | Read-only or read-write | choice | "read-only" | Affects tool selection |
| 5 | Existing skills to use | multi-select | [] | Valid skill paths |
| 6 | New skill needed | boolean | false | Yes/No |

**Question Examples:**

Question 1: "What should this agent do? Describe its primary task and expertise."
- Header: "Agent Purpose"
- Options: [Free text input required]

Question 2: "What should we name this agent?"
- Header: "Agent Name"
- Options: ["[suggested-name] (Recommended)", "Custom name"]
- Note: Suggest based on purpose (e.g., "terraform security review" -> "terraform-security-reviewer")

Question 3: "What is the agent's primary domain?"
- Header: "Domain"
- Options: ["Code Review", "Development", "Security", "Infrastructure", "Documentation", "Testing", "Other"]

Question 4: "Should this agent be able to modify files?"
- Header: "Permissions"
- Options: ["Read-only (safer, for review/analysis)", "Read-write (for code generation/fixes)"]

Question 5: "Which existing skills should this agent use?"
- Header: "Skills"
- Options: [List discovered skills from `skills/*/SKILL.md`]
- Note: Run `ls skills/*/SKILL.md` to discover available skills

Question 6: "Does this agent need a new skill that doesn't exist yet?"
- Header: "New Skill"
- Options: ["No, existing skills are sufficient", "Yes, will need new skill"]

### Phase 2: Design Agent Structure

Based on collected inputs, design the agent:

```
1. Determine persona statement
   - Extract expertise from purpose
   - Define tone and approach

2. Select tools based on permissions:
   - Read-only: Bash, Glob, Grep, Read, WebFetch, WebSearch
   - Read-write: Add Edit, Write, TodoWrite

3. Map skills to load:
   - Primary skill (always load)
   - On-demand references (load based on task)

4. Draft core principles (3-5):
   - Extract from purpose and domain
   - Include common patterns for domain

5. Design workflow with verification:
   - Phase 1: Analysis/Investigation
   - Phase 2: Plan (present for approval)
   - Phase 3: Implement + Verify (test after each change)
   - For complex agents: reference to Workflows/ files

6. Add context awareness:
   - When to delegate to sub-agents (>50K tokens)
   - File loading strategy (search before read)
   - Context reduction guidance

7. Define boundaries (What I DON'T Do):
   - Inverse of capabilities
   - Common mistakes to avoid
```

**Present design for approval before generating:**

```markdown
## Proposed Agent Design

**Name:** [agent-name]
**Purpose:** [purpose statement]
**Permissions:** [read-only/read-write]

### Persona
[Draft persona statement]

### Tools
[List of tools]

### Skills
- Primary: [skill path]
- On-demand: [reference paths]

### Core Principles
1. [Principle 1]
2. [Principle 2]
3. [Principle 3]

### Boundaries
- [What it won't do 1]
- [What it won't do 2]

---
Proceed with agent generation? [Yes/Modify/Cancel]
```

### Phase 3: Extract Content to Skills FIRST

**Before generating the agent, identify and extract reusable content.**

Content that MUST go in skills (not agents):
| Content Type | Example | Extract To |
|--------------|---------|------------|
| Reference tables | Command syntax, options | `SKILL.md` or `references/*.md` |
| Code examples | Templates, patterns | `references/*.md` |
| Domain knowledge | Best practices, anti-patterns | `references/*.md` |
| Type/audience tables | Doc types, diagram types | `SKILL.md` |
| Checklists | Security, quality, review | `references/*.md` |
| Framework mappings | CMMC, NIST, OWASP | `references/*.md` |

**Extraction decision tree:**
```
Is this content reusable by other agents?
  YES → Extract to skill
  NO → Is it domain knowledge (not behavior)?
    YES → Extract to skill
    NO → Keep in agent
```

**Target agent size: <200 lines**
- If estimated size >200 lines, MUST extract before creating agent
- Create skill/references FIRST, then create agent that references them

### Phase 4: Generate Agent File

After extraction (if needed), generate the agent:

1. **Create agent file**: `agents/[agent-name].md`
2. **Use template**: Load `references/templates.md` for structure
3. **Include examples**: Add 2-3 description examples showing trigger patterns
4. **Set color**: Choose appropriate color (cyan for analysis, green for generation, etc.)

**File Generation Order:**
1. Write frontmatter (name, description with examples, model, color, tools)
2. Write persona statement
3. Write Skills to Load section
4. Write Core Principles
5. Write Workflow
6. Write Confidence Indicator (if applicable)
7. Write Tool Availability fallback
8. Write What I DON'T Do

### Phase 5: Create New Skill (if needed)

If Phase 1 indicated new skill needed, or Phase 3 identified content to extract:

1. **Trigger ExtractSkill workflow** or create minimal skill structure:
   ```
   skills/[skill-name]/
     SKILL.md           # Entry point with quick reference
     references/        # (create later as content develops)
   ```

2. **Update agent** to reference new skill

### Phase 6: Validate and Report Success

**Pre-delivery validation (6 dimensions):**
- [ ] **Clarity**: Instructions are unambiguous, no vague terms
- [ ] **Specialization**: Single focused purpose, clear boundaries
- [ ] **Workflow Quality**: Has plan-first workflow with verification loops
- [ ] **Context Awareness**: Has delegation guidance and file loading strategy
- [ ] **Safety**: Has "What I DON'T Do" section and confidence indicator
- [ ] **Efficiency**: Agent is <200 lines, knowledge in skills not embedded

**Content validation:**
- [ ] No reference tables embedded in agent
- [ ] No code examples embedded in agent
- [ ] No >50 line blocks of domain knowledge
- [ ] Skills created for reusable content

If validation fails, return to Phase 3 and extract more content.

```
Agent created successfully!

File: agents/[agent-name].md

Agent summary:
  - Name: [agent-name]
  - Purpose: [purpose]
  - Tools: [tool list]
  - Skills: [skill list]

Next steps:
  1. Review the generated agent file
  2. Test with sample prompts
  3. Iterate on core principles and workflow
  4. Add skill content as patterns emerge

To test: Ask Claude to use the [agent-name] agent
```

## Error Handling

| Error | Action |
|-------|--------|
| Agent name already exists | Prompt for different name or confirm overwrite |
| Invalid skill path | Show available skills, re-prompt |
| Cannot create file | Report error, suggest checking permissions |
| Empty purpose | Re-prompt with examples of good purpose statements |
| Purpose too vague | Ask clarifying questions about specific tasks |

## Validation Rules

### Agent Name
- Lowercase letters, numbers, hyphens only
- No spaces or underscores
- 3-30 characters
- Must not conflict with existing agent

### Purpose Statement
- At least 10 words
- Describes a specific task (not "help with things")
- Includes target domain or technology

### Tool Selection
- Read-only agents: Never include Edit, Write
- Read-write agents: Always include Read (required for Edit)
- All agents: Include Bash for CLI operations

## Design Principles Applied

This workflow applies these agent design patterns (aligned with 6 evaluation dimensions):

1. **Clarity**: Unambiguous instructions, concrete examples
2. **Specialization**: Single focused purpose, persona matches task
3. **Workflow Quality**: Plan-first with verification loops
4. **Context Awareness**: Delegation guidance, file loading strategy
5. **Safety**: Explicit boundaries, confidence indicator with action rules
6. **Efficiency**: <200 lines, knowledge in skills not agent
