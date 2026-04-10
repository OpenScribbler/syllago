---
name: database-patterns
description: Cross-language database patterns for schema design, migrations, indexing, connection pooling, query optimization, transactions, testing, and security. Use when designing schemas, reviewing queries, planning migrations, or debugging database performance. Language-specific ORM APIs (GORM, SQLAlchemy, Prisma, sqlx) stay in language skills; this skill covers universal design decisions.
---

# Database Patterns

Universal patterns for designing, operating, and securing relational databases.

## Database Design Checklist

| Category | Requirement |
|----------|-------------|
| Schema normalization | 3NF for OLTP; denormalize only with measured evidence |
| Naming conventions | snake_case, plural tables, `_id` FKs, `_at` timestamps, `is_`/`has_` booleans |
| Data types | TIMESTAMPTZ not TIMESTAMP, NUMERIC not FLOAT for money, TEXT over VARCHAR(n) in PG |
| Primary keys | BIGSERIAL or UUID; never reuse, never expose internals |
| Indexing | Composite: equality first, range last; audit unused indexes regularly |
| Migrations | Expand-Migrate-Contract for breaking changes; never couple schema + app deploys |
| Connection pooling | PgBouncer transaction mode default; pool_size = num_cores as starting point |
| Transactions | Match isolation level to workload; optimistic locking for read-heavy, pessimistic for write-heavy |
| N+1 prevention | Eager/batch loading; detect with query-count middleware |
| Testing isolation | Transaction rollback per test; testcontainers for real DB in CI |
| Parameterized queries | Always; no exceptions; never string-format SQL |
| Credential management | Secrets manager (Vault, AWS SM, K8s ESO); separate DB users per service role |

## Anti-Patterns

- **EAV tables**: Entity-Attribute-Value destroys query performance and type safety. Use JSONB columns or proper normalization.
- **FLOAT for money**: Floating-point arithmetic causes rounding errors. Use NUMERIC/DECIMAL with explicit precision.
- **SELECT ***: Wastes bandwidth, breaks on schema changes, prevents covering indexes. List columns explicitly.
- **N+1 in loops**: Querying per row in application loops. Batch load or use eager loading.
- **OFFSET on large tables**: Performance degrades linearly. Use keyset/cursor pagination.
- **Long-running transactions**: Hold locks, block vacuums, increase replication lag. Keep transactions short and focused.
- **App superuser credentials**: One credential for all access prevents audit and blast radius control. Use least-privilege roles.

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| Schema normalization, naming, data types, DB selection | [schema-design.md](references/schema-design.md) |
| Zero-downtime migrations, expand-contract, tooling | [migrations.md](references/migrations.md) |
| Composite indexes, covering, partial, when NOT to index | [indexing.md](references/indexing.md) |
| PgBouncer, pool sizing, app-level config | [connection-pooling.md](references/connection-pooling.md) |
| N+1, EXPLAIN, batch loading, ORM vs raw SQL, repository pattern | [query-patterns.md](references/query-patterns.md) |
| Isolation levels, optimistic/pessimistic locking, saga | [transactions.md](references/transactions.md) |
| Transaction rollback, testcontainers, factories, migration testing | [testing.md](references/testing.md) |
| Parameterized queries, least privilege, credential management | [security.md](references/security.md) |

## Related Skills

- **Language skills** (Go, Python, JS, Rust): ORM-specific APIs (GORM.Preload, SQLAlchemy.joinedload, Prisma includes, sqlx queries) -- load the language skill for implementation details
- **architecture-patterns**: Multi-tenancy, sharding strategies, saga orchestration -- load `skills/architecture-patterns/SKILL.md`
- **testing-patterns**: Integration testing patterns, CI database setup -- load `skills/testing-patterns/references/integration-testing.md`
- **security-audit**: Deep SQL injection testing, database threat modeling -- load `skills/security-audit/SKILL.md`
- **kubernetes-patterns**: StatefulSet for databases, init container migrations -- load `skills/kubernetes-patterns/SKILL.md`
