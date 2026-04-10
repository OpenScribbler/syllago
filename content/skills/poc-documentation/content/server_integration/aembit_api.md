## Prerequisites

- Aembit tenant with Administrator access
- Aembit Agent Proxy deployed on the client workload (Agent Controller or deployment module completed)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AEMBIT_API_ROLE}}` | The Aembit role to assign to the credential (e.g., `Auditor`). Navigate to **Administration → Roles** to view available roles. |

## Service Configuration

No external service setup is required. The Aembit API is accessed using credentials generated within the same Aembit tenant.

## Aembit Configuration

> **Start Here:** The Credential Provider generates the Audience value needed for the Server Workload host. Complete step 1 first, then use the Audience value in step 2.

1. Navigate to **Credential Providers** and click **+ New**
   - **Credential Type**: select **Aembit Access Token**
   - **Name**: descriptive label (e.g., `Aembit API - {{AEMBIT_API_ROLE}}`)
   - **Audience**: auto-generated (read-only) - copy this value for the Server Workload host
   - **Role**: select `{{AEMBIT_API_ROLE}}` (use least-privilege: assign the role with the minimum permissions required)
   - **Lifetime**: token validity period
   - Click **Save**

2. Navigate to **Server Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `Aembit API`)
   - **Host**: paste the **Audience** value from the Credential Provider (step 1)
   - **Application Protocol**: HTTP
   - **Port**: 443
   - **Forward to Port**: 443, enable **TLS**
   - **Authentication Method**: HTTP Authentication
   - **Authentication Scheme**: Bearer
   - Enable **TLS Decrypt**
   - Click **Save**

## Verification

- The application accesses the Aembit API without static API keys or tokens in its environment
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show credentials were injected for the Aembit API server workload
- API responses return successfully with the permissions matching the assigned role

## Troubleshooting

- **API returns 401 Unauthorized:** The Credential Provider role does not have sufficient permissions for the API endpoint being called. Verify the role in **Administration → Roles** includes the required permissions
- **Application receives no credentials:** The Access Policy is not active or the client workload identity does not match. Navigate to **Access Policies** and confirm the policy status and client workload selector
- **TLS errors or connection refused:** TLS Decrypt is not enabled on the Server Workload. Enable **TLS Decrypt** in the Server Workload configuration - this is required when using Agent Proxy to access the Aembit API
- **Wrong Audience value:** The Server Workload host must match the Audience value from the Credential Provider exactly. Copy it directly from the Credential Provider dialog
