# Modern Go Patterns (1.21-1.23+)

Features and patterns for Go 1.21+ including generics, iterators, slog, and new stdlib packages.

---

## Generics (Go 1.18+)

### When to Use
- Rule: Use generics when writing identical logic for multiple types (containers, algorithms). Prefer `slices.*` and `maps.*` stdlib functions over custom implementations.
- Don't: Replace interfaces with type parameters. If a function takes `io.Reader`, keep it as `io.Reader`, not `[T io.Reader]`.
- Don't: Add type parameter used only once (e.g., `func Print[T any](v T)` -- just use `any`).

### Type Constraints
- `comparable`: types supporting `==` and `!=`.
- `cmp.Ordered` (Go 1.21+): types supporting `<`, `>`, etc.
- Custom: `type Number interface { ~int | ~int64 | ~float64 }`.

### Generic Set Type (Common Pattern)
```go
type Set[E comparable] struct{ m map[E]struct{} }
func (s *Set[E]) Add(v E)      { s.m[v] = struct{}{} }
func (s *Set[E]) Has(v E) bool { _, ok := s.m[v]; return ok }
```

## Iterators (Go 1.23)

### Push Iterators
- `iter.Seq[V]` = `func(yield func(V) bool)` for single-value iteration.
- `iter.Seq2[K,V]` = `func(yield func(K,V) bool)` for key-value iteration.
- Rule: Always check `!yield(v)` return and exit early if false.

```go
func (s *Stack[E]) All() iter.Seq[E] {
    return func(yield func(E) bool) {
        for i := len(s.items) - 1; i >= 0; i-- {
            if !yield(s.items[i]) { return }
        }
    }
}
// Usage: for v := range stack.All() { ... }
```

### Stdlib Iterator Functions
```go
slices.All(s)        // iter.Seq2[int, E]
slices.Values(s)     // iter.Seq[E]
slices.Collect(seq)  // iter.Seq[E] -> []E
slices.Sorted(seq)   // iter.Seq[E] -> sorted []E
maps.Keys(m)         // iter.Seq[K]
maps.Values(m)       // iter.Seq[V]
maps.Collect(seq2)   // iter.Seq2[K,V] -> map[K]V
```

## Structured Logging with slog (Go 1.21)

### Setup
- Production: `slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))`
- Development: `slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))`

### Key Patterns
- **Contextual logging**: `logger := slog.With("request_id", id, "method", method)` -- adds fields to all subsequent log calls.
- **Attribute groups**: `slog.Group("request", slog.String("method", m))` -- nests fields in JSON output.
- **Redaction**: Implement `LogValue() slog.Value` to redact sensitive fields (passwords, tokens).
- **High-perf**: `slog.LogAttrs(ctx, level, msg, slog.Int("count", n))` -- zero allocation for disabled levels.

## Standard Library Additions

### slices Package (Go 1.21)
`Sort`, `Contains`, `Index`, `BinarySearch`, `Compact` (dedup), `Reverse`, `Delete`, `Insert`, `Equal`, `Clone`, `Concat`.

### maps Package (Go 1.21)
`Clone`, `Copy` (merge), `Equal`, `DeleteFunc`.

### cmp.Or (Go 1.22)
- Returns first non-zero value (like SQL COALESCE): `name := cmp.Or(input, envVar, "default")`
- Multi-key sorting: `cmp.Or(cmp.Compare(a.Priority, b.Priority), strings.Compare(a.Name, b.Name))`

### Builtins (Go 1.21+)
- `min(a, b, c...)` / `max(a, b, c...)` -- variadic, work on any ordered type.
- `clear(m)` -- deletes all map entries or zeros all slice elements.

### Range Over Integer (Go 1.22)
```go
for i := range 10 { ... }   // 0..9
for range 3 { retry() }     // repeat N times
```

### Loop Variable Fix (Go 1.22)
- Each iteration creates new variables. Goroutine closures are safe. Requires `go 1.22` in go.mod.

### Enhanced HTTP Routing (Go 1.22)
```go
mux.HandleFunc("GET /posts/{id}", handler)     // method + path param
mux.HandleFunc("GET /files/{path...}", serve)  // wildcard
mux.HandleFunc("GET /posts/{$}", index)        // exact match
```

### errors.Join (Go 1.20)
- Combine multiple errors: `errors.Join(errs...)`. Returns nil if all nil.
- `errors.Is`/`errors.As` work through joined errors.

## Quick Reference

| Feature | Since | Key Function |
|---------|-------|-------------|
| `slices.Contains` | 1.21 | Check slice membership |
| `maps.Clone` | 1.21 | Shallow copy map |
| `slog` | 1.21 | Structured logging |
| `min`/`max`/`clear` | 1.21 | Builtins |
| `cmp.Or` | 1.22 | First non-zero value |
| `range int` | 1.22 | `for i := range 10` |
| Loop var fix | 1.22 | Per-iteration scope |
| HTTP routing | 1.22 | Method + path patterns |
| `iter.Seq` | 1.23 | Push iterators |
| `slices.Collect` | 1.23 | Iterator to slice |
