# JavaScript/TypeScript Code Patterns

## TypeScript Configuration
**Severity**: high | **Category**: setup

- Rule: Use `strict: true` in tsconfig.json. Target ES2022+, use `NodeNext` module/moduleResolution for Node.js projects, `ESNext`/`bundler` for frontend projects.
- Gotcha: Set `skipLibCheck: true` to avoid type errors from third-party `.d.ts` files.

## Error Handling
**Severity**: high | **Category**: reliability

- Rule: Define typed error classes extending `Error` with `code` and `statusCode` properties. Create domain-specific subclasses (ValidationError, NotFoundError).
- Gotcha: Always set `this.name` in the constructor -- stack traces use it for identification.

```typescript
class AppError extends Error {
  constructor(
    message: string,
    public readonly code: string,
    public readonly statusCode: number = 500
  ) {
    super(message);
    this.name = 'AppError';
  }
}
```

## Async/Await Patterns
**Severity**: high | **Category**: concurrency

- Rule: Use `async/await` over callbacks and raw `.then()` chains. Use `for...of` for sequential execution, `Promise.all()` for parallel.
- Rule: For parallel with concurrency limit, chunk the array and `Promise.all()` each chunk sequentially.
- Rule: Use `AbortController` with `setTimeout` for fetch timeouts -- see `gotchas.md` for the snippet and cleanup pattern.
- Gotcha: `Promise.all()` fails fast on first rejection. Use `Promise.allSettled()` when you need all results regardless of failures.

## HTTP Client with Retry
**Severity**: medium | **Category**: resilience

- Rule: Implement exponential backoff with jitter. Only retry 5xx errors, not 4xx client errors.
- Rule: Use `Math.min(baseDelay * 2^attempt, maxDelay)` for backoff calculation.
- Gotcha: Always set a max retry count (typically 3) and a max delay cap.

## Configuration with Zod
**Severity**: high | **Category**: safety

- Rule: Validate `process.env` at startup using zod schemas. Fail fast on missing/invalid config.

```typescript
import { z } from 'zod';
const configSchema = z.object({
  NODE_ENV: z.enum(['development', 'production', 'test']).default('development'),
  PORT: z.coerce.number().default(3000),
  DATABASE_URL: z.string().url(),
});
export const config = configSchema.parse(process.env);
```

## Module System
**Severity**: medium | **Category**: structure

- Rule: Use ES modules (`import`/`export`) over CommonJS (`require`). Set `"type": "module"` in package.json for Node.js ESM.
- Gotcha: ESM requires file extensions in imports for Node.js (`import './foo.js'`). TypeScript with `NodeNext` enforces this.
