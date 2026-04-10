## Prerequisites

- Azure Entra ID App Registration already created
- Entra App Client ID: `{{ENTRA_APP_CLIENT_ID}}`
- Entra Tenant ID: `{{ENTRA_TENANT_ID}}`
- SHA-256 hash of the client workload binary, pre-calculated at build time (lowercase)
- Hostname of the client workload VM

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{ENTRA_TENANT_ID}}` | Azure portal → **App Registrations** → select your app → **Directory (tenant) ID** |
| `{{ENTRA_APP_CLIENT_ID}}` | Azure portal → **App Registrations** → select your app → **Application (client) ID** |
| `{{AEMBIT_TENANT_ID}}` | Aembit Management Console → **Administration → Tenant Information → Tenant ID** |
| `{{WORKLOAD_HOSTNAME}}` | Hostname of the client workload VM (run `hostname` on the VM) |
| `{{EXECUTABLE_HASH_SHA256}}` | SHA-256 hash of the client workload binary: `sha256sum <path-to-binary>` (use the lowercase output) |

## Service Configuration

### Add Federated Credential to Entra App

1. In the Azure portal, navigate to **App Registrations** → select your app → **Certificates & secrets** → **Federated credentials** → **Add credential**

2. Select **Other issuer** as the scenario

3. Configure the federated credential:
   - **Issuer:** `https://{{AEMBIT_TENANT_ID}}.id.useast2.aembit.io`
   - **Subject identifier:** `spiffe://{{AEMBIT_TENANT_ID}}.aembit.io/{{WORKLOAD_HOSTNAME}}/{{EXECUTABLE_HASH_SHA256}}`
   - **Audience:** `api://AzureADTokenExchange`
   - **Name:** descriptive name (e.g., `aembit-workload-{{WORKLOAD_HOSTNAME}}`)

4. Click **Add**

> **Note:** Each unique combination of workload hostname and binary hash requires its own federated credential. Add additional federated credentials for each workload VM or binary version.

## Aembit Configuration

1. Navigate to **Credential Providers** and click **+ New**
   - **Name:** descriptive name (e.g., `Entra ID JWT-SVID`)
   - **Credential Type:** **JWT-SVID Token**
   - **Subject:** `spiffe://{{AEMBIT_TENANT_ID}}.aembit.io/${os.environment.HOSTNAME}/${client.executable.hash.sha256}`
   - Set Subject to **Dynamic**
   - **Signing Algorithm Type:** **RSASSA-PKCS1-v1_5 using SHA-256**
   - **Audience:** `api://AzureADTokenExchange`
   - Click **Save**

2. Navigate to **Server Workloads** and click **+ New**
   - **Host:** `login.microsoftonline.com`
   - **App Protocol:** **OAuth**
   - **Port:** `443/TLS`
   - **Forward to Port:** `443`
   - **URL Path:** `/{{ENTRA_TENANT_ID}}/oauth2/v2.0/token`
   - **Authentication Method:** **OAuth Client Authentication**
   - **Authentication Scheme:** **POST Body Form URL Encoded**
   - Click **Save**

## Verification

- From the client workload VM, run a test request through the proxy to the Entra token endpoint:

```bash
curl --location --request POST \
  'https://login.microsoftonline.com/{{ENTRA_TENANT_ID}}/oauth2/v2.0/token' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'client_id={{ENTRA_APP_CLIENT_ID}}' \
  --data-urlencode 'grant_type=client_credentials' \
  --data-urlencode 'scope=https://graph.microsoft.com/.default' | jq
```

- Expected result: JSON response containing `access_token`, `token_type`, and `expires_in`
- Navigate to **Reporting** in the Aembit Management Console and confirm log entries show the workload authenticated and credentials were injected

## Troubleshooting

- **Entra returns `AADSTS70021: No matching federated identity record found`:** The subject in the JWT-SVID does not match the federated credential subject identifier in Entra. Verify that `{{WORKLOAD_HOSTNAME}}` matches the VM hostname exactly and that `{{EXECUTABLE_HASH_SHA256}}` matches the current binary hash (re-run `sha256sum` on the binary)
- **Entra returns `AADSTS700016: Application not found`:** The `{{ENTRA_APP_CLIENT_ID}}` is incorrect or the app registration does not exist in the `{{ENTRA_TENANT_ID}}` directory
- **Entra returns `AADSTS50027: JWT token is invalid or malformed`:** The issuer URL in the Entra federated credential does not match. Verify it is set to `https://{{AEMBIT_TENANT_ID}}.id.useast2.aembit.io` exactly
- **401 errors from the proxy:** The Aembit Access Policy is not matching. Check the Access Policy configuration and look for errors in the **Reporting** page
