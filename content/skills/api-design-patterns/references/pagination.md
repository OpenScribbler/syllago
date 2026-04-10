# Pagination Patterns

## Strategy Selection

| Strategy | Best For | Limitations |
|----------|----------|------------|
| Offset (`?offset=20&limit=10`) | Small stable datasets (<10K rows), admin UIs needing "page N" | Phantom records on insert/delete, O(N) DB skip cost |
| Cursor/Keyset (`?cursor=abc&limit=10`) | Growing datasets, feeds, real-time data | No random page access, cursor is opaque |

**Default to cursor/keyset** unless the use case specifically requires page-number navigation on a small, stable dataset.

## Offset Pagination

Response shape:
```json
{
  "data": [...],
  "total_count": 1234,
  "offset": 20,
  "limit": 10
}
```

- `total_count` requires a separate COUNT query -- omit if expensive (large tables)
- Gotcha: inserting/deleting rows between pages causes phantom records (items skipped or duplicated)
- Performance cliff: `OFFSET 100000` scans and discards 100K rows. Switch to cursor at ~10K rows.

## Cursor/Keyset Pagination

- Cursor = Base64-encoded keyset values (e.g., `base64(id=500,created_at=2024-01-15)`)
- Fetch `limit + 1` rows to determine `has_more` without a COUNT query
- Cap `limit` server-side (e.g., max 100) to prevent abuse

### Stripe Model (Recommended)

Parameters: `starting_after=obj_id`, `ending_before=obj_id`, `limit=N`

Response shape:
```json
{
  "data": [...],
  "has_more": true,
  "next_cursor": "eyJpZCI6NTAwfQ=="
}
```

- `starting_after` / `ending_before` use the object ID directly, not a synthetic cursor
- Clearer semantics than opaque cursor tokens
- Supports both forward and backward traversal

## Response Shape Conventions

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `data` | array | Yes | Always an array, even for single result |
| `has_more` | boolean | Yes | True if more pages exist |
| `next_cursor` | string | If has_more | Opaque cursor for next page |
| `total_count` | integer | No | Only if cheap to compute |

- Always wrap collections in an object (never return a bare JSON array)
- Include `has_more` rather than forcing clients to check `data.length < limit`

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| No server-side limit cap | Client requests `?limit=999999` | Enforce max (e.g., 100), ignore larger values |
| Page number with large datasets | O(N) offset, phantom records | Cursor/keyset pagination |
| Bare JSON array response | Can't add metadata without breaking clients | Wrap in `{"data": [...]}` |
| Cursor containing raw SQL | SQL injection via tampered cursor | Base64-encode opaque keyset, validate on decode |
| COUNT(*) on every request | Expensive on large tables | Make `total_count` optional, omit by default |
