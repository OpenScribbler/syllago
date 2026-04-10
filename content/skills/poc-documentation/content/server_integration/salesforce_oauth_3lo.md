## Prerequisites

- Salesforce org with System Administrator access
- Ability to create a Connected App (or External Client App) in Salesforce Setup
- Aembit Agent Proxy deployed on the client workload (Agent Controller module completed)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{SALESFORCE_INSTANCE_URL}}` | Salesforce org URL (format: `yourorg.my.salesforce.com`) — visible in the browser address bar when logged in |
| `{{SALESFORCE_CLIENT_ID}}` | Salesforce Setup → App Manager → your Connected App → **Consumer Key** |
| `{{SALESFORCE_CLIENT_SECRET}}` | Salesforce Setup → App Manager → your Connected App → **Consumer Secret** |
| `{{AEMBIT_CALLBACK_URL}}` | Aembit Management Console → **Credential Providers → New → OAuth 2.0 Authorization Code** → **Callback URL** field (auto-generated, read-only) |

## Service Configuration

### Salesforce Connected App Setup

1. In Salesforce Setup, click the gear icon and select **Setup**

2. In the Quick Find box, search for **App Manager** and select it

3. Click **New Connected App** (or **New External Client App** if using the newer UI)
   - **Connected App Name**: descriptive label (e.g., `Aembit Integration`)
   - **Contact Email**: your email address

4. Expand **API (Enable OAuth Settings)** and check **Enable OAuth Settings**
   - **Callback URL**: paste the `{{AEMBIT_CALLBACK_URL}}` value — retrieve this first by opening the Aembit Credential Provider dialog (see Aembit Configuration step 1 below)
   - **Selected OAuth Scopes**: add the scopes required by your use case (e.g., `Manage user data via APIs (api)`, `Perform requests at any time (refresh_token, offline_access)`)
   - Check **Require Proof Key for Code Exchange (PKCE)**
   - Check **Require secret for Web Server Flow**
   - Check **Require secret for Refresh Token Flow**

5. Click **Save** — Salesforce may take 2–10 minutes to activate the Connected App

6. Open the Connected App and copy the **Consumer Key** (`{{SALESFORCE_CLIENT_ID}}`) and **Consumer Secret** (`{{SALESFORCE_CLIENT_SECRET}}`)

## Aembit Configuration

1. Navigate to **Credential Providers** and click **+ New**
   - **Credential Type**: select **OAuth 2.0 Authorization Code**
   - Copy the **Callback URL** shown — you will need it when configuring the Salesforce Connected App in the Service Configuration section

2. Complete the Credential Provider fields:
   - **Name**: descriptive label (e.g., `Salesforce - {{SALESFORCE_INSTANCE_URL}}`)
   - **Client ID**: `{{SALESFORCE_CLIENT_ID}}`
   - **Client Secret**: `{{SALESFORCE_CLIENT_SECRET}}`
   - **OAuth URL**: `https://{{SALESFORCE_INSTANCE_URL}}/`
   - Click **URL Discovery** — Aembit auto-populates the Authorization URL and Token URL from the Salesforce discovery endpoint
   - **PKCE Required**: enable
   - **Lifetime**: 1 year
   - **Scopes**: leave empty to use the Connected App's defaults, or enter specific scopes

3. Click **Authorize** — a browser window opens to the Salesforce login page
   - Log in with the Salesforce user that will serve as the service account
   - Approve the OAuth consent screen
   - The Credential Provider status changes to **Ready** upon successful authorization

4. Navigate to **Server Workloads** and click **+ New**
   - **Host**: `{{SALESFORCE_INSTANCE_URL}}`
   - **Port**: 443
   - **Application Protocol**: HTTP
   - **Forward to Port**: 443 with **TLS** enabled
   - **Authentication Method**: HTTP Authentication
   - **Authentication Scheme**: Bearer
   - Click **Save**

## Verification

- The Credential Provider shows status **Ready** in the Aembit Management Console
- The application makes successful calls to the Salesforce REST API without any OAuth tokens in its environment or configuration
- Navigate to **Activity** and confirm log entries show credentials were injected for the Salesforce server workload
- In Salesforce Setup → Quick Find → **Login History**, confirm API logins appear for the authorized user originating from Aembit

## Troubleshooting

- **Credential Provider status stays Pending after authorization:** The Callback URL in the Salesforce Connected App does not match the Aembit-generated Callback URL. Edit the Connected App and paste the exact URL from the Aembit Credential Provider dialog
- **Authorization fails with OAUTH_EC_APP_NOT_FOUND:** The OAuth URL is using `login.salesforce.com` instead of the org's My Domain URL. Update the **OAuth URL** field to `https://{{SALESFORCE_INSTANCE_URL}}/` and re-run URL Discovery
- **PKCE error during authorization:** The Connected App does not have **Require Proof Key for Code Exchange (PKCE)** enabled. Enable it in Salesforce Setup and retry
- **Credential Provider becomes Inactive after initial authorization:** The refresh token expired or was revoked. Re-open the Credential Provider and click **Authorize** again to re-establish the token
- **Application receives 401 responses:** The Aembit Agent Proxy is not intercepting the outbound request. Verify the proxy is running and the Server Workload host matches the exact FQDN the application is calling
