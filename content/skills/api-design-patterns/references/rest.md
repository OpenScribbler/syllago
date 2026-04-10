# REST API Design

## Resource Naming

- Use plural nouns: `/users`, `/orders`, `/line-items`
- Use kebab-case for multi-word resources: `/access-policies`, not `/accessPolicies`
- Max 2 levels of nesting: `/users/123/orders` -- never `/users/123/orders/456/items/789`
- For deeper resources, promote to top-level with filter: `/order-items?order_id=456`
- Actions as sub-resources with POST: `POST /orders/123/cancel`, not `POST /cancelOrder`
- Resource identifiers in path, filters in query: `/users/123`, `/users?role=admin`

## HTTP Method Semantics

| Method | Semantics | Idempotent | Safe | Request Body |
|--------|-----------|------------|------|-------------|
| GET | Read resource(s) | Yes | Yes | No |
| POST | Create resource or trigger action | No | No | Yes |
| PUT | Full replace of resource | Yes | No | Yes |
| PATCH | Partial update of resource | No* | No | Yes |
| DELETE | Remove resource | Yes | No | No |

*PATCH is idempotent if using JSON Merge Patch; not idempotent with JSON Patch (array of operations).

## HTTP Status Codes -- Success

| Code | When to Use |
|------|-------------|
| 200 OK | GET, PATCH, PUT success with response body |
| 201 Created | POST created a new resource; include `Location` header |
| 202 Accepted | Request queued for async processing |
| 204 No Content | DELETE success or PUT/PATCH with no response body |

## HTTP Status Codes -- Client Errors

| Code | When to Use | Key Distinction |
|------|-------------|-----------------|
| 400 Bad Request | Malformed syntax, missing required fields | Request cannot be parsed |
| 401 Unauthorized | No credentials or invalid credentials | Not authenticated |
| 403 Forbidden | Valid credentials but insufficient permissions | Not authorized |
| 404 Not Found | Resource does not exist | Also use for unauthorized resource access to avoid leaking existence |
| 405 Method Not Allowed | HTTP method not supported on this endpoint | Include `Allow` header |
| 409 Conflict | State conflict (duplicate, version mismatch) | Resource-level contention |
| 422 Unprocessable Entity | Valid syntax but fails business rules | Request parses but is semantically invalid |
| 429 Too Many Requests | Rate limit exceeded | MUST include `Retry-After` header |

**400 vs 422**: 400 = the request is malformed (bad JSON, missing required field). 422 = the request is well-formed but violates business logic (email already taken, insufficient balance).

## HTTP Status Codes -- Server Errors

| Code | When to Use |
|------|-------------|
| 500 Internal Server Error | Unhandled server failure; never expose details |
| 502 Bad Gateway | Upstream service returned invalid response |
| 503 Service Unavailable | Server overloaded or in maintenance; include `Retry-After` |
| 504 Gateway Timeout | Upstream service timed out |

## Versioning Strategy

| Strategy | Use When | Trade-offs |
|----------|----------|------------|
| URL path `/v1/` | Public APIs (default) | Simple routing, clear in docs; URL changes on major version |
| Date-based header `API-Version: 2024-01-15` | Granular SaaS evolution (Stripe model) | Fine-grained control; harder to discover, client must opt-in |
| Query param `?version=2` | Never in production | Breaks caching, invisible to proxies, pollutes URLs |

### Breaking vs Non-Breaking Changes

**Breaking** (requires new version):
- Removing or renaming fields
- Changing field types
- Removing endpoints
- Changing required/optional status of request fields
- Changing response structure

**Non-breaking** (safe to ship):
- Adding optional request fields
- Adding response fields
- Adding new endpoints
- Adding new enum values (if clients handle unknown values)

### Deprecation Lifecycle

1. Announce deprecation with timeline (min 6 months for public APIs)
2. Add `Deprecation` and `Sunset` headers to responses
3. Log usage metrics to track migration
4. Return `410 Gone` after sunset date

## Query Parameters

- Filtering: `?status=active&role=admin` (field=value)
- Sorting: `?sort=created_at` or `?sort=-created_at` (prefix `-` for descending)
- Field selection: `?fields=id,name,email`
- Search: `?q=search+term` (full-text across multiple fields)
- Use snake_case for parameter names, consistent with JSON field naming

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| Verbs in URLs (`/getUsers`) | Duplicates HTTP method semantics | `GET /users` |
| Singular nouns (`/user/123`) | Inconsistent pluralization | Always plural: `/users/123` |
| Deep nesting (`/a/1/b/2/c/3`) | Tight coupling, fragile URLs | Promote to top-level with filter |
| PATCH without Content-Type | Ambiguous patch format | Require `application/merge-patch+json` or `application/json-patch+json` |
| 200 for everything | Clients can't branch on status | Use correct status codes |
| Exposing DB IDs sequentially | Enumeration attacks | Use UUIDs or opaque IDs |
