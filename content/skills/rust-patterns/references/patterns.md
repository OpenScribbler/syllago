# Rust Code Patterns

## Core Idioms

### Constructors
**Severity**: medium | **Category**: idiom

- Rule: Use `fn new() -> Self` as primary constructor; use `from_*` for alternative constructors (e.g., `from_env()`)
- Rule: Accept `impl Into<String>` for owned string params in constructors to avoid forcing callers to call `.to_string()`

### Default Trait
**Severity**: medium | **Category**: idiom

- Rule: Use `#[derive(Default)]` when zero values are correct; implement `Default` manually for custom defaults
- Rule: Use struct update syntax for partial initialization: `Config { port: 3000, ..Default::default() }`

### String Building
**Severity**: low | **Category**: idiom

- Rule: Use `format!()` for readable string construction instead of multiple `push_str` calls
- Rule: Use `String::with_capacity()` when building strings of known size in hot paths

### mem::take and mem::replace
**Severity**: medium | **Category**: ownership

- Rule: Use `mem::take()` to move a value out of a mutable reference, leaving `Default::default()` behind
- Rule: Use `mem::replace()` to swap a value out, returning the old one -- avoids borrow checker conflicts

```rust
fn take_data(&mut self) -> Vec<u8> {
    std::mem::take(&mut self.data) // leaves empty Vec
}
fn replace_data(&mut self, new: Vec<u8>) -> Vec<u8> {
    std::mem::replace(&mut self.data, new) // returns old
}
```

### Iterating over Option
**Severity**: low | **Category**: idiom

- Rule: `Option` implements `IntoIterator` (0 or 1 items) -- use `.chain(maybe_value)` to conditionally include values in iterator chains

---

## Design Patterns

### Builder Pattern
**Severity**: medium | **Category**: design

- Rule: Use builder for types with 3+ optional fields. Builder methods take `mut self` and return `Self`. `build()` returns `Result<T, E>` to validate required fields.
- Gotcha: Derive `Default` on the builder, not the target type, so `build()` can enforce required fields

### Newtype Pattern
**Severity**: high | **Category**: type safety

- Rule: Wrap primitive types (`UserId(u64)`, `Email(String)`) to prevent argument mixups at compile time with zero runtime cost
- Rule: Implement `Deref` only when the newtype truly IS-A wrapper (like smart pointers), not for inheritance-like behavior

### Type State Pattern
**Severity**: medium | **Category**: design

Use zero-sized marker types + `PhantomData` to encode state transitions at compile time:

```rust
struct Disconnected;
struct Connected;

struct Connection<State> {
    addr: String,
    _state: std::marker::PhantomData<State>,
}

impl Connection<Disconnected> {
    fn connect(self) -> Result<Connection<Connected>, Error> {
        Ok(Connection { addr: self.addr, _state: PhantomData })
    }
}

impl Connection<Connected> {
    fn query(&self, sql: &str) -> Result<Data, Error> { todo!() }
}
// conn.query() only compiles on Connection<Connected>
```

### RAII Guards
**Severity**: medium | **Category**: resource management

- Rule: Wrap resources in structs that implement `Drop` for automatic cleanup (temp files, locks, connections)
- Rule: Return a guard from acquisition functions; cleanup runs when guard goes out of scope

### Retry Pattern
**Severity**: medium | **Category**: resilience

- Rule: Implement async retry with exponential backoff and max attempts. Cap delay with `.min(max_delay)`.
- Rule: Accept `FnMut() -> Future` for the operation to allow retrying closures

### Shared State
**Severity**: high | **Category**: concurrency

- Rule: Use `Arc<Mutex<T>>` (or `Arc<RwLock<T>>` for read-heavy) for shared mutable state across async tasks
- Rule: Use `tokio::sync::Mutex` (not `std::sync::Mutex`) when lock is held across `.await` points
- Gotcha: Minimize lock scope -- fetch data first, then lock briefly to update. See anti-patterns.md for lock-across-await.
