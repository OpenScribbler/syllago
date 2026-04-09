# Structured Logging

## Universal Rules

- Rule: Use JSON format in production, human-readable format in development. Toggle via environment variable (`LOG_FORMAT=json|text`).
- Rule: Log at service boundaries (incoming requests, outgoing calls, queue consumption). Do not log inside tight loops or hot paths.
- Rule: Inject `trace_id` and `span_id` from the OTel active span into every log line. This enables log-to-trace correlation in backends like Grafana Loki + Tempo.
- Rule: Never log sensitive data (passwords, tokens, PII). Use redaction middleware or structured field allowlists.

## Required Fields

Every log line in production must include:

| Field | Example | Notes |
|-------|---------|-------|
| `timestamp` | `2024-01-15T10:30:00.123Z` | ISO 8601, UTC |
| `level` | `info` | Lowercase: debug, info, warn, error |
| `service_name` | `order-service` | Matches OTel `service.name` |
| `message` | `order created` | Human-readable, no interpolated IDs |
| `correlation_id` | `req-abc-123` | Request-scoped, propagated across services |
| `trace_id` | `4bf92f3577b34da6a3ce929d0e0e4736` | From OTel active span |
| `span_id` | `00f067aa0ba902b7` | From OTel active span |

Optional but recommended: `environment`, `version`, `user_id` (if not PII-sensitive), `duration_ms`.

## Canonical Log Lines (Stripe Pattern)

- Rule: Emit one rich log line per request at the end of request processing. Include all accumulated context: duration, status, key business fields, error details.
- Rule: Use canonical log lines as the primary source for request-level debugging. They replace the need to grep through dozens of mid-request log lines.
- Gotcha: Canonical log lines complement, not replace, error-level logs. Still log errors immediately when they occur for real-time alerting.

Example structure (language-agnostic):
```
{"level":"info","message":"request completed","method":"POST","path":"/orders",
 "status":201,"duration_ms":45,"order_id":"ord-789","items_count":3,
 "trace_id":"4bf92f...","correlation_id":"req-abc-123"}
```

## Language Library Routing

| Language | Library | Notes | Skill Reference |
|----------|---------|-------|-----------------|
| Go | `log/slog` | stdlib, structured by default | [go-patterns](../../go-patterns/SKILL.md) |
| Python | `structlog` | Processor pipeline, JSON renderer | [python-patterns](../../python-patterns/SKILL.md) |
| Node.js | `pino` | Fastest JSON logger, async by default | [javascript-patterns](../../javascript-patterns/SKILL.md) |
| Rust | `tracing` + `tracing-subscriber` | Span-aware, structured output | [rust-patterns](../../rust-patterns/SKILL.md) |

SDK-specific initialization and configuration belongs in language skills, not here.

## Log Levels Guide

| Level | Use For | Alert? |
|-------|---------|--------|
| `error` | Unrecoverable failures, broken contracts, data loss risk | Yes |
| `warn` | Degraded operation, fallback used, retries exhausted soon | Monitor |
| `info` | Request lifecycle, business events, configuration loaded | No |
| `debug` | Internal state, variable values, branching decisions | No (disabled in prod) |

- Rule: If everything is `info`, nothing is. Reserve `info` for events you would want in a post-incident timeline.
- Rule: Never use `error` for expected conditions (user input validation, 404s). These are `warn` at most.

## Anti-Patterns

- **String interpolation in log messages**: `log.info(f"User {user_id} created order {order_id}")`. Use structured fields: `log.info("order created", user_id=user_id, order_id=order_id)`.
- **Unstructured messages with data**: `log.info("Processing took 45ms for user 123")`. Data buried in strings cannot be queried or aggregated.
- **Missing correlation_id**: Logs from the same request cannot be grouped. Generate at ingress, propagate via context.
- **Logging request/response bodies**: Massive log volume, potential PII exposure. Log content-length and status code instead.
