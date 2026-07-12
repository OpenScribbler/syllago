# Provider Capabilities

This directory is a **verbatim, attestation-verified mirror** of the Capability Feed published by the external [capmon](https://github.com/OpenScribbler/capmon) project at <https://openscribbler.github.io/capmon/>. Syllago is a consumer only: nothing here is edited by hand except the files on the keep-list below, and no capmon machinery runs in this repository.

## How it stays current

The **Capmon Pull** maintainer tool (`cli/cmd/capmon-pull`, core logic in `cli/internal/capfeed`) runs daily via [`.github/workflows/capmon-pull.yml`](../../.github/workflows/capmon-pull.yml):

1. Polls the feed's `v1/index.json` with a conditional GET (at most daily).
2. Verifies **fail-closed** before anything is written: SLSA provenance on the index (in-process sigstore-go, pinned to capmon's `publish.yml` workflow identity), then every file's `sha256` against the verified index. A tampered, unsigned, or stale (`generated_at` older than the feed's `max_staleness_hours`) feed writes nothing and turns the run red — the committed mirror is always last-known-good.
3. On a new `data_revision`, mirrors the feed byte-for-byte into this directory and force-updates the single rolling PR on the `automation/capmon-pull` branch.

Run it locally (no `gh` binary or token needed):

```bash
cd cli && go run ./cmd/capmon-pull -check          # verify + inspect only
cd cli && go run ./cmd/capmon-pull -repo-root ..   # full pull into this directory
```

## Directory contents

```
docs/provider-capabilities/
├── capabilities/<slug>.json   # Capability Documents (verbatim feed mirror)
├── by-content-type/*.json     # Feed views grouped by content type (verbatim)
├── schemas/, spec/            # Feed schemas + field-semantics spec (verbatim)
├── provenance.json            # Marker: data_revision + generated_at of the mirrored snapshot
├── compatibility-matrix.md    # Human-maintained summary (keep-list)
└── README.md                  # This file (keep-list)
```

Everything except the keep-list (`README.md`, `compatibility-matrix.md`) is owned by the mirror: the pull sweeps away files the feed no longer publishes. Do not hand-edit mirrored files — the next pull will overwrite them, and edited bytes would no longer match any attested hash.

`advisories.json` from the feed is deliberately **not** mirrored (out of scope for Capmon Pull).

## How the data is used

- **Review queue:** a Capability Document records capmon's proposed canonical key mappings per provider. Maintainers review these before mappings graduate into a [Provider Format Document](../provider-formats/) — the authoritative source syllago's converter reads. No Go code reads this directory for runtime behavior.
- **Coverage Drift:** the non-required `coverage-drift` CI job compares Go's `SupportsType` claims against the mirrored `content_types.<type>.supported` fields (`supported` absent = unknown = no finding). Red is a signal to reconcile, never a merge block. Reproduce locally:

  ```bash
  cd cli && SYLLAGO_COVERAGE_FEED=1 go test ./internal/provider/ -run TestCoverageFeedDrift
  ```

## Semantics

Consumption follows the feed's tolerant-reader contract (see the mirrored `spec/field-semantics.md`): unknown fields, files, and enum values are ignored, never errors; `supported` absent means unknown, never false.
