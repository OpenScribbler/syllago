# Error Response Formats

## RFC 9457 -- Problem Details (Base)

Content-Type: `application/problem+json`

```json
{
  "type": "https://api.example.com/errors/insufficient-balance",
  "title": "Insufficient Balance",
  "status": 422,
  "detail": "Account balance is $10.00 but transfer requires $25.00.",
  "instance": "/transfers/abc-123"
}
```

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `type` | URI | Yes | Machine-readable error type (default: `about:blank`) |
| `title` | string | Yes | Short human-readable summary (same for all instances of this type) |
| `status` | integer | Yes | HTTP status code (redundant with header, but useful in logs) |
| `detail` | string | No | Human-readable explanation specific to this occurrence |
| `instance` | URI | No | URI identifying this specific occurrence |

## Extended Error Model

Add these fields to RFC 9457 for production APIs:

| Field | Type | Purpose |
|-------|------|---------|
| `request_id` | string | Correlation ID for log tracing (always include) |
| `code` | string enum | Machine-readable error code for client branching |
| `errors` | array | Per-field validation errors |

### Validation Error Example

```json
{
  "type": "https://api.example.com/errors/validation",
  "title": "Validation Failed",
  "status": 422,
  "request_id": "req_abc123",
  "code": "validation_error",
  "errors": [
    {"field": "email", "message": "must be a valid email address"},
    {"field": "age", "message": "must be at least 18"}
  ]
}
```

## Error Code Taxonomy

Define a finite enum of machine-readable codes. Clients switch on `code`, not `status` or `detail`.

| Code | Status | Meaning |
|------|--------|---------|
| `validation_error` | 422 | One or more fields failed validation |
| `not_found` | 404 | Resource does not exist |
| `already_exists` | 409 | Duplicate resource (unique constraint) |
| `conflict` | 409 | State conflict (optimistic locking, version mismatch) |
| `unauthorized` | 401 | Missing or invalid credentials |
| `forbidden` | 403 | Insufficient permissions |
| `rate_limited` | 429 | Too many requests |
| `idempotency_mismatch` | 422 | Same idempotency key with different parameters |
| `internal_error` | 500 | Unhandled server error |

## Security Rules

- **Never leak stack traces**: Return generic `internal_error` for 500s. Log full trace server-side with `request_id`.
- **Generic messages for auth failures**: Don't distinguish "user not found" from "wrong password" -- both return 401 with same message.
- **request_id in every error**: Enables support to correlate user-reported errors with server logs without exposing internals.
- **No internal identifiers**: Don't expose database IDs, table names, or internal service names in error responses.

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| `{"error": "something went wrong"}` | No structure for clients to parse | Use RFC 9457 format |
| Stack traces in production responses | Leaks internals, aids attackers | Log server-side, return generic message |
| String status codes (`"status": "error"`) | Can't branch reliably | Use integer HTTP status codes |
| Error details only in response body | Client may not parse body on error | Also set correct HTTP status code |
| Different error shapes per endpoint | Client needs per-endpoint parsing | Use consistent error envelope everywhere |
