# ADR Templates

Architecture Decision Record format following [adr.github.io](https://adr.github.io/).

## Standard ADR Format

```markdown
# ADR-NNN: Title (Present Tense Imperative)

## Status

[Proposed | Accepted | Deprecated | Superseded by ADR-XXX]

## Context

What is the issue that we're seeing that motivates this decision?

Include:
- Business context and priorities
- Technical constraints
- Team considerations
- Timeline pressures

## Decision

What is the change that we're proposing and/or doing?

State the decision in active voice: "We will..."

## Consequences

What becomes easier or more difficult because of this change?

### Positive
- [Benefit 1]
- [Benefit 2]

### Negative
- [Trade-off 1]
- [Trade-off 2]

### Neutral
- [Side effect that is neither positive nor negative]
```

## Section Guidance

### Title

- Use present tense imperative: "Use PostgreSQL for user data"
- Be specific: "Use PostgreSQL" not "Choose a database"
- One decision per ADR

### Status

| Status | When to Use |
|--------|-------------|
| **Proposed** | Under discussion, not yet approved |
| **Accepted** | Approved and in effect |
| **Deprecated** | No longer applies, but not replaced |
| **Superseded** | Replaced by another ADR (link to it) |

### Context

Answer these questions:
- What problem are we solving?
- What constraints exist?
- What forces are at play?
- Why is a decision needed now?

**DO**: Include business context, not just technical
**DON'T**: Include the decision in the context section

### Decision

- Start with "We will..."
- Be specific and actionable
- Include scope boundaries if needed

**Example:**
> We will use PostgreSQL 15+ for all user-related data storage. This applies to the user service and authentication service. Analytics data remains in ClickHouse.

### Consequences

Be honest about trade-offs. Every decision has downsides.

**Positive**: What becomes easier, faster, cheaper, more reliable?
**Negative**: What becomes harder, slower, more expensive, riskier?
**Neutral**: Side effects that are neither good nor bad

## Extended ADR Format (MADR Style)

For complex decisions requiring more structure:

```markdown
# ADR-NNN: Title

## Status

[Status]

## Context

[Context description]

## Decision Drivers

- [Driver 1: e.g., "Must support 10K requests/second"]
- [Driver 2: e.g., "Team has PostgreSQL expertise"]
- [Driver 3: e.g., "Budget constraint of $X/month"]

## Considered Options

### Option 1: [Name]

**Description**: [Brief description]

**Pros**:
- [Pro 1]
- [Pro 2]

**Cons**:
- [Con 1]
- [Con 2]

### Option 2: [Name]

[Same structure]

### Option 3: [Name]

[Same structure]

## Decision

We chose **Option N** because [primary reason].

[Detailed explanation of why this option best fits the decision drivers]

## Consequences

### Positive
- [Consequence 1]

### Negative
- [Consequence 2]

### Follow-up Actions
- [ ] [Action needed to implement this decision]
- [ ] [Related ADR to write]
```

## Example ADR

```markdown
# ADR-001: Use PostgreSQL for User Data Storage

## Status

Accepted

## Context

Our user service needs persistent storage for user profiles, preferences, and authentication data. We currently have no database and need to select one.

Key considerations:
- Data is relational (users have roles, roles have permissions)
- Strong consistency required for authentication
- Team has SQL experience, limited NoSQL experience
- Expected scale: 100K users, 1K concurrent connections
- Must support ACID transactions for payment integration

## Decision Drivers

- Must support relational data with joins
- Must provide strong consistency guarantees
- Must scale to 100K users with growth runway
- Should leverage existing team expertise
- Should have mature tooling and community support

## Considered Options

### Option 1: PostgreSQL

**Pros**:
- Excellent relational model with advanced features
- Strong consistency, ACID compliant
- Team has experience
- Mature ecosystem, excellent tooling
- Free, with managed options available

**Cons**:
- Horizontal scaling requires more effort than NoSQL
- Schema migrations need careful planning

### Option 2: MongoDB

**Pros**:
- Flexible schema, easy to evolve
- Built-in horizontal scaling
- Good for document-oriented data

**Cons**:
- Team lacks experience
- Weaker consistency guarantees by default
- Relational patterns require denormalization

### Option 3: CockroachDB

**Pros**:
- PostgreSQL compatible
- Built-in horizontal scaling
- Strong consistency

**Cons**:
- Higher operational complexity
- Team unfamiliar with distributed DB operations
- Overkill for current scale

## Decision

We will use **PostgreSQL 15+** for user data storage.

PostgreSQL best fits our decision drivers: relational data model, strong consistency for auth, and team expertise. While horizontal scaling is harder than CockroachDB, our expected scale (100K users) is well within PostgreSQL's single-node capacity with room to grow. If we exceed this, we can revisit with ADR-XXX.

## Consequences

### Positive
- Team can be productive immediately
- Proven technology with predictable behavior
- Strong ecosystem for monitoring, backups, tooling
- ACID compliance simplifies payment integration

### Negative
- Will need to plan for scaling if we 10x user base
- Schema migrations require coordination

### Follow-up Actions
- [ ] Set up PostgreSQL with replication for HA
- [ ] Establish migration workflow with versioned migrations
- [ ] Document connection pooling strategy (ADR-002)
```

## ADR Index Pattern

When creating multiple ADRs, always produce an index file at `docs/adr/README.md`:

```markdown
# Architecture Decision Records

| # | Decision | Status | Date |
|---|----------|--------|------|
| [ADR-0001](0001-title.md) | Brief 1-line summary | Accepted | YYYY-MM-DD |
| [ADR-0002](0002-title.md) | Brief 1-line summary | Proposed | YYYY-MM-DD |
```

This index enables agents and humans to scan decisions and load individual ADRs on demand.

## ADR Naming Convention

```
docs/adr/
  0001-record-architecture-decisions.md
  0002-use-postgresql-for-user-data.md
  0003-adopt-event-driven-architecture.md
```

- Four-digit prefix for ordering
- Lowercase with hyphens
- Verb-object format when possible

## When to Write an ADR

Write an ADR when:
- Choosing between technologies
- Selecting an architecture pattern
- Making a significant trade-off
- Establishing a standard or convention
- Changing a previous decision

Don't write an ADR for:
- Implementation details that don't affect other components
- Obvious choices with no alternatives
- Temporary solutions (document differently)
