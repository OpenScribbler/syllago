# Kubernetes Storage

Patterns for PVCs, StorageClasses, volume types, and StatefulSet storage.

## Access Modes

| Mode | Abbrev | Description |
|------|--------|-------------|
| ReadWriteOnce | RWO | Single node read-write |
| ReadOnlyMany | ROX | Multi-node read-only |
| ReadWriteMany | RWX | Multi-node read-write (NFS, EFS, etc.) |
| ReadWriteOncePod | RWOP | Single pod read-write (K8s 1.22+) |

## Reclaim Policies

| Policy | Behavior |
|--------|----------|
| Delete | PV deleted when PVC deleted (default for dynamic) |
| Retain | PV kept, manual cleanup needed (use for important data) |

## PersistentVolumeClaim

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: fast-ssd
  resources:
    requests:
      storage: 10Gi
```

## StorageClass (Cloud Examples)

```yaml
# AWS EBS (gp3)
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  encrypted: "true"
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```

Other provisioners: `pd.csi.storage.gke.io` (GCP, `type: pd-ssd`), `disk.csi.azure.com` (Azure, `skuName: Premium_LRS`).

**Rule:** Always use `volumeBindingMode: WaitForFirstConsumer` for topology-aware provisioning.

## Volume Types

| Type | Use Case | Key Detail |
|------|----------|------------|
| `emptyDir` | Scratch/cache/shared between containers | Lost on pod deletion; use `sizeLimit` |
| `emptyDir` + `medium: Memory` | tmpfs (RAM-backed) | Fast but counts against memory limits |
| ConfigMap volume | Mount config files | Auto-updates with delay; use `readOnly: true` |
| Secret volume | Mount credentials | Use `readOnly: true`, `defaultMode: 0400` |
| PVC | Persistent data | Survives pod restarts |

## StatefulSet with Storage

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: postgres
  replicas: 3
  template:
    spec:
      containers:
        - name: postgres
          image: postgres:15
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: [ReadWriteOnce]
        storageClassName: fast-ssd
        resources:
          requests:
            storage: 100Gi
```

Each replica gets its own PVC (`data-postgres-0`, `data-postgres-1`, etc.). PVCs persist across pod restarts and rescheduling.

## Volume Expansion

Set `allowVolumeExpansion: true` on StorageClass. Edit PVC `spec.resources.requests.storage` to increase. Some drivers require pod restart.

## Snapshots

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: app-data-snapshot
spec:
  volumeSnapshotClassName: csi-snapclass
  source:
    persistentVolumeClaimName: app-data
```

Restore: create PVC with `dataSource` referencing the snapshot.

## Troubleshooting

```bash
kubectl get pvc -A                    # PVC status
kubectl describe pvc app-data         # Stuck PVC details
kubectl get storageclass              # Available classes
kubectl get events --field-selector reason=ProvisioningFailed
```

## Anti-Patterns

| Anti-Pattern | Fix |
|--------------|-----|
| No size limits on emptyDir | Set `sizeLimit` to prevent disk fill |
| Wrong access mode | Match mode to use case (RWO vs RWX) |
| Delete reclaim for important data | Use Retain policy |
| No backup strategy | Implement VolumeSnapshots + off-cluster backups |
