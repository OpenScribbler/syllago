# Rate Limiting

## Algorithm Selection

| Algorithm | How It Works | Best For |
|-----------|-------------|----------|
| Fixed window | Count requests per calendar window (e.g., per minute) | Simple, low-overhead |
| Sliding window | Weighted average of current + previous window | Smoother than fixed, still simple |
| Token bucket | Tokens refill at steady rate, each request costs a token | Production standard -- allows bursts while enforcing average rate |
| Leaky bucket | Requests queue and drain at fixed rate | Strict smoothing, no bursts allowed |

**Default to token bucket** -- it handles burst traffic gracefully while maintaining a sustainable average rate.

## IETF RateLimit Headers

Include on every response (not just 429s) so clients can self-throttle:

| Header | Value | Example |
|--------|-------|---------|
| `RateLimit-Limit` | Max requests per window | `RateLimit-Limit: 100` |
| `RateLimit-Remaining` | Requests left in current window | `RateLimit-Remaining: 42` |
| `RateLimit-Reset` | Seconds until window resets | `RateLimit-Reset: 30` |
| `Retry-After` | Seconds to wait (on 429 only) | `Retry-After: 5` |

- `Retry-After` is MANDATORY on 429 responses -- without it, clients cannot back off correctly
- `RateLimit-Reset` is seconds remaining, not a Unix timestamp (IETF draft standard)

## Limit Granularity

| Granularity | Use When | Implementation |
|-------------|----------|----------------|
| Per IP | Public endpoints, unauthenticated | Reverse proxy / CDN layer |
| Per API key | App-level rate limiting | API gateway or middleware |
| Per user | User-level fairness | After authentication |
| Per endpoint | Protect expensive operations | Different limits per route |

- Layer multiple granularities: per-IP at edge + per-key at gateway + per-endpoint at service
- Stricter limits on write endpoints than read endpoints
- Separate limits for search/query endpoints (expensive)

## Client Backoff Requirements

When receiving 429:

1. **Honor `Retry-After`**: If the header is present, wait at least that long
2. **Exponential backoff**: `delay = min(cap, base * 2^attempt)`
3. **Jitter is NOT optional**: Without jitter, all throttled clients retry simultaneously (thundering herd)

### Backoff Formula

```
delay = min(max_delay, base_delay * 2^attempt) + random(0, base_delay)
```

- `base_delay`: 1 second
- `max_delay`: 30-60 seconds (cap prevents absurd waits)
- `random(0, base_delay)`: full jitter -- spread retries across the window
- If `Retry-After` header present: `delay = max(retry_after, calculated_delay)`

### Retry Budget

- Set a max retry count (e.g., 5 attempts)
- Track retry ratio -- if >10% of requests are retries, the client is misconfigured or the limit is too low
- Circuit break after sustained 429s rather than retrying indefinitely

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| 429 without Retry-After | Clients guess, usually too aggressively | Always include Retry-After |
| Backoff without jitter | Thundering herd on recovery | Add random jitter to every retry |
| Same limit for all endpoints | Expensive queries share budget with cheap reads | Per-endpoint or tiered limits |
| Rate limiting after auth only | Unauthenticated endpoints vulnerable to abuse | Per-IP limiting at edge |
| Immediate retry on 429 | Amplifies load on already-stressed server | Exponential backoff + jitter |
