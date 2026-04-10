# Complexity Tradeoffs

## Core Tension

Every observability feature adds operational complexity, dependency surface, and cognitive load. The right amount depends on how much you stand to lose from an undetected failure.

## Tier Definitions

### Tier 1: Demo / Prototype

**Profile**: Hackathon, proof-of-concept, local dev, internal tool with no uptime expectation.

**Decision signals**: No users depending on it, no SLO, no on-call, disposable.

| Signal | Requirement |
|--------|-------------|
| Logs | stdout, unstructured OK |
| Metrics | None |
| Traces | None |
| Alerts | None |
| Dashboards | None |
| Health | `/healthz` endpoint returning 200 |

**Effort**: ~30 minutes. Do not add more.

### Tier 2: Small Production

**Profile**: <3 replicas, no formal SLO, team checks dashboards manually, limited on-call.

**Decision signals**: Real users but low blast radius, small team, simple architecture.

| Signal | Requirement |
|--------|-------------|
| Logs | JSON format, correlation_id, service_name |
| Metrics | RED metrics (rate, errors, duration) per entry point |
| Traces | None (add if debugging cross-service issues) |
| Alerts | Error rate > threshold, latency p99 > threshold |
| Dashboards | 1 service overview dashboard |
| Health | `/healthz` + `/readyz` endpoints |

**Effort**: ~1 day.

### Tier 3: Medium Production

**Profile**: SLOs defined, on-call rotation, multi-service architecture, centralized logging.

**Decision signals**: SLO commitments, incident response process, >3 services interacting.

| Signal | Requirement |
|--------|-------------|
| Logs | Centralized, JSON, trace_id/span_id injected, canonical log lines |
| Metrics | RED + custom business metrics, histogram buckets aligned to SLO |
| Traces | OTel auto-instrumentation + manual spans for business logic |
| Alerts | Multi-window burn-rate on SLOs, runbook links |
| Dashboards | Overview -> drilldown hierarchy, deployment annotations |
| Health | Liveness + readiness + startup probes |

**Effort**: ~1 week initial setup.

### Tier 4: Large / Complex

**Profile**: High-traffic, strict SLOs, platform team, regulatory requirements.

**Decision signals**: >1000 RPS, multi-region, error budgets tracked, compliance needs.

| Signal | Requirement |
|--------|-------------|
| Logs | All of Tier 3 + log sampling for high-volume paths |
| Metrics | Recording rules for SLI expressions, exemplars linking metrics to traces |
| Traces | Tail-based sampling via Collector, custom span attributes for business context |
| Alerts | Error budgets with burn-rate, low-traffic guards, automated escalation |
| Dashboards | Dashboard-as-code (grafonnet/Terraform), PR-reviewed |
| Health | Deep health checks with dependency status |

**Effort**: ~2-4 weeks initial, ongoing maintenance.

## Tradeoff Decision Table

| Addition | Cost | Justified When |
|----------|------|----------------|
| JSON logging | Minimal (library swap) | Always (Tier 2+) |
| RED metrics | Low (3 metrics per endpoint) | Any production service |
| OTel auto-instrumentation | Medium (SDK + Collector) | Multi-service debugging needed |
| Manual spans | Medium (dev time per feature) | Business logic visibility needed |
| Burn-rate alerting | Medium (SLO definition + math) | SLO commitments exist |
| Dashboard-as-code | High (tooling + review process) | 3+ dashboards to maintain |
| Tail-based sampling | High (Collector state + memory) | >1000 RPS or cost pressure |

## Common Mistakes by Tier

- **Tier 1 doing Tier 3**: Adding OTel to a hackathon project. Ship the demo first.
- **Tier 2 skipping alerts**: Having a dashboard nobody watches. Add 2 alerts minimum.
- **Tier 3 without runbooks**: Burn-rate alerts fire at 3am with no remediation guide. Write runbooks before enabling alerts.
- **Tier 4 without error budgets**: Full instrumentation but no process to act on SLO violations. Define error budget policies.
