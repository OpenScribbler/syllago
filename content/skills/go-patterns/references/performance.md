# Go Performance Patterns

Optimization patterns for allocation reduction, profiling, concurrency tuning, and compiler awareness.

---

## Allocation Reduction

### Preallocate Slices
- Rule: Use `make([]T, 0, len(input))` when size is known. Without capacity, `append` doubles the backing array, causing O(log n) allocations.

### sync.Pool for Temporary Objects
- Rule: Pool high-frequency, similar-sized allocations (buffers, temp structs) in hot paths.
- Gotcha: Pool contents may be collected at any GC. Don't rely on persistence.

```go
var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}
buf := bufPool.Get().(*bytes.Buffer)
defer func() { buf.Reset(); bufPool.Put(buf) }()
```

### strings.Builder for Concatenation
- Rule: Use `strings.Builder` instead of `+=` in loops. String concat is O(n^2); Builder uses amortized O(1) append.
- Tip: Call `.Grow(estimatedSize)` to reduce intermediate allocations.

### Avoid Slice-to-Interface Conversion
- Rule: Converting `[]int` to `[]any` requires new allocation + boxing each element. Use generics instead.

### Stack-Allocated Arrays for Small Buffers
- Rule: `var buf [64]byte` stays on stack. `make([]byte, 64)` typically escapes to heap.

### Struct Embedding to Reduce Allocations
- Rule: Embedded (value) fields = single allocation. Pointer fields = separate allocation.
- Trade-off: Embedding increases struct size. Use pointers for optional, shared, or large fields.

## Profiling

### CPU Profiling
```bash
# From running service (import _ "net/http/pprof")
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/profile?seconds=30
# From tests
go test -cpuprofile=cpu.prof -bench=. && go tool pprof -http=:8080 cpu.prof
```

### Memory Profiling
```bash
go tool pprof -inuse_space http://host:6060/debug/pprof/heap   # leaks
go tool pprof -alloc_space http://host:6060/debug/pprof/heap   # allocation-heavy code
```

### Escape Analysis
```bash
go-dev escape ./...                        # heap escapes only
GO_DEV_VERBOSE=1 go-dev escape ./...       # escapes + inlining
```

**Common escape triggers**: return pointer to local, store in interface, send to channel, `fmt.Println(x)`.

### Benchmarking
- Rule: Use `b.ResetTimer()` after setup. `b.ReportAllocs()` for allocation counts.

```bash
go-dev bench ./...                              # all benchmarks
go-dev bench-run BenchmarkX ./...               # specific, 5s benchtime
```

## Concurrency Performance

### Buffer Channels Appropriately
- Rule: Unbuffered = sender blocks until receiver. Size: `1` for signals, `runtime.NumCPU()` for CPU-bound pools.
- Measure with block profile if unsure.

### sync.RWMutex for Read-Heavy Workloads
- Rule: `RWMutex` allows unlimited concurrent readers. Significant for 95% read / 5% write.

### Shard Maps to Reduce Contention
- Rule: Distribute keys across N independent locks (e.g., 64 shards via FNV hash). Reduces contention ~Nx.

### atomic for Simple Counters
- Rule: `atomic.Int64` is 2-5x faster than mutex for single values. Use for counters, flags, simple state.

### Copy-on-Write for Read-Heavy Config
- Rule: `atomic.Pointer[Config]` gives lock-free reads. Writers create complete new value and swap atomically.

### errgroup for Bounded Concurrency
- Rule: `g.SetLimit(n)` caps goroutines. Handles errors and context cancellation.

### sync.Map vs Mutex Map
- Use `sync.Map`: stable key set, disjoint goroutine access.
- Use mutex map: frequent overlapping writes, iteration, atomic read-modify-write.

## I/O Performance

### bufio for Small I/O
- Rule: Batch many small writes with `bufio.NewWriter`. Can improve throughput 10-100x.

### io.Copy Instead of io.ReadAll
- Rule: `io.ReadAll` = O(n) memory. `io.Copy` = constant memory (32KB buffer). Stream when possible.

### HTTP Connection Pooling
- Rule: Default `MaxIdleConnsPerHost` is 2. Set to 10-20 for services making many requests to the same host.

## Data Structures

### Struct Field Ordering
- Rule: Order fields largest-to-smallest to minimize padding. For millions of objects, saves significant memory.

### Slice vs Map for Small Lookups
- Rule: Linear scan over a contiguous slice is faster than map hash+lookup for n < ~50 elements.

## Compiler Awareness

### Inlining
- Rule: Small, simple functions are inlined (eliminates call overhead). Check with `GO_DEV_VERBOSE=1 go-dev escape`.

### Bounds Check Elimination
- Rule: `for _, v := range s` eliminates bounds checks. Manual hint: `_ = s[3]` proves length once.

## Common Pitfalls

### Compile Regex Once
- Rule: `regexp.MustCompile` at package level, not inside functions. Compilation costs microseconds per call.

### Avoid Reflection in Hot Paths
- Rule: Reflect is 10-100x slower. Use generics or code generation.

### strconv vs fmt.Sprintf
- Rule: `strconv.Itoa(n)` avoids reflection/allocations that `fmt.Sprintf("%d", n)` incurs.

### JSON Optimization
- Rule: Reuse `json.NewEncoder`. Implement `MarshalJSON` for hot types. Consider `goccy/go-json` or `bytedance/sonic`.

## GC Tuning

### GOGC and GOMEMLIMIT
- **Containers**: Set `GOMEMLIMIT` to ~90-95% of container limit.
- **CPU-bound**: Increase `GOGC` (200-400) to trade memory for CPU.
- **Aggressive**: `GOGC=off GOMEMLIMIT=900MiB` relies entirely on memory limit.

### Non-Pointer Map Keys Skip GC Scanning
- Rule: `map[int64]int64` is invisible to GC. `map[string]*Record` forces GC to scan. Use index-based lookup to keep maps pointer-free.

## Diagnostic Quick Reference

| Symptom | Profile | Tool |
|---------|---------|------|
| High CPU | CPU | `pprof profile?seconds=30` |
| Growing memory | Heap (inuse) | `pprof -inuse_space heap` |
| High alloc rate | Heap (alloc) | `pprof -alloc_space heap` |
| Goroutine blocking | Block | `-blockprofile=block.prof` |
| Lock contention | Mutex | `SetMutexProfileFraction(1)` |
| Goroutine leak | Goroutine | `pprof goroutine` |
| GC pressure | Trace | `GODEBUG=gctrace=1` |
