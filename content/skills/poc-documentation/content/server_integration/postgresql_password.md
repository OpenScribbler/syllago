## Prerequisites

- PostgreSQL or EnterpriseDB instance accessible from the client workload
- Database hostname: `{{DB_HOSTNAME}}`
- Service account username and password

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{DB_HOSTNAME}} | Database connection string or provided by your DBA |
| {{DB_PORT}} | Database connection string (default: `5432`) |
| {{DB_USERNAME}} | Service account login name - provided by your DBA |
| {{DB_PASSWORD}} | Service account password - provided by your DBA |

## Service Configuration

### Database Setup

1. Confirm your database hostname/IP (`{{DB_HOSTNAME}}`), port (`{{DB_PORT}}`), and service account credentials
   - Ensure the service account has CONNECT privilege on the target database
   - No other database changes are required: Aembit injects the existing credentials at runtime

## Aembit Configuration

1. Navigate to **Credential Providers** and create a new **PostgreSQL Credential Provider**
   - Username: `{{DB_USERNAME}}`
   - Password: `{{DB_PASSWORD}}`

2. Navigate to **Server Workloads** and create a new **PostgreSQL Server Workload**
   - Host: `{{DB_HOSTNAME}}`
   - Port: `{{DB_PORT}}` (default: 5432)
   - Protocol: PostgreSQL Password Authentication

3. Remove any existing database credentials from the application's configuration (connection string, environment variables, secrets manager calls) — the Aembit Agent Proxy injects credentials at runtime

## Verification

- The application connects to PostgreSQL successfully without static credentials in its configuration
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show the workload authenticated and received a credential injection

## Troubleshooting

- **Connection refused:** Verify the hostname (`{{DB_HOSTNAME}}`) and port (`{{DB_PORT}}`) are correct and the database is reachable from the client workload
- **Authentication failed:** Verify the username and password in the Aembit Credential Provider match the service account. Confirm the service account has CONNECT privilege on the target database
- **No Activity log entries:** The client workload is connecting directly to the database, bypassing Aembit. Verify the Agent Proxy is running and the application is not using a hardcoded connection string
