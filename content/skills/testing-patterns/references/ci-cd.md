# CI/CD Testing Patterns

Universal patterns for reliable testing in CI/CD pipelines. Language-specific CI configuration (test commands, coverage tools, dependency caching) belongs in each language skill.

## Matrix Testing

- Rule: Use `strategy.matrix` to test across OS and language versions. Set `fail-fast: false` so one failure does not cancel other shards.
- Gotcha: Upload coverage from only one matrix combination (e.g., `ubuntu-latest` + latest language version) to avoid duplicate reports.

## Separate Unit and Integration Jobs

- Rule: Run unit tests first as a fast gate. Integration tests depend on (`needs:`) the unit job. This gives fast feedback on simple failures.
- Pattern: Use GitHub Actions `services:` block for databases/caches needed by integration tests. Include health checks with `--health-cmd` to wait for readiness.

## Dependency Caching

- Rule: Most `actions/setup-*` actions have built-in `cache: true` (Go, Python, Node). For languages without built-in support (Rust), use `actions/cache@v4` with a key based on lockfile hash.
- Gotcha: Cache keys must include `runner.os` to avoid cross-platform cache corruption.

## Coverage

### Thresholds
- Rule: Enforce minimum coverage in CI to prevent regression. Common threshold: 70-80%.
- Each language has its own mechanism: `--cov-fail-under` (pytest), `--coverageThreshold` (Jest), custom script parsing (Go). See language skill for specifics.

### Reporting
- Rule: Upload to Codecov or Coveralls using their GitHub Actions. Set `fail_ci_if_error: true` to catch upload failures.
- PR comments: Use `5monkeys/cobertura-action` or similar to post coverage diffs on PRs.

## Test Parallelization

- Rule: For large suites, shard tests across matrix jobs. Use `matrix.shard` with language-specific splitters:
  - Go: `gotestsum` with partitioning, or `-parallel N`
  - Python: `pytest-split --splits N --group $shard`
  - Node: `jest --shard=$shard/N`
  - Rust: `cargo-nextest` with partitioning

## Flaky Test Handling

### Quarantine Pattern
- Rule: Skip known flaky tests in the main CI run. Run them separately with `continue-on-error: true`. Track each quarantined test in an issue.
- Fix: Investigate root cause (timing, shared state, network). Remove from quarantine once fixed.

### Retry (Use Sparingly)
- Rule: `nick-fields/retry@v3` with `max_attempts: 3` is a last resort. Retries mask real problems. Prefer fixing the flakiness.

### Detection
- Rule: Run test suite multiple times in a dedicated job to surface non-determinism. A test that fails on any of 3 runs is flaky.

## Test Environment Management

- Rule: Set `CI: true` as a workflow-level env var. Use `env:` blocks at job or step level for test-specific variables (database URLs, API keys from secrets).
- Docker Compose: For complex multi-service setups, use `docker compose -f docker-compose.test.yml up -d` with health-check waits. Always add `if: always()` to the teardown step.

## Test Artifacts

- Rule: Always upload test results (JUnit XML) and coverage reports with `actions/upload-artifact@v4` using `if: always()` so results are available even on failure.
- Use `mikepenz/action-junit-report@v4` or similar to publish results as PR annotations.

## Performance Testing

- Rule: Run benchmarks in CI and compare against a baseline using `benchmark-action/github-action-benchmark@v1`. Set `alert-threshold` (e.g., 150%) to fail on regressions.
- Gotcha: Benchmark results vary by runner type. Pin to a specific runner or accept some noise.

## Security Testing in CI

- Rule: SAST and dependency scanning belong in CI. Upload results in SARIF format with `github/codeql-action/upload-sarif@v3`.
- For language-specific security tools (gosec, bandit, npm audit, cargo audit), see the language skill or `skills/security-audit/references/testing.md`.

## Timeouts

- Rule: Set `timeout-minutes` at both job level (30 min) and step level (15 min). Prevents hung tests from consuming CI minutes indefinitely.

## Best Practices Summary

| Practice | Why |
|----------|-----|
| `fail-fast: false` in matrix | Don't stop other shards on failure |
| Separate unit/integration jobs | Fast feedback from unit tests |
| Cache dependencies | Faster builds, less network |
| Upload artifacts with `if: always()` | Debug failures after job completes |
| Coverage thresholds | Prevent coverage regression |
| Retry sparingly | Fix flaky tests instead |
| Timeouts on jobs and steps | Prevent hung tests from blocking |
| Parallel sharding | Speed up large test suites |
