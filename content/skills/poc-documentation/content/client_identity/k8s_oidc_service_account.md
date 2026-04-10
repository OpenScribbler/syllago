## Prerequisites

- Kubernetes cluster with OIDC issuer enabled
- Kubernetes OIDC Issuer URL: `{{K8S_OIDC_ISSUER}}`
- Service account subject to match: `{{SERVICE_ACCOUNT_SUBJECT}}`
  - Format: `system:serviceaccount:<namespace>:<service-account-name>`

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{K8S_OIDC_ISSUER}} | `kubectl get --raw /.well-known/openid-configuration \| jq -r .issuer`, or from your cluster provider's console (AKS: cluster overview → OIDC Issuer URL) |
| {{SERVICE_ACCOUNT_SUBJECT}} | `system:serviceaccount:<namespace>:<service-account-name>` - replace with the namespace and service account name of the workload |

## Aembit Configuration

1. Navigate to **Client Workloads** and create a new Client Workload
   - Type: **OIDC ID Token**
   - Subject: `{{SERVICE_ACCOUNT_SUBJECT}}`
     - This matches the `sub` claim of the Kubernetes service account token, which Kubernetes sets to `system:serviceaccount:<namespace>:<service-account-name>`

2. Navigate to **Trust Providers** and create a new **OIDC ID Token Trust Provider**
   - **Issuer (`iss`):** `{{K8S_OIDC_ISSUER}}`
   - **Audience (`aud`):** leave at the default value
   - **Subject (`sub`):** `{{SERVICE_ACCOUNT_SUBJECT}}`
