# Python Async Patterns

Patterns for asynchronous programming with asyncio, async/await, and concurrent.futures.

---

## Core Patterns

| Pattern | Use Case |
|---------|----------|
| `async/await` | I/O-bound operations |
| `asyncio.gather(*coros)` | Run multiple coroutines concurrently |
| `asyncio.create_task(coro)` | Fire-and-forget with task handle |
| `asyncio.TaskGroup` (3.11+) | Structured concurrency with auto-cancel |
| `asyncio.to_thread(fn)` | Run blocking code in thread |
| `asyncio.Queue` | Producer-consumer patterns |
| `asyncio.Semaphore(n)` | Rate limiting concurrent ops |
| `asyncio.wait_for(coro, timeout)` | Timeout any coroutine |

---

## Basic Async/Await

```python
async def fetch_data(url: str) -> dict:
    async with httpx.AsyncClient() as client:
        response = await client.get(url)
        return response.json()

asyncio.run(main())  # Entry point
```

## Concurrent Execution

- Rule: Use `asyncio.gather(*tasks, return_exceptions=True)` to run multiple coroutines. Share a single `AsyncClient` session.
- 3.11+: Prefer `asyncio.TaskGroup` -- auto-cancels remaining tasks on failure.

## Task Cancellation

- Rule: Always cancel background tasks in `finally` block. Catch `asyncio.CancelledError` on `await task`.

## Timeouts

- Rule: `await asyncio.wait_for(coro, timeout=N)` raises `asyncio.TimeoutError`.

## Producer-Consumer

- Rule: Use `asyncio.Queue` with sentinel value (`None`) to signal completion. Call `queue.task_done()` after processing.

## Blocking Code Integration

- Rule: `await asyncio.to_thread(blocking_fn, *args)` or `loop.run_in_executor(pool, fn, *args)`.

## Semaphore Rate Limiting

```python
semaphore = asyncio.Semaphore(max_concurrent)
async def fetch_one(url):
    async with semaphore:
        return await do_fetch(url)
```

## Error Handling

- Rule: Use `return_exceptions=True` with `gather()`, then separate successes from errors by checking `isinstance(result, Exception)`.

## Pitfalls

### Async Function Never Awaited
**Severity**: high

- Rule: Calling async function without `await` returns a coroutine object, not the result. Use linters (`ruff`, `flake8-async`).

### Blocking Calls in Async Code
**Severity**: high

- Rule: Never use `requests`, `time.sleep()`, or blocking I/O in async code. Use `httpx.AsyncClient`, `asyncio.sleep()`, or `run_in_executor`.

### ExceptionGroup Handling (3.11+)
**Severity**: medium

- Rule: `asyncio.TaskGroup` raises `ExceptionGroup`. Use `except*` (PEP 654) to handle individual exceptions within the group.

## Anti-Patterns

| Anti-Pattern | Fix |
|--------------|-----|
| `asyncio.run()` in async context | Use `await` directly |
| Blocking calls in async (`requests`, `time.sleep`) | Use `httpx.AsyncClient`, `asyncio.sleep`, `to_thread` |
| Fire-and-forget without tracking | Store task references |
| No timeout on external calls | Use `asyncio.wait_for` |
| Missing `await` on async function | Linter: `ruff`, `flake8-async` |
