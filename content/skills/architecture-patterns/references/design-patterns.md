# Architecture Design Patterns

Patterns for availability, reliability, scalability, and performance. Claude knows Mermaid syntax and these standard patterns -- use rule statements as reminders of trade-offs and selection criteria.

## Pattern Selection Guide

| Need | Pattern Category | Key Patterns |
|------|------------------|--------------|
| System stays up | Availability | Redundancy, Failover, Health Checks |
| System recovers gracefully | Reliability | Retries, Circuit Breaker, Bulkhead |
| System handles growth | Scalability | Horizontal Scaling, Caching, Sharding |
| System responds fast | Performance | Async Processing, Batching, Connection Pooling |

## Availability Patterns

### Redundancy
**Category**: availability

- Rule: Deploy multiple instances of every component to eliminate single points of failure. Requires stateless app tier or external state management.
- Trade-offs: Survives instance failures, enables zero-downtime deploys. Costs more, requires state consistency strategy.
- Diagram: C4Container showing LB -> N app instances -> primary DB + read replica.

### Failover
**Category**: availability

- Rule: Automatically switch to backup systems when primary fails. Choose active-passive (standby takes over) or active-active (all handle traffic).
- Trade-offs: Eliminates downtime from single-region/instance failure. Adds complexity for state sync and split-brain prevention.
- Diagram: sequenceDiagram showing client -> LB -> primary (fails) -> LB reroutes to backup.

### Health Checks
**Category**: availability

- Rule: Implement three probe types -- liveness (process running?), readiness (can handle requests?), startup (finished initializing?). Readiness should verify downstream dependencies.
- Gotcha: Liveness checks must NOT depend on external services -- a DB outage should not restart healthy app pods.
- Diagram: sequenceDiagram showing LB polling /health/live and /health/ready, removing instance on 503.

## Reliability Patterns

### Retry with Exponential Backoff
**Category**: reliability

- Rule: Retry transient failures with increasing delays. Config: max retries 3-5, initial delay 100ms-1s, max delay 30-60s. Always add jitter to prevent thundering herd.
- Gotcha: Only retry idempotent operations. Non-idempotent calls (payment, order creation) need idempotency keys.
- Diagram: sequenceDiagram showing client retrying with 1s, 2s, 4s delays until success.

### Circuit Breaker
**Category**: reliability

- Rule: Three states -- Closed (normal), Open (failing fast without calling downstream), Half-Open (testing recovery). Opens after N consecutive failures, periodically tests with a single request.
- Trade-offs: Prevents cascade failures and resource exhaustion. Requires tuning thresholds per dependency.
- Gotcha: Set different thresholds per downstream service. Critical services may need longer half-open test windows.

### Bulkhead
**Category**: reliability

- Rule: Isolate failures by partitioning resources per dependency -- separate thread pools, connection pools, or processes. A failing downstream exhausts only its own pool.
- Types: Thread pool isolation (per-dependency), connection pool isolation (dedicated connections), process isolation (separate containers).
- Trade-offs: Prevents system-wide impact from one bad dependency. Increases resource overhead and config complexity.

## Scalability Patterns

### Horizontal Scaling
**Category**: scalability

- Rule: Add instances to handle load. Requires stateless app tier, external session storage, and load balancer. Use auto-scaling based on CPU/memory/custom metrics.
- Trade-offs: Near-linear scaling for stateless services. Needs external state (Redis/DB) and adds deployment complexity.

### Caching
**Category**: scalability

- Rule: Layer caches -- L1 in-process (fastest, per-instance), L2 distributed like Redis (shared, network hop), L3 CDN (edge, static content). Choose strategy based on consistency needs.
- Strategies: Cache-aside (app manages), read-through (cache loads on miss), write-through (cache updates on write), write-behind (async updates).
- Gotcha: Cache invalidation is the hard part. Use TTLs as a safety net even with explicit invalidation. Watch for cache stampede on expiration of hot keys.

### Database Sharding
**Category**: scalability

- Rule: Partition data across DB instances. Strategies -- range-based (ID ranges), hash-based (hash(key) % N), geographic (by region), tenant-based (per customer).
- Trade-offs: Linear scalability. Cross-shard queries are complex, rebalancing is difficult, joins across shards require application logic.
- Gotcha: Choose shard key carefully -- changing it later requires full data migration. Prefer tenant-based for SaaS.

## Performance Patterns

### Asynchronous Processing
**Category**: performance

- Rule: Decouple request handling from processing via message queues. Return 202 Accepted immediately, process async, notify via polling or webhook.
- Trade-offs: Handles load spikes, improves response times. Requires eventual consistency tolerance and dead letter queue for failures.

### Batching
**Category**: performance

- Rule: Group multiple operations into single requests. Trigger batch on size threshold or timeout (whichever comes first).
- Trade-offs: Reduces network overhead and DB round-trips. Adds latency for individual items and requires batch error handling.

### Connection Pooling
**Category**: performance

- Rule: Reuse database connections across requests. Config: min connections (warm for baseline), max connections (prevent DB overload), idle timeout (release unused), max lifetime (prevent stale).
- Gotcha: Max pool size should be less than DB max_connections / number_of_instances. Monitor for pool exhaustion under load.

## Diagram Guidance

See SKILL.md "Diagram Type Selection" table for the Mermaid keyword quick reference. Generate diagrams from the pattern descriptions above — Claude knows C4 and sequence diagram syntax natively.

## Design Document Structure

Keep design docs concise — they are reference material, not narratives.

| Section | Target | Content |
|---------|--------|---------|
| Overview | 3-5 lines | Problem statement, scope, one-sentence approach |
| Context Diagram | Mermaid | C4Context — system boundary and external actors |
| Container Diagram | Mermaid | C4Container — internal services and data stores |
| Key Decisions | Table | Link to ADR files: `| ADR-NNN | Summary | Status |` |
| Sequence Diagrams | Mermaid | One per critical flow (auth, data processing) |
| Deployment | Mermaid | C4Deployment if infra design included |

**Anti-patterns:**
- Restating ADR content in the design doc (link instead)
- Prose descriptions of architecture that could be a diagram
- Sections longer than 20 lines without a diagram
