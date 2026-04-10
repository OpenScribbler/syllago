## Prerequisites

- AWS account with permission to create IAM Identity Providers and IAM Roles
- Aembit Agent Proxy deployed on the client workload (Agent Controller or Helm deployment completed)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AWS_ACCOUNT_ID}}` | AWS Console → top-right account menu, or: `aws sts get-caller-identity --query Account --output text` |
| `{{IAM_ROLE_NAME}}` | Name for the new IAM role that Aembit will assume (you choose this) |

## Service Configuration

### AWS Setup

> **Start Here:** Before completing AWS setup, open the Aembit Credential Provider dialog first (Aembit Configuration step 1 below) to retrieve the OIDC Issuer URL and Audience values. Then return here.

1. In the AWS Console, navigate to **IAM → Identity providers → Add provider**
   - **Provider type**: OpenID Connect
   - **Provider URL**: `https://{{AEMBIT_TENANT_ID}}.id.useast2.aembit.io`
   - Click **Get thumbprint**
   - **Audience**: `sts.amazonaws.com`
   - Click **Add provider**

2. Navigate to **IAM → Roles → Create role**
   - **Trusted entity type**: Web identity
   - **Identity provider**: select the provider created in step 1
   - **Audience**: select `sts.amazonaws.com`
   - Attach a permissions policy scoped to least-privilege access for your target AWS services. Refer to [AWS IAM documentation](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html) for service-specific policy configuration.
   - Name the role `{{IAM_ROLE_NAME}}` and click **Create role**
   - Copy the **Role ARN** (format: `arn:aws:iam::{{AWS_ACCOUNT_ID}}:role/{{IAM_ROLE_NAME}}`)

## Aembit Configuration

1. In the Aembit Management Console, navigate to **Credential Providers** and click **+ New**
   - **Credential Type**: select **AWS Security Token Service Federation**
   - Copy the **OIDC Issuer URL** and **Aembit IdP Token Audience** values - you need these for the AWS setup above
   - **Name**: descriptive label (e.g., `AWS Access - {{IAM_ROLE_NAME}}`)
   - **AWS IAM Role Arn**: paste the Role ARN from AWS Setup step 2
   - **Lifetime (seconds)**: `3600` (1 hour) is a reasonable default; range is 900 to 129,600 seconds
   - Click **Save**

2. Navigate to **Server Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `AWS Services`)
   - **Host**: `*.amazonaws.com`
   - **Application Protocol**: HTTP
   - **Port**: 443
   - **Forward to Port**: 443, enable **TLS**
   - **Authentication Method**: HTTP Authentication
   - **Authentication Scheme**: AWS Signature v4
   - Click **Save**

> **Note:** The `*.amazonaws.com` wildcard covers all AWS services and regions with a single Server Workload. Aembit automatically selects SigV4 or SigV4a based on the hostname pattern.

## Verification

- The application accesses AWS services without any AWS credentials in its environment or configuration
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show credentials were injected for the AWS server workload
- In AWS CloudTrail, confirm API calls appear with the IAM role `{{IAM_ROLE_NAME}}` as the principal - not a static IAM user

## Troubleshooting

- **Access denied from AWS (403):** The IAM role trust policy does not match the Aembit OIDC issuer or audience. Verify the Identity Provider URL and Audience in IAM match the values from the Aembit Credential Provider exactly
- **Application receives no credentials:** The Access Policy is not active or the client workload identity does not match. Navigate to **Access Policies** and confirm the policy status and client workload selector
- **Thumbprint mismatch:** Re-fetch the thumbprint in the AWS Identity Provider settings if the Aembit OIDC certificate has rotated
- **Multi-account routing:** When using multiple Credential Providers in a single Access Policy, each must have a unique Access Key ID selector. The proxy routes requests to the correct provider based on the Access Key ID in the SigV4 authorization header
