## Prerequisites

- `kubectl` configured and pointing to the target cluster (`kubectl config use-context <context>`)
- `helm` v3 installed
- Aembit Tenant ID (from your tenant URL: `https://<tenantId>.aembit.io`)
- Outbound internet access from the cluster nodes to Aembit Cloud

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AGENT_CONTROLLER_ID}}` | Aembit Management Console → **Settings → Agent Controllers** → create or select a controller → **Controller ID** |

## Deployment

The Aembit Helm chart deploys two components to the cluster: the **Agent Controller** (authenticates proxies to Aembit Cloud) and the **Sidecar Injector** (automatically adds the Agent Proxy container to annotated pods). Deploy these before annotating application workloads.

### Part 1: Aembit Console — Create Agent Controller

1. In the Aembit Management Console, navigate to **Settings → Agent Controllers**

2. Click **+ New** and create a new Agent Controller
   - Select or create a **Kubernetes Service Account Trust Provider** for the cluster
   - Copy the **Controller ID**: `{{AGENT_CONTROLLER_ID}}`

### Part 2: Helm Deployment

1. Add the Aembit Helm repository:

```bash
helm repo add aembit https://helm.aembit.io
helm repo update aembit
```

2. Create the namespace:

```bash
kubectl create namespace aembit
```

3. Install the chart:

```bash
helm install aembit aembit/aembit \
  -n aembit \
  --set tenant={{AEMBIT_TENANT_ID}},agentController.id={{AGENT_CONTROLLER_ID}}
```

4. Verify both pods are running:

```bash
kubectl get pods -n aembit
```

Both `aembit-agent-controller-*` and `aembit-sidecar-injector-*` pods should show **Running** status.

### Part 3: Annotate Application Workloads

1. For each Kubernetes Deployment (or Pod spec) that should receive the Aembit Agent Proxy sidecar, add the following annotation to the pod template:

```yaml
metadata:
  annotations:
    aembit.io/agent-inject: "enabled"
```

2. Apply the updated manifest:

```bash
kubectl apply -f <your-deployment.yaml>
```

3. Confirm the sidecar was injected (replace `<your-namespace>` with the application workload's namespace, not the `aembit` system namespace):

```bash
kubectl describe pod <pod-name> -n <your-namespace>
```

Look for a container named `aembit-agent-proxy` in the pod spec.

4. Remove any static credentials from the application's environment variables, secret mounts, or configuration files — the proxy injects authentication transparently.

## Verification

- Both `aembit-agent-controller` and `aembit-sidecar-injector` pods show **Running** in the `aembit` namespace
- The Agent Controller shows **Active** status in the Aembit Management Console under **Settings → Agent Controllers**
- Annotated application pods contain the `aembit-agent-proxy` sidecar container
- Application makes successful outbound API calls without static credentials
- Navigate to **Activity** in the Aembit Management Console and confirm credential injection events appear for the workload

## Troubleshooting

- **Agent Controller shows Inactive:** The Trust Provider match rule does not match the cluster identity. Verify the Trust Provider configuration matches the node's identity (e.g., correct AWS Account ID for EKS, correct service account for OIDC-based trust)
- **Sidecar not injected into pods:** The pod annotation `aembit.io/agent-inject: "enabled"` is missing or applied to the Deployment metadata rather than the pod template (`spec.template.metadata.annotations`). Redeploy after correcting the annotation placement
- **TLS errors in application logs:** The Aembit Root CA is not trusted by the container. Download the Root CA from **Settings → Root CA** and mount it into the application container's trust store
- **Helm install fails: namespace already exists:** A previous installation attempt left the `aembit` namespace. Run `helm uninstall aembit -n aembit` and `kubectl delete namespace aembit` before reinstalling
