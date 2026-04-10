# Self-Hosted Runners

## When to Use Self-Hosted Runners

| Scenario | Use Self-Hosted? | Reason |
|----------|-----------------|--------|
| Public repository | NEVER | Forks can execute arbitrary code on your runner |
| Private repo, standard builds | Rarely | GitHub-hosted runners are simpler and sufficient |
| Private repo, special hardware (GPU, ARM) | Yes | GitHub-hosted lacks specialized hardware |
| Private repo, internal network access | Yes | Need to reach private APIs, databases, registries |
| Private repo, large builds (>14 GB RAM) | Yes | GitHub-hosted runner resource limits |
| Private repo, high volume (cost optimization) | Yes | Self-hosted can be cheaper at scale |

- Rule: NEVER use self-hosted runners on public repositories. Fork PRs can run arbitrary code on your runner, compromising the host, network, and secrets.
- Rule: Prefer GitHub-hosted runners unless you have a specific requirement they cannot meet.

## Security Hardening

- Rule: Run the runner process as a dedicated unprivileged user, never as root.
- Rule: Use ephemeral runners (single-job lifetime) to prevent state leakage between jobs. Configure with `--ephemeral` flag.
- Rule: Isolate runners on a dedicated network segment. Limit outbound access to required registries and APIs only.
- Rule: Do not store secrets on the runner filesystem. Use OIDC or GitHub Secrets, fetched at job runtime.
- Rule: Enable audit logging on runner hosts. Monitor for unexpected processes, network connections, and file changes.
- Rule: Harden the OS -- disable unnecessary services, apply security updates automatically, use read-only root filesystem where possible.

## Ephemeral and JIT Runners

### Ephemeral Runners

- Rule: Use `--ephemeral` flag when configuring the runner. The runner picks up one job, executes it, then de-registers. Prevents state leakage.
- Gotcha: Ephemeral runners re-register on each job. Requires automation (systemd, container orchestration) to replace them.

### Just-In-Time (JIT) Runners

- Rule: Use the GitHub REST API to create JIT runner configurations. The API returns a `jitconfig` blob that a fresh runner instance uses to register, run one job, and exit.
- Pattern: Orchestrator (Lambda, K8s Job, systemd timer) watches for `workflow_job` webhook events, provisions a runner with `jitconfig`, tears down after completion.
- Advantage: No persistent runner registration. Each runner exists only for the duration of one job.

## Actions Runner Controller (ARC)

Kubernetes-based autoscaling for self-hosted runners.

### Architecture

```
GitHub webhook --> ARC Controller --> Runner ScaleSet --> Runner Pods
                  (watches for      (creates ephemeral
                   workflow_job       pods per job)
                   events)
```

- Rule: Deploy ARC via Helm chart (`actions-runner-controller-2`). Use runner scale sets (v2 architecture) over the legacy runner deployment mode.
- Rule: Combine ARC with ephemeral runners (`containerMode.type: "dind"` or `"kubernetes"`) for job isolation.
- Rule: Set `minRunners: 0` and `maxRunners` based on expected concurrency. Scale-from-zero saves cost but adds ~30s cold start.

### Runner Modes

| Mode | Isolation | Use Case |
|------|-----------|----------|
| `dind` (Docker-in-Docker) | Container-level | Jobs that need `docker build` |
| `kubernetes` | Pod-level | Jobs that create K8s resources |
| Host (no container mode) | None | Legacy, avoid for new setups |

- Rule: Use `kubernetes` mode when jobs do not need Docker. It provides better isolation and faster startup than `dind`.
- Gotcha: `dind` mode requires privileged containers or a rootless Docker setup. Evaluate security implications for your environment.

## Runner Groups

- Rule: Separate runners into groups by environment (`dev-runners`, `staging-runners`, `prod-runners`). Assign groups to specific repositories or organizations.
- Rule: Use runner labels to route jobs: `runs-on: [self-hosted, linux, production]`. Combine with runner groups to enforce environment isolation.
- Gotcha: Runner group access is managed at the organization level. Repository admins cannot override group restrictions.

## Scaling Strategies

| Strategy | Mechanism | Best For |
|----------|-----------|----------|
| ARC scale sets | Webhook-driven pod creation | K8s-native teams, variable load |
| VM autoscaling | Cloud ASG/VMSS + runner registration | Non-K8s, cloud-hosted runners |
| JIT via API | Webhook + orchestrator + JIT config | Serverless-style, maximum isolation |
| Static pool | Fixed set of always-on runners | Predictable, steady workload |

- Rule: For Kubernetes environments, ARC with scale sets is the recommended approach. For cloud VMs without K8s, use cloud autoscaling groups with the runner application in a launch template.
- Rule: Monitor queue depth (pending workflow_job events) to tune scaling parameters. Long queue times indicate insufficient `maxRunners`.

## Cost Optimization

- Rule: Use scale-to-zero when workload is bursty. Accept the cold start tradeoff (~30-60s) for significant cost savings.
- Rule: Use spot/preemptible instances for CI runners. CI jobs are tolerant of interruption -- the workflow will retry on a new runner.
- Gotcha: Spot instance termination mid-job causes a job failure, not a retry. Use `retry` at the workflow level (`nick-fields/retry`) if needed.
