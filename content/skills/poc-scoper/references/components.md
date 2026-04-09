# POC Component Library

Reference for mapping customer workloads to YAML recipe components.

## Component Library

### client_identity

| Component Path | Use When | Business Value Hint |
|----------------|----------|---------------------|
| `client_identity/ec2_iam_role` | Source workload runs on EC2 with an IAM instance role | Eliminating static API credentials stored on EC2 — credential sprawl, manual rotation burden, and risk of server compromise exposing long-lived secrets. |
| `client_identity/lambda_iam_role` | Source workload is an AWS Lambda function | Eliminating secrets from Lambda environment variables — serverless functions access downstream APIs without any stored credentials or rotation burden. |
| `client_identity/claude_okta_oidc` | Source workload is Claude AI agent (Desktop or Web Enterprise) authenticating via Okta OIDC | Enabling AI agents to access enterprise systems with a verifiable workload identity — the agent never handles or stores credentials, and every access is auditable. |
| `client_identity/k8s_oidc_service_account` | Source workload is a Kubernetes pod identified by its OIDC service account token (any K8s environment — AKS, EKS, GKE, on-prem) | Microservices prove their identity using the cryptographic OIDC token Kubernetes already issues — no secrets injected into pods, no credentials to rotate, and every workload is identified by namespace and service account. |
| `client_identity/vmware_cert_signed_attestation` | Source workload runs on a VMware vSphere VM with Network Identity Attestor deployed | On-prem VMs get cryptographically verifiable workload identity tied to vCenter metadata — no static credentials, and process hash attestation enables zero-trust binary verification. |
| `client_identity/github_actions_oidc` | Source workload is a GitHub Actions workflow | CI/CD pipelines access downstream services without stored secrets — GitHub's native OIDC token proves the workflow's identity and Aembit delivers just-in-time credentials scoped to each job run. |
| `client_identity/gitlab_cicd_oidc` | Source workload is a GitLab CI/CD pipeline job (GitLab Cloud only) | CI/CD pipelines access downstream services without stored secrets — GitLab's native OIDC token proves the job's identity and Aembit delivers just-in-time credentials scoped to each pipeline run. |
| `client_identity/ecs_task_role` | Source workload is an AWS ECS task (Fargate or EC2 launch type) with an IAM Task Role | ECS services access downstream resources without static credentials — the task role proves the workload's identity and Aembit delivers short-lived credentials scoped to each task. |

### server_integration

| Component Path | Use When | Business Value Hint |
|----------------|----------|---------------------|
| `server_integration/box_api_oauth` | Target is Box API with OAuth 2.0 credentials | Developers and services access Box without ever seeing the OAuth client secret — Aembit injects credentials transparently at runtime. |
| `server_integration/mcp_server` | Target is any MCP server accessed via Aembit MCP Identity Gateway (Box, Microsoft 365, FactSet, etc.) | Users access enterprise MCP servers with cryptographically attested identity — no OAuth credentials are shared with or stored by the AI agent, and every access is auditable per user and server. |
| `server_integration/snowflake_jwt` | Target is Snowflake with JWT authentication | Data pipelines and services access Snowflake without static passwords — JWT-based authentication eliminates credential sprawl across ETL jobs and analysts. |
| `server_integration/postgresql_password` | Target is PostgreSQL with password credentials | Applications connect to PostgreSQL without storing database passwords — Aembit injects credentials at runtime and rotates them automatically. |
| `server_integration/jwt_svid_bearer` | Target is a custom HTTPS API or API gateway that validates bearer tokens as SPIFFE JWT-SVIDs | Microservices authenticate to internal APIs with cryptographically verifiable workload identities — no static API keys or shared secrets, and every service-to-service call is auditable by namespace and service account. |
| `server_integration/aws_sts_federation` | Target is any AWS service (S3, SQS, DynamoDB, Lambda, etc.) using short-lived STS credentials | Services access AWS resources without storing long-lived IAM access keys — Aembit obtains short-lived STS tokens via OIDC federation and injects them at runtime. |
| `server_integration/salesforce_oauth_3lo` | Target is Salesforce API using OAuth 3LO (External Client App / AppExchange install) | Applications access Salesforce without embedding Connected App secrets — Aembit manages the OAuth token lifecycle and injects credentials transparently. |
| `server_integration/entra_id_jwt_svid` | Target is Azure Entra ID using JWT-SVID with Workload Identity Federation and process hash attestation | On-prem workloads authenticate to Entra ID with cryptographically signed SPIFFE identities that include binary hash verification — no client secrets stored on VMs, and only approved binaries can obtain tokens. |
| `server_integration/aembit_api` | Target is the Aembit API itself (e.g., CI/CD pipelines or automation tools managing Aembit resources) | Automation tools access the Aembit API without static API keys — Aembit issues short-lived access tokens scoped to the minimum role required. |
| `server_integration/hashicorp_vault` | Target is HashiCorp Vault (self-hosted or HCP) using OIDC authentication | Applications access Vault secrets without static tokens — Aembit authenticates via OIDC and injects short-lived Vault client tokens at runtime. |

### client_deployment

| Component Path | Use When |
|----------------|----------|
| `client_deployment/ec2_proxy` | Aembit Agent Proxy running alongside workload on EC2 |
| `client_deployment/lambda_extension` | Aembit Agent running as a Lambda layer extension |
| `client_deployment/mcp_client` | Claude Desktop or Claude Web Enterprise as MCP client |
| `client_deployment/k8s_helm` | Aembit Agent Proxy injected as sidecar via Helm chart on standard Kubernetes (AKS, GKE, EKS non-Fargate, on-prem) |
| `client_deployment/openshift_helm` | Aembit Agent Proxy injected as sidecar via Helm chart on OpenShift (requires anyuid SCC and explicit steering annotation) |
| `client_deployment/eks_fargate_helm` | Aembit Agent Proxy injected as sidecar via Helm chart on EKS Fargate (requires Fargate profile for aembit namespace, explicit steering) |
| `client_deployment/agent_cli_sidecar` | Aembit CLI running as a K8s sidecar container, writing credentials to a shared in-memory volume (use when the Agent Proxy conflicts with existing service mesh sidecars, e.g. Istio) |
| `client_deployment/vmware_proxy` | Aembit Agent Proxy on a VMware vSphere Linux VM with Network Identity Attestor for attestation and process hash identification |
| `client_deployment/github_actions` | Aembit GitHub Action retrieves credentials in a GitHub Actions workflow via OIDC token exchange |
| `client_deployment/gitlab_cicd` | Aembit GitLab CI/CD Component retrieves credentials in a GitLab pipeline via OIDC token exchange |
| `client_deployment/ecs_sidecar` | Aembit Agent Proxy deployed as ECS sidecar container via Terraform module (Fargate or EC2 launch type) |

### access_conditions

Include access condition modules when the customer wants conditional access beyond workload identity — e.g., device health, location, or time restrictions.

| Component Path | Use When |
|----------------|----------|
| `access_conditions/crowdstrike_posture` | Customer requires CrowdStrike Falcon agent health check before granting access (device posture enforcement) |
| `access_conditions/geo_ip` | Customer wants to restrict access by geographic region or IP range (note: cloud workload IPs may not resolve to expected regions) |
| `access_conditions/time_window` | Customer wants to restrict access to specific hours or days of the week |

### infrastructure

Include infrastructure modules based on what the customer needs beyond the core access policy chain.

| Component Path | Include When |
|----------------|-------------|
| `infrastructure/agent_controller` | The deployment uses the Aembit Agent Proxy or CLI on **non-K8s infrastructure** (e.g., EC2). **Omit for API-model deployments** and **omit for K8s Helm-based deployments** (k8s_helm, openshift_helm, eks_fargate_helm) — those modules create the Agent Controller in their own Part 1. |
| `infrastructure/log_stream_s3` | Customer wants to export Aembit audit logs to an S3 bucket. Note: each log type (Access Events, Audit Events, etc.) requires a separate Log Stream entry. |
| `infrastructure/log_stream_splunk` | Customer wants to export Aembit audit logs to Splunk via HTTP Event Collector (HEC). Note: each log type requires a separate Log Stream entry. |
| `infrastructure/okta_saml` | Customer SSO uses Okta with SAML 2.0 assertion (most common for enterprise Okta) |
| `infrastructure/okta_oidc` | Customer SSO uses Okta with OIDC (use when customer specifically requests OIDC or has an existing OIDC app in Okta) |
| `infrastructure/crowdstrike_integration` | Customer uses CrowdStrike Falcon and wants device posture-based access conditions. Auto-injected when `access_conditions/crowdstrike_posture` is used. |
| `infrastructure/signon_policies` | Customer wants to enforce MFA or session duration policies on Aembit Console access |
| `infrastructure/vmware_network_identity_attestor` | Client workloads run on VMware vSphere VMs and need Network Identity Attestor for workload attestation. Required when using `client_identity/vmware_cert_signed_attestation`. |
| `infrastructure/vmware_agent_controller` | Agent Controller on VMware/on-prem Linux VM using Device Code authentication and customer-managed TLS. Use instead of `infrastructure/agent_controller` when there is no cloud metadata service available. |

---

## Policy Chain Rules

| client_identity | server_integration | policy_chain | policy_chain_labels |
|---|---|---|---|
| `claude_okta_oidc` | `mcp_server` | `dual` | `"Policy 1: Claude → MCP Gateway"`, `"Policy 2: {{MCP_USER_EMAIL}} → {{MCP_SERVER_NAME}}"` |
| Any other combination | Any | `single` | (omit policy_chain_labels) |

---

## YAML Recipe Schemas

### POC Guide Recipe (`<customer>_poc_guide.yaml`)

```yaml
output: <CUSTOMER>_POC_Guide.pdf
introduction: false

vars:
  DOCUMENT_TITLE: Proof of Concept Guide
  CUSTOMER_NAME: <customer name>
  SA_NAME: <SA name>
  SA_EMAIL: <SA email>
  AE_NAME: <AE name>
  AE_EMAIL: <AE email>
  POC_START_DATE: <date>
  TIMELINE_CLOSEOUT_DATE: "{{TIMELINE_CLOSEOUT_DATE}}"  # fill if known
  # CUSTOMER_LOGO: /path/to/logo.png  # optional - omit entirely if no logo available

  # Executive summary — write a 2-3 sentence description of what the customer is evaluating and why.
  EXEC_SUMMARY_INTRO: "<2-3 sentence description of customer evaluation objective>"

# Customer contacts — any number supported.
contacts:
  - name: <name>
    role: <role>
    email: <email>

# Use cases listed in the executive summary — one string per use case.
exec_summary_use_cases:
  - "<Use Case Name>"

# Business goals — use customer's own words where possible. Any number of items.
business_goals:
  - "<goal>"
  - "<goal>"
  - "<goal>"

# Success criteria — structured rows rendered as a table (No | Test Case | Success Criterion | Mandatory).
# mandatory: true renders as "Yes", mandatory: false renders as "No".
success_criteria:
  - no: 1
    test_case: "<test case>"
    criterion: "<criterion>"
    mandatory: true

sections:
  - poc_guide/executive_summary
  - poc_guide/contacts
  - poc_guide/business_goals
  - poc_guide/success_criteria
  - poc_guide/timeline
```

### Implementation Guide Recipe (`<customer>_impl_guide.yaml`)

```yaml
output: <CUSTOMER>_Implementation_Guide.pdf

vars:
  DOCUMENT_TITLE: Implementation Guide
  CUSTOMER_NAME: <customer name>
  SA_NAME: <SA name>
  SA_EMAIL: <SA email>
  AE_NAME: <AE name>
  AE_EMAIL: <AE email>
  POC_START_DATE: <date>
  # CUSTOMER_LOGO: /path/to/logo.png  # optional - omit entirely if no logo available
  # Per-use-case value vars (one per use case, named descriptively — NOT positional):
  # USE_CASE_MCP_VALUE: "<business value statement>"       # e.g. Claude → Box MCP
  # USE_CASE_EC2_BOX_VALUE: "<business value statement>"  # e.g. EC2 → Box API
  # USE_CASE_LAMBDA_SF_VALUE: "<business value statement>" # e.g. Lambda → Snowflake

# CONDITIONAL — include this block only if client_deployment uses Agent Proxy or CLI.
# DELETE ENTIRE infrastructure: block for API-model deployments (workload calls Aembit REST API directly).
infrastructure:
  - infrastructure/agent_controller

use_cases:
  - name: "<use case name>"
    overview: "<technical description of what is configured and how it works>"
    business_value: "{{USE_CASE_<DESCRIPTOR>_VALUE}}"   # use descriptive name, not positional
    # business_value_hint: only include when business_value is a {{PLACEHOLDER}}.
    # Use the Business Value Hint from the component library for the matched client_identity + server_integration.
    # Omit this field when business_value is resolved prose (hint text should not appear in the PDF).
    business_value_hint: "<hint text from component library>"
    client_identity: <component path>
    server_integration: <component path>
    client_deployment: <component path>
    policy_chain: single | dual
    # Only include policy_chain_labels for dual:
    policy_chain_labels:
      - "Policy 1: <label>"
      - "Policy 2: <label>"
    verification:
      - "<step to verify the use case works>"
    success_criteria:
      - "<observable outcome that confirms success>"
    troubleshooting:
      - "**<symptom>:** <diagnosis and fix>"
```

---

## Content Module Reference

See `~/.claude/skills/poc-documentation/content/` for the component content modules and `~/.claude/skills/poc-documentation/assembler.py` for the assembler. Content modules are the authoritative source for verification steps, troubleshooting items, and prerequisites — read them when populating use case fields.

---

## Customer Slug Rules

| Customer Name | Slug |
|---------------|------|
| All lowercase | yes |
| Spaces → hyphens | yes |
| Special chars removed | yes |
| Example: "Acme Corp" | `acme-corp` |
| Example: "ACME" | `acme` |
