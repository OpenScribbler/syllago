# Workflow Design Patterns

## Trigger Events

| Trigger | Fires When | Typical Use |
|---------|------------|-------------|
| `push` | Commit pushed to branch | CI on main/release branches |
| `pull_request` | PR opened/updated | CI on feature branches (safe for forks) |
| `pull_request_target` | PR opened/updated (base context) | Labeling, commenting only (see security.md) |
| `workflow_dispatch` | Manual trigger via UI/API | Ad-hoc deploys, maintenance tasks |
| `schedule` | Cron schedule | Nightly builds, dependency updates |
| `release` | GitHub release published | Publish artifacts after release creation |
| `workflow_call` | Called by another workflow | Reusable workflow entry point |
| `workflow_run` | Another workflow completes | Post-CI deploy, artifact processing |

- Rule: Filter `push` with `branches:` and `paths:` to avoid unnecessary runs. Example: `paths: ['src/**', 'go.sum']` skips CI for docs-only changes.
- Rule: Use `pull_request` (not `pull_request_target`) for CI builds. It is safe for forks and does not expose secrets.

## Reusable Workflows vs Composite Actions

| Aspect | Reusable Workflow | Composite Action |
|--------|-------------------|------------------|
| Scope | Full pipeline (multiple jobs) | Steps within a single job |
| Defined in | `.github/workflows/*.yml` | `action.yml` in any repo/directory |
| Secrets access | Yes, via `secrets: inherit` or explicit | No (must pass as inputs) |
| Runner selection | Each job picks its own runner | Runs on caller's runner |
| Nesting | Cannot call another reusable workflow | Nestable up to 10 levels |
| Outputs | Job-level outputs | Step-level outputs |
| Versioning | Branch/tag/SHA reference | Branch/tag/SHA reference |
| Max per workflow | 20 reusable workflow calls | No limit |

### Decision Rule

- Use **reusable workflows** to standardize full CI/CD pipelines across repositories (e.g., "every Go service runs this build-test-deploy pipeline").
- Use **composite actions** for shared step groups within a job (e.g., "set up tools + cache + lint" as a single action).

### Reusable Workflow Syntax

```yaml
# .github/workflows/reusable-build.yml
on:
  workflow_call:
    inputs:
      go-version:
        required: true
        type: string
    secrets:
      DEPLOY_TOKEN:
        required: false

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@<SHA>
      - uses: actions/setup-go@<SHA>
        with:
          go-version: ${{ inputs.go-version }}
```

```yaml
# Caller workflow
jobs:
  call-build:
    uses: ./.github/workflows/reusable-build.yml
    with:
      go-version: "1.22"
    secrets: inherit
```

### Composite Action Syntax

```yaml
# .github/actions/setup-tools/action.yml
name: Setup Tools
description: Install and cache development tools
inputs:
  go-version:
    required: true
runs:
  using: composite
  steps:
    - uses: actions/setup-go@<SHA>
      with:
        go-version: ${{ inputs.go-version }}
    - shell: bash
      run: go install golang.org/x/tools/cmd/goimports@latest
```

- Gotcha: Composite action steps MUST include `shell:` on every `run:` step. This is not required in normal workflows.

## Matrix Strategies

- Rule: Use `strategy.matrix` for multi-version/multi-OS testing. Set `fail-fast: false` to avoid canceling other matrix legs on first failure.
- Rule: Use `include` to add specific combinations and `exclude` to remove invalid ones.
- Gotcha: Matrix generates the cartesian product. 3 OS x 4 versions = 12 jobs. Be mindful of runner minute consumption.

For CI testing patterns (coverage, sharding, flaky tests, parallelization), see `skills/testing-patterns/references/ci-cd.md`.

## Conditional Execution

| Pattern | Syntax |
|---------|--------|
| Skip on draft PR | `if: github.event.pull_request.draft == false` |
| Only on main branch | `if: github.ref == 'refs/heads/main'` |
| Only on tag push | `if: startsWith(github.ref, 'refs/tags/')` |
| Run on failure | `if: failure()` |
| Always run (cleanup) | `if: always()` |
| Skip if label present | `if: !contains(github.event.pull_request.labels.*.name, 'skip-ci')` |
| Depend on job result | `if: needs.build.result == 'success'` |

- Rule: Use `if:` at the job level (not step level) when possible to skip entire jobs and save runner minutes.

## Concurrency Control

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

- Rule: Always set `concurrency` on CI workflows to cancel stale runs on the same branch.
- Gotcha: Do NOT use `cancel-in-progress: true` on deployment workflows. Canceling a half-finished deploy can leave infrastructure in a broken state. Use `cancel-in-progress: false` (queue behavior) for deploys.

## Job Dependencies

- Rule: Use `needs:` to express job ordering. Jobs without `needs:` run in parallel by default.
- Pattern: Fan-out/fan-in -- multiple test jobs in parallel, then a single deploy job that `needs: [test-go, test-js, lint]`.
