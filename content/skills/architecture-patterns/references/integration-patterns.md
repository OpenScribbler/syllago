# Integration Design Patterns

Patterns for designing system integrations and distributed architectures. For availability/reliability/scalability/performance patterns, see [design-patterns.md](design-patterns.md).

## API Gateway Patterns

| Pattern | Use Case | Key Characteristic |
|---------|----------|-------------------|
| Edge gateway | Public traffic ingress | Auth, rate limiting, TLS termination, WAF |
| Internal gateway | Service-to-service | Service discovery, load balancing, mTLS |
| BFF (Backend-for-Frontend) | Per-client API | Tailored responses for web/mobile/IoT |

- Rule: Use API gateway for cross-cutting concerns (auth, rate limiting, logging, transformation). Never put business logic in the gateway.
- Decision: Single gateway for small teams (<5 services). Per-domain gateway for large orgs with autonomous teams -- avoids the gateway becoming a deployment bottleneck.
- Gotcha: API gateway as a single point of failure. Deploy in HA mode with health checks. Consider regional gateways for global traffic.

## Event-Driven Architecture

### Message Broker Selection

| Broker | Strengths | Best For |
|--------|-----------|----------|
| **Kafka** | High throughput, ordered partitions, replay | Event sourcing, streaming analytics, audit logs |
| **RabbitMQ** | Flexible routing, low latency, mature | Task queues, complex routing, request-reply |
| **SQS/SNS** | Fully managed, no ops | Simple async processing, fan-out |
| **NATS** | Lightweight, cloud-native, JetStream for persistence | Microservices, edge, real-time messaging |

### Event Patterns

| Pattern | Description | When to Use |
|---------|-------------|-------------|
| Event notification | Fire-and-forget, minimal payload | Decoupling, downstream reacts independently |
| Event-carried state transfer | Full entity data in event payload | Consumer needs data without callback to producer |
| Event sourcing | Events as source of truth, rebuild state from log | Audit trail, temporal queries, undo/replay |
| CQRS | Separate read/write models | Different scaling/optimization for reads vs writes |

- Rule: Use events for decoupling services that don't need synchronous responses. Default to event notification; upgrade to event-carried state transfer when consumers need data.
- Gotcha: Event sourcing adds significant complexity (snapshots, projections, versioning). Only use when audit trail and temporal queries are genuine requirements, not speculative.
- Gotcha: Event ordering matters. Use partition keys (Kafka) or message groups (SQS FIFO) to maintain per-entity ordering. Global ordering across all events is rarely needed and kills throughput.

## Service Communication Patterns

### Sync vs Async Decision

| Factor | Synchronous (REST/gRPC) | Asynchronous (events/queues) |
|--------|--------------------------|------------------------------|
| Response needed immediately | Yes | No |
| Caller waits for result | Yes | No -- fire and process later |
| Coupling tolerance | Tight (both services up) | Loose (producer/consumer independent) |
| Failure handling | Retry/circuit breaker | Dead letter queue, reprocessing |

- Rule: Sync (REST/gRPC) for queries needing immediate response. Async (events/queues) for commands that can be processed later.
- Rule: gRPC for internal service-to-service (strong typing, streaming, performance). REST for external/public APIs (ubiquity, tooling, HTTP caching).
- Gotcha: gRPC requires HTTP/2. Verify your load balancer, API gateway, and service mesh support HTTP/2 end-to-end before committing.

### Distributed Transaction Patterns

| Approach | When to Use | Complexity |
|----------|-------------|------------|
| Saga -- choreography | Simple flows, few services, events already in use | Low-medium |
| Saga -- orchestration | Complex flows, compensation logic, conditional branching | Medium-high |
| Two-phase commit (2PC) | Avoid -- poor availability, tight coupling | High (anti-pattern in microservices) |

- Rule: Use saga for distributed transactions. Choreography (events) for simple flows (<4 steps). Orchestration (coordinator service) for complex flows with compensation logic.
- Gotcha: Every saga step needs a compensating action (undo). Design compensation before the happy path -- it's harder to retrofit.

## Multi-Tenancy Patterns

| Isolation Level | Data Isolation | Cost | Compliance |
|-----------------|---------------|------|------------|
| Shared everything + row-level security | Logical (tenant_id filter) | Lowest | Basic |
| Shared infra, separate DB per tenant | Physical DB isolation | Moderate | SOC2, most SaaS |
| Separate infra per tenant | Full stack isolation | Highest | HIPAA, FedRAMP, data residency |

- Rule: Start with shared-everything + row-level security for SaaS. Move to separate DB when tenants have compliance or data residency requirements.
- Gotcha: Always include `tenant_id` in every query and every index. Missing tenant filters = data leaks across tenants. Enforce at the ORM/middleware layer, not per-query.
- Gotcha: Tenant-aware connection pooling is required for separate-DB models. A shared pool pointing at one DB won't work.

## Data Pipeline Architecture

| Pattern | When to Use | Tools |
|---------|-------------|-------|
| Batch (ETL/ELT) | Analytics, reporting, hourly/daily cadence | Airflow, dbt, Spark |
| Streaming | Real-time dashboards, alerts, sub-second latency | Kafka Streams, Flink, Kinesis |
| Hybrid (Lambda) | Need both real-time and batch views | Kafka + Spark/dbt |

- Rule: Batch for analytics/reporting. Streaming for real-time requirements. Default to ELT (load then transform) with modern warehouses (BigQuery, Snowflake, Databricks) -- more flexible than ETL.
- Gotcha: Don't build real-time when batch is sufficient. Real-time adds operational complexity (back-pressure, exactly-once, state management) for marginal benefit in most analytics use cases.
- Gotcha: Lambda architecture means maintaining two codepaths (batch + stream). Consider Kappa architecture (stream-only with replay) if your broker supports replay (Kafka).

## Integration Anti-Patterns

| Anti-Pattern | Why It Fails | Alternative |
|--------------|-------------|-------------|
| Point-to-point spaghetti (N^2 connections) | Unmanageable at scale, no visibility | Event bus or API gateway |
| Synchronous chains (A->B->C->D) | Latency compounds, one failure breaks all | Async where possible, circuit breakers on sync |
| Shared database integration | Tight coupling, schema changes break consumers | APIs or events between services |
| Chatty APIs (many small calls) | Network overhead, latency | Coarse-grained endpoints, batch operations, GraphQL |
| Ignoring idempotency | Duplicate processing on retries | Idempotency keys on all mutation endpoints |
| Distributed monolith | Microservice boundaries but synchronous coupling | True async decoupling or merge back to monolith |
