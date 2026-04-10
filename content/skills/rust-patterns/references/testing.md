# Rust Testing Patterns

## Test Structure

### Unit Tests
- Rule: Place unit tests in `#[cfg(test)] mod tests` at the bottom of each source file; use `use super::*` to access private items
- Rule: Name tests descriptively: `test_<function>_<scenario>_<expected>`

### Integration Tests
- Rule: Place integration tests in `tests/` directory -- each file is a separate crate, can only test public API
- Rule: Share test utilities via `tests/common/mod.rs`

### Test Organization
```
src/
  lib.rs              # #[cfg(test)] mod tests inside
  user.rs
tests/
  common/mod.rs       # Shared utilities
  integration_test.rs # Integration tests
benches/
  benchmark.rs        # Criterion benchmarks
```

---

## Assertions

- Rule: Use `assert_eq!(actual, expected)` / `assert_ne!()` for value comparison with diff output on failure
- Rule: Use `assert!(matches!(value, Pattern { .. }))` for enum variant matching
- Rule: For floating point, use `assert!((actual - expected).abs() < epsilon)`
- Rule: Add custom messages as last arg: `assert_eq!(a, b, "failed for input {}", x)`

### Testing Errors
- Rule: Use `assert!(result.is_err())` for basic error checking
- Rule: Use `matches!(result, Err(MyError::Variant { .. }))` for specific variant matching
- Rule: Return `Result<(), E>` from test functions to use `?` operator in test body

### Testing Panics
- Rule: Use `#[should_panic]` or `#[should_panic(expected = "message")]` for expected panics
- Rule: Use `std::panic::catch_unwind()` when you need to assert on the panic value

---

## Test Attributes

| Attribute | Purpose |
|-----------|---------|
| `#[test]` | Mark as test |
| `#[ignore]` / `#[ignore = "reason"]` | Skip unless `--ignored` flag |
| `#[should_panic]` | Expect panic |
| `#[cfg(feature = "X")]` | Feature-gated test |
| `#[tokio::test]` | Async test with tokio |

---

## Fixtures and Setup

- Rule: Use plain functions returning a test context struct for setup -- Rust has no built-in fixtures
- Rule: Use builder pattern for test data when tests need varied configurations of the same type

---

## Mocking with mockall

Annotate traits with `#[automock]` to generate mock types:

```rust
#[automock]
trait UserRepository {
    fn find(&self, id: &str) -> Option<User>;
    fn save(&self, user: &User) -> Result<(), Error>;
}

#[test]
fn test_service() {
    let mut mock = MockUserRepository::new();
    mock.expect_find()
        .with(eq("123"))
        .times(1)
        .returning(|_| Some(User::new("123", "Test")));

    let svc = UserService::new(mock);
    assert!(svc.get_user("123").is_some());
}
```

- Rule: Use `.with(eq(...))` for exact match, `.withf(|x| ...)` for predicate match
- Rule: Always set `.times(N)` to catch unexpected calls

---

## Async Testing

- Rule: Use `#[tokio::test]` for async tests -- automatically creates a runtime
- Rule: Wrap slow operations in `tokio::time::timeout()` to prevent hanging tests

---

## Property-Based Testing with proptest

Use `proptest!` macro to generate random inputs and verify invariants:

```rust
use proptest::prelude::*;

proptest! {
    #[test]
    fn roundtrip(s in "[0-9]+") {
        let n: u64 = s.parse().unwrap();
        prop_assert_eq!(s, n.to_string());
    }

    #[test]
    fn double_reverse(vec in prop::collection::vec(any::<i32>(), 0..100)) {
        let mut rev = vec.clone();
        rev.reverse();
        rev.reverse();
        prop_assert_eq!(vec, rev);
    }
}
```

---

## HTTP Testing with wiremock

- Rule: Use `MockServer::start().await` to create a local HTTP mock server
- Rule: Mount expectations with `Mock::given(method("GET")).and(path("/...")).respond_with(...)`
- Rule: Pass `mock_server.uri()` to your client under test

---

## Database Testing

- Rule: Use `#[sqlx::test]` for automatic pool creation and cleanup per test
- Rule: Each test runs in a transaction that is rolled back -- tests are isolated

---

## Coverage

- Rule: Use `cargo tarpaulin` for coverage; `--fail-under 70` to enforce minimums in CI
- Rule: Use `--ignore-tests` to exclude test code from coverage metrics
