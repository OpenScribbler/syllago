# Go Anti-Patterns

Design-level code smells relevant when **reviewing** Go code. For language-level gotchas (writing code), see [gotchas.md](gotchas.md).

---

## Error Handling

### Swallowing Errors
**Severity**: high

- Rule: Never discard errors with `_`. Every `_` hides a potential failure leading to corrupt state or silent data loss.
- Fix: Handle, wrap with `fmt.Errorf("context: %w", err)`, or explicitly document why ignoring is safe.

### String Comparison of Errors
**Severity**: high

- Rule: Never compare `err.Error() == "..."`. Error messages are for humans and change without notice.
- Fix: Use `errors.Is(err, ErrSentinel)` or `errors.As(err, &typedErr)`.

### Panic for Control Flow
**Severity**: high

- Rule: Reserve `panic` for truly unrecoverable situations (programmer bugs, impossible states). Library code must return errors.
- Exception: `Must*` constructors called only during init (e.g., `regexp.MustCompile`).

### Error String Formatting
**Severity**: low

- Rule: Error strings should be lowercase, no trailing punctuation. They compose via wrapping: `"connecting: opening socket: permission denied"`.

## Interface Design

### Overly Large Interfaces
**Severity**: high

- Rule: Keep interfaces to 1-3 methods. Large interfaces (5+ methods) are hard to implement, mock, and compose.
- Fix: Split into focused interfaces; compose with embedding when needed.

### Stuttering Names
**Severity**: low

- Rule: Package name provides context. `user.UserService` stutters; prefer `user.Service`.
- Applies to types, functions, and constants.

## Package Design

### Utility/Helper Packages
**Severity**: medium

- Rule: Packages named `utils`, `helpers`, `common` are code smells. They accumulate unrelated functions.
- Fix: Name packages by what they provide (`timeformat`, `auth`, `config`).

### Import Side Effects Without Comment
**Severity**: medium

- Rule: Blank imports (`_ "pkg"`) execute `init()` for side effects. Always document what they register.

```go
_ "github.com/lib/pq"   // registers PostgreSQL driver
_ "net/http/pprof"       // registers /debug/pprof handlers
```

## Concurrency

### Goroutine Leak - No Exit Signal
**Severity**: high

- Rule: Every goroutine must have a clear exit path. Unbuffered channel sends block forever if nobody reads.
- Fix: Use buffered channels, context cancellation, or done channels. Verify with goroutine profile.

### Missing WaitGroup
**Severity**: high

- Rule: Fire-and-forget goroutines cause the parent function to return before work completes.
- Fix: Use `sync.WaitGroup` or `errgroup.Group` to synchronize completion.

## Init Functions

### Complex Logic in init()
**Severity**: high

- Rule: `init()` with I/O, network calls, or complex logic makes testing difficult and hides dependencies.
- Fix: Use explicit constructors called from `main()`. Reserve `init()` for simple variable/constant setup.

### Global Mutable State via init()
**Severity**: high

- Rule: Global state initialized in `init()` creates hidden coupling. Tests can't override it without race conditions.
- Fix: Return values from constructor functions. Pass dependencies explicitly.

## Memory

### String Concatenation in Loops
**Severity**: medium

- Rule: `+=` on strings in loops is O(n^2) allocations. Strings are immutable; each append copies everything.
- Fix: Use `strings.Builder` with optional `.Grow(n)` pre-sizing.

### Not Preallocating Slices
**Severity**: medium

- Rule: `append` without preallocated capacity causes O(log n) reallocations as the backing array doubles.
- Fix: `make([]T, 0, len(input))` when the size is known or estimable.
