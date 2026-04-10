## Prerequisites

- GitLab CI/CD pipeline configuration file (`.gitlab-ci.yml`)
- Edge SDK Client ID and Audience from the Aembit Trust Provider (client identity module completed)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AEMBIT_CLIENT_ID}}` | Aembit Management Console → **Trust Providers** → select the GitLab Trust Provider → **Edge SDK Client ID** |
| `{{AEMBIT_AUDIENCE}}` | Aembit Management Console → **Trust Providers** → select the GitLab Trust Provider → **Audience** |
| `{{SERVER_HOST}}` | Aembit Management Console → **Server Workloads** → select the target workload → **Host** |
| `{{SERVER_PORT}}` | Aembit Management Console → **Server Workloads** → select the target workload → **Port** |

## Deployment

1. Add the Aembit component to your `.gitlab-ci.yml`:

```yaml
include:
  - component: >-
      $CI_SERVER_FQDN/aembit/aembit-edge/
      aembit-get-credentials@v1
    inputs:
      client-id: "{{AEMBIT_CLIENT_ID}}"
      aud: "{{AEMBIT_AUDIENCE}}"
      server-workload-host: "{{SERVER_HOST}}"
      server-workload-port: "{{SERVER_PORT}}"

my-job:
  stage: deploy
  script:
    - |
      # Credentials are available as $TOKEN
      curl --header "Authorization: Bearer $TOKEN" \
        https://{{SERVER_HOST}}
```

2. Commit and push the pipeline configuration

## Verification

- Trigger the pipeline and confirm the Aembit component step completes successfully
- Verify the job can access the target service using the `$TOKEN` environment variable
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show credentials were issued to the GitLab CI/CD job

## Troubleshooting

- **Component step fails with authentication error:** The audience value does not match the Trust Provider. Copy the `aud` value directly from the Trust Provider in the Aembit console
- **Component step fails with "client not found":** The Edge SDK Client ID does not match the Trust Provider. Copy the value directly from the Aembit console
- **$TOKEN variable is empty:** The Aembit component did not run before your job. Verify the `include` block is at the top level of `.gitlab-ci.yml` and the component version tag is correct
- **Target service rejects the token:** The Server Workload host or port does not match the actual service endpoint. Verify the values in the Aembit console
