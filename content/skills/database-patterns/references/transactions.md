# Transactions

## Isolation Levels

| Level | Dirty Read | Non-Repeatable Read | Phantom Read | Use Case |
|-------|-----------|-------------------|-------------|----------|
| Read Uncommitted | Yes | Yes | Yes | Almost never appropriate |
| Read Committed | No | Yes | Yes | PostgreSQL default; general OLTP |
| Repeatable Read | No | No | Yes (PG: No) | MySQL InnoDB default; consistent reads within transaction |
| Serializable | No | No | No | Financial calculations, inventory; highest overhead |

- Rule: PostgreSQL Read Committed is correct for most workloads. Escalate only with measured evidence of anomalies.
- Rule: MySQL Repeatable Read is the default and generally fine. Be aware of gap locking implications.
- Gotcha: PostgreSQL's Repeatable Read also prevents phantom reads (unlike the SQL standard). Serializable adds serialization conflict detection on top.

## Locking Strategies

### Pessimistic Locking (SELECT FOR UPDATE)

- Acquires a row lock until the transaction commits or rolls back.
- Best for: write-heavy workloads, frequent conflicts, critical sections where retry is expensive.
- Gotcha: deadlocks are possible when multiple transactions lock rows in different orders. Always lock in consistent order.
- Variant: `SELECT FOR UPDATE SKIP LOCKED` for queue-like patterns (workers grab unlocked rows).

### Optimistic Locking (Version Column)

```sql
UPDATE orders SET status = 'shipped', version = version + 1
WHERE id = 123 AND version = 5;
-- If affected rows = 0, another transaction modified the row: retry or fail
```

- Best for: read-heavy workloads, low conflict probability, user-facing forms (long think time).
- Implementation: add `version INTEGER NOT NULL DEFAULT 0` column, increment on every update, check affected rows.
- Trade-off: no locks held during reads, but retries needed on conflict. High conflict rates make this worse than pessimistic.

## Distributed Transactions

### Saga Pattern

- For cross-service data consistency. Each service performs a local transaction and publishes an event.
- **Choreography**: services react to events independently. Simpler but harder to debug.
- **Orchestration**: central coordinator directs the saga steps. Easier to understand and monitor.
- Rule: never use two-phase commit (2PC) in microservices -- it couples services, blocks on coordinator failure, and does not scale.

### Compensating Transactions

- Every saga step needs a compensating action for rollback (e.g., CreateOrder -> CancelOrder, ChargePayment -> RefundPayment).
- Design compensations to be idempotent -- they may be retried.

## Transaction Anti-Patterns

- **Long-running transactions**: Hold locks, block autovacuum (PostgreSQL), increase replication lag. Keep transactions under seconds, not minutes.
- **Transactions spanning HTTP requests**: A request starts a transaction, waits for external API, then commits. Network latency holds the transaction open. Move external calls outside the transaction.
- **Relying on Serializable without measuring**: Serializable increases retry rate. Benchmark the conflict rate before committing to it.
- **Implicit transactions in ORMs**: Some ORMs wrap every query in a transaction. Understand your ORM's behavior and disable autocommit wrapping where not needed.
- **Nested transaction confusion**: Most databases do not support true nested transactions. ORMs simulate them with SAVEPOINTs -- understand the difference.

## Transaction Checklist

- [ ] Isolation level chosen deliberately, not by default
- [ ] Locking strategy matches conflict frequency (optimistic vs pessimistic)
- [ ] No external calls (HTTP, messaging) inside transactions
- [ ] Transaction duration is bounded (timeouts configured)
- [ ] Deadlock ordering is consistent across code paths
- [ ] Saga compensations are idempotent
