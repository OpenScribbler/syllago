# GitHub Actions Security Hardening

## OIDC Federation (Keyless Cloud Auth)

Eliminates static credentials. All providers require `permissions: id-token: write` on the job.

### Canonical Example (AWS)

```yaml
permissions:
  id-token: write
  contents: read

steps:
  - uses: aws-actions/configure-aws-credentials@<SHA>
    with:
      role-to-assume: arn:aws:iam::123456789012:role/GitHubActions
      aws-region: us-east-1
```

### Provider Reference

| Provider | Action | Key Input |
|----------|--------|-----------|
| AWS | `aws-actions/configure-aws-credentials@v4` | `role-to-assume` (IAM role ARN) |
| Azure | `azure/login@v1` | `client-id`, `tenant-id`, `subscription-id` |
| GCP | `google-github-actions/auth@v2` | `workload_identity_provider`, `service_account` |

- Rule: Configure the IdP trust policy to restrict by repo, branch, and environment. Never use a wildcard subject claim.
- Gotcha: OIDC tokens are short-lived. Long jobs may need token refresh. AWS handles this automatically; GCP/Azure may need explicit refresh for jobs >1 hour.

## Permissions Model

Set `permissions: {}` at workflow level (deny-all), then grant per-job.

### Common Permission Grants

| Permission | Grants | Used By |
|------------|--------|---------|
| `contents: read` | Checkout code | Almost every job |
| `contents: write` | Push commits, create releases | Release jobs |
| `pull-requests: write` | Comment on PRs | Coverage/lint bots |
| `id-token: write` | Request OIDC token | Cloud auth jobs |
| `packages: write` | Push to GitHub Packages/GHCR | Container publish jobs |
| `deployments: write` | Update deployment status | Deploy jobs |
| `security-events: write` | Upload SARIF results | SAST/DAST jobs |
| `actions: read` | List workflow runs | Reusable workflow callers |

- Rule: Never use `permissions: write-all`. If you cannot enumerate the needed permissions, the workflow is doing too much -- split it.

## Action Pinning and Supply Chain

- Rule: Pin all third-party actions to full commit SHA (`uses: actions/checkout@<40-char-sha>`). Tags and branches are mutable and can be hijacked.
- Rule: GitHub-owned actions (`actions/*`, `github/*`) are lower risk but should still be pinned in security-critical workflows.
- Fix: Use `pin-github-action` CLI or Dependabot to automate SHA pinning and updates.

### Dependabot Configuration

```yaml
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    commit-message:
      prefix: "ci"
```

## Script Injection Prevention

- Rule: Never interpolate user-controlled values in `run:` blocks. Attackers craft PR titles, branch names, or commit messages to inject commands.
- Fix: Pass values through environment variables, which are not interpreted as shell commands.

```yaml
# BAD - injectable
- run: echo "PR title is ${{ github.event.pull_request.title }}"

# GOOD - safe
- env:
    PR_TITLE: ${{ github.event.pull_request.title }}
  run: echo "PR title is $PR_TITLE"
```

- Rule: For action inputs, prefer passing as `with:` parameters over `run:` interpolation.

## Fork and pull_request_target Security

- Rule: `pull_request_target` runs in the context of the **base** (target) branch with access to secrets. It exists for labeling/commenting, not for building PR code.
- Rule: NEVER checkout `github.event.pull_request.head.ref` in a `pull_request_target` workflow. This executes attacker-controlled code with your secrets.
- Safe pattern: Use `pull_request` (no secrets, runs fork code safely) for CI. Use `pull_request_target` only for metadata operations (labeling, commenting) without checking out PR code.
- Gotcha: `workflow_run` triggered by a fork PR also runs in base context. Apply the same caution.

## Secret Management

- Rule: Use GitHub repository or organization secrets for credentials. Use environment secrets for environment-specific values (production API keys).
- Rule: Use `::add-mask::` to redact dynamic values from logs. GitHub automatically masks secrets referenced as `${{ secrets.* }}`, but values derived from secrets (substrings, base64-encoded) are not auto-masked.
- Rule: Environment protection rules (required reviewers, wait timers) gate access to environment secrets. Use for production credentials.
- Gotcha: Secrets are not available to workflows triggered by fork PRs (by design). Do not work around this -- it is a security feature.

## CODEOWNERS

```
# .github/CODEOWNERS
/.github/workflows/ @platform-team
/.github/actions/   @platform-team
```

- Rule: Require CODEOWNERS review for workflow changes. Combined with branch protection, this prevents unauthorized pipeline modifications.
