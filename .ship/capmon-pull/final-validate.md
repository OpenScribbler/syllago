# Final Validate — capmon-pull

Full build: `cd cli && make build && go build ./... && go vet ./...` → PASS.
Full suite: `cd cli && go test ./...` → PASS, 33 packages ok, 0 skipped. (One repo-wide lint, `TestNoRawHashFormatting`, initially flagged the GitHub-API URL segment `"sha256:" +` in `capfeed/attest.go`; added to the lint's existing out-of-domain allowlist alongside the MOAT-artifact entries — it is API URL construction, not library-rule hash storage.)
Binary reinstalled to PATH per repo rules: `cp cli/syllago ~/.local/bin/syllago`.

## Acceptance criteria checked

- [x] Polls `v1/index.json` at most daily with conditional GET; change detection via `data_revision` — Slices 1 & 4 tests (`TestFetch_ConditionalGET`, `TestRun_RevisionMatchShortCircuits`, `TestRun_NotModified304ShortCircuits`), cron in Slice 6
  - Command: `bash .ship/capmon-pull/checks/slice1.sh && bash .ship/capmon-pull/checks/slice4.sh` and `grep -A2 'schedule:' .github/workflows/capmon-pull.yml`
  - Observed: both checkpoints green; slice4 prints `capfeed coverage: 81.0%`; the named tests pass (If-None-Match echo asserted server-side, 304 run makes 0 attestation + 0 per-file requests, revision-match run makes 0 per-file requests); workflow cron is `0 13 * * *` (daily) + `workflow_dispatch`
  - Evidence: `.ship/capmon-pull/checks/slice1.sh`, `checks/slice4.sh`, `cli/internal/capfeed/fetch_test.go`, `run_test.go`, `.github/workflows/capmon-pull.yml:22-26`

- [x] Fail-closed verification: SLSA provenance on `v1/index.json` (pinned signer `OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main`), then per-file sha256, before acting; any mismatch aborts with nothing written — Slice 2 & 3 tests (`TestVerifyFeedProvenance_*`, `TestFetchFeedFiles_HashMismatchAborts`, `TestMain_PullWritesVerifiedMirror`)
  - Command: `cd cli && go test ./internal/capfeed/ ./cmd/capmon-pull/ -count=1` plus live proof `go run ./cmd/capmon-pull -check`
  - Observed: all tamper/identity/no-bundle tables pass against the captured real snapshot (real crypto, no mocks); corrupted-file pull exits non-zero with the mirror tree byte-for-byte unchanged (treeDigest before == after); the LIVE run verified the production feed end-to-end (real attestation via GitHub API, in-process sigstore-go, MOAT bundled trusted root) and printed `data_revision: ea39f43f…` exit 0
  - Evidence: `cli/internal/capfeed/attest_test.go`, `files_test.go`, `cli/cmd/capmon-pull/main_test.go`, live output in session log

- [x] Stale feed (`generated_at` > 48h) keeps last-known-good and exits non-zero — Slice 2 (`TestCheckFreshness_Boundaries`, ordering after verification per design)
  - Command: `cd cli && go test ./internal/capfeed/ -run TestCheckFreshness -v -count=1` and `go test ./cmd/capmon-pull/ -run TestMain_CheckFailsClosedOnStaleFeed -count=1`
  - Observed: boundary table passes (47h59m ok, exactly 48h ok, 48h01m error, future ok, feed-published 12h limit honored); CLI test with clock pinned 49h past `generated_at` exits non-zero printing nothing trusted
  - Evidence: `cli/internal/capfeed/freshness_test.go`, `cli/cmd/capmon-pull/main_test.go`

- [x] Tolerant reader: unknown fields/files/keys ignored, open enums, `supported` absent = unknown — Slices 1, 3, 5 (`TestParseIndex_TolerantUnknownFields`, `TestWriteMirror_VerbatimIncludingUnknownFiles`, `TestCheckFeedCoverage_Contradictions`)
  - Command: `cd cli && go test ./internal/capfeed/ ./internal/provider/ -count=1`
  - Observed: unknown top-level/per-file/provider keys and an unrecognized `extras/new-thing.json` path decode and mirror cleanly; open-enum `status` preserved; `supported` absent yields zero Coverage Drift findings; malformed JSON errors rather than silently passing
  - Evidence: `cli/internal/capfeed/index_test.go`, `mirror_test.go`, `cli/internal/provider/coverage_feed_test.go`

- [x] Capability changes produce a single rolling syllago PR with normal CI via the Aembit credential — Slice 6 manual verification
  - Command: `bash .ship/capmon-pull/checks/slice6.sh` (structural: cron+dispatch, concurrency, SHA pins, persist-credentials, Aembit step, `--body-file`, zero `${{ }}` inside run: blocks) plus a full LIVE pull into a scratch root: `go run ./cmd/capmon-pull -repo-root <scratch> -summary-file <scratch>/summary.json`
  - Observed: structural checks OK; the live pull mirrored 31 files (30 feed files + `provenance.json`; `advisories.json` correctly excluded), summary JSON carried `changed: true` + all 15 provider slugs — exactly what the workflow consumes to build the PR body
  - Evidence: `.github/workflows/capmon-pull.yml`, scratch mirror + `summary.json`. **Deferred to post-merge (flagged):** the live rolling-PR round-trip (steps 1–5 in structure.md slice 6) requires (a) the Aembit credential provisioned for syllago contents+pull-requests write and (b) the workflow file on the default branch for `workflow_dispatch`. Tracked as a follow-up bead.

- [x] Go-claims-vs-data Coverage Drift check runs as a non-required PR check and locally via `SYLLAGO_COVERAGE_FEED=1` — Slice 5 checkpoint
  - Command: `bash .ship/capmon-pull/checks/slice5.sh`
  - Observed: provider suite green; `SYLLAGO_COVERAGE_FEED=1 go test ./internal/provider/ -run TestCoverageFeedDrift` passes at HEAD (no Capability Documents committed yet → zero findings); ci.yml contains the `coverage-drift` job with the env var and full-SHA pins
  - Evidence: `.github/workflows/ci.yml` (`coverage-drift` job), `cli/internal/provider/coverage_feed_test.go`

- [x] 80%+ test coverage on `cli/internal/capfeed` — Slice 4 checkpoint coverage gate; `cli/internal/moat/` byte-identical (imported read-only)
  - Command: `bash .ship/capmon-pull/checks/slice4.sh` and `git diff main...HEAD --stat -- cli/internal/moat/`
  - Observed: `capfeed coverage: 81.0%` ≥ 80% gate; the moat diff is empty — byte-identical to main
  - Evidence: slice4 checkpoint output; empty diff

- [x] Repo conventions honored: httptest for HTTP, no mocking libraries, table-driven tests, `cd cli && make fmt` before commits, `make build` after Go changes
  - Command: `gofmt -l internal/capfeed internal/provider internal/rulestore cmd/capmon-pull` (empty), `cd cli && make build`, `grep -rL httptest` review
  - Observed: gofmt clean; `make build` passes and the binary was reinstalled to PATH; every HTTP boundary in the new tests uses `httptest.NewServer` with hand-rolled fixtures (no mocking library imports anywhere in the new code); tests are table-driven with `t.Run`
  - Evidence: clean gofmt output; test files cited above

## Manual interactions performed

1. Ran `go run ./cmd/capmon-pull -check` against the LIVE production feed: fetched the real `v1/index.json`, fetched its real attestation from GitHub's attestations API, verified SLSA provenance in-process (sigstore-go + MOAT bundled trusted root + pinned publish.yml identity), passed the freshness gate, exit 0.
2. Ran a full live pull into a scratch repo root: 31 files mirrored byte-verified, `advisories.json` excluded, `provenance.json` marker written, summary JSON listed all 15 changed providers.
3. Live rolling-PR round-trip (Aembit credential + workflow_dispatch on default branch) — **deferred to post-merge**, follow-up bead filed; all pre-merge-verifiable elements structurally asserted by `checks/slice6.sh`.

## Status: PASS
