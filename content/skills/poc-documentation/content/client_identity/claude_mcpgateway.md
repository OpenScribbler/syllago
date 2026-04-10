## Prerequisites

- Okta tenant with user groups configured
- Okta Issuer URL: `{{OKTA_ISSUER_URL}}`
- Claude account

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{OKTA_ISSUER_URL}} | Okta Admin → Security → API → Authorization Servers → select server → Issuer URI |

## Aembit Configuration

1. Navigate to **Client Workloads** and create a new Client Workload for Claude
   - Name: Claude
   - Identifier type: **Redirect URI**
   - Value: `https://claude.ai/api/mcp/auth_callback`
   - *(Claude Desktop, Claude Code, and Claude Web share the same redirect URI: this single Client Workload covers all Claude clients)*

2. Navigate to **Trust Providers** and create a new **OIDC Trust Provider** for Okta
   - Name: Okta OIDC
   - Match Rules:
      - Issuer URL: `{{OKTA_ISSUER_URL}}`

3. Navigate to **Credential Providers** and create a new **Credential Provider**
   - Name: Aembit OIDC
   - Credential Type: OIDC ID Token

   Configure the following fields:
   - **Subject** field — enable the **Dynamic** toggle and enter: `${oidc.identityToken.decode.payload.email}`. This puts the email claim from your Okta identity token into the sub claim of your Aembit identity token
   - **Audience** field — enter `https://{{MCP_GATEWAY_ENDPOINT}}/`
   - **Signing Algorithm Type** dropdown — select **RSASSA-PKCS1-v1_5 using SHA-256** (RS256)
   - **Lifetime** field — enter the desired token lifetime in minutes. This controls how frequently you will need to reauthenticate to the MCP Gateway

4. Navigate to **Server Workloads** and create a new Server Workload for the MCP Gateway
   - Name: MCP Gateway
   - Host: `{{MCP_GATEWAY_ENDPOINT}}`
   - Application Protocol: MCP
   - Transport Protocol: TCP
   - Port: 443 (Check TLS Box)
   - URL Path: `/mcp`
