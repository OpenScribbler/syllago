---
name: api-design-patterns
description: Cross-language API design patterns for REST, gRPC, pagination, error handling, versioning, idempotency, and rate limiting. Use when designing, reviewing, or implementing API contracts. Language skills handle framework wiring; this skill covers the design decisions.
---

# API Design Patterns

Universal patterns for designing consistent, reliable APIs.

## API Design Checklist

| Category | Requirement |
|----------|-------------|
| Resource naming | Plural nouns, kebab-case, max 2 levels nesting |
| HTTP methods | Methods are verbs, resources are nouns, actions as sub-resources |
| Status codes | Correct code per operation; never 200 with error in body |
| Versioning | URL path `/v1/` for public APIs (default); date-header for granular SaaS |
| Pagination | Cursor for growing data, offset only for small stable sets |
| Error format | RFC 9457 base with request_id, machine-readable code, validation array |
| Idempotency | `Idempotency-Key` on all non-idempotent mutations (POST) |
| Rate limiting | Token bucket, IETF headers, mandatory Retry-After on 429 |
| Auth | API keys for identity, JWT for stateless auth, OAuth2 for delegation |
| OpenAPI | Design-first for public APIs, lint with Spectral in CI |
| gRPC | Contract-first proto, rich error model, unary default |
| Security | No credentials in URLs, explicit CORS allowlist, enforce Content-Type |

## Anti-Patterns

- **RPC-style URLs**: `/getUser`, `/createOrder` -- resources are nouns, HTTP methods are verbs.
- **200 with error body**: Always use appropriate 4xx/5xx status codes.
- **Query param versioning**: `?version=2` breaks caching and is invisible to proxies.
- **Offset on large datasets**: Performance degrades past ~10K rows. Use cursor/keyset.
- **Stack traces in errors**: Leaks internals. Log server-side with request_id, return generic 500.
- **Missing Retry-After on 429**: Clients cannot back off correctly without it.
- **gRPC without L7 load balancing**: gRPC multiplexes on one HTTP/2 connection; L4 LB pins all requests to one backend.

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| Resource naming, HTTP methods, status codes, versioning | [rest.md](references/rest.md) |
| Offset vs cursor, response shape, Stripe model | [pagination.md](references/pagination.md) |
| RFC 9457, error taxonomy, validation errors | [error-formats.md](references/error-formats.md) |
| Idempotency keys, atomic storage, duplicate handling | [idempotency.md](references/idempotency.md) |
| Rate limiting algorithms, IETF headers, client backoff | [rate-limiting.md](references/rate-limiting.md) |
| Proto design, streaming, rich errors, REST vs gRPC | [grpc.md](references/grpc.md) |
| Design-first, Spectral linting, contract testing | [openapi.md](references/openapi.md) |
| Auth method selection, API keys vs JWT vs OAuth2 vs mTLS | [auth.md](references/auth.md) |

## Related Skills

- **Language skills** (Go, Python, JS, Rust): Framework-specific wiring (FastAPI routers, Express middleware, Go net/http handlers, gRPC interceptors)
- **security-audit**: Deep auth implementation (JWT validation, OAuth2 flows, mTLS config) -- load `skills/security-audit/SKILL.md`
- **architecture-patterns**: API gateways, sync/async integration -- load `skills/architecture-patterns/references/integration-patterns.md`
- **testing-patterns**: Contract testing (Pact, Schemathesis) -- load `skills/testing-patterns/references/integration-testing.md`
