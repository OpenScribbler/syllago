# Kubernetes Networking

Patterns for NetworkPolicy, Service types, Ingress, DNS, and service mesh.

## NetworkPolicy

### Default Deny All (Foundation)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: production
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
```

### Allow Specific Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: api-allow-frontend
spec:
  podSelector:
    matchLabels:
      app: api
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: frontend
        - namespaceSelector:
            matchLabels:
              name: monitoring
      ports:
        - protocol: TCP
          port: 8080
```

**Critical gotcha:** In `from`/`to` arrays, items in the SAME element = AND logic; SEPARATE elements = OR logic. This subtle difference creates overly permissive policies.

### Allow Egress (with DNS)

```yaml
spec:
  podSelector:
    matchLabels:
      app: api
  policyTypes: [Egress]
  egress:
    - to:  # DNS -- ALWAYS include when restricting egress
        - namespaceSelector: {}
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
    - to:  # Database
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - protocol: TCP
          port: 5432
    - to:  # External HTTPS
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - protocol: TCP
          port: 443
```

## Service Types

| Type | Use Case | Key Detail |
|------|----------|------------|
| ClusterIP | Internal services | Default, stable endpoint |
| NodePort | External via node | Port range 30000-32767 |
| LoadBalancer | Cloud LB | Use annotations for NLB/ALB |
| Headless (`clusterIP: None`) | Direct pod IPs | For StatefulSets, service discovery |
| ExternalName | Alias to external DNS | CNAME redirect, no proxying |

## Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api-ingress
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
    - hosts: [api.example.com]
      secretName: api-tls
  rules:
    - host: api.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: api
                port:
                  number: 80
```

For path-based routing, order paths from most specific to least specific.

## DNS

```
api-service                                    # Same namespace
api-service.other-namespace                    # Cross-namespace
api-service.other-namespace.svc.cluster.local  # Fully qualified
```

## Service Mesh (Istio)

- **VirtualService**: Route traffic by headers, weight, or URI match to different subsets
- **DestinationRule**: Define subsets (v1, v2), connection pools, TLS settings

Use service mesh when you need: mTLS between services, traffic splitting by percentage, circuit breaking, or observability without code changes.

## Troubleshooting

```bash
kubectl run -it --rm debug --image=busybox -- nslookup api-service
kubectl run -it --rm debug --image=nicolaka/netshoot -- curl api-service:8080/health
kubectl get endpoints api-service
kubectl get networkpolicies -A
```
