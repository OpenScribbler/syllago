# GitOps Patterns

Patterns for ArgoCD, Flux, repository structure, and continuous delivery.

## ArgoCD vs Flux

| Feature | ArgoCD | Flux |
|---------|--------|------|
| Architecture | Centralized server + UI | Per-cluster controllers |
| UI | Rich web UI + CLI | CLI only (Weave GitOps for UI) |
| Multi-cluster | Single instance manages many | Flux installed per cluster |
| Multi-tenancy | AppProject-based RBAC | Namespace-scoped Kustomizations |
| Helm/Kustomize | Application source types | Native CRDs (HelmRelease, Kustomization) |
| Image automation | Argo Image Updater (separate) | Built-in |
| Resource footprint | Higher | Lower |

**Choose ArgoCD** when: team needs UI, managing many clusters centrally, needs SSO/RBAC.
**Choose Flux** when: team prefers CLI/GitOps-native, wants lightweight per-cluster install, needs native image automation.

## Repository Structure

### Monorepo (Small-Medium Orgs)

```
gitops-config/
  base/                    # Shared base manifests
    app-a/
  clusters/
    production/
      kustomization.yaml   # References base + production patches
    staging/
  infrastructure/          # cert-manager, ingress, etc.
```

### Multi-Repo (Large Orgs)

Separate `fleet-infra/` (cluster infrastructure) from per-app config repos. Use when >10-15 apps or distinct team ownership.

**Anti-pattern:** Environment-per-branch (main=prod, staging branch=staging). Makes promotion harder, creates merge conflicts, no clear diff between environments. Use directory-based overlays instead.

## ArgoCD Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/gitops-config.git
    targetRevision: main
    path: clusters/production/my-app
  destination:
    server: https://kubernetes.default.svc
    namespace: production
  syncPolicy:
    automated:
      prune: true       # Delete resources removed from Git
      selfHeal: true    # Revert manual changes
    syncOptions:
      - CreateNamespace=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        maxDuration: 3m
        factor: 2
```

**App of Apps:** Parent Application whose source directory contains child Application manifests.

**ApplicationSet:** Dynamic generation using generators (git directories, clusters, lists).

**Sync Waves:** `argocd.argoproj.io/sync-wave: "-1"` for ordering: namespaces (-2) > CRDs (-1) > operators (0) > apps (1).

## Flux Kustomization

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-app
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: gitops-config
  path: ./clusters/production/my-app
  prune: true
  targetNamespace: production
  dependsOn:
    - name: infrastructure
```

Flux HelmRelease: reference HelmRepository source + chart + version + values override.

## GitOps Anti-Patterns

| Anti-Pattern | Fix |
|---|---|
| Manual `kubectl apply` | All changes via Git PR |
| Environment-per-branch | Directory-based overlays |
| Storing secrets in Git | Sealed Secrets, SOPS, or ESO |
| No automated sync | Enable auto-sync with self-heal |
| Monolith repo for large org | Split to multi-repo |
