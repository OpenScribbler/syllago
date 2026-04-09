# gRPC Patterns

## REST vs gRPC Decision

| Factor | REST | gRPC |
|--------|------|------|
| External/public clients | Yes (browser-native, universal tooling) | No (requires HTTP/2, proto compilation) |
| Internal service-to-service | Acceptable | Preferred (type safety, performance) |
| Browser clients | Yes (native fetch/XHR) | Only via gRPC-Web proxy |
| Streaming requirements | SSE or WebSocket (separate protocol) | Native bidirectional streaming |
| Strong contract enforcement | OpenAPI (optional) | Proto (mandatory, built-in) |
| Payload size sensitivity | JSON (verbose) | Protobuf (compact binary) |

**Hybrid default**: REST for external/public APIs, gRPC for internal service mesh. This is the most common production pattern.

## Proto Design Rules

### Naming Conventions

- Messages: `CamelCase` (`CreateUserRequest`)
- Fields: `snake_case` (`first_name`, `created_at`)
- Enums: `SCREAMING_SNAKE_CASE` (`ORDER_STATUS_PENDING`)
- Services: `CamelCase` (`UserService`)
- RPCs: `CamelCase` (`CreateUser`, `ListOrders`)

### Message Design

- Separate request/response per RPC: `{Method}Request` / `{Method}Response`
- Never reuse a request message across RPCs (even if fields are identical today)
- Use wrapper types for optional primitives: `google.protobuf.StringValue` for optional string

### Enum Rules

- First value MUST be `UNSPECIFIED = 0` (protobuf default for unset fields)
- Prefix values with enum name to avoid collisions:
  ```protobuf
  enum OrderStatus {
    ORDER_STATUS_UNSPECIFIED = 0;
    ORDER_STATUS_PENDING = 1;
    ORDER_STATUS_COMPLETED = 2;
  }
  ```
- Never reuse enum numbers (use `reserved` for removed values)

### Field Numbering

- Fields 1-15 use 1 byte for tag+type -- reserve for frequently used fields
- Fields 16-2047 use 2 bytes
- Never reuse field numbers: `reserved 3, 7;` and `reserved "old_field_name";`
- Plan field number ranges per message section (1-10 identifiers, 11-20 metadata, etc.)

### Well-Known Types

Use standard types from `google/protobuf/`:

| Type | Use For |
|------|---------|
| `Timestamp` | Points in time (not `int64` seconds or `string` ISO8601) |
| `Duration` | Time spans (not `int64` milliseconds) |
| `FieldMask` | Partial updates (which fields to modify) |
| `Empty` | RPCs with no request or response data |
| `Struct` / `Value` | Dynamic JSON-like data (use sparingly) |

### Package Versioning

```protobuf
package company.service.v1;

option go_package = "company/service/v1;servicev1";
```

- Major version in package name: `v1`, `v2`
- Separate proto files per major version
- Non-breaking changes within a version (add fields, add RPCs)
- Breaking changes require new version package

## Streaming Patterns

| Pattern | Direction | Use When |
|---------|-----------|----------|
| Unary | Client -> Server -> Client | Default. Single request, single response |
| Server streaming | Client -> Server ->> Client | Large result sets, progress updates, event feeds |
| Client streaming | Client ->> Server -> Client | Chunked file uploads, aggregation |
| Bidirectional | Client <<->> Server | Real-time collaboration, chat, live updates |

- **Default to unary** -- add streaming only when unary cannot meet requirements
- Server streaming: use for results too large for single response or real-time push
- Client streaming: use for uploads or when client sends data incrementally
- Bidirectional: use only for real-time interactive scenarios

### Streaming Gotchas

- Always handle stream errors and cancellation (context.Done())
- Set deadlines on streams to prevent resource leaks
- Implement flow control -- don't send faster than receiver can process
- Server streaming is simpler and more debuggable than bidirectional; prefer it when possible

## Rich Error Model

Use `google.rpc.Status` with typed detail messages instead of plain status codes.

### Error Detail Types

| Type | Use For |
|------|---------|
| `BadRequest.FieldViolation` | Per-field validation errors (field name + description) |
| `ErrorInfo` | Machine-readable error (domain, reason, metadata map) |
| `RetryInfo` | Suggest retry delay for transient failures |
| `QuotaFailure` | Quota/rate limit exceeded (which quota, limit value) |
| `RequestInfo` | Request ID for log correlation |
| `PreconditionFailure` | Failed preconditions (type, subject, description) |

### gRPC Status Code Mapping

| Code | When to Use | HTTP Equivalent |
|------|-------------|----------------|
| `OK` | Success | 200 |
| `INVALID_ARGUMENT` | Bad input, validation failure | 400 |
| `NOT_FOUND` | Resource does not exist | 404 |
| `ALREADY_EXISTS` | Duplicate resource | 409 |
| `PERMISSION_DENIED` | Insufficient permissions | 403 |
| `UNAUTHENTICATED` | Missing or invalid credentials | 401 |
| `RESOURCE_EXHAUSTED` | Rate limit or quota exceeded | 429 |
| `FAILED_PRECONDITION` | System not in required state | 400 |
| `UNAVAILABLE` | Transient failure, client should retry | 503 |
| `INTERNAL` | Unexpected server error | 500 |
| `DEADLINE_EXCEEDED` | Operation timed out | 504 |

- `INVALID_ARGUMENT` vs `FAILED_PRECONDITION`: INVALID_ARGUMENT = bad input regardless of state; FAILED_PRECONDITION = input is valid but system state is wrong
- `UNAVAILABLE` is the only code that clearly signals "retry is appropriate"
- Always include `RequestInfo` detail for log correlation

## Infrastructure Requirements

- **L7 load balancing required**: gRPC multiplexes RPCs on a single HTTP/2 connection. L4 LB pins all RPCs to one backend. Use Envoy, Linkerd, or cloud L7 LB.
- **Channel reuse**: Create one gRPC channel per target service, reuse across requests. Do not create a channel per request.
- **Keepalive**: Configure keepalive pings to detect dead connections (default: 2-hour TCP keepalive is too slow). Recommended: 30s keepalive, 5s timeout.
- **Max message size**: Default 4MB. Increase only if needed; prefer streaming for large payloads.
- **Deadlines**: Always set deadlines on RPCs. No deadline = potential infinite wait. Propagate deadlines across service calls.

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| gRPC to browser without gRPC-Web | Browsers don't support HTTP/2 trailers | Use gRPC-Web proxy or REST for browser clients |
| Reusing request messages across RPCs | Coupling -- changing one RPC breaks others | Separate `{Method}Request` per RPC |
| Enum without UNSPECIFIED=0 | Unset fields silently get first enum value | Always `UNSPECIFIED = 0` |
| L4 load balancer for gRPC | All RPCs pinned to one backend | Use L7 LB (Envoy, Linkerd) |
| Channel per request | Connection overhead, port exhaustion | Reuse channels |
| No deadline on RPCs | Resource leaks from hanging calls | Always set context deadline |
| Reusing field numbers | Wire-format corruption | Use `reserved` for removed fields |
