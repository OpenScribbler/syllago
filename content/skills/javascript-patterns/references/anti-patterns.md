# JavaScript/TypeScript Anti-Patterns

Common anti-patterns and their fixes. For language-level footguns, see `gotchas.md`.

## General

| Anti-Pattern | Fix |
|--------------|-----|
| Callback hell / deeply nested `.then()` | Refactor to `async`/`await` |
| Using `any` type liberally | Use `unknown` + type guards, or define proper types |
| Deeply nested ternaries | Extract to named variables or early returns |
| No error boundaries in React | Add `ErrorBoundary` components at route/feature level |
| Giant utility files | Split into focused modules by domain |
| Mutable shared state | Use immutable patterns, avoid side effects in pure functions |

## Express / API

| Anti-Pattern | Fix |
|--------------|-----|
| No async error handling | Use `asyncHandler` wrapper on all async routes |
| Error details in production responses | Check `NODE_ENV`, return generic messages in production |
| No request validation | Validate all inputs with zod/Joi schema |
| Blocking middleware (sync I/O) | Use async operations, avoid `fs.readFileSync` in handlers |
| No rate limiting | Add `express-rate-limit` to API routes |

## Bundling

| Anti-Pattern | Fix |
|--------------|-----|
| No code splitting | Use dynamic `import()` for route-level splitting |
| Bundling node_modules twice | Configure `splitChunks` vendor cacheGroup |
| No tree shaking | Use ES modules, avoid `export default` barrel objects |
| Missing source maps | Enable `sourcemap: true` in build config |
| No cache busting | Use `[contenthash]` in output filenames |

## Testing

| Anti-Pattern | Fix |
|--------------|-----|
| Testing implementation details | Test behavior and outputs, not internal state |
| No mocking of external services | Use MSW for HTTP mocking, `vi.mock()` for modules |
| Snapshot overuse | Use targeted assertions; snapshots for stable UI only |
| No async cleanup | Use `afterEach` to restore mocks and timers |
