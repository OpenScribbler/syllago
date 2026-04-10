# JavaScript/TypeScript Testing Patterns

## Framework Choice
- Rule: Prefer Vitest for new projects (faster, ESM-native, Vite-compatible). Use Jest for existing projects already using it.
- Gotcha: Vitest uses `vi.fn()` / `vi.mock()` / `vi.spyOn()`. Jest uses `jest.fn()` / `jest.mock()` / `jest.spyOn()`. API shapes are nearly identical.

## Test Structure
**Severity**: high | **Category**: organization

- Rule: Use `describe` for grouping, `it`/`test` for cases. Use `beforeEach` for setup, `afterEach` for cleanup.
- Rule: Co-locate unit tests (`*.test.ts`) next to source files. Put integration/e2e tests in `tests/` directory.
- Rule: Call `jest.clearAllMocks()` or `vi.clearAllMocks()` in `afterEach` to prevent mock state leaking between tests.

## Mocking
**Severity**: high | **Category**: isolation

- Rule: Use dependency injection -- pass mocks via constructor, not module-level mocking when possible.
- Rule: For module mocking, use `jest.mock('./module')` or `vi.mock('./module')` with factory function.
- Rule: For partial mocks, spread the actual module and override specific exports: `...jest.requireActual('./utils')` (Jest) or `...await vi.importActual('./utils')` (Vitest).
- Gotcha: `vi.mock()` calls are hoisted to the top of the file automatically. Variable references in the factory must use `vi.hoisted()`.

## HTTP Mocking with MSW
**Severity**: medium | **Category**: integration

- Rule: Use MSW (Mock Service Worker) v2 for HTTP mocking in tests. It intercepts at the network level, testing real fetch/axios calls.

```typescript
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';

const server = setupServer(
  http.get('/api/users/:id', ({ params }) => {
    return HttpResponse.json({ id: params.id, name: 'Test' });
  })
);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
```

- Rule: Use `server.use()` inside individual tests to override handlers for error scenarios.

## Async Testing
**Severity**: high | **Category**: correctness

- Rule: Always `await` assertions on async code. Use `await expect(fn()).resolves.toEqual()` or `await expect(fn()).rejects.toThrow()`.
- Rule: For timer-dependent code, use `vi.useFakeTimers()` / `jest.useFakeTimers()`, advance with `advanceTimersByTime()`, restore with `useRealTimers()`.
- Gotcha: Forgetting `await` on `expect(...).rejects` silently passes -- the test completes before the rejection is checked.

## Parameterized Tests
**Severity**: low | **Category**: structure

- Rule: Use `describe.each([...])` or `it.each([...])` for data-driven tests. Both Jest and Vitest support array-of-objects format.

## React Testing
**Severity**: medium | **Category**: frontend

- Rule: Use React Testing Library. Query by role/label (`getByRole`, `getByLabelText`) not by test ID or CSS class.
- Rule: Use `userEvent.setup()` over `fireEvent` for realistic user interactions (typing, clicking).
- Rule: Use `waitFor()` for assertions on async state changes. Use `screen.queryByX` (returns null) for asserting absence.
- Rule: Test hooks with `renderHook()` from `@testing-library/react`. Wrap state changes in `act()`.
- Gotcha: `getByX` throws if not found (good for presence assertions). `queryByX` returns null (good for absence assertions). `findByX` waits and returns a promise.

## Snapshot Testing
**Severity**: low | **Category**: regression

- Rule: Use inline snapshots (`toMatchInlineSnapshot`) over file snapshots for small values. Update with `npm test -- -u`.
- Trade-off: Snapshots catch unintended changes but are brittle -- prefer explicit assertions for important behavior.

## Configuration

- Rule: Jest: use `ts-jest` preset, set `testMatch`, configure `moduleNameMapper` for path aliases (`^@/(.*)$`).
- Rule: Vitest: use `defineConfig` from `vitest/config`, set `coverage.provider: 'v8'`, configure thresholds.
- Rule: Set coverage thresholds (80% lines/functions/branches) in config to enforce minimums in CI.
- Rule: Create `tests/setup.ts` for global setup (MSW server, custom matchers, test utilities). Reference it via `setupFilesAfterEnv` (Jest) or `setupFiles` (Vitest).
