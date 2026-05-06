# Research: capmon-provider-batching

## Q1: What is the complete call path from RunCapmonCheck to issue creation — which functions are called in what order, and what data flows between them?

`RunCapmonCheck` is defined in `check.go:68`. The call path for issue creation on a changed source hash:

1. `RunCapmonCheck(ctx, opts CapmonCheckOptions)` — `check.go:68`
   - Calls `loadProviderSlugs(opts.ProvidersJSON)` — `check.go:87`
   - Calls `os.ReadDir(opts.FormatsDir)` to enumerate providers — `check.go:93`
   - For each provider slug:
     - `ValidateSources(opts.SourcesDir, provider)` — `check.go:122` (blocking; returns error aborts provider loop)
     - `ValidateFormatDocWithWarnings(opts.FormatsDir, opts.CanonicalKeysPath, provider)` — `check.go:129` (blocking error aborts; non-blocking warnings go to stderr/GH)
     - `LoadFormatDoc(FormatDocPath(opts.FormatsDir, provider))` — `check.go:166`
     - For each `ct` (content type string) and each `src SourceRef` in `doc.ContentTypes[ct].Sources`:
       - `runSourceCheck(ctx, opts, provider, ct, src)` — `check.go:173`

2. `runSourceCheck(ctx, opts, provider, contentType string, src SourceRef)` — `check.go:186`
   - Calls `fetchForCheck(ctx, src.URI)` — `check.go:188`
     - Returns `(body []byte, respContentType string, finalURL string, err error)`
   - On fetch error: calls `logOrCreateFetchErrorIssue(...)` then returns `nil`
   - Calls `ValidateContentResponse(body, respContentType, src.URI, finalURL)` — `check.go:196`
   - On validity error: calls `logOrCreateFetchErrorIssue(...)` then returns `nil`
   - Computes `newHash := SHA256Hex(body)` — `check.go:203`
   - If `src.ContentHash != "" && src.ContentHash == newHash`: returns `nil` (no change)
   - Constructs `message string` — `check.go:209`
   - If `opts.DryRun`: logs to stderr and returns `nil`
   - Calls `FindOpenCapmonIssue(provider, contentType)` — `check.go:219`
     - Returns `(issueNum int, found bool, err error)`
   - If found: calls `AppendCapmonChangeEvent(ctx, issueNum, message)` — `check.go:228`
   - If not found: calls `CreateCapmonChangeIssue(ctx, provider, contentType, title, message)` — `check.go:230`

Data flows:
- `provider` (string slug), `contentType` (string key from `doc.ContentTypes` map), `src` (`SourceRef` struct) are passed from the outer loop into `runSourceCheck`.
- `src.URI`, `src.ContentHash` are read from `SourceRef`.
- `newHash` computed from HTTP response body is the only new data created inside `runSourceCheck`.
- `title` is constructed locally: `"capmon: content change detected for <provider>/<contentType>"` — `check.go:226`.
- `message` is constructed locally — `check.go:209`.

## Q2: What data is available inside runSourceCheck at the point issue creation currently fires — what fields exist on SourceRef, and what does the message string contain?

`SourceRef` is defined in `formatdoc.go:49`. Fields:

```go
type SourceRef struct {
    URI         string `yaml:"uri"`
    Type        string `yaml:"type"`
    FetchMethod string `yaml:"fetch_method"`
    ContentHash string `yaml:"content_hash"`
    FetchedAt   string `yaml:"fetched_at"`
    Name        string `yaml:"name,omitempty"`
    Section     string `yaml:"section,omitempty"`
}
```

At the point issue creation fires (after hash comparison), the following data is in scope in `runSourceCheck`:
- `provider` — provider slug string (e.g., `"claude-code"`)
- `contentType` — content type key string (e.g., `"skills"`)
- `src` — full `SourceRef` struct: `URI`, `Type`, `FetchMethod`, `ContentHash` (the old hash stored in the format doc), `FetchedAt`, `Name`, `Section`
- `body` — the full fetched HTTP response body (`[]byte`)
- `newHash` — `SHA256Hex(body)` result, a `"sha256:<hex>"` string
- `opts` — `CapmonCheckOptions` struct (includes `DryRun`, paths, `ProviderFilter`)
- `ctx` — context

The `message` string is constructed at `check.go:209`:
```
Content hash changed for <provider>/<contentType> source <src.URI>:
Old hash: <src.ContentHash>
New hash: <newHash>
```

The `title` string is constructed at `check.go:226`:
```
capmon: content change detected for <provider>/<contentType>
```

Neither the `src.Name`, `src.Type`, `src.FetchMethod`, nor `src.Section` fields are used in the message or title.

## Q3: What are the exact gh CLI arguments FindOpenCapmonIssue passes, and what is the shape of the JSON response it parses?

`FindOpenCapmonIssue` is defined in `report.go:148`. The gh CLI call at `report.go:155`:

```
gh issue list
  --label capmon-change
  --label provider:<slug>
  --state open
  --json number,body
```

The JSON response is parsed into:
```go
[]struct {
    Number int    `json:"number"`
    Body   string `json:"body"`
}
```

The function then iterates the issues and calls `strings.Contains(iss.Body, anchor)` where `anchor = "<!-- capmon-check: <slug>/<contentType> -->"` — `report.go:153` and `report.go:173`.

No `--limit` flag is passed. No `--search` or `--repo` flags are passed.

## Q4: What does CreateCapmonChangeIssue write to the issue body — anchor placement, label set, and body format?

`CreateCapmonChangeIssue` is defined in `report.go:185`. The gh CLI call at `report.go:193`:

```
gh issue create
  --title <title>
  --label capmon-change
  --label provider:<slug>
  --body <fullBody>
```

The `fullBody` is constructed at `report.go:191`:
```
<!-- capmon-check: <slug>/<contentType> -->
<body>
```

Where `body` is whatever the caller passes (the `message` string from `runSourceCheck`). The anchor is placed at the very beginning of the body, followed by a newline, then the caller-supplied body.

`gh issue create` (without `--json`) prints the issue URL to stdout (`report.go:203`). The function parses the issue number by splitting on `/` and taking the last segment with `strconv.Atoi` — `report.go:207-213`.

## Q5: What are all callers of FindOpenCapmonIssue and CreateCapmonChangeIssue, including any test files?

**FindOpenCapmonIssue** callers:
- `check.go:219` — inside `runSourceCheck`, called once per source that has a changed hash
- `report_test.go:159` — `TestFindOpenCapmonIssue_Found`
- `report_test.go:177` — `TestFindOpenCapmonIssue_NotFound`
- `report_test.go:193` — `TestFindOpenCapmonIssue_WrongAnchor`

**CreateCapmonChangeIssue** callers:
- `check.go:230` — inside `runSourceCheck`, called when no open issue is found for the provider/contentType pair
- `onboard.go:81` — inside `OnboardProvider`, called once per content type after fetching initial sources
- `report_test.go:215` — `TestCreateCapmonChangeIssue_Success`
- `report_test.go:232` — `TestCreateCapmonChangeIssue_InvalidSlug`

The `onboard.go:81` call uses a different title/body format (onboarding-specific wording) and does not go through `runSourceCheck`.

## Q6: What does logOrCreateFetchErrorIssue write — title format, labels, body format — and are there any existing dedup attempts in that function or at its call sites?

`logOrCreateFetchErrorIssue` is defined in `check.go:236`. The gh CLI call at `check.go:243`:

```
gh issue create
  --title "capmon: fetch error for <slug>/<contentType>"
  --label capmon-fetch-error
  --label provider:<slug>
  --body "Source URI: <sourceURI>\nReason: <reason>"
```

Where `<slug>` is the result of `SanitizeSlug(provider)` — `check.go:242`. The function uses `_, _ =` (discards both the output and the error).

The `<reason>` parameter at its two call sites in `runSourceCheck`:
- Fetch error path (`check.go:190-191`): `"fetch error: <fetchErr.Error()>"`
- Validity failure path (`check.go:197-198`): `"content invalid: <err.Error()>"`

**Dedup attempts:** There are none. Neither the function body nor its two call sites check for an existing open issue before calling `gh issue create`. Each fetch error or validity failure unconditionally creates a new issue. There is no `FindOpenCapmonIssue`-style lookup, no anchor in the body, and no equivalent of `FindOpenCapmonWarningIssue`. The return value of `ghRunner` is explicitly discarded with `_, _`.

## Q7: What existing tests cover runSourceCheck, FindOpenCapmonIssue, CreateCapmonChangeIssue, and logOrCreateFetchErrorIssue — what is mocked via SetGHCommandForTest, and what are the coverage gaps?

**runSourceCheck** — tested only indirectly through `RunCapmonCheck` integration tests in `check_test.go`. There is no direct test of `runSourceCheck` (it is unexported). The integration tests that exercise it:
- `TestRunCapmonCheck_NoChange` (`check_test.go:156`) — hash matches, verifies zero gh calls
- `TestRunCapmonCheck_Changed` (`check_test.go:178`) — stale hash, verifies at least one gh call is made
- `TestRunCapmonCheck_FetchError` (`check_test.go:200`) — transport error, verifies `capmon-fetch-error` label appears
- `TestRunCapmonCheck_ContentValidityFailure` (`check_test.go:231`) — tiny body, verifies `capmon-fetch-error` label
- `TestRunCapmonCheck_DryRun` (`check_test.go:508`) — verifies no gh calls when `DryRun=true`

**FindOpenCapmonIssue** — three unit tests in `report_test.go`:
- `TestFindOpenCapmonIssue_Found` (`report_test.go:153`) — stub returns one issue with matching anchor
- `TestFindOpenCapmonIssue_NotFound` (`report_test.go:171`) — stub returns empty list
- `TestFindOpenCapmonIssue_WrongAnchor` (`report_test.go:186`) — stub returns issue with non-matching content type in anchor

**CreateCapmonChangeIssue** — two unit tests in `report_test.go`:
- `TestCreateCapmonChangeIssue_Success` (`report_test.go:202`) — verifies anchor presence and issue number parsing
- `TestCreateCapmonChangeIssue_InvalidSlug` (`report_test.go:231`) — verifies slug validation error

**logOrCreateFetchErrorIssue** — no direct unit tests. Covered only indirectly via `TestRunCapmonCheck_FetchError` and `TestRunCapmonCheck_ContentValidityFailure` in `check_test.go`. Neither test inspects the title or body format; both check only that a gh call with `"capmon-fetch-error"` as an argument was made.

**What SetGHCommandForTest mocks:** `SetGHCommandForTest` (defined in `report.go:32`) replaces the `ghRunner` package-level variable (`report.go:19`). The mock intercepts all `gh` invocations (issue list, issue create, issue comment, issue close, pr create, pr list). In `check_test.go`, the `captureGHCalls` helper (`check_test.go:111`) records every call's argument slice and returns `[]` for `issue list` and a fake URL for `issue create`. In `report_test.go`, each test installs its own inline stub via `SetGHCommandForTest` and restores with `defer SetGHCommandForTest(nil)`.

**Coverage gaps:**
- No test verifies the exact title, body format, or label set written by `logOrCreateFetchErrorIssue`.
- No test verifies that `logOrCreateFetchErrorIssue` creates a new issue unconditionally on every call (no dedup guard exists to test).
- No test exercises the `FindOpenCapmonIssue`-found branch in `runSourceCheck` (the `AppendCapmonChangeEvent` path at `check.go:228`).
- No test covers a provider with multiple content types or multiple sources per content type.
- No test verifies that `FindOpenCapmonIssue` is called with the correct `provider` and `contentType` arguments during a `Changed` run.

## Q8: Does RunCapmonCheck return early on error mid-provider or mid-source, and how does that interact with a deferred issue-write approach where all GH calls happen after the inner loop?

**Mid-provider early returns (blocking):**
- `ValidateSources` failure at `check.go:122-124`: `return fmt.Errorf(...)` — exits the entire provider loop immediately. No subsequent providers are processed.
- `ValidateFormatDocWithWarnings` blocking error at `check.go:130-132`: `return fmt.Errorf(...)` — same.
- `LoadFormatDoc` failure at `check.go:167-169`: `return fmt.Errorf(...)` — same.

**Mid-source behavior:**
- `runSourceCheck` returning a non-nil error causes `return err` at `check.go:174`, which exits the provider loop immediately. However, `runSourceCheck` itself returns `nil` in all fetch-error and validity-error cases (non-blocking by design). `runSourceCheck` only returns a non-nil error from `AppendCapmonChangeEvent` or `CreateCapmonChangeIssue` gh failures — `check.go:228-231`. A gh failure on one source therefore aborts the entire remaining source and provider iteration.

**Interaction with a deferred issue-write approach:**
- If GH calls were deferred until after the inner source loop, the `ValidateSources`, `ValidateFormatDocWithWarnings`, and `LoadFormatDoc` blocking returns would still fire before any deferred writes for that provider. Those early returns would skip the deferred write for the erroring provider but not for previously completed providers if the writes were accumulated per-provider.
- The current inner-loop `return err` on GH failure (line 174) stops processing remaining sources for the current provider and all subsequent providers. If GH calls were deferred, a GH failure at write time would occur after all sources had been inspected, with no cross-source interaction risk.
- The `ciMode` flag (`check.go:113`) is computed once before the provider loop from `os.Getenv("GITHUB_TOKEN") != ""`. It does not change mid-run. Any deferred write approach would need to preserve access to this flag.

## Cross-cutting observations

- `ghRunner` is a package-level variable (`report.go:19`), not injected per-function. All GH functions in the package share it, so `SetGHCommandForTest` affects all GH calls globally within a test.
- `logOrCreateFetchErrorIssue` does not accept a `ctx context.Context` at its gh call site — it calls `ghRunner` directly (not `GHRunner`), and the `ctx` parameter it receives is unused. `check.go:236-248`.
- The `ciMode` branch for warning issues (`check.go:140-155`) has its own dedup pattern (`FindOpenCapmonWarningIssue` + anchor) that `logOrCreateFetchErrorIssue` lacks entirely.
- `CreateCapmonChangeIssue` and `FindOpenCapmonIssue` operate at provider+contentType granularity; there is no per-source URI granularity in the dedup anchor. Multiple sources for the same provider/contentType all share one issue thread.
- `onboard.go` calls `CreateCapmonChangeIssue` directly without calling `FindOpenCapmonIssue` first — it does not check for an existing open issue before creating.
- The `check_test.go` `captureGHCalls` helper returns `"https://github.com/test/repo/issues/1\n"` for any `issue create` call, so `TestRunCapmonCheck_Changed` verifies a gh call is made but does not verify which content type or provider the issue is for.
