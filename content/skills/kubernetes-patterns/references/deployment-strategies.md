# Deployment Strategies

Patterns for rolling updates, blue-green, canary, and progressive delivery.

## Strategy Comparison

| Strategy | Use Case | Complexity |
|----------|----------|------------|
| Rolling Update | Standard deployments, most services | Low |
| Recreate | Schema-incompatible upgrades, singletons | Low |
| Blue-Green | Need instant cutover/rollback, can afford 2x resources | Medium |
| Canary (manual) | Gradual rollout without extra tooling | Medium |
| Argo Rollouts | Automated canary with analysis gates | High |
| Flagger | Automated canary with service mesh | High |

## Rolling Update (Default)

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1            # Max pods above desired during update
      maxUnavailable: 0      # Zero-downtime: new must be ready before old terminates
  revisionHistoryLimit: 10
  minReadySeconds: 30        # Pod must be Ready 30s before considered Available
  progressDeadlineSeconds: 600
```

**Tuning profiles:**

| Profile | maxSurge | maxUnavailable | Behavior |
|---------|----------|----------------|----------|
| Zero-downtime | 1 | 0 | Slow, safe |
| Balanced (default) | 25% | 25% | Reasonable speed |
| Fast | 50% | 0 | Many new pods, no unavailability |
| Resource-constrained | 0 | 1 | No extra capacity needed |

**Recreate strategy:** Terminates all pods before creating new ones. Use only when app cannot run two versions simultaneously.

**Rollback:** `kubectl rollout undo deployment/my-app [--to-revision=3]`

## Blue-Green Deployment

Run two identical environments. Switch traffic atomically by updating the Service selector.

```yaml
# Two Deployments with labels version: blue / version: green
# Service selector points to one:
spec:
  selector:
    app: my-app
    version: blue    # Change to "green" to cut over
```

**Cutover:** `kubectl patch service my-app -p '{"spec":{"selector":{"version":"green"}}}'`

Tradeoffs: Instant cutover/rollback, but requires 2x resources, no gradual traffic shifting.

## Canary Deployment

### Replica-Based (Manual)

Two Deployments sharing a common label. Service selects both. Traffic split proportional to replica count (e.g., 9 stable + 1 canary = ~10% canary).

**Limitation:** Traffic split not configurable as percentage, only by replica ratio.

### StatefulSet Partition Canary

```yaml
spec:
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      partition: 2    # Only pods with ordinal >= 2 get the update
```

Lower partition progressively until 0 for full rollout.

## Progressive Delivery

### Argo Rollouts

Replaces Deployment with Rollout CRD supporting canary + analysis gates:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
spec:
  strategy:
    canary:
      steps:
        - setWeight: 20
        - pause: {duration: 5m}
        - setWeight: 40
        - pause: {duration: 5m}
        - setWeight: 80
        - pause: {duration: 5m}
      canaryService: my-app-canary
      stableService: my-app-stable
      analysis:
        templates:
          - templateName: success-rate
        startingStep: 2
```

### Flagger

Works with existing Deployments. Creates canary/primary copies automatically. Requires service mesh (Istio, Linkerd) or ingress controller for traffic splitting.
