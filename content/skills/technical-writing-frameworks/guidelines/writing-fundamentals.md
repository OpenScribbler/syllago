# Writing Fundamentals

Core documentation principles, types, and quality standards. Merged from documentation-patterns skill.

## Core Principles

- **Accuracy over completeness**: Wrong docs are worse than missing docs. Verify claims against code, test examples, mark uncertain sections "TODO: verify"
- **Show, don't just tell**: Concrete examples over abstract descriptions. "The service supports OAuth2 authentication" is weak; show the curl command
- **Document the why**: Configuration options and decisions need context, not just syntax. Include defaults, trade-offs, recommendations
- **Know your audience**: Every document targets a specific audience (see Audience Targeting below)

## Documentation Types

| Type | When to Create | Key Sections |
|------|----------------|--------------|
| **README** | Every project | Overview, Install, Quick Start, Usage |
| **API Reference** | Services with APIs | Endpoints, Auth, Examples, Errors |
| **Architecture Doc** | Complex systems | Overview, Components, Data Flow, Decisions |
| **Configuration Ref** | Configurable apps | Options, Defaults, Examples, Environment |
| **Security Doc** | Security-sensitive | Threat Model, Controls, Considerations |
| **Contributing Guide** | Open/team projects | Setup, Workflow, Standards, Testing |
| **ADR** | Significant decisions | Context, Decision, Consequences |
| **Runbook** | Production systems | Alerts, Diagnosis, Remediation |

## Audience Targeting

| Audience | Focus On | Avoid |
|----------|----------|-------|
| **Developers** | API usage, code examples, integration | Conceptual overviews without specifics |
| **Operators** | Deployment, configuration, monitoring | Implementation details |
| **End Users** | Features, workflows, troubleshooting | Technical internals |
| **Contributors** | Architecture, patterns, testing | User-facing documentation |

## Decision Tree: What to Document

```
Does README exist and match current code?
  No --> Update/create README first
  Yes --> Are there undocumented APIs?
    Yes --> Create API reference
    No --> Are there complex systems without diagrams?
      Yes --> Create architecture doc
      No --> Review for gaps
```

## Diagram Selection

| Need to Show | Diagram Type |
|--------------|--------------|
| System context, integrations | C4 Context |
| Internal components | C4 Container |
| Request/response flows | Sequence diagram |
| Decision logic | Flowchart |
| State transitions | State diagram |
| Data relationships | Entity-relationship |

## Documentation Quality Checklist

**Accuracy**: Code examples syntactically correct, commands tested, version numbers current, links resolve, config options match behavior

**Completeness**: Install/setup complete, all env vars documented, error conditions described, common use cases covered

**Clarity**: Target audience clear, technical terms defined or linked, steps in logical order, headings describe content

**Maintainability**: No hardcoded changing values, version-specific content marked, relative paths for links, DRY content
