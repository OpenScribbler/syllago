# Plan: capmon-provider-batching

## Execution order

1. **Slice 1: Per-provider report functions** — test bead + impl bead (TDD)
   - Test: `cli/internal/capmon/report_test.go` — `TestFindOpenCapmonProviderIssue_{Found,NotFound,WrongAnchor,InvalidSlug}`, `TestCreateCapmonProviderIssue_{Success,InvalidSlug}`
   - Impl: `cli/internal/capmon/report.go` — add `FindOpenCapmonProviderIssue` and `CreateCapmonProviderIssue` with `<!-- capmon-check: <slug> -->` anchor
   - Checkpoint: `cd cli && go test ./internal/capmon/ -run 'TestFindOpenCapmonProviderIssue|TestCreateCapmonProviderIssue' -v` passes (6 cases green)

2. **Slice 2: Batch accumulator** — test bead + impl bead (TDD)
   - Test: `cli/internal/capmon/check_test.go` — `TestRunCapmonCheck_MultiContentType_SingleIssue` (new); existing `TestRunCapmonCheck_NoChange` and `TestRunCapmonCheck_Changed` must still pass
   - Impl: `cli/internal/capmon/check.go` — add `providerBatch` struct; modify `runSourceCheck` to accept `*providerBatch`; modify `logOrCreateFetchErrorIssue` to record into batch; update `RunCapmonCheck` call site
   - Checkpoint: `cd cli && go test ./internal/capmon/ -run 'TestRunCapmonCheck' -v` passes (all existing + new multi-content-type test green)

3. **Slice 3: Flush phase** — test bead + impl bead (TDD)
   - Test: `cli/internal/capmon/check_test.go` — `TestRunCapmonCheck_BatchFlush_OpenIssueExists`, `TestRunCapmonCheck_FetchErrorOnly_ProducesIssue`; existing `TestRunCapmonCheck_FetchError` and `TestRunCapmonCheck_DryRun` must still pass
   - Impl: `cli/internal/capmon/check.go` — add `flushProviderBatch`; update `RunCapmonCheck` to call flush after inner loops; wire `ciMode` through
   - Checkpoint: `cd cli && go test ./internal/capmon/ -v` all tests pass; dedup and fetch-error-only cases green

4. **Slice 4: Body builder** — test bead + impl bead (TDD)
   - Test: `cli/internal/capmon/check_test.go` — `TestBuildProviderIssueBody_{HashChanges,FetchErrorsOnly,Mixed,Empty}`
   - Impl: `cli/internal/capmon/check.go` — add `buildProviderIssueBody`; call from `flushProviderBatch`
   - Checkpoint: `cd cli && go test ./internal/capmon/ -run 'TestBuildProviderIssueBody' -v` passes; full `go test ./internal/capmon/` still passes at 80%+ coverage

## Gate

Before moving from one slice to the next: the checkpoint for the current slice must pass. If it fails, stop and involve the user — never skip ahead.

## Acceptance

- A provider with N content types each having M changed sources produces exactly one `gh issue list` call and at most one `gh issue create` call per run — verified by `TestRunCapmonCheck_MultiContentType_SingleIssue`
- When `FindOpenCapmonProviderIssue` returns `found=true`, `RunCapmonCheck` makes zero `gh issue create` calls for that provider — verified by `TestRunCapmonCheck_BatchFlush_OpenIssueExists`
- Legacy per-(provider, contentType) anchors (`<!-- capmon-check: <slug>/<ct> -->`) do not match the new provider-only anchor lookup — verified by `TestFindOpenCapmonProviderIssue_WrongAnchor`
- A provider with only fetch errors (no hash changes) still produces one issue containing a `## Fetch Errors` section — verified by `TestRunCapmonCheck_FetchErrorOnly_ProducesIssue`
- `DryRun=true` suppresses all gh calls including the flush phase — verified by `TestRunCapmonCheck_DryRun` (existing, confirmed still passing)
- `FindOpenCapmonIssue` and `CreateCapmonChangeIssue` signatures and behavior are unchanged — verified by all existing `report_test.go` tests passing
- `cd cli && go test ./internal/capmon/` passes at 80%+ coverage

## Non-TDD exemptions

None.
