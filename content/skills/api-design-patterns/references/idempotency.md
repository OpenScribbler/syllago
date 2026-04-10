# Idempotency Patterns

## Overview

Idempotency ensures that retrying a request produces the same result as the original. Critical for payment processing, order creation, and any mutation where network failures cause retries.

## Which Endpoints Need It

| Method | Idempotent by Spec | Idempotency-Key Needed |
|--------|-------------------|----------------------|
| GET | Yes (safe) | No |
| PUT | Yes (full replace) | No |
| DELETE | Yes (same result) | No |
| PATCH | Depends | Optional (if non-idempotent operations) |
| POST | No | Yes -- always require for mutations |

## Client-Side Pattern

- Generate UUID v4 for each unique operation
- Send in `Idempotency-Key` header: `Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000`
- Retry with the same key on timeout or 5xx
- Generate a new key for genuinely new operations (not retries)

## Server-Side Pattern

1. Receive request with `Idempotency-Key`
2. Check if key exists in idempotency store
3. If key exists with completed response: return cached response (same status + body)
4. If key exists with in-progress status: return `409 Conflict` (concurrent duplicate)
5. If key does not exist: atomically store key + execute operation

### Atomic Storage

The check-and-store MUST be atomic to prevent race conditions. Use database transactions or atomic upserts.

```sql
INSERT INTO idempotency_keys (key, params_hash, status, created_at)
VALUES ($1, $2, 'processing', NOW())
ON CONFLICT (key) DO NOTHING
RETURNING key;
```

- If INSERT succeeds (row returned): proceed with operation, update status to `completed` with cached response
- If INSERT fails (no row returned): key already exists -- check status and return accordingly

### Response Caching

- Cache both 2xx and 4xx responses (client should get same error on retry)
- Do NOT cache 5xx responses (server failures should be retried)
- Set TTL on cached responses (24 hours is standard)
- Store: key, params_hash, status, response_status, response_body, created_at, expires_at

## Parameter Mismatch

When a client sends the same `Idempotency-Key` with different request parameters:
- Return `422 Unprocessable Entity` with code `idempotency_mismatch`
- Compare request body hash, not deep equality (faster, handles field ordering)
- This catches bugs where clients reuse keys incorrectly

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| Server-generated idempotency keys | Client can't retry without knowing the key | Client generates UUID v4 |
| Non-atomic check-and-store | Race condition: two requests both pass check | Use DB upsert or transaction |
| Caching 5xx responses | Masks transient failures, prevents retry | Only cache 2xx and 4xx |
| No TTL on cached responses | Unbounded storage growth | 24h TTL, sweep expired keys |
| Silently ignoring parameter mismatch | Hides client bugs | Return 422 with `idempotency_mismatch` |
