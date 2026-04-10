# Aembit Concepts

Core concepts and architecture for Aembit Workload IAM platform.

## What is Aembit?

Aembit is a cloud-native Workload Identity and Access Management (IAM) platform that manages access between non-human identities (workloads) by verifying workload identity and enforcing dynamic policies. It enables "secretless" access — workloads never store long-lived credentials.

## Core Principles

1. **Manage Access, Not Secrets** — Grant access based on verified workload identity and policy, not distributed credentials
2. **Zero Trust Architecture** — Every access request requires verification regardless of source
3. **Least Privilege** — Grant only necessary permissions for specific tasks at specific times

## Architecture

Aembit uses a three-plane architecture:

```
+-----------------------+     +---------------------------+
|    Aembit Cloud       |     |    Your Environment       |
|  (Control + Mgmt)     |     |                           |
|                       |     |  +---------------------+  |
|  - Policy evaluation  |<--->|  | Agent Controller    |  |
|  - Identity verify    |     |  |   + Agent Proxy(s)  |  |
|  - Credential broker  |     |  |   + Agent Injector  |  |
|  - Admin UI/API       |     |  +---------------------+  |
+-----------------------+     |       |                    |
                              |       v                    |
                              |  +-----------+             |
                              |  | Workloads |             |
                              |  +-----------+             |
                              +---------------------------+
```

- **Management plane** (Aembit Cloud) — Configuration, administration, auditing, monitoring
- **Control plane** (Aembit Cloud) — Real-time policy evaluation, identity verification, credential brokering
- **Data plane** (Aembit Edge) — Request interception, credential injection, local enforcement

This separation enables **static stability** — Edge components continue operating with cached credentials during temporary Cloud outages.

## Aembit Edge Components

| Component | Role | Deployment |
|-----------|------|------------|
| **Agent Controller** | Registers with Aembit Cloud, manages Agent Proxies | One per environment |
| **Agent Proxy** | Intercepts traffic, collects identity evidence, injects credentials | Alongside each workload |
| **Agent Injector** | Kubernetes-specific: automatically injects Agent Proxy as sidecar | K8s only, via Helm |

**Registration flow:**
1. Agent Controller registers with Aembit Cloud (Device Code flow or Trust Provider)
2. Agent Proxy registers with Agent Controller via `/api/token`
3. Aembit Cloud grants Agent Proxy a token
4. Agent Proxy uses token to receive policies and interact with Cloud

## Core Components

### Access Policies

Central control mechanism linking all components:

| Component | Purpose |
|-----------|---------|
| Client Workload | Who is requesting access |
| Trust Provider | How identity is verified |
| Access Conditions | What context is required (optional) |
| Credential Provider | How credentials are obtained |
| Server Workload | What is being accessed |

**Note:** Multiple credential providers per policy is only supported for Snowflake JWT (`jwt-token` type configured for Snowflake) and AWS STS Federation credential types.

### Client Workloads

Applications, scripts, and automated processes that initiate access requests.

**Identification methods:**

| Platform | Identifiers |
|----------|-------------|
| AWS | EC2 Instance ID, ECS Task Family, Lambda ARN, IAM Role ARN, Account ID, Region |
| Azure | Subscription ID, VM ID |
| GCP | Identity Token claims |
| Kubernetes | Pod Name, Pod Name Prefix, Service Account Name, Namespace |
| GitHub Actions | Repository, Subject claims from OIDC tokens |
| GitLab CI | Namespace Path, Project Path, Ref Path, Subject claims |
| Terraform Cloud | Organization ID, Project ID, Workspace ID from OIDC tokens |
| Generic | Hostname, process name, process user, source IP, Aembit Client ID |

Multiple identifiers can be configured per Client Workload for increased specificity.

### Server Workloads

Target services receiving requests: REST APIs, databases, cloud services, SaaS applications, secret managers.

### Trust Providers

Verify workload identity using evidence from the runtime environment (workload attestation), not pre-shared secrets.

| Type | Verifies |
|------|----------|
| AWS | EC2 instance identity, Lambda, ECS, STS caller identity |
| Azure | Managed identity, VM identity |
| GCP | Service account, instance identity |
| Kubernetes | Service account JWT |
| GitHub Actions | OIDC token with repo/workflow claims |
| GitLab CI | OIDC token with project/pipeline claims |
| Kerberos | Domain-joined machine identity (via Agent Controller) |

### Access Conditions

Dynamic constraints evaluated at access time. Aembit caches condition data to avoid latency.

| Type | Examples |
|------|----------|
| Time-based | Business hours, maintenance windows, follow-the-sun |
| Geographic (GeoIP) | Country/subdivision restrictions, data sovereignty |
| Security posture | Wiz (cloud posture), CrowdStrike (endpoint health/attributes) |

### Credential Providers

Obtain authentication credentials after access is authorized. Credentials are ephemeral and just-in-time.

| Type | API Name | Use Case |
|------|----------|----------|
| Aembit Access Token | `aembit-access-token` | Aembit-native access tokens |
| API Key | `api-key` | Static API key injection |
| AWS STS Federation | `aws-sts` | AWS temporary credentials via STS |
| AWS Secrets Manager | — | Retrieve secrets from AWS Secrets Manager |
| Azure Entra Federation | `azure-entra-federation` | Azure resource access via WIF |
| Azure Key Vault | — | Retrieve secrets from Azure Key Vault |
| GCP WIF | `google-workflow-id` | GCP resource access via WIF |
| GitLab Managed Account | `gitlab-managed-account` | GitLab credential lifecycle (create, rotate, delete) |
| JWT Token | `jwt-token` | JWT-based authentication |
| OAuth2 Authorization Code | `oauth2-authcode` | OAuth2 auth code flow |
| OAuth2 Client Credentials | `oauth-client-credential` | OAuth2 client credentials flow |
| Username/Password | `username-password` | Basic authentication |
| Vault Client Token | `vault-client-token` | HashiCorp Vault access |

## Access Authorization Flow

1. Client Workload initiates request — intercepted by Agent Proxy
2. Agent Proxy collects identity evidence from runtime environment
3. Agent Proxy sends evidence to Aembit Cloud (evidence is cached for resilience)
4. Aembit Cloud matches request to Access Policy
5. Trust Provider verifies identity via attestation (result cached)
6. Access Conditions evaluated — time, geo, security posture (data cached)
7. Credential Provider obtains just-in-time credentials
8. Credentials returned to Agent Proxy and injected into request
9. Modified request forwarded to Server Workload

## Deployment Environments

| Environment | Method |
|-------------|--------|
| Kubernetes | Helm chart — Agent Controller + Agent Injector; Proxy auto-injected as sidecar |
| Amazon ECS/Fargate | ECS tasks/services via Terraform modules |
| Linux VMs | Installer packages (Ubuntu 20.04/22.04 LTS, RHEL 8/9 with SELinux) |
| Windows VMs | MSI packages (Windows Server 2019/2022) |
| GitHub Actions | Agent Proxy as GitHub Action |
| GitLab CI/CD | Agent Proxy as GitLab Runner |
| Jenkins Pipelines | Agent Proxy as Jenkins Pipeline step |
| AWS Lambda (containers) | Agent Proxy as Lambda Extension layer |
| AWS Lambda (functions) | Agent Proxy as Lambda layer |
| Virtual appliance | Pre-packaged `.ova` (bundles Controller + Proxy) |

High availability supported via multiple Agent Controller instances with load balancing.

## Administration

### Tenants
Each deployment is an isolated tenant at `{tenant}.aembit.io` with separate users, resources, and security boundaries.

### Resource Sets
Logical groupings for multi-tenancy: separate environments (dev/staging/prod), teams, or business units. Delegate administration per resource set.

### RBAC
Predefined and custom roles with granular permissions for policy management, workload configuration, credential provider management, and audit access.

### Workload Discovery
Automated workload identification via security tool integrations (Wiz).

### Global Policy Compliance
Organization-wide security standards for Access Policies and Agent Controllers. Prevents exposure-creating policies. Includes compliance reporting dashboard.

## Observability

### Event Tiers

| Tier | Content |
|------|---------|
| Audit Logs | Administrative changes (who changed what) |
| Workload Events | High-level workload interactions with severity |
| Access Authorization Events | Granular step-by-step policy evaluation details |

### Log Export (Log Streams)

| Destination | Type |
|-------------|------|
| AWS S3 | `AwsS3Bucket` |
| Google Cloud Storage | `GcsBucket` |
| Splunk | `SplunkHttpEventCollector` |
| CrowdStrike | `CrowdstrikeHttpEventCollector` |

### Credential Provider Integrations

| Type | Use Case |
|------|----------|
| GitLab | Credential lifecycle management (create, rotate, delete) |
| AWS IAM Role | AWS IAM role credential integration |

## Advanced Edge Features

These features are configurable through administration. Check live docs for current details:

- **TLS Decrypt** — Agent Proxy can decrypt TLS traffic for inspection. Uses Standalone Certificate Authorities for PKI.
- **Proxy Steering** — Methods for routing workload traffic through the Agent Proxy.
- **PKI-based TLS** — TLS certificate management for Agent Proxy communications.

## Infrastructure as Code

The Aembit Terraform Provider enables version-controlled, GitOps-compatible configuration of all Aembit resources. For Terraform patterns, load `skills/terraform-patterns/references/providers/aembit.md`.
