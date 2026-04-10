# Prometheus Metrics Design

## Naming Conventions

- Rule: Use `snake_case` for all metric names.
- Rule: Use base SI units: `seconds` not `milliseconds`, `bytes` not `kilobytes`.
- Rule: Append `_total` suffix to all counters.
- Rule: Prefix with namespace matching `service.name`: `order_service_requests_total`.
- Rule: Include unit in metric name: `http_request_duration_seconds`, not `http_request_duration`.

## Metric Types

| Type | Use For | Key Rules |
|------|---------|-----------|
| Counter | Cumulative totals (requests, errors, bytes) | Always append `_total`, never decrements |
| Histogram | Request durations, response sizes | Use for anything with SLO targets |
| Gauge | Current state (queue depth, active connections, temperature) | Can go up and down |
| Summary | Quantile calculation at client | Avoid in replicated services (cannot aggregate across instances) |

## RED Metrics (Minimum Viable Metrics)

Every service entry point (HTTP endpoint, gRPC method, queue consumer) MUST expose:

| Metric | Name Pattern | Type |
|--------|-------------|------|
| **R**ate | `<namespace>_requests_total` | Counter |
| **E**rrors | `<namespace>_requests_total{status="error"}` or `<namespace>_errors_total` | Counter |
| **D**uration | `<namespace>_request_duration_seconds` | Histogram |

- Rule: Label with `method`, `path` (templated), `status_code` (or status category: 2xx/4xx/5xx).
- Rule: This is the absolute minimum for any Tier 2+ service. Skip RED only for Tier 1.

## Cardinality

Cardinality = unique combinations of label values. High cardinality causes Prometheus memory exhaustion and query timeouts.

| Label Value | Safe? | Reason |
|-------------|-------|--------|
| HTTP method (GET, POST, PUT, DELETE) | Yes | Bounded, <10 values |
| Status code category (2xx, 4xx, 5xx) | Yes | Bounded, 3-5 values |
| Endpoint template (`/users/{id}`) | Yes | Bounded by route count |
| User ID | **No** | Unbounded, millions of values |
| Request ID | **No** | Unbounded, unique per request |
| Full URL path (`/users/12345`) | **No** | Unbounded, unique per user |
| Error message string | **No** | Unbounded, varies per error |

- Rule: Never use unbounded values as label values. If you need per-user metrics, use logs or traces instead.
- Rule: Estimate cardinality before adding a label: multiply all possible values across all labels. Target: <1000 series per metric.
- Rule: Use a cardinality analysis query in Prometheus to find offenders: `topk(10, count by (__name__)({__name__=~".+"}))`

## Histogram Bucket Design

- Rule: Default buckets (`.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10`) work for most HTTP APIs.
- Rule: Include your SLO threshold value as a bucket boundary. If SLO is "p99 < 500ms", include a `0.5` bucket.
- Rule: Add buckets at meaningful business thresholds (e.g., user-noticeable latency at 200ms, timeout at 30s).
- Rule: More buckets = more time series. Keep to 10-15 buckets maximum.
- Gotcha: Histogram buckets are cumulative (`le` label). The `+Inf` bucket always equals the total count.

## Recording Rules

For Tier 3+, pre-compute expensive SLI expressions as recording rules:

```yaml
groups:
  - name: sli_recording_rules
    rules:
      - record: service:request_errors:ratio_rate5m
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m])) by (service)
          /
          sum(rate(http_requests_total[5m])) by (service)
```

- Rule: Name recording rules as `level:metric:operations` (e.g., `service:request_errors:ratio_rate5m`).
- Rule: Use recording rules for any expression used in both dashboards AND alerts. Ensures consistency and reduces query load.
- Rule: Recording rule evaluation interval should match or be shorter than the shortest alert `for` duration.
