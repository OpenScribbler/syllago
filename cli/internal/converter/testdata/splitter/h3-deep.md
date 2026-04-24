<!-- modeled after: kubernetes/kops AGENTS.md -->
## Architecture

The control loop is the heart of the project. It reads desired state,
compares it to observed state, and plans the actions required to bring
the two together.

### Reconciler

The reconciler runs on every event and every scheduled tick. It is
idempotent, so running it twice yields the same result as once.

### Scheduler

The scheduler fans out reconciliation across workers. Each worker owns a
slice of the keyspace partitioned by stable hash of the resource name.

### Event Bus

The event bus delivers at-least-once. Handlers must tolerate duplicate
deliveries and should use an idempotency key when writing external state.

### State Store

State is persisted in a key-value store. The schema is flat; nested
structures are serialized as opaque blobs under a well-known prefix.

## Development Workflow

Local development mirrors production as closely as feasible. A single
docker-compose file brings up the dependencies the services need.

### Bootstrapping

Run ./scripts/bootstrap after cloning. It installs tooling, starts
dependencies, applies database migrations, and seeds example data.

### Running Tests

Tests run via make test at the repo root. Individual packages support
their own make targets for faster iteration.

### Linting

Lint and format on every save. The editor config in .editorconfig is the
source of truth for whitespace; the pre-commit hook enforces it.

### Debugging

The debug target starts the services with verbose logs and a debugger
port exposed on localhost. Use your IDE's attach-to-process feature to
step through code.

## Release

Releases happen on the first Tuesday of each month. The release captain
cuts a branch, tags the commit, and drives the rollout across environments.

### Canary

Every release goes to canary first. The canary tier sees ten percent of
traffic for at least one hour before promotion.

### Promotion

Promotion is gated on health signals from the canary. Any red signal
blocks promotion until the release captain approves an override with a
documented rationale.

### Rollback

Rollback is a one-command operation. The prior release's artifact is
kept hot for twenty-four hours after a rollout.
