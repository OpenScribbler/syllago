# Resilience Patterns for Go Services

Patterns for retry, circuit breaker, timeout, connection pooling, and graceful degradation.

---

## Retry with Exponential Backoff

- Rule: Always use jitter (e.g., +/-25%) to prevent thundering herd. Respect context cancellation between retries.

```go
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
    var result T
    var lastErr error
    delay := cfg.BaseDelay
    for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
        result, lastErr = fn()
        if lastErr == nil { return result, nil }
        if attempt == cfg.MaxRetries { break }
        jitter := delay / 4
        sleepTime := delay + time.Duration(rand.Int63n(int64(jitter*2))) - jitter
        select {
        case <-ctx.Done(): return result, ctx.Err()
        case <-time.After(sleepTime):
        }
        delay = min(time.Duration(float64(delay)*cfg.Multiplier), cfg.MaxDelay)
    }
    return result, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

- Typical config: `MaxRetries: 3`, `BaseDelay: 100ms`, `MaxDelay: 5s`, `Multiplier: 2.0`.

## Circuit Breaker

- Rule: Track failures with mutex. States: closed (normal) -> open (fail fast) -> half-open (probe).
- Open when failures >= threshold. Reset to half-open after timeout. Close on first success in half-open.
- Gotcha: Lock mutex for state checks but release before calling the wrapped function.

## Timeout Patterns

### HTTP Client
- Rule: Set multiple timeout layers: overall `Client.Timeout`, `DialContext.Timeout`, `TLSHandshakeTimeout`, `ResponseHeaderTimeout`.

```go
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
        TLSHandshakeTimeout:  5 * time.Second,
        ResponseHeaderTimeout: 10 * time.Second,
        MaxIdleConnsPerHost:   10,
    },
}
```

### Context Timeout
- Rule: Create per-request context with `context.WithTimeout`. Check `ctx.Err() == context.DeadlineExceeded` for timeout-specific handling.

## Connection Pooling

### Database
```go
db.SetMaxOpenConns(25)                  // max concurrent
db.SetMaxIdleConns(5)                   // keep warm
db.SetConnMaxLifetime(5 * time.Minute)  // recycle
db.SetConnMaxIdleTime(1 * time.Minute)  // close idle
```

### gRPC
- Rule: Configure `keepalive.ClientParameters` with `Time`, `Timeout`, `PermitWithoutStream`. Use `round_robin` load balancing with health checks.

## Graceful Degradation

- Rule: Layer fallbacks: cache -> primary DB -> replica -> stale cache -> error.
- Log degraded responses with warning level so operators know.

## Idempotency

- Rule: Use an idempotency key to deduplicate operations. Check key before processing, store result with key after (TTL: 24h typical).
- Gotcha: The idempotency key check + store should be atomic or use optimistic concurrency control.
