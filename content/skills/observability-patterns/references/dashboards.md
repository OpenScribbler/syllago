# Dashboard Design

## Layout Principles

- Rule: Follow Z-pattern scanning. Place the most critical panels (error rate, latency) in the top-left. Users scan left-to-right, top-to-bottom.
- Rule: Limit overview dashboards to 6-8 panels. More panels means slower load times and cognitive overload.
- Rule: Use consistent time ranges across panels. Default to `$__range` or a shared time picker.
- Rule: Group panels by signal type in rows: request metrics row, error metrics row, resource metrics row.

## Standard Panel Types

| Panel | Metric | Visualization |
|-------|--------|---------------|
| Request rate | `rate(requests_total[5m])` | Time series (lines) |
| Error rate | `rate(errors_total[5m]) / rate(requests_total[5m])` | Time series + threshold line at SLO |
| Latency percentiles | `histogram_quantile(0.99, rate(duration_bucket[5m]))` | Time series (p50, p95, p99) |
| Active requests | `gauge` of in-flight requests | Stat or gauge |
| Error budget remaining | SLO calculation | Stat with color thresholds |
| Top errors | `topk(5, sum by (error) (errors_total))` | Table |

## Template Variables

- Rule: Every dashboard must use template variables for `environment`, `namespace`, and `instance`/`pod`. This enables one dashboard to serve all environments.
- Rule: Use `$__rate_interval` instead of hardcoded intervals in `rate()` and `increase()` functions. It automatically adjusts to scrape interval and step.
- Rule: Add a `service` variable if the dashboard covers multiple services.
- Gotcha: Template variable queries should use `label_values()` for fast loading, not full metric queries.

## Annotations

- Rule: Add deployment annotations showing when new versions were deployed. This is the single most useful correlation tool for debugging post-deploy issues.
- Rule: Add incident annotations from your incident management system (PagerDuty, Opsgenie). Correlate metric changes with known incidents.
- Rule: Keep annotation density low. >5 annotations per hour makes the dashboard unreadable.

Implementation: Use CI/CD pipeline to POST deployment events to Grafana's annotation API or use a Kubernetes controller that watches Deployment rollouts.

## Dashboard Hierarchy

| Level | Purpose | Audience | Content |
|-------|---------|----------|---------|
| Overview | "Is anything broken?" | On-call, management | RED metrics for all services, SLO status, error budget |
| Drilldown | "What is broken in service X?" | Service team | Detailed metrics for one service, per-endpoint breakdown |
| Infrastructure | "Why is it broken?" | Platform team | Node resources, Collector health, database connections |

- Rule: Overview links to drilldown via click-through. Drilldown links to infrastructure. Users should never need to search for the next dashboard.
- Rule: Every service gets a drilldown dashboard. The overview dashboard should never have more than 2 rows of panels per service.

## Dashboard-as-Code

For teams managing 3+ dashboards, define dashboards in code:

| Tool | When | Notes |
|------|------|-------|
| Grafonnet (Jsonnet) | Complex dashboards, reusable components | Most flexible, steeper learning curve |
| Terraform `grafana_dashboard` | Infrastructure-managed dashboards | Integrates with IaC workflow |
| Grafana provisioning (YAML) | Simple, static dashboards | File-based, no API needed |

- Rule: Store dashboard definitions in version control. Review dashboard changes in PRs.
- Rule: Use dashboard provisioning to prevent manual drift. Dashboards modified via UI should be overwritten on next deploy.
- Gotcha: Grafonnet requires the Jsonnet toolchain. Start with Terraform `grafana_dashboard` resource if you already use Terraform.

## Anti-Patterns

- **Screenshot dashboards**: Dashboards designed for presentations, not debugging. Include filter variables and drill-down links, not static summaries.
- **Everything on one dashboard**: 30+ panels on a single dashboard. Split into hierarchy: overview -> drilldown -> infrastructure.
- **Hardcoded time intervals**: Using `rate(metric[5m])` instead of `rate(metric[$__rate_interval])`. Hardcoded intervals break when scrape interval or step changes.
- **No deployment markers**: Cannot correlate metric changes with deployments. Add CI/CD annotation integration.
