# Rust Error Handling

## Core Rules

### Library vs Application Errors
**Severity**: high | **Category**: architecture

- Rule: Libraries use `thiserror` with specific enum variants; applications use `anyhow` for convenience
- Rule: Define a public `type Result<T> = std::result::Result<T, MyError>` alias in library crates
- Rule: Make error types `Send + Sync` for async compatibility

### thiserror Pattern (Libraries)
**Severity**: high | **Category**: error design

Canonical form -- one snippet covers `#[error]`, `#[from]`, `#[source]`, and structured variants:

```rust
use thiserror::Error;

#[derive(Error, Debug)]
pub enum AppError {
    #[error("validation failed: {0}")]
    Validation(String),

    #[error("not found: {resource} with id {id}")]
    NotFound { resource: &'static str, id: String },

    #[error("database error")]
    Database(#[from] sqlx::Error),

    #[error(transparent)]
    Internal(#[source] Box<dyn std::error::Error + Send + Sync>),
}
```

### anyhow Pattern (Applications)
**Severity**: medium | **Category**: error design

- Rule: Use `.context()` / `.with_context(|| format!(...))` to add human-readable context to every `?` propagation
- Rule: Use `bail!("msg")` for early returns; `anyhow!("msg")` for ad-hoc errors
- Gotcha: `context()` allocates -- use `with_context()` with closures for messages that are expensive to format

### Error Propagation
**Severity**: high | **Category**: correctness

- Rule: Use `?` operator for propagation, not `.unwrap()` or `.expect()` in production code
- Rule: Implement `From<SourceError>` for automatic conversion (or use `#[from]` with thiserror)
- Rule: Convert `Option<T>` to `Result` with `.ok_or()` or `.ok_or_else(|| ...)`

### Result Combinators
**Severity**: medium | **Category**: idiom

- Rule: Use `.map()`, `.map_err()`, `.and_then()` for chaining instead of nested match blocks
- Rule: Use `.unwrap_or_default()` or `.unwrap_or_else(|| ...)` for providing fallback values
- Rule: Prefer `?` over combinators when the function already returns Result

### Error Handling in main()
**Severity**: medium | **Category**: structure

- Rule: Return `anyhow::Result<()>` from main for automatic error display
- Rule: For custom exit codes, use `if let Err(e) = run()` pattern with explicit `process::exit()`
- Rule: Print the full cause chain: iterate `e.chain()` or `e.source()` for debugging

### Manual Error Implementation
**Severity**: low | **Category**: advanced

- Rule: Only implement `Error` manually when thiserror is not available or you need custom behavior
- Rule: Implement `Display`, `Debug`, and optionally `source()` returning the wrapped cause

### Testing Errors
**Severity**: medium | **Category**: testing

- Rule: Use `assert!(result.is_err())` for basic error checking
- Rule: Use `assert!(matches!(result, Err(MyError::Variant { .. })))` for specific variant matching
- Rule: Use `.unwrap_err().to_string().contains("expected")` for error message assertions

- Rule: Always add `.context()` on `?` propagation for debuggable error chains

For error anti-patterns (unwrap abuse, stringly-typed errors, swallowing), see anti-patterns.md.
