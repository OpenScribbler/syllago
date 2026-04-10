## Prerequisites

- Snowflake account with a service account user
- Snowflake Account Identifier: `{{SNOWFLAKE_ACCOUNT_IDENTIFIER}}`
- Snowflake Service User: `{{SNOWFLAKE_SERVICE_USER}}`

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{SNOWFLAKE_ACCOUNT_IDENTIFIER}} | Snowflake console → bottom-left account menu → Account Identifier (format: `orgname-accountname`) |
| {{SNOWFLAKE_SERVICE_USER}} | Snowflake console → Admin → Users & Roles → the service account login name |

## Service Configuration

### Snowflake Setup

> **Note:** Do not create or assign a key pair to the Snowflake service user manually — Aembit generates the RSA keypair automatically in the step below and provides the ALTER USER command to bind it.

1. In Snowflake, confirm the service account user: `{{SNOWFLAKE_SERVICE_USER}}`

## Aembit Configuration

1. Navigate to **Credential Providers** and create a new **Snowflake JWT Credential Provider**
   - Snowflake User: `{{SNOWFLAKE_SERVICE_USER}}`
   - Aembit will generate an RSA keypair automatically
   - Copy the **ALTER USER** SQL command shown in the UI

2. In the Snowflake web UI, navigate to **Worksheets**, open a new SQL worksheet, and run the ALTER USER command to bind the keypair to the service user

3. Navigate to **Server Workloads** and create a new **Snowflake Server Workload**
   - Account: `{{SNOWFLAKE_ACCOUNT_IDENTIFIER}}`

## Verification

- The Snowflake Credential Provider shows as **Active** in the Aembit Management Console
- The application successfully connects to Snowflake without static credentials
- Navigate to **Activity** and confirm log entries show the workload authenticated and received a JWT credential

## Troubleshooting

- **Snowflake authentication rejected:** The ALTER USER command was not run, or the keypair was not bound to the correct service user. Re-run the ALTER USER SQL command from the Credential Provider UI
- **Wrong account identifier format:** Snowflake account identifiers use the format `orgname-accountname`. Verify the value in Snowflake console → bottom-left account menu
- **Credential Provider shows Inactive:** Check that the Snowflake service user exists and has not been locked or disabled
- **Snowflake rejects JWT: JWT token is expired or invalid:** Verify that the clock on the system running the Aembit Agent is synchronized (NTP). Snowflake enforces strict JWT timestamp validation; clock skew greater than a few minutes will cause token rejection
