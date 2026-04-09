# Indexing

## Composite Index Ordering

- Rule: place equality predicates first, range predicates last in composite indexes.
- `CREATE INDEX idx_orders_status_created ON orders(status, created_at)` supports:
  - `WHERE status = 'active' AND created_at > '2024-01-01'` (both columns used)
  - `WHERE status = 'active'` (prefix used)
  - But NOT `WHERE created_at > '2024-01-01'` alone (prefix skipped)
- Gotcha: adding a column after a range predicate in a composite index provides no selectivity benefit -- the B-tree scan stops being ordered after the range column.

## Index Types

### Covering Indexes

```sql
CREATE INDEX idx_orders_user_covering
  ON orders(user_id)
  INCLUDE (status, total);
```

- Eliminates heap fetches when all queried columns are in the index.
- Use for hot read paths where you SELECT a small subset of columns.
- Trade-off: larger index size, slower writes.

### Partial Indexes

```sql
CREATE INDEX idx_orders_pending
  ON orders(created_at)
  WHERE status = 'pending';
```

- Indexes only rows matching the predicate. Dramatically smaller for skewed distributions.
- Common use: active records, pending items, unprocessed queues.
- Gotcha: queries must include the exact WHERE clause for the planner to use the partial index.

### Expression Indexes

```sql
CREATE INDEX idx_users_email_lower ON users(LOWER(email));
```

- Required when queries use functions on columns. Without this, the index on `email` is not used for `WHERE LOWER(email) = ...`.
- Rule: prefer rewriting queries to avoid functions on columns when possible (better for standard index usage).

## When NOT to Index

- **Low-cardinality columns**: Boolean or status columns with 2-5 values -- full table scan is often faster than index + heap fetch.
  - Exception: partial index on the rare value (e.g., `WHERE is_admin = true` when 0.1% of rows are admins).
- **Small tables**: Tables under ~10K rows -- sequential scan is faster than index lookup.
- **Write-heavy tables with rare reads**: Each index slows INSERT/UPDATE/DELETE. Only index what is queried.
- **Columns rarely in WHERE/JOIN/ORDER BY**: Indexes on columns only in SELECT provide no query benefit (unless covering).

## Multi-Tenant Index Gotcha

- Rule: every index in a multi-tenant database MUST include `tenant_id` as the first column.
- Without tenant prefix, queries filter by tenant after the index scan, reading far more pages than necessary.
- Example: `idx_orders_tenant_status ON orders(tenant_id, status)` not `idx_orders_status ON orders(status)`.

## Production Index Workflow

1. **Identify slow queries**: `pg_stat_statements` (PostgreSQL) or slow query log (MySQL)
2. **Analyze**: `EXPLAIN ANALYZE` on the slow query -- look for Seq Scan on large tables, estimate vs actual row divergence
3. **Prototype**: Use HypoPG (PostgreSQL) to test hypothetical indexes without creating them
4. **Create**: `CREATE INDEX CONCURRENTLY` (PostgreSQL) to avoid blocking writes
5. **Verify**: Re-run `EXPLAIN ANALYZE` to confirm the index is used
6. **Audit**: Periodically check `pg_stat_user_indexes` for unused indexes (zero scans) -- drop them

## Index Audit Checklist

- [ ] All foreign key columns are indexed
- [ ] Composite indexes follow equality-first, range-last ordering
- [ ] Multi-tenant indexes have tenant_id as first column
- [ ] No unused indexes (check pg_stat_user_indexes)
- [ ] Hot read paths use covering indexes where beneficial
- [ ] Partial indexes used for skewed distributions
- [ ] All indexes created with CONCURRENTLY on live tables
