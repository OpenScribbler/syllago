## Prerequisites

- ECS cluster (Fargate or EC2 launch type)
- Terraform configured with valid AWS credentials
- Aembit Agent Controller ID (created during deployment below)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AEMBIT_TENANT_ID}}` | Aembit Management Console → **Settings** → Tenant ID |
| `{{AEMBIT_AGENT_CONTROLLER_ID}}` | Aembit Management Console → **Edge Components → Agent Controllers** → select controller → ID |
| `{{ECS_CLUSTER_NAME}}` | AWS Console → ECS → Clusters → cluster name |

## Deployment

The Aembit ECS Terraform module deploys the Agent Controller as a standalone ECS service and provides an Agent Proxy sidecar container for your workload task definitions.

1. In the Aembit Management Console, navigate to **Edge Components → Agent Controllers** and click **+ New**
   - **Name**: descriptive label (e.g., `ECS - {{ECS_CLUSTER_NAME}}`)
   - **Trust Provider**: select the AWS Role Trust Provider created in the client identity module
   - Click **Save**
   - Copy the **Agent Controller ID** (`{{AEMBIT_AGENT_CONTROLLER_ID}}`)

2. Add the Aembit ECS Terraform module to your infrastructure code. The module requires your Aembit Tenant ID, the Agent Controller ID, and your ECS cluster details (cluster name, VPC ID, subnets, security groups). Refer to the [Aembit ECS Terraform module documentation](https://docs.aembit.io/user-guide/deploy-install/serverless/aws-ecs-fargate/) for the complete module configuration and all available variables.

3. Integrate the Agent Proxy sidecar into your workload task definition. The module outputs a ready-to-use proxy container definition (`agent_proxy_container`) that should be added as the first container in your task definition.

4. Add explicit proxy environment variables to your workload container to route traffic through the Agent Proxy. The module outputs `aembit_http_proxy` and `aembit_https_proxy` values for the `http_proxy` and `https_proxy` environment variables.

5. Run `terraform init` to download the module, then `terraform apply` to deploy

## Verification

- The Agent Controller appears as **Active** in the Aembit Management Console under **Edge Components → Agent Controllers**
- The Agent Proxy sidecar container starts successfully in the ECS task (check ECS task logs in CloudWatch)
- The application accesses target services without stored credentials
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show credentials were injected for the ECS workload

## Troubleshooting

- **Agent Controller shows Inactive:** The ECS task role does not match the Trust Provider match rules. Verify the `accountId` and `assumedRole` values match the task role assigned to the Agent Controller ECS service
- **Agent Proxy container fails to start:** The Agent Controller is not reachable from the proxy. Verify the security group allows HTTP access from the proxy subnet to the Agent Controller service
- **Application does not route through proxy:** The `http_proxy` and `https_proxy` environment variables are missing from the workload container. Add them using the Terraform module outputs
- **Credentials not injected:** The Access Policy is not active or the Client Workload identifier does not match. Verify the ECS Task Family matches the task definition family name
