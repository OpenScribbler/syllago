# CreateADR Workflow

> **Trigger:** "create ADR", "new ADR", "document architecture decision", "write ADR", "record decision"

## Purpose

Interactively create an Architecture Decision Record (ADR) with proper structure, options analysis, and consequences documentation.

## Prerequisites

- Clear decision to document (or problem statement to explore)
- Knowledge of existing ADRs in the repository
- Write access to docs/adr/ directory
- Load [references/adr-templates.md](../references/adr-templates.md) for template content

## Interactive Flow

### Phase 1: Context and Problem

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | Decision topic | text | (required) | Non-empty, describes a decision |
| 2 | Problem statement | text | (required) | What we're solving |
| 3 | Constraints | text | (optional) | Technical/business constraints |
| 4 | ADR format | choice | "standard" | Standard / Extended (MADR) |

**Question Examples:**

Question 1: "What decision needs to be documented?"
- Header: "Decision Topic"
- Note: Be specific - "Use PostgreSQL for user data" not "Choose a database"

Question 2: "What problem or need is driving this decision?"
- Header: "Problem Statement"
- Note: Focus on the WHY, not the WHAT

Question 3: "What constraints should guide this decision? (optional)"
- Header: "Constraints"
- Options: [Free text or skip]

Question 4: "What ADR format should we use?"
- Header: "Format"
- Options: ["Standard (simpler, for clear-cut decisions)", "Extended/MADR (detailed options analysis)"]

**Check for existing ADRs:**

```bash
ls docs/adr/*.md 2>/dev/null || echo "No existing ADRs"
```

**Present initial context for confirmation:**

```markdown
## ADR Context Draft

**Number**: ADR-[NNN] (next available)
**Topic**: [topic]

### Context
[Problem statement expanded]

### Constraints
- [Constraint 1]

### Decision Drivers (derived)
- [Driver 1: extracted from problem/constraints]

---
Is this context accurate? [Confirm / Modify / Add constraints]
```

### Phase 2: Options Generation

**For Standard format, collect the decision directly:**

Question: "What is the decision?"
- Header: "Decision"
- Note: State in active voice - "We will use X because Y"

**For Extended format, explore options:**

| Order | Input | Type | Default |
|-------|-------|------|---------|
| 1 | Number of options | choice | 3 (range: 2-5) |
| 2 | Option names | text[] | (required) |
| 3 | For each option: pros | text | (required) |
| 4 | For each option: cons | text | (required) |

Collect option name, pros, and cons for each option. Present all options in a comparison table and ask: "Which option should we select? [Option 1 / Option 2 / ... / Need more analysis]"

### Phase 3: Decision Documentation

**Collect decision rationale:**

Question: "Why did you choose [Selected Option]?"
- Header: "Decision Rationale"
- Note: Connect back to decision drivers and constraints

**Present decision section for confirmation:**

```markdown
## Decision

We will use **[Selected Option]** for [context].

### Rationale
[Expanded rationale connecting to drivers]

### Scope
[What this decision applies to, and what it doesn't]

---
Is the decision statement accurate? [Confirm / Modify]
```

### Phase 4: Consequences

**Collect consequences:**

| Order | Input | Type | Default |
|-------|-------|------|---------|
| 1 | Positive consequences | text | (required) |
| 2 | Negative consequences | text | (required) |
| 3 | Follow-up actions | text | (optional) |
| 4 | Related ADRs | text | (optional) |

Ask what becomes easier (positive), what becomes harder (negative), what follow-up actions are needed, and whether related ADRs should be linked.

Present consequences for validation with: "Are the consequences complete? [Confirm / Add more / Modify]"

### Phase 5: Output Generation

**Determine output path:**

```
Default: docs/adr/[NNNN]-[title-slug].md
```

Use four-digit prefix, lowercase with hyphens, verb-object format. See `references/adr-templates.md` for naming convention details.

**Present full ADR for final approval.** Use the appropriate template (Standard or Extended/MADR) from `references/adr-templates.md`. Set status to "Proposed".

Ask: "Write ADR to file? [Write / Modify / Change path / Cancel]"

**After approval:**

```bash
mkdir -p docs/adr
# Write ADR file content
```

**Update ADR index:**

Check for existing index at `docs/adr/README.md`. If it exists, append a new row to the table. If not, create the index file with the table header and this ADR as the first entry. Use the format from `references/adr-templates.md`'s "ADR Index Pattern" section.

### Phase 6: Completion

Report:
- File path written
- Status: Proposed
- Brief summary (decision, options considered count, key trade-off)
- Next steps: share for review, update status to Accepted after approval, implement follow-up actions

## Error Handling

| Error | Action |
|-------|--------|
| docs/adr/ doesn't exist | Create directory, or ask for alternative path |
| ADR number conflict | Increment to next available number |
| Cannot write file | Ask for alternative path or permissions |
| User cancels mid-flow | Offer to save draft or show ADR content to copy |
| No constraints provided | Generate ADR without Decision Drivers section |
| Topic too vague | Ask clarifying questions, suggest specificity |
