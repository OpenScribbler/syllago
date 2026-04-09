---
name: kubernetes-patterns
description: Kubernetes deployment patterns and best practices. Use when creating K8s manifests, debugging deployments, preparing services for production, reviewing for anti-patterns, configuring autoscaling, setting up GitOps, or designing multi-tenant clusters.
---

# Kubernetes Deployment Patterns

This skill provides patterns for deploying and managing services on Kubernetes.

## Production Readiness Quick Reference

| Category | Requirement |
|----------|-------------|
| Images | Pinned version tags, no `latest` |
| Resources | CPU requests + memory requests/limits set |
| Health | Liveness + readiness + startup probes |
| Replicas | >= 2 with PDB for production |
| Topology | Spread across zones and nodes |
| Signals | Handle SIGTERM/SIGINT gracefully |
| Security | Non-root, read-only FS, drop ALL capabilities, PSS enforced |
| Network | Default deny NetworkPolicy per namespace |
| Config | ConfigMaps for config, Secrets for credentials (volume-mounted) |
| Labels | `app.kubernetes.io/*` labels on all resources |
| GitOps | All changes via Git, not manual kubectl |

## Debugging Checklist

When a pod is not working:

1. **Check pod status**: `kubectl get pods -n <ns> | grep <name>`
2. **Check events**: `kubectl get events -n <ns> --sort-by='.lastTimestamp' | tail -20`
3. **Check logs**: `kubectl logs <pod> -n <ns> --tail=100`
4. **Check resources**: `kubectl top pod <name> -n <ns>`

Common issues: ImagePullBackOff (registry auth), CrashLoopBackOff (startup errors), Pending (resource requests), OOMKilled (memory limits).

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| Creating Deployments, Services, ConfigMaps, Ingress manifests | [manifests.md](references/manifests.md) |
| Building controllers/operators: controller-runtime, webhooks, finalizers | [controllers.md](references/controllers.md) |
| ESO: SecretStore, ExternalSecret, provider config (Vault, AWS, Azure, GCP) | [external-secrets.md](references/external-secrets.md) |
| NetworkPolicy (deny/allow), Service types, Ingress, DNS, service mesh | [networking.md](references/networking.md) |
| PVCs, StorageClasses, StatefulSet storage, snapshots, access modes | [storage.md](references/storage.md) |
| ServiceAccounts, Roles, ClusterRoles, RoleBindings, RBAC debugging | [rbac.md](references/rbac.md) |
| Anti-patterns: images, resources, probes, security, config, operations | [anti-patterns.md](references/anti-patterns.md) |
| Sidecar, ambassador, adapter, init container patterns | [pod-patterns.md](references/pod-patterns.md) |
| Rolling update, blue-green, canary, Argo Rollouts, Flagger | [deployment-strategies.md](references/deployment-strategies.md) |
| HPA, VPA, KEDA, Karpenter, QoS, CPU limits debate, right-sizing | [scaling.md](references/scaling.md) |
| Prometheus, logging architecture, tracing, SLI/SLO, alerting rules | [observability.md](references/observability.md) |
| Jobs, CronJobs, indexed jobs, work queues, pod failure policy | [jobs.md](references/jobs.md) |
| ArgoCD, Flux, repo structure, GitOps anti-patterns | [gitops.md](references/gitops.md) |
| Helm charts, Kustomize overlays, hooks, library charts | [helm-kustomize.md](references/helm-kustomize.md) |
| Namespace isolation, vCluster, quotas, cost optimization, spot nodes | [multi-tenancy.md](references/multi-tenancy.md) |

## Related Skills

- **Go patterns**: Load [go-patterns](../go-patterns/SKILL.md) for Go service implementation
- **Terraform/IaC**: Load [terraform-patterns](../terraform-patterns/SKILL.md) for infrastructure code
- **Code review**: Load [code-review-standards](../code-review-standards/SKILL.md) when reviewing K8s manifests
