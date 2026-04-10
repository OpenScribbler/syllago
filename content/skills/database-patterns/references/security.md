# Database Security

## Parameterized Queries

- Rule: parameterized queries everywhere, no exceptions. Every language and driver supports them.
- Never string-format SQL, even for values you consider "safe" (like column names or table names).

| Language | Placeholder | Example |
|----------|-------------|---------|
| Go (pgx/sqlx) | `$1, $2` | `db.Query("SELECT * FROM users WHERE id = $1", id)` |
| Python (psycopg) | `%s` or `%(name)s` | `cur.execute("SELECT * FROM users WHERE id = %s", (id,))` |
| Node (pg) | `$1, $2` | `pool.query('SELECT * FROM users WHERE id = $1', [id])` |
| Rust (sqlx) | `$1, $2` | `sqlx::query("SELECT * FROM users WHERE id = $1").bind(id)` |
| Java (JDBC) | `?` | `ps = conn.prepareStatement("SELECT * FROM users WHERE id = ?")` |

- Gotcha: dynamic column/table names cannot be parameterized. Validate against an explicit allowlist of permitted names.
- Gotcha: `IN` clauses with variable-length lists need special handling per driver (array parameters or query building).

## Least Privilege

### Role Separation

| Role | Permissions | Used By |
|------|------------|---------|
| `app_readonly` | SELECT on application tables | Read replicas, reporting, analytics |
| `app_readwrite` | SELECT, INSERT, UPDATE, DELETE | Application services |
| `app_migration` | DDL (CREATE, ALTER, DROP) + DML | Migration runner only |
| `app_admin` | SUPERUSER or equivalent | Emergency access only, audited |

- Rule: the application runtime should NEVER use a migration or admin credential.
- Rule: each microservice should have its own database user, scoped to its tables/schemas.
- PostgreSQL: use `GRANT` on schemas and tables. Consider Row-Level Security (RLS) for multi-tenant isolation.

## Credential Management

- Store credentials in a secrets manager: HashiCorp Vault, AWS Secrets Manager, GCP Secret Manager, Azure Key Vault, Kubernetes External Secrets Operator.
- Never commit credentials to source control, even in "private" repos.
- Rotate credentials regularly. Vault and cloud secret managers support automatic rotation.
- Connection strings: use environment variables or secret-mounted files, never config files in the repo.

## Network Security

- Database servers must NOT be internet-accessible. Place behind private subnets/VPCs.
- Enforce TLS for all connections (`sslmode=verify-full` for PostgreSQL, `require_secure_transport=ON` for MySQL).
- Use firewall rules / security groups to restrict access to known application IPs or VPC CIDR ranges.

## Encryption

- **In transit**: TLS mandatory (see above).
- **At rest**: Enable storage-level encryption (AWS RDS encryption, Azure TDE, GCP default encryption). This is transparent to the application.
- **Application-level encryption**: For highly sensitive fields (PII, payment data), encrypt before storing. Use envelope encryption (data key encrypted by master key). Trade-off: encrypted fields cannot be indexed or queried server-side.

## Audit Logging

- Log all DDL operations (schema changes) and privileged DML (bulk deletes, admin actions).
- PostgreSQL: use `pgaudit` extension for statement-level audit logging.
- MySQL: enable general query log or audit plugin for compliance.
- Never log query parameters containing credentials, PII, or payment data -- redact sensitive values.

## Security Checklist

- [ ] All queries use parameterized placeholders
- [ ] No string concatenation in SQL construction
- [ ] Separate DB users per service role (read, write, migrate, admin)
- [ ] Credentials in secrets manager, not source control
- [ ] TLS enforced on all database connections
- [ ] Database not internet-accessible
- [ ] Encryption at rest enabled
- [ ] Audit logging configured for DDL and privileged operations
