---
name: testing-patterns
description: Universal testing patterns and practices across languages. Use when reviewing tests, designing test strategies, or improving test quality. For language-specific patterns, load the respective language skill's testing reference.
---

# Testing Patterns

Universal testing principles, quality checklists, and anti-patterns that apply across all languages.

## Test Quality Checklist

Run through this for every test suite review:

| Dimension | Check | Red Flag |
|-----------|-------|----------|
| **Behavior** | Tests what, not how? | Mocks internal implementation |
| **Clarity** | Intent clear in 10 seconds? | Cryptic names like `test1`, `testX` |
| **Determinism** | Same result every run? | Uses `time.Now()`, random without seed |
| **Independence** | Tests run in any order? | Shared mutable state, setup dependencies |
| **Assertions** | Specific assertions on behavior? | Only checks "no error", no value verification |
| **Edge Cases** | Boundaries and errors covered? | Only happy path tested |
| **Speed** | Unit tests complete in seconds? | Network calls, file I/O in unit tests |

## Quick Anti-Patterns

| Anti-Pattern | Why It's Bad | Fix |
|--------------|--------------|-----|
| Testing implementation | Breaks on refactor | Test public behavior only |
| Flaky time dependencies | Non-deterministic | Inject clock, use fixed times |
| Shared test state | Order-dependent failures | Fresh state per test |
| Too many mocks | Tests mocks, not code | Integration test or real deps |
| No assertions | Test always passes | Assert specific outcomes |
| Giant test methods | Hard to debug failures | One behavior per test |
| Copy-paste tests | Maintenance nightmare | Table-driven or parameterized |

## Test Levels

| Level | Purpose | Speed | Dependencies |
|-------|---------|-------|--------------|
| **Unit** | Single function/method logic | ms | None (mocked) |
| **Integration** | Component interactions | seconds | Real deps (DB, APIs) |
| **E2E** | Full user workflows | minutes | Full stack |
| **Contract** | API compatibility | seconds | Schema validation |

**Rule of thumb**: Unit > Integration > E2E (pyramid shape)

## Coverage Prioritization

| Code Type | Priority | Why |
|-----------|----------|-----|
| Business logic | HIGH | Core value, complex decisions |
| Error handling | HIGH | Silent failures are worst failures |
| Security paths | HIGH | Auth, authz, input validation |
| Data transformations | MEDIUM | Often has edge cases |
| CRUD operations | MEDIUM | Usually straightforward |
| Configuration loading | LOW | Tested at integration level |
| Generated code | LOW | Generator should be tested |

## References

Load on-demand based on task:

| When you need | Reference |
|---------------|-----------|
| Test design principles, anti-patterns, edge case checklists, naming conventions | [test-design.md](references/test-design.md) |
| CI/CD pipeline patterns: matrix, caching, parallelization, flaky test handling | [ci-cd.md](references/ci-cd.md) |
| Integration testing strategy: containers, DB isolation, API contracts, K8s | [integration-testing.md](references/integration-testing.md) |
| Presenting test analysis findings (copy-paste template) | [report-template.md](references/report-template.md) |

## Language-Specific References

For language-specific patterns and wrapper commands, load from the language skill:

| Language | Testing Patterns | Wrapper Reference |
|----------|-----------------|-------------------|
| Go | `skills/go-patterns/references/testing.md` | `skills/go-patterns/references/go-dev-wrapper.md` |
| Python | `skills/python-patterns/references/testing.md` | `skills/python-patterns/references/py-dev-wrapper.md` |
| JavaScript/TypeScript | `skills/javascript-patterns/references/testing.md` | -- |
| Rust | `skills/rust-patterns/references/testing.md` | -- |
| Terraform | -- | `skills/terraform-patterns/references/tf-dev-wrapper.md` |

## Related Skills

- **Security testing**: `skills/security-audit/references/testing.md` for auth, injection, crypto test patterns
- **Code review**: `skills/code-review-standards/SKILL.md` when reviewing test code quality
