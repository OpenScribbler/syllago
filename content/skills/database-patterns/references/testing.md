# Database Testing

## Test Isolation Strategies

### Transaction Rollback (Fastest)

- Wrap each test in a transaction, rollback at teardown. Zero cleanup cost.
- Limitation: breaks if the code under test issues its own COMMIT or manages transactions explicitly.
- Best for: repository/DAO unit tests where the test controls the connection.

### Separate Database per Test

- Clone a template database per test. Each test gets a pristine copy.
- Tools: pgtestdb (Go), pytest-postgresql (Python), testcontainers (any language).
- Trade-off: slower setup than rollback, but fully isolated and supports code that manages its own transactions.

### TRUNCATE for Cleanup

- `TRUNCATE table1, table2, ... CASCADE` between tests.
- Faster than DROP+CREATE but slower than rollback.
- Use when rollback is not viable and per-test databases are too expensive.
- Gotcha: TRUNCATE acquires ACCESS EXCLUSIVE lock -- cannot run concurrently on the same table.

## Testcontainers

- Spin up a real database in Docker for integration tests. Tests run against the actual engine, not mocks.
- Rule: always test against the same database engine used in production. SQLite-in-tests-for-PostgreSQL-in-prod hides real bugs.
- Available for: Go (testcontainers-go), Python (testcontainers-python), Java (testcontainers-java), Node (testcontainers-node), Rust (testcontainers-rs).
- CI setup: Docker-in-Docker or service containers (GitHub Actions `services:`).

## Test Data: Factories over Fixtures

- **Fixtures** (static JSON/SQL files): brittle, hard to maintain as schema evolves, create invisible dependencies between tests.
- **Factories** (programmatic builders): flexible, composable, self-documenting.
  - Python: Factory Boy, Faker
  - Go: custom factory functions, gofakeit
  - Node: Fishery, Faker
  - Rust: fake crate, custom builders
- Rule: factories should produce valid domain objects by default. Override only the fields relevant to each test.
- Rule: use random but deterministic data (seeded generators) for reproducibility.

## Migration Testing

- Run forward and rollback migrations against a real database in CI.
- Verify data integrity after migration: row counts, constraint checks, spot-check transformed data.
- Do NOT only unit-test migration files -- real databases expose locking, type conversion, and constraint issues that unit tests miss.
- Include a "smoke test" that runs the full migration chain from empty database to current version.

## Query Testing

- Test queries against real databases, not mocks. Mocked query results cannot catch:
  - SQL syntax errors specific to the engine
  - Index usage and query plan changes
  - Type coercion differences between engines
  - Transaction isolation behavior
- Assert both results AND query count (to catch N+1 regressions).

## Testing Checklist

- [ ] Test isolation strategy chosen (rollback, separate DB, or truncate)
- [ ] Tests run against the same DB engine as production
- [ ] Factories used instead of static fixtures
- [ ] Migration forward + rollback tested in CI
- [ ] N+1 detection in tests (query count assertions)
- [ ] CI uses testcontainers or equivalent for real DB
