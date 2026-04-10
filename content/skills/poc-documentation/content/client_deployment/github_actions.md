## Prerequisites

- GitHub Actions workflow file in the repository (`.github/workflows/`)
- Edge SDK Client ID from the Aembit Trust Provider (client identity module completed)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AEMBIT_CLIENT_ID}}` | Aembit Management Console → **Trust Providers** → select the GitHub Trust Provider → **Edge SDK Client ID** |
| `{{SERVER_HOST}}` | Aembit Management Console → **Server Workloads** → select the target workload → **Host** |
| `{{SERVER_PORT}}` | Aembit Management Console → **Server Workloads** → select the target workload → **Port** |

## Deployment

1. Add the Aembit credential retrieval step to your GitHub Actions workflow:

```yaml
jobs:
  my-job:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - name: Get credentials from Aembit
        id: aembit
        uses: Aembit/get-credentials@v1
        with:
          client-id: '{{AEMBIT_CLIENT_ID}}'
          server-host: '{{SERVER_HOST}}'
          server-port: '{{SERVER_PORT}}'

      - name: Use credentials
        run: |
          # Credentials are available as step outputs
          # Reference: ${{ steps.aembit.outputs.TOKEN }}
```

2. Commit and push the workflow file to the repository

> **Note:** The `permissions.id-token: write` setting is required for GitHub to issue OIDC tokens to the workflow.

## Verification

- Trigger the workflow and confirm the Aembit step completes successfully
- Verify subsequent steps can access the target service using the injected credentials
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show credentials were issued to the GitHub Actions workflow

## Troubleshooting

- **Aembit step fails with authentication error:** The `id-token: write` permission is missing from the workflow. Add it under `permissions` at the job or workflow level
- **Aembit step fails with "client not found":** The Edge SDK Client ID does not match the Trust Provider. Copy the value directly from the Trust Provider in the Aembit console
- **Credentials issued but target service rejects them:** The Server Workload host or port does not match the actual service endpoint. Verify the values in the Aembit console match the service your workflow is calling
