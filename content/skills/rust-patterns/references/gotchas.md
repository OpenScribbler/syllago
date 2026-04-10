# Rust Gotchas

Language-specific footguns and subtle bugs. High-value reminders even for experienced Rust developers.

---

## Borrow Checker Surprises

### Temporary Lifetime in Match
**Severity**: high

- Gotcha: Temporaries created in a `match` scrutinee are dropped at the end of the match expression, not the match block -- can cause use-after-free with references to temporaries
- Fix: Bind the temporary to a `let` variable before the match

### NLL Edge Cases
**Severity**: medium

- Gotcha: Non-lexical lifetimes (NLL) usually help, but borrows through function calls can still surprise -- `vec.push(vec.len())` fails because `vec` is mutably borrowed by `push` while `len()` needs a shared borrow
- Fix: Bind the intermediate value first: `let n = vec.len(); vec.push(n);`

### Reborrowing vs Moving
**Severity**: medium

- Gotcha: `&mut T` is automatically reborrowed when passed to functions expecting `&mut T`, but `Box<T>` and other smart pointers are moved -- different behavior despite similar usage
- Fix: Be explicit about ownership; use `&mut *boxed_val` for reborrow through Box

---

## Lifetime Gotchas

### Lifetime Elision Surprises
**Severity**: high

- Gotcha: `fn foo(&self, s: &str) -> &str` elides to `fn foo<'a>(&'a self, s: &str) -> &'a str` -- the return borrows from `self`, NOT from `s`, even if the implementation only uses `s`
- Fix: Add explicit lifetimes when the elided default is wrong: `fn foo<'a>(&self, s: &'a str) -> &'a str`

### Struct Lifetime Bounds
**Severity**: medium

- Gotcha: Storing a reference in a struct (`struct Foo<'a> { bar: &'a str }`) ties the struct's usability to the borrow's lifetime -- the borrowed data cannot be modified or dropped while the struct exists
- Fix: Consider owned types (`String`) or `Cow<'a, str>` unless the zero-copy performance gain is proven necessary

### 'static Misconception
**Severity**: medium

- Gotcha: `'static` does NOT mean "lives forever" -- it means "CAN live forever." Owned types like `String` satisfy `'static` because they have no borrows.
- Rule: `T: 'static` in bounds means "T contains no non-static references" -- it does NOT mean T is a static variable

---

## Type System Traps

### Integer Overflow
**Severity**: high

- Gotcha: In debug mode, integer overflow panics. In release mode, it wraps silently (two's complement). This means tests pass but production silently corrupts data.
- Fix: Use `.checked_add()`, `.saturating_add()`, or `.wrapping_add()` to be explicit about overflow behavior

### Implicit Deref Coercion
**Severity**: medium

- Gotcha: Rust auto-derefs through multiple layers (`&Box<String>` -> `&String` -> `&str`) which can make it unclear what method is being called when types have same-named methods
- Fix: Be explicit when ambiguous; use fully-qualified syntax `<Type as Trait>::method()`

### Turbofish Ambiguity
**Severity**: low

- Gotcha: `let x = "42".parse::<i32>()` needs turbofish because `parse()` is generic. Forgetting it gives a confusing "type annotations needed" error.
- Fix: Either use turbofish or annotate the binding: `let x: i32 = "42".parse()?`

### Trait Object Sizing
**Severity**: medium

- Gotcha: `dyn Trait` is unsized -- cannot use as a bare type. Must be behind a pointer: `Box<dyn Trait>`, `&dyn Trait`, `Arc<dyn Trait>`
- Gotcha: `dyn Trait` only works for object-safe traits -- traits with `Self: Sized` bounds or generic methods are NOT object-safe

---

## Async Gotchas

### Non-Send Types in Async
**Severity**: high

- Gotcha: Holding an `Rc`, `RefCell` guard, or `MutexGuard` (from `std::sync`) across an `.await` point makes the future `!Send` -- it cannot be spawned on a multi-threaded runtime
- Fix: Drop non-Send values before `.await` or use async-compatible alternatives (`tokio::sync::Mutex`)

### Cancellation Safety
**Severity**: high

- Gotcha: When `tokio::select!` completes one branch, other futures are DROPPED mid-execution. If a dropped future was mid-write, data is lost.
- Fix: Use cancellation-safe operations in select branches, or use `tokio::pin!` + loop patterns for safety

### Blocking the Runtime
**Severity**: high

- Gotcha: `std::fs::read()`, `std::thread::sleep()`, or any CPU-heavy computation in an async context blocks the entire tokio worker thread
- Fix: Use `tokio::task::spawn_blocking()` or async equivalents (`tokio::fs`, `tokio::time::sleep`)
- See also: anti-patterns.md "Blocking in Async Context" and "Holding Lock Across Await"

---

## String Gotchas

### String Indexing
**Severity**: medium

- Gotcha: `s[0]` does NOT compile -- Rust strings are UTF-8, and byte index may split a multi-byte character. `s.chars().nth(0)` works but is O(n).
- Fix: Use `.chars()` iterator, `.as_bytes()` for ASCII-only data, or byte ranges `&s[0..4]` only when you're certain of character boundaries

### to_string() vs to_owned() vs into()
**Severity**: low

- Gotcha: `"hello".to_string()`, `"hello".to_owned()`, and `String::from("hello")` all do the same thing -- but `to_string()` goes through `Display` trait (slightly slower in theory, optimized in practice)
- Rule: Use `.to_owned()` or `String::from()` for clarity; `.to_string()` when you want the `Display` representation

---

## Collection Gotchas

### HashMap Iteration Order
**Severity**: medium

- Gotcha: `HashMap` iteration order is NOT deterministic and changes between runs -- do not rely on insertion order
- Fix: Use `BTreeMap` for deterministic order, or `IndexMap` for insertion-order preservation

### Vec::drain vs Vec::clear
**Severity**: low

- Gotcha: `.drain(..)` removes AND returns elements (useful for processing); `.clear()` drops all elements but keeps capacity (useful for reuse)
- Rule: Use `.clear()` for buffer reuse, `.drain(..)` when you need the removed elements

---

## Module System

### use super::* in Tests
**Severity**: low

- Gotcha: `use super::*` in `#[cfg(test)] mod tests` imports everything from the parent module, including private items -- this is intentional and correct for unit tests
- Rule: This is the standard pattern for testing private functions in Rust

### Visibility vs Accessibility
**Severity**: medium

- Gotcha: `pub(crate)` makes an item visible within the crate but NOT to dependents. `pub` in a private module is still inaccessible externally unless the module is also public.
- Rule: Both the item AND its containing module chain must be `pub` for external access
