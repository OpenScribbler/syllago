## Prerequisites

- Kubernetes cluster with ability to modify pod specs
- Container registry to publish the sidecar image
- Aembit CLI version to pin: `{{AEMBIT_CLI_VERSION}}`
  - Find the latest release version in the Aembit Management Console or release notes

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{AEMBIT_CLI_VERSION}} | Aembit release notes or Management Console → Downloads |
| {{JWT_LIFETIME_MINUTES}} | Configured in the Aembit Credential Provider → **Lifetime** field (in minutes) |
| {{AEMBIT_CLIENT_ID}} | Aembit Management Console → **Trust Providers** → select your Trust Provider → **Edge SDK Client ID** |
| {{API_GW_HOST}} | Hostname of the target API gateway (same as Server Workload host) |
| {{API_GW_PORT}} | Port of the target API gateway (same as Server Workload port) |
| {{CONTAINER_REGISTRY}} | Your container registry hostname (e.g., `myregistry.azurecr.io`) |
| {{APP_IMAGE}} | Your application container image reference (e.g., `myregistry.azurecr.io/myapp:latest`) |

## Deployment

### Step 1: Build the Aembit CLI Sidecar Image

Create a dedicated sidecar container image that runs the Aembit CLI and writes the credential to a shared in-memory volume. You do not need to modify your application container image.

**`refresh-token.sh`**

```sh
#!/bin/sh
set -e
while true; do
  eval $(aembit credentials get \
    --client-id "${AEMBIT_CLIENT_ID}" \
    --server-workload-host "${API_GW_HOST}" \
    --server-workload-port "${API_GW_PORT}" \
    --id-token "$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)")
  printf '%s' "${TOKEN}" > /var/aembit/token
  sleep 180
done
```

This script fetches a fresh credential on startup and refreshes every 180 seconds. Set `sleep` to roughly half the token lifetime in seconds (e.g., a 10-minute token lifetime → `sleep 300`).

**`Dockerfile`**

```dockerfile
FROM alpine:3.19

ARG AEMBIT_CLI_VERSION={{AEMBIT_CLI_VERSION}}

RUN apk add --no-cache curl \
    && curl -fsSL "https://releases.aembit.io/agent/${AEMBIT_CLI_VERSION}/linux/amd64/aembit_agent_cli_linux_amd64_${AEMBIT_CLI_VERSION}.tar.gz" \
       | tar -xz \
    && mv aembit /usr/local/bin/ \
    && chmod +x /usr/local/bin/aembit

COPY refresh-token.sh /usr/local/bin/refresh-token.sh
RUN chmod +x /usr/local/bin/refresh-token.sh

CMD ["/usr/local/bin/refresh-token.sh"]
```

Build and push to your container registry:

```sh
docker build --build-arg AEMBIT_CLI_VERSION={{AEMBIT_CLI_VERSION}} -t {{CONTAINER_REGISTRY}}/aembit-cli-sidecar:{{AEMBIT_CLI_VERSION}} .
docker push {{CONTAINER_REGISTRY}}/aembit-cli-sidecar:{{AEMBIT_CLI_VERSION}}
```

### Step 2: Add the Sidecar, Init Container, and Shared Volume to Your Pod Spec

Add the following to your existing Deployment or Pod manifest:

```yaml
volumes:
  - name: aembit-token
    emptyDir:
      medium: Memory   # token stored in RAM only: never written to node filesystem

initContainers:
  # Fetches the first token before the main container starts
  - name: aembit-cli-init
    image: {{CONTAINER_REGISTRY}}/aembit-cli-sidecar:{{AEMBIT_CLI_VERSION}}
    env:
      - name: AEMBIT_CLIENT_ID
        value: "{{AEMBIT_CLIENT_ID}}"
      - name: API_GW_HOST
        value: "{{API_GW_HOST}}"
      - name: API_GW_PORT
        value: "{{API_GW_PORT}}"
    command: ["/bin/sh", "-c"]
    args:
      - |
        eval $(aembit credentials get \
          --client-id "${AEMBIT_CLIENT_ID}" \
          --server-workload-host "${API_GW_HOST}" \
          --server-workload-port "${API_GW_PORT}" \
          --id-token "$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)")
        printf '%s' "${TOKEN}" > /var/aembit/token
    volumeMounts:
      - name: aembit-token
        mountPath: /var/aembit

containers:
  # --- existing application container ---
  - name: app
    image: {{APP_IMAGE}}
    volumeMounts:
      - name: aembit-token
        mountPath: /var/aembit
        readOnly: true   # app only reads; sidecar writes

  # --- Aembit CLI sidecar (handles ongoing refresh) ---
  - name: aembit-cli
    image: {{CONTAINER_REGISTRY}}/aembit-cli-sidecar:{{AEMBIT_CLI_VERSION}}
    env:
      - name: AEMBIT_CLIENT_ID
        value: "{{AEMBIT_CLIENT_ID}}"
      - name: API_GW_HOST
        value: "{{API_GW_HOST}}"
      - name: API_GW_PORT
        value: "{{API_GW_PORT}}"
    volumeMounts:
      - name: aembit-token
        mountPath: /var/aembit
    resources:
      requests:
        memory: "64Mi"
        cpu: "50m"
      limits:
        memory: "128Mi"
        cpu: "100m"
```

### Step 3: Update Application to Read the Token

The application must read the token from `/var/aembit/token` and attach it as a Bearer token on each outbound request:

```
Authorization: Bearer <contents of /var/aembit/token>
```

## Verification

- Confirm the init container completes before the main container starts:

```bash
kubectl describe pod <pod-name> | grep -A5 "Init Containers"
```

- Confirm the token file exists and is non-empty:

```bash
kubectl exec <pod-name> -c app -- cat /var/aembit/token
```

A valid JWT string (three dot-separated base64 segments) confirms the sidecar is working correctly.

## Troubleshooting

- **Token file empty or missing:** Check sidecar container logs for authentication errors: `kubectl logs <pod-name> -c aembit-cli`. Verify `AEMBIT_CLIENT_ID`, `API_GW_HOST`, and `API_GW_PORT` environment variables are set correctly.
- **Init container crashloopbackoff:** The first credential fetch failed. Check init container logs: `kubectl logs <pod-name> -c aembit-cli-init`. Common causes: incorrect Client ID, OIDC issuer mismatch, or the Trust Provider subject does not match the pod's service account.
- **Application receives expired token errors:** The token is expiring before the next refresh cycle - the refresh interval is more than half the token lifetime. Reduce `sleep 180` to less than half the token lifetime in seconds (token lifetime in minutes multiplied by 30).
- **curl returns 404 or download fails:** The `{{AEMBIT_CLI_VERSION}}` value is incorrect or the version has been removed. Verify the correct version string in the Aembit Management Console → Downloads, or check `https://releases.aembit.io/agent/` for available versions.
