## Prerequisites

- HashiCorp Vault cluster (self-hosted or HCP) with admin access
- Vault address: `{{VAULT_HOST}}:{{VAULT_PORT}}`
- Aembit Agent Proxy deployed on the client workload (Agent Controller or deployment module completed)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{VAULT_HOST}}` | Vault cluster hostname (e.g., `vault-cluster.hashicorp.cloud`) |
| `{{VAULT_PORT}}` | Vault port (typically `8200`) |
| `{{VAULT_NAMESPACE}}` | Vault UI → top-left namespace selector (HCP and enterprise only, omit if not applicable) |
| `{{VAULT_ROLE_NAME}}` | Name for the OIDC role you will create in Vault (you choose this) |
| `{{VAULT_AUDIENCE}}` | Audience value for the Vault role's `bound_audiences` (you choose this, e.g., `aembit-vault`) |

## Service Configuration

### Vault Setup

> **Start Here:** Before completing Vault setup, open the Aembit Credential Provider dialog first (Aembit Configuration step 1 below) to retrieve the OIDC Issuer URL. Then return here.

1. In the Vault UI or CLI, enable the OIDC auth method:

```
vault auth enable oidc
```

2. Configure the OIDC auth method with the Aembit Issuer URL:

```
vault write auth/oidc/config \
  oidc_discovery_url="<OIDC Issuer URL from Aembit>" \
  default_role="{{VAULT_ROLE_NAME}}"
```

3. Create a role with `bound_audiences` matching your chosen audience value:

```
vault write auth/oidc/role/{{VAULT_ROLE_NAME}} \
  bound_audiences="{{VAULT_AUDIENCE}}" \
  user_claim="sub" \
  token_policies="<your-policy>" \
  token_ttl="1h"
```

> **Note:** Replace `<your-policy>` with the Vault policy that grants the access your workload needs. Refer to [Vault policy documentation](https://developer.hashicorp.com/vault/docs/concepts/policies) for policy configuration.

## Aembit Configuration

1. Navigate to **Credential Providers** and click **+ New**
   - **Credential Type**: select **Vault Client Token**
   - Copy the **OIDC Issuer URL** - you need this for the Vault setup above
   - **Name**: descriptive label (e.g., `Vault - {{VAULT_ROLE_NAME}}`)
   - **Subject**: a Vault-compatible value (e.g., `aembit`)
   - Click **+ New Claim** and add a custom claim:
     - **Name**: `aud`
     - **Value**: `{{VAULT_AUDIENCE}}`
   - **Host**: `{{VAULT_HOST}}`
   - **Port**: `{{VAULT_PORT}}`, enable **TLS**
   - **Authentication Path**: `oidc`
   - **Role**: `{{VAULT_ROLE_NAME}}`
   - **Namespace**: `{{VAULT_NAMESPACE}}` (leave blank if not using namespaces)
   - **Forwarding**: No Forwarding
   - Click **Save**

2. Navigate to **Server Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `Vault`)
   - **Host**: `{{VAULT_HOST}}`
   - **Application Protocol**: HTTP
   - **Port**: `{{VAULT_PORT}}`
   - **Forward to Port**: `{{VAULT_PORT}}`, enable **TLS**
   - **Authentication Method**: HTTP Authentication
   - **Authentication Scheme**: Header
   - **Header Name**: `X-Vault-Token`
   - Enable **TLS Decrypt**
   - Click **Save**

## Verification

- The application reads secrets from Vault without any Vault token in its environment or configuration
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show credentials were injected for the Vault server workload
- In Vault, confirm audit log entries show API calls authenticated via the OIDC auth method with role `{{VAULT_ROLE_NAME}}`

## Troubleshooting

- **Vault returns 403 Permission Denied:** The Vault role's token policy does not grant access to the requested path. Verify the policy attached to `{{VAULT_ROLE_NAME}}` includes the secrets paths your workload needs
- **OIDC authentication fails in Vault:** The OIDC Issuer URL in Vault does not match the Aembit Credential Provider. Copy the Issuer URL directly from the Credential Provider dialog and re-run `vault write auth/oidc/config`
- **Audience mismatch error:** The `aud` custom claim in the Aembit Credential Provider does not match `bound_audiences` in the Vault role. Verify both values are identical
- **Application receives no credentials:** The Access Policy is not active or the client workload identity does not match. Navigate to **Access Policies** and confirm the policy status and client workload selector
- **TLS errors connecting to Vault:** TLS Decrypt is not enabled on the Server Workload. Enable **TLS Decrypt** in the Server Workload configuration - this is required for Vault integration
