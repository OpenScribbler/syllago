# Helm and Kustomize Patterns

Patterns for Helm chart development and Kustomize overlay management.

## Helm vs Kustomize

| Aspect | Helm | Kustomize |
|--------|------|-----------|
| Approach | Templating engine | Patch-based overlay |
| Parameterization | Powerful (conditionals, loops) | Limited (patches, generators) |
| Package distribution | Yes (chart repos, OCI) | No native packaging |
| Rollback | Built-in `helm rollback` | Via Git revert |
| Testing | Built-in test framework | No built-in |
| Built-in to kubectl | No | Yes (`kubectl -k`) |
| Best for | Third-party apps, complex params | Internal apps, env customization |

**Combination:** Helm for third-party charts + Kustomize for post-render customization.

## Helm Chart Structure

```
my-chart/
  Chart.yaml          # Metadata, dependencies
  values.yaml         # Defaults
  values.schema.json  # JSON Schema validation
  templates/
    _helpers.tpl      # Named template definitions
    deployment.yaml
    service.yaml
    NOTES.txt
  charts/             # Dependency subcharts
```

### Values Conventions

- Default to secure settings: `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `capabilities.drop: ["ALL"]`
- Never put secrets in values.yaml
- Use `values.schema.json` for input validation
- Flat values for simple `--set` override; nested for logical grouping

### Hooks

```yaml
annotations:
  "helm.sh/hook": pre-install,pre-upgrade
  "helm.sh/hook-weight": "-5"           # Lower runs first
  "helm.sh/hook-delete-policy": hook-succeeded
```

Types: `pre-install`, `post-install`, `pre-delete`, `post-delete`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `test`.

### Library Charts

`type: library` in Chart.yaml. Provides reusable template helpers without generating manifests. Use as dependency.

### Helm Production Rules

1. Always use `values.schema.json` for validation
2. Pin chart versions in dependencies (`Chart.lock`)
3. Include PDB, HPA, NetworkPolicy templates gated by values flags
4. Default to secure settings
5. Version charts independently from app versions (use `appVersion`)
6. Use `helm diff` before upgrades

### Testing

```bash
helm lint ./my-chart
helm template my-release ./my-chart        # Render templates
helm install my-release ./my-chart --dry-run
helm test my-release                       # Run test pods
helm diff upgrade my-release ./my-chart    # Preview changes (plugin)
```

## Kustomize Patterns

### Base/Overlay Structure

```
app/
  base/
    kustomization.yaml    # resources: [deployment.yaml, service.yaml]
    deployment.yaml
    service.yaml
  overlays/
    production/
      kustomization.yaml  # resources: [../../base], patches, configMapGenerator
      replica-count.yaml  # Strategic merge patch
```

### Production Overlay

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: [../../base]
namespace: production
namePrefix: prod-
labels:
  - pairs:
      environment: production
patches:
  - path: replica-count.yaml
configMapGenerator:
  - name: app-config
    behavior: merge
    literals: [LOG_LEVEL=warn]
```

### Components (Reusable Feature Modules)

Cross-cutting concerns (monitoring, network policies) as `kind: Component`:

```yaml
# overlays/production/kustomization.yaml
components:
  - ../../components/monitoring
  - ../../components/network-policies
```

### Kustomize Production Rules

- Enable content-hash suffixes on ConfigMaps/Secrets for automatic rollouts
- Structure overlays by environment
- Use components for cross-cutting features
- Combine with Helm for third-party charts
