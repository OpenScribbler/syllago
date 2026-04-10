## Prerequisites

- GitHub repository with Actions enabled
- Repository: `{{GITHUB_REPOSITORY}}`

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{GITHUB_REPOSITORY}}` | GitHub repository in `org/repo-name` format (e.g., `acme-corp/api-service`) |

## Aembit Configuration

1. Navigate to **Trust Providers** and click **+ New**
   - **Trust Provider Type**: select **GitHub**
   - Copy the **Edge SDK Client ID** - you need this for the GitHub Actions workflow configuration
   - Click **Save**

2. Navigate to **Client Workloads** and click **+ New**
   - **Name**: descriptive label (e.g., `GitHub - {{GITHUB_REPOSITORY}}`)
   - Add a **GitHub ID Token Repository** identifier: `{{GITHUB_REPOSITORY}}`
   - Click **Save**
