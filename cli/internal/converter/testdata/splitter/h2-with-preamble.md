<!-- modeled after: steadycursor/steadystart CLAUDE.md -->
This document collects the working agreements for the repository. Read it
before you start editing code; it is the shortest path to a passing pull
request. Updates land via normal PR; large changes deserve a design note in
docs/plans/ first.

## Branch Strategy

The main branch is always deployable. Feature work lives on short-lived
branches named feature/<short-slug>. Long-running branches are discouraged
because rebasing drift compounds quickly.

Hotfixes cut from main and merge back to main and develop. Tag the commit
with the patch version before deploying.

## Commit Messages

Use the Conventional Commits format. The type and scope should make the
diff navigable from git log alone. Keep the subject line under seventy-two
characters and wrap the body at eighty.

## Code Review

Every PR needs at least one reviewer from a different sub-team. If the
change touches shared infrastructure, add an infra reviewer too. Self-merge
is allowed only for documentation fixes.

Reviewers respond within one business day. If you cannot, flag it and
reassign rather than letting a PR sit.

## Testing Strategy

Unit tests live alongside the code they cover. Integration tests sit under
tests/integration and run only in CI by default. End-to-end tests require a
staging database and are gated behind a nightly job.

Every bug fix must land with a regression test. If writing the test is
hard, that is a signal to refactor first.

## Dependency Management

Lockfiles are committed. Upgrade dependencies in dedicated PRs so the diff
is easy to review. Pin the major version on anything that affects the
public API.

## Release Process

Releases cut from main on a two-week cadence. The release captain runs the
release checklist in docs/release.md and announces the new version in the
team channel.

## Observability

Every service emits structured logs and exports metrics to the central
collector. Add a trace around any network call that is not already
instrumented by the framework.

## Security

Never commit secrets. Use the vault integration for production credentials
and .env.example for local defaults. Report suspected vulnerabilities to
security@example.invalid and do not open a public issue.

## Documentation

Keep docs alongside the code they describe. When a behavior changes, update
the doc in the same commit. Orphaned documentation is worse than none.
