# Job and CronJob Patterns

Patterns for batch processing, parallel execution, and scheduled workloads.

## Job Execution Patterns

| Pattern | completions | parallelism | Use Case |
|---------|-------------|-------------|----------|
| Single job | 1 (default) | 1 (default) | One-off task |
| Fixed completion | N | M | Process exactly N items |
| Work queue | unset | M | Process items from shared queue |
| Indexed | N | M | Static work assignment per pod |

## Indexed Job

Each pod receives `JOB_COMPLETION_INDEX`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: batch-processor
spec:
  completions: 10
  parallelism: 5
  completionMode: Indexed
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: worker
          image: batch-worker:v1.0
          command: ["./process", "--partition=$(JOB_COMPLETION_INDEX)", "--total=10"]
```

Use cases: Video rendering (frame ranges), data partitioning, parallel testing.

## TTL Cleanup

**Always set** `ttlSecondsAfterFinished`. Without it, completed Jobs and Pods accumulate forever.

## Pod Failure Policy

Distinguish retriable from non-retriable failures:

```yaml
spec:
  backoffLimit: 6
  podFailurePolicy:
    rules:
      - action: FailJob
        onExitCodes:
          containerName: main
          operator: In
          values: [1, 2, 3]           # Non-retriable exit codes
      - action: Ignore
        onPodConditions:
          - type: DisruptionTarget    # Node drain -- don't count
```

## CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-backup
spec:
  schedule: "0 2 * * *"
  timeZone: "America/New_York"
  concurrencyPolicy: Forbid            # Don't overlap runs
  startingDeadlineSeconds: 200
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 1
  jobTemplate:
    spec:
      backoffLimit: 3
      ttlSecondsAfterFinished: 86400
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: backup
              image: backup-tool:v1.0
```

### Concurrency Policies

| Policy | Behavior |
|--------|----------|
| Allow (default) | Multiple concurrent runs |
| Forbid | Skip new if previous running |
| Replace | Cancel running, start new |

### CronJob Rules

- `concurrencyPolicy: Forbid` for jobs that shouldn't overlap (backups, reports)
- Always set `ttlSecondsAfterFinished` on jobTemplate
- Jobs must be idempotent -- K8s may create two Jobs or zero in edge cases
- CronJob name max: 52 characters (controller appends 11; Job name limit = 63)

## Quick Reference

| Setting | Recommendation |
|---------|---------------|
| `backoffLimit` | 3-6 for most workloads |
| `activeDeadlineSeconds` | Set for runaway protection |
| `ttlSecondsAfterFinished` | Always set (3600-86400) |
| `completionMode: Indexed` | When work is pre-sharded |
