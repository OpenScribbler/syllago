# Aembit Terraform Provider Patterns

Provider-specific patterns for Aembit Workload IAM. Load when working with `source = "aembit/aembit"`.

---

## Core Concepts

Aembit manages **workload-to-workload access** (non-human identity):

- **Client Workload** -- requester app/service initiating access
- **Server Workload** -- target API/service being accessed
- **Trust Provider** -- verifies workload identity via platform attestation
- **Credential Provider** -- provides credentials to inject into requests
- **Access Policy** -- ties together: client + server + trust + credential
- **Access Condition** -- additional criteria (time-based, security tool compliance)
- **Agent Controller** -- manages Aembit Edge proxies

## Provider Configuration

### Production: OIDC (Recommended)

```hcl
provider "aembit" {
  client_id = "aembit:useast2:tenant:identity:github_idtoken:xxx"  # Or AEMBIT_CLIENT_ID env var
  resource_set_id = "xxx"  # Optional, or AEMBIT_RESOURCE_SET_ID env var
}
```

For Terraform Cloud: set `TFC_WORKLOAD_IDENTITY_AUDIENCE` to tenant endpoint.

### Development Only: Token Auth

- Rule: Set `AEMBIT_TENANT_ID` and `AEMBIT_TOKEN` via environment variables only. Never hardcode.
- Gotcha: Token expires with UI session -- will break randomly in CI. Use OIDC for CI/CD.

## Resource Patterns

### Client Workload

```hcl
resource "aembit_client_workload" "my_service" {
  name      = "my-backend-service"
  is_active = true
  identities { type = "k8sNamespace"; value = "production" }
  identities { type = "k8sServiceAccountName"; value = "my-service-sa" }
  tags = { environment = "production", team = "platform" }
}
```

**Identity Types**: `k8sNamespace`, `k8sServiceAccountName`, `awsAccountId`, `awsRoleArn`, `gcpIdentity`, `githubRepository`, `gitlabProject`.

### Server Workload

```hcl
resource "aembit_server_workload" "github_api" {
  name      = "github-api"
  is_active = true
  service_endpoint { host = "api.github.com"; port = 443; tls = true; tls_verification = "full" }
  authentication { method = "HTTP Bearer" }
}
```

### Trust Provider

```hcl
# Kubernetes
resource "aembit_trust_provider" "k8s" {
  name = "prod-eks"; is_active = true
  kubernetes { issuer = "https://oidc.eks..."; audiences = ["sts.amazonaws.com"] }
}

# GitHub Actions
resource "aembit_trust_provider" "github" {
  name = "github-ci"; is_active = true
  github_action { organization = "my-org"; repository = "my-repo" }
}

# AWS
resource "aembit_trust_provider" "aws" {
  name = "aws-prod"; is_active = true
  aws_role { account_id = "123456789012" }
}
```

### Credential Provider

Types: `api_key {}`, `oauth_client_credentials {}`, `aws_sts {}`, `aembit_access_token {}`.

```hcl
resource "aembit_credential_provider" "token" {
  name = "github-api-token"; is_active = true
  api_key { api_key = var.github_token }  # Never hardcode
}

resource "aembit_credential_provider" "oauth" {
  name = "service-oauth"; is_active = true
  oauth_client_credentials {
    token_url = "https://auth.example.com/oauth/token"
    client_id = var.oauth_client_id; client_secret = var.oauth_client_secret
    scopes = ["read:data", "write:data"]
  }
}
```

### Access Policy

```hcl
resource "aembit_access_policy" "service_to_api" {
  name               = "backend-to-github"
  is_active          = true
  client_workload_id = aembit_client_workload.my_service.id
  server_workload_id = aembit_server_workload.github_api.id
  trust_provider_ids = [aembit_trust_provider.k8s.id]
  credential_provider_id = aembit_credential_provider.token.id
  access_condition_ids   = [aembit_access_condition.hours.id]  # Optional
}
```

### Access Condition

Types: `time_condition {}`, `crowdstrike_condition {}`.

```hcl
resource "aembit_access_condition" "hours" {
  name = "business-hours"; is_active = true
  time_condition { timezone = "America/New_York"; days = ["monday","tuesday","wednesday","thursday","friday"]; time_from = "08:00"; time_to = "18:00" }
}
```

### Agent Controller

```hcl
resource "aembit_agent_controller" "prod" {
  name = "prod-controller"; is_active = true; tls_enabled = true
}
data "aembit_agent_controller_device_code" "prod" {
  agent_controller_id = aembit_agent_controller.prod.id
}
output "agent_device_code" { value = data.aembit_agent_controller_device_code.prod.device_code; sensitive = true }
```

## Security Gotchas

### Overly Broad Trust Provider
**Severity**: high

- Rule: Always scope trust providers to specific repo + branch, not just organization.
- Fix: Add `repository`, `ref_type`, `ref` constraints to `github_action` blocks.

### Hardcoded Secrets
**Severity**: critical

- Rule: Use `var.` references for all secrets. Never hardcode tokens or keys.

### Token Auth in CI/CD
**Severity**: high

- Rule: Use OIDC auth (`client_id`) for CI/CD. Token auth expires with UI session.

## Reusable Module Pattern

```hcl
# modules/aembit-workload-access/main.tf
variable "client_name" {}
variable "server_name" {}
variable "server_host" {}
variable "credential_provider_id" {}
variable "trust_provider_ids" { type = list(string) }

resource "aembit_client_workload" "this" { name = var.client_name; is_active = true }
resource "aembit_server_workload" "this" {
  name = var.server_name; is_active = true
  service_endpoint { host = var.server_host; port = 443; tls = true }
}
resource "aembit_access_policy" "this" {
  name = "${var.client_name}-to-${var.server_name}"; is_active = true
  client_workload_id = aembit_client_workload.this.id
  server_workload_id = aembit_server_workload.this.id
  trust_provider_ids = var.trust_provider_ids
  credential_provider_id = var.credential_provider_id
}
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `AEMBIT_CLIENT_ID` | Trust Provider Client ID (OIDC auth) |
| `AEMBIT_TENANT_ID` | Tenant ID (token auth, dev only) |
| `AEMBIT_TOKEN` | API token (dev only) |
| `AEMBIT_RESOURCE_SET_ID` | Target Resource Set |
| `TFC_WORKLOAD_IDENTITY_AUDIENCE` | Terraform Cloud OIDC audience |

## Security Checklist

- [ ] No hardcoded secrets in `.tf` files
- [ ] Credentials use `var.` references
- [ ] Trust providers scoped (not overly broad)
- [ ] TLS verification enabled for server workloads
- [ ] OIDC auth used for CI/CD (not token auth)
- [ ] Provider version pinned (`~> 1.0`)
- [ ] Sensitive outputs marked `sensitive = true`
