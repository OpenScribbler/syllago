## Prerequisites

- EC2 instance with application workload running
- IAM Role attached to the instance

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AWS_ACCOUNT_ID}}` | AWS Console → top-right account menu, or: `aws sts get-caller-identity --query Account --output text` |
| `{{EC2_IAM_ROLE_NAME}}` | AWS Console → IAM → Roles → select the role attached to the EC2 instance → Role name (not the full ARN) |
| `{{EC2_HOSTNAME}}` | AWS Console → EC2 → Instances → select instance → Public or Private DNS name |

## Aembit Configuration

1. Navigate to **Trust Providers** and click **+ New**
   - **Trust Provider Type**: select **AWS Role**
   - Add match rules:
     - **accountId**: `{{AWS_ACCOUNT_ID}}`
     - **assumedRole**: `{{EC2_IAM_ROLE_NAME}}`
   - Click **Save**

2. Navigate to **Client Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `EC2 - {{EC2_HOSTNAME}}`)
   - Add a **Hostname** identifier: `{{EC2_HOSTNAME}}`
   - Click **Save**
