<!-- modeled after: nammayatri/nammayatri .clinerules -->
## 1. Coding Style

Favor plain functions over classes when behavior and data can live
apart. Keep cyclomatic complexity under ten per function and prefer
early returns over deeply nested conditionals.

Names are spelled out. Avoid three-letter abbreviations unless they are
well-known domain acronyms used throughout the codebase already.

## 2. Error Handling

Propagate errors up to a boundary that has the context to decide what to
do. Do not swallow errors with empty catch or except blocks.

Wrap errors with context at the boundary so the stack trace points at
the originating call, not the last hop.

## 3. Testing Conventions

Table-driven tests are the default. Each row covers one behavior. Names
read as predicates: it_returns_zero_for_empty, not test_empty_case.

Keep setup in the test body; shared fixtures live in helpers that are
explicit about what they install.

## 4. Documentation

Every exported function has a one-line summary plus a list of arguments
and return values. If the behavior is non-obvious, add an example in a
code fence.

Docs are for humans; do not repeat type signatures in prose when the
type system already states them.

## 5. Logging

Structured logs only. Every log line has a level, a message, and a
key-value map of context. Messages are stable; context fields are where
per-invocation detail lives.

Do not log secrets or personally identifiable information. When in
doubt, hash the value or drop the field entirely.
