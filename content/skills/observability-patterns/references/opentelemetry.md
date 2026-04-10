# OpenTelemetry Patterns

## Architecture Rule

**SDK -> Collector -> Backend.** Never send telemetry directly from SDK to backend in production. The Collector provides buffering, retry, sampling, and export decoupling.

## SDK Initialization

- Rule: Initialize the OTel SDK before importing or initializing any instrumented library (HTTP clients, DB drivers, queue consumers). Critical in Node.js where module-level initialization happens at import time.
- Rule: Use `BatchSpanProcessor` in production. `SimpleSpanProcessor` is synchronous and blocks the request path.
- Rule: Set `service.name`, `service.version`, and `deployment.environment` as resource attributes on every SDK instance.
- Rule: Generate metrics from spans BEFORE sampling discards them. Use OTel's span-to-metrics connector or SDK-level metrics to avoid losing signal on sampled-out requests.

## Instrumentation Strategy

- Rule: Use auto-instrumentation first for infrastructure concerns (HTTP, gRPC, database, message queue). It covers 80% of tracing needs with zero code changes.
- Rule: Add manual spans only for business logic that auto-instrumentation cannot capture (payment processing, ML inference, complex workflows).
- Rule: Set semantic convention attributes on spans (`http.method`, `db.system`, `messaging.system`). Custom attributes use reverse-domain naming (`com.acme.order_id`).
- Rule: Keep span names low-cardinality. Use `GET /users/{id}` not `GET /users/12345`.

## Sampling

| Strategy | Use When | Config |
|----------|----------|--------|
| Always-on (ratio=1.0) | Tier 2, <100 RPS | `ParentBased(AlwaysOn)` |
| Head-based ratio | Tier 3, 100-1000 RPS | `ParentBased(TraceIdRatioBased(0.1))` |
| Tail-based (Collector) | Tier 4, >1000 RPS or cost pressure | Collector `tail_sampling` processor |

- Rule: `ParentBased(TraceIdRatioBased(N))` is the default sampler. It respects parent decisions, preventing broken traces.
- Rule: Always sample errors regardless of ratio. Use tail-based sampling in the Collector with an error policy to retain 100% of error traces.
- Rule: Set sampling ratio based on traffic volume, not gut feel. Target: retain enough traces to debug any issue within 15 minutes.

## Propagation

- Rule: Use W3C TraceContext as the default propagator. It is the industry standard and supported by all modern backends.
- Rule: Use B3 propagation only for legacy Zipkin integration. When migrating, run both propagators in parallel (`CompositeTextMapPropagator`) during transition.
- Rule: Verify propagation across every service boundary (HTTP headers, message queue metadata, gRPC metadata). Missing propagation breaks distributed traces silently.

## Collector: App-Side Decisions

The Collector deployment model affects application configuration:

| Model | When | App SDK Config |
|-------|------|---------------|
| Agent (DaemonSet/sidecar) | Single cluster, low latency needs | `endpoint: localhost:4317` (gRPC) |
| Gateway (Deployment) | Multi-cluster, centralized processing | `endpoint: otel-gateway.monitoring:4317` |

- Rule: Use gRPC (port 4317) over HTTP (port 4318) for Collector communication. gRPC has lower overhead and supports streaming.
- Rule: Set Collector memory limits to prevent OOM. Start with 512Mi for agent mode, 2Gi for gateway mode.
- Gotcha: Deep Collector configuration (pipelines, processors, exporters) is an infrastructure concern. See [kubernetes-patterns/references/observability.md](../../kubernetes-patterns/references/observability.md) for Collector deployment patterns.

## Performance Budget

- Rule: Properly configured OTel adds 2-5% CPU overhead. If overhead exceeds 5%, check for excessive custom spans, synchronous processors, or missing batching.
- Rule: Benchmark instrumentation impact in staging before production rollout. Measure p99 latency with and without instrumentation.
