---
name: observability-patterns
description: Application-level observability patterns for OpenTelemetry, structured logging, Prometheus metrics, alerting, and Grafana dashboards. Scales recommendations to project maturity (demo to production HA). Language-agnostic hub -- language skills handle SDK initialization code.
---

# Application Observability Patterns

Universal patterns for instrumenting applications with logs, metrics, and traces. Match complexity to project maturity -- demos get minimal instrumentation, production services get full three-pillar coverage.

## Key Constraint

**Match observability complexity to project maturity.** Over-instrumenting a demo wastes effort; under-instrumenting production causes outages. Start with the [ObservabilityDesign workflow](Workflows/ObservabilityDesign.md) to classify your project tier.

## Maturity Tier Quick Reference

| Tier | Profile | What You Need |
|------|---------|---------------|
| 1 - Demo | Prototype, hackathon, local dev | stdout logs, /healthz, nothing else |
| 2 - Small prod | <3 replicas, no SLO, no on-call | JSON logs + correlation IDs, RED metrics, 1 dashboard, 2 alerts |
| 3 - Medium prod | SLOs defined, on-call rotation, multi-service | Centralized logs + trace_id, OTel auto-instrumentation, SLO burn-rate alerting, dashboard hierarchy, runbooks |
| 4 - Large/complex | High-traffic, strict SLOs, platform team | Full three pillars, tail-based sampling, error budgets, recording rules, dashboard-as-code |

## Production Readiness Checklist

| Signal | Requirement |
|--------|-------------|
| Logs | JSON format, trace_id/span_id injected, required fields present, log levels correct |
| Metrics | RED metrics per entry point, cardinality bounded, histogram buckets include SLO thresholds |
| Traces | OTel auto + manual instrumentation, Collector deployed, sampling configured |
| Alerts | Symptom-based (RED not USE), burn-rate for SLOs, runbook link on every alert |
| Dashboards | Overview dashboard exists, template variables, deployment annotations |

## Anti-Patterns

- **Over-instrumenting demos**: Adding OTel, Prometheus, and Grafana to a prototype. Use Tier 1 -- stdout logs and /healthz only.
- **Cardinality explosion**: Using user IDs, request IDs, or unbounded values as metric labels. Use bounded categories only.
- **Cause-based alerting**: Alerting on CPU/memory instead of request errors and latency. Alert on symptoms (RED), investigate causes.
- **Summary in replicated services**: Prometheus Summary cannot be aggregated across instances. Use Histogram for replicated services.
- **Missing trace_id in logs**: Logs without trace context cannot be correlated with traces. Inject trace_id/span_id from OTel active span.
- **Alerts without runbooks**: Every alert must link to a runbook. Alerts without runbooks cause panic during incidents.

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| Choosing instrumentation level, tier classification, cost/benefit | [complexity-tradeoffs.md](references/complexity-tradeoffs.md) |
| OTel SDK rules, Collector decisions, sampling, propagation | [opentelemetry.md](references/opentelemetry.md) |
| JSON logging, required fields, canonical log lines, trace correlation | [structured-logging.md](references/structured-logging.md) |
| Prometheus naming, cardinality, histograms, RED metrics, recording rules | [metrics-design.md](references/metrics-design.md) |
| Alert design, burn-rate, severity tiers, runbooks, low-traffic guards | [alerting.md](references/alerting.md) |
| Panel layout, template variables, annotations, dashboard-as-code | [dashboards.md](references/dashboards.md) |

## Workflows

| Trigger | Workflow |
|---------|----------|
| "add observability", "instrument service", "set up monitoring", "observability plan" | [ObservabilityDesign](Workflows/ObservabilityDesign.md) |

## Related Skills

- **Language SDK setup**: Load [go-patterns](../go-patterns/SKILL.md), [python-patterns](../python-patterns/SKILL.md), [javascript-patterns](../javascript-patterns/SKILL.md), or [rust-patterns](../rust-patterns/SKILL.md) for language-specific OTel SDK initialization and structured logging library setup
- **K8s infra observability**: Load [kubernetes-patterns/references/observability.md](../kubernetes-patterns/references/observability.md) for Collector DaemonSets, ServiceMonitors, scrape configs, logging sidecars
- **CI/CD integration**: Load [cicd-patterns](../cicd-patterns/SKILL.md) for pipeline-level metrics and deployment annotations
- **Security audit**: Load [security-audit](../security-audit/SKILL.md) for log redaction and sensitive data in traces
