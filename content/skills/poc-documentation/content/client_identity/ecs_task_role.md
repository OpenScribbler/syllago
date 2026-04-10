## Prerequisites

- ECS service running on Fargate or EC2 launch type
- IAM Task Role assigned to the ECS task definition

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AWS_ACCOUNT_ID}}` | AWS Console → top-right account menu, or: `aws sts get-caller-identity --query Account --output text` |
| `{{ECS_TASK_ROLE_NAME}}` | AWS Console → IAM → Roles → select the task role assigned to the ECS task definition → Role name (not the full ARN) |
| `{{ECS_TASK_FAMILY}}` | AWS Console → ECS → Task Definitions → select the task definition → Family name |

## Aembit Configuration

1. Navigate to **Trust Providers** and click **+ New**
   - **Trust Provider Type**: select **AWS Role**
   - Add match rules:
     - **accountId**: `{{AWS_ACCOUNT_ID}}`
     - **assumedRole**: `{{ECS_TASK_ROLE_NAME}}`
   - Click **Save**

2. Navigate to **Client Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `ECS - {{ECS_TASK_FAMILY}}`)
   - Add an **AWS ECS Task Family** identifier: `{{ECS_TASK_FAMILY}}`
   - Click **Save**
