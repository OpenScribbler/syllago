# Scaling and Resource Management

Patterns for HPA, VPA, KEDA, cluster autoscaling, QoS classes, quotas, and the CPU limits debate.

## Horizontal Pod Autoscaler (HPA)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  minReplicas: 2
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
        - type: Percent
          value: 100
          periodSeconds: 15
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Percent
          value: 25
          periodSeconds: 60
```

**Algorithm:** `desiredReplicas = ceil(currentReplicas * (currentMetric / targetMetric))`

**Critical:** Resource requests MUST be set (HPA calculates utilization as usage/request). Target 70-80%, not 100%.

**Metric types:** Resource (CPU/memory), Pods (per-pod custom), Object (single object), External (SQS queue, etc.).

## Vertical Pod Autoscaler (VPA)

Adjusts CPU/memory requests/limits based on observed usage.

| Mode | Behavior |
|------|----------|
| `Off` | Recommendations only (start here) |
| `Initial` | Apply at pod creation only |
| `Recreate` | Evict and recreate with new values |
| `Auto` | Currently same as Recreate; will support in-place resize |

**VPA + HPA conflict:** Do NOT combine on same CPU/memory metrics. Safe: HPA on custom metrics (RPS, queue depth) + VPA manages CPU/memory.

## KEDA (Event-Driven Autoscaling)

Extends HPA with 60+ external scalers. Supports scale-to-zero.

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
spec:
  scaleTargetRef:
    name: queue-processor
  idleReplicaCount: 0       # Scale to zero
  minReplicaCount: 1
  maxReplicaCount: 100
  fallback:
    failureThreshold: 3
    replicas: 6             # Fallback if metrics fail
  triggers:
    - type: rabbitmq
      metadata:
        queueName: tasks
        queueLength: "5"
```

| Feature | HPA | KEDA |
|---------|-----|------|
| Scale to zero | No (min 1) | Yes |
| Event-driven | Limited | 60+ scalers |
| Queue-based | Requires custom adapter | Native |

## Cluster Autoscaler vs Karpenter

| Feature | Cluster Autoscaler | Karpenter |
|---------|-------------------|-----------|
| Configuration | Pre-defined node groups | Dynamic NodePool constraints |
| Instance selection | Scales group count | Picks optimal instance per pod |
| Consolidation | Basic | Advanced bin-packing |
| Multi-cloud | All major | AWS native; others emerging |

## QoS Classes

| QoS Class | Criteria | Eviction Priority |
|-----------|----------|-------------------|
| Guaranteed | limits == requests for CPU and memory | Last evicted |
| Burstable | At least one request or limit set | Medium |
| BestEffort | No requests or limits | First evicted |

**Rule:** Production = Guaranteed or Burstable. Never BestEffort. Guaranteed also enables static CPU pinning.

## Resource Quotas and LimitRange

**ResourceQuota** (namespace-level): caps total requests/limits/pod count. When compute quotas exist, every Pod MUST specify requests and limits.

**LimitRange** (per-container defaults): sets `default`, `defaultRequest`, `min`, `max`. Use to auto-apply defaults when ResourceQuota is enforced.

See `multi-tenancy.md` for YAML examples.

## The CPU Limits Debate

### Against CPU Limits

- **CFS throttling causes latency spikes**: Container gets quota in 100ms periods. Burst through quota = throttled remainder, even if CPU is idle.
- **Historical kernel bug**: Fixed in kernel 5.4 (`512ac999`). Pre-5.4 throttled containers even under quota.
- **Wasted resources**: CPU is compressible. Throttling idle capacity wastes resources.

### For CPU Limits

- **Noisy neighbor**: Misbehaving workload (infinite loop) starves other pods.
- **Predictability**: Guaranteed QoS (requests == limits) enables static CPU pinning.
- **Multi-tenant**: Prevents one tenant impacting another.

### Pragmatic Guidance

| Scenario | Recommendation |
|----------|---------------|
| Single-tenant, trusted | CPU requests only, no CPU limits |
| Multi-tenant | CPU limits to prevent noisy neighbors |
| Latency-sensitive | Guaranteed QoS (requests == limits) |
| Batch/background | No CPU limits (use idle CPU) |
| **All scenarios** | **ALWAYS set CPU requests. ALWAYS set memory limits.** |

**Memory limits are NOT debated:** Memory is incompressible. Exceeding = OOM kill. Always set.

## Right-Sizing Strategy

1. Deploy VPA in `Off` mode
2. Observe 2-4 weeks
3. Set requests to P95 of usage
4. Set memory limits to 1.5-2x requests
5. Consider omitting CPU limits for latency-sensitive workloads
6. Monitor for OOM kills and CPU throttling

**Anti-pattern:** Memory overcommit (requests much lower than limits). When pods burst simultaneously, nodes OOM unpredictably. Do not overcommit memory >10-20%.
