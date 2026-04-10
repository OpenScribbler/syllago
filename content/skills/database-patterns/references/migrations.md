# Migrations

## Core Rules

- Never couple schema deployment with application deployment. The "atomic deploy" is a myth at the database layer -- schemas must be backward-compatible during rollouts.
- Every migration must be idempotent. Use `IF NOT EXISTS`, `IF EXISTS` guards.
- Every migration must have a rollback script tested against real data.

## Expand-Migrate-Contract Pattern

The only safe pattern for breaking schema changes in production:

| Phase | Action | Duration |
|-------|--------|----------|
| 1. Expand | Add new column/table (nullable) | Deploy immediately |
| 2. Double-write | Application writes to both old and new | Until backfill completes |
| 3. Backfill | Migrate existing data to new structure | Batched, may take hours/days |
| 4. Switch reads | Application reads from new structure | Verify correctness |
| 5. Remove old writes | Application stops writing old structure | After read verification |
| 6. Contract | Drop old column/table | After full validation |

- Rule: each phase is a separate deployment. Never combine phases.
- Rule: the Contract phase should wait at least one release cycle after removing old writes.

## PostgreSQL Locking Gotchas

**Severity**: high

- `ALTER TABLE ADD COLUMN` with `NOT NULL DEFAULT value` acquires ACCESS EXCLUSIVE lock on PG < 11. On PG 11+, this is near-instant.
- `ALTER TABLE ADD COLUMN` nullable with no default is always near-instant.
- `CREATE INDEX` acquires a SHARE lock (blocks writes). Always use `CREATE INDEX CONCURRENTLY` on live tables.
- `ALTER TABLE SET NOT NULL` acquires ACCESS EXCLUSIVE lock. Prefer adding a CHECK constraint with `NOT VALID`, then `VALIDATE CONSTRAINT` separately.
- `DROP COLUMN` only marks the column as dropped (fast), but `VACUUM` must reclaim space.

## Backfill Safety

- Never backfill millions of rows in a single transaction -- this holds locks and bloats WAL.
- Batch in chunks of 1,000-10,000 rows with brief sleeps between batches.
- Use a progress marker (e.g., `WHERE id > last_processed_id ORDER BY id LIMIT batch_size`).
- Monitor replication lag during backfills -- pause if lag exceeds threshold.

## Tooling Selection

| Tool | Style | Best For |
|------|-------|----------|
| Flyway | SQL-first, versioned | Teams preferring raw SQL migrations |
| Liquibase | XML/YAML/JSON + SQL | Enterprise, multi-DB support |
| Atlas | Declarative (desired state) | Modern, HCL-based, auto-diff |
| pgroll | PostgreSQL zero-downtime | Automated expand-contract for PG |
| golang-migrate | SQL files, Go CLI/library | Go projects |
| Alembic | Python, auto-detect changes | Python/SQLAlchemy projects |
| Prisma Migrate | TypeScript, schema-first | Node.js/TypeScript projects |

- Rule: choose based on team language ecosystem and SQL-first vs declarative preference.
- Rule: all tools should run in CI against a real database, not just unit-test the migration files.

## Migration CI Checklist

- [ ] Forward migration runs against real DB (testcontainers or CI database)
- [ ] Rollback migration runs successfully after forward
- [ ] Migration is idempotent (running twice produces same result)
- [ ] No ACCESS EXCLUSIVE locks on large tables without documented plan
- [ ] Backfill scripts are batched with progress tracking
- [ ] Data integrity verified after migration (row counts, constraint checks)
