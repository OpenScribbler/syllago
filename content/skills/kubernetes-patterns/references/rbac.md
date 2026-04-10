# Kubernetes RBAC

Patterns for ServiceAccounts, Roles, ClusterRoles, and RoleBindings.

## Resource Scope

| Resource | Scope | Purpose |
|----------|-------|---------|
| ServiceAccount | Namespace | Pod identity |
| Role | Namespace | Permission set |
| ClusterRole | Cluster | Cluster-wide permission set |
| RoleBinding | Namespace | Binds Role/ClusterRole to subjects |
| ClusterRoleBinding | Cluster | Binds ClusterRole cluster-wide |

**Built-in ClusterRoles:** `cluster-admin` (full access), `admin` (namespace-full), `edit` (read/write), `view` (read-only).

## ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-sa
  namespace: production
automountServiceAccountToken: true  # Set false if not needed
```

- Set `automountServiceAccountToken: false` when pods don't need K8s API access.
- Add `imagePullSecrets` for private registries.

## Role + RoleBinding (Namespace-Scoped)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: deployment-manager
  namespace: production
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
    resourceNames: ["app-secrets", "db-credentials"]  # Restrict to specific names
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-deployment-manager
  namespace: production
subjects:
  - kind: ServiceAccount
    name: app-sa
    namespace: production
roleRef:
  kind: Role
  name: deployment-manager
  apiGroup: rbac.authorization.k8s.io
```

Subjects can be: `ServiceAccount`, `User`, or `Group`.

## ClusterRole + ClusterRoleBinding

Use ClusterRoles for: cluster-scoped resources (nodes, PVs, namespaces), CRD management, or when a RoleBinding should reference a shared role across namespaces.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: myresource-manager
rules:
  - apiGroups: ["mycompany.io"]
    resources: ["myresources", "myresources/status"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

## Aggregated ClusterRoles

Add custom resources to built-in roles by labeling:

```yaml
metadata:
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"   # Adds to `view` role
    rbac.authorization.k8s.io/aggregate-to-edit: "true"   # Adds to `edit` role
```

## Common Patterns

### Operator ServiceAccount

Needs ClusterRole for CRD management + ClusterRoleBinding. Grant only specific verbs on specific resources. Avoid wildcards on core API groups.

### CI/CD Deployment Account

Namespace-scoped Role with `deployments: [get, list, patch, update]` and `configmaps: [get, list, create, update, patch]`. Bind from CI namespace to target namespace.

## Anti-Patterns

| Anti-Pattern | Fix |
|--------------|-----|
| Using cluster-admin for apps | Create specific roles |
| Wildcard resources `["*"]` | List specific resources |
| No resourceNames | Restrict to specific names when possible |
| automount when unused | Set `automountServiceAccountToken: false` |
| Shared ServiceAccounts | One SA per workload |

## Troubleshooting

```bash
kubectl auth can-i create deployments --namespace production
kubectl auth can-i list pods --as system:serviceaccount:production:app-sa
kubectl get roles,rolebindings -n production
kubectl get rolebindings,clusterrolebindings -A -o json | jq '.items[] | select(.subjects[]?.name=="my-sa")'
```
