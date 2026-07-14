# ACIF conformance reports

Runner-emitted report JSONs from full all-scope conformance runs of the
syllago adapter (`cli/cmd/acif-adapter`) against the ACIF suite. Each
report pins the suite's binding-set and catalog hashes (compare against
the ACIF repo's `conformance/suite-manifest.yaml` entry) and records the
adapter's handshake, per-vector results, and claimed scopes.

These are impl #1's conformance claims per the runner DESIGN §8 — not
ACIF spec-graduation milestones, which are tracked upstream.

Naming: `acif-all-scope-report-<run date>-suite<N>.json`.
