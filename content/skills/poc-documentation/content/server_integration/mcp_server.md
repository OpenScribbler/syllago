## Prerequisites

- MCP server vendor account with ability to create OAuth 2.0 applications
- OAuth 2.0 credentials from the MCP server vendor: Client ID, Client Secret, Authorization URL, and Token URL

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{MCP_SERVER_NAME}}` | Descriptive name for this MCP server (e.g., `Box`, `Microsoft 365`, `FactSet`) |
| `{{MCP_SERVER_HOST}}` | Hostname of the MCP server (e.g., `mcp.box.com`) - provided by the MCP server vendor |
| `{{MCP_CLIENT_ID}}` | MCP server vendor admin console - OAuth application Client ID |
| `{{MCP_CLIENT_SECRET}}` | MCP server vendor admin console - OAuth application Client Secret |
| `{{MCP_AUTH_URL}}` | MCP server vendor documentation - OAuth 2.0 Authorization URL |
| `{{MCP_TOKEN_URL}}` | MCP server vendor documentation - OAuth 2.0 Token URL |
| `{{MCP_GATEWAY_ENDPOINT}}` | Aembit MCP Identity Gateway hostname (coordinate with Aembit Solutions Engineer) |
| `{{MCP_USER_EMAIL}}` | Email address of the user authorized to access this MCP server |

## Service Configuration

### MCP Server Vendor Setup

1. In the MCP server vendor's admin console, create or configure an OAuth 2.0 application
   - Note the **Client ID** (`{{MCP_CLIENT_ID}}`) and **Client Secret** (`{{MCP_CLIENT_SECRET}}`)
   - Note the **Authorization URL** (`{{MCP_AUTH_URL}}`) and **Token URL** (`{{MCP_TOKEN_URL}}`)
   - *(You will add the Aembit Callback URL to the OAuth application's redirect URIs in Aembit Configuration step 3 below)*

> **Note:** Refer to the MCP server vendor's documentation for OAuth application setup. Each vendor's admin console and configuration steps differ.

## Aembit Configuration

1. Navigate to **Client Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `{{MCP_USER_EMAIL}}`)
   - Add an **OIDC ID Token Subject** identifier: `{{MCP_USER_EMAIL}}`
   - Click **Save**

2. Navigate to **Trust Providers** and click **+ New**
   - **Name**: `MCP Gateway Users`
   - **Issuer URL**: `https://{{AEMBIT_TENANT_ID}}.id.useast2.aembit.io`
   - **Audience**: `https://{{MCP_GATEWAY_ENDPOINT}}/`
   - **Attestation Method**: OIDC Discovery
   - **OIDC Endpoint**: `https://{{AEMBIT_TENANT_ID}}.id.useast2.aembit.io`
   - Click **Save**

3. Navigate to **Credential Providers** and click **+ New**
   - **Credential Type**: select **MCP User-Based Access Token**
   - **Name**: `{{MCP_SERVER_NAME}}`
   - **MCP Server URL**: `https://{{MCP_SERVER_HOST}}`
   - **Client ID**: `{{MCP_CLIENT_ID}}`
   - **Client Secret**: `{{MCP_CLIENT_SECRET}}`
   - **Authorization URL**: `{{MCP_AUTH_URL}}`
   - **Token URL**: `{{MCP_TOKEN_URL}}`
   - Enable **PKCE Required**
   - Copy the **Callback URL** and add it to the MCP server vendor's OAuth application redirect URIs (from Service Configuration step 1)
   - Click **Save**

4. Navigate to **Server Workloads** and click **+ New**
   - **Name**: `{{MCP_SERVER_NAME}}`
   - **Host**: `{{MCP_SERVER_HOST}}`
   - **Application Protocol**: MCP
   - **Transport Protocol**: TCP
   - **Port**: 443, enable **TLS**
   - Click **Save**

## Verification

- In Claude, send a prompt that requires access to the MCP server and confirm results are returned
- Navigate to **Reporting** in the Aembit Management Console and confirm two policy chain entries appear: one for the MCP client to the gateway and one for the user to the MCP server
- Each activity entry shows the authenticated identity and the target workload

## Troubleshooting

- **OAuth error during MCP server authentication:** The Callback URL was not added to the vendor's OAuth application redirect URIs. Copy the Callback URL from the Aembit Credential Provider and add it in the vendor's admin console
- **User not authorized to MCP server:** The Client Workload OIDC ID Token Subject does not match the user's email. Verify the email address matches exactly
- **MCP server returns empty results:** The OAuth application scopes may be insufficient. Check the vendor's admin console for scope configuration
- **Two policy chain entries not appearing:** Verify both Access Policies are active - one linking the MCP client to the gateway and one linking the user Client Workload to the MCP Server Workload
