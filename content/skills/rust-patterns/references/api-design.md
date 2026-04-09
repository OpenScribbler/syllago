# Rust API Design

## Function Arguments

### Borrowed Types
- Rule: Accept `&str` not `&String`, `&[T]` not `&Vec<T>`, `&T` not `&Box<T>` -- broadest compatibility
- Rule: Accept `impl AsRef<Path>` for path parameters -- works with `String`, `&str`, `PathBuf`, `&Path`

### Owned Type Parameters
- Rule: Accept `impl Into<String>` when the function stores the value -- avoids forcing callers to `.to_string()`
- Rule: Only take ownership when the function needs to store or move the value

### Generics vs Trait Objects

| Use | When |
|-----|------|
| Generics (`impl Trait`, `<T>`) | Performance critical, single type per call site |
| Trait objects (`&dyn Trait`, `Box<dyn Trait>`) | Heterogeneous collections, reduce binary size |

- Rule: Use `impl Trait` in return position to hide concrete type while keeping static dispatch

---

## Getters and Setters

- Rule: Getters have NO `get_` prefix: `fn port(&self) -> u16`, `fn host(&self) -> &str`
- Rule: Setters use `set_` prefix: `fn set_port(&mut self, port: u16)`
- Rule: Builder-style setters take `mut self` and return `Self`: `fn with_port(mut self, port: u16) -> Self`
- Rule: Mutable access variants use `_mut` suffix: `fn get_mut(&mut self) -> &mut T`

---

## Iterator Conventions

| Method | Returns | Consumes Self |
|--------|---------|---------------|
| `iter()` | `Iterator<Item = &T>` | No |
| `iter_mut()` | `Iterator<Item = &mut T>` | No |
| `into_iter()` | `Iterator<Item = T>` | Yes |

Implement `IntoIterator` for ergonomic `for` loops:

```rust
impl<'a> IntoIterator for &'a Collection {
    type Item = &'a T;
    type IntoIter = std::slice::Iter<'a, T>;
    fn into_iter(self) -> Self::IntoIter { self.items.iter() }
}
```

---

## Conversion Traits

| Trait | Use | Notes |
|-------|-----|-------|
| `From<T>` | Infallible conversion | Implementing `From` auto-provides `Into` |
| `TryFrom<T>` | Fallible conversion | Returns `Result<Self, Self::Error>` |
| `AsRef<T>` | Cheap reference conversion | For generic function params |
| `FromStr` | Parse from string | Enables `.parse::<MyType>()` |

- Rule: Implement `From`, not `Into` -- `Into` is auto-derived from `From`
- Rule: Use `TryFrom` when conversion can fail (e.g., negative i64 to UserId)
- Rule: Implement `FromStr` to enable `.parse()` on string types

---

## Trait Implementation Checklist

See SKILL.md "Common Traits Checklist" for the full table. Additionally: derive `Copy` for small, trivially copyable types only.

---

## Error Handling in APIs

- Rule: Libraries use `thiserror` with specific error enums; see error-handling.md for canonical pattern
- Rule: Use `#[error(transparent)]` for wrapped errors that should display the inner error
- Rule: Always implement `std::error::Error` (thiserror does this automatically)

---

## Documentation

- Rule: Every public item needs a doc comment (`///`) with a one-line summary
- Rule: Include `# Examples` with runnable code for all public functions
- Rule: Document panics (`# Panics`), errors (`# Errors`), and safety (`# Safety` for unsafe)
- Rule: Section order: summary, description, Arguments, Returns, Examples, Panics, Errors, Safety

---

## Sealed Traits

Prevent external implementations of public traits:

```rust
mod private {
    pub trait Sealed {}
}

pub trait MyTrait: private::Sealed {
    fn method(&self);
}

// Only types implementing Sealed can implement MyTrait
impl private::Sealed for MyType {}
impl MyTrait for MyType {
    fn method(&self) { /* ... */ }
}
```
