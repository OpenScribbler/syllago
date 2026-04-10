---
name: rust-patterns
description: Rust development patterns and best practices. Use when building or fixing Rust services, libraries, or CLI tools.
---

# Rust Development Patterns

## Quick Reference

| Category | Best Practice |
|----------|---------------|
| Arguments | Accept `&str` over `&String`, `&[T]` over `&Vec<T>` |
| Errors | `thiserror` for libs, `anyhow` for apps |
| Options | Use `Option<T>`, avoid `.unwrap()` in production |
| Ownership | Prefer borrowing over cloning; `Cow` for flexibility |
| Constructors | `fn new() -> Self`; implement `Default` trait |
| Concurrency | `Arc<Mutex<T>>` for shared state; tokio for async |
| Naming | snake_case fns, CamelCase types, SCREAMING_SNAKE consts |
| Testing | `#[cfg(test)]` module; mockall for mocking |

## Naming Conventions

| Prefix | Meaning | Example |
|--------|---------|---------|
| `as_` | Cheap reference conversion | `as_str()`, `as_bytes()` |
| `to_` | Expensive conversion | `to_string()`, `to_vec()` |
| `into_` | Consuming conversion | `into_inner()`, `into_bytes()` |
| `is_`/`has_` | Boolean query | `is_empty()`, `has_key()` |
| `_mut` | Mutable variant | `iter_mut()`, `get_mut()` |
| `try_` | Fallible operation | `try_from()`, `try_into()` |

## Common Traits Checklist

| Trait | When |
|-------|------|
| `Debug` | Always (`#[derive(Debug)]`) |
| `Clone` | When values need copying |
| `Default` | When zero/empty value makes sense |
| `PartialEq`/`Eq` | For equality comparison |
| `Hash` | For use in HashMap/HashSet |
| `Display` | For user-facing output |
| `From`/`Into` | For type conversions |
| `Send`/`Sync` | Auto-derived; manually impl for unsafe types |

## References

Load on-demand based on task:

| When task involves | Reference |
|--------------------|-----------|
| Idioms, builder, newtype, type state, RAII | [patterns.md](references/patterns.md) |
| Result, thiserror, anyhow, error context | [error-handling.md](references/error-handling.md) |
| Tokio, spawn, select!, channels, semaphore, shutdown | [async-patterns.md](references/async-patterns.md) |
| cargo test, mockall, proptest, wiremock, fixtures | [testing.md](references/testing.md) |
| Trait impl, conversions, sealed traits, doc comments | [api-design.md](references/api-design.md) |
| Clap, serde attributes, tracing, config loading | [cli-serde.md](references/cli-serde.md) |
| Allocations, collections, iterators, Cow, benchmarks | [performance.md](references/performance.md) |
| Clone abuse, unwrap, blocking async, lock scope | [anti-patterns.md](references/anti-patterns.md) |
| Borrow checker surprises, lifetime gotchas, footguns | [gotchas.md](references/gotchas.md) |

## Related Skills

| Context | Skill |
|---------|-------|
| Test design and strategy | `skills/testing-patterns/SKILL.md` |
| Code review checklists | `skills/code-review-standards/SKILL.md` |
