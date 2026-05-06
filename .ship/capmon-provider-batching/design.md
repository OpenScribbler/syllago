# Design Discussion: capmon-provider-batching

## Summary

**Current state:** `RunCapmonCheck` calls `FindOpenCapmonIssue` + `CreateCapmonChangeIssue` or `AppendCapmonChangeEvent` once per source inside the inner source loop, keyed on `(provider, contentType)`, and calls `logOrCreateFetchErrorIssue` with no dedup on every fetch/validity failure, producing a new issue unconditionally each time.

**Desired state:** `RunCapmonCheck` accumulates all change events and fetch errors for a provider across the full source loop, then creates a single GitHub issue per provider (anchored on Provider Slug alone) after the inner loop completes; if an open issue for that provider already exists, the run is silent.

**End state (narrative):** After a capmon check run with changed content across multiple Content Types, reviewers see one GitHub issue per Provider Slug, with `## <content-type>` sections listing each changed source's old and new hash, and an optional `## Fetch Errors` section. Re-running capmon against an already-open issue produces no GitHub API call at all. The ~225 legacy per-content-type issues remain open and are closed manually as a one-time ops task outside this code change.

## Research questions answered

- Q1: What is the complete call path from RunCapmonCheck to issue creation?
- Q2: What data is available inside runSourceCheck at the point issue creation fires?
- Q3: What are the exact gh CLI arguments FindOpenCapmonIssue passes, and what is the shape of the JSON response it parses?
- Q4: What does CreateCapmonChangeIssue write to the issue body?
- Q5: What are all callers of FindOpenCapmonIssue and CreateCapmonChangeIssue?
- Q6: What does logOrCreateFetchErrorIssue write, and are there existing dedup attempts?
- Q7: What existing tests cover the affected functions?
- Q8: Does RunCapmonCheck return early on error mid-provider, and how does that interact with a deferred-write approach?

## Patterns to Follow

### Pattern: ciMode-gated warning issue dedup (FindOpenCapmonWarningIssue + anchor)

**Source:** `cli/internal/capmon/check.go:140`

**Snippet:**
```go
if ciMode && !opts.DryRun {
    issueNum, found, findErr := FindOpenCapmonWarningIssue(provider, w)
    if findErr != nil {
        fmt.Fprintf(os.Stderr, "warning: find warning issue for %s: %v\n", provider, findErr)
        continue
    }
    if found {
        _ = AppendCapmonChangeEvent(ctx, issueNum, ...)
    } else {
        _, createErr := CreateCapmonWarningIssue(ctx, provider, w)
        ...
    }
}
```

**Why it applies here:** The warning-issue path already does exactly what fetch-error issues need: find-or-create keyed on an anchor, silent skip when found. The new provider-batched change issue follows the same lookup-before-create shape, scaled up to per-provider rather than per-warning.

---

### Pattern: anchor-in-body dedup (FindOpenCapmonIssue + CreateCapmonChangeIssue)

**Source:** `cli/internal/capmon/report.go:148` and `cli/internal/capmon/report.go:185`

**Snippet:**
```go
// Find:
anchor := fmt.Sprintf("<!-- capmon-check: %s/%s -->", slug, contentType)
// strings.Contains(iss.Body, anchor) — report.go:174

// Create:
anchor := fmt.Sprintf("<!-- capmon-check: %s/%s -->", slug, contentType)
fullBody := anchor + "\n" + body
```

**Why it applies here:** The anchor pattern is the dedup mechanism this feature builds on. The new design changes the anchor key from `<slug>/<contentType>` to `<slug>` (provider-only), and widens the body from a single-source message to a multi-section document. The find-then-create structure and anchor placement at body head remain the same.

---

### Pattern: accumulate-then-write (deferred write after inner loop)

**Source:** `cli/internal/capmon/pipeline.go:155`

**Snippet:**
```go
for _, m := range manifests {
    status := manifest.Providers[m.Slug]
    // ...inner content-type + source loops that populate status...
    manifest.Providers[m.Slug] = status  // write-back after full provider loop
}
```

**Why it applies here:** `runStage1Fetch` in pipeline.go already accumulates per-source results into a `ProviderStatus` struct and writes back after the full provider loop. The batched issue write follows the same accumulate-then-flush shape: collect change events and fetch errors across all content types, flush once per provider after the inner loops end.

---

### Pattern: SetGHCommandForTest for gh interception

**Source:** `cli/internal/capmon/report.go:32` (definition), `cli/internal/capmon/check_test.go:114` (usage)

**Snippet:**
```go
// Definition:
func SetGHCommandForTest(fn func(args ...string) ([]byte, error)) {
    if fn == nil { ghRunner = /* default exec */; return }
    ghRunner = fn
}

// Usage in tests:
capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
    cp := make([]string, len(args))
    copy(cp, args)
    *calls = append(*calls, cp)
    if args[0] == "issue" && args[1] == "list" {
        return []byte(`[]`), nil
    }
    if args[0] == "issue" && args[1] == "create" {
        return []byte("https://github.com/test/repo/issues/1\n"), nil
    }
    return []byte(""), nil
})
t.Cleanup(func() { capmon.SetGHCommandForTest(nil) })
```

**Why it applies here:** All new tests for the batched-issue write path must intercept gh calls via `SetGHCommandForTest` to assert that issue list is called with the correct provider anchor and that issue create is called exactly once (or zero times when an issue is found). The existing `captureGHCalls` helper in check_test.go is the reference model for new test helpers that inspect call count and arguments more precisely.

---

### Disambiguation: per-provider anchor vs per-(provider, contentType) anchor

**Chosen:** Per-provider anchor `<!-- capmon-check: <slug> -->` (new)
**Considered:** Per-(provider, contentType) anchor `<!-- capmon-check: <slug>/<contentType> -->` (`cli/internal/capmon/report.go:153`, `cli/internal/capmon/report.go:190`)

**Why:** The design concept establishes that one issue per Provider Slug is the target state. Using the provider-only anchor as the dedup key means `FindOpenCapmonIssue` needs one `gh issue list` call regardless of how many Content Types changed in a run. Using the existing per-(provider, contentType) anchor would require either multiple find-or-create calls (replicating the race window the design explicitly rejects) or a compound lookup that scans all content types, which is more complex with no benefit. The new anchor is a narrower key that correctly represents the new batching granularity.

**Consequences:** `FindOpenCapmonIssue` and `CreateCapmonChangeIssue` in report.go currently take `contentType string` as a parameter and embed it in the anchor. The new per-provider variants will omit `contentType` from the anchor and from their signatures. The existing `(provider, contentType)`-scoped functions remain unchanged because `onboard.go:81` calls `CreateCapmonChangeIssue` directly at the existing granularity and is explicitly out of scope for this change. Structure must introduce new `FindOpenCapmonProviderIssue` and `CreateCapmonProviderIssue` functions rather than modifying the existing ones.

---

### Disambiguation: silent skip vs comment-on-redetection

**Chosen:** Silent skip — make no GitHub API call when an open issue already exists for the Provider Slug
**Considered:** Append a comment to the existing issue on redetection (current behavior for the per-content-type path via `AppendCapmonChangeEvent` at `cli/internal/capmon/check.go:228`)

**Why:** The design concept explicitly calls out that comment spam adds noise without helping reviewers prioritize. The existing open issue is the signal; appending identical hash-change messages each run buries the original event in noise. The warning-issue path (check.go:140–155) already uses a combined find-or-create pattern but does append on re-detection — however, warning re-detection carries state (the violation is still present), whereas a hash-change re-detection does not carry new information (the old issue documents the changed hash already). The two cases are semantically different; silencing re-detection is correct only for the hash-change issue type.

**Consequences:** The `AppendCapmonChangeEvent` function is not called from `runSourceCheck` under the new design. It remains available for the warning-issue path (check.go:147) which legitimately appends. Any future decision to re-enable appending on hash-change redetection would require revisiting this ADR. Tests that currently stub `AppendCapmonChangeEvent` behavior need to be updated to assert it is NOT called.

## Design Questions (resolved)

1. **Where does the per-provider accumulator live during the source loop?**
   - **Chosen: B** — A new `providerBatch` struct instantiated per provider and threaded through `runSourceCheck` as a parameter. Named struct makes batched state explicit and testable in isolation.

2. **How should the `## Fetch Errors` section be conditioned?**
   - **Chosen: A** — Always emit `## Fetch Errors` if any fetch/validity errors exist for the provider, even with no hash changes. Fetch errors are actionable signals regardless of whether a hash change also occurred.

3. **How should the `gh issue list` query be scoped to avoid legacy false matches?**
   - **Chosen: A** — Rely on anchor disambiguation alone. Legacy `<!-- capmon-check: <slug>/<contentType> -->` anchors won't match new `<!-- capmon-check: <slug> -->`. No new label needed.

## Decisions made (not questions)

- The batched issue body uses `## <content-type>` H2 sections, one per Content Type that had at least one changed source, listing each source URI with its old and new hash — this is derived directly from the design concept and is consistent with the PR body pattern in `BuildPRBody` (`report.go:350`) which uses H2 sections per field.
- `logOrCreateFetchErrorIssue` will be replaced or wrapped with dedup logic that calls the new `FindOpenCapmonProviderIssue` before creating — the research shows it currently calls `ghRunner` directly with no find-or-create guard (`check.go:243`), and the design concept calls out adding dedup logic as an explicit in-scope item.
- `logOrCreateFetchErrorIssue`'s unused `ctx` parameter will be wired through to the `ghRunner` call as part of this change, following the same pattern as `CreateCapmonChangeIssue` (`report.go:185`) which accepts but effectively ignores ctx today; wiring it is a correctness improvement at no cost.
- The dedup check fires once per provider (not per content type) before any issue body is assembled, matching the "find-then-create" shape in `FindOpenCapmonIssue` + `CreateCapmonChangeIssue`.
- `onboard.go:81` is unchanged — it calls `CreateCapmonChangeIssue` directly without a find-or-create guard and that path is explicitly out of scope.
- The `ciMode` flag (`check.go:113`) is preserved and must remain accessible to the deferred write phase; it is computed once from `os.Getenv("GITHUB_TOKEN")` before the provider loop and does not change mid-run.

## Out of Scope

- Drift PRs — the design concept and research both scope this to the check pipeline only; `runStage4Review` / `DeduplicatePR` / `CreateDriftPR` in pipeline.go are untouched.
- Healing failure system — `RecordConsecutiveHealFailure`, `ResolveHealFailure`, and all healing_*.go files are untouched.
- Hash comparison mechanics — `SHA256Hex`, `ValidateContentResponse`, and `fetchForCheck` are read-only from this feature's perspective.
- Automated bulk-close of the ~225 existing open issues — manual ops task, explicitly called out in the design concept.
- `onboard.go` — the onboarding-specific `CreateCapmonChangeIssue` call at `onboard.go:81` does not go through `runSourceCheck` and is out of scope.
- Changes to `pipeline.go` (the four-stage fetch/extract/diff/review pipeline) — the batching change targets `check.go` and `report.go` only.
- Adding the `--limit` flag to `gh issue list` calls — a future hardening concern; not in scope here.

## Interfaces affected (preview)

- `cli/internal/capmon/report.go` — new `FindOpenCapmonProviderIssue(provider string)` and `CreateCapmonProviderIssue(ctx, provider, title, body string)` functions with per-provider anchor; existing `FindOpenCapmonIssue` and `CreateCapmonChangeIssue` remain unchanged
- `cli/internal/capmon/check.go` — `runSourceCheck` accumulates into a `providerBatch` rather than calling GitHub directly; new `flushProviderBatch` (or equivalent) function called after the inner source loops; `logOrCreateFetchErrorIssue` gains dedup via `FindOpenCapmonProviderIssue`
- `cli/internal/capmon/check_test.go` — new integration tests covering: multi-content-type provider produces exactly one issue, provider with only fetch errors produces exactly one issue, open issue causes zero gh calls, DryRun suppresses all gh calls across batch

DESIGN_COMPLETE
