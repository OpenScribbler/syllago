## Prerequisites

- Box developer account with ability to create Custom Apps in the Box Developer Console
- Access to a Box Admin Console to authorize the application (or coordination with a Box administrator)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{BOX_ENTERPRISE_ID}} | Box Developer Console → your app → **General Settings** → **Enterprise ID** |
| {{BOX_CLIENT_ID}} | Box Developer Console → your app → **Configuration** → **OAuth 2.0 Credentials** → **Client ID** |
| {{BOX_CLIENT_SECRET}} | Box Developer Console → your app → **Configuration** → **OAuth 2.0 Credentials** → **Client Secret** |

## Service Configuration

### Box Developer Console Setup

1. In the Box Developer Console, click **Create New App**
   - Select **Custom App**
   - Authentication method: **Server Authentication (Client Credentials Grant)**
   - Enter an app name and click **Create App**

2. On the app **Configuration** tab, configure the application scopes for the use case (e.g., **Read all files and folders stored in Box**, **Write all files and folders stored in Box**)

3. Submit the app for admin authorization:
   - On the **Authorization** tab, click **Review and Submit**
   - The app must be authorized in the Box Admin Console at **Integrations → Platform Apps Manager** before credentials can be obtained

4. After authorization, collect the following values from the app:
   - **Enterprise ID** (`{{BOX_ENTERPRISE_ID}}`) — from the **General Settings** tab
   - **Client ID** (`{{BOX_CLIENT_ID}}`) and **Client Secret** (`{{BOX_CLIENT_SECRET}}`) — from the **Configuration** tab under **OAuth 2.0 Credentials**

## Aembit Configuration

1. Navigate to **Credential Providers** and create a new **OAuth 2.0 Client Credentials Credential Provider**
   - **Token URL**: `https://api.box.com/oauth2/token`
   - **Client ID**: `{{BOX_CLIENT_ID}}`
   - **Client Secret**: `{{BOX_CLIENT_SECRET}}`
   - **Scopes**: leave empty to use the Developer Console defaults, or specify (e.g., `root_readonly`)
   - **Credential Style**: POST Body
   - Under **Additional Parameters**, add the following key-value pairs (required to authenticate as the enterprise service account):
     - Key: `box_subject_type` / Value: `enterprise`
     - Key: `box_subject_id` / Value: `{{BOX_ENTERPRISE_ID}}`

2. Navigate to **Server Workloads** and create a new **HTTPS Server Workload** for Box API
   - **Host**: `api.box.com`
   - **Port**: 443
   - **Protocol**: HTTPS
   - **Authentication**: Bearer

## Verification

- The Aembit Credential Provider shows as **Active** (confirms the client credentials grant is working and Box accepted the enterprise authentication parameters)
- The application makes successful Box API calls without static credentials
- Navigate to **Activity** and confirm log entries show the workload authenticated and credentials were injected

## Troubleshooting

- **Box API returns 401 or "App not authorized":** The app has not been authorized in the Box Admin Console. The Box admin must approve the app under **Apps → Custom Apps Manager** before the Client Credentials Grant will succeed
- **Box API returns 400 with invalid `box_subject_type`:** Confirm the `box_subject_type` additional parameter is set to `enterprise` (not `user`) and `box_subject_id` is the Enterprise ID (not a user ID)
- **No Activity log entries:** The application is making requests directly to Box rather than through the Aembit Agent Proxy. Verify the proxy is running and the application's outbound requests are routed through it
