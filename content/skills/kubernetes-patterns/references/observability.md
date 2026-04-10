# Observability Patterns

Patterns for metrics, logging, distributed tracing, and SLI/SLO.

## The Four Golden Signals (SRE)

| Signal | What to Measure | Example Metric |
|--------|----------------|----------------|
| Latency | Request duration (separate success/error) | `http_request_duration_seconds` histogram |
| Traffic | Demand on the system | `http_requests_total` counter |
| Errors | Rate of failed requests | `http_requests_total{status=~"5.."}` / total |
| Saturation | How "full" the system is | CPU/memory utilization, queue depth |

**USE Method** (infrastructure): Utilization (CPU%, memory%), Saturation (queue depth, pending pods), Errors (OOM kills, CrashLoopBackOff).

## Kubernetes Metrics Pipeline

| Component | Key Metrics |
|-----------|-------------|
| kube-apiserver | Request latency, rate, errors |
| kubelet/cadvisor | Container CPU/memory, pod status |
| kube-scheduler | Scheduling latency |

Metrics Server (in-memory, for HPA/VPA/`kubectl top`) vs Prometheus (persistent, alerting, dashboards).

## Logging Architecture

| Pattern | How | Best For |
|---------|-----|----------|
| Node-level DaemonSet | Fluent Bit/Promtail collects from `/var/log/pods` | Most deployments (recommended) |
| Streaming sidecar | Sidecar reads app log files, writes to stdout | Apps writing to files |
| Logging agent sidecar | Sidecar ships directly to backend | Maximum flexibility |

**Rules:** Write to stdout/stderr (not files). Use structured/JSON logging. Include correlation IDs. Deploy centralized collection.

**Stacks:** EFK (Elasticsearch + Fluent Bit + Kibana), PLG (Promtail + Loki + Grafana), cloud-native (CloudWatch/Stackdriver).

## Distributed Tracing (OpenTelemetry)

| Deployment | Use Case |
|-----------|----------|
| DaemonSet | Standard, shared infrastructure |
| Sidecar | Per-pod isolation (native sidecar with `restartPolicy: Always`) |
| Deployment | Gateway/aggregation layer |

## SLI/SLO

| SLI | Target Example |
|-----|----------------|
| Availability (% non-5xx) | 99.9% |
| Latency P99 | < 500ms |

```yaml
# Prometheus recording rules
- record: sli:availability:ratio
  expr: sum(rate(http_requests_total{status!~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
- record: sli:latency:p99
  expr: histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))
```

## Minimum Alerting Rules

| Alert | Condition | Severity |
|-------|-----------|----------|
| CrashLoopBackOff | `kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff"} > 0` | Warning |
| OOMKilled | `kube_pod_container_status_last_terminated_reason{reason="OOMKilled"} > 0` | Warning |
| High error rate | `sli:availability:ratio < 0.999` for 5m | Critical |
| High latency | `sli:latency:p99 > 0.5` for 5m | Warning |
| Node not ready | `kube_node_status_condition{condition="Ready",status="true"} == 0` | Critical |
| Pending pods | `kube_pod_status_phase{phase="Pending"} > 0` for 15m | Warning |
| PV low capacity | `available_bytes / capacity_bytes < 0.1` | Warning |
| Certificate expiry | `apiserver_client_certificate_expiration_seconds_bucket{le="604800"} > 0` | Warning |
