## Overview

The Agent Controller authenticates Aembit Agent Proxies to Aembit Cloud. It must be deployed and showing **Active** in the Aembit Management Console before any Agent Proxies can function. This module covers an on-premises Agent Controller deployment using Device Code authentication.

For the POC we will install an Agent Controller locally on every virtual machine that is using the proxy, but in production you would deploy this as a centralized service.

> **Already have an Agent Controller?** If an Agent Controller is already deployed and showing Active, skip this module and use the existing controller when configuring Agent Proxies.

> **TLS options:** The Agent Controller supports both Aembit Managed TLS and customer-managed TLS. This module documents customer-managed TLS. To use Aembit Managed TLS instead, replace the `TLS_PEM_PATH` and `TLS_KEY_PATH` variables in the install command with `AEMBIT_MANAGED_TLS_HOSTNAME={{AC_HOSTNAME}}`.

## Prerequisites

- Linux VM (Ubuntu 24.04 recommended)
- VM has outbound internet access to Aembit Cloud
- Customer-provided TLS certificate and private key (CN must match the Agent Controller hostname, full chain required)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AC_HOSTNAME}}` | Hostname assigned to the Agent Controller VM |
| `{{AEMBIT_TENANT_ID}}` | Aembit Management Console → **Administration → Tenant Information → Tenant ID** |
| `{{AEMBIT_STACK_DOMAIN}}` | Aembit Management Console → **Administration → Tenant Information → Stack Domain** |
| `{{AEMBIT_DEVICE_CODE}}` | Aembit Management Console → **Edge Components → Agent Controllers** → create a controller → **Device Code** (see Part 1) |
| `{{AGENT_CONTROLLER_VERSION}}` | Aembit Management Console → **Edge Components → Deploy Aembit Edge → Red Hat Enterprise Linux** → Edge component versions |
| `{{AC_TLS_CERT_PATH}}` | Path to the TLS certificate file on the Agent Controller VM |
| `{{AC_TLS_KEY_PATH}}` | Path to the TLS private key file on the Agent Controller VM |

## Part 1: Aembit Console - Create Agent Controller

1. In the Aembit Management Console, navigate to **Edge Components → Agent Controllers** and [create a new Agent Controller](https://docs.aembit.io/user-guide/deploy-install/advanced-options/agent-controller/create-agent-controller)
   - Copy the **Device Code**: `{{AEMBIT_DEVICE_CODE}}`

## Part 2: Prepare TLS Certificates

Generate a TLS certificate and private key from your organization's internal PKI or certificate authority:
- CN must match the Agent Controller hostname (`{{AC_HOSTNAME}}`)
- SAN required
- Full certificate chain (leaf first, then intermediates)

Place the certificate and key on the Agent Controller VM at accessible paths.

## Part 3: Agent Controller Deployment

1. SSH into the VM designated as the Agent Controller host

2. Download and extract the Agent Controller package:

```bash
wget https://releases.aembit.io/agent_controller/{{AGENT_CONTROLLER_VERSION}}/linux/amd64/aembit_agent_controller_linux_amd64_{{AGENT_CONTROLLER_VERSION}}.tar.gz
tar xf aembit_agent_controller_linux_amd64_{{AGENT_CONTROLLER_VERSION}}.tar.gz
cd aembit_agent_controller_linux_amd64_{{AGENT_CONTROLLER_VERSION}}
```

3. Run the installer with the required environment variables:

```bash
sudo AEMBIT_TENANT_ID={{AEMBIT_TENANT_ID}} \
     AEMBIT_DEVICE_CODE={{AEMBIT_DEVICE_CODE}} \
     AEMBIT_STACK_DOMAIN={{AEMBIT_STACK_DOMAIN}} \
     TLS_PEM_PATH={{AC_TLS_CERT_PATH}} \
     TLS_KEY_PATH={{AC_TLS_KEY_PATH}} \
     AEMBIT_LOG_LEVEL=debug \
     ./install
```

4. Verify the controller is healthy:

```bash
curl -k https://localhost:5443/health
```

A `200 OK` response with `{"status":"healthy"}` confirms the controller is running.

## Verification

- Agent Controller status shows as **Active** in the Aembit Management Console
- `curl -k https://localhost:5443/health` returns a healthy response

## Troubleshooting

- **Controller shows Inactive:** Verify the Device Code matches what was generated in the Aembit Console. Verify the VM has outbound internet access to Aembit Cloud endpoints. Check logs: `sudo journalctl -u aembit-agent-controller -n 50`
- **TLS errors:** Verify the TLS certificate CN and SAN match the Agent Controller hostname. Ensure the private key is in PKCS8 format. If using a PKCS1 key, convert it: `openssl pkcs8 -in tls.key -topk8 -out tls.pkcs8.key -nocrypt`
- **Health endpoint not responding:** Confirm the service is running: `sudo systemctl status aembit-agent-controller`. With customer-managed TLS, the health endpoint listens on port `5443` (HTTPS), not `5000` (HTTP).
