# Python Performance & Concurrency

Profiling, optimization, and concurrency patterns for Python.

---

## Profiling Tools

| Tool | Best For | Install |
|------|----------|---------|
| `timeit` | Micro-benchmarks | stdlib |
| `cProfile` | Function-level CPU profiling | stdlib |
| `line_profiler` | Line-by-line timing | pip |
| `py-spy` | Production profiling (no code change) | pip |
| `scalene` | CPU + memory + GPU | pip |

- Rule: **Profile first, optimize second.** Use `cProfile` with `SortKey.CUMULATIVE` for bottlenecks, `SortKey.TIME` for hot loops.
- Use `min(timeit.repeat(...))` for micro-benchmarks -- higher values reflect system noise.

---

## Data Structure Performance

### Time Complexity

| Operation | list | dict/set | deque |
|-----------|------|----------|-------|
| `x in col` | O(n) | O(1) | O(n) |
| Index `col[i]` | O(1) | O(1) | O(n) |
| Append end | O(1) | O(1) | O(1) |
| Append left | O(n) | -- | O(1) |
| Pop end | O(1) | -- | O(1) |
| Pop left | O(n) | -- | O(1) |
| Sort | O(n log n) | -- | -- |

### Choosing the Right Structure

| Scenario | Best Structure | Why |
|----------|---------------|-----|
| Counting | `Counter` | `most_common()`, arithmetic ops |
| Grouping by key | `defaultdict(list)` | No KeyError, auto-creates |
| FIFO queue | `deque` | O(1) `popleft()` |
| Membership test | `set`/`frozenset` | O(1) vs list's O(n) |
| Immutable hashable | `frozenset` or `tuple` | Dict keys / set members |
| Lightweight records | `namedtuple` or `@dataclass(slots=True)` | Memory efficient |
| Layered config | `ChainMap` | No copying, first-found |

---

## Generators and itertools

- Rule: Use generators when iterating once without `len()` or indexing. O(1) memory regardless of input size.
- `yield from sub_generator`: Delegate to sub-generators.
- Generator pipelines: Compose generators for Unix pipe-like processing. Each stage processes one item at a time.

### Key itertools

| Function | Purpose |
|----------|---------|
| `chain.from_iterable()` | Flatten one level |
| `islice(it, n)` | Lazy slicing |
| `batched(it, n)` (3.12+) | Chunk processing |
| `groupby(sorted_data, key)` | Group consecutive items |
| `product(a, b)` | Cartesian product |

---

## Caching

### functools

| Decorator | Bounded | Thread-safe | Use Case |
|-----------|---------|-------------|----------|
| `@cache` (3.9+) | No | Yes | Pure functions, careful of memory |
| `@lru_cache(maxsize=N)` | Yes (LRU) | Yes | DB lookups, API calls |

### Gotchas
- Argument order matters: `add(1, 2)` and `add(b=2, a=1)` are different cache keys.
- Args must be hashable (no lists/dicts -- use tuples).
- `@lru_cache` on instance methods caches `self` -- prevents GC. Use `@staticmethod` + cache instead.

### cachetools
- Rule: Use `TTLCache(maxsize, ttl)` for time-based expiration. Use `LFUCache` for frequency-based eviction.

---

## Concurrency Decision Tree

```
I/O-bound?
  Many connections (100+) --> asyncio
  Few tasks, simple code --> ThreadPoolExecutor
  Blocking libraries --> asyncio.to_thread()
CPU-bound?
  NumPy/Pandas available --> Vectorize (no concurrency)
  Pure Python --> ProcessPoolExecutor
```

### Comparison

| Feature | threading | multiprocessing | asyncio |
|---------|-----------|-----------------|---------|
| Best for | I/O-bound | CPU-bound | High-concurrency I/O |
| GIL impact | Blocked for CPU | Bypassed | N/A |
| Memory/unit | ~8 KB | ~50-100 MB | ~1 KB |
| Max practical | ~1,000 | ~CPU count | ~100,000 |
| Start overhead | ~1 ms | ~50-200 ms | ~0.01 ms |

### Threading

```python
with ThreadPoolExecutor(max_workers=10) as executor:
    future_to_url = {executor.submit(fetch_url, url): url for url in urls}
    for future in as_completed(future_to_url):
        data = future.result()
```

- GIL released during I/O -- threads run truly concurrent for I/O-bound work.

### Multiprocessing
- Rule: Use `ProcessPoolExecutor` for CPU-bound work >100ms per task. Args/returns must be picklable.
- Always use `if __name__ == '__main__':` guard.

For asyncio patterns, see [async-patterns.md](async-patterns.md).

---

## Memory Optimization

### __slots__
- Rule: Use `__slots__ = ('x', 'y')` for classes with many instances (10,000+). ~60% memory savings.
- Gotcha: Subclass must also define `__slots__` or it gains `__dict__`. Incompatible with `@cached_property`.

### Object Size Reference

| Object | Bytes | Notes |
|--------|-------|-------|
| `int(0)` | 28 | Fixed overhead |
| `float` | 24 | Fixed |
| `""` | 49 | +1/char ASCII |
| `[]` | 56 | Empty list |
| `()` | 40 | Smaller than list |
| `{}` | 64 | Empty dict |
| `set()` | 216 | High base overhead |

- `sys.getsizeof()` measures container only, NOT referenced objects.
- Prefer tuples over lists for fixed-size data.

### Weak References
- Rule: Use `weakref.WeakValueDictionary()` for caches that should not prevent GC.
- Supported: class instances, functions, sets. NOT: `list`, `dict`, `tuple`, `int`, `str`.

---

## String Performance

- Rule: `"".join(strings)` for 4+ strings (O(n) vs `+=` O(n^2)). F-strings for 2-3 values.
- Pre-compile regexes with `re.compile()` for tight loops.
- Use `str.replace()`, `str.startswith()`, `str.isdigit()` over regex for simple operations.

---

## I/O Optimization

- Rule: Iterate file objects directly (`for line in f:`) -- lazy, memory-efficient. Never `f.readlines()` for large files.
- Use `mmap` for random access on large files -- zero-copy, O(1) access.
- Batch database inserts (1000x faster than individual). Use `writelines()` for multi-line output.

### Connection Pooling
- Use `httpx.AsyncClient` with `Limits(max_connections=100)` for HTTP.
- Use `requests.Session()` with `HTTPAdapter` for sync.
- Use httpx for: async, HTTP/2, fine-grained timeouts. Use requests for: simple sync scripts.

---

## Database Performance

### Connection Pooling (SQLAlchemy)
- Rule: Set `pool_size=5-20`, `max_overflow=10-30`, `pool_pre_ping=True`, `pool_recycle=1800`.

### N+1 Prevention

| Strategy | Queries | Best For |
|----------|---------|----------|
| `joinedload` | 1 (JOIN) | One-to-one, small sets |
| `selectinload` | 2 (SELECT + IN) | One-to-many, large sets |
| `subqueryload` | 2 (subquery) | Complex parent filters |
| `raiseload` | Error | Debugging |

### Batch Inserts

| Method | Relative Speed |
|--------|---------------|
| Individual inserts | 1x |
| Bulk executemany | 10x |
| Core insert | 25x |
| COPY protocol (PostgreSQL) | 100x |

### Query Optimization
- Select only needed columns, not full ORM objects.
- Use `exists()` for existence checks.
- Use keyset pagination (`WHERE id > last_id`) instead of `OFFSET` (O(1) vs O(offset)).
- Use `stream_results=True` for large result sets.

---

## NumPy/Pandas Performance

- Rule: **Vectorize first.** NumPy vectorized ops are 100x faster than Python loops.
- Never use `df.iterrows()` (500-1000x slower). Use vectorized ops, `np.where()`, or `np.select()`.
- Use `pd.to_numeric(col, downcast='integer')` and `astype('category')` to reduce memory 50-95%.
- Read only needed columns: `pd.read_csv('f.csv', usecols=[...], dtype={...})`.
- Use `chunksize=N` for files that don't fit in memory.
- Use Numba `@jit(nopython=True)` for custom functions needing near-C speed.

---

## Anti-Patterns Quick Reference

| Anti-Pattern | Fix |
|-------------|-----|
| `list.pop(0)` in loop | `deque.popleft()` |
| `if x in large_list` | Convert to `set` |
| String `+=` in loop | `"".join()` |
| `df.iterrows()` | Vectorized ops / `np.where` |
| `SELECT *` from ORM | Select specific columns |
| No connection pooling | SQLAlchemy pool |
| `time.time()` for benchmarks | `timeit` / `time.perf_counter()` |
| `lru_cache` on methods | `staticmethod` + cache |
| `asyncio.gather()` without limits | `asyncio.Semaphore` |
| `readlines()` on large files | Iterate file object directly |
