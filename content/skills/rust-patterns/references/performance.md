# Rust Performance Patterns

## Zero-Cost Abstractions

- Rule: Iterator chains compile to efficient loops with no intermediate allocations -- prefer `.iter().filter().map().collect()` over manual indexing loops
- Rule: Generics are monomorphized (specialized at compile time) -- zero runtime dispatch overhead
- Rule: Closures passed to iterator methods can be inlined by the compiler

---

## Collection Selection

| Need | Use | Alternative |
|------|-----|-------------|
| Ordered sequence | `Vec<T>` | `VecDeque` for front insertion |
| Key-value lookup | `HashMap<K, V>` | `BTreeMap` for sorted keys |
| Unique values | `HashSet<T>` | `BTreeSet` for sorted |
| FIFO queue | `VecDeque<T>` | - |
| Priority queue | `BinaryHeap<T>` | - |
| Small fixed size | `[T; N]` | `SmallVec` for stack/heap hybrid |

### Complexity

| Operation | Vec | HashMap | BTreeMap |
|-----------|-----|---------|----------|
| Get by index | O(1) | N/A | N/A |
| Get by key | O(n) | O(1)* | O(log n) |
| Insert | O(1)** | O(1)* | O(log n) |
| Remove | O(n) | O(1)* | O(log n) |

*Amortized, **At end

---

## Avoiding Allocations

### Buffer Reuse
**Severity**: medium | **Category**: allocation

- Rule: Allocate buffers outside loops, use `.clear()` inside -- `.clear()` keeps capacity
- Rule: Use `Vec::with_capacity(n)` / `String::with_capacity(n)` when size is known or estimable
- Rule: `.collect()` on iterators with `size_hint()` auto-pre-allocates

### Cow for Conditional Ownership
**Severity**: medium | **Category**: allocation

- Rule: Return `Cow<'_, str>` when a function sometimes modifies input and sometimes returns it unchanged -- avoids unnecessary cloning on the no-modification path

```rust
fn process(input: &str) -> Cow<'_, str> {
    if input.contains("bad") {
        Cow::Owned(input.replace("bad", "good"))
    } else {
        Cow::Borrowed(input) // no allocation
    }
}
```

### SmallVec
**Severity**: low | **Category**: allocation

- Rule: Use `SmallVec<[T; N]>` for collections that are usually small but occasionally large -- stores up to N elements on the stack

---

## String Performance

- Rule: Use `format!()` over `+` concatenation or multiple `push_str` calls
- Rule: Accept `&str` for read-only access; accept `impl Into<String>` when ownership is needed
- Rule: Compile regexes once with `once_cell::sync::Lazy` or `std::sync::LazyLock` -- never inside a function body

```rust
static RE: std::sync::LazyLock<Regex> = std::sync::LazyLock::new(|| {
    Regex::new(r"\d+").unwrap()
});
```

---

## Iterator Performance

- Rule: Prefer `for item in &items` over `for i in 0..items.len()` -- avoids bounds checks
- Rule: Chain iterator adapters (`.filter().map().sum()`) instead of collecting into intermediate `Vec`s
- Rule: Use `.any()`, `.all()`, `.find()`, `.count()` instead of collecting then checking

---

## Concurrency Performance

### Synchronization Selection

| Pattern | Use When | Speed |
|---------|----------|-------|
| `AtomicUsize` | Counters, flags | Fast |
| `Mutex<T>` | Complex state, short holds | Medium |
| `RwLock<T>` | Read-heavy workloads | Medium |
| Channels | Message passing between tasks | Medium |

- Rule: Use atomics for simple counters instead of `Arc<Mutex<usize>>`
- Rule: Minimize lock scope -- fetch async data before acquiring lock
- Rule: Use `rayon::par_iter()` for CPU-bound data parallelism (drop-in replacement for `.iter()`)

---

## Memory Layout

- Rule: Rust may reorder struct fields for optimal packing -- use `#[repr(C)]` only when C-compatible layout is required
- Rule: Use smaller integer types (`u32`, `u16`) when the value range allows -- reduces struct size and improves cache locality
- Rule: Box large arrays/buffers to avoid stack overflow: `Box<[u8]>` via `vec![0u8; N].into_boxed_slice()`

---

## Benchmarking

- Rule: Use Criterion for benchmarks, never `std::time::Instant` in a loop
- Rule: Profile before optimizing -- use `cargo flamegraph` for hotspot analysis

```toml
# Cargo.toml
[dev-dependencies]
criterion = "0.5"

[[bench]]
name = "benchmark"
harness = false
```
