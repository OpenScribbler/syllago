# Schema Design

## DB Selection Framework

Choose based on workload characteristics, not popularity:

| Database | Best For | Key Strengths |
|----------|----------|---------------|
| PostgreSQL | Complex queries, mixed workloads | JSONB+GIN, RLS, advisory locks, PostGIS, LISTEN/NOTIFY, rich type system |
| MySQL | Simple predictable OLTP, stable schemas | Proven replication, wide hosting support, team familiarity |
| SQLite | Embedded, single-writer, edge/serverless | Zero-config, <few GB datasets, file-based |

- Rule: if the team has deep expertise in one engine, that is a legitimate reason to stay -- migration cost is real.
- Rule: SQLite is NOT for multi-user production web services. Its single-writer lock serializes all writes.
- Rule: when in doubt, default to PostgreSQL -- it handles the widest range of future requirements.

## Normalization

### Rules

- Default to 3NF for OLTP workloads. Denormalize only when you have measured query performance evidence.
- Denormalization trade-off: faster reads, slower writes, risk of data inconsistency. Document why you denormalized.
- Acceptable denormalization: materialized views, read replicas with flattened schemas, caching layers.
- Anti-pattern: premature denormalization "for performance" without benchmarks.

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Tables | snake_case, plural | `user_accounts`, `order_items` |
| Columns | snake_case | `created_at`, `first_name` |
| Foreign keys | `<singular_table>_id` | `user_id`, `order_id` |
| Timestamps | `*_at` suffix, TIMESTAMPTZ | `created_at`, `deleted_at` |
| Booleans | `is_*` or `has_*` prefix | `is_active`, `has_verified` |
| Indexes | `idx_<table>_<columns>` | `idx_orders_user_id_created_at` |
| Constraints | `<type>_<table>_<columns>` | `uq_users_email`, `chk_orders_total_positive` |

## Data Types

### Rules

- **Timestamps**: Always TIMESTAMPTZ, never TIMESTAMP. Bare timestamps lose timezone context and cause bugs across regions.
- **Money**: NUMERIC(19,4) or DECIMAL. Never FLOAT/DOUBLE -- floating-point arithmetic rounds incorrectly for financial calculations.
- **Primary keys**: BIGSERIAL for internal-only IDs, UUID (v4 or v7) when IDs cross service boundaries or are exposed to clients. UUID v7 is time-sortable (better for B-tree indexes).
- **Text**: In PostgreSQL, prefer TEXT over VARCHAR(n) -- there is no performance difference, and VARCHAR(n) requires future migrations when limits change. In MySQL, use VARCHAR(n) with intentional limits.
- **JSON**: In PostgreSQL, always JSONB (indexable, queryable) not JSON (stored as text). In MySQL, use JSON type (binary storage since 5.7).
- **Enums**: Use CHECK constraints or reference tables, not database-level ENUM types -- ENUM modifications require schema migrations.

## Soft Deletes

- Add `deleted_at TIMESTAMPTZ NULL` early in table design. Retrofitting requires rebuilding indexes and updating every query.
- Add a partial index: `CREATE INDEX idx_<table>_active ON <table>(...) WHERE deleted_at IS NULL` to keep queries fast.
- Gotcha: unique constraints must be filtered to exclude soft-deleted rows, or use a composite unique including `deleted_at`.

## Standard Audit Columns

Every table should include:

```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ
```

- `created_at` is immutable, always set by the database.
- `updated_at` is set by the application or trigger on modification.
- For regulatory audit: add `created_by` and `updated_by` columns referencing the acting user/service.

## Schema Review Checklist

- [ ] All tables have explicit primary keys
- [ ] Foreign keys have ON DELETE behavior specified (CASCADE, SET NULL, or RESTRICT)
- [ ] Timestamps use TIMESTAMPTZ
- [ ] Money columns use NUMERIC/DECIMAL
- [ ] Boolean columns use `is_`/`has_` prefix
- [ ] Indexes exist on all foreign key columns
- [ ] Unique constraints exist where business rules require them
- [ ] Soft-delete pattern includes partial indexes
- [ ] No ENUM types -- use CHECK constraints or reference tables
