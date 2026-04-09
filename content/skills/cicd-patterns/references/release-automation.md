# Release Automation

## Tag-Triggered Releases

- Rule: Use `on: push: tags: ['v*']` to trigger release workflows on semantic version tags.
- Rule: Create tags via `git tag v1.2.3 && git push origin v1.2.3` or through GitHub's release UI. Never create tags in CI (circular trigger risk).

```yaml
on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'      # Stable releases
      - 'v[0-9]+.[0-9]+.[0-9]+-rc.*'  # Release candidates
```

- Gotcha: Tag pushes do not trigger `pull_request` workflows. If your release workflow needs PR context, use `release: types: [published]` instead.

## Container Image Publishing

### Docker Build and Push

```yaml
jobs:
  publish:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@<SHA>

      - uses: docker/login-action@<SHA>
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - uses: docker/metadata-action@<SHA>
        id: meta
        with:
          images: ghcr.io/${{ github.repository }}
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha,prefix=

      - uses: docker/build-push-action@<SHA>
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### Registry Reference

| Registry | Login Action `registry` | Credentials |
|----------|------------------------|-------------|
| GHCR | `ghcr.io` | `GITHUB_TOKEN` (packages:write) |
| Docker Hub | (omit for default) | `DOCKERHUB_USERNAME` + `DOCKERHUB_TOKEN` |
| AWS ECR | `<account>.dkr.ecr.<region>.amazonaws.com` | OIDC (see security.md) |
| GCP Artifact Registry | `<region>-docker.pkg.dev` | OIDC (see security.md) |

- Rule: Use `docker/metadata-action` to generate consistent tags (semver, sha, branch). Do not hand-craft tag strings.
- Rule: Enable build cache with `cache-from: type=gha` and `cache-to: type=gha,mode=max` to speed up rebuilds.
- Gotcha: GHCR images are private by default. Set package visibility to public in repository settings if needed.

## GitHub Release Creation

```yaml
- uses: softprops/action-gh-release@<SHA>
  with:
    generate_release_notes: true
    files: |
      dist/*.tar.gz
      dist/*.zip
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- Rule: Use `generate_release_notes: true` to auto-generate changelogs from PR titles since the last release. Requires conventional PR titles.
- Rule: Upload build artifacts (binaries, archives, SBOMs) as release assets for reproducibility.

## Environment Promotion

### Pattern: Staging -> Production

```yaml
jobs:
  deploy-staging:
    environment: staging
    runs-on: ubuntu-latest
    steps:
      - run: ./deploy.sh staging

  deploy-production:
    needs: deploy-staging
    environment: production    # Has required reviewers
    runs-on: ubuntu-latest
    steps:
      - run: ./deploy.sh production
```

### Environment Protection Rules

| Rule | Purpose | Typical Config |
|------|---------|----------------|
| Required reviewers | Manual approval gate | 1-2 reviewers from platform team |
| Wait timer | Bake time between stages | 5-15 minutes for staging, 0 for prod (reviewer is the gate) |
| Branch restriction | Only deploy from specific branches | `main` or `release/*` only |
| Custom rules | External policy check | Deployment freeze calendar, change management |

- Rule: Staging deploys automatically on merge to main. Production requires manual approval via environment protection rules.
- Rule: Use `workflow_dispatch` for emergency or ad-hoc production deploys. Include required inputs for the target version.

## Manual Approval Gates

### workflow_dispatch for On-Demand Deploys

```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to deploy (e.g., v1.2.3)'
        required: true
        type: string
      environment:
        description: 'Target environment'
        required: true
        type: choice
        options:
          - staging
          - production
```

- Rule: Combine `workflow_dispatch` with environment protection rules for defense-in-depth. The dispatch trigger starts the workflow; the environment gate requires reviewer approval before the deploy job runs.

## Versioning Strategy

- Rule: Follow semantic versioning (`vMAJOR.MINOR.PATCH`). Use pre-release suffixes for candidates (`v1.2.3-rc.1`).
- Rule: For libraries, create GitHub releases with the tag. For services, the tag triggers a container build and deploy pipeline.
- Rule: Never overwrite or move existing tags. Tags must be immutable for reproducibility.
