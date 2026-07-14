# ACIF conformance reports

Runner-emitted report JSON from the latest full all-scope conformance
run of the syllago adapter (`cli/cmd/acif-adapter`) against the ACIF
suite. The report pins the suite's binding-set and catalog hashes
(compare against the ACIF repo's `conformance/suite-manifest.yaml`
entry) and records the adapter's handshake, per-vector results, and
claimed scopes.

These are impl #1's conformance claims per the runner DESIGN §8 — not
ACIF spec-graduation milestones, which are tracked upstream.

## Retention policy

Only the **latest** full report lives in HEAD; each new run replaces
it. Reports are derived artifacts — re-running the runner at the same
adapter and suite commits regenerates them byte-for-byte, and every
superseded report remains reachable in git history at the ledger's
commit. The ledger below is the durable claim history.

Naming: `acif-all-scope-report-<run date>-suite<N>.json`.

## Run ledger

| Date | Suite | Adapter protocol | Adapter commit | Result | binding_set |
|---|---|---|---|---|---|
| 2026-07-14 | 5 | 2 | `59181871` | 164 pass + 2 vacuous · 0 fail · all ten scopes | `c0d2accc6bb38f1927def449c0453a05c5817500202e0861ed8ef8f50c7abc2b` |
| 2026-07-14 | 3 | 1 | `b426d3ce` | 164 pass + 2 vacuous · 0 fail · all ten scopes | `1142a672f79a0865b935bb8eac0a619656f789eea81544ebfba05f1103d7e536` |
