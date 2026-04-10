# Test Design Principles

Designing effective tests that catch bugs and survive refactoring.

## The Four Properties of Good Tests

### 1. Protection Against Regression
- Rule: Test observable behavior -- outputs, side effects, state changes. If tests pass but behavior is broken, the tests failed their purpose.
- Practice: Assert on what the system does, not how. Include edge cases that historically caused bugs.

### 2. Resistance to Refactoring
- Rule: Tests should not break when code is restructured without changing behavior.
- Red flags: Tests break when private methods are renamed, implementation strategy changes, or internal components are mocked.
- Practice: Test through public interfaces only. Mock at architectural boundaries (network, database, external APIs), never internal dependencies.

### 3. Fast Feedback
- Rule: Unit tests run in milliseconds. If they hit network, spin up containers, or write to filesystem, they are integration tests.
- Practice: In-memory implementations for storage. Inject dependencies. Separate unit from integration tests.

### 4. Maintainability
- Rule: One behavior per test. Descriptive names. Table-driven/parameterized tests for similar scenarios.
- Red flags: 200+ line test methods, complex setup/teardown, shared mutable state.

---

## Anti-Patterns

### Testing Implementation Details
**Severity**: high

- Rule: Never assert on internal state (cache entries, private fields). Instead, observe the behavior indirectly -- e.g., verify a second call does not hit the DB (proves caching without inspecting cache internals).

### Non-Deterministic Tests
**Severity**: high

- Rule: Never use `time.sleep()` or real clocks for time-dependent assertions. Inject a controllable clock/fake timer. Seed random generators.

### Shared Mutable State
**Severity**: high

- Rule: Each test creates its own state via factory functions or builders. Never share mutable objects across tests via `beforeAll`/module-level variables.

### No Assertions
**Severity**: high

- Rule: Every test must assert on specific outcomes. A test that only checks "no error" without verifying the result is incomplete.

### Overmocking
**Severity**: medium

- Rule: Mock only at external boundaries (HTTP APIs, third-party services). If a test mocks 3+ internal collaborators, it tests the mocking framework, not the code. Use fakes (in-memory implementations) instead.

### Testing Private Methods
**Severity**: medium

- Rule: Test through the public interface. If a private method needs direct testing, it should be extracted into its own unit.

---

## Test Naming Conventions

| Style | Example |
|-------|---------|
| Given/When/Then | `test_given_empty_cart_when_checkout_then_fails` |
| Should | `test_checkout_should_fail_with_empty_cart` |
| Method_Scenario_Expected | `test_checkout_emptyCart_throwsException` |
| Plain descriptive | `test_empty_cart_cannot_checkout` |

Rules: Be consistent within a codebase. Include the scenario/condition and expected outcome. Avoid `test1`, `testX`.

---

## Edge Cases Checklist

### Input Boundaries
- [ ] Empty input (null, empty string, empty array)
- [ ] Single element
- [ ] Minimum and maximum valid values
- [ ] Just below minimum / just above maximum

### Data Types
- [ ] Zero, negative numbers, very large numbers
- [ ] Unicode and special characters
- [ ] Whitespace-only strings

### State
- [ ] Uninitialized / partially initialized
- [ ] Already completed/closed
- [ ] Concurrent modifications

### Error Conditions
- [ ] Network failures and timeouts
- [ ] Authentication and permission failures
- [ ] Resource not found / already exists
- [ ] Validation failures

---

## Assertion Best Practices

- **Be specific**: Assert on concrete values (`result.status == "completed"`, `len(items) == 3`), not just truthiness (`result is not None`).
- **Include context**: Error messages should show input, actual, and expected -- e.g., `ParseDate(%q) = %v, want %v`.
- **One logical assert per test**: Multiple assertions verifying the same outcome are fine. Testing multiple independent behaviors in one test is not.

---

## When to Write Tests

| Approach | Best For |
|----------|----------|
| **Test-first (TDD)** | Well-understood requirements, pure functions, bug fixes, clear contracts |
| **Test-after** | Exploratory code, UI experiments, prototypes (but write tests before shipping) |
| **Always test** | Security-critical code, business logic with edge cases, regression tests, public APIs |

---

## Test Organization

- Co-locate unit tests with source or use a parallel `test/` directory
- Separate integration tests (build tags, markers, or directory)
- Use `fixtures/` or `testdata/` for test data files
- Use shared helper modules for builders, assertions, factory functions
- Builder pattern for complex test objects: provide sensible defaults, override per-test via fluent methods

For language-specific test organization patterns, see the language skill's testing reference.
