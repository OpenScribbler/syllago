## Prerequisites

- GitLab Cloud project with CI/CD pipelines enabled
- Project path: `{{GITLAB_PROJECT_PATH}}`

> **Note:** Aembit supports GitLab Cloud only. Self-hosted GitLab instances are not supported.

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{GITLAB_PROJECT_PATH}}` | GitLab project page heading or **Settings → General**, in `group/project` format (e.g., `acme-corp/api-service`) |

## Aembit Configuration

1. Navigate to **Trust Providers** and click **+ New**
   - **Trust Provider Type**: select **Gitlab Job ID Token**
   - Copy the **Edge SDK Client ID** and **Audience** values - you need both for the GitLab pipeline configuration
   - Click **Save**

2. Navigate to **Client Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `GitLab - {{GITLAB_PROJECT_PATH}}`)
   - Add a **GitLab ID Token Project Path** identifier: `{{GITLAB_PROJECT_PATH}}`
   - Click **Save**
