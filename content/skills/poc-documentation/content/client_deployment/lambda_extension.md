## Prerequisites

- Lambda function deployed as a **container image** (zip deployments are not supported)
- ECR repository for publishing the updated image
- Agent Controller deployed and accessible (complete the Agent Controller module first)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{LAMBDA_IMAGE_NAME}} | Your local image tag (e.g., `my-function:latest`) |
| {{ECR_REPOSITORY_URI}} | AWS Console → ECR → your repository → URI |
| {{AWS_REGION}} | AWS region where the ECR repository resides (same region as the Lambda function) |
| {{AGENT_CONTROLLER_FQDN}} | Agent Controller load balancer DNS name (from the Agent Controller module) — used in the full URL format `http://{{AGENT_CONTROLLER_FQDN}}:5000` |

## Deployment

1. Add the Aembit Lambda Extension to your Lambda container Dockerfile. Place this line before your application layer so the extension is available at runtime:

```dockerfile
COPY --from=public.ecr.aws/aembit/lambda-extension:latest /opt/extensions/aembit-lambda-extension /opt/extensions/aembit-lambda-extension
```

2. Build and push the updated container image:

```bash
docker build -t {{LAMBDA_IMAGE_NAME}} .
docker tag {{LAMBDA_IMAGE_NAME}} {{ECR_REPOSITORY_URI}}:latest
aws ecr get-login-password --region {{AWS_REGION}} | docker login --username AWS --password-stdin {{ECR_REPOSITORY_URI}}
docker push {{ECR_REPOSITORY_URI}}:latest
```

3. Update the Lambda function configuration with required environment variables:
   - **AEMBIT_AGENT_CONTROLLER_URL**: `http://{{AGENT_CONTROLLER_FQDN}}:5000`
   - **AEMBIT_TENANT_ID**: `{{AEMBIT_TENANT_ID}}`

4. Update the Lambda function to use the new image. In the Lambda console, navigate to the **Image** section and click **Deploy new image**, selecting the updated ECR URI. Or run:

```bash
aws lambda update-function-code --function-name <your-function-name> --image-uri {{ECR_REPOSITORY_URI}}:latest
```

5. Invoke the function to verify the extension loads. In the Lambda console, click **Test** and run a test event, or run:

```bash
aws lambda invoke --function-name <your-function-name> /tmp/response.json
```

## Verification

- Lambda function invocation succeeds (exit code 200, no authentication errors in response)
- In CloudWatch Logs, confirm the extension initialization message appears before the function handler runs
- Navigate to **Activity** in the Aembit Management Console and confirm a log entry shows the Lambda workload authenticated successfully

## Troubleshooting

- **Extension not loading:** Confirm the `COPY --from` line appears in the Dockerfile before the application layer. Verify the `/opt/extensions/aembit-lambda-extension` binary is executable in the built image
- **Controller unreachable:** Verify `AEMBIT_AGENT_CONTROLLER_URL` is set to `http://{{AGENT_CONTROLLER_FQDN}}:5000` (including the `http://` prefix and port 5000) and the Lambda function's VPC security group allows outbound traffic to the controller
- **Tenant ID mismatch:** Verify `AEMBIT_TENANT_ID` matches the value in **Settings → Tenant** in the Aembit Management Console
