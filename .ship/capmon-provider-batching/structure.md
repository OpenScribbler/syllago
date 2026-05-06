# Structure Outline: capmon-provider-batching

## Current / Desired / End State

**Current:** `RunCapmonCheck` calls `FindOpenCapmonIssue` + `CreateCapmonChangeIssue` (or `AppendCapmonChangeEvent`) once per `(provider, contentType, source)` inside the inner source loop, and calls `logOrCreateFetchErrorIssue` with no dedup guard on every fetch or validity failure — unconditionally invoking `gh issue create` each time. A provider with three content types and two sources each can produce up to six distinct issues per run, and fetch errors produce a new issue on every re-run.

**Desired:** `RunCapmonCheck` accumulates all hash-change events and fetch errors for a provider across all content types and sources into a single per-provider batch, then writes at most one GitHub issue per provider per run using a new provider-only anchor `<!-- capmon-check: <slug> -->`. If an open issue already exists for the provider (detected by anchor lookup), the run produces zero GitHub API calls for that provider's batch.

**End state (narrative):** After a capmon check run that detects changed content across multiple content types for a provider, reviewers see one GitHub issue with `## <content-type>` H2 sections for each changed content type and an optional `## Fetch Errors` section. Re-running against an already-open issue produces no GitHub API call. The existing `FindOpenCapmonIssue` and `CreateCapmonChangeIssue` functions are untouched (onboard.go continues to use them). The new `FindOpenCapmonProviderIssue` and `CreateCapmonProviderIssue` functions in `report.go` implement the provider-scoped anchor. The ~225 legacy per-content-type issues are unaffected and handled manually outside this change.

## Patterns to Follow

```go
// Anchor-in-body dedup (existing pattern, adapted to provider-only key):
anchor := fmt.Sprintf("<!-- capmon-check: %s -->", slug)
// Find: strings.Contains(iss.Body, anchor)
// Create: fullBody = anchor + "\n" + body

// Accumulate-then-flush shape (from pipeline.go runStage1Fetch):
batch := &providerBatch{}
for ct, ctDoc := range doc.ContentTypes {
    for _, src := range ctDoc.Sources {
        if err := runSourceCheck(ctx, opts, provider, ct, src, batch); err != nil {
            return err
        }
    }
}
if err := flushProviderBatch(ctx, opts, provider, batch); err != nil {
    return err
}

// ciMode-gated find-or-create (from check.go warning-issue path):
if ciMode && !opts.DryRun {
    _, found, findErr := FindOpenCapmonProviderIssue(provider)
    if findErr != nil { /* log, return */ }
    if found { return nil }
    _, createErr := CreateCapmonProviderIssue(ctx, provider, title, body)
    // ...
}
```

## Design Summary

The feature introduces a `providerBatch` struct (holding per-content-type change records and a flat list of fetch errors) that is instantiated once per provider before the inner source loop and passed by pointer into `runSourceCheck`. After the inner loops complete, a new `flushProviderBatch` function reads the batch and, if it is non-empty, calls `FindOpenCapmonProviderIssue` (a new function in `report.go` with the provider-only anchor) to decide whether to call `CreateCapmonProviderIssue` (also new in `report.go`) or silently skip. `logOrCreateFetchErrorIssue` is refactored to record into the batch rather than calling `gh` directly, eliminating its unconditional `gh issue create` call. The existing `FindOpenCapmonIssue` and `CreateCapmonChangeIssue` are left unchanged for `onboard.go`. The `ciMode` flag computed before the provider loop is threaded into `flushProviderBatch`.

## Slices

---

### Slice 1: Per-provider report functions with anchor-based dedup

**Observable outcome:** `FindOpenCapmonProviderIssue` and `CreateCapmonProviderIssue` can be called from test code with a stubbed `ghRunner` and correctly find-or-create an issue keyed on `<!-- capmon-check: <slug> -->`. Unit tests in `report_test.go` pass green.

**Interfaces introduced or modified:**

- `report.go` — `FindOpenCapmonProviderIssue(provider string) (issueNum int, found bool, err error)` — **Deps:** `true-external`
  - **Hides:** `gh issue list` invocation, JSON parsing of the issue list response, anchor string construction, slug validation
  - **Exposes:** A single boolean "does an open provider-scoped capmon issue exist" plus the issue number if found

- `report.go` — `CreateCapmonProviderIssue(ctx context.Context, provider, title, body string) (issueNum int, err error)` — **Deps:** `true-external`
  - **Hides:** `gh issue create` invocation, anchor prepending to body, slug validation, URL-to-number parsing from gh output
  - **Exposes:** The created issue number for the caller; body anchoring is internal

**Files:**

- `cli/internal/capmon/report.go` — add two new exported functions; no existing functions modified
- `cli/internal/capmon/report_test.go` — unit tests for both new functions

**Test cases:**

- Unit: `TestFindOpenCapmonProviderIssue_Found` — stub returns one issue whose body contains `<!-- capmon-check: test-provider -->`; assert `found=true` and correct issue number returned
- Unit: `TestFindOpenCapmonProviderIssue_NotFound` — stub returns empty list; assert `found=false`
- Unit: `TestFindOpenCapmonProviderIssue_WrongAnchor` — stub returns issue with `<!-- capmon-check: test-provider/skills -->` (old per-content-type format); assert `found=false` (legacy anchors must not match)
- Unit: `TestFindOpenCapmonProviderIssue_InvalidSlug` — call with `"INVALID SLUG"`; assert non-nil error, no gh call made
- Unit: `TestCreateCapmonProviderIssue_Success` — stub captures `--body` arg; assert issue number parsed correctly, body starts with `<!-- capmon-check: test-provider -->` anchor
- Unit: `TestCreateCapmonProviderIssue_InvalidSlug` — call with `"INVALID SLUG"`; assert non-nil error

**Checkpoint:** `cd cli && go test ./internal/capmon/ -run 'TestFindOpenCapmonProviderIssue|TestCreateCapmonProviderIssue' -v` passes; all six test cases are green.

---

### Slice 2: Batch accumulator — runSourceCheck writes into providerBatch instead of calling gh

**Observable outcome:** After refactoring `runSourceCheck` to accept `*providerBatch` and accumulate into it, a test that runs `RunCapmonCheck` with two content types that both have changed hashes produces exactly zero `gh issue create` calls during source iteration (all gh calls are deferred to the flush phase). The existing `TestRunCapmonCheck_NoChange` and `TestRunCapmonCheck_Changed` tests still pass after the signature change.

**Interfaces introduced or modified:**

- `check.go` — `providerBatch` struct (unexported) — **Deps:** `in-process`
  - **Hides:** Per-content-type change records (`map[string][]sourceChange`) and fetch error list (`[]fetchError`); provides `isEmpty()` predicate
  - **Exposes:** Mutable fields for accumulation by `runSourceCheck` and `logOrCreateFetchErrorIssue`; read-only access for body building in `flushProviderBatch`

- `check.go` — `runSourceCheck(ctx, opts, provider, contentType string, src SourceRef, batch *providerBatch) error` (signature modified) — **Deps:** `in-process`
  - **Hides:** Hash comparison, HTTP fetch delegation, decision to record a change vs fetch error vs no-op
  - **Exposes:** Returns `error` only for unexpected failures; all content-change and fetch-error events are written into `batch`, never directly to gh

- `check.go` — `logOrCreateFetchErrorIssue(ctx, opts, provider, contentType, sourceURI, reason string, batch *providerBatch)` (signature modified) — **Deps:** `in-process`
  - **Hides:** Dry-run stderr logging; batch record construction for fetch errors
  - **Exposes:** Side effect of appending to `batch.fetchErrors`; no gh call in this function

**Files:**

- `cli/internal/capmon/check.go` — add `providerBatch` struct; modify `runSourceCheck` to accept `*providerBatch`; modify `logOrCreateFetchErrorIssue` to record into batch instead of calling gh; update call site in `RunCapmonCheck` to instantiate batch and pass it

**Test cases:**

- Integration: `TestRunCapmonCheck_MultiContentType_SingleIssue` — two content types with changed hashes; assert exactly one `gh issue create` call with `capmon-change` label at end of provider (not two); confirms batching is deferred
- Integration: `TestRunCapmonCheck_NoChange` (existing) — must still pass; zero gh calls when hash matches
- Integration: `TestRunCapmonCheck_Changed` (existing) — must still pass; at least one gh call for changed hash

**Checkpoint:** `cd cli && go test ./internal/capmon/ -run 'TestRunCapmonCheck' -v` passes with all existing tests green plus `TestRunCapmonCheck_MultiContentType_SingleIssue` green.

---

### Slice 3: Flush phase — single provider issue written after source loop with dedup

**Observable outcome:** After a `RunCapmonCheck` run where one or more content types accumulated changes, exactly one `gh issue list` call and at most one `gh issue create` call are made for the provider. When the list call returns an open issue (matching `<!-- capmon-check: <slug> -->`), zero `gh issue create` calls are made. Fetch-error-only providers (no hash changes) also trigger the flush and produce one issue.

**Interfaces introduced or modified:**

- `check.go` — `flushProviderBatch(ctx context.Context, opts CapmonCheckOptions, provider string, batch *providerBatch, ciMode bool) error` (new, unexported) — **Deps:** `local-substitutable`
  - **Hides:** Issue body construction (`## <content-type>` sections + `## Fetch Errors` section), dry-run branch, `FindOpenCapmonProviderIssue` call, `CreateCapmonProviderIssue` call, `isEmpty()` guard
  - **Exposes:** Returns `error`; called once per provider from `RunCapmonCheck` after inner source loops complete

- `check.go` — `RunCapmonCheck` (modified call site) — **Deps:** `in-process`
  - **Hides:** Batch lifecycle (init before inner loop, pass to `runSourceCheck`, flush after inner loop)
  - **Exposes:** No new exported surface; existing `RunCapmonCheck` signature unchanged

**Files:**

- `cli/internal/capmon/check.go` — add `flushProviderBatch`; update `RunCapmonCheck` to call `flushProviderBatch` after the content-type/source loops; wire `ciMode` through to flush

**Test cases:**

- Integration: `TestRunCapmonCheck_BatchFlush_OpenIssueExists` — stub `gh issue list` returns one issue with `<!-- capmon-check: test-provider -->` anchor; assert zero `gh issue create` calls (silent skip)
- Integration: `TestRunCapmonCheck_FetchErrorOnly_ProducesIssue` — all sources return fetch errors, no hash changes; assert one `gh issue list` call and one `gh issue create` call containing `## Fetch Errors` section
- Integration: `TestRunCapmonCheck_MultiContentType_SingleIssue` (from Slice 2, extended) — assert issue body contains `## skills` and `## hooks` H2 sections when two content types changed
- Integration: `TestRunCapmonCheck_FetchError` (existing) — must still pass; `capmon-fetch-error` label check updated to match new batched label (or assertion relaxed to check body content)
- Integration: `TestRunCapmonCheck_DryRun` (existing) — must still pass; no gh calls

**Checkpoint:** `cd cli && go test ./internal/capmon/ -v` all tests pass; `TestRunCapmonCheck_BatchFlush_OpenIssueExists` and `TestRunCapmonCheck_FetchErrorOnly_ProducesIssue` are green.

---

### Slice 4: Body builder — structured multi-section issue body

**Observable outcome:** The helper that constructs the per-provider issue body produces a markdown document with one `## <content-type>` section per content type that had at least one changed source (each listing source URI, old hash, new hash), and a `## Fetch Errors` section listing each failed URI and reason when fetch errors are present. Unit tests assert the exact section structure and that no content-type section appears when only fetch errors exist.

**Interfaces introduced or modified:**

- `check.go` — `buildProviderIssueBody(provider string, batch *providerBatch) string` (new, unexported) — **Deps:** `in-process`
  - **Hides:** String builder logic, section ordering (changes first, fetch errors last), empty-section suppression
  - **Exposes:** A ready-to-pass `body string` for `CreateCapmonProviderIssue`; extracted values are never interpolated outside fenced blocks or list items

**Files:**

- `cli/internal/capmon/check.go` — add `buildProviderIssueBody`; call it from `flushProviderBatch`
- `cli/internal/capmon/check_test.go` — unit tests driving `buildProviderIssueBody` directly (call it via a thin exported wrapper or test it indirectly through the flush integration tests)

**Test cases:**

- Unit: `TestBuildProviderIssueBody_HashChanges` — batch with two content types, each with one changed source; assert body contains `## skills` and `## hooks` H2 headers, source URIs, old/new hashes; assert no `## Fetch Errors` section
- Unit: `TestBuildProviderIssueBody_FetchErrorsOnly` — batch with only fetch errors (no hash changes); assert body contains `## Fetch Errors` header and error reasons; assert no content-type H2 sections
- Unit: `TestBuildProviderIssueBody_Mixed` — batch with one hash change and one fetch error; assert both sections present
- Unit: `TestBuildProviderIssueBody_Empty` — empty batch; assert empty string (or `flushProviderBatch` short-circuits before calling body builder; test the guard)

**Checkpoint:** `cd cli && go test ./internal/capmon/ -run 'TestBuildProviderIssueBody' -v` passes; all four body-builder cases are green; full `go test ./internal/capmon/` still passes.

## Acceptance

- A provider with N content types each having M changed sources produces exactly one `gh issue list` call and at most one `gh issue create` call per run — verified by `TestRunCapmonCheck_MultiContentType_SingleIssue`
- When `FindOpenCapmonProviderIssue` returns `found=true`, `RunCapmonCheck` makes zero `gh issue create` calls for that provider — verified by `TestRunCapmonCheck_BatchFlush_OpenIssueExists`
- Legacy per-(provider, contentType) anchors (`<!-- capmon-check: <slug>/<ct> -->`) do not match the new provider-only anchor lookup — verified by `TestFindOpenCapmonProviderIssue_WrongAnchor`
- A provider with only fetch errors (no hash changes) still produces one issue containing a `## Fetch Errors` section — verified by `TestRunCapmonCheck_FetchErrorOnly_ProducesIssue`
- `DryRun=true` suppresses all gh calls including the flush phase — verified by `TestRunCapmonCheck_DryRun` (existing, confirmed still passing)
- `FindOpenCapmonIssue` and `CreateCapmonChangeIssue` signatures and behavior are unchanged — verified by all existing `report_test.go` tests passing
- `cd cli && go test ./internal/capmon/` passes at 80%+ coverage

## Out of Scope

- Drift PRs — `runStage4Review`, `DeduplicatePR`, `CreateDriftPR`, and `pipeline.go` are untouched
- Healing failure system — `RecordConsecutiveHealFailure`, `ResolveHealFailure`, and all `healing_*.go` files are untouched
- Hash comparison mechanics — `SHA256Hex`, `ValidateContentResponse`, and `fetchForCheck` are read-only from this feature's perspective
- Automated bulk-close of the ~225 existing open per-(provider, contentType) issues — manual ops task
- `onboard.go` — the onboarding-specific `CreateCapmonChangeIssue` call at `onboard.go:81` does not go through `runSourceCheck` and is unchanged
- `pipeline.go` (the four-stage fetch/extract/diff/review pipeline) — the batching change targets `check.go` and `report.go` only
- Adding `--limit` flag to `gh issue list` calls — future hardening; not in scope
