## Prerequisites

- `oc` CLI configured and logged in to the target cluster
- `helm` v3 installed
- Aembit Tenant ID (from your tenant URL: `https://<tenantId>.aembit.io`)
- Outbound internet access from the cluster nodes to Aembit Cloud
- The deploying service account must have permission to use the `anyuid` Security Context Constraint (SCC)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{AGENT_CONTROLLER_ID}}` | Aembit Management Console → **Settings → Agent Controllers** → create or select a controller → **Controller ID** |

## Deployment

The Aembit Helm chart deploys two components to an OpenShift cluster: the **Agent Controller** (authenticates proxies to Aembit Cloud) and the **Sidecar Injector** (automatically adds the Agent Proxy container to annotated pods). OpenShift requires additional Security Context Constraint configuration and does not support transparent traffic steering. Deploy these before annotating application workloads.

### Part 1: Aembit Console — Create Agent Controller

1. In the Aembit Management Console, navigate to **Settings → Agent Controllers**

2. Click **+ New** and create a new Agent Controller
   - Select or create a **Kubernetes Service Account Trust Provider** for the cluster
   - Copy the **Controller ID**: `{{AGENT_CONTROLLER_ID}}`

### Part 2: Helm Deployment

1. Verify the deploying service account has `anyuid` SCC rights:

```bash
oc adm policy who-can use SecurityContextConstraints anyuid
```

Confirm your service account appears in the output before proceeding. If it does not, grant the SCC before continuing:

```bash
oc adm policy add-scc-to-user anyuid system:serviceaccount:aembit:default
```

Replace `aembit:default` with `<namespace>:<service-account-name>` if your deploying service account differs from the namespace default.

2. Add the Aembit Helm repository:

```bash
helm repo add aembit https://helm.aembit.io
helm repo update aembit
```

3. Create the namespace:

```bash
oc create namespace aembit
```

4. Install the chart with OpenShift-required flags:

```bash
helm install aembit aembit/aembit \
  -n aembit \
  --set tenant={{AEMBIT_TENANT_ID}},agentController.id={{AGENT_CONTROLLER_ID}} \
  --set serviceAccount.openshift.scc=anyuid,agentProxy.runAsRestricted=true
```

5. Verify both pods are running:

```bash
oc get pods -n aembit
```

Both `aembit-agent-controller-*` and `aembit-sidecar-injector-*` pods should show **Running** status.

### Part 3: Annotate Application Workloads

OpenShift does not support transparent traffic steering. Client workload pods must include both the inject annotation and the explicit steering annotation.

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
oc apply -f <your-deployment.yaml>
```

4. Confirm the sidecar was injected:

```bash
oc describe pod <pod-name> -n <your-namespace>
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

- **OpenShift pods fail to start:** The `anyuid` SCC is required for the Agent Controller and Sidecar Injector. Confirm the deploying service account has SCC permissions and that `--set serviceAccount.openshift.scc=anyuid` was included in the Helm install
- **Upgrading fails on OpenShift:** Avoid upgrading from a previously failed installation. Uninstall the chart (`helm uninstall aembit -n aembit`) and reinstall cleanly
- **Agent Controller shows Inactive:** The Trust Provider match rule does not match the cluster identity. Verify the Trust Provider configuration matches the node's identity (e.g., correct service account for OIDC-based trust)
- **Sidecar not injected into pods:** The pod annotation `aembit.io/agent-inject: "enabled"` is missing or applied to the Deployment metadata rather than the pod template (`spec.template.metadata.annotations`). Redeploy after correcting the annotation placement
- **TLS errors in application logs:** The Aembit Root CA is not trusted by the container. Download the Root CA from **Settings → Root CA** and mount it into the application container's trust store
