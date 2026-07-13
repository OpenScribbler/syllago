# Plan: capmon-pull

## Execution order

1. **Slice 1: Polite Capability Feed polling with tolerant index read** — test bead + impl bead (TDD)
   - Test: `cli/internal/capfeed/fetch_test.go`, `cli/internal/capfeed/index_test.go`, `cli/cmd/capmon-pull/main_test.go` — assert conditional GET (`If-None-Match` echo, 304 → NotModified), User-Agent/Accept headers, size cap + bad-status errors, tolerant index decode (unknown fields retained, missing required fields error), `-check` prints `data_revision`/`generated_at` and exits non-zero on malformed index
   - Impl: `cli/internal/capfeed/fetch.go`, `cli/internal/capfeed/index.go`, `cli/cmd/capmon-pull/main.go` — satisfies tests
   - Checkpoint: `bash .ship/capmon-pull/checks/slice1.sh`

2. **Slice 2: Fail-closed provenance verification and staleness gate** — test bead + impl bead (TDD)
   - Test: `cli/internal/capfeed/attest_test.go`, `cli/internal/capfeed/freshness_test.go`, `cli/cmd/capmon-pull/main_test.go` — assert valid `testdata/feedsnapshot` verifies; tampered index, wrong signer identity, no usable bundle each error; attestations-API request shape (path, headers, no auth); 48h freshness boundaries; `-check` fails closed on tamper printing nothing trusted
   - Impl: `cli/internal/capfeed/attest.go`, `cli/internal/capfeed/freshness.go`, `cli/cmd/capmon-pull/main.go` (verification-gated `-check` sequencing, trusted root via `moat.BundledTrustedRoot(...).Bytes` read-only) — satisfies tests
   - Checkpoint: `bash .ship/capmon-pull/checks/slice2.sh`

3. **Slice 3: Verified verbatim mirror of Capability Documents** — test bead + impl bead (TDD)
   - Test: `cli/internal/capfeed/files_test.go`, `cli/internal/capfeed/mirror_test.go`, `cli/cmd/capmon-pull/main_test.go` — assert per-file hash mismatch aborts with nil map; all-verified completeness; unknown listed files mirrored verbatim; sweep retires unmanaged files but keeps keep-list (`README.md`, `compatibility-matrix.md`); `provenance.json` marker + changed-provider diff; end-to-end pull writes mirror, corrupted file leaves tree unchanged
   - Impl: `cli/internal/capfeed/files.go`, `cli/internal/capfeed/mirror.go`, `cli/cmd/capmon-pull/main.go` (full-pull wiring) — satisfies tests
   - Checkpoint: `bash .ship/capmon-pull/checks/slice3.sh`

4. **Slice 4: Change detection, ETag persistence, and automation summary** — test bead + impl bead (TDD)
   - Test: `cli/internal/capfeed/run_test.go`, `cli/internal/capfeed/marker_test.go`, `cli/cmd/capmon-pull/main_test.go` — assert `data_revision`-match and 304 short-circuits (request-counted, no writes); ETag round-trip; verification precedes marker compare (tampered index with matching revision still fails); summary JSON shape; missing/tolerant marker decode; second run is a no-op
   - Impl: `cli/internal/capfeed/run.go`, `cli/internal/capfeed/marker.go`, `cli/cmd/capmon-pull/main.go` (thin shim over `capfeed.Run`) — satisfies tests
   - Checkpoint: `bash .ship/capmon-pull/checks/slice4.sh` (includes ≥80% coverage gate on `cli/internal/capfeed`)

5. **Slice 5: Coverage Drift findings from mirrored Capability Documents** — test bead + impl bead (TDD)
   - Test: `cli/internal/provider/coverage_feed_test.go` — assert contradiction tables (feed true vs Go false and vice versa → finding; absent → none; no doc → none; unknown fields ignored; malformed JSON → error); `CheckCoverage` integration returns findings tagged `go-vs-capability-feed`; `TestCoverageFeedDrift` gated on `SYLLAGO_COVERAGE_FEED=1`
   - Impl: `cli/internal/provider/coverage_feed.go`, `cli/internal/provider/coverage.go` (fifth constant + call site), `.github/workflows/ci.yml` (non-required `coverage-drift` job) — satisfies tests
   - Checkpoint: `bash .ship/capmon-pull/checks/slice5.sh`

6. **Slice 6: Daily Capmon Pull cron with rolling PR and consume-only docs** — test bead + impl bead (structural checks; workflow YAML + prose)
   - Test: `.ship/capmon-pull/checks/slice6.sh` structural assertions — workflow hygiene (cron + dispatch, concurrency, `contents: read`, full-SHA pins, `persist-credentials: false`, `--body-file`, no feed-derived `${{ }}` interpolation, `actions/cache` for ETag); stale-doc greps (no `syllago capmon`, `cli/internal/capmon/`, `capmon.yml`, `.capmon-pause`); `go build ./... && go vet ./...`
   - Impl: `.github/workflows/capmon-pull.yml`, `docs/provider-capabilities/README.md`, `CONTRIBUTING.md`, `docs/guides/adding-a-provider.md` — satisfies checks
   - Checkpoint: `bash .ship/capmon-pull/checks/slice6.sh` (plus the 5-step manual rolling-PR verification in structure.md, which requires the Aembit credential provisioned first — design DQ1)

## Gate

Before moving from one slice to the next: Checkpoint for the current slice must pass. If it fails, stop and involve the user — never skip ahead.

## Acceptance

- Polls `v1/index.json` at most daily with conditional GET; change detection via `data_revision` — Slices 1 & 4 tests (`TestFetch_ConditionalGET`, `TestRun_RevisionMatchShortCircuits`, `TestRun_NotModified304ShortCircuits`), cron in Slice 6
- Fail-closed verification: SLSA provenance on `v1/index.json` (pinned signer `OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main`), then per-file sha256, before acting; any mismatch aborts with nothing written — Slice 2 & 3 tests (`TestVerifyFeedProvenance_*`, `TestFetchFeedFiles_HashMismatchAborts`, `TestMain_PullWritesVerifiedMirror`)
- Stale feed (`generated_at` > 48h) keeps last-known-good and exits non-zero — Slice 2 (`TestCheckFreshness_Boundaries`, ordering after verification per design)
- Tolerant reader: unknown fields/files/keys ignored, open enums, `supported` absent = unknown — Slices 1, 3, 5 (`TestParseIndex_TolerantUnknownFields`, `TestWriteMirror_VerbatimIncludingUnknownFiles`, `TestCheckFeedCoverage_Contradictions`)
- Capability changes produce a single rolling syllago PR with normal CI via the Aembit credential — Slice 6 manual verification
- Go-claims-vs-data Coverage Drift check runs as a non-required PR check and locally via `SYLLAGO_COVERAGE_FEED=1` — Slice 5 checkpoint
- 80%+ test coverage on `cli/internal/capfeed` — Slice 4 checkpoint coverage gate; `cli/internal/moat/` byte-identical (imported read-only)
- Repo conventions honored: httptest for HTTP, no mocking libraries, table-driven tests, `cd cli && make fmt` before commits, `make build` after Go changes

## Non-TDD exemptions

None
