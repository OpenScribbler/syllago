# Query Patterns

## EXPLAIN ANALYZE

- Rule: always run `EXPLAIN ANALYZE` before optimizing. Intuition about query performance is frequently wrong.
- Key indicators of problems:
  - **Seq Scan on large tables**: Missing or unused index
  - **Estimate vs actual row divergence**: Stale statistics (`ANALYZE table_name`)
  - **Nested Loop with large outer set**: Consider Hash Join via better indexing or query rewrite
  - **Sort with high memory**: Add index matching ORDER BY, or increase `work_mem` for the session
  - **Bitmap Heap Scan with high lossy blocks**: Index is helping but not enough selectivity

## N+1 Problem

The single most common ORM performance failure:

```
-- N+1: 1 query for orders + N queries for users (one per order)
SELECT * FROM orders;                    -- 1 query
SELECT * FROM users WHERE id = 1;        -- repeated N times
SELECT * FROM users WHERE id = 2;
...
```

### Detection

- Query count logging middleware: if a single request executes >10 queries, investigate.
- Application Performance Monitoring (APM) tools flag N+1 patterns automatically.
- In tests: assert query count per operation.

### Universal Fixes

- **Eager loading**: Load associations in a single JOIN query. ORM-specific syntax varies (see language skills).
- **Batch loading**: Load associations with `WHERE id IN (...)`. Works without JOINs, good for polymorphic relations.
- **DataLoader pattern**: Batch and deduplicate loads within a request cycle. Standard in GraphQL, applicable anywhere.

## ORM vs Raw SQL Decision

| Use ORM For | Use Raw SQL For |
|-------------|-----------------|
| Standard CRUD operations | Complex multi-table JOINs |
| Simple WHERE/ORDER/LIMIT queries | Window functions (ROW_NUMBER, LAG, LEAD) |
| Schema-tracked associations | Recursive CTEs |
| Rapid prototyping | Performance-critical hot paths |
| Type-safe query building | Bulk operations (INSERT...ON CONFLICT) |
| Migration management | Database-specific features (LATERAL JOIN, JSONB operators) |

- Rule: hybrid is the production standard. Use ORM for 80% of queries, drop to raw SQL for the complex/hot 20%.
- Rule: wrap raw SQL in repository methods -- callers should not know whether a query is ORM or raw.

## Repository Pattern

- Abstract all database access behind interfaces/classes. Centralizes query optimization and enables ORM-to-raw-SQL swapping without caller changes.
- Repository methods should return domain objects, not ORM models or raw rows.
- Keep query logic in the repository layer -- controllers/handlers should not construct queries.

## Query Anti-Patterns

### Functions on Indexed Columns

- `WHERE YEAR(created_at) = 2024` cannot use an index on `created_at`.
- Fix: rewrite as range predicate: `WHERE created_at >= '2024-01-01' AND created_at < '2025-01-01'`.
- Same applies to: `LOWER()`, `CAST()`, `COALESCE()`, `DATE()`.

### EXISTS vs IN

- `WHERE id IN (SELECT id FROM ...)` materializes the full subquery result.
- `WHERE EXISTS (SELECT 1 FROM ... WHERE ...)` short-circuits on first match.
- Rule: prefer EXISTS for correlated subqueries, especially when the subquery returns many rows.

### Pagination

- OFFSET pagination degrades linearly: `OFFSET 100000` scans and discards 100K rows.
- Keyset pagination is constant-time: `WHERE id > :last_seen_id ORDER BY id LIMIT 20`.
- See `skills/api-design-patterns/references/pagination.md` for API-level pagination design.

### SELECT *

- Wastes network bandwidth on unused columns.
- Prevents covering index optimization.
- Breaks when columns are added/removed.
- Rule: always list columns explicitly in production queries.

## Batch Operations

- For bulk inserts: use multi-row INSERT or COPY (PostgreSQL) instead of row-by-row.
- For upserts: `INSERT ... ON CONFLICT DO UPDATE` (PostgreSQL) / `INSERT ... ON DUPLICATE KEY UPDATE` (MySQL).
- For bulk updates: use CTEs with UPDATE...FROM or temporary staging tables for large datasets.
- Rule: batch size of 1,000-10,000 rows per statement. Too large risks lock duration and memory; too small wastes round trips.
