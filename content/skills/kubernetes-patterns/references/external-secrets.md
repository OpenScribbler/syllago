# External Secrets Operator (ESO)

Patterns for syncing secrets from external providers into Kubernetes Secrets.

## Components

| Component | Purpose |
|-----------|---------|
| SecretStore | Namespace-scoped connection to secret backend |
| ClusterSecretStore | Cluster-wide connection (platform-team managed) |
| ExternalSecret | Defines what secrets to fetch and how to store them |
| PushSecret | Push K8s secrets TO external providers (v1alpha1) |

## SecretStore (Vault Example)

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault
  namespace: my-app
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "my-app"
          serviceAccountRef:
            name: "my-app-sa"
```

For ClusterSecretStore, add `namespace` to `serviceAccountRef`. Use `conditions.namespaces` to restrict which namespaces can reference it.

## Provider Auth Summary

| Provider | Recommended Auth | Config Key |
|----------|-----------------|------------|
| Vault | Kubernetes auth | `vault.auth.kubernetes` |
| AWS SM | IRSA (IAM Roles for SA) | `aws.auth.jwt.serviceAccountRef` |
| Azure KV | Workload Identity | `azurekv.authType: WorkloadIdentity` |
| GCP SM | Workload Identity | `gcpsm.auth.workloadIdentity` |

All providers also support static credentials via `secretRef` (not recommended for production).

**AWS IAM permissions needed**: `secretsmanager:GetSecretValue`, `secretsmanager:ListSecrets`. Scope `Resource` to specific secret paths.

## ExternalSecret Patterns

### Basic Secret Fetch

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: my-secret
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault
    kind: SecretStore
  target:
    name: my-k8s-secret
    creationPolicy: Owner
  data:
    - secretKey: username
      remoteRef:
        key: apps/my-app/creds
        property: username
    - secretKey: password
      remoteRef:
        key: apps/my-app/creds
        property: password
```

### Fetch All Keys from One Path

Use `dataFrom.extract.key` instead of `data[]` to fetch all key-value pairs from a single remote path.

### Template Transformations

Build derived secrets (connection strings, config files, TLS certs) using Go templates in `target.template.data`:

```yaml
target:
  template:
    type: Opaque
    data:
      connection_string: |
        postgresql://{{ .username }}:{{ .password }}@{{ .host }}:{{ .port }}/{{ .database }}
```

**Special types**: `kubernetes.io/tls` with `pkcs12cert`/`pkcs12key` filters, `kubernetes.io/dockerconfigjson` for registry credentials.

## Refresh Intervals

| Use Case | Interval |
|----------|----------|
| Static secrets | 24h |
| Rotating credentials | 15m - 1h |
| Short-lived tokens | 5m - 15m |

Force refresh: `kubectl annotate es my-secret force-sync=$(date +%s) --overwrite`

## Security Best Practices

- Restrict who can create ExternalSecrets via RBAC (deny SecretStore management to app teams)
- Apply NetworkPolicy limiting ESO egress to secret providers + kube-dns only
- Disable unused providers: `--set extraArgs.disable-provider="alibaba,ibm,oracle,yandex"`
- Limit IAM/SA permissions to only required secret paths
- Use `deletionPolicy: Retain` to keep secrets if ExternalSecret is deleted

## Troubleshooting

```bash
kubectl describe externalsecret my-secret -n my-app    # Status and conditions
kubectl describe secretstore vault -n my-app            # Store validation
kubectl logs -n external-secrets deployment/external-secrets -f  # Controller logs
```

| Symptom | Fix |
|---------|-----|
| `SecretSyncedError` | Check SecretStore auth config |
| `SecretNotFound` | Verify remote ref path matches provider |
| Secret exists but empty | Check template syntax and data refs |
| No sync happening | Check `refreshInterval`, force sync |
