# Express.js Patterns

Patterns for building Express.js APIs with middleware, routing, and error handling.

## App Setup
**Severity**: high | **Category**: structure

- Rule: Apply security middleware first: `helmet()`, `cors()`, then parsing (`express.json({ limit: '10kb' })`), then logging (`morgan`), then routes, then 404 handler, then error handler (must be last).
- Gotcha: `express.json()` limit defaults to 100kb -- set explicit limit to prevent payload abuse.
- Rule: Add a `/health` endpoint returning `{ status: 'ok' }` for readiness probes.

## Router Pattern
**Severity**: medium | **Category**: structure

- Rule: Use `express.Router()` for modular route files. Mount at versioned paths (`/api/v1/users`).
- Rule: Apply middleware per-route: `router.put('/:id', authenticate, validateUser, controller.update)`.

## Async Handler
**Severity**: high | **Category**: reliability

- Rule: Wrap all async route handlers to catch rejected promises and forward to error middleware.

```javascript
const asyncHandler = (fn) => (req, res, next) => {
  Promise.resolve(fn(req, res, next)).catch(next);
};
```

- Gotcha: Without this wrapper, unhandled promise rejections crash the process or silently fail.

## Error Handling Middleware
**Severity**: high | **Category**: reliability

- Rule: Create an `AppError` class with `statusCode` and `isOperational` properties. Use 4-parameter middleware signature `(err, req, res, next)`.
- Rule: In production, return generic message for non-operational errors. In development, include stack trace.
- Gotcha: Express only recognizes error middleware by the 4-parameter signature -- all four params are required even if `next` is unused.

## Authentication Middleware
**Severity**: high | **Category**: security

- Rule: Extract Bearer token from `Authorization` header, verify with `jwt.verify()`, attach decoded payload to `req.user`.
- Rule: Create `authorize(...roles)` middleware factory for role-based access control.
- Gotcha: Always catch jwt.verify errors and return 401, not 500.

## Validation Middleware
**Severity**: high | **Category**: security

- Rule: Create a `validate(schema)` middleware factory. Use Joi or zod to validate `req.body`. Return 400 with all validation errors on failure.
- Rule: Use `abortEarly: false` (Joi) to collect all errors, not just the first.

## Rate Limiting
**Severity**: high | **Category**: security

- Rule: Use `express-rate-limit` on API routes. Set `windowMs` (e.g., 15 min), `max` requests, and `standardHeaders: true`.

```javascript
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, max: 100,
  standardHeaders: true, legacyHeaders: false,
});
app.use('/api/', limiter);
```

## Graceful Shutdown
**Severity**: medium | **Category**: reliability

- Rule: Listen for `SIGTERM`, call `server.close()` to stop accepting new connections, then `process.exit(0)` in the callback.
- Gotcha: Without graceful shutdown, in-flight requests are dropped during deployments.

For Express anti-patterns, see `anti-patterns.md`.
