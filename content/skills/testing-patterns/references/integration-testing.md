# Integration Testing Patterns

Universal strategies for testing component interactions. Language-specific test code (Go testcontainers, Python fixtures, Rust sqlx::test) belongs in each language skill's testing reference.

## Test Containers

- Rule: Use test containers (testcontainers-go, testcontainers-python, testcontainers-rust) for real database/service dependencies. Containers are ephemeral, reproducible, and avoid shared test infrastructure.
- Pattern: Start container, extract host/port, construct connection string, run tests, terminate container in defer/finally.
- Gotcha: Use `WaitingFor` / health checks to ensure the service is ready before running tests. Without this, tests will fail intermittently on slow container starts.

## Database Test Isolation

### Transaction Rollback Pattern
- Rule: Wrap each test in a transaction, then roll back. This isolates test data without truncating tables between tests.
- Gotcha: Some ORMs auto-commit. Verify your test helper actually rolls back.

### Fresh Database Per Test
- Rule: For tests that need DDL changes or cannot use transactions (e.g., testing migration scripts), create a fresh database per test or test suite. Slower but fully isolated.

### Seeding
- Rule: Use test fixtures for reference data. Load once per suite (session scope) for read-only data, per-test for mutable data.

## API Contract Testing

### Schema Validation
- Rule: Validate requests and responses against the OpenAPI spec in tests. This catches drift between spec and implementation.
- Tools: `kin-openapi` (Go), `openapi-core` (Python), `swagger-jsdoc` + validation middleware (JS).

### Consumer-Driven Contracts (Pact)
- Rule: Consumer defines expected interactions. Provider verifies against them. Useful for microservice boundaries where teams own different sides.
- Pattern: Consumer writes a Pact file describing expected request/response pairs. Provider runs a verification test with state handlers to set up preconditions.
- Gotcha: Pact tests need state handlers to seed data. Without them, verification fails on missing data rather than contract mismatches.

## HTTP Integration Testing

### Test Server Pattern
- Rule: Start a local test server (httptest in Go, TestClient in Python/FastAPI, supertest in Node) that runs the real handler stack. Test against it with a real HTTP client.
- Advantage over mocking: Tests the full middleware chain (auth, validation, serialization).

### Mock Server Pattern
- Rule: For testing clients that call external APIs, use a mock HTTP server (gock/Go, responses/Python, MSW/JS, wiremock/Rust). Define expected requests and responses. Verify all expectations were met.
- Gotcha: Always verify all expected requests were made (e.g., `gock.IsDone()`, `server.resetHandlers()`). Unmatched expectations indicate missing test coverage.

## Kubernetes Integration Testing

### envtest (controller-runtime)
- Rule: Use `envtest.Environment` for testing K8s controllers without a full cluster. It runs a real API server and etcd but no scheduler/controller-manager.
- Test: Create resources, trigger reconciliation, verify status updates and owned resources.
- Gotcha: `envtest` does not run controllers other than yours. If your controller depends on other controllers creating resources, you must simulate that in test setup.

### Kind (Kubernetes in Docker)
- Rule: For full-cluster testing (webhooks, RBAC, multi-controller interactions), use Kind in CI with `helm/kind-action@v1`.
- Pattern: Create cluster, install CRDs, deploy controller, run integration tests with build tags, tear down cluster.

## Message Queue Testing

- Rule: Use test containers for Kafka, RabbitMQ, NATS, etc. Test the full produce-consume cycle: send a message, consume it, assert on content.
- Gotcha: Message ordering and delivery guarantees vary by queue. Test your consumer's idempotency and error handling, not just the happy path.

## Integration Test Organization

- Separate integration tests from unit tests using build tags (`//go:build integration`), markers (`@pytest.mark.integration`), or directory structure (`tests/integration/`).
- Integration tests run after unit tests pass (`needs: unit-tests` in CI).
- Set timeouts on integration tests to prevent hung connections from blocking CI.

## Best Practices

| Practice | Why |
|----------|-----|
| Test containers over shared DBs | Reproducible, isolated, no cleanup drift |
| Transaction rollback per test | Fast isolation without truncation |
| Contract testing at boundaries | Catch API drift before deployment |
| Build tags / markers | Control which tests run in which context |
| Timeouts on all integration tests | Prevent hung connections from blocking CI |
| Mock external, use real internal | Test actual integration with your dependencies |
| Health checks on containers | Prevent flaky failures from slow starts |
