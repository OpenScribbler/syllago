---
name: cicd-patterns
description: GitHub Actions CI/CD patterns for workflow design, security hardening, release automation, and self-hosted runners. Use when creating or reviewing GitHub Actions workflows, configuring OIDC auth, publishing containers, or managing runners. Language-agnostic -- language skills handle language-specific CI config.
---

# GitHub Actions CI/CD Patterns

Patterns for building secure, maintainable GitHub Actions pipelines.

## Production Readiness Checklist

| Category | Requirement |
|----------|-------------|
| Permissions | `permissions: {}` at workflow level, grant per-job |
| Action pinning | Pin third-party actions to full SHA, not tags |
| Secrets | GitHub Secrets or environment secrets, never hardcoded |
| Timeouts | `timeout-minutes` on every job (default: 30) |
| Concurrency | `concurrency` group to prevent duplicate runs |
| OIDC | Use OIDC for cloud auth, no static credentials |
| Environments | Protection rules on production (required reviewers, wait timers) |
| Dependabot | `package-ecosystem: "github-actions"` configured |
| CODEOWNERS | `.github/workflows` requires review for changes |
| Branch protection | Require status checks to pass before merge |
| Artifacts | Upload test results and logs with `if: always()` |
| Runners | Self-hosted only on private repos, ephemeral preferred |

## Anti-Patterns

- **Overprivileged workflows**: Using `permissions: write-all` or omitting permissions (gets full default). Always start with `permissions: {}` and add per-job.
- **Tag-pinned actions**: `uses: actions/checkout@v4` is mutable. Pin to SHA for supply chain security.
- **Script injection**: Interpolating `${{ github.event.*.title }}` in `run:` blocks. Use environment variables instead.
- **pull_request_target + checkout PR**: Checking out PR head code in `pull_request_target` exposes secrets to fork PRs. Never combine these.
- **Long-lived cloud credentials**: Static AWS/Azure/GCP keys in secrets. Use OIDC federation instead.
- **Monolithic workflows**: Single 500-line workflow file. Split into reusable workflows by concern.
- **No concurrency control**: Duplicate workflow runs on rapid pushes waste minutes. Use `concurrency: { group: ${{ github.workflow }}-${{ github.ref }}, cancel-in-progress: true }`.
- **Self-hosted on public repos**: Forks can run arbitrary code on your runner. Never use self-hosted runners for public repositories.

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| OIDC setup, permissions, action pinning, script injection, fork security, Dependabot | [security.md](references/security.md) |
| Reusable workflows, composite actions, triggers, matrix, conditionals | [workflows.md](references/workflows.md) |
| Container publishing, tag-triggered releases, environment promotion, approvals | [release-automation.md](references/release-automation.md) |
| Self-hosted runners, ephemeral/JIT, Actions Runner Controller, scaling | [runners.md](references/runners.md) |

## Related Skills

- **CI testing patterns** (matrix, caching, coverage, flaky tests, parallelization): Load [testing-patterns/references/ci-cd.md](../testing-patterns/references/ci-cd.md)
- **Kubernetes deployment**: Load [kubernetes-patterns](../kubernetes-patterns/SKILL.md) for K8s manifests and deployment strategies
- **Security audit**: Load [security-audit](../security-audit/SKILL.md) for threat modeling and OWASP checks
- **Terraform/IaC**: Load [terraform-patterns](../terraform-patterns/SKILL.md) for infrastructure-as-code CI integration
