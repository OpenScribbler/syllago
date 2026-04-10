# Go Language Gotchas

Language-level footguns relevant when **writing** Go code. For design-level anti-patterns (code review), see [anti-patterns.md](anti-patterns.md).

---

### Nil Interface vs Nil Pointer
**Severity**: high | **Category**: types

- Rule: Never return a typed nil pointer as an `error` interface. An interface is nil only when both type and value are unset; a nil pointer creates a non-nil interface `(T=*MyError, V=nil)`.
- Fix: Always `return nil` explicitly for the "no error" path.

### Slice Append Shared Backing Array
**Severity**: high | **Category**: slices

- Rule: Sub-slices share the backing array. `append` on a sub-slice with remaining capacity corrupts the original.
- Fix: Use full slice expression `s[:3:3]` to cap capacity, or `copy` into a new slice.

### Slice Memory Leak from Sub-Slicing
**Severity**: medium | **Category**: slices

- Rule: A small sub-slice keeps the entire backing array alive, preventing GC of the larger allocation.
- Fix: Copy the needed bytes into a new slice before returning.

### Writing to a Nil Map Panics
**Severity**: high | **Category**: maps

- Rule: Nil maps allow reads (return zero values) but panic on writes. Always initialize with `make()`.

### Map Iteration Order is Random
**Severity**: medium | **Category**: maps

- Rule: Go intentionally randomizes map iteration. If deterministic order matters, sort the keys first.

### Defer Arguments Evaluated Immediately
**Severity**: medium | **Category**: defer

- Rule: Arguments to a deferred function call are evaluated at the `defer` statement, not when the deferred function executes.
- Fix: Wrap in a closure: `defer func() { ... }()`.

### Defer in Loops
**Severity**: high | **Category**: defer

- Rule: `defer` runs when the enclosing *function* returns, not when the loop iteration ends. Resources accumulate across iterations.
- Fix: Extract loop body to a separate function so `defer` fires per iteration.

### Copying Structs with Mutex
**Severity**: high | **Category**: concurrency

- Rule: `sync.Mutex` (and all sync types) must not be copied after first use. Value receivers copy the struct.
- Fix: Use pointer receivers for any type containing sync primitives.

### Loop Variable Capture (Pre-Go 1.22)
**Severity**: high | **Category**: closures

- Rule: Before Go 1.22, loop variables had per-loop scope. Closures (especially goroutines) all captured the same variable.
- Fix: Go 1.22+ fixes this automatically (requires `go 1.22` in go.mod). For older code: pass as argument or shadow with `v := v`.

### Using math/rand for Security
**Severity**: high | **Category**: security

- Rule: `math/rand` is predictable. Use `crypto/rand` for tokens, session IDs, API keys, and any security-sensitive randomness.

### Concurrent Map Access
**Severity**: high | **Category**: concurrency

- Rule: Go maps are not safe for concurrent read+write. Concurrent access causes `fatal error: concurrent map read and map write`.
- Fix: Use `sync.RWMutex` or `sync.Map`.

### Not Closing HTTP Response Bodies
**Severity**: high | **Category**: resources

- Rule: Response bodies hold TCP connections. Failing to close them exhausts the connection pool.
- Fix: Always `defer resp.Body.Close()` immediately after the error check.

### Unbounded io.ReadAll
**Severity**: high | **Category**: security

- Rule: `io.ReadAll` has no size limit. Untrusted input can exhaust memory.
- Fix: Wrap with `io.LimitReader(r, maxSize)`.

### Context Stored in Struct
**Severity**: medium | **Category**: design

- Rule: Context should be per-call, not per-struct. Store context in struct fields and it becomes stale, breaking cancellation chains.
- Fix: Pass `ctx context.Context` as the first parameter to methods.
