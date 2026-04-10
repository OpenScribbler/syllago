# Pod Design Patterns

Multi-container pod patterns: sidecars, ambassadors, adapters, and init containers.

## Pattern Overview

| Pattern | Purpose | Lifecycle |
|---------|---------|-----------|
| Sidecar | Extend/enhance main container | Runs alongside |
| Ambassador | Proxy outbound connections | Runs alongside |
| Adapter | Normalize output/interfaces | Runs alongside |
| Init Container | One-time setup before main starts | Runs to completion, then exits |

## Decision Tree

```
Need to run before main container?
  Yes, one-time setup --> Init Container
  No, runs alongside
    Proxies outbound connections --> Ambassador
    Transforms output format --> Adapter
    Adds cross-cutting capability --> Sidecar
```

## Native Sidecar (Kubernetes 1.28+)

Init containers with `restartPolicy: Always`. Start before regular containers, run for pod lifetime.

```yaml
initContainers:
  - name: log-collector
    image: fluent-bit:latest
    restartPolicy: Always      # Makes this a native sidecar
    volumeMounts:
      - name: logs
        mountPath: /var/log/app
containers:
  - name: app
    image: myapp:v1.0
    volumeMounts:
      - name: logs
        mountPath: /var/log/app
```

**Advantages over legacy sidecars:** Guaranteed startup order (sidecar before main), guaranteed shutdown order (sidecar outlives main), sidecar completion doesn't affect Job completion.

### Common Sidecar Use Cases

| Use Case | Sidecar | Function |
|----------|---------|----------|
| Logging | Fluent Bit, Promtail | Ship logs to central system |
| Metrics | OTEL Collector | Export metrics/traces |
| Service mesh | Envoy, Linkerd proxy | mTLS, traffic management |
| Secrets | Vault agent | Inject/rotate secrets |
| Config | Config reloader | Watch and reload config |

## Ambassador Pattern

Main container connects to localhost; ambassador proxies to external service.

```yaml
containers:
  - name: app
    env:
      - name: DB_HOST
        value: "localhost"       # Connects to ambassador
  - name: db-proxy
    image: cloud-sql-proxy:latest
    args: ["--port=5432", "--credentials-file=/secrets/sa.json", "project:region:instance"]
```

Use cases: Cloud SQL Proxy, Redis proxy, connection pooling.

## Adapter Pattern

Transforms main container output to match expected interface (e.g., custom log format to JSON, JMX to Prometheus). Share data via `emptyDir` volume.

## Init Container Pattern

Run one-time setup tasks sequentially before main containers start. All must succeed (exit 0).

```yaml
initContainers:
  - name: wait-for-db
    image: busybox:1.28
    command: ['sh', '-c', 'until nslookup postgres; do sleep 2; done']
  - name: db-migrate
    image: myapp-migrations:v1.0
    command: ['./migrate', 'up']
```

| Use Case | Example |
|----------|---------|
| Wait for dependencies | DNS lookup, TCP check, HTTP health |
| Database migrations | Schema updates before app starts |
| Config generation | Template rendering, secret fetching |
| Data initialization | Clone repo, download assets |
| Permission setup | `chown`/`chmod` on mounted volumes |

**Anti-patterns:** Don't use init containers for long-running tasks (use sidecars), continuous monitoring (use sidecars), or heavy downloads on every restart (use PVs).

## Resource Considerations

- **Init containers:** Effective init request = max of all init containers (sequential). Pod request = max(sum of app containers, max of init containers).
- **Sidecar containers:** Count toward pod totals. Size appropriately.
- **Native sidecars:** Init resource accounts for these running concurrently with regular containers.
