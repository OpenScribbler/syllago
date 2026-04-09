# Kubernetes Controller Framework Patterns

Patterns for building custom controllers using controller-runtime (Kubebuilder).

## Core Principles

| Principle | Description |
|-----------|-------------|
| Idempotent | Running reconcile multiple times produces the same result |
| Level-based | Respond to current state, not individual events |
| Single responsibility | One controller reconciles one Kind |

## Reconciliation Loop

```go
func (r *MyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var resource myv1.MyResource
    if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
        if apierrors.IsNotFound(err) {
            return ctrl.Result{}, nil // Deleted
        }
        return ctrl.Result{}, err
    }

    if !resource.DeletionTimestamp.IsZero() {
        return r.reconcileDelete(ctx, &resource)
    }

    if !controllerutil.ContainsFinalizer(&resource, finalizerName) {
        controllerutil.AddFinalizer(&resource, finalizerName)
        if err := r.Update(ctx, &resource); err != nil {
            return ctrl.Result{}, err
        }
    }

    return r.reconcileNormal(ctx, &resource)
}
```

## Return Value Patterns

| Pattern | Code | Behavior |
|---------|------|----------|
| Success | `return ctrl.Result{}, nil` | No requeue |
| Requeue with backoff | `return ctrl.Result{}, err` | Exponential backoff (5ms to 1000s) |
| Requeue after delay | `return ctrl.Result{RequeueAfter: 30*time.Second}, nil` | Fixed delay |
| Terminal error | `return ctrl.Result{}, reconcile.TerminalError(err)` | No requeue, error logged |

- Use `RequeueAfter` for periodic polling. Use error returns only for actual errors.

## Finalizers for Cleanup

Finalizers ensure cleanup of external resources before deletion. Without a finalizer, Kubernetes deletes the CR immediately and cleanup code never runs.

```go
func (r *MyReconciler) reconcileDelete(ctx context.Context, obj *myv1.MyResource) (ctrl.Result, error) {
    if controllerutil.ContainsFinalizer(obj, finalizerName) {
        if err := r.cleanupExternalResources(ctx, obj); err != nil {
            return ctrl.Result{}, err
        }
        controllerutil.RemoveFinalizer(obj, finalizerName)
        if err := r.Update(ctx, obj); err != nil {
            return ctrl.Result{}, err
        }
    }
    return ctrl.Result{}, nil
}
```

## Owner References

- `SetControllerReference`: for resources the controller owns (enables garbage collection + watch). Only ONE controller reference allowed per object.
- `SetOwnerReference`: for dependencies without controller relationship (multiple owners OK).

```go
if err := controllerutil.SetControllerReference(owner, configMap, r.Scheme); err != nil {
    return err
}
```

## Status Subresource

Always update status via the status subresource to avoid conflicts with spec updates.

```go
// +kubebuilder:subresource:status
type MyResourceStatus struct {
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
    Phase              string             `json:"phase,omitempty"`
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

// Use Status().Update() -- not Update()
meta.SetStatusCondition(&obj.Status.Conditions, condition)
return r.Status().Update(ctx, obj)
```

Standard condition types: `Ready`, `Progressing`, `Degraded`.

## Watch Setup

```go
func (r *MyReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&myv1.MyResource{}).           // Primary resource
        Owns(&corev1.ConfigMap{}).          // Owned resources (auto owner ref watch)
        Owns(&corev1.Secret{}).
        WithEventFilter(predicate.GenerationChangedPredicate{}). // Skip status-only updates
        WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
        Complete(r)
}
```

For watching external resources, use `Watches()` with `handler.EnqueueRequestsFromMapFunc` to map external objects to reconcile requests.

## Leader Election

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
    LeaderElection:   true,
    LeaderElectionID: "my-controller-leader-election",
})
```

Only the leader reconciles; other replicas wait on standby.

## Webhooks

### Validating Webhook

```go
// +kubebuilder:webhook:path=/validate-...,mutating=false,failurePolicy=fail,...
func (v *MyResourceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
    r := obj.(*myv1.MyResource)
    var allErrs field.ErrorList
    if r.Spec.Replicas < 1 {
        allErrs = append(allErrs, field.Invalid(
            field.NewPath("spec").Child("replicas"), r.Spec.Replicas, "must be at least 1"))
    }
    if len(allErrs) > 0 {
        return nil, apierrors.NewInvalid(schema.GroupKind{...}, r.Name, allErrs)
    }
    return nil, nil
}
```

### Mutating Webhook (Defaulting)

- Mutating webhooks must be idempotent. Set defaults only when fields are zero/empty.
- Use `field.Forbidden` for immutable field checks in `ValidateUpdate`.

## Conflict Handling

Use `retry.RetryOnConflict(retry.DefaultBackoff, func() error { ... })` for optimistic concurrency conflicts. Always re-fetch the latest version inside the retry loop.

## Event Recording

Record events via `r.Recorder.Event(obj, corev1.EventTypeNormal, "Reason", "message")` for visibility into controller actions. Setup: `mgr.GetEventRecorderFor("my-controller")`.

## RBAC Markers

```go
// +kubebuilder:rbac:groups=mygroup.example.com,resources=myresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mygroup.example.com,resources=myresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mygroup.example.com,resources=myresources/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
```
