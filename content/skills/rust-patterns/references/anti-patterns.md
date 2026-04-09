# Rust Anti-Patterns

## Ownership and Borrowing

### Clone to Satisfy Borrow Checker
**Severity**: high | **Category**: ownership

- Rule: Do not add `.clone()` to silence borrow errors -- restructure code to reduce borrow scope or use references
- Fix: Decompose structs to separate borrow targets; use `Cow<T>` for conditional ownership
- Gotcha: Cloning an `Arc` is cheap (reference count bump), but cloning the inner data is not

### Rc/RefCell Abuse
**Severity**: high | **Category**: ownership

- Rule: Do not blanket-wrap fields in `Rc<RefCell<T>>` to avoid ownership design
- Fix: Redesign data structures -- use indices for graph structures, split state into focused structs, consider arena allocation
- Gotcha: `RefCell` panics at runtime on double-borrow; `Mutex` deadlocks -- both hide design problems

---

## Error Handling

### Unwrap in Production
**Severity**: high | **Category**: correctness

- Rule: Reserve `.unwrap()` / `.expect()` for tests and provably-impossible cases only
- Fix: Use `?` for propagation, `match` or combinators for explicit handling

### Stringly-Typed Errors
**Severity**: high | **Category**: error design

- Rule: Never use `Result<(), String>` -- callers cannot match on error variants
- Fix: Define typed error enums with thiserror

### Swallowing Errors
**Severity**: high | **Category**: correctness

- Rule: Never silently discard errors with `let _ = fallible_call()`
- Fix: Log with `if let Err(e) = ...` or propagate with `?`

---

## Type System

### Stringly-Typed Code
**Severity**: high | **Category**: type safety

- Rule: Do not use `String` for IDs, states, or categories -- use newtypes and enums
- Fix: `struct AccountId(String)`, `enum OrderStatus { Pending, Shipped, Delivered }`

### Deref Polymorphism
**Severity**: medium | **Category**: design

- Rule: Do not implement `Deref` for inheritance-like behavior (Admin derefs to User)
- Fix: Use composition with explicit delegation methods
- Gotcha: `Deref` is for smart pointers only; misuse breaks trait object dispatch

---

## Build and CI

### deny(warnings) in Source
**Severity**: medium | **Category**: CI

- Rule: Never put `#![deny(warnings)]` in source files -- new compiler versions add warnings and break builds
- Fix: Set `RUSTFLAGS="-D warnings"` in CI only

### Not Using Clippy
**Severity**: medium | **Category**: CI

- Rule: Always run `cargo clippy -- -D warnings` in CI

---

## Concurrency

### Blocking in Async Context
**Severity**: high | **Category**: concurrency

- Rule: Never call `std::fs::*`, `std::thread::sleep`, or other blocking ops inside async functions
- Fix: Use `tokio::fs::*`, `tokio::time::sleep`, or `tokio::task::spawn_blocking` for unavoidable blocking

### Holding Lock Across Await
**Severity**: high | **Category**: concurrency

- Rule: Never hold a `MutexGuard` across `.await` points -- causes deadlocks with single-threaded runtimes
- Fix: Fetch async data first, then lock briefly to update

### Unbounded Channels
**Severity**: medium | **Category**: concurrency

- Rule: Prefer bounded channels (`mpsc::channel(N)`) over unbounded -- unbounded can exhaust memory under backpressure

---

## Performance

### Unnecessary Allocations
**Severity**: medium | **Category**: performance

- Rule: Reuse buffers across loop iterations with `.clear()` instead of allocating new collections each time
- Rule: Pre-allocate with known sizes -- see performance.md for collection selection and allocation patterns

### Box<dyn Trait> When Generics Suffice
**Severity**: low | **Category**: performance

- Rule: Prefer generics (`impl Trait` / `<T: Trait>`) for static dispatch when the concrete type is known at each call site
- Rule: Use `Box<dyn Trait>` only for heterogeneous collections or when reducing binary size matters

---

## API Design

### Taking Ownership When Borrowing Suffices
**Severity**: medium | **Category**: API

- Rule: Accept `&str` not `String`, `&[T]` not `Vec<T>` when the function only reads data
- Fix: Take ownership only when the function needs to store or move the value

### Exposing Internal Mutability
**Severity**: medium | **Category**: encapsulation

- Rule: Do not return `&mut Vec<Item>` -- expose focused methods (`items() -> &[Item]`, `add_item()`)
