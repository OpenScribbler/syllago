<!-- modeled after: pingcap/tidb AGENTS.md -->
## Correctness

Code MUST pass the full test suite on every supported platform before
merge. Reviewers MUST block any PR that fails CI without a documented
override rationale.

Where the specification is ambiguous, contributors SHOULD prefer the
interpretation that aligns with prior art in the codebase. If prior art
is absent, open a design note in docs/plans/ and cite it in the PR.

Authors MAY propose deviations from existing conventions when the
deviation serves a well-scoped purpose. Deviations MUST include a
rationale in the PR description.

## Concurrency

Shared mutable state MUST be protected by a clearly-documented
synchronization primitive. Unsynchronized shared state is a bug, not a
style choice.

Long-running goroutines SHOULD accept a context.Context and honor its
cancellation. Goroutines that cannot be cancelled MUST be documented in
a comment explaining why.

Error propagation MAY use channels when the caller is a pipeline, and
SHOULD use return values otherwise.

## Security

Input from untrusted sources MUST be validated at the boundary. Code
that forwards untrusted input into a subprocess, a database query, or a
templated output MUST escape per the target's rules.

Secrets MUST NOT be logged. Log redaction is opt-in at the call site;
never rely on sinks to redact for you.

Contributors SHOULD favor well-reviewed libraries over hand-rolled
primitives for cryptography. Hand-rolled crypto MAY appear in tests or
in migration scripts, but MUST NOT ship in production.

## Performance

Hot paths MUST be covered by benchmarks. Regressions over five percent
block merge unless the PR documents a conscious trade-off.

Memory allocations in hot paths SHOULD be tracked. Allocations that
cannot be eliminated MAY be pooled. Pools MUST include a comment
explaining why pooling was chosen over a fresh allocation.
