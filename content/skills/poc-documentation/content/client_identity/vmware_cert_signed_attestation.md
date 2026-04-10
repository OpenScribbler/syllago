## Prerequisites

- Network Identity Attestor deployed and healthy (complete the NIA infrastructure module first)
- Attestation signing certificate (the public `.crt` file from the NIA) available for upload

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AEMBIT_CLIENT_ID}}` | Aembit Management Console → **Client Workloads** → select the Client Workload → **Client Identification** section → **Aembit Client ID** value (auto-generated) |
| `{{SIGNING_CERTIFICATE}}` | The attestation signing certificate file (`.crt`) generated for the NIA during the infrastructure module |

## Aembit Configuration

1. Navigate to **Client Workloads** and create a new Client Workload
   - Select **Aembit Client ID** from the identifier dropdown
   - The value is auto-generated - copy it for use when configuring the Agent Proxy: `{{AEMBIT_CLIENT_ID}}`

2. Navigate to **Trust Providers** and create a new **Certificate Signed Attestation** Trust Provider
   - Click **Add** to upload the attestation signing certificate (`{{SIGNING_CERTIFICATE}}`)
   - This is the public certificate from the NIA signing key pair, not the TLS certificate
