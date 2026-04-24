<!-- modeled after: DataDog/lading AGENTS.md -->
## Metrics

The metrics pipeline writes to the central collector. Every service emits
its own namespace so dashboards can mix and match without collisions.

| Metric | Type | Unit |
|---|---|---|
| requests_total | counter | req |
| request_duration | histogram | ms |
| queue_depth | gauge | items |
| errors_total | counter | err |

The collector scrapes every fifteen seconds. Missed scrapes show up as
gaps in dashboards; investigate any gap wider than one minute.

## Traces

Traces are sampled at one percent in production and one hundred percent
in staging. The trace header is propagated across every service hop.

| Header | Direction | Propagation |
|---|---|---|
| traceparent | request | in + out |
| tracestate | request | in + out |
| baggage | request | in + out |

When adding a new service, install the tracing middleware before any
other middleware so that every handler inherits the active span.

## Logs

Structured logs land in the central log store. Every log line includes
a level, a message, and a context map.

| Level | Use for | Volume expectation |
|---|---|---|
| debug | local only | N/A |
| info | normal operations | moderate |
| warn | recoverable issues | low |
| error | failed operations | rare |

Log retention is thirty days. Anything older is aggregated into daily
summaries and the raw lines are purged.

## Alerts

Alerts are defined declaratively alongside the service they cover.
Every alert has a runbook link; alerts without runbooks fail review.

| Severity | Response time | Paging |
|---|---|---|
| p0 | 5 minutes | always |
| p1 | 30 minutes | business hours |
| p2 | 4 hours | email only |
| p3 | best effort | ticket only |
