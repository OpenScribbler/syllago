## Prerequisites

- Agent Controller deployed and accessible (complete the VMware Agent Controller module first)
- Network Identity Attestor deployed and healthy (complete the NIA module first)
- Client workload VM can resolve the Agent Controller and NIA hostnames via DNS
- Client workload VM trusts the root CA that issued the Agent Controller and NIA TLS certificates

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AGENT_PROXY_VERSION}}` | Aembit Management Console → **Edge Components → Deploy Aembit Edge → Red Hat Enterprise Linux** → Edge component versions |
| `{{AC_HOSTNAME}}` | Hostname of the Agent Controller VM |
| `{{NIA_HOSTNAME}}` | Hostname of the Network Identity Attestor VM |
| `{{AEMBIT_TENANT_ID}}` | Aembit Management Console → **Administration → Tenant Information → Tenant ID** |

## Deployment

1. SSH into the client workload VM

2. Install the root CA certificate used to sign the Agent Controller and NIA TLS certificates into the system trust store:

```bash
sudo cp {{ROOT_CA_CERT_PATH}} /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

3. Download, extract, and install the Aembit Agent Proxy:

```bash
wget https://releases.aembit.io/agent_proxy/{{AGENT_PROXY_VERSION}}/linux/amd64/aembit_agent_proxy_linux_amd64_{{AGENT_PROXY_VERSION}}.tar.gz
tar xf aembit_agent_proxy_linux_amd64_{{AGENT_PROXY_VERSION}}.tar.gz
cd aembit_agent_proxy_linux_amd64_{{AGENT_PROXY_VERSION}}
sudo AEMBIT_AGENT_CONTROLLER=https://{{AC_HOSTNAME}}:5443 \
     AEMBIT_NETWORK_ATTESTOR_URL=https://{{NIA_HOSTNAME}}:443 \
     AEMBIT_CLIENT_WORKLOAD_PROCESS_IDENTIFICATION_ENABLED=true \
     AEMBIT_LOG_LEVEL=debug \
     ./install
```

4. Download and install the Aembit tenant Root CA for TLS decrypt:

```bash
sudo curl https://{{AEMBIT_TENANT_ID}}.aembit.io/api/v1/root-ca --output /usr/local/share/ca-certificates/aembit.crt
sudo update-ca-certificates
```

5. Remove any existing credential management from the application:
   - Delete environment variables, secrets manager calls, and hardcoded tokens used for outbound authentication
   - Some applications require credentials to have values to function. In this case you can use placeholder values like 'AembitManaged' and the Agent Proxy will overwrite the values
   - The application should now make unauthenticated outbound requests - the proxy injects credentials transparently

6. Run the application and verify it executes successfully

## Verification

- Application completes outbound requests without 401 errors
- Navigate to **Reporting** in the Aembit Management Console and confirm log entries show the workload authenticated and credentials were injected
- No credentials appear in the application's environment variables or configuration files

## Troubleshooting

- **Proxy not starting:** Verify the Agent Controller is healthy (`curl -k https://{{AC_HOSTNAME}}:5443/health`). Check proxy service status: `sudo systemctl status aembit-agent-proxy`
- **TLS errors connecting to Agent Controller or NIA:** The client workload VM does not trust the root CA that issued the TLS certificates. Re-run the CA install step and run `sudo update-ca-certificates`
- **NIA attestation failures:** Verify the NIA is healthy (`curl -k https://{{NIA_HOSTNAME}}:443/health`). Confirm the client workload VM is on the same L2 network segment as the NIA. Check proxy logs: `sudo journalctl -u aembit-agent-proxy -n 50`
- **TLS errors on outbound application requests:** The Aembit tenant Root CA is not trusted. Re-run the tenant Root CA download step and run `sudo update-ca-certificates`
- **401 errors persist after removing credentials:** The Aembit Access Policy is not matching. Check the Access Policy Conditions and look for errors in the **Reporting** page.
