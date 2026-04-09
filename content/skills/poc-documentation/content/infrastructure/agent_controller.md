## Overview

The Agent Controller authenticates Aembit Agent Proxies to Aembit Cloud. It must be deployed and showing **Active** in the Aembit Management Console before any Agent Proxies can function. This module covers an EC2-based Agent Controller deployment using AWS Metadata trust.

For the POC we will install an Agent Controller locally on every virtual machine that is using the proxy, but in production you would deploy this as a centralized service.

> **Already have an Agent Controller?** If an Agent Controller is already deployed and showing Active, skip Parts 1–3 and use the existing Controller ID when configuring Agent Proxies.

> **Kubernetes deployments:** If the client workload runs on Kubernetes and you are using a Helm-based deployment (k8s_helm, openshift_helm, or eks_fargate_helm), the Agent Controller is created as part of that module's Part 1. This module is not needed for those deployments.

## Prerequisites

- AWS EC2 instance (Amazon Linux 2023 recommended)
- EC2 instance has outbound internet access to Aembit Cloud

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{AWS_ACCOUNT_ID}} | AWS Console → top-right account menu, or run: `aws sts get-caller-identity --query Account --output text` |
| {{AGENT_CONTROLLER_ID}} | Aembit Management Console → **Edge Components → Agent Controllers** → create or select a controller → **Controller ID** |
| {{AGENT_CONTROLLER_VERSION}} | Aembit Management Console → **Edge Components → Deploy Aembit Edge → Red Hat Enterprise Linux** → Edge component versions |

## Part 1: Aembit Console — Create Agent Controller

1. In the Aembit Management Console, navigate to **Trust Providers** and create a new [**AWS Metadata Trust Provider**](https://docs.aembit.io/user-guide/access-policies/trust-providers/aws-metadata-service-trust-provider)
   - Set the Match Rule to **Account ID** with value `{{AWS_ACCOUNT_ID}}`

2. Navigate to **Edge Components → Agent Controllers** and [create a new Agent Controller](https://docs.aembit.io/user-guide/deploy-install/advanced-options/agent-controller/create-agent-controller)
   - Select the Trust Provider created in step 1
   - Copy the **Controller ID**: `{{AGENT_CONTROLLER_ID}}`

## Part 2: Agent Controller Deployment

1. SSH into the EC2 instance designated as the Agent Controller host

2. Download and extract the Agent Controller package:

```bash
wget https://releases.aembit.io/agent_controller/{{AGENT_CONTROLLER_VERSION}}/linux/amd64/aembit_agent_controller_linux_amd64_{{AGENT_CONTROLLER_VERSION}}.tar.gz
tar xf aembit_agent_controller_linux_amd64_{{AGENT_CONTROLLER_VERSION}}.tar.gz
cd aembit_agent_controller_linux_amd64_{{AGENT_CONTROLLER_VERSION}}
sudo AEMBIT_TENANT_ID={{AEMBIT_TENANT_ID}} AEMBIT_AGENT_CONTROLLER_ID={{AGENT_CONTROLLER_ID}} ./install
```

3. Verify the controller is healthy by checking the health endpoint from the Agent Controller EC2 instance:

```bash
curl http://localhost:5000/health
```

A `200 OK` response with `{"status":"healthy"}` confirms the controller is running.

## Verification

- Agent Controller status shows as **Active** in the Aembit Management Console
- `curl http://localhost:5000/health` returns a healthy response

## Troubleshooting

- **Controller shows Inactive:** 
  * The AWS Metadata Service Trust Provider has the [wrong public certificate for the AWS Region being used](https://docs.aembit.io/user-guide/access-policies/trust-providers/aws-metadata-service-trust-provider#additional-configurations)
  * the account ID in the Trust Provider match rule does not match. Verify both and restart the service: `sudo systemctl restart aembit-agent-controller`
  * The EC2 instance cannot reach Aembit Cloud endpoints. Verify outbound internet access and that no security group or NACL blocks the connection
