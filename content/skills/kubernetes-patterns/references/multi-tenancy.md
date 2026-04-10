# Multi-Tenancy and Cost Optimization

Patterns for namespace isolation, resource fairness, virtual clusters, and cost management.

## Multi-Tenancy Models

| Model | Trust Level | Isolation | Example |
|-------|-------------|-----------|---------|
| Multiple teams | Medium (soft) | Namespace + RBAC + NetworkPolicy | Engineering teams within a company |
| Multiple customers | Low (hard) | vCluster or separate clusters | SaaS vendor serving customers |

## Decision Matrix

| Isolation Need | Solution | Overhead |
|----------------|----------|----------|
| Basic team separation | Namespaces + RBAC + NetworkPolicy | Low |
| Resource fairness | + ResourceQuota + LimitRange | Low |
| Policy inheritance | + Hierarchical Namespaces (HNC) | Medium |
| Strong API isolation | vCluster | Medium |
| Complete isolation | Separate clusters | High |

## Namespace Isolation (Foundation)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tenant-a
  labels:
    tenant: a
    pod-security.kubernetes.io/enforce: restricted
```

### Per-Tenant RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: tenant-a
  name: tenant-a-developer
rules:
  - apiGroups: ["", "apps", "batch"]
    resources: ["pods", "deployments", "services", "configmaps", "jobs"]
    verbs: ["get", "list", "create", "update", "delete"]
```

Bind to Group via RoleBinding. See `rbac.md` for binding examples.

### Per-Tenant ResourceQuota

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  namespace: tenant-a
  name: tenant-a-quota
spec:
  hard:
    requests.cpu: "10"
    requests.memory: 20Gi
    limits.cpu: "20"
    limits.memory: 40Gi
    pods: "100"
    persistentvolumeclaims: "20"
    requests.storage: 100Gi
```

### Per-Tenant Network Isolation

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  namespace: tenant-a
  name: tenant-isolation
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
  ingress:
    - from:
        - podSelector: {}          # Same namespace only
  egress:
    - to:
        - podSelector: {}
    - to:                          # Allow DNS
        - namespaceSelector: {}
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
```

### Node Isolation

Taint nodes for specific tenants: `kubectl taint nodes <node> tenant=a:NoSchedule`. Pods use `nodeSelector` + `tolerations`.

## Hierarchical Namespaces (HNC)

Children inherit RBAC, NetworkPolicies, ResourceQuotas from parents. Tenant admins can create sub-namespaces without cluster-admin.

## vCluster

Lightweight virtual K8s clusters inside a host namespace. Each runs its own API server. Use for: dev environments, CI/CD (ephemeral testing), strong isolation.

```bash
vcluster create tenant-a -n tenant-a-ns
vcluster connect tenant-a -n tenant-a-ns
```

## Cost Optimization

### Right-Sizing Workflow

1. Deploy VPA in `Off` mode (see `scaling.md`)
2. Observe 2-4 weeks
3. Set requests to P95 of usage, memory limits to 1.5-2x requests
4. Monitor for OOM kills and throttling

### Cost Strategies

| Strategy | Savings | Effort |
|----------|---------|--------|
| Set resource requests/limits on all pods | 10-30% | Low |
| Right-size with VPA recommendations | 20-40% | Medium |
| Spot/preemptible for tolerant workloads | 60-90% | Medium |
| HPA for variable workloads | 20-50% | Low |
| Scale to zero with KEDA | 70-100% idle | Medium |
| Karpenter bin-packing | 10-30% | Medium |
| Schedule-based scaling (KEDA cron) | 30-50% off-hours | Low |

**Spot/preemptible suitable for:** Batch jobs, CI/CD, stateless replicated services, dev/test.
**NOT suitable for:** Databases, single-replica services, long-running jobs without checkpoints.

### Target Utilization

| Metric | Target |
|--------|--------|
| CPU vs requests | 60-80% |
| Memory vs requests | 70-85% |
| Node utilization | 65-80% |

### Cost Checklist

- [ ] Resource requests on every container
- [ ] VPA in recommendation mode
- [ ] Spot for non-critical workloads
- [ ] HPA for web services, KEDA for event-driven
- [ ] Monthly review: unused PVCs, LoadBalancers, idle deployments
- [ ] Resource budgets per team/namespace
