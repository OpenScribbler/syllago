## Prerequisites

- Agent Controller deployed and accessible (complete the Agent Controller module first)
- EC2 instance has outbound internet access to Aembit Cloud

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{AGENT_PROXY_VERSION}} | Aembit Management Console → **Edge Components → Deploy Aembit Edge → Red Hat Enterprise Linux** → Edge component versions |

## Deployment

1. SSH into the EC2 instance running the Agent Controller

2. Download, extract, and install the Aembit Agent Proxy:

```bash
wget https://releases.aembit.io/agent_proxy/{{AGENT_PROXY_VERSION}}/linux/amd64/aembit_agent_proxy_linux_amd64_{{AGENT_PROXY_VERSION}}.tar.gz
tar xf aembit_agent_proxy_linux_amd64_{{AGENT_PROXY_VERSION}}.tar.gz
cd aembit_agent_proxy_linux_amd64_{{AGENT_PROXY_VERSION}}
sudo AEMBIT_AGENT_CONTROLLER=http://localhost:5000 ./install
```

3. Configure the EC2 instance to trust the Aembit tenant Root CA
   - Download Root CA from Aembit Management Console (**Edge Components → TLS Decrypt → Download Tenant Root CA**)
   - Install to system trust store: `sudo update-ca-trust`

4. Remove any existing credential management from the application:
   - Delete environment variables, secrets manager calls, and hardcoded tokens used for outbound authentication
   - Some applications require credentials to have values to function.  In this case you can use placeholder values like 'AembitManaged' and the Agent Proxy will overwrite the values
   - The application should now make unauthenticated outbound requests - the proxy injects credentials transparently

5. Run the application and verify it executes successfully

## Verification

- Application completes outbound requests without 401 errors
- Navigate to **Reporting** in the Aembit Management Console and confirm log entries show the workload authenticated and credentials were injected
- No credentials appear in the application's environment variables or configuration files

## Troubleshooting

- **Proxy not starting:** Verify the Agent Controller is healthy. Check proxy service status: `sudo systemctl status aembit-agent-proxy`
- **TLS errors on outbound requests:** The Aembit Root CA is not trusted. Re-run the `update-ca-trust` step and restart the application
- **401 errors persist after removing credentials:** The Aembit Access Policy isn't matching.  Check the Access Policy Conditions and look for errors in the **Reporting** page.
