## Prerequisites

- AWS Lambda function (container image deployment)
- AWS ECR repository for the container image
- Lambda Function ARN: `{{LAMBDA_FUNCTION_ARN}}`

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{LAMBDA_FUNCTION_ARN}} | AWS Console → Lambda → your function → Function ARN (top of the function overview page) |

## Aembit Configuration

1. Navigate to **Client Workloads** and create a new Client Workload
   - Add an **AWS Lambda ARN** identifier: `{{LAMBDA_FUNCTION_ARN}}`

2. Navigate to **Trust Providers** and create a new **AWS Lambda Trust Provider**
   - Match Rule: **Function ARN** with value `{{LAMBDA_FUNCTION_ARN}}`
