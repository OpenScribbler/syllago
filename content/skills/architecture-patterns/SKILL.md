---
name: architecture-patterns
description: Architecture design patterns, ADR templates, and security framework mappings. Use when designing systems, documenting decisions, or mapping compliance frameworks.
---

# Architecture Patterns

Patterns, templates, and references for system architecture and design documentation.

## Workflows

| Trigger | Workflow |
|---------|----------|
| "create ADR", "new ADR", "document architecture decision" | [Workflows/CreateADR.md](Workflows/CreateADR.md) |

## References

| Task | Load |
|------|------|
| Selecting architecture patterns + design document structure | [design-patterns.md](references/design-patterns.md) |
| Mapping to CMMC/NIST/OWASP/MITRE ATT&CK | [security-frameworks.md](references/security-frameworks.md) |
| Writing an ADR (standard + extended templates, examples) | [adr-templates.md](references/adr-templates.md) |
| Gathering requirements (functional, non-functional, constraints) | [requirements-checklist.md](references/requirements-checklist.md) |
| Designing security controls (authn, authz, secrets, zero trust) | [security-architecture.md](references/security-architecture.md) |
| Integration design (API gateway, events, sagas, multi-tenancy) | [integration-patterns.md](references/integration-patterns.md) |

## Decision Tree: What Artifact to Create

```
Need to document something
        |
        v
Is it a specific decision with alternatives?
        |-- Yes --> Create ADR
        |-- No
            v
Is it a system overview or integration design?
        |-- Yes --> Create Design Document with diagrams
        |-- No
            v
Is it a single flow or interaction?
        |-- Yes --> Create Sequence Diagram
        |-- No --> Describe in prose
```

## Diagram Type Selection

| Need to Show | Diagram Type | Mermaid Keyword |
|--------------|--------------|-----------------|
| System context, external actors | C4 Context | `C4Context` |
| Internal containers/services | C4 Container | `C4Container` |
| Component internals | C4 Component | `C4Component` |
| Infrastructure layout | C4 Deployment | `C4Deployment` |
| Request/response flows | Sequence | `sequenceDiagram` |

## ADR Status Values

| Status | Meaning |
|--------|---------|
| Proposed | Under discussion |
| Accepted | Approved and in effect |
| Deprecated | Superseded by another ADR |
| Superseded | Replaced (link to replacement) |

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

## Output Artifacts Checklist

For a complete system design, produce:

- [ ] **ADRs** - One per significant decision
- [ ] **Context Diagram** - System boundary, external actors
- [ ] **Container Diagram** - Internal services/components
- [ ] **Sequence Diagrams** - Critical flows (auth, data processing)
- [ ] **Deployment Diagram** - If infrastructure design included

## Cross-References

- For threat modeling: Collaborate with **senior-security-engineer**
- For implementation: Hand off to **senior-engineer**
- For security scanning: Load `skills/security-audit/SKILL.md`
