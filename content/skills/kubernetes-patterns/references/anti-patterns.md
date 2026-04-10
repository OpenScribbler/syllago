# Kubernetes Anti-Patterns

Common anti-patterns across deployment, configuration, security, and operations.

## Deployment Anti-Patterns

### Using `latest` Tag
**Severity**: high

- Rule: Always use specific version tags (`v1.2.3`) or SHA digests. Never `latest`.
- Why: `latest` is mutable, breaks rollbacks, defaults to `imagePullPolicy: Always`, causes inconsistency across replicas.

### No Resource Limits
**Severity**: high

- Rule: ALWAYS set memory limits. ALWAYS set CPU/memory requests. CPU limits are debated -- see `scaling.md`.
- Why: No requests = bad scheduling. No limits = noisy neighbor + OOM kills. No requests = BestEffort QoS = first evicted.

### Missing Health Probes
**Severity**: high

- Rule: Every container needs liveness + readiness probes. Add startup probes for slow-starting apps.
- Gotcha: Liveness probes must NOT check external dependencies (causes cascading restarts when DB is down). Readiness probes SHOULD check dependencies.
- Fix: Startup probe `failureThreshold * periodSeconds` must exceed worst-case startup time.

### Single Replica in Production
**Severity**: high

- Rule: `minReplicas >= 2` for any service requiring availability. Use topology spread constraints.

### No Pod Disruption Budget
**Severity**: medium

- Rule: All production deployments with >1 replica need a PDB (`minAvailable` or `maxUnavailable`).
- Gotcha: PDBs only constrain voluntary disruptions (drain, scaling). Direct pod deletion bypasses PDBs -- only the Eviction API respects them.

### Not Handling SIGTERM
**Severity**: medium

- Rule: Handle SIGTERM to drain connections. Set `terminationGracePeriodSeconds` to match drain time.
- Fix: Add `preStop` hook with `sleep 5` to allow load balancer deregistration before shutdown begins.

### No Topology Spread Constraints
**Severity**: medium

- Rule: Spread pods across zones (`whenUnsatisfiable: DoNotSchedule`) and nodes (`whenUnsatisfiable: ScheduleAnyway`).

## Configuration Anti-Patterns

### Hardcoded Configuration
**Severity**: medium

- Rule: Use ConfigMaps for non-sensitive config. Volume mounts auto-update; env vars require pod restart. ConfigMaps have 1 MiB limit.

### Secrets in Environment Variables
**Severity**: high

- Rule: Mount secrets as volumes with `readOnly: true` and `defaultMode: 0400`. Better: use external secrets manager (Vault, ESO).
- Why: Env var secrets are visible in `kubectl describe pod`, `/proc/<pid>/environ`, crash dumps.

### Missing Labels
**Severity**: medium

- Rule: Apply `app.kubernetes.io/*` labels consistently: `name`, `instance`, `version`, `component`, `part-of`, `managed-by`.

### ConfigMap Misuse
**Severity**: medium

- Rule: Never store secrets in ConfigMaps. Use `immutable: true` in production. Env vars don't auto-update (use volume mounts).

## Security Anti-Patterns

### Running as Root
**Severity**: critical

- Rule: Containers must run as non-root with minimal capabilities. See `manifests.md` for security context template.
- Gotcha: If `runAsGroup` is omitted, primary group defaults to root (GID 0).

### No Network Policies (Flat Network)
**Severity**: high

- Rule: Default deny all traffic per namespace, then explicitly allow. See `networking.md`.
- Gotcha: Blocking egress without allowing DNS (port 53/UDP to kube-dns) breaks all service discovery.
- Gotcha: NetworkPolicy `from`/`to` arrays -- items in same element = AND; separate elements = OR.

### Overprivileged RBAC
**Severity**: high

- Rule: Never grant wildcards. Use namespace-scoped Roles over ClusterRoles. One ServiceAccount per workload.
- Dangerous permissions: `nodes/proxy`, `escalate`, `bind`, `impersonate`, `system:masters` group, `serviceaccounts/token` create, webhook config control.
- See `rbac.md` for correct patterns.

### Missing Pod Security Standards
**Severity**: high

- Rule: Enforce PSS `restricted` profile on production namespaces. Start with `audit` + `warn` before `enforce`.
- Three levels: Privileged (no restrictions) > Baseline (no hostNetwork/privileged/hostPath) > Restricted (non-root, drop ALL, read-only FS, seccomp).

### Exposed Dashboards
**Severity**: critical

- Rule: Never expose dashboards publicly. Use `kubectl proxy` or port-forward. Require authentication + RBAC.

## Operational Anti-Patterns

### No Monitoring or Alerting
**Severity**: high

- Rule: Minimum alerts -- OOM kills, CrashLoopBackOff, pending pods, node not ready, cert expiry, high error rate/latency. See `observability.md`.

### Poor Logging
**Severity**: medium

- Rule: Write to stdout/stderr (not files). Use structured/JSON logging. Deploy DaemonSet log collector. Include correlation IDs.

### Manual kubectl in Production
**Severity**: high

- Rule: GitOps with ArgoCD or Flux. All manifests in Git. Changes via PR. See `gitops.md`.

### No Backup Strategy
**Severity**: high

- Rule: Automated etcd snapshots, CRD backups, VolumeSnapshots for PVs, tested restore procedures.
