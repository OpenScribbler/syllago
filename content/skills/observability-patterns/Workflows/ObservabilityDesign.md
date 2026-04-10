# ObservabilityDesign Workflow

> **Trigger:** "add observability", "instrument service", "set up monitoring", "observability plan", "add metrics and logging"

## Purpose

Interactively classify a project's observability maturity tier and generate a tailored instrumentation plan. Prevents over-instrumenting demos and under-instrumenting production services.

## Prerequisites

- A service or project to instrument
- Knowledge of the service's deployment context
- Load [references/complexity-tradeoffs.md](../references/complexity-tradeoffs.md) for tier definitions

## Interactive Flow

### Phase 1: Project Classification

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Purpose |
|-------|-------|------|---------|
| 1 | Service name and purpose | text | Identify the service |
| 2 | Lifecycle stage | choice | Demo/prototype, Early production, Established production, High-scale production |
| 3 | SLO requirements | choice | None, Informal ("should be fast"), Formal (defined targets), Strict (error budgets tracked) |
| 4 | On-call rotation | choice | None, Informal (team checks Slack), Formal (PagerDuty/Opsgenie rotation) |
| 5 | Replica count and architecture | text | Single instance, <3 replicas, 3+ replicas, multi-service |
| 6 | Current logging/monitoring stack | text | What exists today (stdout, ELK, Prometheus, none) |
| 7 | Primary language | choice | Go, Python, Node.js, Rust, Other |

**Question Examples:**

Question 1: "What service are we instrumenting?"
- Header: "Service Identity"
- Note: Include the service name and a one-line description of what it does.

Question 2: "What is the lifecycle stage of this service?"
- Header: "Lifecycle Stage"
- Options: ["Demo / prototype (no real users)", "Early production (real users, small scale)", "Established production (SLOs, on-call, multi-service)", "High-scale production (>1000 RPS, strict SLOs, platform team)"]

Question 3: "What SLO requirements exist for this service?"
- Header: "SLO Requirements"
- Options: ["None", "Informal - 'it should be fast and reliable'", "Formal - defined availability/latency targets", "Strict - error budgets actively tracked and enforced"]

Question 4: "Is there an on-call rotation for this service?"
- Header: "On-Call"
- Options: ["None", "Informal - team watches a Slack channel", "Formal - PagerDuty/Opsgenie rotation with escalation"]

Question 5: "How is the service deployed?"
- Header: "Deployment Architecture"
- Note: Include replica count, whether it calls other services, and whether other services call it.

Question 6: "What logging and monitoring exists today?"
- Header: "Current Stack"
- Note: Examples: stdout only, JSON logs to ELK, Prometheus + Grafana, none.

Question 7: "What is the primary language?"
- Header: "Language"
- Options: ["Go", "Python", "Node.js / TypeScript", "Rust", "Other"]

### Phase 2: Tier Classification

Map answers to a tier using this decision logic:

| Signal | Tier 1 | Tier 2 | Tier 3 | Tier 4 |
|--------|--------|--------|--------|--------|
| Lifecycle | Demo/prototype | Early production | Established production | High-scale |
| SLO | None | None or informal | Formal | Strict |
| On-call | None | None or informal | Formal | Formal |
| Replicas | 1 | 1-3 | 3+ | 3+, multi-region |
| Architecture | Standalone | Standalone or simple | Multi-service | Complex distributed |

**Classification rules:**
- Tier = minimum tier where ALL signals match or exceed that tier's column
- If signals are mixed (e.g., formal SLO but no on-call), use the LOWER tier and flag the gap
- When in doubt, recommend the lower tier with an upgrade path

**Present classification for confirmation:**

```markdown
## Tier Classification

**Service**: [name]
**Recommended Tier**: Tier [N] - [label]

### Signal Assessment
| Signal | Value | Maps To |
|--------|-------|---------|
| Lifecycle | [answer] | Tier [N] |
| SLO | [answer] | Tier [N] |
| On-call | [answer] | Tier [N] |
| Replicas | [answer] | Tier [N] |

### Gaps Identified
- [Any signal mismatches, e.g., "Formal SLO but no on-call -- consider establishing on-call before Tier 3 alerting"]

---
Does this classification look correct? [Confirm / Adjust tier / Provide more context]
```

### Phase 3: Instrumentation Plan Generation

Based on the confirmed tier, generate a plan. Load reference files on-demand as needed.

**Tier 1 Plan** (~30 minutes effort):
- Add `/healthz` endpoint returning 200
- Keep stdout logging as-is
- Explicitly skip: metrics, traces, alerts, dashboards
- Note: "Revisit when the service has real users"

**Tier 2 Plan** (~1 day effort):
- Switch to JSON structured logging (load [structured-logging.md](../references/structured-logging.md))
- Add correlation_id generation at ingress
- Expose RED metrics per entry point (load [metrics-design.md](../references/metrics-design.md))
- Create 1 overview dashboard (load [dashboards.md](../references/dashboards.md))
- Add 2 alerts: error rate, latency (load [alerting.md](../references/alerting.md))
- Add `/healthz` + `/readyz` endpoints

**Tier 3 Plan** (~1 week effort):
- All of Tier 2, plus:
- Deploy OTel Collector (load [opentelemetry.md](../references/opentelemetry.md))
- Add OTel auto-instrumentation for HTTP, DB, queue
- Add manual spans for critical business logic
- Inject trace_id/span_id into all log lines
- Implement canonical log lines for request lifecycle
- Configure head-based sampling with `ParentBased(TraceIdRatioBased(N))`
- Replace static alert thresholds with multi-window burn-rate (load [alerting.md](../references/alerting.md))
- Build dashboard hierarchy: overview -> service drilldown
- Write runbooks for all alerts
- Align histogram buckets with SLO thresholds

**Tier 4 Plan** (~2-4 weeks effort):
- All of Tier 3, plus:
- Configure tail-based sampling in Collector (retain 100% errors)
- Add recording rules for SLI expressions
- Implement error budget tracking and policies
- Add exemplars linking metrics to trace IDs
- Adopt dashboard-as-code (grafonnet or Terraform)
- Add deployment and incident annotations
- Review and tune sampling ratios quarterly

**Present the plan:**

```markdown
## Instrumentation Plan: [Service Name]

**Tier**: [N] - [label]
**Estimated Effort**: [time]
**Language**: [language]

### Checklist

- [ ] [Task 1 with specific guidance]
- [ ] [Task 2 with specific guidance]
...

### Reference Files to Load During Implementation
- [List of relevant reference files]

### What We're Explicitly Skipping (and why)
- [Feature]: Not needed at Tier [N] because [reason]

---
Approve this plan? [Approve / Modify / Change tier]
```

### Phase 4: Language-Specific Routing

After plan approval, route to the appropriate language skill for SDK setup:

| Language | Skill | Key References |
|----------|-------|---------------|
| Go | `skills/go-patterns/SKILL.md` | slog setup, OTel SDK init, Prometheus client |
| Python | `skills/python-patterns/SKILL.md` | structlog setup, OTel SDK init, prometheus-client |
| Node.js | `skills/javascript-patterns/SKILL.md` | pino setup, OTel SDK init, prom-client |
| Rust | `skills/rust-patterns/SKILL.md` | tracing setup, OTel SDK init, prometheus crate |

Present: "For [language]-specific SDK initialization and library setup, load `skills/[language]-patterns/SKILL.md`. The observability plan above defines WHAT to instrument; the language skill defines HOW."

### Phase 5: Output and Delivery

**Final output:**

```markdown
## Observability Design Complete

**Service**: [name]
**Tier**: [N]
**Plan**: [summary]

### Implementation Order
1. [Logging changes first -- lowest risk, immediate value]
2. [Metrics second -- enables alerting]
3. [Alerts third -- requires metrics]
4. [Dashboards fourth -- visualization of existing metrics]
5. [Traces last -- highest complexity, requires Collector]

### Handoff
- Load `skills/[language]-patterns/SKILL.md` for SDK implementation
- Load `skills/kubernetes-patterns/SKILL.md` for Collector deployment (if Tier 3+)
- Implementation agent: **senior-engineer** (app code) or **senior-platform-engineer** (infra)
```

## Error Handling

| Error | Action |
|-------|--------|
| Cannot determine lifecycle stage | Ask clarifying questions about user count and uptime expectations |
| Mixed signals across tiers | Use lower tier, flag gaps, suggest upgrade path |
| No existing monitoring stack | Start from scratch; plan includes all setup steps |
| Language not in routing table | Provide universal guidance, skip SDK-specific routing |
| User wants higher tier than signals suggest | Warn about complexity cost, confirm they accept the overhead |
| User wants lower tier than signals suggest | Warn about risk, document the decision, proceed with lower tier |
