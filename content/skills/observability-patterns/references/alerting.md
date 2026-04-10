# Alerting Patterns

## Core Principle

**Alert on symptoms (RED), not causes (USE).** Users notice errors and latency (symptoms). CPU and memory usage (causes) are diagnostic context, not alerting signals.

- Rule: Every alert must answer "What is the user impact?" If you cannot state the impact, it is a diagnostic metric, not an alert.
- Rule: Start with 2 alerts per service (error rate, latency). Add more only when incident reviews reveal gaps.

## RED Alert Baseline

Minimum 2 alerts per service entry point (Tier 2+):

| Alert | Condition | Severity |
|-------|-----------|----------|
| High error rate | Error ratio > threshold for 5 minutes | Critical |
| High latency | p99 latency > SLO threshold for 5 minutes | Warning |

For Tier 2 (no formal SLO), use static thresholds:
- Error rate: >1% of requests returning 5xx
- Latency: p99 > 2x your typical response time

## Multi-Window Burn-Rate Alerting (Tier 3+)

For services with SLOs, use multi-window burn-rate instead of static thresholds. This alerts based on how fast you are consuming your error budget.

| Window | Burn Rate | Budget Consumed | Action |
|--------|-----------|-----------------|--------|
| 1 hour | 14.4x | 2% of monthly budget | Page immediately |
| 6 hours | 6x | 5% of monthly budget | Page |
| 3 days | 1x | 10% of monthly budget | Ticket |

- Rule: Use short AND long windows together. Short window catches acute spikes; long window catches slow burns.
- Rule: Require both windows to fire before alerting to reduce false positives.

Burn-rate alert structure:

```yaml
groups:
  - name: slo_burn_rate
    rules:
      - alert: HighErrorBudgetBurn
        expr: |
          (
            service:request_errors:ratio_rate1h > (14.4 * 0.001)
            and
            service:request_errors:ratio_rate5m > (14.4 * 0.001)
          )
          or
          (
            service:request_errors:ratio_rate6h > (6 * 0.001)
            and
            service:request_errors:ratio_rate30m > (6 * 0.001)
          )
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "{{ $labels.service }} burning error budget too fast"
          runbook: "https://wiki.example.com/runbooks/high-error-budget-burn"
```

- Gotcha: The `0.001` in the expression is the SLO error rate target (99.9% availability = 0.1% error budget). Adjust to match your SLO.

## Low-Traffic Service Guards

- Rule: For services with <10 RPS, add a minimum absolute error count alongside the rate threshold. A single error in 10 requests = 10% error rate, but one error is not an incident.
- Rule: Use `and http_requests_total > 100` (or similar floor) to suppress alerts during low-traffic periods.
- Rule: Consider using longer evaluation windows (15m instead of 5m) for low-traffic services.

## Alert Fatigue Prevention

- Rule: If an alert fires >3 times per week without action, it is broken. Fix the threshold, fix the underlying issue, or delete the alert.
- Rule: Every alert must be actionable. If the response is "wait and see if it resolves," it should be a dashboard panel, not an alert.
- Rule: Group related alerts. 10 individual pod alerts should be 1 service-level alert.
- Rule: Use `for` duration to avoid transient spikes. Minimum `for: 2m` for critical, `for: 5m` for warning.

## Runbook Requirements

Every alert MUST include a `runbook` annotation linking to a runbook document.

Minimum runbook content:

| Section | Content |
|---------|---------|
| Alert description | What this alert means in plain language |
| Impact | What users experience when this fires |
| Triage steps | 3-5 diagnostic steps to identify root cause |
| Remediation | Common fixes with exact commands |
| Escalation | Who to contact if triage steps fail |

- Rule: Write the runbook BEFORE enabling the alert. An alert without a runbook causes panic at 3am.
- Rule: Review and update runbooks after every incident where the runbook was insufficient.

## Alert Severity Tiers

| Severity | Response Time | Channel | Examples |
|----------|--------------|---------|----------|
| Critical | <15 min | Page on-call | SLO burn >2% in 1hr, service down, data loss |
| Warning | <4 hours | Slack channel | Elevated error rate, approaching SLO budget |
| Info | Next business day | Dashboard / ticket | Certificate expiry in 30d, disk at 70% |

- Rule: Critical alerts must page. If a "critical" alert does not warrant waking someone up, it is Warning severity.
- Rule: Info-level items are not alerts -- they are dashboard panels or automated tickets.
