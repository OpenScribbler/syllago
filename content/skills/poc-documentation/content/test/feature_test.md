<div class="cover-page" style="text-align:center; padding-top: 40px; padding-bottom: 40px;">

<img src="assets/aembit-logo-white.png" alt="Aembit" style="width: 100px; margin-bottom: 30px;">

<div class="cover-title">Feature Test Document</div>
<div class="cover-subtitle">PDF Pipeline Validation</div>

<div class="cover-meta" style="margin-top: 30px;">
  <div>Generated: {{POC_START_DATE}}</div>
  <div style="margin-top: 8px;">Prepared by: <strong>{{SA_NAME}}</strong>, Solutions Architect</div>
  <div>{{SA_EMAIL}}</div>
</div>

</div>

<div class="page-break"></div>

# Typography & Headings

This section tests all heading levels and basic typography.

## H2: Section Heading

Standard paragraph text in system-ui at 10.5pt. This paragraph contains **bold text** and *italic text* for emphasis. It also contains `inline code` which should render with a light gray background. The line height should be 1.5 for comfortable reading.

### H3: Subsection Heading

A third-level heading in accent red (#D9312A). Below it, a nested paragraph.

This paragraph tests **bold**, *italic*, and ***bold italic*** combinations. It also references a placeholder: <span class="placeholder">{{CUSTOMER_NAME}}</span> appears here in bold orange.

---

# Annotation & Blockquote

> *This is an annotation hint — italic gray text with a gold left border. It guides the SA filling out the template without becoming part of the final deliverable content.*

> *A second annotation block. These blockquotes use the `blockquote` CSS rule with a 3px solid #FFCA45 left border.*

<span class="annotation">This is inline annotation text using the `.annotation` class — gray and italic, but inline rather than block-quoted.</span>

<div class="page-break"></div>

# Lists

## Numbered Steps with Nested Sub-items

1. Navigate to **Trust Providers** in the Aembit Management Console
   - Click **Create New** in the top right
   - Select **AWS Metadata Trust Provider** from the dropdown
   - Enter a descriptive name: `{{CUSTOMER_NAME}}-aws-trust-provider`

2. Configure the Match Rule
   - Set **Match Type** to **Account ID**
   - Enter the AWS Account ID: <span class="placeholder">{{AWS_ACCOUNT_ID}}</span>
   - Save the Trust Provider

3. Navigate to **Agent Controllers** and create a new Agent Controller
   - Select the Trust Provider created in step 1
   - Copy the **Controller ID** — needed in Part 3

4. Deploy the Agent Controller to EC2
   - Follow the installation instructions from the Console
   - Use the Controller ID from step 3

## Unordered List

- First bullet point with **bold** emphasis
- Second bullet point with *italic* emphasis
- Third bullet point with `inline code`
- Fourth bullet point with a <span class="placeholder">{{PLACEHOLDER}}</span> value

## Nested Unordered List

- Top-level item one
  - Nested item A
  - Nested item B
    - Doubly nested item
- Top-level item two
  - Nested item C

## Checklist

<ul class="checklist">
  <li>Agent Controller is Active in Aembit Management Console</li>
  <li>Health endpoint returns healthy status at port 9090</li>
  <li>Agent Proxy authenticates successfully through the Controller</li>
  <li>Access Authorization Events appear in the Aembit audit log</li>
  <li>Access is denied when the access policy is disabled</li>
</ul>

<div class="page-break"></div>

# Code Blocks

## Bash Script

```bash
# Install Aembit Agent Controller
sudo systemctl start aembit-agent-controller
sudo systemctl enable aembit-agent-controller

# Check service status
sudo systemctl status aembit-agent-controller

# Verify health endpoint
curl -s http://localhost:9090/health | jq .
```

## JSON Configuration

```json
{
  "mcpServers": {
    "box": {
      "url": "https://mcp-gateway.aembit.io/v1/mcp",
      "transport": "http"
    }
  },
  "aembit": {
    "tenantId": "{{AEMBIT_TENANT_ID}}",
    "region": "us-east-1"
  }
}
```

## YAML Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aembit-agent-proxy
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: aembit-agent-proxy
  template:
    spec:
      containers:
        - name: proxy
          image: aembit/agent-proxy:latest
          env:
            - name: AEMBIT_TENANT_ID
              value: "{{AEMBIT_TENANT_ID}}"
```

## Inline Code Examples

Use `kubectl get pods -n default` to list pods. The configuration file lives at `~/.aembit/config.yaml`. Set the `AEMBIT_AGENT_PROXY_URL` environment variable to point workloads at the proxy.

<div class="page-break"></div>

# Tables

## 2-Column Table: Configuration Reference

| Parameter | Value |
|-----------|-------|
| Tenant ID | <span class="placeholder">{{AEMBIT_TENANT_ID}}</span> |
| Region | us-east-1 |
| Agent Controller Host | <span class="placeholder">{{CONTROLLER_HOST}}</span> |
| Agent Proxy Port | 8080 |
| Health Check Port | 9090 |

## 3-Column Table: Contact Information

| Name | Role | Contact |
|------|------|---------|
| <span class="placeholder">{{CONTACT_1_NAME}}</span> | <span class="placeholder">{{CONTACT_1_ROLE}}</span> | <span class="placeholder">{{CONTACT_1_EMAIL}}</span> |
| <span class="placeholder">{{CONTACT_2_NAME}}</span> | <span class="placeholder">{{CONTACT_2_ROLE}}</span> | <span class="placeholder">{{CONTACT_2_EMAIL}}</span> |
| {{SA_NAME}} | Solutions Engineer | {{SA_EMAIL}} |

## 4-Column Table: Success Criteria

| No | Category | Criterion | Status |
|----|----------|-----------|--------|
| 1 | Secretless Access | Access uses short-lived credentials per request | <ul class="checklist"><li></li></ul> |
| 2 | Secretless Access | No static credentials (API keys, fixed tokens) | <ul class="checklist"><li></li></ul> |
| 3 | Verified Identity | Only registered workloads allowed access | <ul class="checklist"><li></li></ul> |
| 4 | Policy Enforcement | Access evaluated based on workload attributes | <ul class="checklist"><li></li></ul> |
| 5 | Zero Trust | Auth evaluated for each request, not session | <ul class="checklist"><li></li></ul> |
| 6 | Governance | All access decisions logged and exportable | <ul class="checklist"><li></li></ul> |

<div class="page-break"></div>

# MODULE 1: Deploy Aembit Agent Controller

---

## Overview

The Agent Controller authenticates Aembit Agent Proxies to Aembit Cloud. It must be deployed before any Agent Proxies can function. This module tests the full section layout including all sub-sections.

## Business Value

<span class="placeholder">{{USE_CASE_CONTROLLER_VALUE}}</span>

> *e.g. "Enables all subsequent use cases by providing the secure control plane for workload authentication. Without the Agent Controller, no workload-to-workload access policies can be enforced."*

## Prerequisites

- AWS EC2 instance (Amazon Linux 2 or Ubuntu 22.04 recommended)
- EC2 instance has outbound internet access to Aembit Cloud
- EC2 IAM role with `ec2:DescribeInstances` permission
- AWS Account ID: <span class="placeholder">{{AWS_ACCOUNT_ID}}</span>
- Customer Name: **{{CUSTOMER_NAME}}**

## Part 1: Service Configuration

*No external service configuration required for this module.*

## Part 2: Access Policy Configuration

1. Navigate to **Trust Providers** and create a new **AWS Metadata Trust Provider**
   - Set the Match Rule to **Account ID** with value <span class="placeholder">{{AWS_ACCOUNT_ID}}</span>

2. Navigate to **Agent Controllers** and create a new Agent Controller
   - Select the Trust Provider created in step 1
   - Copy the **Controller ID** — you will need this in Part 3

## Part 3: Agent Controller Deployment

1. SSH into the EC2 instance designated as the Agent Controller host

2. Install the Aembit Agent Controller service:

```bash
sudo systemctl start aembit-agent-controller
sudo systemctl enable aembit-agent-controller
```

3. Verify the controller is healthy:

```bash
curl -s http://localhost:9090/health
```

## Verification

- Agent Controller status shows as **Active** in the Aembit Management Console
- Health endpoint on the controller EC2 instance returns a healthy response
- Logs show no authentication errors: `journalctl -u aembit-agent-controller -n 50`

## Success Criteria

<ul class="checklist">
  <li>Agent Controller is Active in Aembit Management Console</li>
  <li>Health endpoint returns healthy status</li>
  <li>No errors in Agent Controller logs</li>
</ul>

## Troubleshooting

- **Controller shows Inactive:** Verify the EC2 IAM role has `ec2:DescribeInstances` permission and the account ID match rule is correct
- **Installation fails:** Verify outbound connectivity from EC2 to Aembit Cloud endpoints
- **Health check fails:** Check that port 9090 is not blocked by a local firewall rule

<div class="page-break"></div>

# HR and Mixed Content

This section tests horizontal rules and mixed content flow.

---

First rule above uses `#FFCA45` gold styling.

Some normal text follows the rule. **Bold text** and *italic text* continue here. The rule provides a visual separator that is thinner and less aggressive than a full page break.

---

A second HR separator.

> *Note: HRs render as a 2px gold line. They are suitable for separating sub-sections within a module without a heading.*

Final paragraph in this section. The document ends here — the last section does not need a trailing page-break div.
