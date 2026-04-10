## Prerequisites

- `kubectl` configured and pointing to the target EKS cluster (`kubectl config use-context <context>`)
- `helm` installed
- `aws` CLI configured with access to the EKS cluster
- Aembit Tenant ID (from your tenant URL: `https://<tenantId>.aembit.io`)
- Outbound internet access from the cluster to Aembit Cloud
- A Fargate profile exists that selects the `aembit` namespace (or a namespace/label selector that includes it) — pods scheduled to a namespace not covered by a Fargate profile will remain Pending

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AGENT_CONTROLLER_ID}}` | Aembit Management Console → **Settings → Agent Controllers** → create or select a controller → **Controller ID** |
| `{{EKS_CLUSTER_NAME}}` | AWS Console → **EKS → Clusters** → cluster name, or `aws eks list-clusters` |
| `{{AWS_REGION}}` | AWS Console → top-right region selector, or `aws configure get region` |

## Deployment

The Aembit Helm chart deploys two components to an EKS Fargate cluster: the **Agent Controller** (authenticates proxies to Aembit Cloud) and the **Sidecar Injector** (automatically adds the Agent Proxy container to annotated pods). EKS Fargate requires a Fargate profile that selects the `aembit` namespace and does not support transparent traffic steering. Deploy these before annotating application workloads.

### Part 1: Aembit Console — Create Agent Controller

1. In the Aembit Management Console, navigate to **Settings → Agent Controllers**

2. Click **+ New** and create a new Agent Controller
   - Select or create a **Kubernetes Service Account Trust Provider** for the cluster
   - Copy the **Controller ID**: `{{AGENT_CONTROLLER_ID}}`

### Part 2: Helm Deployment

1. Confirm a Fargate profile exists that selects the `aembit` namespace:

```bash
aws eks list-fargate-profiles \
  --cluster-name {{EKS_CLUSTER_NAME}} \
  --region {{AWS_REGION}}
```

If no profile covers `aembit`, create one before proceeding.

2. Add the Aembit Helm repository:

```bash
helm repo add aembit https://helm.aembit.io
helm repo update aembit
```

3. Create the namespace — this must match the namespace selector in your Fargate profile:

```bash
kubectl create namespace aembit
```

4. Install the chart:

```bash
helm install aembit aembit/aembit \
  -n aembit \
  --set tenant={{AEMBIT_TENANT_ID}},agentController.id={{AGENT_CONTROLLER_ID}}
```

5. Verify both pods are running:

```bash
kubectl get pods -n aembit
```

Both `aembit-agent-controller-*` and `aembit-sidecar-injector-*` pods should show **Running** status. On Fargate, pod startup takes longer than on EC2 nodes — allow up to two minutes.

### Part 3: Annotate Application Workloads

EKS Fargate does not support transparent traffic steering. Client workload pods must include both the inject annotation and the explicit steering annotation. The application pod's namespace must also be covered by a Fargate profile.

1. Configure the application to route outbound requests through the Aembit Agent Proxy by setting the `http_proxy` and `https_proxy` environment variables to `http://localhost:8000`.

2. For each Kubernetes Deployment (or Pod spec) that should receive the Aembit Agent Proxy sidecar, add the following annotations to the pod template:

```yaml
metadata:
  annotations:
    aembit.io/agent-inject: "enabled"
    aembit.io/steering-mode: "explicit"
```

3. Apply the updated manifest:

```bash
kubectl apply -f <your-deployment.yaml>
```

4. Confirm the sidecar was injected:

```bash
kubectl describe pod <pod-name> -n <your-namespace>
```

Look for a container named `aembit-agent-proxy` in the pod spec.

5. Remove any static credentials from the application's environment variables, secret mounts, or configuration files — the proxy injects authentication transparently.

## Verification

- Both `aembit-agent-controller` and `aembit-sidecar-injector` pods show **Running** in the `aembit` namespace
- The Agent Controller shows **Active** status in the Aembit Management Console under **Settings → Agent Controllers**
- Annotated application pods contain the `aembit-agent-proxy` sidecar container
- Application makes successful outbound API calls without static credentials
- Navigate to **Activity** in the Aembit Management Console and confirm credential injection events appear for the workload

## Troubleshooting

- **Pods remain Pending indefinitely:** No Fargate profile covers the namespace where the pods are scheduled. Create or update a Fargate profile to include the `aembit` namespace (and the application workload namespace if separate)
- **Sidecar not injected into pods:** The pod annotation `aembit.io/agent-inject: "enabled"` is missing or applied to the Deployment metadata rather than the pod template (`spec.template.metadata.annotations`). Redeploy after correcting the annotation placement
- **Application cannot reach target service:** Explicit steering is required on Fargate. Confirm `aembit.io/steering-mode: "explicit"` is present in the pod template annotations alongside the inject annotation
- **Agent Controller shows Inactive:** The Trust Provider match rule does not match the cluster identity. Verify the Trust Provider configuration matches the node's identity (e.g., correct AWS Account ID for the EKS cluster)
- **TLS errors in application logs:** The Aembit Root CA is not trusted by the container. Download the Root CA from **Settings → Root CA** and mount it into the application container's trust store
