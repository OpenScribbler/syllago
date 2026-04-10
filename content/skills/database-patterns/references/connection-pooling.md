# Connection Pooling

## Why Pooling Matters

- PostgreSQL forks a process per connection (~10MB each). Without a pooler, 500+ connections consume 5GB+ RAM on the DB server alone.
- MySQL uses threads (lighter), but still benefits from pooling to avoid connection setup overhead.
- Rule: always use a connection pooler in production for PostgreSQL. Consider one for MySQL at scale.

## PgBouncer

### Transaction Mode (Production Default)

- Each query/transaction gets a connection, returned to pool immediately after.
- Handles thousands of application connections with tens of database connections.
- **Caveats in transaction mode** (features that require session affinity):
  - No prepared statements (use `simple_query` protocol or named prepared statements with `pgbouncer_prepared_statement_pool_size`)
  - No temporary tables
  - No LISTEN/NOTIFY
  - No advisory locks spanning transactions
  - No SET statements (session variables reset between transactions)

### Session Mode

- Pins a connection to a client for the session lifetime. Use only when transaction mode caveats are blocking.
- Reduces pooling efficiency significantly.

### Statement Mode

- Each individual statement gets a connection. Prevents multi-statement transactions entirely.
- Use only for simple read replicas with autocommit workloads.

## Pool Sizing

### Database Side

```
total_pool ≤ max_connections - reserved_connections
```

- Reserve 5-10 connections for admin/monitoring (pg_stat, migrations, manual debugging).
- `max_connections` default is 100 in PostgreSQL. Increase cautiously -- each connection costs memory.

### Application Side

```
instances x pool_size ≤ total_pool
```

- Start with `pool_size = num_cpu_cores` per instance.
- Gotcha: Kubernetes rolling deploys temporarily double instance count. Pool sizing must account for peak: `(instances x 2) x pool_size ≤ total_pool`.

### Sizing Formula

| Parameter | Starting Value |
|-----------|---------------|
| `max_connections` (PG) | 200-500 depending on server RAM |
| PgBouncer `default_pool_size` | 20-50 per database |
| App `pool_size` (behind PgBouncer) | 2-5 (pooler handles multiplexing) |
| App `pool_size` (direct to DB) | num_cores (5-20 typically) |

- Rule: app pool behind PgBouncer should be small (2-5). The pooler is the multiplexer -- large app pools defeat its purpose.

## Language Defaults

| Language/Library | Default Pool Size | Config Key |
|-----------------|-------------------|------------|
| Go pgx | 4 (MaxConns) | `MaxConns` |
| Python SQLAlchemy | 5 (pool_size) | `pool_size` + `max_overflow` |
| Node.js pg | 10 (max) | `max` |
| Rust sqlx | 10 (max_connections) | `max_connections` |
| Java HikariCP | 10 (maximumPoolSize) | `maximumPoolSize` |

- Rule: always explicitly set pool size. Do not rely on library defaults -- they are rarely optimal for your workload.

## Connection Health

- Configure connection validation: `SELECT 1` or driver-native ping before checkout from pool.
- Set `max_lifetime` (Go) / `pool_recycle` (Python) to recycle connections before the database server kills them (default PostgreSQL `idle_session_timeout` or `tcp_keepalives_idle`).
- Set `idle_timeout` to release unused connections during low traffic.

## Monitoring Checklist

- [ ] Pool utilization (active vs idle vs max) is dashboarded
- [ ] Connection wait time is tracked (high wait = pool too small)
- [ ] Connection errors (refused, timeout) are alerted on
- [ ] Rolling deploy scenario tested with 2x instance count
- [ ] PgBouncer `show pools` and `show stats` are monitored
